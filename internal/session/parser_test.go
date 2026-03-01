package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantNil  bool
		wantType string
	}{
		{"empty", "", true, ""},
		{"malformed", "not json", true, ""},
		{"user message", `{"type":"user","timestamp":"2026-01-01T10:00:00Z"}`, false, "user"},
		{"assistant message", `{"type":"assistant","timestamp":"2026-01-01T10:00:05Z"}`, false, "assistant"},
		{"result", `{"type":"result","input_tokens":100}`, false, "result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _ := ParseLine([]byte(tt.line))
			if tt.wantNil {
				if obj != nil {
					t.Errorf("expected nil, got %v", obj)
				}
				return
			}
			if obj == nil {
				t.Fatal("expected non-nil result")
			}
			if typ, _ := obj["type"].(string); typ != tt.wantType {
				t.Errorf("type = %q, want %q", typ, tt.wantType)
			}
		})
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		content interface{}
		full    bool
		want    string
	}{
		{"nil content", "user", nil, false, ""},
		{"string content", "user", "hello world", false, "hello world"},
		{"text block", "assistant", []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
		}, false, "hello"},
		{"tool_use summarized", "assistant", []interface{}{
			map[string]interface{}{"type": "tool_use", "name": "Read", "input": map[string]interface{}{"path": "/foo"}},
		}, false, "[tool: Read]"},
		{"tool_use full", "assistant", []interface{}{
			map[string]interface{}{"type": "tool_use", "name": "Read", "input": map[string]interface{}{"path": "/foo"}},
		}, true, `[tool: Read] {"path":"/foo"}`},
		{"mixed blocks", "assistant", []interface{}{
			map[string]interface{}{"type": "text", "text": "Let me read the file."},
			map[string]interface{}{"type": "tool_use", "name": "Bash", "input": map[string]interface{}{}},
		}, false, "Let me read the file.\n[tool: Bash]"},
		{"thinking block skipped", "assistant", []interface{}{
			map[string]interface{}{"type": "thinking", "thinking": "hmm"},
			map[string]interface{}{"type": "text", "text": "Result"},
		}, false, "Result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractContent(tt.typ, tt.content, tt.full)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMeta(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-session.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "What is Go?"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "text", "text": "Go is a programming language."},
			}}},
		{"type": "result", "input_tokens": float64(100), "output_tokens": float64(50)},
		{"type": "user", "timestamp": "2026-01-15T10:05:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Tell me more."}},
		{"type": "assistant", "timestamp": "2026-01-15T10:05:10Z", "uuid": "a2",
			"message": map[string]interface{}{"role": "assistant", "content": "It was created by Google."}},
		{"type": "result", "input_tokens": float64(200), "output_tokens": float64(75)},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}

	if meta.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want %q", meta.SessionID, "test-session")
	}
	if meta.FirstMessage != "What is Go?" {
		t.Errorf("FirstMessage = %q, want %q", meta.FirstMessage, "What is Go?")
	}
	if meta.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want %d", meta.MessageCount, 4)
	}
	if meta.Tokens.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want %d", meta.Tokens.InputTokens, 300)
	}
	if meta.Tokens.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want %d", meta.Tokens.OutputTokens, 125)
	}

	expectedDuration := 5*time.Minute + 10*time.Second
	if int(meta.DurationSecs) != int(expectedDuration.Seconds()) {
		t.Errorf("DurationSecs = %.0f, want %.0f", meta.DurationSecs, expectedDuration.Seconds())
	}
}

func TestExtractMetaEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", meta.MessageCount)
	}
}

func TestExtractMetaMalformedLines(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "mixed.jsonl")

	content := "not json\n" +
		`{"type":"user","timestamp":"2026-01-15T10:00:00Z","uuid":"u1","message":{"role":"user","content":"hello"}}` + "\n" +
		"also bad\n" +
		`{"type":"assistant","timestamp":"2026-01-15T10:00:05Z","uuid":"a1","message":{"role":"assistant","content":"world"}}` + "\n"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", meta.MessageCount)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a longer string", 10, "this is..."},
		{"exactly10!", 10, "exactly10!"},
		{"newlines\nhere", 20, "newlines here"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func writeMockSession(t *testing.T, filePath string, entries []map[string]interface{}) {
	t.Helper()
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
}

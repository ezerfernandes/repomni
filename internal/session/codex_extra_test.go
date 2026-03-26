package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodexLineMalformed(t *testing.T) {
	rec := parseCodexLine([]byte("not json"))
	if rec != nil {
		t.Error("expected nil for malformed JSON")
	}
}

func TestParseCodexLineEmptyObject(t *testing.T) {
	rec := parseCodexLine([]byte(`{}`))
	if rec != nil {
		t.Error("expected nil for empty object")
	}
}

func TestParseCodexLineSessionMetaNilPayload(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"session_meta"}`))
	if rec != nil {
		t.Error("expected nil for session_meta with no payload")
	}
}

func TestParseCodexLineResponseItemNilPayload(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"response_item"}`))
	if rec != nil {
		t.Error("expected nil for response_item with no payload")
	}
}

func TestParseCodexLineResponseItemUnknownType(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"response_item","payload":{"type":"unknown_type"}}`))
	if rec != nil {
		t.Error("expected nil for response_item with unknown payload type")
	}
}

func TestParseCodexLineResponseItemMessageEmptyRole(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"response_item","payload":{"type":"message","role":"","content":[{"type":"input_text","text":"hello"}]}}`))
	if rec != nil {
		t.Error("expected nil for message with empty role")
	}
}

func TestParseCodexLineResponseItemMessageEmptyText(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"response_item","payload":{"type":"message","role":"user","content":[]}}`))
	if rec != nil {
		t.Error("expected nil for message with empty text")
	}
}

func TestParseCodexLineResponseItemFunctionCall(t *testing.T) {
	rec := parseCodexLine([]byte(`{"timestamp":"2026-01-01T10:00:00Z","type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"{\"cmd\":\"ls\"}","call_id":"c1"}}`))
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.FunctionCall == nil {
		t.Fatal("expected FunctionCall")
	}
	if rec.FunctionCall.Name != "shell" {
		t.Errorf("name = %q, want shell", rec.FunctionCall.Name)
	}
	if rec.FunctionCall.CallID != "c1" {
		t.Errorf("call_id = %q, want c1", rec.FunctionCall.CallID)
	}
}

func TestParseCodexLineResponseItemFunctionCallOutput(t *testing.T) {
	rec := parseCodexLine([]byte(`{"timestamp":"2026-01-01T10:00:00Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"hello"}}`))
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.FunctionOut == nil {
		t.Fatal("expected FunctionOut")
	}
	if rec.FunctionOut.CallID != "c1" {
		t.Errorf("call_id = %q, want c1", rec.FunctionOut.CallID)
	}
	if rec.FunctionOut.Output != "hello" {
		t.Errorf("output = %q, want hello", rec.FunctionOut.Output)
	}
}

func TestParseCodexLineEventMsgNonTokenCount(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"event_msg","payload":{"type":"user_message"}}`))
	if rec != nil {
		t.Error("expected nil for event_msg that is not token_count")
	}
}

func TestParseCodexLineEventMsgNilPayload(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"event_msg"}`))
	if rec != nil {
		t.Error("expected nil for event_msg with nil payload")
	}
}

func TestParseCodexLineEventMsgTokenCount(t *testing.T) {
	rec := parseCodexLine([]byte(`{"timestamp":"2026-01-01T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"output_tokens":50,"cached_input_tokens":20,"reasoning_output_tokens":10}}}}`))
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.TokenCount == nil {
		t.Fatal("expected TokenCount")
	}
	if rec.TokenCount.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", rec.TokenCount.InputTokens)
	}
	if rec.TokenCount.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", rec.TokenCount.OutputTokens)
	}
	if rec.TokenCount.CachedTokens != 20 {
		t.Errorf("CachedTokens = %d, want 20", rec.TokenCount.CachedTokens)
	}
	if rec.TokenCount.ReasonTokens != 10 {
		t.Errorf("ReasonTokens = %d, want 10", rec.TokenCount.ReasonTokens)
	}
}

func TestParseCodexLineOldFormatHeader(t *testing.T) {
	rec := parseCodexLine([]byte(`{"id":"abc-123","timestamp":"2026-01-01T10:00:00Z"}`))
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.SessionHeader == nil {
		t.Fatal("expected SessionHeader")
	}
	if rec.SessionHeader.ID != "abc-123" {
		t.Errorf("ID = %q, want abc-123", rec.SessionHeader.ID)
	}
}

func TestParseCodexLineOldFormatFunctionCall(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"function_call","name":"shell","arguments":"{\"cmd\":\"ls\"}","call_id":"c1"}`))
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.FunctionCall == nil {
		t.Fatal("expected FunctionCall")
	}
	if rec.FunctionCall.Name != "shell" {
		t.Errorf("name = %q, want shell", rec.FunctionCall.Name)
	}
}

func TestParseCodexLineOldFormatFunctionCallOutput(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"function_call_output","call_id":"c1","output":"result"}`))
	if rec == nil {
		t.Fatal("expected non-nil record")
	}
	if rec.FunctionOut == nil {
		t.Fatal("expected FunctionOut")
	}
}

func TestParseCodexLineOldFormatUnknownType(t *testing.T) {
	rec := parseCodexLine([]byte(`{"type":"reasoning"}`))
	if rec != nil {
		t.Error("expected nil for unknown old-format type")
	}
}

func TestParseCodexTokenCountNilInfo(t *testing.T) {
	rec := parseCodexTokenCount("2026-01-01T10:00:00Z", map[string]interface{}{})
	if rec != nil {
		t.Error("expected nil when info is missing")
	}
}

func TestParseCodexTokenCountNilUsage(t *testing.T) {
	rec := parseCodexTokenCount("2026-01-01T10:00:00Z", map[string]interface{}{
		"info": map[string]interface{}{},
	})
	if rec != nil {
		t.Error("expected nil when total_token_usage is missing")
	}
}

func TestExtractCodexMessageTextEmptyContent(t *testing.T) {
	got := extractCodexMessageText(map[string]interface{}{})
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractCodexMessageTextNonArrayContent(t *testing.T) {
	got := extractCodexMessageText(map[string]interface{}{
		"content": "just a string",
	})
	if got != "" {
		t.Errorf("expected empty for non-array content, got %q", got)
	}
}

func TestExtractCodexMessageTextNonMapBlock(t *testing.T) {
	got := extractCodexMessageText(map[string]interface{}{
		"content": []interface{}{"not a map"},
	})
	if got != "" {
		t.Errorf("expected empty for non-map blocks, got %q", got)
	}
}

func TestExtractCodexMessageTextUnknownBlockType(t *testing.T) {
	got := extractCodexMessageText(map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "unknown", "text": "ignored"},
		},
	})
	if got != "" {
		t.Errorf("expected empty for unknown block type, got %q", got)
	}
}

func TestExtractCodexMessageTextMultipleBlocks(t *testing.T) {
	got := extractCodexMessageText(map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "input_text", "text": "part1"},
			map[string]interface{}{"type": "output_text", "text": "part2"},
		},
	})
	if got != "part1\npart2" {
		t.Errorf("got %q, want %q", got, "part1\npart2")
	}
}

func TestExtractCodexMessageTextEmptyText(t *testing.T) {
	got := extractCodexMessageText(map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "input_text", "text": ""},
		},
	})
	if got != "" {
		t.Errorf("expected empty for empty text blocks, got %q", got)
	}
}

func TestIsCodexSystemMessage(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"# AGENTS.md\nsome content", true},
		{"<environment_context>\n<cwd>/home</cwd>", true},
		{"<permissions instructions>\nallow all", true},
		{"<INSTRUCTIONS>\ndo stuff", true},
		{"<collaboration_mode>\ncolab", true},
		{"<user_instructions>\ninstruct", true},
		{"Regular user message", false},
		{"", false},
		{"Some text with <environment_context> in middle", false},
	}

	for _, tt := range tests {
		got := isCodexSystemMessage(tt.text)
		if got != tt.want {
			t.Errorf("isCodexSystemMessage(%q) = %v, want %v", tt.text[:min(len(tt.text), 30)], got, tt.want)
		}
	}
}

func TestExtractCodexUUIDVariousFormats(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		// Valid UUID
		{"rollout-2026-03-01T10-00-00-019c9d89-ae90-75e2-9555-7855bac35c45.jsonl", "019c9d89-ae90-75e2-9555-7855bac35c45"},
		// Too short
		{"short.jsonl", ""},
		// Invalid UUID format (no hyphens at expected positions)
		{"rollout-2026-03-01T10-00-00-0000000000000000000000000000000000000.jsonl", ""},
		// Exactly 36 chars (just UUID)
		{"019c9d89-ae90-75e2-9555-7855bac35c45.jsonl", "019c9d89-ae90-75e2-9555-7855bac35c45"},
	}

	for _, tt := range tests {
		got := extractCodexUUID(tt.filename)
		if got != tt.want {
			t.Errorf("extractCodexUUID(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestReadCodexMessagesNonexistent(t *testing.T) {
	_, err := ReadCodexMessages("/nonexistent/path.jsonl", 0, 0, false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadCodexMessagesOffsetBeyondEnd(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	msgs, err := ReadCodexMessages(path, 100, 0, false)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil for offset beyond end, got %d messages", len(msgs))
	}
}

func TestReadCodexMessagesLimitOnly(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	msgs, err := ReadCodexMessages(path, 0, 1, false)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1 (limited)", len(msgs))
	}
}

func TestReadCodexMessagesFunctionCallWithoutAssistant(t *testing.T) {
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"test-id","cwd":"/tmp"}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Do stuff"}]}}
{"timestamp":"2026-03-01T10:00:02Z","type":"response_item","payload":{"type":"function_call","name":"exec","arguments":"{\"cmd\":\"ls\"}","call_id":"c1"}}
{"timestamp":"2026-03-01T10:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done."}]}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}

	// user, assistant (from function_call with no prior assistant), assistant("Done.")
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
	if msgs[1].Type != "assistant" {
		t.Errorf("msg[1] type = %q, want assistant", msgs[1].Type)
	}
	if len(msgs[1].ToolUses) != 1 {
		t.Errorf("msg[1] should have 1 tool use, got %d", len(msgs[1].ToolUses))
	}
}

func TestReadCodexMessagesFunctionCallNoName(t *testing.T) {
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"test-id","cwd":"/tmp"}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Do stuff"}]}}
{"timestamp":"2026-03-01T10:00:02Z","type":"response_item","payload":{"type":"function_call","name":"","arguments":"{}","call_id":"c1"}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}

	// function_call with empty name gets "unknown".
	found := false
	for _, m := range msgs {
		for _, tu := range m.ToolUses {
			if tu.Name == "unknown" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected tool use with name 'unknown' for empty function_call name")
	}
}

func TestReadCodexMessagesFunctionOutputNoAssistant(t *testing.T) {
	// function_call_output without a prior assistant message in full mode.
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"test-id","cwd":"/tmp"}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Do stuff"}]}}
{"timestamp":"2026-03-01T10:00:02Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"some output"}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	msgs, err := ReadCodexMessages(path, 0, 0, true)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}

	// user + assistant (created for orphan function_call_output)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[1].Type != "assistant" {
		t.Errorf("msg[1] type = %q, want assistant", msgs[1].Type)
	}
	if !contains(msgs[1].Content, "[result]") {
		t.Errorf("msg[1] should contain [result], got %q", msgs[1].Content)
	}
}

func TestReadCodexMessagesFunctionOutputEmptyOutput(t *testing.T) {
	// function_call_output with empty output should be skipped even in full mode.
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"test-id","cwd":"/tmp"}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Do stuff"}]}}
{"timestamp":"2026-03-01T10:00:02Z","type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":""}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	msgs, err := ReadCodexMessages(path, 0, 0, true)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}

	// Only the user message; empty output is skipped.
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
}

func TestReadCodexMessagesFullModeFunctionArgs(t *testing.T) {
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"test-id","cwd":"/tmp"}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Running command."}]}}
{"timestamp":"2026-03-01T10:00:02Z","type":"response_item","payload":{"type":"function_call","name":"exec","arguments":"{\"cmd\":\"ls -la\"}","call_id":"c1"}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	// Full mode should include arguments.
	msgs, err := ReadCodexMessages(path, 0, 0, true)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}

	found := false
	for _, m := range msgs {
		if contains(m.Content, `[tool: exec] {"cmd":"ls -la"}`) {
			found = true
		}
	}
	if !found {
		t.Error("expected function arguments in full mode")
	}
}

func TestSearchCodexSessionAssistantMode(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	matches := searchCodexSession(path, "bug", "assistant")
	if len(matches) == 0 {
		t.Fatal("expected at least one assistant match for 'bug'")
	}
	for _, m := range matches {
		if m.Type != "assistant" {
			t.Errorf("match type = %q, want assistant", m.Type)
		}
	}
}

func TestSearchCodexSessionNonexistent(t *testing.T) {
	matches := searchCodexSession("/nonexistent/file.jsonl", "query", "all")
	if len(matches) != 0 {
		t.Errorf("expected no matches for nonexistent file, got %d", len(matches))
	}
}

func TestSearchCodexSessionMatchIndex(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	matches := searchCodexSession(path, "thanks", "all")
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	// "Thanks!" is the last user message.
	if matches[0].Index < 1 {
		t.Errorf("match index = %d, expected > 0", matches[0].Index)
	}
}

func TestExtractCodexMetaFallbackSessionID(t *testing.T) {
	// Session with no header - should fall back to UUID from filename.
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "rollout-2026-03-01T10-00-00-019c9d89-ae90-75e2-9555-7855bac35c45.jsonl", data)

	meta, err := ExtractCodexMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if meta.SessionID != "019c9d89-ae90-75e2-9555-7855bac35c45" {
		t.Errorf("SessionID = %q, want UUID from filename", meta.SessionID)
	}
}

func TestExtractCodexMetaNonexistent(t *testing.T) {
	_, err := ExtractCodexMeta("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtractCodexMetaEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := ExtractCodexMeta(path)
	if err != nil {
		t.Fatalf("ExtractCodexMeta() error: %v", err)
	}
	if meta.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", meta.MessageCount)
	}
}

func TestExtractCodexMetaDeveloperSkipped(t *testing.T) {
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"system message"}]}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	meta, err := ExtractCodexMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if meta.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1 (developer skipped)", meta.MessageCount)
	}
}

func TestReadCodexMessagesDeveloperSkipped(t *testing.T) {
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"system message"}]}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (developer skipped)", len(msgs))
	}
	if msgs[0].Content != "Hello" {
		t.Errorf("msg[0].Content = %q, want Hello", msgs[0].Content)
	}
}

func TestReadCodexMessagesNonUserAssistantSkipped(t *testing.T) {
	// Messages with roles other than user/assistant/developer should be skipped.
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"response_item","payload":{"type":"message","role":"system","content":[{"type":"input_text","text":"system"}]}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello"}]}}
`
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", data)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadCodexMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (system skipped)", len(msgs))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMetaWithAssistantUsageTokens(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "usage-session.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Hi there",
				"usage": map[string]interface{}{
					"input_tokens":                float64(50),
					"output_tokens":               float64(30),
					"cache_read_input_tokens":     float64(10),
					"cache_creation_input_tokens": float64(5),
				},
			}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}

	if meta.Tokens.InputTokens != 50 {
		t.Errorf("InputTokens = %d, want 50", meta.Tokens.InputTokens)
	}
	if meta.Tokens.OutputTokens != 30 {
		t.Errorf("OutputTokens = %d, want 30", meta.Tokens.OutputTokens)
	}
	if meta.Tokens.CacheReadTokens != 10 {
		t.Errorf("CacheReadTokens = %d, want 10", meta.Tokens.CacheReadTokens)
	}
	if meta.Tokens.CacheCreationTokens != 5 {
		t.Errorf("CacheCreationTokens = %d, want 5", meta.Tokens.CacheCreationTokens)
	}
}

func TestExtractMetaCombinesResultAndUsageTokens(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "combined-tokens.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Q1"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "A1",
				"usage": map[string]interface{}{
					"input_tokens":  float64(100),
					"output_tokens": float64(50),
				},
			}},
		{"type": "result", "input_tokens": float64(200), "output_tokens": float64(75)},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}

	// Tokens from both assistant usage and result should be summed.
	if meta.Tokens.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", meta.Tokens.InputTokens)
	}
	if meta.Tokens.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want 125", meta.Tokens.OutputTokens)
	}
}

func TestExtractMetaSkipsNoMessageField(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "no-message.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Real message"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", meta.MessageCount)
	}
}

func TestExtractMetaSkipsEmptyContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty-content.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": ""}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Real message"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", meta.MessageCount)
	}
	if meta.FirstMessage != "Real message" {
		t.Errorf("FirstMessage = %q, want %q", meta.FirstMessage, "Real message")
	}
}

func TestExtractMetaNonexistentFile(t *testing.T) {
	_, err := ExtractMeta("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtractMetaMessageNotMap(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad-message.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": "not a map"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Good message"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", meta.MessageCount)
	}
}

func TestExtractContentToolResult(t *testing.T) {
	// tool_result is only shown in full mode.
	content := []interface{}{
		map[string]interface{}{"type": "tool_result", "content": "result output here"},
	}

	got := ExtractContent("user", content, false)
	if got != "" {
		t.Errorf("tool_result in non-full mode: got %q, want empty", got)
	}

	got = ExtractContent("user", content, true)
	want := "[result] result output here"
	if got != want {
		t.Errorf("tool_result in full mode: got %q, want %q", got, want)
	}
}

func TestExtractContentToolResultLong(t *testing.T) {
	// tool_result content is truncated to 500 chars in full mode.
	longContent := make([]byte, 600)
	for i := range longContent {
		longContent[i] = 'x'
	}

	content := []interface{}{
		map[string]interface{}{"type": "tool_result", "content": string(longContent)},
	}

	got := ExtractContent("user", content, true)
	// [result] prefix + 497 chars + "..." = truncated to 500 chars
	if len(got) > len("[result] ")+500 {
		t.Errorf("tool_result should be truncated, got length %d", len(got))
	}
}

func TestExtractContentToolUseNoName(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "tool_use", "input": map[string]interface{}{}},
	}

	got := ExtractContent("assistant", content, false)
	if got != "[tool: unknown]" {
		t.Errorf("got %q, want %q", got, "[tool: unknown]")
	}
}

func TestExtractContentNonArrayNonString(t *testing.T) {
	got := ExtractContent("user", 42, false)
	if got != "" {
		t.Errorf("non-array/string content: got %q, want empty", got)
	}

	got = ExtractContent("user", true, false)
	if got != "" {
		t.Errorf("bool content: got %q, want empty", got)
	}
}

func TestExtractContentSkipsNonMapBlocks(t *testing.T) {
	content := []interface{}{
		"just a string",
		map[string]interface{}{"type": "text", "text": "real text"},
	}

	got := ExtractContent("assistant", content, false)
	if got != "real text" {
		t.Errorf("got %q, want %q", got, "real text")
	}
}

func TestExtractContentEmptyTextBlock(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": ""},
		map[string]interface{}{"type": "text", "text": "non-empty"},
	}

	got := ExtractContent("assistant", content, false)
	if got != "non-empty" {
		t.Errorf("got %q, want %q", got, "non-empty")
	}
}

func TestExtractContentUnknownBlockType(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "unknown_type", "data": "ignored"},
		map[string]interface{}{"type": "text", "text": "kept"},
	}

	got := ExtractContent("assistant", content, false)
	if got != "kept" {
		t.Errorf("got %q, want %q", got, "kept")
	}
}

func TestExtractToolUses(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "Some text"},
		map[string]interface{}{"type": "tool_use", "name": "Read"},
		map[string]interface{}{"type": "tool_use", "name": "Bash"},
	}

	tools := extractToolUses(content)
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	if tools[0].Name != "Read" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "Read")
	}
	if tools[1].Name != "Bash" {
		t.Errorf("tools[1].Name = %q, want %q", tools[1].Name, "Bash")
	}
}

func TestExtractToolUsesNilContent(t *testing.T) {
	tools := extractToolUses(nil)
	if tools != nil {
		t.Errorf("expected nil, got %v", tools)
	}
}

func TestExtractToolUsesStringContent(t *testing.T) {
	tools := extractToolUses("just a string")
	if tools != nil {
		t.Errorf("expected nil for string content, got %v", tools)
	}
}

func TestExtractToolUsesEmptyName(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "tool_use", "name": ""},
		map[string]interface{}{"type": "tool_use", "name": "Valid"},
	}

	tools := extractToolUses(content)
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1 (empty name skipped)", len(tools))
	}
	if tools[0].Name != "Valid" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "Valid")
	}
}

func TestExtractToolUsesNonMapBlock(t *testing.T) {
	content := []interface{}{
		"not a map",
		map[string]interface{}{"type": "tool_use", "name": "Read"},
	}

	tools := extractToolUses(content)
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
}

func TestJsonInt64FromMapFloat64(t *testing.T) {
	m := map[string]interface{}{
		"count": float64(42),
	}
	got := jsonInt64FromMap(m, "count")
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
}

func TestJsonInt64FromMapInt64(t *testing.T) {
	m := map[string]interface{}{
		"count": int64(99),
	}
	got := jsonInt64FromMap(m, "count")
	if got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}

func TestJsonInt64FromMapJsonNumber(t *testing.T) {
	m := map[string]interface{}{
		"count": json.Number("12345"),
	}
	got := jsonInt64FromMap(m, "count")
	if got != 12345 {
		t.Errorf("got %d, want 12345", got)
	}
}

func TestJsonInt64FromMapMissingKey(t *testing.T) {
	m := map[string]interface{}{
		"other": float64(10),
	}
	got := jsonInt64FromMap(m, "count")
	if got != 0 {
		t.Errorf("got %d, want 0 for missing key", got)
	}
}

func TestJsonInt64FromMapUnsupportedType(t *testing.T) {
	m := map[string]interface{}{
		"count": "not a number",
	}
	got := jsonInt64FromMap(m, "count")
	if got != 0 {
		t.Errorf("got %d, want 0 for unsupported type", got)
	}
}

func TestJsonInt64(t *testing.T) {
	// jsonInt64 is a wrapper for jsonInt64FromMap.
	m := map[string]interface{}{
		"tokens": float64(500),
	}
	got := jsonInt64(m, "tokens")
	if got != 500 {
		t.Errorf("got %d, want 500", got)
	}
}

func TestExtractMetaFirstMessageFromAssistantOnly(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "assistant-only.jsonl")

	// Only assistant messages, no user message.
	lines := []map[string]interface{}{
		{"type": "assistant", "timestamp": "2026-01-15T10:00:00Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "I'm here"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}

	if meta.FirstMessage != "" {
		t.Errorf("FirstMessage = %q, want empty (no user message)", meta.FirstMessage)
	}
	if meta.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", meta.MessageCount)
	}
}

func TestExtractMetaDedupUUIDs(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "dedup.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "same-uuid",
			"message": map[string]interface{}{"role": "user", "content": "First"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "same-uuid",
			"message": map[string]interface{}{"role": "user", "content": "Duplicate"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:02Z", "uuid": "different",
			"message": map[string]interface{}{"role": "user", "content": "Second"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2 (one deduped)", meta.MessageCount)
	}
}

func TestExtractMetaNoTimestamp(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "no-ts.jsonl")

	lines := []map[string]interface{}{
		{"type": "user", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "No timestamp"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if !meta.CreatedAt.IsZero() {
		t.Errorf("CreatedAt should be zero when no timestamps, got %v", meta.CreatedAt)
	}
	if meta.DurationSecs != 0 {
		t.Errorf("DurationSecs should be 0 when no timestamps, got %f", meta.DurationSecs)
	}
}

func TestExtractMetaEmptyUUIDs(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "no-uuid.jsonl")

	// Messages with no UUID should not be deduped.
	lines := []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z",
			"message": map[string]interface{}{"role": "user", "content": "First"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z",
			"message": map[string]interface{}{"role": "user", "content": "Second"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", meta.MessageCount)
	}
}

func TestExtractMetaAssistantUsageNoUsageField(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "no-usage.jsonl")

	// Assistant message with message map but no usage field.
	lines := []map[string]interface{}{
		{"type": "assistant", "timestamp": "2026-01-15T10:00:00Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Response"}},
	}

	writeMockSession(t, filePath, lines)

	meta, err := ExtractMeta(filePath)
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}
	if meta.Tokens.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0 when no usage", meta.Tokens.InputTokens)
	}
}

func TestReadMessagesNonexistentFile(t *testing.T) {
	_, err := ReadMessages("/nonexistent/path/file.jsonl", 0, 0, false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadMessagesOffsetBeyondEnd(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Only message"}},
	})

	msgs, err := ReadMessages(filePath, 100, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil for offset beyond end, got %d messages", len(msgs))
	}
}

func TestReadMessagesFullMode(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Show me"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "text", "text": "Here's the file."},
				map[string]interface{}{"type": "tool_use", "name": "Read", "input": map[string]interface{}{"path": "/foo"}},
				map[string]interface{}{"type": "tool_result", "content": "file contents here"},
			}}},
	})

	// Non-full mode: tool_result should not appear.
	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if contains(msgs[1].Content, "[result]") {
		t.Error("tool_result should not appear in non-full mode")
	}

	// Full mode: tool_result should appear.
	msgs, err = ReadMessages(filePath, 0, 0, true)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if !contains(msgs[1].Content, "[result] file contents here") {
		t.Errorf("expected tool_result in full mode, got %q", msgs[1].Content)
	}
	if !contains(msgs[1].Content, `[tool: Read] {"path":"/foo"}`) {
		t.Errorf("expected full tool_use in full mode, got %q", msgs[1].Content)
	}
}

func TestReadMessagesToolUses(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Do something"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "tool_use", "name": "Bash"},
				map[string]interface{}{"type": "tool_use", "name": "Read"},
			}}},
	})

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs[1].ToolUses) != 2 {
		t.Fatalf("got %d tool uses, want 2", len(msgs[1].ToolUses))
	}
	if msgs[1].ToolUses[0].Name != "Bash" {
		t.Errorf("tool[0] = %q, want Bash", msgs[1].ToolUses[0].Name)
	}
	if msgs[1].ToolUses[1].Name != "Read" {
		t.Errorf("tool[1] = %q, want Read", msgs[1].ToolUses[1].Name)
	}
}

func TestReadMessagesNoToolUsesForUser(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs[0].ToolUses) != 0 {
		t.Errorf("user messages should have no tool uses, got %d", len(msgs[0].ToolUses))
	}
}

func TestReadMessagesNoTimestamp(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "No ts"}},
	})

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if !msgs[0].Timestamp.IsZero() {
		t.Errorf("expected zero timestamp, got %v", msgs[0].Timestamp)
	}
}

func TestReadMessagesSkipsNonUserAssistant(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "system", "timestamp": "2026-01-15T10:00:00Z",
			"message": map[string]interface{}{"role": "system", "content": "System prompt"}},
		{"type": "file-history-snapshot"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1 (system/other types skipped)", len(msgs))
	}
}

func TestReadMessagesNoMessageField(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Real"}},
	})

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1", len(msgs))
	}
}

func TestReadMessagesMessageNotMap(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": "not a map"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Real"}},
	})

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1", len(msgs))
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		count int
	}{
		{"empty", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines", "line1\nline2\n", 2},
		{"trailing content", "line1\nline2", 2},
		{"empty lines", "a\n\nb\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines([]byte(tt.data))
			if len(lines) != tt.count {
				t.Errorf("splitLines(%q) returned %d lines, want %d", tt.data, len(lines), tt.count)
			}
		})
	}
}

func TestReadLinesFiltersEmpty(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "lines.jsonl")

	content := `{"type":"user"}` + "\n\n" + `{"type":"assistant"}` + "\n\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLines(filePath)
	if err != nil {
		t.Fatalf("readLines() error: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2 (empty lines filtered)", len(lines))
	}
}

func TestReadLinesNonexistent(t *testing.T) {
	_, err := readLines("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

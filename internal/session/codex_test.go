package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const codexSessionData = `{"timestamp":"2026-03-01T10:00:00.000Z","type":"session_meta","payload":{"id":"019c9d89-ae90-75e2-9555-7855bac35c45","timestamp":"2026-03-01T10:00:00.000Z","cwd":"/home/user/myproject","originator":"codex_cli_rs","cli_version":"0.111.0","source":"cli","model_provider":"openai"}}
{"timestamp":"2026-03-01T10:00:01.000Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"system instructions"}]}}
{"timestamp":"2026-03-01T10:00:02.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Fix the bug in main.go"}]}}
{"timestamp":"2026-03-01T10:00:03.000Z","type":"event_msg","payload":{"type":"user_message"}}
{"timestamp":"2026-03-01T10:00:04.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":500,"cached_input_tokens":200,"output_tokens":100,"reasoning_output_tokens":30,"total_tokens":600}}}}
{"timestamp":"2026-03-01T10:00:05.000Z","type":"response_item","payload":{"type":"reasoning","summary":"thinking about the bug"}}
{"timestamp":"2026-03-01T10:00:06.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I'll look at main.go to find the bug."}]}}
{"timestamp":"2026-03-01T10:00:07.000Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"cat main.go\"}","call_id":"call_123"}}
{"timestamp":"2026-03-01T10:00:08.000Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call_123","output":"package main\nfunc main() {}"}}
{"timestamp":"2026-03-01T10:00:09.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"The bug is fixed now."}]}}
{"timestamp":"2026-03-01T10:00:09.500Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1200,"cached_input_tokens":800,"output_tokens":350,"reasoning_output_tokens":90,"total_tokens":1550}}}}
{"timestamp":"2026-03-01T10:00:10.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Thanks!"}]}}
`

func writeCodexSession(t *testing.T, dir, filename, data string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractCodexMeta(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "rollout-2026-03-01T10-00-00-019c9d89-ae90-75e2-9555-7855bac35c45.jsonl", codexSessionData)

	meta, err := ExtractCodexMeta(path)
	if err != nil {
		t.Fatal(err)
	}

	if meta.CLI != "codex" {
		t.Errorf("expected CLI=codex, got %q", meta.CLI)
	}
	if meta.SessionID != "019c9d89-ae90-75e2-9555-7855bac35c45" {
		t.Errorf("unexpected session ID: %s", meta.SessionID)
	}
	if meta.ProjectPath != "/home/user/myproject" {
		t.Errorf("unexpected project path: %s", meta.ProjectPath)
	}
	// 2 user + 2 assistant = 4 messages (developer is skipped)
	if meta.MessageCount != 4 {
		t.Errorf("expected 4 messages, got %d", meta.MessageCount)
	}
	if meta.FirstMessage != "Fix the bug in main.go" {
		t.Errorf("unexpected first message: %q", meta.FirstMessage)
	}
	if meta.DurationSecs <= 0 {
		t.Errorf("expected positive duration, got %f", meta.DurationSecs)
	}
	// Token counts should reflect the last cumulative token_count event.
	if meta.Tokens.InputTokens != 1200 {
		t.Errorf("expected 1200 input tokens, got %d", meta.Tokens.InputTokens)
	}
	if meta.Tokens.OutputTokens != 350 {
		t.Errorf("expected 350 output tokens, got %d", meta.Tokens.OutputTokens)
	}
	if meta.Tokens.CacheReadTokens != 800 {
		t.Errorf("expected 800 cache read tokens, got %d", meta.Tokens.CacheReadTokens)
	}
}

func TestReadCodexMessages(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	// Expected: user("Fix the bug"), assistant("I'll look" + [tool: exec_command]), assistant("The bug is fixed"), user("Thanks!")
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	if msgs[0].Type != "user" || msgs[0].Content != "Fix the bug in main.go" {
		t.Errorf("msg[0] unexpected: type=%s content=%q", msgs[0].Type, msgs[0].Content)
	}

	if msgs[1].Type != "assistant" {
		t.Errorf("msg[1] expected assistant, got %s", msgs[1].Type)
	}
	// Should contain tool summary.
	if len(msgs[1].ToolUses) != 1 || msgs[1].ToolUses[0].Name != "exec_command" {
		t.Errorf("msg[1] expected tool use exec_command, got %v", msgs[1].ToolUses)
	}

	if msgs[2].Type != "assistant" || msgs[2].Content != "The bug is fixed now." {
		t.Errorf("msg[2] unexpected: type=%s content=%q", msgs[2].Type, msgs[2].Content)
	}

	if msgs[3].Type != "user" || msgs[3].Content != "Thanks!" {
		t.Errorf("msg[3] unexpected: type=%s content=%q", msgs[3].Type, msgs[3].Content)
	}
}

func TestReadCodexMessages_Full(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	msgs, err := ReadCodexMessages(path, 0, 0, true)
	if err != nil {
		t.Fatal(err)
	}

	// In full mode, function_call_output should be included.
	found := false
	for _, m := range msgs {
		if m.Type == "assistant" && len(m.Content) > 0 {
			if contains(m.Content, "[result]") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected [result] in full mode output")
	}
}

func TestReadCodexMessages_Pagination(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	msgs, err := ReadCodexMessages(path, 1, 2, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages with offset=1 limit=2, got %d", len(msgs))
	}
	if msgs[0].Type != "assistant" {
		t.Errorf("expected first message to be assistant after offset, got %s", msgs[0].Type)
	}
}

func TestExtractCodexUUID(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"rollout-2026-03-01T10-00-00-019c9d89-ae90-75e2-9555-7855bac35c45.jsonl", "019c9d89-ae90-75e2-9555-7855bac35c45"},
		{"short.jsonl", ""},
		{"no-uuid-here.jsonl", ""},
	}

	for _, tt := range tests {
		got := extractCodexUUID(tt.filename)
		if got != tt.want {
			t.Errorf("extractCodexUUID(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestSearchCodexSession(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexSessionData)

	matches := searchCodexSession(path, "bug", "all")
	if len(matches) == 0 {
		t.Fatal("expected at least one match for 'bug'")
	}

	// Search user-only mode.
	userMatches := searchCodexSession(path, "bug", "user")
	if len(userMatches) == 0 {
		t.Fatal("expected user match for 'bug'")
	}

	// Search title mode.
	titleMatches := searchCodexSession(path, "bug", "title")
	if len(titleMatches) == 0 {
		t.Fatal("expected title match for 'bug'")
	}

	// No match.
	noMatches := searchCodexSession(path, "nonexistent", "all")
	if len(noMatches) != 0 {
		t.Errorf("expected no matches, got %d", len(noMatches))
	}
}

// Old format with <user_instructions> system message (pre-Oct 2025 pattern).
const codexOldUserInstructionsData = `{"id":"1b66bf05-f332-4f85-a233-c5c50bdac687","timestamp":"2025-10-01T10:52:16.000Z","instructions":null}
{"record_type":"state"}
{"type":"message","role":"user","content":[{"type":"input_text","text":"<user_instructions>\n\n# CLAUDE.md\n\nThis file provides guidance.\n</user_instructions>"}]}
{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n  <cwd>/home/user/mlproject</cwd>\n</environment_context>"}]}
{"record_type":"state"}
{"type":"message","role":"user","content":[{"type":"input_text","text":"Change load_model to use onnx"}]}
{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I'll update the model loading code."}]}
`

func TestExtractCodexMeta_OldUserInstructions(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "rollout-2025-10-01T10-52-16-1b66bf05-f332-4f85-a233-c5c50bdac687.jsonl", codexOldUserInstructionsData)

	meta, err := ExtractCodexMeta(path)
	if err != nil {
		t.Fatal(err)
	}

	if meta.ProjectPath != "/home/user/mlproject" {
		t.Errorf("expected project path /home/user/mlproject, got %q", meta.ProjectPath)
	}
	if meta.FirstMessage != "Change load_model to use onnx" {
		t.Errorf("expected real user prompt as first message, got %q", meta.FirstMessage)
	}
	// Only the real user message + assistant = 2 (system messages filtered)
	if meta.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", meta.MessageCount)
	}
}

func TestReadCodexMessages_OldUserInstructions(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexOldUserInstructionsData)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system filtered), got %d", len(msgs))
	}
	if msgs[0].Type != "user" || msgs[0].Content != "Change load_model to use onnx" {
		t.Errorf("msg[0] unexpected: type=%s content=%q", msgs[0].Type, msgs[0].Content)
	}
	if msgs[1].Type != "assistant" {
		t.Errorf("msg[1] expected assistant, got %s", msgs[1].Type)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Old-format Codex session data (pre-Jan 2026).
const codexOldSessionData = `{"id":"02f3baa6-e4d8-46ad-9590-6cd34fdd8ac1","timestamp":"2025-09-11T16:38:04.973Z","instructions":null,"git":{"commit_hash":"abc123","branch":"main"}}
{"record_type":"state"}
{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n  <cwd>/home/user/oldproject</cwd>\n  <approval_policy>on-request</approval_policy>\n</environment_context>"}]}
{"record_type":"state"}
{"type":"message","role":"user","content":[{"type":"input_text","text":"Change main.py to use tinybert model"}]}
{"record_type":"state"}
{"type":"reasoning"}
{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I'll update main.py for the tinybert model."}]}
{"type":"function_call","name":"shell","arguments":"{\"cmd\":\"cat main.py\"}","call_id":"call_abc"}
{"type":"function_call_output","call_id":"call_abc","output":"{\"output\":\"import torch\\nmodel = load('bert')\"}"}
{"record_type":"state"}
{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done, main.py now uses tinybert."}]}
{"type":"message","role":"user","content":[{"type":"input_text","text":"Great, thanks!"}]}
`

func TestExtractCodexMeta_OldFormat(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "rollout-2025-09-11T13-38-04-02f3baa6-e4d8-46ad-9590-6cd34fdd8ac1.jsonl", codexOldSessionData)

	meta, err := ExtractCodexMeta(path)
	if err != nil {
		t.Fatal(err)
	}

	if meta.CLI != "codex" {
		t.Errorf("expected CLI=codex, got %q", meta.CLI)
	}
	if meta.SessionID != "02f3baa6-e4d8-46ad-9590-6cd34fdd8ac1" {
		t.Errorf("unexpected session ID: %s", meta.SessionID)
	}
	if meta.ProjectPath != "/home/user/oldproject" {
		t.Errorf("unexpected project path: %q", meta.ProjectPath)
	}
	// user("Change main.py") + assistant("I'll update") + assistant("Done") + user("Great") = 4
	if meta.MessageCount != 4 {
		t.Errorf("expected 4 messages, got %d", meta.MessageCount)
	}
	if meta.FirstMessage != "Change main.py to use tinybert model" {
		t.Errorf("unexpected first message: %q", meta.FirstMessage)
	}
}

func TestReadCodexMessages_OldFormat(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexOldSessionData)

	msgs, err := ReadCodexMessages(path, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	// Expected: user("Change main.py"), assistant("I'll update" + [tool: shell]), assistant("Done"), user("Great")
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	if msgs[0].Type != "user" || msgs[0].Content != "Change main.py to use tinybert model" {
		t.Errorf("msg[0] unexpected: type=%s content=%q", msgs[0].Type, msgs[0].Content)
	}

	if msgs[1].Type != "assistant" {
		t.Errorf("msg[1] expected assistant, got %s", msgs[1].Type)
	}
	if len(msgs[1].ToolUses) != 1 || msgs[1].ToolUses[0].Name != "shell" {
		t.Errorf("msg[1] expected tool use 'shell', got %v", msgs[1].ToolUses)
	}

	if msgs[2].Type != "assistant" || !strings.Contains(msgs[2].Content, "tinybert") {
		t.Errorf("msg[2] unexpected: type=%s content=%q", msgs[2].Type, msgs[2].Content)
	}

	if msgs[3].Type != "user" || msgs[3].Content != "Great, thanks!" {
		t.Errorf("msg[3] unexpected: type=%s content=%q", msgs[3].Type, msgs[3].Content)
	}
}

func TestReadCodexMessages_OldFormat_Full(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexOldSessionData)

	msgs, err := ReadCodexMessages(path, 0, 0, true)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, m := range msgs {
		if strings.Contains(m.Content, "[result]") {
			found = true
		}
	}
	if !found {
		t.Error("expected [result] in full mode output for old format")
	}
}

func TestSearchCodexSession_OldFormat(t *testing.T) {
	tmp := t.TempDir()
	path := writeCodexSession(t, tmp, "session.jsonl", codexOldSessionData)

	matches := searchCodexSession(path, "tinybert", "all")
	if len(matches) == 0 {
		t.Fatal("expected at least one match for 'tinybert'")
	}

	// Title search should match the first real user message.
	titleMatches := searchCodexSession(path, "tinybert", "title")
	if len(titleMatches) == 0 {
		t.Fatal("expected title match for 'tinybert'")
	}
}

func TestPathContains(t *testing.T) {
	tests := []struct {
		parent, child string
		want          bool
	}{
		{"/work/foo", "/work/foo", true},
		{"/work/foo", "/work/foo/bar", true},
		{"/work/foo", "/work/foobar", false},
		{"/work/foo", "/work/foobar/baz", false},
		{"/work/foo", "/work/fo", false},
		{"/work/foo", "/other/path", false},
		{"/", "/anything", true},
		{"/work/foo/", "/work/foo/bar", true},  // trailing slash on parent
		{"/work/foo", "/work/foo/", true},       // trailing slash on child
	}
	for _, tt := range tests {
		got := PathContains(tt.parent, tt.child)
		if got != tt.want {
			t.Errorf("PathContains(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
		}
	}
}

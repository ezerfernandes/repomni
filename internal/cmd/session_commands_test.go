package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/session"
)

func TestRunSessionList_NoSessions(t *testing.T) {
	resetSessionCommandFlags(t)
	_, _ = setupSessionProject(t)

	out := captureStdout(t, func() {
		if err := runSessionList(sessionListCmd, nil); err != nil {
			t.Fatalf("runSessionList() error: %v", err)
		}
	})

	if !strings.Contains(out, "No sessions found.") {
		t.Fatalf("expected no sessions message, got %q", out)
	}
}

func TestRunSessionList_JSON(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "claude-list-1", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "List this session"}},
	})

	sessionCLIFilter = "claude"
	sessionListJSON = true
	sessionListLimit = 1

	out := captureStdout(t, func() {
		if err := runSessionList(sessionListCmd, nil); err != nil {
			t.Fatalf("runSessionList() error: %v", err)
		}
	})

	var metas []session.SessionMeta
	if err := json.Unmarshal([]byte(out), &metas); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, out)
	}
	if len(metas) != 1 {
		t.Fatalf("got %d sessions, want 1", len(metas))
	}
	if metas[0].SessionID != "claude-list-1" {
		t.Fatalf("SessionID = %q, want %q", metas[0].SessionID, "claude-list-1")
	}
}

func TestRunSessionShow_Human(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "show-human", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello there"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:01Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Hi back"}},
	})

	sessionCLIFilter = "claude"

	out := captureStdout(t, func() {
		if err := runSessionShow(sessionShowCmd, []string{"show-human"}); err != nil {
			t.Fatalf("runSessionShow() error: %v", err)
		}
	})

	if !strings.Contains(out, "Hello there") || !strings.Contains(out, "Hi back") {
		t.Fatalf("expected messages in output, got %q", out)
	}
}

func TestRunSessionShow_JSONFull(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "show-json", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Show tools"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:01Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "text", "text": "Reading file"},
				map[string]interface{}{"type": "tool_use", "name": "Read", "input": map[string]interface{}{"path": "/tmp/file"}},
				map[string]interface{}{"type": "tool_result", "content": "file contents"},
			}}},
	})

	sessionCLIFilter = "claude"
	sessionShowJSON = true
	sessionShowFull = true
	sessionShowOffset = 1
	sessionShowLimit = 1

	out := captureStdout(t, func() {
		if err := runSessionShow(sessionShowCmd, []string{"show-json"}); err != nil {
			t.Fatalf("runSessionShow() error: %v", err)
		}
	})

	var msgs []session.Message
	if err := json.Unmarshal([]byte(out), &msgs); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, out)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, `[tool: Read] {"path":"/tmp/file"}`) {
		t.Fatalf("expected tool_use details, got %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "[result] file contents") {
		t.Fatalf("expected tool_result in content, got %q", msgs[0].Content)
	}
}

func TestRunSessionSearch_InvalidMode(t *testing.T) {
	resetSessionCommandFlags(t)
	_, _ = setupSessionProject(t)

	sessionSearchMode = "bad-mode"
	err := runSessionSearch(sessionSearchCmd, []string{"query"})
	if err == nil || !strings.Contains(err.Error(), "invalid search mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestRunSessionSearch_EmptyAndJSON(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	emptyOut := captureStdout(t, func() {
		if err := runSessionSearch(sessionSearchCmd, []string{"docker"}); err != nil {
			t.Fatalf("runSessionSearch() error: %v", err)
		}
	})
	if !strings.Contains(emptyOut, "No matches found.") {
		t.Fatalf("expected no matches message, got %q", emptyOut)
	}

	writeClaudeSessionFixture(t, home, projectDir, "search-json", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Tell me about Docker"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:01Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Docker runs containers"}},
	})

	sessionCLIFilter = "claude"
	sessionSearchJSON = true
	sessionSearchMode = "assistant"
	sessionSearchLimit = 1

	jsonOut := captureStdout(t, func() {
		if err := runSessionSearch(sessionSearchCmd, []string{"docker"}); err != nil {
			t.Fatalf("runSessionSearch() error: %v", err)
		}
	})

	var results []session.SearchResult
	if err := json.Unmarshal([]byte(jsonOut), &results); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, jsonOut)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if len(results[0].Matches) != 1 || results[0].Matches[0].Type != "assistant" {
		t.Fatalf("unexpected matches: %+v", results[0].Matches)
	}
}

func TestRunSessionStats_HumanAndJSON(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "stats-1", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "One"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:10Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Two"}},
	})

	humanOut := captureStdout(t, func() {
		if err := runSessionStats(sessionStatsCmd, nil); err != nil {
			t.Fatalf("runSessionStats() error: %v", err)
		}
	})
	if !strings.Contains(humanOut, "Sessions:") {
		t.Fatalf("expected human stats output, got %q", humanOut)
	}

	sessionCLIFilter = "claude"
	sessionStatsJSON = true
	jsonOut := captureStdout(t, func() {
		if err := runSessionStats(sessionStatsCmd, nil); err != nil {
			t.Fatalf("runSessionStats() error: %v", err)
		}
	})

	var stats session.Stats
	if err := json.Unmarshal([]byte(jsonOut), &stats); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, jsonOut)
	}
	if stats.TotalSessions != 1 || stats.TotalMessages != 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRunSessionExport_JSONToFileAndStdout(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "export-1", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Question"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:01Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "tool_use", "name": "Bash", "input": map[string]interface{}{"cmd": "pwd"}},
			}}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:02Z", "uuid": "a2",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "text", "text": "Answer"},
				map[string]interface{}{"type": "tool_use", "name": "Read", "input": map[string]interface{}{"path": "/tmp/file"}},
			}}},
	})

	sessionCLIFilter = "claude"
	stdoutOut := captureStdout(t, func() {
		if err := runSessionExport(sessionExportCmd, []string{"export-1"}); err != nil {
			t.Fatalf("runSessionExport() error: %v", err)
		}
	})
	if !strings.Contains(stdoutOut, "# Session export-1") {
		t.Fatalf("expected markdown on stdout, got %q", stdoutOut)
	}

	outPath := filepath.Join(t.TempDir(), "session.md")
	sessionExportOutput = outPath
	sessionExportFull = true
	sessionExportNoTools = true
	sessionExportJSON = true

	jsonOut := captureStdout(t, func() {
		if err := runSessionExport(sessionExportCmd, []string{"export-1"}); err != nil {
			t.Fatalf("runSessionExport() error: %v", err)
		}
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, jsonOut)
	}
	if got := result["output_path"]; got != outPath {
		t.Fatalf("output_path = %v, want %q", got, outPath)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "[tool:") || strings.Contains(content, "[result]") {
		t.Fatalf("expected tool lines to be stripped, got %q", content)
	}
	if !strings.Contains(content, "Answer") {
		t.Fatalf("expected assistant text to remain, got %q", content)
	}
}

func TestRunSessionResume_ClaudeSuccessContinue(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "resume-claude", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Resume me"}},
	})

	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "claude-args.txt")
	writeExecutable(t, binDir, "claude", fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", argsFile))
	prependPath(t, binDir)

	sessionCLIFilter = "claude"
	sessionResumeContinue = true

	if err := runSessionResume(sessionResumeCmd, []string{"resume-claude"}); err != nil {
		t.Fatalf("runSessionResume() error: %v", err)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("os.ReadFile() error: %v", err)
	}
	args := string(argsData)
	if !strings.Contains(args, "--resume") || !strings.Contains(args, "resume-claude") || !strings.Contains(args, "--continue") {
		t.Fatalf("unexpected claude args: %q", args)
	}
}

func TestRunSessionResume_CodexSuccess(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	sessionID := "11111111-2222-3333-4444-555555555555"
	writeCodexSessionFixture(t, home, projectDir, sessionID, "")

	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "codex-args.txt")
	writeExecutable(t, binDir, "codex", fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", argsFile))
	prependPath(t, binDir)

	sessionCLIFilter = "codex"

	if err := runSessionResume(sessionResumeCmd, []string{sessionID}); err != nil {
		t.Fatalf("runSessionResume() error: %v", err)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("os.ReadFile() error: %v", err)
	}
	args := string(argsData)
	if !strings.Contains(args, "resume") || !strings.Contains(args, sessionID) {
		t.Fatalf("unexpected codex args: %q", args)
	}
}

func TestRunSessionResume_MissingBinary(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	writeClaudeSessionFixture(t, home, projectDir, "resume-missing", []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Resume me"}},
	})

	t.Setenv("PATH", t.TempDir())
	sessionCLIFilter = "claude"

	err := runSessionResume(sessionResumeCmd, []string{"resume-missing"})
	if err == nil || !strings.Contains(err.Error(), "claude not found in PATH") {
		t.Fatalf("expected missing claude error, got %v", err)
	}
}

func TestRunSessionClean_InvalidOlderThan(t *testing.T) {
	resetSessionCommandFlags(t)
	_, _ = setupSessionProject(t)

	sessionCleanOlderThan = "not-a-duration"
	err := runSessionClean(sessionCleanCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid --older-than value") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestRunSessionClean_JSONAndForceRemoval(t *testing.T) {
	resetSessionCommandFlags(t)
	home, projectDir := setupSessionProject(t)

	sessionID := "clean-1234"
	filePath := filepath.Join(home, ".claude", "projects", session.EncodePath(projectDir), sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(strings.TrimSuffix(filePath, ".jsonl"), 0755); err != nil {
		t.Fatal(err)
	}

	sessionCLIFilter = "claude"
	sessionCleanJSON = true
	jsonOut := captureStdout(t, func() {
		if err := runSessionClean(sessionCleanCmd, nil); err != nil {
			t.Fatalf("runSessionClean() error: %v", err)
		}
	})

	var candidates []cleanCandidate
	if err := json.Unmarshal([]byte(jsonOut), &candidates); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, jsonOut)
	}
	if len(candidates) != 1 || candidates[0].SessionID != sessionID {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}

	sessionCleanJSON = false
	sessionCleanForce = true
	out := captureStdout(t, func() {
		if err := runSessionClean(sessionCleanCmd, nil); err != nil {
			t.Fatalf("runSessionClean() error: %v", err)
		}
	})
	if !strings.Contains(out, "Removed 1 session file") {
		t.Fatalf("expected removal summary, got %q", out)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("expected session file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(strings.TrimSuffix(filePath, ".jsonl")); !os.IsNotExist(err) {
		t.Fatalf("expected session directory to be removed, stat err=%v", err)
	}
}

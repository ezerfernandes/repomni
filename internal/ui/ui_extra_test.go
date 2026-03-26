package ui

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ezerfernandes/repomni/internal/session"
)

// captureOutput captures stdout output during the execution of fn.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// --- PrintJSON ---

func TestPrintJSON_Simple(t *testing.T) {
	out := captureOutput(t, func() {
		err := PrintJSON(map[string]string{"key": "value"})
		if err != nil {
			t.Errorf("PrintJSON error: %v", err)
		}
	})

	if !strings.Contains(out, `"key": "value"`) {
		t.Errorf("expected JSON output with key/value, got %q", out)
	}
}

func TestPrintJSON_Struct(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	out := captureOutput(t, func() {
		err := PrintJSON(testStruct{Name: "test", Count: 42})
		if err != nil {
			t.Errorf("PrintJSON error: %v", err)
		}
	})

	if !strings.Contains(out, `"name": "test"`) {
		t.Errorf("expected name in output, got %q", out)
	}
	if !strings.Contains(out, `"count": 42`) {
		t.Errorf("expected count in output, got %q", out)
	}
}

func TestPrintJSON_Slice(t *testing.T) {
	out := captureOutput(t, func() {
		err := PrintJSON([]int{1, 2, 3})
		if err != nil {
			t.Errorf("PrintJSON error: %v", err)
		}
	})

	if !strings.Contains(out, "1") || !strings.Contains(out, "3") {
		t.Errorf("expected slice elements in output, got %q", out)
	}
}

func TestPrintJSON_Nil(t *testing.T) {
	out := captureOutput(t, func() {
		err := PrintJSON(nil)
		if err != nil {
			t.Errorf("PrintJSON error: %v", err)
		}
	})

	if strings.TrimSpace(out) != "null" {
		t.Errorf("expected null, got %q", out)
	}
}

// --- PrintSessionsList ---

func TestPrintSessionsList_Empty(t *testing.T) {
	out := captureOutput(t, func() {
		PrintSessionsList(nil)
	})

	if !strings.Contains(out, "No sessions found.") {
		t.Errorf("expected 'No sessions found.', got %q", out)
	}
}

func TestPrintSessionsList_Single(t *testing.T) {
	sessions := []session.SessionMeta{
		{
			SessionID:    "abc-123",
			CLI:          "claude",
			FirstMessage: "Hello world",
			MessageCount: 5,
			DurationSecs: 120,
			ModifiedAt:   time.Now().Add(-10 * time.Minute),
			Tokens: session.TokenUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
		},
	}

	out := captureOutput(t, func() {
		PrintSessionsList(sessions)
	})

	if !strings.Contains(out, "abc-123") {
		t.Error("expected session ID in output")
	}
	if !strings.Contains(out, "claude") {
		t.Error("expected CLI name in output")
	}
	if !strings.Contains(out, "Hello world") {
		t.Error("expected first message in output")
	}
}

func TestPrintSessionsList_Multiple(t *testing.T) {
	sessions := []session.SessionMeta{
		{SessionID: "session-1", CLI: "claude", FirstMessage: "First", MessageCount: 2, ModifiedAt: time.Now()},
		{SessionID: "session-2", CLI: "codex", FirstMessage: "Second", MessageCount: 3, ModifiedAt: time.Now()},
	}

	out := captureOutput(t, func() {
		PrintSessionsList(sessions)
	})

	if !strings.Contains(out, "session-1") {
		t.Error("expected session-1 in output")
	}
	if !strings.Contains(out, "session-2") {
		t.Error("expected session-2 in output")
	}
}

// --- PrintSessionMessages ---

func TestPrintSessionMessages_UserAndAssistant(t *testing.T) {
	meta := session.SessionMeta{
		SessionID:    "test-session",
		CLI:          "claude",
		ProjectPath:  "/test/project",
		CreatedAt:    time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		MessageCount: 2,
		DurationSecs: 60,
	}

	messages := []session.Message{
		{Type: "user", Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), Content: "Hello there"},
		{Type: "assistant", Timestamp: time.Date(2026, 1, 15, 10, 0, 5, 0, time.UTC), Content: "Hi, how can I help?"},
	}

	out := captureOutput(t, func() {
		PrintSessionMessages(&meta, messages, false)
	})

	if !strings.Contains(out, "test-session") {
		t.Error("expected session ID in output")
	}
	if !strings.Contains(out, "Hello there") {
		t.Error("expected user message in output")
	}
	if !strings.Contains(out, "Hi, how can I help?") {
		t.Error("expected assistant message in output")
	}
}

func TestPrintSessionMessages_UnknownType(t *testing.T) {
	meta := session.SessionMeta{
		SessionID: "test",
		CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	messages := []session.Message{
		{Type: "system", Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), Content: "System message"},
	}

	out := captureOutput(t, func() {
		PrintSessionMessages(&meta, messages, false)
	})

	if !strings.Contains(out, "system") {
		t.Error("expected unknown type label in output")
	}
}

func TestPrintSessionMessages_MultilineContent(t *testing.T) {
	meta := session.SessionMeta{
		SessionID: "test",
		CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	messages := []session.Message{
		{Type: "user", Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), Content: "line one\nline two\nline three"},
	}

	out := captureOutput(t, func() {
		PrintSessionMessages(&meta, messages, false)
	})

	if !strings.Contains(out, "line one") {
		t.Error("expected first line in output")
	}
	if !strings.Contains(out, "line three") {
		t.Error("expected last line in output")
	}
}

// --- PrintSessionStats ---

func TestPrintSessionStats_Basic(t *testing.T) {
	stats := session.Stats{
		TotalSessions:     5,
		TotalMessages:     50,
		TotalDurationSecs: 3600,
		TotalSizeBytes:    1024 * 1024,
		TotalTokens: session.TokenUsage{
			InputTokens:  10000,
			OutputTokens: 5000,
		},
		OldestSession: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NewestSession: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	out := captureOutput(t, func() {
		PrintSessionStats(&stats)
	})

	if !strings.Contains(out, "5") {
		t.Error("expected session count in output")
	}
	if !strings.Contains(out, "50") {
		t.Error("expected message count in output")
	}
	if !strings.Contains(out, "2025-01-01") {
		t.Error("expected oldest session date in output")
	}
	if !strings.Contains(out, "2026-01-01") {
		t.Error("expected newest session date in output")
	}
}

func TestPrintSessionStats_WithCache(t *testing.T) {
	stats := session.Stats{
		TotalSessions: 1,
		TotalTokens: session.TokenUsage{
			InputTokens:         100,
			OutputTokens:        50,
			CacheReadTokens:     200,
			CacheCreationTokens: 30,
		},
	}

	out := captureOutput(t, func() {
		PrintSessionStats(&stats)
	})

	if !strings.Contains(out, "200") {
		t.Error("expected cache read tokens in output")
	}
}

func TestPrintSessionStats_NoCache(t *testing.T) {
	stats := session.Stats{
		TotalSessions: 1,
		TotalTokens: session.TokenUsage{
			InputTokens:     100,
			OutputTokens:    50,
			CacheReadTokens: 0,
		},
	}

	out := captureOutput(t, func() {
		PrintSessionStats(&stats)
	})

	// "Cache:" label should not appear when cache tokens are 0.
	if strings.Contains(out, "Cache:") {
		t.Error("cache line should not appear when cache tokens are 0")
	}
}

func TestPrintSessionStats_ZeroDates(t *testing.T) {
	stats := session.Stats{
		TotalSessions: 1,
	}

	out := captureOutput(t, func() {
		PrintSessionStats(&stats)
	})

	// Oldest/Newest lines should not appear with zero times.
	if strings.Contains(out, "Oldest:") {
		t.Error("oldest line should not appear with zero time")
	}
	if strings.Contains(out, "Newest:") {
		t.Error("newest line should not appear with zero time")
	}
}

// --- PrintSearchResults ---

func TestPrintSearchResults_Empty(t *testing.T) {
	out := captureOutput(t, func() {
		PrintSearchResults(nil, "query")
	})

	if !strings.Contains(out, "No matches found.") {
		t.Errorf("expected 'No matches found.', got %q", out)
	}
}

func TestPrintSearchResults_WithMatches(t *testing.T) {
	results := []session.SearchResult{
		{
			Meta: session.SessionMeta{
				SessionID:    "abc-123-def-456",
				FirstMessage: "How do I use Docker?",
			},
			Matches: []session.Match{
				{Type: "user", Preview: "How do I use Docker?", Index: 0},
				{Type: "assistant", Preview: "Docker is a container platform.", Index: 1},
			},
		},
	}

	out := captureOutput(t, func() {
		PrintSearchResults(results, "docker")
	})

	if !strings.Contains(out, "abc-1") {
		t.Error("expected truncated session ID in output")
	}
	if !strings.Contains(out, "2 matches") {
		t.Error("expected match count in output")
	}
}

func TestPrintSearchResults_MultipleResults(t *testing.T) {
	results := []session.SearchResult{
		{
			Meta:    session.SessionMeta{SessionID: "session-1", FirstMessage: "First"},
			Matches: []session.Match{{Type: "user", Preview: "match one"}},
		},
		{
			Meta:    session.SessionMeta{SessionID: "session-2", FirstMessage: "Second"},
			Matches: []session.Match{{Type: "assistant", Preview: "match two"}},
		},
	}

	out := captureOutput(t, func() {
		PrintSearchResults(results, "match")
	})

	if strings.Count(out, "1 matches") != 2 {
		t.Errorf("expected each result to show '1 matches' in output twice, got %q", out)
	}
	// Both sessions should appear.
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Error("expected multiple lines of output for multiple results")
	}
}

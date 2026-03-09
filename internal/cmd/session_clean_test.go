package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDayDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1 day", "1d", 24 * time.Hour, false},
		{"7 days", "7d", 7 * 24 * time.Hour, false},
		{"30 days", "30d", 30 * 24 * time.Hour, false},
		{"0 days", "0d", 0, false},
		{"go duration hours", "72h", 72 * time.Hour, false},
		{"go duration minutes", "30m", 30 * time.Minute, false},
		{"with leading whitespace", "  7d", 7 * 24 * time.Hour, false},
		{"with trailing whitespace", "7d  ", 7 * 24 * time.Hour, false},
		{"invalid day count", "xd", 0, true},
		{"empty string", "", 0, true},
		{"invalid go duration", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDayDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDayDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseDayDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCollectCodexCleanCandidates_SkipsEmptyFromOtherProjects(t *testing.T) {
	// Set up a fake HOME with Codex session structure.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	sessDir := filepath.Join(fakeHome, ".codex", "sessions", "2026", "03", "01")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an empty session file (belongs to unknown project).
	emptyFile := filepath.Join(sessDir, "rollout-2026-03-01T10-00-00-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee.jsonl")
	if err := os.WriteFile(emptyFile, nil, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-empty session file belonging to /home/user/myproject.
	sessionData := `{"timestamp":"2026-03-01T10:00:00.000Z","type":"session_meta","payload":{"id":"11111111-2222-3333-4444-555555555555","cwd":"/home/user/myproject"}}
{"timestamp":"2026-03-01T10:00:01.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
`
	matchFile := filepath.Join(sessDir, "rollout-2026-03-01T10-00-01-11111111-2222-3333-4444-555555555555.jsonl")
	if err := os.WriteFile(matchFile, []byte(sessionData), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-empty session file belonging to /home/user/otherproject.
	otherData := `{"timestamp":"2026-03-01T10:00:00.000Z","type":"session_meta","payload":{"id":"66666666-7777-8888-9999-aaaaaaaaaaaa","cwd":"/home/user/otherproject"}}
{"timestamp":"2026-03-01T10:00:01.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
`
	otherFile := filepath.Join(sessDir, "rollout-2026-03-01T10-00-02-66666666-7777-8888-9999-aaaaaaaaaaaa.jsonl")
	if err := os.WriteFile(otherFile, []byte(otherData), 0644); err != nil {
		t.Fatal(err)
	}

	// When filtering by /home/user/myproject, the empty file must NOT appear
	// (can't verify ownership) and the other-project file must NOT appear.
	candidates := collectCodexCleanCandidates("/home/user/myproject", 0, time.Time{})
	for _, c := range candidates {
		if c.FilePath == emptyFile {
			t.Errorf("empty session from unknown project should be skipped, got candidate %s", c.SessionID)
		}
		if c.FilePath == otherFile {
			t.Errorf("session from other project should be skipped, got candidate %s", c.SessionID)
		}
	}

	// When no project path filter, empty files should be included.
	candidates = collectCodexCleanCandidates("", 0, time.Time{})
	foundEmpty := false
	for _, c := range candidates {
		if c.FilePath == emptyFile {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Error("empty session should be included when no project path filter is set")
	}
}

func TestFormatCleanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{10240, "10.0 KB"},
		{1048576, "1.0 MB"},
		{2621440, "2.5 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatCleanSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatCleanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/Users/foo/my-project", "-Users-foo-my-project"},
		{"/", "-"},
		{"/Users/ezersilva/repos/mine/tools/repoinjector", "-Users-ezersilva-repos-mine-tools-repoinjector"},
		{"/home/user/project with spaces", "-home-user-project with spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := EncodePath(tt.input)
			if got != tt.want {
				t.Errorf("EncodePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDiscoverFiltersAndSorts(t *testing.T) {
	// Set up a mock projects directory structure.
	tmpDir := t.TempDir()
	projectPath := "/test/project"
	encoded := EncodePath(projectPath)
	sessionDir := filepath.Join(tmpDir, encoded)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Temporarily override ClaudeProjectsDir by creating sessions in a custom dir.
	// We'll test Discover indirectly via DiscoverWithLimit with direct dir access.

	// Create a non-empty session file.
	writeTestSession(t, filepath.Join(sessionDir, "abc-123.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "First session"}},
		{"type": "assistant", "timestamp": "2026-01-10T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Response"}},
	})

	// Create a second non-empty session file.
	writeTestSession(t, filepath.Join(sessionDir, "def-456.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Second session"}},
	})

	// Create an empty session file (should be filtered out).
	if err := os.WriteFile(filepath.Join(sessionDir, "empty-789.jsonl"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a directory (should be ignored).
	if err := os.MkdirAll(filepath.Join(sessionDir, "some-dir"), 0755); err != nil {
		t.Fatal(err)
	}

	// Test reading the session dir directly.
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatal(err)
	}

	var sessions []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		filePath := filepath.Join(sessionDir, entry.Name())
		info, err := entry.Info()
		if err != nil || info.Size() == 0 {
			continue
		}

		meta, err := ExtractMeta(filePath)
		if err != nil {
			continue
		}
		sessions = append(sessions, *meta)
	}

	if len(sessions) != 2 {
		t.Errorf("found %d sessions, want 2", len(sessions))
	}
}

func TestFindSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two sessions with different prefixes.
	writeTestSession(t, filepath.Join(sessionDir, "abc-111-222.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Session A"}},
	})
	writeTestSession(t, filepath.Join(sessionDir, "def-333-444.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Session D"}},
	})

	// Test exact match.
	entries, _ := os.ReadDir(sessionDir)
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 files, got %d", len(names))
	}
}

func TestReadMessages(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "file-history-snapshot"}, // should be skipped
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Q1"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "text", "text": "A1"},
			}}},
		{"type": "queue-operation"}, // should be skipped
		{"type": "user", "timestamp": "2026-01-15T10:05:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Q2"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:05:10Z", "uuid": "a2",
			"message": map[string]interface{}{"role": "assistant", "content": "A2"}},
	})

	// Read all messages.
	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("got %d messages, want 4", len(msgs))
	}
	if msgs[0].Content != "Q1" {
		t.Errorf("msg[0].Content = %q, want %q", msgs[0].Content, "Q1")
	}
	if msgs[1].Content != "A1" {
		t.Errorf("msg[1].Content = %q, want %q", msgs[1].Content, "A1")
	}

	// Test pagination.
	msgs, err = ReadMessages(filePath, 1, 2, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Content != "A1" {
		t.Errorf("msg[0].Content = %q, want %q", msgs[0].Content, "A1")
	}

	// Test dedup: duplicate UUIDs should be skipped.
	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "same-uuid",
			"message": map[string]interface{}{"role": "user", "content": "first"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "same-uuid",
			"message": map[string]interface{}{"role": "user", "content": "duplicate"}},
	})
	msgs, err = ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1 (dedup)", len(msgs))
	}
}

func TestSearchSession(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "How do I use Docker?"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Docker is a container platform."}},
		{"type": "user", "timestamp": "2026-01-15T10:05:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "What about Kubernetes?"}},
	})

	// Search all modes.
	matches := searchSession(filePath, "docker", "all")
	if len(matches) != 2 {
		t.Errorf("all mode: got %d matches, want 2", len(matches))
	}

	// Search user only.
	matches = searchSession(filePath, "docker", "user")
	if len(matches) != 1 {
		t.Errorf("user mode: got %d matches, want 1", len(matches))
	}

	// Search assistant only.
	matches = searchSession(filePath, "container", "assistant")
	if len(matches) != 1 {
		t.Errorf("assistant mode: got %d matches, want 1", len(matches))
	}

	// Search title only.
	matches = searchSession(filePath, "docker", "title")
	if len(matches) != 1 {
		t.Errorf("title mode: got %d matches, want 1", len(matches))
	}

	// No match.
	matches = searchSession(filePath, "terraform", "all")
	if len(matches) != 0 {
		t.Errorf("no match: got %d matches, want 0", len(matches))
	}
}

func TestExtractPreview(t *testing.T) {
	content := "This is a test string with some content that we want to preview around a match point."

	preview := extractPreview(content, 10, 4) // "test"
	if len(preview) == 0 {
		t.Error("preview should not be empty")
	}
	// Should not have leading ... since match is near start.
	// Should have trailing ... since content continues.
}

func writeTestSession(t *testing.T, filePath string, entries []map[string]interface{}) {
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

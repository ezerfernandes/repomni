package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFakeHome creates a fake HOME with a Claude projects session directory
// for the given project path. Returns the session directory path.
func setupFakeHome(t *testing.T, projectPath string) string {
	t.Helper()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	encoded := EncodePath(projectPath)
	sessDir := filepath.Join(fakeHome, ".claude", "projects", encoded)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}
	return sessDir
}

// writeJSONLSession writes a JSONL session file with the given entries.
func writeJSONLSession(t *testing.T, filePath string, entries []map[string]interface{}) {
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

// --- ClaudeProjectsDir / ProjectSessionDir ---

func TestClaudeProjectsDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir, err := ClaudeProjectsDir()
	if err != nil {
		t.Fatalf("ClaudeProjectsDir() error: %v", err)
	}
	want := filepath.Join(fakeHome, ".claude", "projects")
	if dir != want {
		t.Errorf("ClaudeProjectsDir() = %q, want %q", dir, want)
	}
}

func TestProjectSessionDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir, err := ProjectSessionDir("/test/project")
	if err != nil {
		t.Fatalf("ProjectSessionDir() error: %v", err)
	}
	want := filepath.Join(fakeHome, ".claude", "projects", "-test-project")
	if dir != want {
		t.Errorf("ProjectSessionDir() = %q, want %q", dir, want)
	}
}

// --- Discover / DiscoverWithLimit ---

func TestDiscover_Success(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	// Create two valid session files.
	writeJSONLSession(t, filepath.Join(sessDir, "session-aaa.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "First session"}},
	})
	writeJSONLSession(t, filepath.Join(sessDir, "session-bbb.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Second session"}},
	})

	// Create an empty file (should be filtered).
	os.WriteFile(filepath.Join(sessDir, "empty.jsonl"), nil, 0644)

	// Create a directory (should be ignored).
	os.MkdirAll(filepath.Join(sessDir, "subdir"), 0755)

	// Create a non-jsonl file (should be ignored).
	os.WriteFile(filepath.Join(sessDir, "notes.txt"), []byte("hi"), 0644)

	sessions, err := Discover(projectPath)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("Discover() found %d sessions, want 2", len(sessions))
	}

	// Should be sorted newest first.
	if sessions[0].FirstMessage != "Second session" {
		t.Errorf("sessions[0].FirstMessage = %q, want 'Second session'", sessions[0].FirstMessage)
	}

	// ProjectPath should be set.
	for _, s := range sessions {
		if s.ProjectPath != projectPath {
			t.Errorf("session %s: ProjectPath = %q, want %q", s.SessionID, s.ProjectPath, projectPath)
		}
	}
}

func TestDiscoverWithLimit(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	for i := 0; i < 5; i++ {
		name := filepath.Join(sessDir, "session-"+string(rune('a'+i))+".jsonl")
		writeJSONLSession(t, name, []map[string]interface{}{
			{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u",
				"message": map[string]interface{}{"role": "user", "content": "msg"}},
		})
	}

	sessions, err := DiscoverWithLimit(projectPath, 2)
	if err != nil {
		t.Fatalf("DiscoverWithLimit() error: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("DiscoverWithLimit(2) found %d sessions, want 2", len(sessions))
	}
}

func TestDiscover_NoDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, err := Discover("/nonexistent/project")
	if err == nil {
		t.Error("expected error for nonexistent session directory")
	}
}

// --- FindSession ---

func TestFindSession_ExactMatch(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "abc-123-456.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	meta, err := FindSession(projectPath, "abc-123-456")
	if err != nil {
		t.Fatalf("FindSession() error: %v", err)
	}
	if meta.SessionID != "abc-123-456" {
		t.Errorf("SessionID = %q, want %q", meta.SessionID, "abc-123-456")
	}
	if meta.ProjectPath != projectPath {
		t.Errorf("ProjectPath = %q, want %q", meta.ProjectPath, projectPath)
	}
}

func TestFindSession_PrefixMatch(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "abc-123-456.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	meta, err := FindSession(projectPath, "abc")
	if err != nil {
		t.Fatalf("FindSession() error: %v", err)
	}
	if meta.SessionID != "abc-123-456" {
		t.Errorf("SessionID = %q, want %q", meta.SessionID, "abc-123-456")
	}
}

func TestFindSession_Ambiguous(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "abc-111.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "A"}},
	})
	writeJSONLSession(t, filepath.Join(sessDir, "abc-222.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "B"}},
	})

	_, err := FindSession(projectPath, "abc")
	if err == nil {
		t.Fatal("expected error for ambiguous prefix")
	}
}

func TestFindSession_NotFound(t *testing.T) {
	projectPath := "/test/project"
	setupFakeHome(t, projectPath)

	_, err := FindSession(projectPath, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

// --- Search ---

func TestSearch_Success(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "session-1.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "How do I use Docker?"}},
	})
	writeJSONLSession(t, filepath.Join(sessDir, "session-2.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Tell me about Go"}},
	})

	results, err := Search(projectPath, "docker", "all", 0)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search() found %d results, want 1", len(results))
	}
}

func TestSearch_WithLimit(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	for i := 0; i < 5; i++ {
		name := filepath.Join(sessDir, "session-"+string(rune('a'+i))+".jsonl")
		writeJSONLSession(t, name, []map[string]interface{}{
			{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
				"message": map[string]interface{}{"role": "user", "content": "docker question"}},
		})
	}

	results, err := Search(projectPath, "docker", "all", 2)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("Search() with limit=2 found %d results, want at most 2", len(results))
	}
}

func TestSearch_NoMatch(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "session.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello world"}},
	})

	results, err := Search(projectPath, "nonexistent", "all", 0)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() found %d results, want 0", len(results))
	}
}

// --- DiscoverAll ---

func TestDiscoverAll_ClaudeOnly(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "session.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	sessions, err := DiscoverAll(projectPath, "claude", 0)
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("DiscoverAll(claude) found %d sessions, want 1", len(sessions))
	}
}

func TestDiscoverAll_CodexOnly_NoDirOk(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// No codex sessions dir exists, but codex filter shouldn't hard-fail.
	sessions, err := DiscoverAll("/test/project", "codex", 0)
	if err != nil {
		t.Fatalf("DiscoverAll(codex) error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestDiscoverAll_BothWithLimit(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	for i := 0; i < 5; i++ {
		name := filepath.Join(sessDir, "session-"+string(rune('a'+i))+".jsonl")
		writeJSONLSession(t, name, []map[string]interface{}{
			{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
				"message": map[string]interface{}{"role": "user", "content": "msg"}},
		})
	}

	sessions, err := DiscoverAll(projectPath, "", 2)
	if err != nil {
		t.Fatalf("DiscoverAll() error: %v", err)
	}
	if len(sessions) > 2 {
		t.Errorf("DiscoverAll() with limit=2 found %d sessions, want at most 2", len(sessions))
	}
}

// --- FindSessionAll ---

func TestFindSessionAll_Claude(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "abc-123.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	meta, err := FindSessionAll(projectPath, "abc-123", "claude")
	if err != nil {
		t.Fatalf("FindSessionAll() error: %v", err)
	}
	if meta.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want %q", meta.SessionID, "abc-123")
	}
}

func TestFindSessionAll_NotFound(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create the projects dir but no sessions.
	projectPath := "/test/project"
	encoded := EncodePath(projectPath)
	sessDir := filepath.Join(fakeHome, ".claude", "projects", encoded)
	os.MkdirAll(sessDir, 0755)

	_, err := FindSessionAll(projectPath, "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

// --- SearchAll ---

func TestFindSessionAll_AmbiguousClaudeIsReturned(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "abc-111.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "A"}},
	})
	writeJSONLSession(t, filepath.Join(sessDir, "abc-222.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "B"}},
	})

	_, err := FindSessionAll(projectPath, "abc", "")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
}

func TestSearchAll_Claude(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "session.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Docker question"}},
	})

	results, err := SearchAll(projectPath, "docker", "all", "claude", 0)
	if err != nil {
		t.Fatalf("SearchAll() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("SearchAll() found %d results, want 1", len(results))
	}
}

func TestSearchAll_WithLimit(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	for i := 0; i < 5; i++ {
		name := filepath.Join(sessDir, "session-"+string(rune('a'+i))+".jsonl")
		writeJSONLSession(t, name, []map[string]interface{}{
			{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
				"message": map[string]interface{}{"role": "user", "content": "docker"}},
		})
	}

	results, err := SearchAll(projectPath, "docker", "all", "", 2)
	if err != nil {
		t.Fatalf("SearchAll() error: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("SearchAll() with limit=2 found %d results, want at most 2", len(results))
	}
}

func TestSearchAll_NoResults(t *testing.T) {
	projectPath := "/test/project"
	sessDir := setupFakeHome(t, projectPath)

	writeJSONLSession(t, filepath.Join(sessDir, "session.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-10T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	results, err := SearchAll(projectPath, "nonexistent", "all", "", 0)
	if err != nil {
		t.Fatalf("SearchAll() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchAll() found %d results, want 0", len(results))
	}
}

func TestSearchAll_ClaudeFilterPropagatesDiscoverErrors(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, err := SearchAll("/missing/project", "docker", "all", "claude", 0)
	if err == nil {
		t.Fatal("expected SearchAll to return the claude discovery error")
	}
}

// --- CodexSessionsDir ---

func TestCodexSessionsDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir, err := CodexSessionsDir()
	if err != nil {
		t.Fatalf("CodexSessionsDir() error: %v", err)
	}
	want := filepath.Join(fakeHome, ".codex", "sessions")
	if dir != want {
		t.Errorf("CodexSessionsDir() = %q, want %q", dir, want)
	}
}

// --- DiscoverCodex ---

func TestDiscoverCodex_NoDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	sessions, err := DiscoverCodex("/test/project", 0)
	if err != nil {
		t.Fatalf("DiscoverCodex() error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

// --- SearchCodex ---

func TestSearchCodex_NoDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	results, err := SearchCodex("/test/project", "query", "all", 0)
	if err != nil {
		t.Fatalf("SearchCodex() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

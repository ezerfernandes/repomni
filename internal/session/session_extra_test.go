package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchSessionDedup(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "same",
			"message": map[string]interface{}{"role": "user", "content": "Docker question"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "same",
			"message": map[string]interface{}{"role": "user", "content": "Docker duplicate"}},
	})

	matches := searchSession(filePath, "docker", "all")
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1 (duplicate UUID should be deduped)", len(matches))
	}
}

func TestSearchSessionNoUUID(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z",
			"message": map[string]interface{}{"role": "user", "content": "Docker first"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z",
			"message": map[string]interface{}{"role": "user", "content": "Docker second"}},
	})

	matches := searchSession(filePath, "docker", "all")
	if len(matches) != 2 {
		t.Errorf("got %d matches, want 2 (no UUID means no dedup)", len(matches))
	}
}

func TestSearchSessionNonexistentFile(t *testing.T) {
	matches := searchSession("/nonexistent/file.jsonl", "query", "all")
	if len(matches) != 0 {
		t.Errorf("expected no matches for nonexistent file, got %d", len(matches))
	}
}

func TestSearchSessionEmptyContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": ""}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Searchable content"}},
	})

	matches := searchSession(filePath, "searchable", "all")
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearchSessionNoMessageField(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Findable"}},
	})

	matches := searchSession(filePath, "findable", "all")
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearchSessionMessageNotMap(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": "not a map"},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Findable"}},
	})

	matches := searchSession(filePath, "findable", "all")
	if len(matches) != 1 {
		t.Errorf("got %d matches, want 1", len(matches))
	}
}

func TestSearchSessionTitleModeNoMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "First question"}},
		{"type": "user", "timestamp": "2026-01-15T10:00:01Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Docker question"}},
	})

	// "docker" is only in the second user message, not the title (first).
	matches := searchSession(filePath, "docker", "title")
	if len(matches) != 0 {
		t.Errorf("title mode: got %d matches, want 0 (docker not in first message)", len(matches))
	}
}

func TestSearchSessionAssistantMode(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Tell me about golang"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "Golang is great"}},
	})

	// "golang" is in both, but assistant mode should only match assistant.
	matches := searchSession(filePath, "golang", "assistant")
	if len(matches) != 1 {
		t.Fatalf("assistant mode: got %d matches, want 1", len(matches))
	}
	if matches[0].Type != "assistant" {
		t.Errorf("match type = %q, want assistant", matches[0].Type)
	}
}

func TestExtractPreviewAtStart(t *testing.T) {
	content := "Hello world, this is a test string."
	preview := extractPreview(content, 0, 5) // "Hello"
	if strings.HasPrefix(preview, "...") {
		t.Error("preview should not start with ... when match is at start")
	}
}

func TestExtractPreviewAtEnd(t *testing.T) {
	content := "Hello world"
	preview := extractPreview(content, 6, 5) // "world"
	if strings.HasSuffix(preview, "...") {
		t.Error("preview should not end with ... when match is at end")
	}
}

func TestExtractPreviewMiddle(t *testing.T) {
	// Create content long enough that the match is in the middle with truncation on both sides.
	content := strings.Repeat("a", 100) + "MATCH" + strings.Repeat("b", 100)
	preview := extractPreview(content, 100, 5)
	if !strings.HasPrefix(preview, "...") {
		t.Error("preview should start with ... when match is far from start")
	}
	if !strings.HasSuffix(preview, "...") {
		t.Error("preview should end with ... when content continues after match")
	}
	if !strings.Contains(preview, "MATCH") {
		t.Error("preview should contain the match")
	}
}

func TestExtractPreviewNewlines(t *testing.T) {
	content := "Line one\nLine two\nLine three"
	preview := extractPreview(content, 9, 4) // "Line" in "Line two"
	if strings.Contains(preview, "\n") {
		t.Error("preview should replace newlines with spaces")
	}
}

func TestExtractPreviewShortContent(t *testing.T) {
	content := "short"
	preview := extractPreview(content, 0, 5)
	if preview != "short" {
		t.Errorf("preview = %q, want %q", preview, "short")
	}
}

func TestDiscoverFiltersEmptyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an empty file.
	if err := os.WriteFile(filepath.Join(sessionDir, "empty.jsonl"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a valid session file.
	writeTestSession(t, filepath.Join(sessionDir, "valid.jsonl"), []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello"}},
	})

	// Create a non-jsonl file (should be ignored).
	if err := os.WriteFile(filepath.Join(sessionDir, "notes.txt"), []byte("not a session"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatal(err)
	}

	var sessions []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		meta, err := ExtractMeta(filepath.Join(sessionDir, entry.Name()))
		if err != nil {
			continue
		}
		sessions = append(sessions, *meta)
	}

	if len(sessions) != 1 {
		t.Errorf("found %d sessions, want 1 (empty and non-jsonl filtered)", len(sessions))
	}
}

func TestReadMessagesLimitOnly(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Q1"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "A1"}},
		{"type": "user", "timestamp": "2026-01-15T10:05:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Q2"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:05:05Z", "uuid": "a2",
			"message": map[string]interface{}{"role": "assistant", "content": "A2"}},
	})

	msgs, err := ReadMessages(filePath, 0, 2, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (limited)", len(msgs))
	}
	if msgs[0].Content != "Q1" {
		t.Errorf("first msg = %q, want Q1", msgs[0].Content)
	}
	if msgs[1].Content != "A1" {
		t.Errorf("second msg = %q, want A1", msgs[1].Content)
	}
}

func TestSearchSessionMatchIndex(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "First message"}},
		{"type": "assistant", "timestamp": "2026-01-15T10:00:05Z", "uuid": "a1",
			"message": map[string]interface{}{"role": "assistant", "content": "First response"}},
		{"type": "user", "timestamp": "2026-01-15T10:01:00Z", "uuid": "u2",
			"message": map[string]interface{}{"role": "user", "content": "Second message with target"}},
	})

	matches := searchSession(filePath, "target", "all")
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if matches[0].Index != 2 {
		t.Errorf("match index = %d, want 2", matches[0].Index)
	}
	if matches[0].Type != "user" {
		t.Errorf("match type = %q, want user", matches[0].Type)
	}
}

func TestSearchSessionCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Tell me about GoLang"}},
	})

	matches := searchSession(filePath, "golang", "all")
	if len(matches) != 1 {
		t.Errorf("case-insensitive search should match, got %d", len(matches))
	}
}

func TestReadMessagesForSessionClaude(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	writeTestSession(t, filePath, []map[string]interface{}{
		{"type": "user", "timestamp": "2026-01-15T10:00:00Z", "uuid": "u1",
			"message": map[string]interface{}{"role": "user", "content": "Hello Claude"}},
	})

	meta := &SessionMeta{CLI: "claude", FilePath: filePath}
	msgs, err := ReadMessagesForSession(meta, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessagesForSession() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Content != "Hello Claude" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "Hello Claude")
	}
}

func TestReadMessagesForSessionCodex(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "session.jsonl")
	data := `{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"test","cwd":"/tmp"}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello Codex"}]}}
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	meta := &SessionMeta{CLI: "codex", FilePath: path}
	msgs, err := ReadMessagesForSession(meta, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessagesForSession() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Content != "Hello Codex" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "Hello Codex")
	}
}

func TestReadMessagesEmptyFileReturnsNoError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("got %d messages, want 0", len(msgs))
	}
}

func TestReadMessagesMalformedLines(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	content := "not json\n" +
		`{"type":"user","timestamp":"2026-01-15T10:00:00Z","uuid":"u1","message":{"role":"user","content":"Hello"}}` + "\n" +
		"also bad\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	msgs, err := ReadMessages(filePath, 0, 0, false)
	if err != nil {
		t.Fatalf("ReadMessages() error: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1", len(msgs))
	}
}

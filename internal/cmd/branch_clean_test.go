package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repoinjector/internal/ui"
)

func TestDirSize(t *testing.T) {
	dir := t.TempDir()

	// Create files with known sizes.
	writeFile(t, filepath.Join(dir, "a.txt"), 100)
	writeFile(t, filepath.Join(dir, "sub", "b.txt"), 200)

	got := dirSize(dir)
	if got != 300 {
		t.Errorf("dirSize = %d, want 300", got)
	}
}

func TestDirSize_Empty(t *testing.T) {
	dir := t.TempDir()
	got := dirSize(dir)
	if got != 0 {
		t.Errorf("dirSize of empty dir = %d, want 0", got)
	}
}

func TestDirSize_NonExistent(t *testing.T) {
	got := dirSize("/nonexistent/path")
	if got != 0 {
		t.Errorf("dirSize of nonexistent path = %d, want 0", got)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCleanDanglingSymlinks(t *testing.T) {
	dir := t.TempDir()

	// Create a valid target and a symlink to it.
	validTarget := filepath.Join(dir, "target")
	if err := os.WriteFile(validTarget, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(validTarget, filepath.Join(dir, "valid-link")); err != nil {
		t.Fatal(err)
	}

	// Create a dangling symlink.
	if err := os.Symlink(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "broken-link")); err != nil {
		t.Fatal(err)
	}

	// Create a regular file (should be untouched).
	if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	cleaned := cleanDanglingSymlinks(dir)
	if cleaned != 1 {
		t.Errorf("cleanDanglingSymlinks = %d, want 1", cleaned)
	}

	// Valid symlink should still exist.
	if _, err := os.Lstat(filepath.Join(dir, "valid-link")); err != nil {
		t.Error("valid symlink was removed")
	}

	// Broken symlink should be gone.
	if _, err := os.Lstat(filepath.Join(dir, "broken-link")); err == nil {
		t.Error("broken symlink was not removed")
	}

	// Regular file should still exist.
	if _, err := os.Stat(filepath.Join(dir, "regular.txt")); err != nil {
		t.Error("regular file was removed")
	}
}

func TestCleanDanglingSymlinks_NoSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	cleaned := cleanDanglingSymlinks(dir)
	if cleaned != 0 {
		t.Errorf("cleanDanglingSymlinks = %d, want 0", cleaned)
	}
}

func TestArchiveBranches(t *testing.T) {
	dir := t.TempDir()

	candidates := []ui.CleanCandidate{
		{
			Info:      ui.BranchInfo{Name: "feat-a", State: "merged", Path: "/tmp/feat-a"},
			SizeHuman: "1.0 MB",
		},
		{
			Info:    ui.BranchInfo{Name: "feat-b", State: "closed", Path: "/tmp/feat-b"},
			Skipped: true,
			Reason:  "uncommitted changes",
		},
	}

	if err := archiveBranches(dir, candidates); err != nil {
		t.Fatalf("archiveBranches: %v", err)
	}

	archivePath := filepath.Join(dir, ".repoinjector-archive.json")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	var entries []archiveEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal archive: %v", err)
	}

	// Only non-skipped candidate should be archived.
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Info.Name != "feat-a" {
		t.Errorf("entry name = %q, want %q", entries[0].Info.Name, "feat-a")
	}
	if entries[0].ArchivedAt == "" {
		t.Error("ArchivedAt is empty")
	}
}

func TestArchiveBranches_Append(t *testing.T) {
	dir := t.TempDir()

	// Write an existing archive.
	existing := []archiveEntry{
		{ArchivedAt: "2025-01-01T00:00:00Z", Info: ui.BranchInfo{Name: "old-branch"}},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	archivePath := filepath.Join(dir, ".repoinjector-archive.json")
	if err := os.WriteFile(archivePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	candidates := []ui.CleanCandidate{
		{Info: ui.BranchInfo{Name: "new-branch", State: "merged"}},
	}

	if err := archiveBranches(dir, candidates); err != nil {
		t.Fatalf("archiveBranches: %v", err)
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}

	var entries []archiveEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Info.Name != "old-branch" {
		t.Errorf("first entry = %q, want %q", entries[0].Info.Name, "old-branch")
	}
	if entries[1].Info.Name != "new-branch" {
		t.Errorf("second entry = %q, want %q", entries[1].Info.Name, "new-branch")
	}
}

func TestPluralY(t *testing.T) {
	if got := pluralY(1); got != "y" {
		t.Errorf("pluralY(1) = %q, want %q", got, "y")
	}
	if got := pluralY(2); got != "ies" {
		t.Errorf("pluralY(2) = %q, want %q", got, "ies")
	}
	if got := pluralY(0); got != "ies" {
		t.Errorf("pluralY(0) = %q, want %q", got, "ies")
	}
}

func TestCountDeletable(t *testing.T) {
	candidates := []ui.CleanCandidate{
		{Skipped: false},
		{Skipped: true},
		{Skipped: false},
	}
	if got := countDeletable(candidates); got != 2 {
		t.Errorf("countDeletable = %d, want 2", got)
	}
}

// writeFile creates a file with the given size in bytes, creating parent dirs as needed.
func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0644); err != nil {
		t.Fatal(err)
	}
}

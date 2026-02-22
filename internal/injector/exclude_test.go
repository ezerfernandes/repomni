package injector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "info"), 0755)
	return dir
}

func TestUpdateExclude(t *testing.T) {
	gitDir := setupGitDir(t)
	paths := []string{".claude/skills", ".envrc", ".env"}

	if err := UpdateExclude(gitDir, paths); err != nil {
		t.Fatalf("UpdateExclude failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	if err != nil {
		t.Fatalf("cannot read exclude file: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, excludeMarkerStart) {
		t.Error("missing start marker")
	}
	if !strings.Contains(s, excludeMarkerEnd) {
		t.Error("missing end marker")
	}
	if !strings.Contains(s, ".claude/skills") {
		t.Error("missing .claude/skills path")
	}
	if !strings.Contains(s, ".envrc") {
		t.Error("missing .envrc path")
	}
}

func TestUpdateExcludeIdempotent(t *testing.T) {
	gitDir := setupGitDir(t)
	paths := []string{".envrc"}

	UpdateExclude(gitDir, paths)
	UpdateExclude(gitDir, paths)

	content, _ := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	count := strings.Count(string(content), excludeMarkerStart)
	if count != 1 {
		t.Errorf("expected 1 managed block, found %d", count)
	}
}

func TestUpdateExcludePreservesExisting(t *testing.T) {
	gitDir := setupGitDir(t)

	existing := "# existing patterns\n*.log\n"
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte(existing), 0644)

	UpdateExclude(gitDir, []string{".env"})

	content, _ := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	s := string(content)
	if !strings.Contains(s, "*.log") {
		t.Error("existing content was not preserved")
	}
	if !strings.Contains(s, ".env") {
		t.Error("new path was not added")
	}
}

func TestCleanExclude(t *testing.T) {
	gitDir := setupGitDir(t)

	existing := "# existing\n*.log\n"
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte(existing), 0644)

	UpdateExclude(gitDir, []string{".env"})
	CleanExclude(gitDir)

	content, _ := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	s := string(content)
	if strings.Contains(s, excludeMarkerStart) {
		t.Error("managed block was not removed")
	}
	if !strings.Contains(s, "*.log") {
		t.Error("existing content was removed")
	}
}

func TestGetExcludedPaths(t *testing.T) {
	gitDir := setupGitDir(t)
	UpdateExclude(gitDir, []string{".envrc", ".env"})

	paths := GetExcludedPaths(gitDir)
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

func TestHasManagedBlock(t *testing.T) {
	gitDir := setupGitDir(t)

	if HasManagedBlock(gitDir) {
		t.Error("should not have managed block initially")
	}

	UpdateExclude(gitDir, []string{".env"})

	if !HasManagedBlock(gitDir) {
		t.Error("should have managed block after update")
	}
}

func TestHasManagedBlock_NoFile(t *testing.T) {
	// Use a directory without an info/exclude file
	dir := t.TempDir()
	if HasManagedBlock(dir) {
		t.Error("should return false when exclude file does not exist")
	}
}

func TestHasManagedBlock_FileWithoutBlock(t *testing.T) {
	gitDir := setupGitDir(t)
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte("# just comments\n*.log\n"), 0644)

	if HasManagedBlock(gitDir) {
		t.Error("should return false when file exists but has no managed block")
	}
}

func TestHasManagedBlock_AfterClean(t *testing.T) {
	gitDir := setupGitDir(t)
	UpdateExclude(gitDir, []string{".env"})

	if !HasManagedBlock(gitDir) {
		t.Fatal("expected managed block after update")
	}

	CleanExclude(gitDir)

	if HasManagedBlock(gitDir) {
		t.Error("should not have managed block after clean")
	}
}

func TestCleanExclude_NoFile(t *testing.T) {
	dir := t.TempDir()
	// CleanExclude on non-existent file should not error
	if err := CleanExclude(dir); err != nil {
		t.Errorf("CleanExclude on non-existent file should not error: %v", err)
	}
}

func TestGetExcludedPaths_NoFile(t *testing.T) {
	dir := t.TempDir()
	paths := GetExcludedPaths(dir)
	if paths != nil {
		t.Errorf("expected nil paths for non-existent file, got %v", paths)
	}
}

func TestRemoveManagedBlock(t *testing.T) {
	content := "# existing\n*.log\n" + excludeMarkerStart + "\n.env\n.envrc\n" + excludeMarkerEnd + "\n# after\n"
	result := removeManagedBlock(content)

	if strings.Contains(result, excludeMarkerStart) {
		t.Error("should remove start marker")
	}
	if strings.Contains(result, excludeMarkerEnd) {
		t.Error("should remove end marker")
	}
	if strings.Contains(result, ".env") {
		t.Error("should remove managed paths")
	}
	if !strings.Contains(result, "*.log") {
		t.Error("should preserve existing content before block")
	}
	if !strings.Contains(result, "# after") {
		t.Error("should preserve content after block")
	}
}

func TestRemoveManagedBlock_NoBlock(t *testing.T) {
	content := "# just comments\n*.log\n"
	result := removeManagedBlock(content)
	if result != content {
		t.Errorf("expected content unchanged, got %q", result)
	}
}

func TestRemoveManagedBlock_EmptyInput(t *testing.T) {
	result := removeManagedBlock("")
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestUpdateExclude_MultiplePaths(t *testing.T) {
	gitDir := setupGitDir(t)
	paths := []string{".env", ".envrc", ".claude/skills/test.md", ".claude/hooks.json"}

	if err := UpdateExclude(gitDir, paths); err != nil {
		t.Fatalf("UpdateExclude failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	s := string(content)
	for _, p := range paths {
		if !strings.Contains(s, p) {
			t.Errorf("missing path %q in exclude file", p)
		}
	}
}

func TestUpdateExclude_UpdatesExistingBlock(t *testing.T) {
	gitDir := setupGitDir(t)

	// First update with one set of paths
	UpdateExclude(gitDir, []string{".env"})

	// Second update with different paths
	UpdateExclude(gitDir, []string{".envrc", ".claude/skills"})

	content, _ := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	s := string(content)

	// Should have new paths but NOT old ones
	if !strings.Contains(s, ".envrc") {
		t.Error("missing .envrc")
	}
	if !strings.Contains(s, ".claude/skills") {
		t.Error("missing .claude/skills")
	}
	// The old .env should be gone since the block was replaced
	// Count occurrences of markers to verify only one block
	count := strings.Count(s, excludeMarkerStart)
	if count != 1 {
		t.Errorf("expected 1 managed block, found %d", count)
	}
}

func TestGetExcludedPaths_MultiplePaths(t *testing.T) {
	gitDir := setupGitDir(t)
	UpdateExclude(gitDir, []string{".env", ".envrc", ".claude/skills"})

	paths := GetExcludedPaths(gitDir)
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d: %v", len(paths), paths)
	}
}

func TestGetExcludedPaths_EmptyBlock(t *testing.T) {
	gitDir := setupGitDir(t)
	content := excludeMarkerStart + "\n" + excludeMarkerEnd + "\n"
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte(content), 0644)

	paths := GetExcludedPaths(gitDir)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for empty block, got %d", len(paths))
	}
}

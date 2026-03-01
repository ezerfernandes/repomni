package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectBranchInfo(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	info := collectBranchInfo(repoDir)

	if info.Name != "my-repo" {
		t.Errorf("Name = %q, want %q", info.Name, "my-repo")
	}
	if info.Path != repoDir {
		t.Errorf("Path = %q, want %q", info.Path, repoDir)
	}
	if info.Branch == "" || info.Branch == "(detached)" {
		t.Errorf("Branch = %q, want a valid branch name", info.Branch)
	}
	if info.Dirty {
		t.Error("Dirty = true, want false for clean repo")
	}
	if info.State != "" {
		t.Errorf("State = %q, want empty for repo without config", info.State)
	}
}

func TestCollectBranchInfo_Dirty(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "dirty-repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Create and commit a file, then modify it to make the repo dirty.
	filePath := filepath.Join(repoDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repoDir, "add", "file.txt")
	runGitCmd(t, repoDir, "commit", "-m", "add file")
	if err := os.WriteFile(filePath, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	info := collectBranchInfo(repoDir)
	if !info.Dirty {
		t.Error("Dirty = false, want true for repo with modified tracked file")
	}
}

func TestCollectBranchInfo_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	plainDir := filepath.Join(dir, "not-a-repo")
	if err := os.Mkdir(plainDir, 0755); err != nil {
		t.Fatal(err)
	}

	info := collectBranchInfo(plainDir)

	if info.Name != "not-a-repo" {
		t.Errorf("Name = %q, want %q", info.Name, "not-a-repo")
	}
	if info.Branch != "(detached)" {
		t.Errorf("Branch = %q, want %q for non-git directory", info.Branch, "(detached)")
	}
}

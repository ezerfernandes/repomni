package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

func TestRunChecks_NoMergeURL(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Save config without MergeURL.
	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{Version: 1, State: "active"}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	// Change to the repo dir so the command can find the git repo.
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runChecks(checksCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no PR/MR attached; use \"branch submit\" or \"branch attach\" first"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunChecks_NoConfig(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runChecks(checksCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no PR/MR attached; use \"branch submit\" or \"branch attach\" first"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

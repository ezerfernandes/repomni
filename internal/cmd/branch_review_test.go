package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

func TestRunReview_NoFlags(t *testing.T) {
	origApprove, origComment := reviewApprove, reviewComment
	defer func() { reviewApprove, reviewComment = origApprove, origComment }()

	reviewApprove = false
	reviewComment = ""

	err := runReview(reviewCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "provide --approve and/or --comment"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunReview_NoMergeURL(t *testing.T) {
	origApprove, origComment := reviewApprove, reviewComment
	defer func() { reviewApprove, reviewComment = origApprove, origComment }()

	reviewApprove = true
	reviewComment = ""

	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{Version: 1, State: "active"}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runReview(reviewCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no PR/MR attached; use \"branch submit\" or \"branch attach\" first"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunReview_NoConfig(t *testing.T) {
	origApprove, origComment := reviewApprove, reviewComment
	defer func() { reviewApprove, reviewComment = origApprove, origComment }()

	reviewApprove = true
	reviewComment = ""

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

	err := runReview(reviewCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no PR/MR attached; use \"branch submit\" or \"branch attach\" first"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

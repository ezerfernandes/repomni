package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProjectPath_GitRepo(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-project")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	subDir := filepath.Join(repoDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	path, err := resolveProjectPath()
	if err != nil {
		t.Fatalf("resolveProjectPath() error: %v", err)
	}
	if path != repoDir {
		t.Errorf("resolveProjectPath() = %q, want %q", path, repoDir)
	}
}

func TestResolveProjectPath_AtRepoRoot(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	path, err := resolveProjectPath()
	if err != nil {
		t.Fatalf("resolveProjectPath() error: %v", err)
	}
	if path != repoDir {
		t.Errorf("resolveProjectPath() = %q, want %q", path, repoDir)
	}
}

func TestResolveProjectPath_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	path, err := resolveProjectPath()
	if err != nil {
		t.Fatalf("resolveProjectPath() error: %v", err)
	}
	if path != dir {
		t.Errorf("resolveProjectPath() = %q, want %q", path, dir)
	}
}

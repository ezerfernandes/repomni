package brancher

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ezer/repoinjector/internal/gitutil"
)

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// initBareCloneEnv creates a bare repo and a clone of it with an initial commit.
func initBareCloneEnv(t *testing.T) (bareDir, cloneDir string) {
	t.Helper()

	bareDir = filepath.Join(t.TempDir(), "origin.git")
	run(t, "", "git", "init", "--bare", bareDir)

	cloneDir = filepath.Join(t.TempDir(), "parent")
	run(t, "", "git", "clone", bareDir, cloneDir)

	writeFile(t, filepath.Join(cloneDir, "README.md"), "init")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "initial commit")
	run(t, cloneDir, "git", "push")

	return bareDir, cloneDir
}

func TestValidateBranchName(t *testing.T) {
	valid := []string{
		"my-feature",
		"fix-123",
		"UPPERCASE",
		"with.dot",
		"numbers123",
		"a",
		"kebab-case-name",
		"under_score",
	}
	for _, name := range valid {
		if err := ValidateBranchName(name); err != nil {
			t.Errorf("ValidateBranchName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateBranchNameInvalid(t *testing.T) {
	cases := []struct {
		name string
		desc string
	}{
		{"", "empty"},
		{".", "dot"},
		{"..", "double dot"},
		{"@", "at sign"},
		{"has/slash", "contains slash"},
		{"has\\backslash", "contains backslash"},
		{".starts-with-dot", "starts with dot"},
		{"ends-with-dot.", "ends with dot"},
		{"has..double-dots", "contains double dots"},
		{"ends.lock", "ends with .lock"},
		{"has space", "contains space"},
		{"has~tilde", "contains tilde"},
		{"has^caret", "contains caret"},
		{"has:colon", "contains colon"},
		{"has?question", "contains question mark"},
		{"has*star", "contains star"},
		{"has[bracket", "contains bracket"},
		{"has@{at-brace", "contains @{"},
		{string([]byte{0x01}), "control character"},
	}
	for _, tc := range cases {
		if err := ValidateBranchName(tc.name); err == nil {
			t.Errorf("ValidateBranchName(%q) [%s] = nil, want error", tc.name, tc.desc)
		}
	}
}

func TestFindParentGitRepo(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	subDir := filepath.Join(cloneDir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	repo, err := FindParentGitRepo(subDir)
	if err != nil {
		t.Fatalf("FindParentGitRepo failed: %v", err)
	}
	if repo != cloneDir {
		t.Errorf("expected %q, got %q", cloneDir, repo)
	}
}

func TestFindParentGitRepoFromRepo(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	repo, err := FindParentGitRepo(cloneDir)
	if err != nil {
		t.Fatalf("FindParentGitRepo failed: %v", err)
	}
	if repo != cloneDir {
		t.Errorf("expected %q, got %q", cloneDir, repo)
	}
}

func TestFindParentGitRepoNotFound(t *testing.T) {
	// t.TempDir() is in /tmp which should not be inside a git repo.
	dir := t.TempDir()
	_, err := FindParentGitRepo(dir)
	if err == nil {
		t.Error("expected error when no git repo in parents")
	}
}

func TestBranch(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Create a working directory inside the clone (simulates tmp/branches/).
	workDir := filepath.Join(cloneDir, "workdir")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := Branch(workDir, "my-feature")
	if err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	targetDir := filepath.Join(workDir, "my-feature")

	// Verify result fields.
	if result.ParentRepo != cloneDir {
		t.Errorf("ParentRepo = %q, want %q", result.ParentRepo, cloneDir)
	}
	if result.RemoteURL != bareDir {
		t.Errorf("RemoteURL = %q, want %q", result.RemoteURL, bareDir)
	}
	if result.TargetDir != targetDir {
		t.Errorf("TargetDir = %q, want %q", result.TargetDir, targetDir)
	}
	if result.Branch != "my-feature" {
		t.Errorf("Branch = %q, want %q", result.Branch, "my-feature")
	}

	// Verify the directory was created and is a git repo.
	if !gitutil.IsGitRepo(targetDir) {
		t.Fatal("target is not a git repository")
	}

	// Verify the correct branch was checked out.
	branch, err := gitutil.CurrentBranch(targetDir)
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}
	if branch != "my-feature" {
		t.Errorf("current branch = %q, want %q", branch, "my-feature")
	}
}

func TestBranchInvalidName(t *testing.T) {
	_, err := Branch(".", "has space")
	if err == nil {
		t.Error("expected error for invalid branch name")
	}
}

func TestBranchDirectoryExists(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	workDir := filepath.Join(cloneDir, "workdir")
	existing := filepath.Join(workDir, "existing")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Branch(workDir, "existing")
	if err == nil {
		t.Error("expected error when directory already exists")
	}
}

func TestBranchNoParentRepo(t *testing.T) {
	workDir := t.TempDir()
	_, err := Branch(workDir, "my-feature")
	if err == nil {
		t.Error("expected error when no parent git repo exists")
	}
}

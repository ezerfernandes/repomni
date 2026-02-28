package brancher

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
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

func TestSanitizeBranchName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"feature/my-thing", "feature-my-thing"},
		{"fix/bug/deep", "fix-bug-deep"},
		{"simple", "simple"},
		{"has:colon", "has-colon"},
		{"has space", "has-space"},
		{"combo/with:stuff", "combo-with-stuff"},
		{"back\\slash", "back-slash"},
		{"question?mark", "question-mark"},
		{"star*name", "star-name"},
		{"bracket[name", "bracket-name"},
		{"tilde~name", "tilde-name"},
		{"caret^name", "caret-name"},
	}
	for _, tc := range cases {
		got := SanitizeBranchName(tc.input)
		if got != tc.want {
			t.Errorf("SanitizeBranchName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClone(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Create a remote branch by pushing from the clone.
	run(t, cloneDir, "git", "checkout", "-b", "feature/my-thing")
	writeFile(t, filepath.Join(cloneDir, "feature.txt"), "feature work")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "feature commit")
	run(t, cloneDir, "git", "push", "-u", "origin", "feature/my-thing")
	// Switch back so the clone doesn't interfere.
	run(t, cloneDir, "git", "checkout", "master")

	// Create a working directory inside the clone.
	workDir := filepath.Join(cloneDir, "workdir")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := Clone(workDir, "feature/my-thing")
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	expectedDir := filepath.Join(workDir, "feature-my-thing")

	if result.ParentRepo != cloneDir {
		t.Errorf("ParentRepo = %q, want %q", result.ParentRepo, cloneDir)
	}
	if result.RemoteURL != bareDir {
		t.Errorf("RemoteURL = %q, want %q", result.RemoteURL, bareDir)
	}
	if result.TargetDir != expectedDir {
		t.Errorf("TargetDir = %q, want %q", result.TargetDir, expectedDir)
	}
	if result.Branch != "feature/my-thing" {
		t.Errorf("Branch = %q, want %q", result.Branch, "feature/my-thing")
	}

	// Verify the directory was created and is a git repo.
	if !gitutil.IsGitRepo(expectedDir) {
		t.Fatal("target is not a git repository")
	}

	// Verify the correct branch was checked out.
	branch, err := gitutil.CurrentBranch(expectedDir)
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}
	if branch != "feature/my-thing" {
		t.Errorf("current branch = %q, want %q", branch, "feature/my-thing")
	}
}

func TestCloneDirectoryExists(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	workDir := filepath.Join(cloneDir, "workdir")
	existing := filepath.Join(workDir, "feature-existing")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Clone(workDir, "feature/existing")
	if err == nil {
		t.Error("expected error when directory already exists")
	}
}

func TestCloneEmptyName(t *testing.T) {
	_, err := Clone(".", "")
	if err == nil {
		t.Error("expected error for empty branch name")
	}
}

func TestCloneNoParentRepo(t *testing.T) {
	workDir := t.TempDir()
	_, err := Clone(workDir, "feature/something")
	if err == nil {
		t.Error("expected error when no parent git repo exists")
	}
}

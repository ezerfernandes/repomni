package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
}

// initBareCloneEnv creates a bare repo with an initial commit and a clone of it.
// Returns (bareDir, cloneDir). The clone tracks origin/main (or origin/master).
func initBareCloneEnv(t *testing.T) (string, string) {
	t.Helper()

	bareDir := filepath.Join(t.TempDir(), "bare.git")
	run(t, "", "git", "init", "--bare", bareDir)

	cloneDir := filepath.Join(t.TempDir(), "clone")
	run(t, "", "git", "clone", bareDir, cloneDir)

	// Create initial commit and push
	writeFile(t, filepath.Join(cloneDir, "README.md"), "init")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "initial commit")
	run(t, cloneDir, "git", "push")

	return bareDir, cloneDir
}

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

func TestFindGitDir(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	gitDir, err := FindGitDir(repo)
	if err != nil {
		t.Fatalf("FindGitDir failed: %v", err)
	}

	expected := filepath.Join(repo, ".git")
	if gitDir != expected {
		t.Errorf("expected %q, got %q", expected, gitDir)
	}
}

func TestFindGitDirNotARepo(t *testing.T) {
	dir := t.TempDir()
	_, err := FindGitDir(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestIsGitRepo(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	if !IsGitRepo(repo) {
		t.Error("expected true for git repo")
	}

	notRepo := t.TempDir()
	if IsGitRepo(notRepo) {
		t.Error("expected false for non-git directory")
	}
}

func TestFindGitRepos(t *testing.T) {
	parent := t.TempDir()

	// Create 2 git repos and 1 regular dir
	initGitRepo(t, filepath.Join(parent, "repo-a"))
	initGitRepo(t, filepath.Join(parent, "repo-b"))
	if err := os.MkdirAll(filepath.Join(parent, "not-a-repo"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	repos, err := FindGitRepos(parent)
	if err != nil {
		t.Fatalf("FindGitRepos failed: %v", err)
	}

	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d: %v", len(repos), repos)
	}
}

func TestFindGitDirWorktree(t *testing.T) {
	// Create a main repo
	mainRepo := t.TempDir()
	initGitRepo(t, mainRepo)

	// Create an initial commit so we can create worktrees
	cmd := exec.Command("git", "-C", mainRepo, "commit", "--allow-empty", "-m", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create a worktree
	wtDir := filepath.Join(t.TempDir(), "worktree")
	cmd = exec.Command("git", "-C", mainRepo, "worktree", "add", wtDir, "-b", "test-branch")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git worktree add failed: %v", err)
	}

	gitDir, err := FindGitDir(wtDir)
	if err != nil {
		t.Fatalf("FindGitDir on worktree failed: %v", err)
	}

	// gitDir should point inside the main repo's .git/worktrees/
	if gitDir == filepath.Join(wtDir, ".git") {
		t.Errorf("gitDir should not be the worktree .git file, got %q", gitDir)
	}
}

func TestRunGit(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	out, err := RunGit(repo, "status", "--porcelain")
	if err != nil {
		t.Fatalf("RunGit failed: %v", err)
	}
	// Empty repo, no changes
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestRunGitError(t *testing.T) {
	dir := t.TempDir()
	_, err := RunGit(dir, "log")
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestCurrentBranch(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	branch, err := CurrentBranch(cloneDir)
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
}

func TestCurrentBranchDetached(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	run(t, cloneDir, "git", "checkout", "--detach")

	branch, err := CurrentBranch(cloneDir)
	if err != nil {
		t.Fatalf("CurrentBranch failed: %v", err)
	}
	if branch != "" {
		t.Errorf("expected empty branch for detached HEAD, got %q", branch)
	}
}

func TestUpstreamRef(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	upstream, err := UpstreamRef(cloneDir)
	if err != nil {
		t.Fatalf("UpstreamRef failed: %v", err)
	}
	if upstream == "" {
		t.Fatal("expected non-empty upstream ref")
	}
}

func TestUpstreamRefNone(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	// Create a commit so HEAD exists
	writeFile(t, filepath.Join(repo, "file.txt"), "content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "init")

	upstream, err := UpstreamRef(repo)
	if err != nil {
		t.Fatalf("UpstreamRef failed: %v", err)
	}
	if upstream != "" {
		t.Errorf("expected empty upstream, got %q", upstream)
	}
}

func TestAheadBehind(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	ahead, behind, err := AheadBehind(cloneDir)
	if err != nil {
		t.Fatalf("AheadBehind failed: %v", err)
	}
	if ahead != 0 || behind != 0 {
		t.Errorf("expected 0/0, got %d/%d", ahead, behind)
	}
}

func TestAheadBehindWithCommits(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone to make the first clone behind
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "new.txt"), "content")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "new commit")
	run(t, clone2, "git", "push")

	// Fetch in original clone
	run(t, cloneDir, "git", "fetch")

	ahead, behind, err := AheadBehind(cloneDir)
	if err != nil {
		t.Fatalf("AheadBehind failed: %v", err)
	}
	if ahead != 0 {
		t.Errorf("expected 0 ahead, got %d", ahead)
	}
	if behind != 1 {
		t.Errorf("expected 1 behind, got %d", behind)
	}
}

func TestIsDirty(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	dirty, err := IsDirty(cloneDir)
	if err != nil {
		t.Fatalf("IsDirty failed: %v", err)
	}
	if dirty {
		t.Error("expected clean repo")
	}

	// Make it dirty
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	dirty, err = IsDirty(cloneDir)
	if err != nil {
		t.Fatalf("IsDirty failed: %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo")
	}
}

func TestIsDirtyStagedOnly(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	writeFile(t, filepath.Join(cloneDir, "staged.txt"), "staged content")
	run(t, cloneDir, "git", "add", "staged.txt")

	dirty, err := IsDirty(cloneDir)
	if err != nil {
		t.Fatalf("IsDirty failed: %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo with staged changes")
	}
}

func TestIsDirtyError(t *testing.T) {
	dir := t.TempDir() // not a git repo
	_, err := IsDirty(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestFindGitDirUnexpectedFormat(t *testing.T) {
	dir := t.TempDir()
	// Write a .git file with unexpected content (not "gitdir: ...")
	writeFile(t, filepath.Join(dir, ".git"), "this is not a valid gitdir pointer")

	_, err := FindGitDir(dir)
	if err == nil {
		t.Error("expected error for unexpected .git file format")
	}
}

func TestFindGitReposInvalidDir(t *testing.T) {
	_, err := FindGitRepos("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestFindGitReposEmpty(t *testing.T) {
	parent := t.TempDir()
	repos, err := FindGitRepos(parent)
	if err != nil {
		t.Fatalf("FindGitRepos failed: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestFindGitReposSkipsFiles(t *testing.T) {
	parent := t.TempDir()
	// Create a file (not a directory) at top level
	writeFile(t, filepath.Join(parent, "not-a-dir.txt"), "content")
	initGitRepo(t, filepath.Join(parent, "real-repo"))

	repos, err := FindGitRepos(parent)
	if err != nil {
		t.Fatalf("FindGitRepos failed: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d: %v", len(repos), repos)
	}
}

func TestAheadBehindAhead(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	// Make a local commit without pushing
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local only")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local commit")

	ahead, behind, err := AheadBehind(cloneDir)
	if err != nil {
		t.Fatalf("AheadBehind failed: %v", err)
	}
	if ahead != 1 {
		t.Errorf("expected 1 ahead, got %d", ahead)
	}
	if behind != 0 {
		t.Errorf("expected 0 behind, got %d", behind)
	}
}

func TestAheadBehindNoUpstream(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, "file.txt"), "content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "init")

	_, _, err := AheadBehind(repo)
	if err == nil {
		t.Error("expected error for repo without upstream")
	}
}

func TestFetch(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	err := Fetch(cloneDir, false)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
}

func TestFetchNoRemote(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, "file.txt"), "content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "init")

	err := Fetch(repo, false)
	if err != nil {
		// git fetch on a repo with no remotes exits 0 but produces no output
		t.Fatalf("Fetch on repo without remote failed unexpectedly: %v", err)
	}
}

func TestPullFFOnly(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "new.txt"), "new content")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "new commit")
	run(t, clone2, "git", "push")

	_, err := Pull(cloneDir, "ff-only", false)
	if err != nil {
		t.Fatalf("Pull ff-only failed: %v", err)
	}

	// Verify the file arrived
	if _, statErr := os.Stat(filepath.Join(cloneDir, "new.txt")); statErr != nil {
		t.Error("expected new.txt to exist after pull")
	}
}

func TestPullRebase(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote content")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	// Make a local commit too (divergent history)
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local content")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local commit")

	_, err := Pull(cloneDir, "rebase", false)
	if err != nil {
		t.Fatalf("Pull rebase failed: %v", err)
	}

	// Both files should exist
	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after rebase pull")
	}
	if _, statErr := os.Stat(filepath.Join(cloneDir, "local.txt")); statErr != nil {
		t.Error("expected local.txt after rebase pull")
	}
}

func TestPullMerge(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	_, err := Pull(cloneDir, "merge", false)
	if err != nil {
		t.Fatalf("Pull merge failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after merge pull")
	}
}

func TestPullRebaseAutoStash(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	// Dirty the working tree by modifying a tracked file
	writeFile(t, filepath.Join(cloneDir, "README.md"), "local dirty")

	_, err := Pull(cloneDir, "rebase", true)
	if err != nil {
		t.Fatalf("Pull rebase+autostash failed: %v", err)
	}

	// Remote file should be present
	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after pull")
	}
	// Local modification should be restored by git's --autostash
	content, readErr := os.ReadFile(filepath.Join(cloneDir, "README.md"))
	if readErr != nil {
		t.Fatalf("failed to read README.md: %v", readErr)
	}
	if string(content) != "local dirty" {
		t.Errorf("expected README.md to contain autostashed changes, got %q", string(content))
	}
}

func TestPullFFOnlyAutoStashWithDirtyTree(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	// Dirty the working tree by modifying a tracked file (git stash only stashes tracked files)
	writeFile(t, filepath.Join(cloneDir, "README.md"), "modified locally")

	// ff-only with autoStash uses manual stash/unstash
	_, err := Pull(cloneDir, "ff-only", true)
	if err != nil {
		t.Fatalf("Pull ff-only+autostash failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after pull")
	}
	// Verify the local modification was restored from stash
	content, readErr := os.ReadFile(filepath.Join(cloneDir, "README.md"))
	if readErr != nil {
		t.Fatalf("failed to read README.md: %v", readErr)
	}
	if string(content) != "modified locally" {
		t.Errorf("expected README.md to contain stashed changes, got %q", string(content))
	}
}

func TestPullFFOnlyAutoStashCleanTree(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push a commit from a second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	// No dirty files — stash should be a no-op
	_, err := Pull(cloneDir, "ff-only", true)
	if err != nil {
		t.Fatalf("Pull ff-only+autostash on clean tree failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after pull")
	}
}

func TestPullDefaultStrategy(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	// Empty string strategy should default to ff-only
	_, err := Pull(cloneDir, "", false)
	if err != nil {
		t.Fatalf("Pull with default strategy failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after pull")
	}
}

func TestPullMergeAutoStash(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push from second clone
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "remote.txt"), "remote")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "remote commit")
	run(t, clone2, "git", "push")

	// Dirty the working tree by modifying a tracked file
	writeFile(t, filepath.Join(cloneDir, "README.md"), "local edit")

	// merge with autoStash uses manual stash/unstash (same as ff-only)
	_, err := Pull(cloneDir, "merge", true)
	if err != nil {
		t.Fatalf("Pull merge+autostash failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(cloneDir, "remote.txt")); statErr != nil {
		t.Error("expected remote.txt after pull")
	}
	// Verify the local modification was restored from stash
	content, readErr := os.ReadFile(filepath.Join(cloneDir, "README.md"))
	if readErr != nil {
		t.Fatalf("failed to read README.md: %v", readErr)
	}
	if string(content) != "local edit" {
		t.Errorf("expected README.md to contain stashed changes, got %q", string(content))
	}
}

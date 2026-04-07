package syncer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initBareCloneEnv creates a bare repo with an initial commit and a clone.
func initBareCloneEnv(t *testing.T) (bareDir, cloneDir string) {
	t.Helper()

	bareDir = filepath.Join(t.TempDir(), "bare.git")
	run(t, "", "git", "init", "--bare", bareDir)

	cloneDir = filepath.Join(t.TempDir(), "clone")
	run(t, "", "git", "clone", bareDir, cloneDir)

	writeFile(t, filepath.Join(cloneDir, "README.md"), "init")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "initial commit")
	run(t, cloneDir, "git", "push")

	return
}

// pushCommitFromSecondClone pushes a new commit to bare from a second clone,
// making the original clone behind by 1.
func pushCommitFromSecondClone(t *testing.T, bareDir string) {
	t.Helper()
	clone2 := filepath.Join(t.TempDir(), "clone2")
	run(t, "", "git", "clone", bareDir, clone2)
	writeFile(t, filepath.Join(clone2, "new.txt"), "from clone2")
	run(t, clone2, "git", "add", ".")
	run(t, clone2, "git", "commit", "-m", "upstream commit")
	run(t, clone2, "git", "push")
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

func TestCheckStatusCurrent(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	s := CheckStatus(cloneDir, true, false)
	if s.State != StateCurrent {
		t.Errorf("expected current, got %s: %s", s.State, s.Detail)
	}
	if s.Branch == "" {
		t.Error("expected non-empty branch")
	}
}

func TestCheckStatusBehind(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	// Use noFetch=false so it fetches the new commit
	s := CheckStatus(cloneDir, false, false)
	if s.State != StateBehind {
		t.Errorf("expected behind, got %s: %s", s.State, s.Detail)
	}
	if s.Behind != 1 {
		t.Errorf("expected 1 behind, got %d", s.Behind)
	}
}

func TestCheckStatusDirty(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	s := CheckStatus(cloneDir, true, false)
	if s.State != StateDirty {
		t.Errorf("expected dirty, got %s: %s", s.State, s.Detail)
	}
	if !s.Dirty {
		t.Error("expected Dirty=true")
	}
}

func TestCheckStatusDiverged(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)

	// Push from second clone
	pushCommitFromSecondClone(t, bareDir)

	// Make a local commit too
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local change")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local commit")

	// Fetch to see the divergence
	run(t, cloneDir, "git", "fetch")

	s := CheckStatus(cloneDir, true, false) // noFetch since we already fetched
	if s.State != StateDiverged {
		t.Errorf("expected diverged, got %s: %s", s.State, s.Detail)
	}
	if s.Ahead != 1 || s.Behind != 1 {
		t.Errorf("expected 1/1, got %d/%d", s.Ahead, s.Behind)
	}
}

func TestCheckStatusNoUpstream(t *testing.T) {
	repo := t.TempDir()
	run(t, "", "git", "init", repo)
	writeFile(t, filepath.Join(repo, "file.txt"), "content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "init")

	s := CheckStatus(repo, true, false)
	if s.State != StateNoUpstream {
		t.Errorf("expected no-upstream, got %s: %s", s.State, s.Detail)
	}
}

func TestCheckStatusAhead(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	// Make a local commit without pushing
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local commit")

	s := CheckStatus(cloneDir, true, false)
	if s.State != StateAhead {
		t.Errorf("expected ahead, got %s: %s", s.State, s.Detail)
	}
	if s.Ahead != 1 {
		t.Errorf("expected 1 ahead, got %d", s.Ahead)
	}
}

func TestSyncRepoPulls(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: false})
	if r.Action != "pulled" {
		t.Errorf("expected pulled, got %s: %s", r.Action, r.PostDetail)
	}
	if r.State != StatePulled {
		t.Errorf("expected state pulled, got %s", r.State)
	}
}

func TestSyncRepoDryRun(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	r := SyncRepo(cloneDir, SyncOptions{DryRun: true, NoFetch: false})
	if r.Action != "dry-run" {
		t.Errorf("expected dry-run, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoSkipsDirty(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: false})
	if r.Action != "skipped" {
		t.Errorf("expected skipped, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoAutoStash(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	r := SyncRepo(cloneDir, SyncOptions{AutoStash: true, NoFetch: false})
	if r.Action != "pulled" {
		t.Errorf("expected pulled, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoSkipsCurrent(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: true})
	if r.Action != "skipped" {
		t.Errorf("expected skipped, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoSkipsDiverged(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local")
	run(t, cloneDir, "git", "fetch")

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: true})
	if r.Action != "skipped" {
		t.Errorf("expected skipped, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncAllParallel(t *testing.T) {
	parent := t.TempDir()

	// Create 3 repos, each behind by 1
	var repos []string
	for _, name := range []string{"repo-a", "repo-b", "repo-c"} {
		bareDir := filepath.Join(t.TempDir(), name+".git")
		run(t, "", "git", "init", "--bare", bareDir)

		cloneDir := filepath.Join(parent, name)
		run(t, "", "git", "clone", bareDir, cloneDir)

		writeFile(t, filepath.Join(cloneDir, "README.md"), "init")
		run(t, cloneDir, "git", "add", ".")
		run(t, cloneDir, "git", "commit", "-m", "init")
		run(t, cloneDir, "git", "push")

		pushCommitFromSecondClone(t, bareDir)
		repos = append(repos, cloneDir)
	}

	results, summary := SyncAll(repos, SyncOptions{Jobs: 2, NoFetch: false})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if summary.Total != 3 {
		t.Errorf("expected total=3, got %d", summary.Total)
	}
	if summary.Pulled != 3 {
		t.Errorf("expected pulled=3, got %d", summary.Pulled)
	}
	for i, r := range results {
		if r.Action != "pulled" {
			t.Errorf("repo %d: expected pulled, got %s: %s", i, r.Action, r.PostDetail)
		}
	}
}

func TestCheckStatusDetachedHead(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)
	run(t, cloneDir, "git", "checkout", "--detach", "HEAD")

	s := CheckStatus(cloneDir, true, false)
	if s.State != StateError {
		t.Errorf("expected error state for detached HEAD, got %s: %s", s.State, s.Detail)
	}
}

func TestCheckStatusNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	s := CheckStatus(dir, true, false)
	if s.State != StateError {
		t.Errorf("expected error state for non-git dir, got %s: %s", s.State, s.Detail)
	}
}

func TestCheckStatusFields(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	s := CheckStatus(cloneDir, true, false)
	if s.Path != cloneDir {
		t.Errorf("expected path=%q, got %q", cloneDir, s.Path)
	}
	if s.Name != filepath.Base(cloneDir) {
		t.Errorf("expected name=%q, got %q", filepath.Base(cloneDir), s.Name)
	}
	if s.Upstream == "" {
		t.Error("expected non-empty upstream")
	}
	if s.Ahead != 0 || s.Behind != 0 {
		t.Errorf("expected ahead=0, behind=0, got %d/%d", s.Ahead, s.Behind)
	}
}

func TestCheckStatusDirtyAndBehind(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	run(t, cloneDir, "git", "fetch")
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	s := CheckStatus(cloneDir, true, false)
	if s.State != StateDirty {
		t.Errorf("expected dirty (behind + dirty), got %s: %s", s.State, s.Detail)
	}
	if !s.Dirty {
		t.Error("expected Dirty=true")
	}
	if s.Behind != 1 {
		t.Errorf("expected behind=1, got %d", s.Behind)
	}
}

func TestSyncRepoSkipsAhead(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local commit")

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: true})
	if r.Action != "skipped" {
		t.Errorf("expected skipped for ahead repo, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoDryRunDirty(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	run(t, cloneDir, "git", "fetch")
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: true, DryRun: true})
	if r.Action != "dry-run" {
		t.Errorf("expected dry-run, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoDryRunDirtyAutoStash(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	run(t, cloneDir, "git", "fetch")
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: true, DryRun: true, AutoStash: true})
	if r.Action != "dry-run" {
		t.Errorf("expected dry-run, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoDryRunDiverged(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local")
	run(t, cloneDir, "git", "fetch")

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: true, DryRun: true})
	if r.Action != "dry-run" {
		t.Errorf("expected dry-run, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoNoUpstreamSkips(t *testing.T) {
	repo := t.TempDir()
	run(t, "", "git", "init", repo)
	writeFile(t, filepath.Join(repo, "file.txt"), "content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "init")

	r := SyncRepo(repo, SyncOptions{NoFetch: true})
	if r.Action != "skipped" {
		t.Errorf("expected skipped for no-upstream, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncRepoStrategy(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: false, Strategy: "rebase"})
	if r.Action != "pulled" {
		t.Errorf("expected pulled with rebase strategy, got %s: %s", r.Action, r.PostDetail)
	}
}

func TestSyncAllEmpty(t *testing.T) {
	results, summary := SyncAll(nil, SyncOptions{})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if summary.Total != 0 {
		t.Errorf("expected total=0, got %d", summary.Total)
	}
}

func TestSyncAllDefaultJobs(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	// Jobs=0 should default to 1 internally
	results, summary := SyncAll([]string{cloneDir}, SyncOptions{NoFetch: true, Jobs: 0})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if summary.Total != 1 {
		t.Errorf("expected total=1, got %d", summary.Total)
	}
}

func TestSyncAllSummary(t *testing.T) {
	bareDir, behindClone := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	_, currentClone := initBareCloneEnv(t)

	results, summary := SyncAll([]string{behindClone, currentClone}, SyncOptions{NoFetch: false, Jobs: 1})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if summary.Pulled != 1 {
		t.Errorf("expected pulled=1, got %d", summary.Pulled)
	}
	if summary.Current != 1 {
		t.Errorf("expected current=1, got %d", summary.Current)
	}
}

func TestStatusAllEmpty(t *testing.T) {
	statuses := StatusAll(nil, true, false, 1)
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestStatusAllDefaultJobs(t *testing.T) {
	_, cloneDir := initBareCloneEnv(t)

	statuses := StatusAll([]string{cloneDir}, true, false, 0)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].State != StateCurrent {
		t.Errorf("expected current, got %s", statuses[0].State)
	}
}

func TestStatusAll(t *testing.T) {
	parent := t.TempDir()
	var repos []string

	for _, name := range []string{"repo-a", "repo-b"} {
		bareDir := filepath.Join(t.TempDir(), name+".git")
		run(t, "", "git", "init", "--bare", bareDir)

		cloneDir := filepath.Join(parent, name)
		run(t, "", "git", "clone", bareDir, cloneDir)

		writeFile(t, filepath.Join(cloneDir, "README.md"), "init")
		run(t, cloneDir, "git", "add", ".")
		run(t, cloneDir, "git", "commit", "-m", "init")
		run(t, cloneDir, "git", "push")
		repos = append(repos, cloneDir)
	}

	statuses := StatusAll(repos, true, false, 1)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	for i, s := range statuses {
		if s.State != StateCurrent {
			t.Errorf("repo %d: expected current, got %s: %s", i, s.State, s.Detail)
		}
	}
}

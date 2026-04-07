package syncer

import (
	"path/filepath"
	"testing"
)

// --- CheckStatus: fetch failure path ---

func TestCheckStatusFetchFails(t *testing.T) {
	// A repo with no remote; fetch will fail when noFetch=false.
	repo := t.TempDir()
	run(t, "", "git", "init", repo)
	writeFile(t, filepath.Join(repo, "file.txt"), "content")
	run(t, repo, "git", "add", ".")
	run(t, repo, "git", "commit", "-m", "init")

	s := CheckStatus(repo, false, false) // noFetch=false triggers fetch
	if s.State != StateError {
		// Fetch on a repo with no remote might fail or be a no-op depending
		// on git version. If it doesn't error, it should be no-upstream.
		if s.State != StateNoUpstream {
			t.Errorf("expected error or no-upstream for repo with no remote and fetch, got %s: %s", s.State, s.Detail)
		}
	}
}

// --- SyncAll summary: dry-run branch ---

func TestSyncAllSummary_DryRun(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	results, summary := SyncAll([]string{cloneDir}, SyncOptions{
		NoFetch: false,
		DryRun:  true,
		Jobs:    1,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "dry-run" {
		t.Errorf("expected dry-run action, got %s", results[0].Action)
	}
	if summary.Skipped != 1 {
		t.Errorf("expected skipped=1 (dry-run), got %d", summary.Skipped)
	}
}

// --- SyncAll summary: error branch ---

func TestSyncAllSummary_Error(t *testing.T) {
	// A non-git directory will produce an error.
	notRepo := t.TempDir()

	results, summary := SyncAll([]string{notRepo}, SyncOptions{
		NoFetch: true,
		Jobs:    1,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Error or skipped - either way, not pulled.
	if summary.Pulled != 0 {
		t.Errorf("expected pulled=0, got %d", summary.Pulled)
	}
}

// --- SyncAll summary: conflicts (diverged counted as conflict) ---

func TestSyncAllSummary_Conflicts(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	// Make clone diverge.
	writeFile(t, filepath.Join(cloneDir, "local.txt"), "local")
	run(t, cloneDir, "git", "add", ".")
	run(t, cloneDir, "git", "commit", "-m", "local commit")
	run(t, cloneDir, "git", "fetch")

	results, summary := SyncAll([]string{cloneDir}, SyncOptions{
		NoFetch: true,
		Jobs:    1,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "skipped" {
		t.Errorf("expected skipped for diverged, got %s", results[0].Action)
	}
	if summary.Conflicts != 1 {
		t.Errorf("expected conflicts=1 for diverged repo, got %d", summary.Conflicts)
	}
}

// --- SyncAll summary: dirty conflict ---

func TestSyncAllSummary_DirtyConflict(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)
	run(t, cloneDir, "git", "fetch")
	writeFile(t, filepath.Join(cloneDir, "dirty.txt"), "uncommitted")

	results, summary := SyncAll([]string{cloneDir}, SyncOptions{
		NoFetch: true,
		Jobs:    1,
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "skipped" {
		t.Errorf("expected skipped for dirty, got %s", results[0].Action)
	}
	if summary.Conflicts != 1 {
		t.Errorf("expected conflicts=1 for dirty+behind repo, got %d", summary.Conflicts)
	}
}

// --- SyncAll summary: mixed results ---

func TestSyncAllSummary_Mixed(t *testing.T) {
	// Create one behind repo (will be pulled).
	bareDir1, clone1 := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir1)

	// Create one current repo (will be skipped/current).
	_, clone2 := initBareCloneEnv(t)

	// Create one dry-run behind repo.
	bareDir3, clone3 := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir3)

	// Sync clone1 and clone2 normally.
	results12, summary12 := SyncAll([]string{clone1, clone2}, SyncOptions{
		NoFetch: false,
		Jobs:    1,
	})
	if summary12.Pulled != 1 {
		t.Errorf("normal sync: expected pulled=1, got %d", summary12.Pulled)
	}
	if summary12.Current != 1 {
		t.Errorf("normal sync: expected current=1, got %d", summary12.Current)
	}
	_ = results12

	// Sync clone3 with dry-run.
	results3, summary3 := SyncAll([]string{clone3}, SyncOptions{
		NoFetch: false,
		DryRun:  true,
		Jobs:    1,
	})
	if summary3.Skipped != 1 {
		t.Errorf("dry-run sync: expected skipped=1, got %d", summary3.Skipped)
	}
	_ = results3
}

// --- SyncRepo with strategy option ---

func TestSyncRepoStrategy_Merge(t *testing.T) {
	bareDir, cloneDir := initBareCloneEnv(t)
	pushCommitFromSecondClone(t, bareDir)

	r := SyncRepo(cloneDir, SyncOptions{NoFetch: false, Strategy: "no-rebase"})
	if r.Action != "pulled" {
		t.Errorf("expected pulled with no-rebase strategy, got %s: %s", r.Action, r.PostDetail)
	}
}

// --- StatusAll with multiple repos in parallel ---

func TestStatusAllParallel(t *testing.T) {
	parent := t.TempDir()
	var repos []string

	for _, name := range []string{"repo-a", "repo-b", "repo-c", "repo-d"} {
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

	statuses := StatusAll(repos, true, false, 4)
	if len(statuses) != 4 {
		t.Fatalf("expected 4 statuses, got %d", len(statuses))
	}
	for i, s := range statuses {
		if s.State != StateCurrent {
			t.Errorf("repo %d: expected current, got %s: %s", i, s.State, s.Detail)
		}
	}
}

// --- SyncAll with multiple jobs ---

func TestSyncAllParallelJobs(t *testing.T) {
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

		pushCommitFromSecondClone(t, bareDir)
		repos = append(repos, cloneDir)
	}

	results, summary := SyncAll(repos, SyncOptions{Jobs: 4, NoFetch: false})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if summary.Total != 2 {
		t.Errorf("expected total=2, got %d", summary.Total)
	}
	if summary.Pulled != 2 {
		t.Errorf("expected pulled=2, got %d", summary.Pulled)
	}
}

// --- SyncRepo error from non-git dir ---

func TestSyncRepoError(t *testing.T) {
	notRepo := t.TempDir()
	r := SyncRepo(notRepo, SyncOptions{NoFetch: true})
	if r.Action != "skipped" {
		t.Errorf("expected skipped for non-git dir, got %s: %s", r.Action, r.PostDetail)
	}
}

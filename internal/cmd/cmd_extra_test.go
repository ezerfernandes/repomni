package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/ui"
)

// --- collectBranchInfo with repoconfig ---

func TestCollectBranchInfo_WithRepoConfig(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-feature")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{
		Version:     1,
		State:       "review",
		MergeURL:    "https://github.com/org/repo/pull/42",
		Ticket:      "PROJ-123",
		Description: "My feature branch",
	}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	info := collectBranchInfo(repoDir)

	if info.State != "review" {
		t.Errorf("State = %q, want %q", info.State, "review")
	}
	if info.MergeURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("MergeURL = %q, want the saved URL", info.MergeURL)
	}
	if info.Ticket != "PROJ-123" {
		t.Errorf("Ticket = %q, want %q", info.Ticket, "PROJ-123")
	}
	if info.Description != "My feature branch" {
		t.Errorf("Description = %q, want %q", info.Description, "My feature branch")
	}
}

// --- runBranches ---

func TestRunBranches_NoRepos(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runBranches(branchesCmd, []string{dir})
	if err == nil {
		t.Fatal("expected error for dir with no repos")
	}
	if !strings.Contains(err.Error(), "no git repositories found") {
		t.Errorf("error = %q, want mention of no repos", err.Error())
	}
}

func TestRunBranches_WithRepos(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origJSON := branchesJSON
	defer func() { branchesJSON = origJSON }()
	branchesJSON = true

	err := runBranches(branchesCmd, []string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBranches_StateFilterNoMatch(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origState := branchesState
	defer func() { branchesState = origState }()
	branchesState = "nonexistent-state"

	err := runBranches(branchesCmd, []string{dir})
	if err == nil {
		t.Fatal("expected error for unmatched state filter")
	}
	if !strings.Contains(err.Error(), "no repos with state") {
		t.Errorf("error = %q, want mention of no repos with state", err.Error())
	}
}

// --- runList ---

func TestRunList_NoRepos(t *testing.T) {
	dir := t.TempDir()

	err := runList(listCmd, []string{dir})
	if err == nil {
		t.Fatal("expected error for dir with no repos")
	}
	if !strings.Contains(err.Error(), "no git repositories found") {
		t.Errorf("error = %q, want mention of no repos", err.Error())
	}
}

func TestRunList_WithRepos(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Test with --names.
	origNames := listNames
	defer func() { listNames = origNames }()
	listNames = true

	err := runList(listCmd, []string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunList_JSON(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origJSON := listJSON
	defer func() { listJSON = origJSON }()
	listJSON = true

	err := runList(listCmd, []string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runSubmit additional error paths ---

func TestRunSubmit_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want 'not inside a git repository'", err.Error())
	}
}

func TestRunSubmit_DetachedHead(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// Create a detached HEAD.
	runGitCmd(t, repoDir, "checkout", "--detach")

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "detached") {
		t.Errorf("error = %q, want mention of detached HEAD", err.Error())
	}
}

// --- runMerge additional error paths ---

func TestRunMerge_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runMerge(mergeCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want 'not inside a git repository'", err.Error())
	}
}

// --- runOpen / runReady / runChecks / runReview: not a git repo ---

func TestRunOpen_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runOpen(openCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not inside a git repo", err.Error())
	}
}

func TestRunReady_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runReady(readyCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not inside a git repo", err.Error())
	}
}

func TestRunChecks_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runChecks(checksCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not inside a git repo", err.Error())
	}
}

func TestRunReview_NotGitRepo(t *testing.T) {
	origApprove := reviewApprove
	defer func() { reviewApprove = origApprove }()
	reviewApprove = true

	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runReview(reviewCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not inside a git repo", err.Error())
	}
}

// --- resolveMainDir auto-discovery ---

func TestResolveMainDir_AutoDiscover(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	// Create parent with a git repo.
	parentDir := filepath.Join(dir, "parent")
	if err := os.Mkdir(parentDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, parentDir)

	// Branch dir is a sibling.
	branchDir := filepath.Join(dir, "branch")
	if err := os.Mkdir(branchDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, branchDir)

	execDiffMainDir = ""

	got, err := resolveMainDir(branchDir)
	if err != nil {
		// Auto-discovery may fail if there's no parent repo layout.
		// Just verify the error message is sensible.
		if !strings.Contains(err.Error(), "could not find main repo") {
			t.Errorf("unexpected error: %v", err)
		}
		return
	}
	if got == "" {
		t.Error("expected non-empty main dir")
	}
}

// --- collectClaudeCleanCandidates ---

func TestCollectClaudeCleanCandidates(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	projectPath := "/test/project"
	encoded := strings.ReplaceAll(projectPath, "/", "-")
	encoded = strings.ReplaceAll(encoded, "_", "-")
	sessDir := filepath.Join(fakeHome, ".claude", "projects", encoded)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an empty session file.
	if err := os.WriteFile(filepath.Join(sessDir, "empty-session.jsonl"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-empty session file that is old.
	oldFile := filepath.Join(sessDir, "old-session.jsonl")
	if err := os.WriteFile(oldFile, []byte(`{"type":"user"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set modification time to 60 days ago.
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create a recent non-empty session file.
	if err := os.WriteFile(filepath.Join(sessDir, "recent-session.jsonl"), []byte(`{"type":"user"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-jsonl file (should be ignored).
	if err := os.WriteFile(filepath.Join(sessDir, "notes.txt"), []byte("not a session"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a directory (should be ignored).
	if err := os.MkdirAll(filepath.Join(sessDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	// Test: collect empty sessions.
	origEmpty := sessionCleanEmpty
	defer func() { sessionCleanEmpty = origEmpty }()
	sessionCleanEmpty = true

	candidates := collectClaudeCleanCandidates(projectPath, 0, time.Time{})
	if len(candidates) != 1 {
		t.Fatalf("expected 1 empty candidate, got %d", len(candidates))
	}
	if candidates[0].Reason != "empty (0 bytes)" {
		t.Errorf("reason = %q, want 'empty (0 bytes)'", candidates[0].Reason)
	}

	// Test: collect old sessions (not empty-only mode).
	sessionCleanEmpty = false
	origOlderThan := sessionCleanOlderThan
	defer func() { sessionCleanOlderThan = origOlderThan }()
	sessionCleanOlderThan = "30d"

	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	candidates = collectClaudeCleanCandidates(projectPath, 30*24*time.Hour, cutoff)

	// Should find the old-session and the empty session.
	foundOld := false
	foundEmpty := false
	for _, c := range candidates {
		if strings.Contains(c.SessionID, "old-session") {
			foundOld = true
		}
		if strings.Contains(c.SessionID, "empty-session") {
			foundEmpty = true
		}
	}
	if !foundOld {
		t.Error("expected to find old session in candidates")
	}
	if !foundEmpty {
		t.Error("expected to find empty session in candidates")
	}
}

func TestCollectClaudeCleanCandidates_NoDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// No sessions dir exists.
	candidates := collectClaudeCleanCandidates("/nonexistent/project", 0, time.Time{})
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for nonexistent dir, got %d", len(candidates))
	}
}

// --- runExecDiff additional error paths ---

func TestRunExecDiff_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// ArgsLenAtDash returns -1 when no -- present, so parseUserCommand will error.
	err := runExecDiff(execDiffCmd, []string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- DirSize edge case ---

func TestDirSize_WithSymlink(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "real.txt"), 50)

	// Create a symlink to the file.
	if err := os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	got := dirSize(dir)
	// Should count real file, symlink may or may not be counted depending on OS.
	if got < 50 {
		t.Errorf("dirSize should be at least 50, got %d", got)
	}
}

// --- archiveBranches edge cases ---

func TestArchiveBranches_AllSkipped(t *testing.T) {
	dir := t.TempDir()

	candidates := []ui.CleanCandidate{
		{Info: ui.BranchInfo{Name: "skip-1"}, Skipped: true, Reason: "test"},
		{Info: ui.BranchInfo{Name: "skip-2"}, Skipped: true, Reason: "test"},
	}

	if err := archiveBranches(dir, candidates); err != nil {
		t.Fatalf("archiveBranches: %v", err)
	}

	archivePath := filepath.Join(dir, ".repomni-archive.json")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive should still be written: %v", err)
	}
	// All skipped, so the archive should have no real entries.
	content := strings.TrimSpace(string(data))
	if content != "null" && content != "[]" {
		t.Errorf("expected null or empty array in archive, got: %s", content)
	}
}

// --- cleanDanglingSymlinks edge case ---

func TestCleanDanglingSymlinks_NonexistentDir(t *testing.T) {
	cleaned := cleanDanglingSymlinks("/nonexistent/path")
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned for nonexistent dir, got %d", cleaned)
	}
}

// --- runSetState / runSetDescription / runSetTicket not in git repo ---

func TestRunSetState_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runSetState(setStateCmd, []string{"active"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not in git repo", err.Error())
	}
}

func TestRunSetDescription_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runSetDescription(setDescriptionCmd, []string{"desc"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not in git repo", err.Error())
	}
}

func TestRunSetTicket_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	err := runSetTicket(setTicketCmd, []string{"PROJ-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("error = %q, want mention of not in git repo", err.Error())
	}
}

// --- runSetState in a real git repo ---

func TestRunSetState_NoArgs(t *testing.T) {
	origClear := setStateClear
	defer func() { setStateClear = origClear }()
	setStateClear = false

	err := runSetState(setStateCmd, []string{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provide a state name") {
		t.Errorf("error = %q, want mention of provide a state name", err.Error())
	}
}

func TestRunSetState_URLWithNonReviewState(t *testing.T) {
	err := runSetState(setStateCmd, []string{"active", "https://github.com/org/repo/pull/1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "merge URL can only be provided when state is") {
		t.Errorf("error = %q, want mention of merge URL only for review", err.Error())
	}
}

func TestRunSetState_Success(t *testing.T) {
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

	origClear, origJSON := setStateClear, setStateJSON
	defer func() { setStateClear, setStateJSON = origClear, origJSON }()
	setStateClear = false
	setStateJSON = false

	err := runSetState(setStateCmd, []string{"active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify state was saved.
	gitDir := filepath.Join(repoDir, ".git")
	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.State != "active" {
		t.Errorf("State = %q, want %q", cfg.State, "active")
	}
}

func TestRunSetState_ReviewWithURL(t *testing.T) {
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

	origClear, origJSON := setStateClear, setStateJSON
	defer func() { setStateClear, setStateJSON = origClear, origJSON }()
	setStateClear = false
	setStateJSON = false

	err := runSetState(setStateCmd, []string{"review", "https://github.com/org/repo/pull/42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitDir := filepath.Join(repoDir, ".git")
	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.State != "review" {
		t.Errorf("State = %q, want %q", cfg.State, "review")
	}
	if cfg.MergeURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("MergeURL = %q, want the PR URL", cfg.MergeURL)
	}
}

func TestRunSetState_Clear(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// First set a state.
	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{Version: 1, State: "review", MergeURL: "https://example.com/pr/1"}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	origClear, origJSON := setStateClear, setStateJSON
	defer func() { setStateClear, setStateJSON = origClear, origJSON }()
	setStateClear = true
	setStateJSON = false

	err := runSetState(setStateCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err = repoconfig.Load(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.State != "" {
		t.Errorf("State = %q, want empty", cfg.State)
	}
	if cfg.MergeURL != "" {
		t.Errorf("MergeURL = %q, want empty", cfg.MergeURL)
	}
}

func TestRunSetState_JSON(t *testing.T) {
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

	origClear, origJSON := setStateClear, setStateJSON
	defer func() { setStateClear, setStateJSON = origClear, origJSON }()
	setStateClear = false
	setStateJSON = true

	err := runSetState(setStateCmd, []string{"active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetState_InvalidState(t *testing.T) {
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

	origClear := setStateClear
	defer func() { setStateClear = origClear }()
	setStateClear = false

	err := runSetState(setStateCmd, []string{"INVALID STATE"})
	if err == nil {
		t.Fatal("expected error for invalid state, got nil")
	}
}

// --- runSetDescription in a real git repo ---

func TestRunSetDescription_NoArgs(t *testing.T) {
	origClear := setDescriptionClear
	defer func() { setDescriptionClear = origClear }()
	setDescriptionClear = false

	err := runSetDescription(setDescriptionCmd, []string{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provide a description") {
		t.Errorf("error = %q, want mention of provide a description", err.Error())
	}
}

func TestRunSetDescription_Success(t *testing.T) {
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

	origClear, origJSON := setDescriptionClear, setDescriptionJSON
	defer func() { setDescriptionClear, setDescriptionJSON = origClear, origJSON }()
	setDescriptionClear = false
	setDescriptionJSON = false

	err := runSetDescription(setDescriptionCmd, []string{"working on auth"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitDir := filepath.Join(repoDir, ".git")
	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Description != "working on auth" {
		t.Errorf("Description = %q, want %q", cfg.Description, "working on auth")
	}
}

func TestRunSetDescription_Clear(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{Version: 1, Description: "old desc"}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	origClear, origJSON := setDescriptionClear, setDescriptionJSON
	defer func() { setDescriptionClear, setDescriptionJSON = origClear, origJSON }()
	setDescriptionClear = true
	setDescriptionJSON = false

	err := runSetDescription(setDescriptionCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, _ = repoconfig.Load(gitDir)
	if cfg.Description != "" {
		t.Errorf("Description = %q, want empty", cfg.Description)
	}
}

// --- runSetTicket in a real git repo ---

func TestRunSetTicket_NoArgs(t *testing.T) {
	origClear := setTicketClear
	defer func() { setTicketClear = origClear }()
	setTicketClear = false

	err := runSetTicket(setTicketCmd, []string{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "provide a ticket identifier") {
		t.Errorf("error = %q, want mention of provide a ticket", err.Error())
	}
}

func TestRunSetTicket_Success(t *testing.T) {
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

	origClear, origJSON := setTicketClear, setTicketJSON
	defer func() { setTicketClear, setTicketJSON = origClear, origJSON }()
	setTicketClear = false
	setTicketJSON = false

	err := runSetTicket(setTicketCmd, []string{"PROJ-456"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gitDir := filepath.Join(repoDir, ".git")
	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ticket != "PROJ-456" {
		t.Errorf("Ticket = %q, want %q", cfg.Ticket, "PROJ-456")
	}
}

func TestRunSetTicket_Clear(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{Version: 1, Ticket: "OLD-123"}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	origClear, origJSON := setTicketClear, setTicketJSON
	defer func() { setTicketClear, setTicketJSON = origClear, origJSON }()
	setTicketClear = true
	setTicketJSON = false

	err := runSetTicket(setTicketCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, _ = repoconfig.Load(gitDir)
	if cfg.Ticket != "" {
		t.Errorf("Ticket = %q, want empty", cfg.Ticket)
	}
}

// --- runAttach URL validation ---

func TestRunAttach_InvalidURL(t *testing.T) {
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

	origCurrent := attachCurrent
	defer func() { attachCurrent = origCurrent }()
	attachCurrent = false

	// Pass a non-URL string.
	err := runAttach(attachCmd, []string{"not-a-url"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// --- runChecks / runReview with merge URL (reaches forge platform detection) ---

func TestRunChecks_WithMergeURL(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{
		Version:  1,
		State:    "review",
		MergeURL: "https://github.com/org/repo/pull/42",
	}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	origJSON, origWatch := checksJSON, checksWatch
	defer func() { checksJSON, checksWatch = origJSON, origWatch }()
	checksJSON = false
	checksWatch = false

	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	if err := runChecks(checksCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	got := strings.Join((*calls)[0].Args, " ")
	if got != "pr checks" {
		t.Errorf("expected args 'pr checks', got %q", got)
	}
}

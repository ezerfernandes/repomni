package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

// fakeForgeCall records a single call to a stubbed forge function.
type fakeForgeCall struct {
	Platform forge.Platform
	Args     []string
}

// stubForge replaces forge.CheckCLI, forge.RunForgeDir, and forge.RunForgePassthrough
// with fakes that record calls instead of executing real CLI tools.
// Returns a pointer to the recorded calls and a cleanup function that restores originals.
func stubForge(t *testing.T) (*[]fakeForgeCall, func()) {
	t.Helper()
	origCheckCLI := forge.CheckCLI
	origRunDir := forge.RunForgeDir
	origRunPT := forge.RunForgePassthrough

	var calls []fakeForgeCall

	forge.CheckCLI = func(platform forge.Platform) error {
		return nil
	}
	forge.RunForgeDir = func(dir string, platform forge.Platform, args ...string) (string, error) {
		calls = append(calls, fakeForgeCall{Platform: platform, Args: args})
		return "", nil
	}
	forge.RunForgePassthrough = func(dir string, platform forge.Platform, args ...string) error {
		calls = append(calls, fakeForgeCall{Platform: platform, Args: args})
		return nil
	}

	return &calls, func() {
		forge.CheckCLI = origCheckCLI
		forge.RunForgeDir = origRunDir
		forge.RunForgePassthrough = origRunPT
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// --- printSyncStateResults ---

func TestPrintSyncStateResults_AllBranches(t *testing.T) {
	results := []mergestatus.Result{
		{Name: "repo-a", NewState: "approved", PreviousState: "review", Changed: true},
		{Name: "repo-b", NewState: "review", PreviousState: "review", Changed: false},
		{Name: "repo-c", NewState: "closed", PreviousState: "review", Changed: true},
		{Name: "repo-d", Error: "network timeout"},
	}
	summary := mergestatus.Summary{Total: 4, Updated: 2, Unchanged: 1, Errors: 1}

	out := captureStdout(t, func() {
		printSyncStateResults(results, summary)
	})

	if !strings.Contains(out, "Checking 4 branches") {
		t.Error("expected total count in output")
	}
	if !strings.Contains(out, "repo-a") {
		t.Error("expected repo-a in output")
	}
	if !strings.Contains(out, "no change") {
		t.Error("expected 'no change' for unchanged repo")
	}
	if !strings.Contains(out, "network timeout") {
		t.Error("expected error message for failed repo")
	}
	if !strings.Contains(out, "2 updated") {
		t.Error("expected '2 updated' in summary")
	}
	if !strings.Contains(out, "1 unchanged") {
		t.Error("expected '1 unchanged' in summary")
	}
	if !strings.Contains(out, "1 errors") {
		t.Error("expected '1 errors' in summary")
	}
}

func TestPrintSyncStateResults_Empty(t *testing.T) {
	out := captureStdout(t, func() {
		printSyncStateResults(nil, mergestatus.Summary{Total: 0})
	})

	if !strings.Contains(out, "Checking 0 branches") {
		t.Error("expected zero count in output")
	}
}

func TestPrintSyncStateResults_OnlyUpdated(t *testing.T) {
	results := []mergestatus.Result{
		{Name: "repo", NewState: "merged", PreviousState: "approved", Changed: true},
	}
	summary := mergestatus.Summary{Total: 1, Updated: 1}

	out := captureStdout(t, func() {
		printSyncStateResults(results, summary)
	})

	if !strings.Contains(out, "1 updated") {
		t.Error("expected '1 updated' in summary")
	}
	// Should not contain "unchanged" or "errors" parts.
	if strings.Contains(out, "unchanged") {
		t.Error("unchanged should not appear when count is 0")
	}
}

// --- loadRepoConfig ---

func TestLoadRepoConfig_WithConfig(t *testing.T) {
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

	loaded, err := loadRepoConfig(repoDir)
	if err != nil {
		t.Fatalf("loadRepoConfig() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil config")
	}
	if loaded.State != "active" {
		t.Errorf("State = %q, want %q", loaded.State, "active")
	}
}

func TestLoadRepoConfig_NoConfig(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	loaded, err := loadRepoConfig(repoDir)
	if err != nil {
		t.Fatalf("loadRepoConfig() error: %v", err)
	}
	// No config file exists, should return nil, nil.
	if loaded != nil {
		t.Errorf("expected nil config, got %+v", loaded)
	}
}

func TestLoadRepoConfig_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	loaded, err := loadRepoConfig(dir)
	if err != nil {
		t.Fatalf("loadRepoConfig() error: %v", err)
	}
	// Not a git repo - returns nil, nil.
	if loaded != nil {
		t.Errorf("expected nil config for non-git dir, got %+v", loaded)
	}
}

// --- printAttachJSON ---

func TestPrintAttachJSON(t *testing.T) {
	cfg := &repoconfig.RepoConfig{
		MergeURL:    "https://github.com/org/repo/pull/42",
		MergeNumber: 42,
		Draft:       false,
		BaseBranch:  "main",
	}

	out := captureStdout(t, func() {
		err := printAttachJSON(cfg, "review")
		if err != nil {
			t.Errorf("printAttachJSON() error: %v", err)
		}
	})

	if !strings.Contains(out, "https://github.com/org/repo/pull/42") {
		t.Error("expected merge URL in output")
	}
	if !strings.Contains(out, `"merge_number": 42`) {
		t.Error("expected merge number in output")
	}
	if !strings.Contains(out, `"state": "review"`) {
		t.Error("expected state in output")
	}
	if !strings.Contains(out, `"base_branch": "main"`) {
		t.Error("expected base branch in output")
	}
}

// --- runStatus ---

func TestRunStatus_NoRepos_WithAll(t *testing.T) {
	dir := t.TempDir()

	origAll := statusAll
	defer func() { statusAll = origAll }()
	statusAll = true

	err := runStatus(statusCmd, []string{dir})
	if err == nil {
		t.Fatal("expected error for empty dir with --all")
	}
	if !strings.Contains(err.Error(), "no git repositories found") {
		t.Errorf("error = %q, want mention of no repos", err.Error())
	}
}

func TestRunStatus_GitMode(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origGit, origAll, origJSON := statusGit, statusAll, statusJSON
	defer func() { statusGit, statusAll, statusJSON = origGit, origAll, origJSON }()
	statusGit = true
	statusAll = false
	statusJSON = false

	err := runStatus(statusCmd, []string{repoDir})
	if err != nil {
		t.Fatalf("runStatus(git) error: %v", err)
	}
}

func TestRunStatus_GitMode_JSON(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origGit, origAll, origJSON := statusGit, statusAll, statusJSON
	defer func() { statusGit, statusAll, statusJSON = origGit, origAll, origJSON }()
	statusGit = true
	statusAll = false
	statusJSON = true

	err := runStatus(statusCmd, []string{repoDir})
	if err != nil {
		t.Fatalf("runStatus(git,json) error: %v", err)
	}
}

// --- runSetState JSON ---

func TestRunSetState_ClearJSON(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

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
	setStateJSON = true

	err := runSetState(setStateCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runSetDescription JSON ---

func TestRunSetDescription_JSON(t *testing.T) {
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
	setDescriptionJSON = true

	err := runSetDescription(setDescriptionCmd, []string{"my desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetDescription_ClearJSON(t *testing.T) {
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
	setDescriptionClear = true
	setDescriptionJSON = true

	err := runSetDescription(setDescriptionCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runSetTicket JSON ---

func TestRunSetTicket_JSON(t *testing.T) {
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
	setTicketJSON = true

	err := runSetTicket(setTicketCmd, []string{"PROJ-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetTicket_ClearJSON(t *testing.T) {
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
	setTicketClear = true
	setTicketJSON = true

	err := runSetTicket(setTicketCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runBranches with --detailed ---

func TestRunBranches_Detailed(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "my-repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origJSON, origDetailed, origState := branchesJSON, branchesDetailed, branchesState
	defer func() { branchesJSON, branchesDetailed, branchesState = origJSON, origDetailed, origState }()
	branchesJSON = false
	branchesDetailed = true
	branchesState = ""

	err := runBranches(branchesCmd, []string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runList with default target (current dir) ---

func TestRunList_DefaultTarget(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	origNames, origJSON := listNames, listJSON
	defer func() { listNames, listJSON = origNames, origJSON }()
	listNames = false
	listJSON = false

	err := runList(listCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runSetState with review URL and JSON ---

func TestRunSetState_ReviewWithURLJSON(t *testing.T) {
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

	err := runSetState(setStateCmd, []string{"review", "https://github.com/org/repo/pull/42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- run* functions with merge URL set (exercises platform detection and forge call error paths) ---

func setupRepoWithMergeURL(t *testing.T, url string) (repoDir string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir = filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{
		Version:  1,
		State:    "review",
		MergeURL: url,
	}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	return repoDir, func() { _ = os.Chdir(origDir) }
}

func TestRunMerge_WithMergeURL(t *testing.T) {
	repoDir, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	origSquash, origRebase, origDelete := mergeSquash, mergeRebase, mergeDeleteBranch
	defer func() { mergeSquash, mergeRebase, mergeDeleteBranch = origSquash, origRebase, origDelete }()
	mergeSquash = false
	mergeRebase = false
	mergeDeleteBranch = false

	out := captureStdout(t, func() {
		if err := runMerge(mergeCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Merged.") {
		t.Fatalf("expected merge confirmation, got %q", out)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	c := (*calls)[0]
	if c.Platform != forge.PlatformGitHub {
		t.Errorf("expected GitHub platform, got %s", c.Platform)
	}
	got := strings.Join(c.Args, " ")
	if got != "pr merge" {
		t.Errorf("expected args 'pr merge', got %q", got)
	}
	cfg, err := repoconfig.Load(filepath.Join(repoDir, ".git"))
	if err != nil {
		t.Fatalf("repoconfig.Load() error: %v", err)
	}
	if cfg.State != string(repoconfig.StateMerged) {
		t.Fatalf("state = %q, want %q", cfg.State, repoconfig.StateMerged)
	}
}

func TestRunOpen_WithMergeURL(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	if err := runOpen(openCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	c := (*calls)[0]
	if c.Platform != forge.PlatformGitHub {
		t.Errorf("expected GitHub platform, got %s", c.Platform)
	}
	got := strings.Join(c.Args, " ")
	want := "pr view --web https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999"
	if got != want {
		t.Errorf("expected args %q, got %q", want, got)
	}
}

func TestRunReady_WithMergeURL(t *testing.T) {
	repoDir, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	out := captureStdout(t, func() {
		if err := runReady(readyCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "PR/MR marked as ready for review.") {
		t.Fatalf("expected ready confirmation, got %q", out)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	c := (*calls)[0]
	if c.Platform != forge.PlatformGitHub {
		t.Errorf("expected GitHub platform, got %s", c.Platform)
	}
	got := strings.Join(c.Args, " ")
	if got != "pr ready" {
		t.Errorf("expected args 'pr ready', got %q", got)
	}
	cfg, err := repoconfig.Load(filepath.Join(repoDir, ".git"))
	if err != nil {
		t.Fatalf("repoconfig.Load() error: %v", err)
	}
	if cfg.Draft {
		t.Fatal("expected draft=false after ready")
	}
}

func TestRunReview_WithMergeURL_Approve(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	origApprove, origComment, origJSON := reviewApprove, reviewComment, reviewJSON
	defer func() { reviewApprove, reviewComment, reviewJSON = origApprove, origComment, origJSON }()
	reviewApprove = true
	reviewComment = ""
	reviewJSON = false

	out := captureStdout(t, func() {
		if err := runReview(reviewCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Approved.") {
		t.Fatalf("expected approve confirmation, got %q", out)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	got := strings.Join((*calls)[0].Args, " ")
	if got != "pr review --approve" {
		t.Errorf("expected args 'pr review --approve', got %q", got)
	}
}

func TestRunReview_WithMergeURL_Comment(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	origApprove, origComment, origJSON := reviewApprove, reviewComment, reviewJSON
	defer func() { reviewApprove, reviewComment, reviewJSON = origApprove, origComment, origJSON }()
	reviewApprove = false
	reviewComment = "looks good"
	reviewJSON = false

	out := captureStdout(t, func() {
		if err := runReview(reviewCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Comment submitted.") {
		t.Fatalf("expected comment confirmation, got %q", out)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	got := strings.Join((*calls)[0].Args, " ")
	if got != "pr review --comment --body looks good" {
		t.Errorf("expected args 'pr review --comment --body looks good', got %q", got)
	}
}

func TestRunReview_WithMergeURL_ApproveAndComment(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	origApprove, origComment, origJSON := reviewApprove, reviewComment, reviewJSON
	defer func() { reviewApprove, reviewComment, reviewJSON = origApprove, origComment, origJSON }()
	reviewApprove = true
	reviewComment = "looks good"
	reviewJSON = false

	out := captureStdout(t, func() {
		if err := runReview(reviewCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Approved with comment.") {
		t.Fatalf("expected combined review confirmation, got %q", out)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	got := strings.Join((*calls)[0].Args, " ")
	if got != "pr review --approve --body looks good" {
		t.Errorf("expected args 'pr review --approve --body looks good', got %q", got)
	}
}

func TestRunReview_WithGitLabURL(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://gitlab.com/nonexistent-group-xyzzy/nonexistent-project-xyzzy/-/merge_requests/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	origApprove, origComment, origJSON := reviewApprove, reviewComment, reviewJSON
	defer func() { reviewApprove, reviewComment, reviewJSON = origApprove, origComment, origJSON }()
	reviewApprove = true
	reviewComment = "nice"
	reviewJSON = false

	out := captureStdout(t, func() {
		if err := runReview(reviewCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Approved.") || !strings.Contains(out, "Comment submitted.") {
		t.Fatalf("expected GitLab review confirmations, got %q", out)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 forge calls (approve + comment), got %d", len(*calls))
	}
	if (*calls)[0].Platform != forge.PlatformGitLab {
		t.Errorf("expected GitLab platform, got %s", (*calls)[0].Platform)
	}
	approveArgs := strings.Join((*calls)[0].Args, " ")
	if approveArgs != "mr approve 999999" {
		t.Errorf("expected args 'mr approve 999999', got %q", approveArgs)
	}
	noteArgs := strings.Join((*calls)[1].Args, " ")
	if noteArgs != "mr note 999999 --message nice" {
		t.Errorf("expected args 'mr note 999999 --message nice', got %q", noteArgs)
	}
}

func TestRunChecks_WithMergeURL_Watch(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	calls, forgeCleanup := stubForge(t)
	defer forgeCleanup()

	origWatch, origJSON := checksWatch, checksJSON
	defer func() { checksWatch, checksJSON = origWatch, origJSON }()
	checksWatch = true
	checksJSON = false

	if err := runChecks(checksCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 forge call, got %d", len(*calls))
	}
	got := strings.Join((*calls)[0].Args, " ")
	if got != "pr checks --watch" {
		t.Errorf("expected args 'pr checks --watch', got %q", got)
	}
}

func TestRunChecks_JSON(t *testing.T) {
	_, cleanup := setupRepoWithMergeURL(t, "https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	defer cleanup()
	// Stub RunForgeDir to return valid JSON so runChecksJSON can parse it.
	origCheckCLI := forge.CheckCLI
	origRunDir := forge.RunForgeDir
	origRunPT := forge.RunForgePassthrough
	defer func() {
		forge.CheckCLI = origCheckCLI
		forge.RunForgeDir = origRunDir
		forge.RunForgePassthrough = origRunPT
	}()

	var gotArgs []string
	forge.CheckCLI = func(platform forge.Platform) error { return nil }
	forge.RunForgeDir = func(dir string, platform forge.Platform, args ...string) (string, error) {
		gotArgs = args
		return "[]", nil // valid empty JSON array
	}
	forge.RunForgePassthrough = func(dir string, platform forge.Platform, args ...string) error { return nil }

	origWatch, origJSON := checksWatch, checksJSON
	defer func() { checksWatch, checksJSON = origWatch, origJSON }()
	checksWatch = false
	checksJSON = true

	out := captureStdout(t, func() {
		if err := runChecks(checksCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected JSON output [], got %q", out)
	}
	got := strings.Join(gotArgs, " ")
	want := "pr checks --json name,state,conclusion,detailsUrl"
	if got != want {
		t.Errorf("expected args %q, got %q", want, got)
	}
}

func TestRunSubmit_WithFeatureBranch(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)
	runGitCmd(t, repoDir, "checkout", "-b", "feature-branch")

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	// Should get past branch check, then fail at platform detection (no remote).
	err := runSubmit(submitCmd, nil)
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "cannot submit from main") {
		t.Error("should have gotten past branch check")
	}
	if strings.Contains(err.Error(), "PR/MR already exists") {
		t.Error("should have gotten past existing PR check")
	}
}

// --- runSetState with invalid merge URL ---

func TestRunSetState_ReviewWithInvalidURL(t *testing.T) {
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

	err := runSetState(setStateCmd, []string{"review", "not-a-url"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

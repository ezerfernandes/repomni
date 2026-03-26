package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/injector"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/syncer"
)

type injectJSONResult struct {
	Target string `json:"target"`
	Action string `json:"action"`
	Item   string `json:"item"`
	Detail string `json:"detail"`
}

type ejectJSONResult struct {
	Target string `json:"target"`
	Action string `json:"action"`
	Item   string `json:"item"`
	Detail string `json:"detail"`
}

func installFakeForgeCLIs(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()
	writeExecutable(t, binDir, "gh", `#!/bin/sh
case "$*" in
  *fail*)
    echo "simulated gh failure" >&2
    exit 1
    ;;
  *)
    echo '{"state":"OPEN","reviewDecision":"APPROVED","statusCheckRollup":[]}'
    ;;
esac
`)
	writeExecutable(t, binDir, "glab", `#!/bin/sh
echo '{"state":"opened","approved":true}'
`)
	prependPath(t, binDir)
}

func TestRunInject_JSONSuccessAndStatusJSON(t *testing.T) {
	resetRepoWrapperFlags(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	sourceDir := setupSourceDir(t, map[string]string{"shared.txt": "hello from source\n"})
	saveGlobalConfig(t, &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{{
			Type:       config.ItemTypeFile,
			SourcePath: "shared.txt",
			TargetPath: ".repomni/shared.txt",
			Enabled:    true,
		}},
	})

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	injectJSON = true
	injectCopy = true
	injectYes = true

	injectOut := captureStdout(t, func() {
		if err := runInject(injectCmd, []string{repoDir}); err != nil {
			t.Fatalf("runInject() error: %v", err)
		}
	})

	var injectResults []injectJSONResult
	if err := json.Unmarshal([]byte(injectOut), &injectResults); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, injectOut)
	}
	if len(injectResults) != 1 {
		t.Fatalf("got %d inject results, want 1", len(injectResults))
	}
	if injectResults[0].Action != "created" || injectResults[0].Item != ".repomni/shared.txt" {
		t.Fatalf("unexpected inject result: %+v", injectResults[0])
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".repomni", "shared.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile() error: %v", err)
	}
	if string(data) != "hello from source\n" {
		t.Fatalf("copied content = %q, want %q", string(data), "hello from source\n")
	}

	statusJSON = true
	statusOut := captureStdout(t, func() {
		if err := runStatus(statusCmd, []string{repoDir}); err != nil {
			t.Fatalf("runStatus() error: %v", err)
		}
	})

	var statusResults []jsonStatusOutput
	if err := json.Unmarshal([]byte(statusOut), &statusResults); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, statusOut)
	}
	if len(statusResults) != 1 || len(statusResults[0].Items) != 1 {
		t.Fatalf("unexpected status results: %+v", statusResults)
	}
	item := statusResults[0].Items[0]
	if !item.Present || !item.Current || !item.Excluded {
		t.Fatalf("expected present/current/excluded item, got %+v", item)
	}
}

func TestRunInject_JSONErrorAndAllModes(t *testing.T) {
	resetRepoWrapperFlags(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	sourceDir := setupSourceDir(t, map[string]string{"shared.txt": "hello"})
	saveGlobalConfig(t, &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{{
			Type:       config.ItemTypeFile,
			SourcePath: "shared.txt",
			TargetPath: "../escape.txt",
			Enabled:    true,
		}},
	})

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	injectJSON = true
	injectYes = true
	out := captureStdout(t, func() {})
	_ = out
	injectOut := captureStdout(t, func() {
		err := runInject(injectCmd, []string{repoDir})
		if err == nil || !strings.Contains(err.Error(), "some items had errors") {
			t.Fatalf("expected aggregate inject error, got %v", err)
		}
	})

	var injectResults []injectJSONResult
	if err := json.Unmarshal([]byte(injectOut), &injectResults); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, injectOut)
	}
	if len(injectResults) != 1 || injectResults[0].Action != "error" {
		t.Fatalf("unexpected inject results: %+v", injectResults)
	}
	if !strings.Contains(injectResults[0].Detail, "escapes target directory") {
		t.Fatalf("unexpected error detail: %+v", injectResults[0])
	}

	validSourceDir := setupSourceDir(t, map[string]string{"shared.txt": "multi"})
	saveGlobalConfig(t, &config.Config{
		Version:   1,
		SourceDir: validSourceDir,
		Mode:      config.ModeCopy,
		Items: []config.Item{{
			Type:       config.ItemTypeFile,
			SourcePath: "shared.txt",
			TargetPath: ".repomni/shared.txt",
			Enabled:    true,
		}},
	})

	parentDir := t.TempDir()
	repoA := filepath.Join(parentDir, "repo-a")
	repoB := filepath.Join(parentDir, "repo-b")
	if err := os.MkdirAll(repoA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoB, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoA)
	initGitRepo(t, repoB)

	injectAll = true
	injectJSON = true
	injectYes = true
	allOut := captureStdout(t, func() {
		if err := runInject(injectCmd, []string{parentDir}); err != nil {
			t.Fatalf("runInject(all) error: %v", err)
		}
	})
	injectAll = false

	injectResults = nil
	if err := json.Unmarshal([]byte(allOut), &injectResults); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, allOut)
	}
	if len(injectResults) != 2 {
		t.Fatalf("got %d all-target results, want 2", len(injectResults))
	}
}

func TestRunInject_AllNoRepos(t *testing.T) {
	resetRepoWrapperFlags(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	sourceDir := setupSourceDir(t, map[string]string{"shared.txt": "hello"})
	saveGlobalConfig(t, &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeCopy,
		Items: []config.Item{{
			Type:       config.ItemTypeFile,
			SourcePath: "shared.txt",
			TargetPath: ".repomni/shared.txt",
			Enabled:    true,
		}},
	})

	injectAll = true
	err := runInject(injectCmd, []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "no git repositories found") {
		t.Fatalf("expected no repos error, got %v", err)
	}
}

func TestRunEject_JSONSuccessAndError(t *testing.T) {
	resetRepoWrapperFlags(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	sourceDir := setupSourceDir(t, map[string]string{"shared.txt": "hello"})
	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeCopy,
		Items: []config.Item{{
			Type:       config.ItemTypeFile,
			SourcePath: "shared.txt",
			TargetPath: ".repomni/shared.txt",
			Enabled:    true,
		}},
	}
	saveGlobalConfig(t, cfg)

	repoDir := filepath.Join(t.TempDir(), "repo-success")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)
	if _, err := injector.Inject(cfg, repoDir, injector.Options{Mode: config.ModeCopy}); err != nil {
		t.Fatalf("injector.Inject() error: %v", err)
	}

	ejectJSON = true
	ejectOut := captureStdout(t, func() {
		if err := runEject(ejectCmd, []string{repoDir}); err != nil {
			t.Fatalf("runEject() error: %v", err)
		}
	})

	var ejectResults []ejectJSONResult
	if err := json.Unmarshal([]byte(ejectOut), &ejectResults); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, ejectOut)
	}
	if len(ejectResults) != 1 || ejectResults[0].Action != "removed" {
		t.Fatalf("unexpected eject results: %+v", ejectResults)
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".repomni", "shared.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected injected file to be removed, stat err=%v", err)
	}

	repoErr := filepath.Join(t.TempDir(), "repo-error")
	if err := os.MkdirAll(filepath.Join(repoErr, ".repomni"), 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoErr)
	if err := os.WriteFile(filepath.Join(repoErr, ".repomni", "shared.txt"), []byte("manual"), 0644); err != nil {
		t.Fatal(err)
	}

	ejectOut = captureStdout(t, func() {
		err := runEject(ejectCmd, []string{repoErr})
		if err == nil || !strings.Contains(err.Error(), "some items had errors") {
			t.Fatalf("expected aggregate eject error, got %v", err)
		}
	})

	ejectResults = nil
	if err := json.Unmarshal([]byte(ejectOut), &ejectResults); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, ejectOut)
	}
	if len(ejectResults) != 1 {
		t.Fatalf("unexpected eject results: %+v", ejectResults)
	}
	if !strings.Contains(ejectResults[0].Detail, "manifest missing") {
		t.Fatalf("unexpected eject detail: %+v", ejectResults[0])
	}
}

func TestRunStatus_GitJSONAndErrors(t *testing.T) {
	resetRepoWrapperFlags(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	statusGit = true
	statusJSON = true
	statusNoFetch = true

	gitOut := captureStdout(t, func() {
		if err := runStatus(statusCmd, []string{repoDir}); err != nil {
			t.Fatalf("runStatus(git) error: %v", err)
		}
	})

	var statuses []syncer.RepoStatus
	if err := json.Unmarshal([]byte(gitOut), &statuses); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, gitOut)
	}
	if len(statuses) != 1 || statuses[0].State != syncer.StateNoUpstream {
		t.Fatalf("unexpected git statuses: %+v", statuses)
	}

	sourceDir := setupSourceDir(t, map[string]string{"shared.txt": "hello"})
	saveGlobalConfig(t, &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeCopy,
		Items: []config.Item{{
			Type:       config.ItemTypeFile,
			SourcePath: "shared.txt",
			TargetPath: ".repomni/shared.txt",
			Enabled:    true,
		}},
	})

	statusGit = false
	statusJSON = true
	nonRepo := t.TempDir()
	stdout, stderr := captureStdoutAndStderr(t, func() {
		if err := runStatus(statusCmd, []string{nonRepo}); err != nil {
			t.Fatalf("runStatus(non-repo) error: %v", err)
		}
	})

	var outputs []jsonStatusOutput
	if err := json.Unmarshal([]byte(stdout), &outputs); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, stdout)
	}
	if len(outputs) != 0 {
		t.Fatalf("expected no status outputs for non-repo, got %+v", outputs)
	}
	if !strings.Contains(stderr, "Error checking") {
		t.Fatalf("expected stderr error message, got %q", stderr)
	}
}

func TestRunStatus_AllNoRepos(t *testing.T) {
	resetRepoWrapperFlags(t)
	statusAll = true
	err := runStatus(statusCmd, []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "no git repositories found") {
		t.Fatalf("expected no repos error, got %v", err)
	}
}

func TestRunSyncState_JSONUpdatesAndDryRun(t *testing.T) {
	resetRepoWrapperFlags(t)
	installFakeForgeCLIs(t)

	parentDir := t.TempDir()

	repoGH := filepath.Join(parentDir, "repo-gh")
	repoGL := filepath.Join(parentDir, "repo-gl")
	repoFail := filepath.Join(parentDir, "repo-fail")
	for _, repo := range []string{repoGH, repoGL, repoFail} {
		if err := os.MkdirAll(repo, 0755); err != nil {
			t.Fatal(err)
		}
		initGitRepo(t, repo)
	}

	gitDirGH := filepath.Join(repoGH, ".git")
	gitDirGL := filepath.Join(repoGL, ".git")
	gitDirFail := filepath.Join(repoFail, ".git")
	if err := repoconfig.Save(gitDirGH, &repoconfig.RepoConfig{Version: 1, State: "review", MergeURL: "https://github.com/org/repo/pull/1"}); err != nil {
		t.Fatal(err)
	}
	if err := repoconfig.Save(gitDirGL, &repoconfig.RepoConfig{Version: 1, State: "approved", MergeURL: "https://gitlab.com/group/project/-/merge_requests/2"}); err != nil {
		t.Fatal(err)
	}
	if err := repoconfig.Save(gitDirFail, &repoconfig.RepoConfig{Version: 1, State: "review", MergeURL: "https://github.com/org/repo/pull/fail"}); err != nil {
		t.Fatal(err)
	}

	syncStateJSON = true
	jsonOut := captureStdout(t, func() {
		if err := runSyncState(syncStateCmd, []string{parentDir}); err != nil {
			t.Fatalf("runSyncState() error: %v", err)
		}
	})

	var payload struct {
		Results []mergestatus.Result `json:"results"`
		Summary mergestatus.Summary  `json:"summary"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput=%s", err, jsonOut)
	}
	if payload.Summary.Total != 3 || payload.Summary.Updated != 1 || payload.Summary.Unchanged != 1 || payload.Summary.Errors != 1 {
		t.Fatalf("unexpected summary: %+v", payload.Summary)
	}

	cfgGH, err := repoconfig.Load(gitDirGH)
	if err != nil {
		t.Fatal(err)
	}
	if cfgGH.State != "approved" {
		t.Fatalf("GitHub repo state = %q, want approved", cfgGH.State)
	}
	cfgGL, err := repoconfig.Load(gitDirGL)
	if err != nil {
		t.Fatal(err)
	}
	if cfgGL.State != "approved" {
		t.Fatalf("GitLab repo state = %q, want approved", cfgGL.State)
	}
	cfgFail, err := repoconfig.Load(gitDirFail)
	if err != nil {
		t.Fatal(err)
	}
	if cfgFail.State != "review" {
		t.Fatalf("failed repo state = %q, want review", cfgFail.State)
	}

	dryParent := t.TempDir()
	repoDry := filepath.Join(dryParent, "repo-dry")
	if err := os.MkdirAll(repoDry, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDry)
	gitDirDry := filepath.Join(repoDry, ".git")
	if err := repoconfig.Save(gitDirDry, &repoconfig.RepoConfig{Version: 1, State: "review", MergeURL: "https://github.com/org/repo/pull/2"}); err != nil {
		t.Fatal(err)
	}

	syncStateJSON = false
	syncStateDryRun = true
	dryOut := captureStdout(t, func() {
		if err := runSyncState(syncStateCmd, []string{dryParent}); err != nil {
			t.Fatalf("runSyncState(dry-run) error: %v", err)
		}
	})
	if !strings.Contains(dryOut, "Dry run -- no changes were made.") {
		t.Fatalf("expected dry-run message, got %q", dryOut)
	}
	cfgDry, err := repoconfig.Load(gitDirDry)
	if err != nil {
		t.Fatal(err)
	}
	if cfgDry.State != "review" {
		t.Fatalf("dry-run should not persist state changes, got %q", cfgDry.State)
	}
}

func TestRunSyncState_NoReposAndNoEligibleRepos(t *testing.T) {
	resetRepoWrapperFlags(t)
	installFakeForgeCLIs(t)

	err := runSyncState(syncStateCmd, []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "no git repositories found") {
		t.Fatalf("expected no repos error, got %v", err)
	}

	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)
	if err := repoconfig.Save(filepath.Join(repoDir, ".git"), &repoconfig.RepoConfig{Version: 1, State: "merged", MergeURL: "https://github.com/org/repo/pull/99"}); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := runSyncState(syncStateCmd, []string{parentDir}); err != nil {
			t.Fatalf("runSyncState(no-eligible) error: %v", err)
		}
	})
	if !strings.Contains(out, "No repos with active merge requests found.") {
		t.Fatalf("expected no eligible repos message, got %q", out)
	}
}

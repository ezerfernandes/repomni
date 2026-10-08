package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

// setupBranchEnv creates a bare origin, a parent clone of it with each of
// branches pushed (one commit each), and an empty workdir inside the parent.
func setupBranchEnv(t *testing.T, branches ...string) (originDir, parentDir, workDir string) {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(t.TempDir())

	originDir = filepath.Join(dir, "origin.git")
	runGitCmd(t, dir, "init", "--bare", originDir)
	parentDir = filepath.Join(dir, "parent")
	runGitCmd(t, dir, "clone", originDir, parentDir)
	runGitCmd(t, parentDir, "commit", "--allow-empty", "-m", "init")
	runGitCmd(t, parentDir, "push", "origin", "HEAD")
	for _, b := range branches {
		runGitCmd(t, parentDir, "checkout", "-b", b)
		runGitCmd(t, parentDir, "commit", "--allow-empty", "-m", b)
		runGitCmd(t, parentDir, "push", "origin", b)
		runGitCmd(t, parentDir, "checkout", "-")
	}

	workDir = filepath.Join(parentDir, "workdir")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	return originDir, parentDir, workDir
}

// resetCloneFlags sets clone flags for a test and restores them afterwards.
func resetCloneFlags(t *testing.T) {
	t.Helper()
	origNoInject, origTicket, origJSON := cloneNoInject, cloneTicket, cloneJSON
	t.Cleanup(func() {
		cloneNoInject, cloneTicket, cloneJSON = origNoInject, origTicket, origJSON
	})
	cloneNoInject, cloneTicket, cloneJSON = true, "", false
}

// stubForgeOutput stubs forge calls to return out, recording each call's args.
func stubForgeOutput(t *testing.T, out string) *[][]string {
	t.Helper()
	origCheckCLI, origRunDir := forge.CheckCLI, forge.RunForgeDir
	t.Cleanup(func() { forge.CheckCLI, forge.RunForgeDir = origCheckCLI, origRunDir })

	var calls [][]string
	forge.CheckCLI = func(forge.Platform) error { return nil }
	forge.RunForgeDir = func(dir string, platform forge.Platform, args ...string) (string, error) {
		calls = append(calls, args)
		return out, nil
	}
	return &calls
}

func mustLoadRepoConfig(t *testing.T, repoDir string) *repoconfig.RepoConfig {
	t.Helper()
	gitDir, err := gitutil.FindGitDir(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := repoconfig.Load(gitDir)
	if err != nil || cfg == nil {
		t.Fatalf("cannot load repo config: %v", err)
	}
	return cfg
}

func TestRunClone_FromGitHubPRURL(t *testing.T) {
	_, _, workDir := setupBranchEnv(t, "feature/pr-branch")
	resetCloneFlags(t)
	chdirForTest(t, workDir)
	calls := stubForgeOutput(t, `{"url":"https://github.com/org/repo/pull/42","number":42,"state":"OPEN",
		"isDraft":true,"baseRefName":"main","headRefName":"feature/pr-branch","isCrossRepository":false}`)

	prURL := "https://github.com/org/repo/pull/42"
	out := captureStdout(t, func() {
		if err := runClone(cloneCmd, []string{prURL}); err != nil {
			t.Fatalf("runClone: %v", err)
		}
	})

	if len(*calls) != 1 || (*calls)[0][0] != "pr" || (*calls)[0][2] != prURL {
		t.Fatalf("forge calls = %v, want one pr view of %s", *calls, prURL)
	}

	targetDir := filepath.Join(workDir, "feature-pr-branch")
	branch, err := gitutil.CurrentBranch(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/pr-branch" {
		t.Errorf("current branch = %q, want %q", branch, "feature/pr-branch")
	}

	cfg := mustLoadRepoConfig(t, targetDir)
	if cfg.MergeURL != prURL || cfg.MergeNumber != 42 || cfg.BaseBranch != "main" || !cfg.Draft {
		t.Errorf("merge config = %+v, want PR 42 against main, draft", cfg)
	}
	if cfg.State != string(repoconfig.StateReview) || !cfg.Remote {
		t.Errorf("state = %q remote = %v, want review/true", cfg.State, cfg.Remote)
	}
	if !strings.Contains(out, "Attached: "+prURL) {
		t.Errorf("output %q should report the attached PR", out)
	}
}

func TestRunClone_FromGitLabMRURL(t *testing.T) {
	_, _, workDir := setupBranchEnv(t, "mr-branch")
	resetCloneFlags(t)
	chdirForTest(t, workDir)
	calls := stubForgeOutput(t, `{"web_url":"https://gitlab.com/group/project/-/merge_requests/7","iid":7,
		"state":"opened","target_branch":"main","source_branch":"mr-branch",
		"source_project_id":10,"target_project_id":10,"approved":true}`)

	mrURL := "https://gitlab.com/group/project/-/merge_requests/7"
	captureStdout(t, func() {
		if err := runClone(cloneCmd, []string{mrURL}); err != nil {
			t.Fatalf("runClone: %v", err)
		}
	})

	if got := strings.Join((*calls)[0], " "); got != "mr view 7 --output json" {
		t.Errorf("forge args = %q, want %q", got, "mr view 7 --output json")
	}
	cfg := mustLoadRepoConfig(t, filepath.Join(workDir, "mr-branch"))
	if cfg.MergeNumber != 7 || cfg.State != string(repoconfig.StateApproved) {
		t.Errorf("merge config = %+v, want MR 7 approved", cfg)
	}
}

func TestRunClone_PRURLErrors(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		origin    string // origin URL override for the parent repo
		forgeOut  string
		wantErr   string
		wantCalls int
	}{
		{
			name:    "not a PR URL",
			url:     "https://github.com/org/repo",
			wantErr: "not a pull/merge request URL",
		},
		{
			name:    "PR from another repo",
			url:     "https://github.com/org/other/pull/1",
			origin:  "https://github.com/org/repo.git",
			wantErr: "belongs to org/other",
		},
		{
			name: "fork PR",
			url:  "https://github.com/org/repo/pull/1",
			forgeOut: `{"number":1,"state":"OPEN","baseRefName":"main",
				"headRefName":"main","isCrossRepository":true}`,
			wantErr:   "comes from a fork",
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, parentDir, workDir := setupBranchEnv(t)
			if tt.origin != "" {
				runGitCmd(t, parentDir, "remote", "set-url", "origin", tt.origin)
			}
			resetCloneFlags(t)
			chdirForTest(t, workDir)
			calls := stubForgeOutput(t, tt.forgeOut)

			err := runClone(cloneCmd, []string{tt.url})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
			}
			if len(*calls) != tt.wantCalls {
				t.Errorf("forge calls = %d, want %d", len(*calls), tt.wantCalls)
			}
			if entries, _ := os.ReadDir(workDir); len(entries) != 0 {
				t.Errorf("workdir should stay empty, has %d entries", len(entries))
			}
		})
	}
}

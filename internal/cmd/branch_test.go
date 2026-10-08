package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/gitutil"
)

func TestRunCreate_WithBase(t *testing.T) {
	_, parentDir, workDir := setupBranchEnv(t, "develop")
	chdirForTest(t, workDir)

	origNoInject, origTicket, origBase, origJSON := createNoInject, createTicket, createBase, createJSON
	t.Cleanup(func() {
		createNoInject, createTicket, createBase, createJSON = origNoInject, origTicket, origBase, origJSON
	})
	createNoInject, createTicket, createBase, createJSON = true, "", "develop", true

	out := captureStdout(t, func() {
		if err := runCreate(createCmd, []string{"my-fix"}); err != nil {
			t.Fatalf("runCreate: %v", err)
		}
	})
	if !strings.Contains(out, `"base": "develop"`) {
		t.Errorf("JSON output %q should include the base", out)
	}

	targetDir := filepath.Join(workDir, "my-fix")
	developTip, _ := gitutil.RunGit(parentDir, "rev-parse", "develop")
	if head, _ := gitutil.RunGit(targetDir, "rev-parse", "HEAD"); head != developTip {
		t.Errorf("HEAD = %s, want develop tip %s", head, developTip)
	}
	if cfg := mustLoadRepoConfig(t, targetDir); cfg.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want %q", cfg.BaseBranch, "develop")
	}
}

package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Mark a draft PR/MR as ready for review",
	Args:  cobra.NoArgs,
	RunE:  runReady,
}

func init() {
	branchCmd.AddCommand(readyCmd)
}

func runReady(cmd *cobra.Command, args []string) error {
	repoRoot, err := gitutil.RunGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	gitDir, err := gitutil.FindGitDir(repoRoot)
	if err != nil {
		return err
	}

	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		return err
	}
	if cfg == nil || cfg.MergeURL == "" {
		return fmt.Errorf("no PR/MR attached; use \"branch submit\" or \"branch attach\" first")
	}

	platform := mergestatus.DetectPlatform(cfg.MergeURL)
	if err := forge.CheckCLI(platform); err != nil {
		return err
	}

	var readyArgs []string
	switch platform {
	case forge.PlatformGitHub:
		readyArgs = []string{"pr", "ready"}
	case forge.PlatformGitLab:
		readyArgs = []string{"mr", "update", fmt.Sprintf("%d", resolveMergeNumber(cfg)), "--ready"}
	}

	if _, err := forge.RunForgeDir(repoRoot, platform, readyArgs...); err != nil {
		return fmt.Errorf("failed to mark as ready: %w", err)
	}

	cfg.Draft = false
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	fmt.Println("PR/MR marked as ready for review.")
	return nil
}

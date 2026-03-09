package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the attached PR/MR in a browser",
	Args:  cobra.NoArgs,
	RunE:  runOpen,
}

func init() {
	branchCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
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

	var viewArgs []string
	switch platform {
	case forge.PlatformGitHub:
		viewArgs = []string{"pr", "view", "--web", cfg.MergeURL}
	case forge.PlatformGitLab:
		viewArgs = []string{"mr", "view", fmt.Sprintf("%d", resolveMergeNumber(cfg)), "--web"}
	}

	_, err = forge.RunForgeDir(repoRoot, platform, viewArgs...)
	return err
}

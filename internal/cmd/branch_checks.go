package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var checksCmd = &cobra.Command{
	Use:   "checks",
	Short: "Show CI check status for the attached PR/MR",
	Long: `Display the status of CI checks for the pull/merge request attached to
the current branch repo.

  repomni branch checks
  repomni branch checks --watch`,
	Args: cobra.NoArgs,
	RunE: runChecks,
}

var checksWatch bool

func init() {
	branchCmd.AddCommand(checksCmd)
	checksCmd.Flags().BoolVar(&checksWatch, "watch", false, "poll until checks complete")
}

func runChecks(cmd *cobra.Command, args []string) error {
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

	if checksWatch {
		return runChecksWatch(repoRoot, platform)
	}
	return runChecksOnce(repoRoot, platform)
}

func runChecksOnce(repoRoot string, platform forge.Platform) error {
	var checkArgs []string
	switch platform {
	case forge.PlatformGitHub:
		checkArgs = []string{"pr", "checks"}
	case forge.PlatformGitLab:
		checkArgs = []string{"ci", "status"}
	}

	out, err := forge.RunForgeDir(repoRoot, platform, checkArgs...)
	if err != nil {
		return fmt.Errorf("failed to get checks: %w", err)
	}
	fmt.Println(out)
	return nil
}

func runChecksWatch(repoRoot string, platform forge.Platform) error {
	var watchArgs []string
	switch platform {
	case forge.PlatformGitHub:
		watchArgs = []string{"pr", "checks", "--watch"}
	case forge.PlatformGitLab:
		watchArgs = []string{"ci", "status", "--live"}
	}

	return forge.RunForgePassthrough(repoRoot, platform, watchArgs...)
}

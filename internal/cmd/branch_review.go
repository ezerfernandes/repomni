package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Submit a review on the attached PR/MR",
	Long: `Submit a review on the pull/merge request attached to the current branch repo.

  repomni branch review --approve
  repomni branch review --comment "looks good"`,
	Args: cobra.NoArgs,
	RunE: runReview,
}

var (
	reviewApprove bool
	reviewComment string
)

func init() {
	branchCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().BoolVar(&reviewApprove, "approve", false, "approve the PR/MR")
	reviewCmd.Flags().StringVar(&reviewComment, "comment", "", "leave a review comment")
}

func runReview(cmd *cobra.Command, args []string) error {
	if !reviewApprove && reviewComment == "" {
		return fmt.Errorf("provide --approve and/or --comment")
	}

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

	switch platform {
	case forge.PlatformGitHub:
		return reviewGitHub(repoRoot, cfg)
	case forge.PlatformGitLab:
		return reviewGitLab(repoRoot, cfg)
	}
	return nil
}

func reviewGitHub(repoRoot string, _ *repoconfig.RepoConfig) error {
	if reviewApprove && reviewComment != "" {
		// gh pr review supports both at once
		_, err := forge.RunForgeDir(repoRoot, forge.PlatformGitHub,
			"pr", "review", "--approve", "--body", reviewComment)
		if err != nil {
			return fmt.Errorf("review failed: %w", err)
		}
		fmt.Println("Approved with comment.")
		return nil
	}
	if reviewApprove {
		if _, err := forge.RunForgeDir(repoRoot, forge.PlatformGitHub,
			"pr", "review", "--approve"); err != nil {
			return fmt.Errorf("approve failed: %w", err)
		}
		fmt.Println("Approved.")
		return nil
	}
	if _, err := forge.RunForgeDir(repoRoot, forge.PlatformGitHub,
		"pr", "review", "--comment", "--body", reviewComment); err != nil {
		return fmt.Errorf("comment failed: %w", err)
	}
	fmt.Println("Comment submitted.")
	return nil
}

func reviewGitLab(repoRoot string, cfg *repoconfig.RepoConfig) error {
	mrID := fmt.Sprintf("%d", resolveMergeNumber(cfg))

	if reviewApprove {
		if _, err := forge.RunForgeDir(repoRoot, forge.PlatformGitLab,
			"mr", "approve", mrID); err != nil {
			return fmt.Errorf("approve failed: %w", err)
		}
		fmt.Println("Approved.")
	}
	if reviewComment != "" {
		if _, err := forge.RunForgeDir(repoRoot, forge.PlatformGitLab,
			"mr", "note", mrID, "--message", reviewComment); err != nil {
			return fmt.Errorf("comment failed: %w", err)
		}
		fmt.Println("Comment submitted.")
	}
	return nil
}

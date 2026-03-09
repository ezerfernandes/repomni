package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge the attached PR/MR",
	Long: `Merge the pull/merge request attached to the current branch repo.

  repomni branch merge --squash
  repomni branch merge --rebase --delete-branch`,
	Args: cobra.NoArgs,
	RunE: runMerge,
}

var (
	mergeSquash       bool
	mergeRebase       bool
	mergeDeleteBranch bool
)

func init() {
	branchCmd.AddCommand(mergeCmd)
	mergeCmd.Flags().BoolVar(&mergeSquash, "squash", false, "squash commits before merging")
	mergeCmd.Flags().BoolVar(&mergeRebase, "rebase", false, "rebase before merging")
	mergeCmd.Flags().BoolVar(&mergeDeleteBranch, "delete-branch", false, "delete the branch after merging")
}

func runMerge(cmd *cobra.Command, args []string) error {
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

	mergeArgs := buildMergeArgs(platform)

	if _, err := forge.RunForgeDir(repoRoot, platform, mergeArgs...); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	cfg.State = string(repoconfig.StateMerged)
	cfg.Draft = false
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	fmt.Println("Merged.")
	return nil
}

func buildMergeArgs(platform forge.Platform) []string {
	var args []string
	if platform == forge.PlatformGitHub {
		args = []string{"pr", "merge"}
		if mergeSquash {
			args = append(args, "--squash")
		}
		if mergeRebase {
			args = append(args, "--rebase")
		}
		if mergeDeleteBranch {
			args = append(args, "--delete-branch")
		}
	} else {
		args = []string{"mr", "merge"}
		if mergeSquash {
			args = append(args, "--squash")
		}
		if mergeRebase {
			args = append(args, "--rebase")
		}
		if mergeDeleteBranch {
			args = append(args, "--remove-source-branch")
		}
	}
	return args
}

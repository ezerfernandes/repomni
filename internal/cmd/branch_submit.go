package cmd

import (
	"fmt"
	"strings"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Create a pull/merge request for the current branch",
	Long: `Push the current branch and create a pull request (GitHub) or merge request
(GitLab) using the gh or glab CLI.

The platform is detected from the origin remote URL. After creation the PR/MR
URL is stored in .git/repomni/config.yaml and the workflow state is set to
"review".

  repomni branch submit --fill --draft --reviewer alice,bob`,
	Args: cobra.NoArgs,
	RunE: runSubmit,
}

var (
	submitFill      bool
	submitDraft     bool
	submitReviewers []string
	submitBase      string
	submitTitle     string
	submitBody      string
)

func init() {
	branchCmd.AddCommand(submitCmd)
	submitCmd.Flags().BoolVar(&submitFill, "fill", false, "auto-fill title and body from commits")
	submitCmd.Flags().BoolVar(&submitDraft, "draft", false, "create as draft")
	submitCmd.Flags().StringSliceVar(&submitReviewers, "reviewer", nil, "reviewers (comma-separated or repeated)")
	submitCmd.Flags().StringVar(&submitBase, "base", "", "base/target branch")
	submitCmd.Flags().StringVar(&submitTitle, "title", "", "PR/MR title")
	submitCmd.Flags().StringVar(&submitBody, "body", "", "PR/MR body")
}

func runSubmit(cmd *cobra.Command, args []string) error {
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
	if cfg == nil {
		cfg = &repoconfig.RepoConfig{Version: 1}
	}

	if cfg.MergeURL != "" {
		return fmt.Errorf("PR/MR already exists: %s\nUse \"branch open\" to view it", cfg.MergeURL)
	}

	branch, err := gitutil.CurrentBranch(repoRoot)
	if err != nil {
		return err
	}
	if branch == "" {
		return fmt.Errorf("HEAD is detached; checkout a branch first")
	}
	if branch == "main" || branch == "master" {
		return fmt.Errorf("cannot submit from %s; create a feature branch first", branch)
	}

	platform, err := forge.DetectPlatformFromRemote(repoRoot)
	if err != nil {
		return err
	}
	if err := forge.CheckCLI(platform); err != nil {
		return err
	}

	// Push the branch
	if _, err := gitutil.RunGit(repoRoot, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Build create args
	createArgs := buildCreateArgs(platform)

	out, err := forge.RunForgeDir(repoRoot, platform, createArgs...)
	if err != nil {
		return fmt.Errorf("failed to create PR/MR: %w", err)
	}

	// gh/glab print the URL as the last non-empty line
	mergeURL := lastNonEmptyLine(out)
	if mergeURL == "" {
		return fmt.Errorf("could not parse PR/MR URL from output:\n%s", out)
	}

	cfg.State = string(repoconfig.StateReview)
	cfg.MergeURL = mergeURL
	cfg.MergeNumber = forge.ParseMergeNumber(mergeURL)
	cfg.BaseBranch = submitBase
	cfg.Draft = submitDraft

	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	fmt.Printf("Created: %s\n", mergeURL)
	if submitDraft {
		fmt.Println("Status: draft")
	}
	return nil
}

func buildCreateArgs(platform forge.Platform) []string {
	var args []string
	if platform == forge.PlatformGitHub {
		args = []string{"pr", "create"}
		if submitBase != "" {
			args = append(args, "--base", submitBase)
		}
	} else {
		args = []string{"mr", "create"}
		if submitBase != "" {
			args = append(args, "--target-branch", submitBase)
		}
	}

	if submitFill {
		args = append(args, "--fill")
	}
	if submitDraft {
		args = append(args, "--draft")
	}
	if submitTitle != "" {
		args = append(args, "--title", submitTitle)
	}
	if submitBody != "" {
		if platform == forge.PlatformGitHub {
			args = append(args, "--body", submitBody)
		} else {
			args = append(args, "--description", submitBody)
		}
	}
	for _, r := range submitReviewers {
		args = append(args, "--reviewer", r)
	}
	return args
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

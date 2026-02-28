package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var branchesCmd = &cobra.Command{
	Use:   "branches [directory]",
	Short: "List branch repos with their workflow states",
	Long: `List all git repositories that are immediate subdirectories of the target
directory, showing each repo's directory name, git branch, and workflow state.

States are color-coded: active (green), review (yellow), approved (lime green),
review-blocked (red), merged (purple), closed (red), done (gray), paused (blue).

If no directory is specified, the current directory is used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBranches,
}

var (
	branchesState string
	branchesJSON  bool
)

func init() {
	rootCmd.AddCommand(branchesCmd)
	branchesCmd.Flags().StringVar(&branchesState, "state", "",
		"filter by workflow state (e.g., active, review, done, paused)")
	branchesCmd.Flags().BoolVar(&branchesJSON, "json", false, "output as JSON")
}

func runBranches(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	repos, err := gitutil.FindGitRepos(target)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no git repositories found under %s", target)
	}

	var infos []ui.BranchInfo
	for _, repo := range repos {
		info := collectBranchInfo(repo)

		if branchesState != "" && info.State != branchesState {
			continue
		}

		infos = append(infos, info)
	}

	if len(infos) == 0 && branchesState != "" {
		return fmt.Errorf("no repos with state %q found", branchesState)
	}

	if branchesJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	}

	ui.PrintBranchesTable(infos)
	return nil
}

func collectBranchInfo(repoPath string) ui.BranchInfo {
	info := ui.BranchInfo{
		Path: repoPath,
		Name: filepath.Base(repoPath),
	}

	branch, err := gitutil.CurrentBranch(repoPath)
	if err == nil && branch != "" {
		info.Branch = branch
	} else {
		info.Branch = "(detached)"
	}

	gitDir, err := gitutil.FindGitDir(repoPath)
	if err == nil {
		cfg, _ := repoconfig.Load(gitDir)
		if cfg != nil {
			info.State = cfg.State
			info.MergeURL = cfg.MergeURL
			info.Remote = cfg.Remote
		}
	}

	dirty, err := gitutil.IsDirty(repoPath)
	if err == nil {
		info.Dirty = dirty
	}

	return info
}

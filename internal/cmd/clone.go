package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ezerfernandes/repomni/internal/brancher"
	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/scripter"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <branch-name | pr-url>",
	Short: "Clone the parent repo and check out an existing remote branch",
	Long: `Finds the closest parent directory that is a git repository, clones it
into the current directory, and checks out an existing remote branch.

The local directory name is derived from the branch name by replacing
forbidden characters (such as '/') with '-'. For example,
"feature/my-thing" becomes "feature-my-thing".

Instead of a branch name, a pull request (GitHub) or merge request (GitLab)
URL can be given. Its source branch is looked up with gh or glab, cloned as
above, and the PR/MR is attached to the new clone:

  repomni branch clone https://github.com/org/repo/pull/42

PRs/MRs opened from forks are not supported, since their branch is not on
origin.

This is useful for creating isolated working copies for existing branches.`,
	Args: cobra.ExactArgs(1),
	RunE: runClone,
}

var (
	cloneNoInject bool
	cloneTicket   string
	cloneJSON     bool
)

func init() {
	branchCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().BoolVar(&cloneNoInject, "no-inject", false, "skip automatic injection into the new clone")
	cloneCmd.Flags().StringVar(&cloneTicket, "ticket", "", "associate a ticket identifier (e.g., PROJ-123)")
	cloneCmd.Flags().BoolVar(&cloneJSON, "json", false, "output as JSON")
}

func runClone(cmd *cobra.Command, args []string) error {
	branchName := args[0]
	var mergeURL string
	var mergeInfo *mergeRequestInfo
	if isURLArg(args[0]) {
		mergeURL = args[0]
		info, err := resolveMergeBranch(mergeURL)
		if err != nil {
			return err
		}
		mergeInfo = info
		branchName = info.HeadBranch
	}

	result, err := brancher.Clone(".", branchName)
	if err != nil {
		return err
	}
	if !cloneJSON {
		fmt.Printf("Cloned %s into %s\n", result.RemoteURL, result.TargetDir)
		fmt.Printf("Checked out branch: %s\n", result.Branch)
	}

	state := string(repoconfig.StateActive)
	if mergeInfo != nil {
		state = mergeInfo.State
	}

	parentGitDir, parentGitDirErr := gitutil.FindGitDir(result.ParentRepo)

	// Auto-inject into the new repo unless --no-inject is set.
	if !cloneNoInject {
		if err := autoInject(result, parentGitDir, parentGitDirErr, cloneJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: auto-injection failed: %v\n", err)
		}
	}

	// Set initial workflow state for the new branch.
	if newGitDir, err := gitutil.FindGitDir(result.TargetDir); err == nil {
		newCfg, _ := repoconfig.Load(newGitDir)
		if newCfg == nil {
			newCfg = &repoconfig.RepoConfig{Version: 1}
		}
		newCfg.State = string(repoconfig.StateActive)
		newCfg.Remote = true
		newCfg.Ticket = cloneTicket
		if mergeInfo != nil {
			populateMergeConfig(newCfg, mergeURL, mergeInfo)
		}
		if err := repoconfig.Save(newGitDir, newCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set initial state: %v\n", err)
		} else if mergeInfo != nil && !cloneJSON {
			fmt.Printf("Attached: %s (state: %s)\n", mergeURL, state)
		}
	}

	// Run setup script if configured in the parent repo.
	if parentGitDirErr == nil {
		if _, exists := scripter.GetScript(parentGitDir, scripter.ScriptSetup); exists {
			if !cloneJSON {
				fmt.Println("Running setup script...")
			}
			if err := scripter.RunScript(parentGitDir, scripter.ScriptSetup, result.TargetDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: setup script failed: %v\n", err)
			}
		}
	}

	if cloneJSON {
		return ui.PrintJSON(branchResult{
			Path:       result.TargetDir,
			Branch:     result.Branch,
			RemoteURL:  result.RemoteURL,
			ParentRepo: result.ParentRepo,
			State:      state,
			Remote:     true,
			Ticket:     cloneTicket,
			MergeURL:   mergeURL,
		})
	}

	return nil
}

// isURLArg reports whether arg looks like a URL rather than a branch name.
func isURLArg(arg string) bool {
	return strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://")
}

// resolveMergeBranch looks up the PR/MR at mergeURL and returns its details,
// including the source branch to check out. The PR/MR must belong to the parent
// repo's origin and its branch must live there (not in a fork).
func resolveMergeBranch(mergeURL string) (*mergeRequestInfo, error) {
	if err := validateMergeURL(mergeURL); err != nil {
		return nil, err
	}
	if forge.ParseMergeNumber(mergeURL) == 0 {
		return nil, fmt.Errorf("not a pull/merge request URL: %s", mergeURL)
	}

	parentRepo, err := brancher.FindParentGitRepo(".")
	if err != nil {
		return nil, err
	}
	originURL, err := gitutil.RunGit(parentRepo, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("cannot get remote URL from %s: %w", parentRepo, err)
	}
	mergeSlug := forge.RepoSlugFromMergeURL(mergeURL)
	originSlug := forge.RepoSlugFromRemote(originURL)
	if mergeSlug != "" && originSlug != "" && mergeSlug != originSlug {
		return nil, fmt.Errorf("%s belongs to %s, but the parent repo's origin is %s", mergeURL, mergeSlug, originSlug)
	}

	platform := mergestatus.DetectPlatform(mergeURL)
	if err := forge.CheckCLI(platform); err != nil {
		return nil, err
	}
	info, err := queryMergeRequest(platform, mergeURL, parentRepo)
	if err != nil {
		return nil, err
	}
	if info.CrossRepo {
		return nil, fmt.Errorf("%s comes from a fork; its branch %q is not on origin", mergeURL, info.HeadBranch)
	}
	if info.HeadBranch == "" {
		return nil, fmt.Errorf("cannot determine the source branch of %s", mergeURL)
	}
	return info, nil
}

package cmd

import (
	"fmt"
	"net/url"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
	"github.com/spf13/cobra"
)

var setStateCmd = &cobra.Command{
	Use:   "set-state <state> [url]",
	Short: "Set the workflow state for the current branch repo",
	Long: `Set a workflow state label for the current repository. The state is stored
in .git/repoinjector/config.yaml and displayed by the "branches" command.

Predefined states: active, review, approved, review-blocked, merged, closed, done, paused.
Custom states are also accepted (lowercase letters, digits, hyphens).

When setting state to "review", you may provide a PR/MR URL as the second argument.
This URL is stored and used by the "sync-state" command to track PR/MR status.

  repoinjector set-state review https://github.com/org/repo/pull/42

Use "set-state --clear" to remove the state and merge URL.`,
	Args: cobra.MaximumNArgs(2),
	RunE: runSetState,
}

var setStateClear bool

func init() {
	rootCmd.AddCommand(setStateCmd)
	setStateCmd.Flags().BoolVar(&setStateClear, "clear", false, "remove the workflow state and merge URL")
}

func runSetState(cmd *cobra.Command, args []string) error {
	if !setStateClear && len(args) == 0 {
		return fmt.Errorf("provide a state name, or use --clear to remove")
	}

	if len(args) == 2 && args[0] != "review" {
		return fmt.Errorf("merge URL can only be provided when state is \"review\"")
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
	if cfg == nil {
		cfg = &repoconfig.RepoConfig{Version: 1}
	}

	if setStateClear {
		cfg.State = ""
		cfg.MergeURL = ""
		if err := repoconfig.Save(gitDir, cfg); err != nil {
			return err
		}
		fmt.Println("State and merge URL cleared.")
		return nil
	}

	state := args[0]
	if err := repoconfig.ValidateState(state); err != nil {
		return err
	}

	cfg.State = state

	if len(args) == 2 {
		mergeURL := args[1]
		if err := validateMergeURL(mergeURL); err != nil {
			return err
		}
		cfg.MergeURL = mergeURL
	}

	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	if cfg.MergeURL != "" {
		fmt.Printf("State set to: %s (merge URL: %s)\n", state, cfg.MergeURL)
	} else {
		fmt.Printf("State set to: %s\n", state)
	}
	return nil
}

func validateMergeURL(u string) error {
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("merge URL must use http or https scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("merge URL must include a host")
	}
	return nil
}

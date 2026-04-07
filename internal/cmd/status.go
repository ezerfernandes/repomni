package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/injector"
	"github.com/ezerfernandes/repomni/internal/syncer"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [target]",
	Short: "Show injection or git status of target repo(s)",
	Long: `Check whether configured files are present, current, and excluded from git
in the target repository.

Use --git to show git sync status (branch, ahead/behind, dirty) instead of
injection status.

If no target is specified, the current directory is used.
Use --all to check all git repos under the target directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

var (
	statusAll     bool
	statusJSON    bool
	statusGit     bool
	statusNoFetch bool
)

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&statusAll, "all", false, "check all git repos under target directory")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "output as JSON")
	statusCmd.Flags().BoolVar(&statusGit, "git", false, "show git sync status (branch, ahead/behind, dirty)")
	statusCmd.Flags().BoolVar(&statusNoFetch, "no-fetch", false, "skip git fetch when checking git status")
}

type jsonStatusOutput struct {
	Repo  string           `json:"repo"`
	Items []jsonItemStatus `json:"items"`
}

type jsonItemStatus struct {
	TargetPath string `json:"target_path"`
	Type       string `json:"type"`
	Present    bool   `json:"present"`
	Current    bool   `json:"current"`
	Excluded   bool   `json:"excluded"`
	Detail     string `json:"detail"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	var targets []string
	if statusAll {
		targets, err = gitutil.FindGitRepos(target)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("no git repositories found under %s", target)
		}
	} else {
		targets = []string{target}
	}

	if statusGit {
		return runGitStatus(targets)
	}

	return runInjectionStatus(targets)
}

func runGitStatus(targets []string) error {
	statuses := syncer.StatusAll(targets, statusNoFetch, false, 1)

	if statusJSON {
		return ui.PrintGitStatusJSON(statuses)
	}

	ui.PrintGitStatusTable(statuses)
	return nil
}

func runInjectionStatus(targets []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var jsonOutputs []jsonStatusOutput

	for _, t := range targets {
		statuses, err := injector.Status(cfg, t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking %s: %v\n", t, err)
			continue
		}

		if statusJSON {
			out := jsonStatusOutput{Repo: t}
			for _, s := range statuses {
				out.Items = append(out.Items, jsonItemStatus{
					TargetPath: s.Item.TargetPath,
					Type:       string(s.Item.Type),
					Present:    s.Present,
					Current:    s.Current,
					Excluded:   s.Excluded,
					Detail:     s.Detail,
				})
			}
			jsonOutputs = append(jsonOutputs, out)
		} else {
			ui.PrintStatusTable(t, statuses)
		}
	}

	if statusJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonOutputs)
	}

	return nil
}

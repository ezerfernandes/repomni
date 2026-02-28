package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/mergestatus"
	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var syncStateCmd = &cobra.Command{
	Use:   "sync-state [directory]",
	Short: "Update workflow states from PR/MR status",
	Long: `Query GitHub or GitLab for the current status of pull/merge requests
associated with branch repos and update their workflow states accordingly.

Only repos with a stored merge URL and a review-related state (review, approved,
review-blocked) are checked. The platform is detected from the URL: github.com
uses "gh", all others use "glab".

Requires the respective CLI tool (gh or glab) to be installed and authenticated.

If no directory is specified, the current directory is used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSyncState,
}

var (
	syncStateDryRun bool
	syncStateJSON   bool
)

func init() {
	rootCmd.AddCommand(syncStateCmd)
	syncStateCmd.Flags().BoolVar(&syncStateDryRun, "dry-run", false,
		"show what would change without updating configs")
	syncStateCmd.Flags().BoolVar(&syncStateJSON, "json", false, "output as JSON")
}

func runSyncState(cmd *cobra.Command, args []string) error {
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

	reviewStates := mergestatus.ReviewStates()
	var results []mergestatus.Result
	var summary mergestatus.Summary

	for _, repo := range repos {
		gitDir, err := gitutil.FindGitDir(repo)
		if err != nil {
			continue
		}

		cfg, _ := repoconfig.Load(gitDir)
		if cfg == nil || cfg.MergeURL == "" || !reviewStates[cfg.State] {
			continue
		}

		summary.Total++

		result := mergestatus.Result{
			Path:          repo,
			Name:          filepath.Base(repo),
			MergeURL:      cfg.MergeURL,
			PreviousState: cfg.State,
		}

		newState, platform, queryErr := mergestatus.QueryMergeStatus(cfg.MergeURL)
		result.Platform = platform

		if queryErr != nil {
			result.Error = queryErr.Error()
			result.NewState = cfg.State
			summary.Errors++
			results = append(results, result)
			continue
		}

		result.NewState = newState
		result.Changed = newState != cfg.State

		if result.Changed && !syncStateDryRun {
			cfg.State = newState
			if saveErr := repoconfig.Save(gitDir, cfg); saveErr != nil {
				result.Error = fmt.Sprintf("failed to save config: %v", saveErr)
				summary.Errors++
				results = append(results, result)
				continue
			}
		}

		if result.Changed {
			summary.Updated++
		} else {
			summary.Unchanged++
		}

		results = append(results, result)
	}

	if summary.Total == 0 {
		fmt.Println("No repos with active merge requests found.")
		return nil
	}

	if syncStateJSON {
		out := struct {
			Results []mergestatus.Result `json:"results"`
			Summary mergestatus.Summary  `json:"summary"`
		}{Results: results, Summary: summary}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	printSyncStateResults(results, summary)

	if syncStateDryRun {
		fmt.Println("Dry run -- no changes were made.")
	}

	return nil
}

func printSyncStateResults(results []mergestatus.Result, summary mergestatus.Summary) {
	nameW := 0
	for _, r := range results {
		if len(r.Name) > nameW {
			nameW = len(r.Name)
		}
	}

	fmt.Printf("\nChecking %d branches in review...\n\n", summary.Total)

	for _, r := range results {
		var icon, detail string

		switch {
		case r.Error != "":
			icon = "⚠️"
			detail = r.Error
		case !r.Changed:
			icon = "⏳"
			detail = fmt.Sprintf("%s (no change)", ui.RenderState(r.NewState))
		case r.NewState == string(repoconfig.StateClosed):
			icon = "❌"
			detail = fmt.Sprintf("%s ← was %s",
				ui.RenderState(r.NewState), r.PreviousState)
		default:
			icon = "✅"
			detail = fmt.Sprintf("%s ← was %s",
				ui.RenderState(r.NewState), r.PreviousState)
		}

		fmt.Printf("  %s %-*s  %s\n", icon, nameW, r.Name, detail)
	}

	var parts []string
	if summary.Updated > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", summary.Updated))
	}
	if summary.Unchanged > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", summary.Unchanged))
	}
	if summary.Errors > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", summary.Errors))
	}

	fmt.Printf("\n%s.\n", strings.Join(parts, ", "))
}

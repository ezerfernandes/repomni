package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ezer/repoinjector/internal/syncer"
)

// PrintSyncResults displays sync results as a table with a summary line.
func PrintSyncResults(results []syncer.SyncResult, summary syncer.SyncSummary) {
	// Compute column widths from data.
	repoW := len("Repository")
	branchW := len("Branch")
	actionW := len("Action")
	for _, r := range results {
		if w := len(syncActionIcon(r.Action)) + 1 + len(r.Name); w > repoW {
			repoW = w
		}
		if len(r.Branch) > branchW {
			branchW = len(r.Branch)
		}
		if len(r.Action) > actionW {
			actionW = len(r.Action)
		}
	}

	hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%s\n", repoW, branchW, actionW)
	fmt.Println()
	fmt.Printf(hdrFmt, "Repository", "Branch", "Action", "Detail")
	fmt.Printf(hdrFmt,
		strings.Repeat("─", repoW),
		strings.Repeat("─", branchW),
		strings.Repeat("─", actionW),
		strings.Repeat("─", 6))

	rowFmt := fmt.Sprintf("  %%s %%-%ds  %%-%ds  %%-%ds  %%s\n", repoW-5, branchW, actionW)
	for _, r := range results {
		icon := syncActionIcon(r.Action)
		fmt.Printf(rowFmt, icon, r.Name, r.Branch, r.Action, r.PostDetail)
	}

	fmt.Printf("\nDone. %d pulled, %d current, %d skipped, %d conflicts, %d errors (out of %d repos).\n",
		summary.Pulled, summary.Current, summary.Skipped, summary.Conflicts, summary.Errors, summary.Total)
}

// PrintGitStatusTable displays git status for multiple repos.
func PrintGitStatusTable(statuses []syncer.RepoStatus) {
	fmt.Println()
	fmt.Printf("  %-25s  %-12s  %-12s  %-6s  %s\n", "Repository", "Branch", "State", "Dirty", "Detail")
	fmt.Printf("  %-25s  %-12s  %-12s  %-6s  %s\n",
		"\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500",
		"\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500",
		"\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500",
		"\u2500\u2500\u2500\u2500\u2500\u2500",
		"\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500")

	for _, s := range statuses {
		dirty := "No"
		if s.Dirty {
			dirty = "Yes"
		}
		fmt.Printf("  %-25s  %-12s  %-12s  %-6s  %s\n", s.Name, s.Branch, string(s.State), dirty, s.Detail)
	}
	fmt.Println()
}

// PrintSyncJSON outputs sync results as JSON to stdout.
func PrintSyncJSON(results []syncer.SyncResult, summary syncer.SyncSummary) error {
	out := struct {
		Results []syncer.SyncResult  `json:"results"`
		Summary syncer.SyncSummary   `json:"summary"`
	}{Results: results, Summary: summary}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// PrintGitStatusJSON outputs git status as JSON to stdout.
func PrintGitStatusJSON(statuses []syncer.RepoStatus) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(statuses)
}

func syncActionIcon(action string) string {
	switch action {
	case "pulled":
		return "[ok]"
	case "skipped", "dry-run":
		return "[--]"
	case "error":
		return "[!!]"
	default:
		return "[??]"
	}
}

package ui

import (
	"fmt"

	"github.com/ezerfernandes/repoinjector/internal/injector"
)

// PrintResults prints injection or ejection results with a summary line.
func PrintResults(results []injector.Result) {
	created, skipped, errors := 0, 0, 0

	warnings := 0

	for _, r := range results {
		icon := actionIcon(r.Action)
		fmt.Printf("  %s %s — %s\n", icon, r.Item.TargetPath, r.Detail)

		switch r.Action {
		case "created", "updated", "removed":
			created++
		case "skipped", "dry-run":
			skipped++
		case "warning":
			warnings++
		case "error":
			errors++
		}
	}

	summary := fmt.Sprintf("\nDone. %d changed, %d skipped", created, skipped)
	if warnings > 0 {
		summary += fmt.Sprintf(", %d warnings", warnings)
	}
	summary += fmt.Sprintf(", %d errors.\n", errors)
	fmt.Print(summary)
}

// PrintStatusTable prints a table showing the injection status of each item in a repo.
func PrintStatusTable(repoPath string, statuses []injector.ItemStatus) {
	fmt.Printf("\nRepository: %s\n", repoPath)
	fmt.Println("  Item                   Present   Current   Excluded")
	fmt.Println("  ─────────────────────  ────────  ────────  ────────")

	for _, s := range statuses {
		present := boolIcon(s.Present)
		current := "-"
		if s.Present {
			current = boolIcon(s.Current)
		}
		excluded := boolIcon(s.Excluded)

		fmt.Printf("  %-21s  %-8s  %-8s  %s\n",
			s.Item.TargetPath, present, current, excluded)
	}
	fmt.Println()
}

func actionIcon(action string) string {
	switch action {
	case "created", "updated", "removed":
		return "[ok]"
	case "skipped", "dry-run":
		return "[--]"
	case "warning":
		return "[!!]"
	case "error":
		return "[!!]"
	default:
		return "[??]"
	}
}

func boolIcon(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

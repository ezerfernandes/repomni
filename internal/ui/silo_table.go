package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ezerfernandes/repomni/internal/silo"
)

// PrintSiloTable displays silo analysis results as a formatted table.
func PrintSiloTable(result *silo.Result) {
	if len(result.Silos) == 0 {
		fmt.Println("\nNo knowledge silos detected.")
		return
	}

	pathW := len("File/Directory")
	ownerW := len("Sole Owner")
	for _, s := range result.Silos {
		if len(s.Path) > pathW {
			pathW = len(s.Path)
		}
		if len(s.Owner) > ownerW {
			ownerW = len(s.Owner)
		}
	}

	hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%8s  %%s\n", pathW, ownerW)
	fmt.Println()
	fmt.Printf(hdrFmt, "File/Directory", "Sole Owner", "Commits", "Risk")
	fmt.Printf(hdrFmt,
		strings.Repeat("─", pathW),
		strings.Repeat("─", ownerW),
		strings.Repeat("─", 8),
		strings.Repeat("─", 6))

	rowFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%8d  %%s\n", pathW, ownerW)
	for _, s := range result.Silos {
		fmt.Printf(rowFmt, s.Path, s.Owner, s.Commits, riskIcon(s.RiskLevel))
	}

	fmt.Printf("\n%d silo(s) found out of %d files analyzed.", result.Summary.TotalSilos, result.Summary.TotalFiles)
	if result.Summary.MostAtRisk != "" {
		fmt.Printf(" Most at-risk: %s (%d commits)", result.Summary.MostAtRisk, result.Summary.MaxCommits)
	}
	fmt.Println()
}

// PrintSiloJSON outputs silo results as JSON.
func PrintSiloJSON(result *silo.Result) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func riskIcon(level string) string {
	switch level {
	case "high":
		return "[!!] high"
	case "medium":
		return "[--] medium"
	default:
		return "[  ] low"
	}
}

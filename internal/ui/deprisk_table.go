package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ezerfernandes/repomni/internal/deprisk"
)

var (
	riskLowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#2e8b57")) // green
	riskMediumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))       // yellow
	riskHighStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))       // red
	riskCriticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#b22222")).Bold(true)
)

func riskStyle(level deprisk.RiskLevel) lipgloss.Style {
	switch level {
	case deprisk.RiskCritical:
		return riskCriticalStyle
	case deprisk.RiskHigh:
		return riskHighStyle
	case deprisk.RiskMedium:
		return riskMediumStyle
	default:
		return riskLowStyle
	}
}

// PrintDepRiskTable renders a dependency risk table for a single repo.
func PrintDepRiskTable(result *deprisk.ScanResult) {
	if len(result.Deps) == 0 {
		fmt.Printf("\nRepository: %s\n  No dependencies found.\n\n", result.RepoPath)
		return
	}

	fmt.Printf("\nRepository: %s\n", result.RepoPath)

	// Calculate column widths
	pathW := len("Module")
	verW := len("Version")
	latestW := len("Latest")
	for _, d := range result.Deps {
		if len(d.Path) > pathW {
			pathW = len(d.Path)
		}
		if len(d.Version) > verW {
			verW = len(d.Version)
		}
		lv := d.LatestVersion
		if lv == "" {
			lv = "-"
		}
		if len(lv) > latestW {
			latestW = len(lv)
		}
	}

	// Cap path width for readability
	if pathW > 50 {
		pathW = 50
	}

	scoreW := 5 // "Score"
	riskW := 8  // "Critical"
	vulnW := 5  // "Vulns"

	hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%%ds  %%-%ds  %%%ds\n",
		pathW, verW, latestW, scoreW, riskW, vulnW)
	fmt.Printf(hdrFmt, "Module", "Version", "Latest", "Score", "Risk", "Vulns")
	fmt.Printf(hdrFmt,
		strings.Repeat("─", pathW),
		strings.Repeat("─", verW),
		strings.Repeat("─", latestW),
		strings.Repeat("─", scoreW),
		strings.Repeat("─", riskW),
		strings.Repeat("─", vulnW))

	rowFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%%ds  %%s%%s  %%%dd\n",
		pathW, verW, latestW, scoreW, vulnW)

	for _, d := range result.Deps {
		latest := d.LatestVersion
		if latest == "" {
			latest = "-"
		}
		path := d.Path
		if len(path) > pathW {
			path = path[:pathW-3] + "..."
		}

		riskStr := riskStyle(d.Risk).Render(string(d.Risk))
		rawRisk := string(d.Risk)
		pad := riskW - len(rawRisk)
		if pad < 0 {
			pad = 0
		}

		fmt.Printf(rowFmt,
			path,
			d.Version,
			latest,
			fmt.Sprintf("%.1f", d.RiskScore),
			riskStr, strings.Repeat(" ", pad),
			len(d.Vulnerabilities))
	}
	fmt.Println()
}

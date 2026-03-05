package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/deprisk"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var depsCmd = &cobra.Command{
	Use:   "deps [target]",
	Short: "Scan Go dependencies for security and maintenance risks",
	Long: `Analyze Go module dependencies for outdated versions and known
vulnerabilities. Each dependency receives a risk score (0-10) based on
how far behind the latest version it is and any known CVEs.

Requires 'go' in PATH. Optionally uses 'govulncheck' for vulnerability
detection.

If no target is specified, the current directory is used.
Use --all to scan all git repos under the target directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeps,
}

var (
	depsAll     bool
	depsJSON    bool
	depsMinRisk string
)

func init() {
	rootCmd.AddCommand(depsCmd)
	depsCmd.Flags().BoolVar(&depsAll, "all", false, "scan all git repos under target directory")
	depsCmd.Flags().BoolVar(&depsJSON, "json", false, "output as JSON")
	depsCmd.Flags().StringVar(&depsMinRisk, "min-risk", "", "filter by minimum risk level (low, medium, high, critical)")
}

func runDeps(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	var targets []string
	if depsAll {
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

	var minRisk deprisk.RiskLevel
	if depsMinRisk != "" {
		switch depsMinRisk {
		case "low":
			minRisk = deprisk.RiskLow
		case "medium":
			minRisk = deprisk.RiskMedium
		case "high":
			minRisk = deprisk.RiskHigh
		case "critical":
			minRisk = deprisk.RiskCritical
		default:
			return fmt.Errorf("invalid --min-risk value %q: use low, medium, high, or critical", depsMinRisk)
		}
	}

	var results []*deprisk.ScanResult
	for _, t := range targets {
		// Check if target has a go.mod
		if _, err := os.Stat(filepath.Join(t, "go.mod")); err != nil {
			if depsAll {
				continue
			}
			return fmt.Errorf("no go.mod found in %s", t)
		}

		result, err := deprisk.Scan(t)
		if err != nil {
			if depsAll {
				results = append(results, &deprisk.ScanResult{RepoPath: t, Error: err.Error()})
				continue
			}
			return err
		}

		if minRisk != "" {
			result.Deps = deprisk.FilterByMinRisk(result.Deps, minRisk)
		}

		results = append(results, result)
	}

	if depsJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(os.Stderr, "\nRepository: %s\n  Error: %s\n\n", r.RepoPath, r.Error)
			continue
		}
		ui.PrintDepRiskTable(r)
	}

	return nil
}

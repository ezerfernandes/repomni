package cmd

import (
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/silo"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var siloCmd = &cobra.Command{
	Use:   "silo [directory]",
	Short: "Detect knowledge silos in git history",
	Long: `Analyze git history to find files or directories where only one contributor
has made changes. These knowledge silos represent a bus-factor risk — if that
person leaves, no one else has context on the code.

If no directory is specified, the current directory is used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSilo,
}

var (
	siloJSON      bool
	siloMinCommit int
	siloThreshold int
	siloByDir     bool
)

func init() {
	rootCmd.AddCommand(siloCmd)
	siloCmd.Flags().BoolVar(&siloJSON, "json", false, "output as JSON")
	siloCmd.Flags().IntVar(&siloMinCommit, "min-commits", 3, "minimum commits for a file to be considered")
	siloCmd.Flags().IntVar(&siloThreshold, "threshold", 1, "max unique contributors to count as silo")
	siloCmd.Flags().BoolVar(&siloByDir, "by-dir", false, "aggregate analysis by directory")
}

func runSilo(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	result, err := silo.Analyze(target, silo.Options{
		MinCommits: siloMinCommit,
		Threshold:  siloThreshold,
		ByDir:      siloByDir,
	})
	if err != nil {
		return err
	}

	if siloJSON {
		return ui.PrintSiloJSON(result)
	}

	ui.PrintSiloTable(result)
	return nil
}

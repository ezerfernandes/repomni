package cmd

import (
	"encoding/json"
	"os"

	"github.com/ezerfernandes/repomni/internal/session"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var sessionStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show aggregate session statistics",
	Args:  cobra.NoArgs,
	RunE:  runSessionStats,
}

var sessionStatsJSON bool

func init() {
	sessionCmd.AddCommand(sessionStatsCmd)
	sessionStatsCmd.Flags().BoolVar(&sessionStatsJSON, "json", false, "output as JSON")
}

func runSessionStats(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	sessions, err := session.DiscoverAll(projectPath, sessionCLIFilter, 0)
	if err != nil {
		return err
	}

	stats := session.Aggregate(sessions)

	if sessionStatsJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	ui.PrintSessionStats(stats)
	return nil
}

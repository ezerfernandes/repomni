package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ezerfernandes/repoinjector/internal/session"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Claude Code sessions for the current project",
	Args:  cobra.NoArgs,
	RunE:  runSessionList,
}

var (
	sessionListJSON  bool
	sessionListLimit int
)

func init() {
	sessionCmd.AddCommand(sessionListCmd)
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "output as JSON")
	sessionListCmd.Flags().IntVar(&sessionListLimit, "limit", 0, "maximum number of sessions to show")
}

func runSessionList(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	sessions, err := session.DiscoverWithLimit(projectPath, sessionListLimit)
	if err != nil {
		return err
	}

	if sessionListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	ui.PrintSessionsList(sessions)
	return nil
}

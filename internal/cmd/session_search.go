package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ezerfernandes/repoinjector/internal/session"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var sessionSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Claude Code sessions by content",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionSearch,
}

var (
	sessionSearchJSON  bool
	sessionSearchMode  string
	sessionSearchLimit int
)

func init() {
	sessionCmd.AddCommand(sessionSearchCmd)
	sessionSearchCmd.Flags().BoolVar(&sessionSearchJSON, "json", false, "output as JSON")
	sessionSearchCmd.Flags().StringVar(&sessionSearchMode, "mode", "all",
		`search mode: "title" (first message), "user", "assistant", or "all"`)
	sessionSearchCmd.Flags().IntVar(&sessionSearchLimit, "limit", 10, "maximum number of matching sessions")
}

func runSessionSearch(cmd *cobra.Command, args []string) error {
	switch sessionSearchMode {
	case "title", "user", "assistant", "all":
		// valid
	default:
		return fmt.Errorf("invalid search mode %q; must be title, user, assistant, or all", sessionSearchMode)
	}

	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	results, err := session.Search(projectPath, args[0], sessionSearchMode, sessionSearchLimit)
	if err != nil {
		return err
	}

	if sessionSearchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if len(results) == 0 {
		fmt.Println("No matches found.")
		return nil
	}

	ui.PrintSearchResults(results)
	return nil
}

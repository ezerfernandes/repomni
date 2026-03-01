package cmd

import (
	"encoding/json"
	"os"

	"github.com/ezerfernandes/repoinjector/internal/session"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var sessionShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show messages from a Claude Code session",
	Long: `Display the conversation history of a specific session. Supports
prefix matching on the session ID (e.g., first 6+ characters).`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionShow,
}

var (
	sessionShowJSON   bool
	sessionShowLimit  int
	sessionShowOffset int
	sessionShowFull   bool
)

func init() {
	sessionCmd.AddCommand(sessionShowCmd)
	sessionShowCmd.Flags().BoolVar(&sessionShowJSON, "json", false, "output as JSON")
	sessionShowCmd.Flags().IntVar(&sessionShowLimit, "limit", 0, "maximum number of messages to show")
	sessionShowCmd.Flags().IntVar(&sessionShowOffset, "offset", 0, "skip the first N messages")
	sessionShowCmd.Flags().BoolVar(&sessionShowFull, "full", false, "show full tool_use/tool_result blocks")
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	meta, err := session.FindSession(projectPath, args[0])
	if err != nil {
		return err
	}

	messages, err := session.ReadMessages(meta.FilePath, sessionShowOffset, sessionShowLimit, sessionShowFull)
	if err != nil {
		return err
	}

	if sessionShowJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(messages)
	}

	ui.PrintSessionMessages(*meta, messages, sessionShowFull)
	return nil
}

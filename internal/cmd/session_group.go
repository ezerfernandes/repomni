package cmd

import "github.com/spf13/cobra"

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Explore Claude Code and Codex sessions",
	Long: `Commands for listing, viewing, searching, and managing Claude Code
and Codex sessions for the current project.`,
}

var sessionCLIFilter string

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.PersistentFlags().StringVar(&sessionCLIFilter, "cli", "",
		`filter by CLI tool: "claude", "codex", or empty for both`)
}

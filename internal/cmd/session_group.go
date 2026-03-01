package cmd

import "github.com/spf13/cobra"

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Explore Claude Code sessions",
	Long: `Commands for listing, viewing, searching, and managing Claude Code
sessions for the current project.`,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}

package cmd

import "github.com/spf13/cobra"

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Run commands across repos",
	Long: `Commands for executing and comparing command output across the
main repo and branch repos.`,
}

func init() {
	rootCmd.AddCommand(execCmd)
}

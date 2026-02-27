package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "repoinjector",
	Short: "Inject shared config files into multiple repo clones",
	Long: `Repoinjector symlinks or copies shared configuration files (.claude/skills,
.claude/hooks, .envrc, .env) from a central source into one or more target
repository clones, keeping injected files invisible to git.`,
}

// Execute runs the root Cobra command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = version
}

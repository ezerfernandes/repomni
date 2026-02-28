package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/injector"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var ejectCmd = &cobra.Command{
	Use:   "eject [target]",
	Short: "Remove injected files from target repo(s)",
	Long: `Remove all injected symlinks and files from the target repository and clean
up the managed block in .git/info/exclude.

If no target is specified, the current directory is used.
Use --all to eject from all git repos under the target directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEject,
}

var ejectAll bool

func init() {
	rootCmd.AddCommand(ejectCmd)
	ejectCmd.Flags().BoolVar(&ejectAll, "all", false, "eject from all git repos under target directory")
}

func runEject(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}

	var targets []string
	if ejectAll {
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

	hasErrors := false
	for _, t := range targets {
		if ejectAll {
			fmt.Printf("\nEjecting from %s...\n", t)
		}

		results, err := injector.Eject(cfg, t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			hasErrors = true
			continue
		}

		ui.PrintResults(results)

		for _, r := range results {
			if r.Action == "error" {
				hasErrors = true
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("some items had errors")
	}

	return nil
}

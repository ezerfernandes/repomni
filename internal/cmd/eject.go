package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/injector"
	"github.com/ezerfernandes/repomni/internal/ui"
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

var (
	ejectAll  bool
	ejectJSON bool
)

func init() {
	rootCmd.AddCommand(ejectCmd)
	ejectCmd.Flags().BoolVar(&ejectAll, "all", false, "eject from all git repos under target directory")
	ejectCmd.Flags().BoolVar(&ejectJSON, "json", false, "output as JSON")
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

	type jsonEjectResult struct {
		Target string `json:"target"`
		Action string `json:"action"`
		Item   string `json:"item"`
		Detail string `json:"detail"`
	}
	var jsonResults []jsonEjectResult

	hasErrors := false
	for _, t := range targets {
		targetCfg := cfg
		repoCfg, _ := loadRepoConfig(t)
		if repoCfg != nil {
			targetCfg = repoCfg.FilterGlobalConfig(cfg)
		}

		if ejectAll && !ejectJSON {
			fmt.Printf("\nEjecting from %s...\n", t)
		}

		results, err := injector.Eject(targetCfg, t)
		if err != nil {
			if len(results) > 0 && !ejectJSON {
				ui.PrintResults(results)
			}
			if ejectJSON {
				for _, r := range results {
					jsonResults = append(jsonResults, jsonEjectResult{
						Target: t, Action: r.Action, Item: r.Item.TargetPath, Detail: r.Detail,
					})
				}
			}
			if !ejectJSON {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			hasErrors = true
			continue
		}

		if ejectJSON {
			for _, r := range results {
				jsonResults = append(jsonResults, jsonEjectResult{
					Target: t, Action: r.Action, Item: r.Item.TargetPath, Detail: r.Detail,
				})
			}
		} else {
			ui.PrintResults(results)
		}

		for _, r := range results {
			if r.Action == "error" {
				hasErrors = true
			}
		}
	}

	if ejectJSON {
		ui.PrintJSON(jsonResults)
		if hasErrors {
			return fmt.Errorf("some items had errors")
		}
		return nil
	}

	if hasErrors {
		return fmt.Errorf("some items had errors")
	}

	return nil
}

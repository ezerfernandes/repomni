package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure injection settings for this repository",
	Long: `Interactively select which items and entries to inject into this repository.

The configuration is saved to .git/repoinjector/config.yaml and is used by
"repoinjector inject" and "repoinjector branch" to filter which items get injected.`,
	RunE: runConfigure,
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	repoRoot, err := gitutil.RunGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	gitDir, err := gitutil.FindGitDir(repoRoot)
	if err != nil {
		return err
	}

	globalCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("global config not found (run 'repoinjector settings' first): %w", err)
	}

	existingRepoCfg, err := repoconfig.Load(gitDir)
	if err != nil {
		return fmt.Errorf("cannot read existing repo config: %w", err)
	}

	repoCfg, err := ui.RunConfigureRepoForm(globalCfg, existingRepoCfg)
	if err != nil {
		return fmt.Errorf("configuration cancelled: %w", err)
	}

	if err := repoconfig.Save(gitDir, repoCfg); err != nil {
		return err
	}

	fmt.Printf("\nRepository configuration saved to %s\n", repoconfig.ConfigPath(gitDir))
	return nil
}

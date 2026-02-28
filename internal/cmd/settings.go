package cmd

import (
	"fmt"
	"os"

	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Interactively configure repoinjector",
	Long: `Run an interactive wizard to set up repoinjector. Configures the source
directory, injection mode, and which items to inject.

The configuration is saved to ~/.config/repoinjector/config.yaml.`,
	RunE: runSettings,
}

var (
	settingsSource         string
	settingsNonInteractive bool
)

func init() {
	rootCmd.AddCommand(settingsCmd)
	settingsCmd.Flags().StringVar(&settingsSource, "source", "", "source directory (skip interactive prompt)")
	settingsCmd.Flags().BoolVar(&settingsNonInteractive, "non-interactive", false, "use defaults without prompting")
}

func runSettings(cmd *cobra.Command, args []string) error {
	// Load existing config as defaults, or start fresh
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if settingsNonInteractive {
		if settingsSource == "" {
			return fmt.Errorf("--source is required in non-interactive mode")
		}
		cfg.SourceDir = config.ExpandPath(settingsSource)
	} else {
		// Override source if flag provided
		if settingsSource != "" {
			cfg.SourceDir = config.ExpandPath(settingsSource)
		}

		cfg, err = ui.RunSettingsForm(cfg)
		if err != nil {
			return fmt.Errorf("configuration cancelled: %w", err)
		}
	}

	// Validate source directory
	cfg.SourceDir = config.ExpandPath(cfg.SourceDir)
	if cfg.SourceDir == "" {
		return fmt.Errorf("source directory cannot be empty")
	}
	info, err := os.Stat(cfg.SourceDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source directory does not exist or is not a directory: %s", cfg.SourceDir)
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	path, _ := config.ConfigPath()
	fmt.Printf("\nConfiguration saved to %s\n", path)
	return nil
}

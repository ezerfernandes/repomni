package cmd

import (
	"fmt"
	"os"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

type jsonConfigItem struct {
	Type       string `json:"type"`
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Enabled    bool   `json:"enabled"`
}

var settingsCmd = &cobra.Command{
	Use:   "global",
	Short: "Interactively configure repomni",
	Long: `Run an interactive wizard to set up repomni. Configures the source
directory, injection mode, and which items to inject.

The configuration is saved to ~/.config/repomni/config.yaml.`,
	RunE: runSettings,
}

var (
	settingsSource         string
	settingsNonInteractive bool
	settingsJSON           bool
)

func init() {
	configCmd.AddCommand(settingsCmd)
	settingsCmd.Flags().StringVar(&settingsSource, "source", "", "source directory (skip interactive prompt)")
	settingsCmd.Flags().BoolVar(&settingsNonInteractive, "non-interactive", false, "use defaults without prompting")
	settingsCmd.Flags().BoolVar(&settingsJSON, "json", false, "output as JSON")
}

func runSettings(cmd *cobra.Command, args []string) error {
	// Load existing config as defaults, or start fresh
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if settingsNonInteractive || settingsJSON {
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

	if settingsJSON {
		items := make([]jsonConfigItem, len(cfg.Items))
		for i, item := range cfg.Items {
			items[i] = jsonConfigItem{
				Type:       string(item.Type),
				SourcePath: item.SourcePath,
				TargetPath: item.TargetPath,
				Enabled:    item.Enabled,
			}
		}
		return ui.PrintJSON(struct {
			Path      string           `json:"path"`
			SourceDir string           `json:"source_dir"`
			Mode      string           `json:"mode"`
			Items     []jsonConfigItem `json:"items"`
		}{
			Path:      path,
			SourceDir: cfg.SourceDir,
			Mode:      string(cfg.Mode),
			Items:     items,
		})
	}

	fmt.Printf("\nConfiguration saved to %s\n", path)
	return nil
}

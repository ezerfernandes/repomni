package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repoinjector/internal/config"
)

type itemDef struct {
	label      string
	key        string // stable key for selection (matches target_path)
	sourcePath string
	targetPath string
	itemType   config.ItemType
}

var defaultItemDefs = []itemDef{
	{label: "skills/ (directory)", key: ".claude/skills", sourcePath: "skills", targetPath: ".claude/skills", itemType: config.ItemTypeDirectory},
	{label: "hooks.json (file)", key: ".claude/hooks.json", sourcePath: "hooks.json", targetPath: ".claude/hooks.json", itemType: config.ItemTypeFile},
	{label: ".envrc (file)", key: ".envrc", sourcePath: ".envrc", targetPath: ".envrc", itemType: config.ItemTypeFile},
	{label: ".env (file)", key: ".env", sourcePath: ".env", targetPath: ".env", itemType: config.ItemTypeFile},
}

// RunSettingsForm runs the interactive TUI for editing the global configuration.
func RunSettingsForm(cfg *config.Config) (*config.Config, error) {
	var sourceDir string
	if cfg.SourceDir != "" {
		sourceDir = cfg.SourceDir
	}

	var mode string
	if cfg.Mode != "" {
		mode = string(cfg.Mode)
	} else {
		mode = string(config.ModeSymlink)
	}

	// Build source path overrides from existing config, falling back to defaults
	existingSourcePaths := make(map[string]string)
	enabledSet := make(map[string]bool)
	for _, item := range cfg.Items {
		existingSourcePaths[item.TargetPath] = item.SourcePath
		if item.Enabled {
			enabledSet[item.TargetPath] = true
		}
	}

	skillsSource := getSourcePath(existingSourcePaths, ".claude/skills", "skills")
	hooksSource := getSourcePath(existingSourcePaths, ".claude/hooks.json", "hooks.json")
	envrcSource := getSourcePath(existingSourcePaths, ".envrc", ".envrc")
	envSource := getSourcePath(existingSourcePaths, ".env", ".env")

	var selectedItems []string
	for _, def := range defaultItemDefs {
		if enabledSet[def.key] || len(cfg.Items) == 0 {
			selectedItems = append(selectedItems, def.key)
		}
	}

	var selectOptions []huh.Option[string]
	for _, def := range defaultItemDefs {
		selectOptions = append(selectOptions, huh.NewOption(def.label, def.key))
	}

	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Source directory").
				Description("Path to the directory containing files to inject").
				Value(&sourceDir).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("source directory is required")
					}
					abs, err := filepath.Abs(config.ExpandPath(s))
					if err != nil {
						return err
					}
					info, err := os.Stat(abs)
					if err != nil {
						return fmt.Errorf("directory not found: %s", abs)
					}
					if !info.IsDir() {
						return fmt.Errorf("not a directory: %s", abs)
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Injection mode").
				Description("Symlinks reflect source changes instantly. Copy creates independent snapshots.").
				Options(
					huh.NewOption("Symlink (recommended)", "symlink"),
					huh.NewOption("Copy", "copy"),
				).
				Value(&mode),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Items to inject").
				Description("Select which files/directories to inject into target repos").
				Options(selectOptions...).
				Value(&selectedItems),
		),
		huh.NewGroup(
			huh.NewNote().
				Title("Source path overrides").
				Description("Customize where each item is read from in the source directory.\nLeave defaults to use the standard paths."),
			huh.NewInput().
				Title("Skills directory").
				Description("Source path for .claude/skills/ (default: skills)").
				Value(&skillsSource),
			huh.NewInput().
				Title("Hooks file").
				Description("Source path for .claude/hooks.json (default: hooks.json)").
				Value(&hooksSource),
			huh.NewInput().
				Title("Envrc file").
				Description("Source path for .envrc (default: .envrc)").
				Value(&envrcSource),
			huh.NewInput().
				Title("Env file").
				Description("Source path for .env (default: .env)").
				Value(&envSource),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save this configuration?").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	if !confirm {
		return nil, fmt.Errorf("cancelled by user")
	}

	absSource, err := filepath.Abs(config.ExpandPath(sourceDir))
	if err != nil {
		return nil, err
	}

	selectedSet := make(map[string]bool)
	for _, s := range selectedItems {
		selectedSet[s] = true
	}

	// Map overridden source paths by target path key
	sourceOverrides := map[string]string{
		".claude/skills":     skillsSource,
		".claude/hooks.json": hooksSource,
		".envrc":             envrcSource,
		".env":               envSource,
	}

	var items []config.Item
	for _, def := range defaultItemDefs {
		sp := sourceOverrides[def.key]
		if sp == "" {
			sp = def.sourcePath
		}
		items = append(items, config.Item{
			Type:       def.itemType,
			SourcePath: sp,
			TargetPath: def.targetPath,
			Enabled:    selectedSet[def.key],
		})
	}

	return &config.Config{
		Version:   1,
		SourceDir: absSource,
		Mode:      config.InjectionMode(mode),
		Items:     items,
	}, nil
}

func getSourcePath(existing map[string]string, targetPath, defaultSource string) string {
	if sp, ok := existing[targetPath]; ok && sp != "" {
		return sp
	}
	return defaultSource
}

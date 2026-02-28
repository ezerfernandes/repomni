package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
)

// RunConfigureRepoForm runs the interactive TUI for configuring per-repo injection settings.
// If existing is non-nil, its values are used as defaults.
func RunConfigureRepoForm(globalCfg *config.Config, existing *repoconfig.RepoConfig) (*repoconfig.RepoConfig, error) {
	sourceDir, err := filepath.Abs(globalCfg.SourceDir)
	if err != nil {
		return nil, err
	}

	// Build the set of previously enabled items and entries for pre-selection.
	existingEnabled := make(map[string]bool)
	existingEntries := make(map[string]map[string]bool)
	if existing != nil {
		for _, item := range existing.Items {
			if item.Enabled {
				existingEnabled[item.TargetPath] = true
				if len(item.Entries) > 0 {
					set := make(map[string]bool)
					for _, e := range item.Entries {
						set[e] = true
					}
					existingEntries[item.TargetPath] = set
				}
			}
		}
	}

	// Stage 1: Pick which top-level items are relevant for this repo.
	var itemOptions []huh.Option[string]
	for _, item := range globalCfg.Items {
		label := fmt.Sprintf("%s (%s)", item.TargetPath, item.Type)
		itemOptions = append(itemOptions, huh.NewOption(label, item.TargetPath))
	}

	var selectedItems []string
	if existing != nil {
		for _, item := range globalCfg.Items {
			if existingEnabled[item.TargetPath] {
				selectedItems = append(selectedItems, item.TargetPath)
			}
		}
	} else {
		for _, item := range globalCfg.EnabledItems() {
			selectedItems = append(selectedItems, item.TargetPath)
		}
	}

	stage1 := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Items to inject for this repository").
				Description("Select which items are relevant for this project.").
				Options(itemOptions...).
				Value(&selectedItems),
		),
	)
	if err := stage1.Run(); err != nil {
		return nil, err
	}

	selectedSet := make(map[string]bool)
	for _, tp := range selectedItems {
		selectedSet[tp] = true
	}

	// Stage 2: For each selected directory item, pick individual entries.
	type dirPick struct {
		targetPath string
		options    []huh.Option[string]
		selected   []string
	}
	var dirPicks []dirPick

	for _, item := range globalCfg.Items {
		if !selectedSet[item.TargetPath] || item.Type != config.ItemTypeDirectory {
			continue
		}

		src := filepath.Join(sourceDir, item.SourcePath)
		entries, err := os.ReadDir(src)
		if err != nil || len(entries) == 0 {
			continue
		}

		var options []huh.Option[string]
		var preSelected []string
		prevEntries := existingEntries[item.TargetPath]

		for _, entry := range entries {
			name := entry.Name()
			label := name
			if entry.IsDir() {
				label += "/"
			}
			options = append(options, huh.NewOption(label, name))

			if prevEntries != nil {
				if prevEntries[name] {
					preSelected = append(preSelected, name)
				}
			} else {
				preSelected = append(preSelected, name)
			}
		}

		dirPicks = append(dirPicks, dirPick{
			targetPath: item.TargetPath,
			options:    options,
			selected:   preSelected,
		})
	}

	if len(dirPicks) > 0 {
		var groups []*huh.Group
		for i := range dirPicks {
			groups = append(groups, huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(fmt.Sprintf("Select entries for %s", dirPicks[i].targetPath)).
					Description("Deselect entries you don't need for this project.").
					Options(dirPicks[i].options...).
					Value(&dirPicks[i].selected),
			))
		}

		stage2 := huh.NewForm(groups...)
		if err := stage2.Run(); err != nil {
			return nil, err
		}
	}

	// Confirmation.
	var confirm bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save this repository configuration?").
				Value(&confirm),
		),
	)
	if err := confirmForm.Run(); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, fmt.Errorf("cancelled by user")
	}

	// Build RepoConfig from selections.
	dirPickMap := make(map[string]*dirPick)
	for i := range dirPicks {
		dirPickMap[dirPicks[i].targetPath] = &dirPicks[i]
	}

	var items []repoconfig.RepoItemConfig
	for _, item := range globalCfg.Items {
		rc := repoconfig.RepoItemConfig{
			TargetPath: item.TargetPath,
			Enabled:    selectedSet[item.TargetPath],
		}
		if dp, ok := dirPickMap[item.TargetPath]; ok && rc.Enabled {
			rc.Entries = dp.selected
		}
		items = append(items, rc)
	}

	return &repoconfig.RepoConfig{
		Version: 1,
		Items:   items,
	}, nil
}

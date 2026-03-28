package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repomni/internal/config"
)

type dirEntryOption struct {
	name  string
	label string
}

type dirSelectionPlan struct {
	targetPath string
	options    []dirEntryOption
	selected   []string
}

// buildDirSelectionPlans collects directory entries for enabled directory items.
//
// If preSelectedEntries contains an entry for an item's TargetPath, those names
// are selected by default. All currently available source entries are still
// shown so newly added skills/files remain visible in the picker.
//
// If preSelectedEntries does not contain an entry for an item, all entries are
// selected by default.
func buildDirSelectionPlans(cfg *config.Config, preSelectedEntries map[string]map[string]bool) ([]dirSelectionPlan, error) {
	sourceDir, err := filepath.Abs(cfg.SourceDir)
	if err != nil {
		return nil, err
	}

	var dirs []dirSelectionPlan
	for _, item := range cfg.EnabledItems() {
		if item.Type != config.ItemTypeDirectory {
			continue
		}

		src := filepath.Join(sourceDir, item.SourcePath)
		entries, err := os.ReadDir(src)
		if err != nil || len(entries) == 0 {
			continue
		}

		var options []dirEntryOption
		var selected []string
		preSelected := preSelectedEntries[item.TargetPath]

		for _, entry := range entries {
			name := entry.Name()
			label := name
			if entry.IsDir() {
				label += "/"
			}

			options = append(options, dirEntryOption{name: name, label: label})

			if preSelected != nil {
				if preSelected[name] {
					selected = append(selected, name)
				}
			} else {
				selected = append(selected, name)
			}
		}

		dirs = append(dirs, dirSelectionPlan{
			targetPath: item.TargetPath,
			options:    options,
			selected:   selected,
		})
	}

	return dirs, nil
}

// SelectDirEntries shows a multi-select for each enabled directory item,
// listing entries from the source directory.
//
// If preSelectedEntries contains an entry for a directory item's TargetPath,
// those names are selected by default. Otherwise, all entries are selected by
// default.
//
// Returns a map from item TargetPath to a set of selected entry names.
// Returns nil if there are no directory items with entries.
func SelectDirEntries(cfg *config.Config, preSelectedEntries map[string]map[string]bool) (map[string]map[string]bool, error) {
	dirs, err := buildDirSelectionPlans(cfg, preSelectedEntries)
	if err != nil {
		return nil, err
	}

	if len(dirs) == 0 {
		return nil, nil
	}

	var groups []*huh.Group
	for i := range dirs {
		var options []huh.Option[string]
		for _, option := range dirs[i].options {
			options = append(options, huh.NewOption(option.label, option.name))
		}

		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select entries to inject into %s", dirs[i].targetPath)).
				Description("All entries are shown. Deselect any you want to skip.").
				Options(options...).
				Value(&dirs[i].selected),
		))
	}

	form := huh.NewForm(groups...)
	if err := form.Run(); err != nil {
		return nil, err
	}

	result := make(map[string]map[string]bool)
	for _, d := range dirs {
		set := make(map[string]bool)
		for _, name := range d.selected {
			set[name] = true
		}
		result[d.targetPath] = set
	}

	return result, nil
}

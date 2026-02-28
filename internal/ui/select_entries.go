package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repoinjector/internal/config"
)

// SelectDirEntries shows a multi-select for each enabled directory item,
// listing entries from the source directory (all initially selected).
// If allowedEntries is non-nil and has an entry for a directory item's TargetPath,
// only entries in that set are shown. Pass nil to show all entries.
// Returns a map from item TargetPath to a set of selected entry names.
// Returns nil if there are no directory items with entries.
func SelectDirEntries(cfg *config.Config, allowedEntries map[string]map[string]bool) (map[string]map[string]bool, error) {
	sourceDir, err := filepath.Abs(cfg.SourceDir)
	if err != nil {
		return nil, err
	}

	// First pass: collect directory items and their entries
	type dirInfo struct {
		targetPath string
		options    []huh.Option[string]
		selected   []string
	}

	var dirs []dirInfo
	for _, item := range cfg.EnabledItems() {
		if item.Type != config.ItemTypeDirectory {
			continue
		}

		src := filepath.Join(sourceDir, item.SourcePath)
		entries, err := os.ReadDir(src)
		if err != nil || len(entries) == 0 {
			continue
		}

		var options []huh.Option[string]
		var selected []string
		allowed := allowedEntries[item.TargetPath] // nil if no filter for this item
		for _, entry := range entries {
			name := entry.Name()
			if allowed != nil && !allowed[name] {
				continue
			}
			label := name
			if entry.IsDir() {
				label += "/"
			}
			options = append(options, huh.NewOption(label, name))
			selected = append(selected, name)
		}

		dirs = append(dirs, dirInfo{
			targetPath: item.TargetPath,
			options:    options,
			selected:   selected,
		})
	}

	if len(dirs) == 0 {
		return nil, nil
	}

	// Second pass: build form groups (slice is stable now, safe to take pointers)
	var groups []*huh.Group
	for i := range dirs {
		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select entries to inject into %s", dirs[i].targetPath)).
				Description("All entries are selected by default. Deselect any you want to skip.").
				Options(dirs[i].options...).
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

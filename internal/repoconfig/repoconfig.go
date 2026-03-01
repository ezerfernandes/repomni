package repoconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/config"
	"gopkg.in/yaml.v3"
)

// RepoConfig stores per-repo injection preferences inside .git/repomni/config.yaml.
// It records which global config items and directory entries are relevant for a particular repo.
type RepoConfig struct {
	Version  int              `yaml:"version"`
	State    string           `yaml:"state,omitempty"`
	MergeURL string           `yaml:"merge_url,omitempty"`
	Ticket   string           `yaml:"ticket,omitempty"`
	Remote   bool             `yaml:"remote,omitempty"`
	Items    []RepoItemConfig `yaml:"items"`
}

// RepoItemConfig records whether a global config item is enabled for this repo,
// and for directory items, which specific entries to include.
type RepoItemConfig struct {
	TargetPath string   `yaml:"target_path"`
	Enabled    bool     `yaml:"enabled"`
	Entries    []string `yaml:"entries,omitempty"`
}

// ConfigPath returns the path to the per-repo config file inside the git directory.
func ConfigPath(gitDir string) string {
	return filepath.Join(gitDir, "repomni", "config.yaml")
}

// Load reads the per-repo config. Returns (nil, nil) if the file does not exist.
func Load(gitDir string) (*RepoConfig, error) {
	data, err := os.ReadFile(ConfigPath(gitDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read repo config: %w", err)
	}

	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid repo config: %w", err)
	}
	return &cfg, nil
}

// Save writes the per-repo config, creating directories as needed.
func Save(gitDir string, cfg *RepoConfig) error {
	path := ConfigPath(gitDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("cannot create repo config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot serialize repo config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write repo config: %w", err)
	}
	return nil
}

// EnabledTargetPaths returns the set of target paths that are enabled in this config.
func (rc *RepoConfig) EnabledTargetPaths() map[string]bool {
	m := make(map[string]bool)
	for _, item := range rc.Items {
		if item.Enabled {
			m[item.TargetPath] = true
		}
	}
	return m
}

// ToSelectedEntries converts the config into the map format expected by injector.Options.SelectedEntries.
// Only directory items with a non-empty Entries list produce an entry in the map.
func (rc *RepoConfig) ToSelectedEntries() map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	for _, item := range rc.Items {
		if item.Enabled && len(item.Entries) > 0 {
			set := make(map[string]bool)
			for _, e := range item.Entries {
				set[e] = true
			}
			m[item.TargetPath] = set
		}
	}
	return m
}

// FilterGlobalConfig returns a copy of the global config with only the items
// that are enabled in this per-repo config.
func (rc *RepoConfig) FilterGlobalConfig(globalCfg *config.Config) *config.Config {
	enabled := rc.EnabledTargetPaths()
	filtered := &config.Config{
		Version:   globalCfg.Version,
		SourceDir: globalCfg.SourceDir,
		Mode:      globalCfg.Mode,
	}
	for _, item := range globalCfg.Items {
		if enabled[item.TargetPath] {
			item.Enabled = true
			filtered.Items = append(filtered.Items, item)
		}
	}
	return filtered
}

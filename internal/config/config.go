package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type InjectionMode string

const (
	ModeSymlink InjectionMode = "symlink"
	ModeCopy    InjectionMode = "copy"
)

type ItemType string

const (
	ItemTypeFile      ItemType = "file"
	ItemTypeDirectory ItemType = "directory"
)

type Item struct {
	Type       ItemType `yaml:"type"`
	SourcePath string   `yaml:"source_path"`
	TargetPath string   `yaml:"target_path"`
	Enabled    bool     `yaml:"enabled"`
}

type Config struct {
	Version   int           `yaml:"version"`
	SourceDir string        `yaml:"source_dir"`
	Mode      InjectionMode `yaml:"mode"`
	Items     []Item        `yaml:"items"`
}

func DefaultItems() []Item {
	return []Item{
		{Type: ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		{Type: ItemTypeFile, SourcePath: "hooks.json", TargetPath: ".claude/hooks.json", Enabled: true},
		{Type: ItemTypeFile, SourcePath: ".envrc", TargetPath: ".envrc", Enabled: true},
		{Type: ItemTypeFile, SourcePath: ".env", TargetPath: ".env", Enabled: true},
	}
}

func DefaultConfig() *Config {
	return &Config{
		Version: 1,
		Mode:    ModeSymlink,
		Items:   DefaultItems(),
	}
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(dir, "repoinjector"), nil
}

func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config (run 'repoinjector settings' first): %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config file %s: %w", path, err)
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("cannot serialize config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	return nil
}

// ExpandPath expands ~ and environment variables in a path string.
func ExpandPath(path string) string {
	path = os.ExpandEnv(path)
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func (c *Config) EnabledItems() []Item {
	var items []Item
	for _, item := range c.Items {
		if item.Enabled {
			items = append(items, item)
		}
	}
	return items
}

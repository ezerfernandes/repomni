package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Mode != ModeSymlink {
		t.Errorf("expected mode symlink, got %s", cfg.Mode)
	}
	if len(cfg.Items) != 4 {
		t.Errorf("expected 4 default items, got %d", len(cfg.Items))
	}
}

func TestEnabledItems(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Items[1].Enabled = false
	cfg.Items[3].Enabled = false

	enabled := cfg.EnabledItems()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled items, got %d", len(enabled))
	}
}

func TestEnabledItems_NoneEnabled(t *testing.T) {
	cfg := DefaultConfig()
	for i := range cfg.Items {
		cfg.Items[i].Enabled = false
	}
	enabled := cfg.EnabledItems()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled items, got %d", len(enabled))
	}
}

func TestEnabledItems_AllEnabled(t *testing.T) {
	cfg := DefaultConfig()
	enabled := cfg.EnabledItems()
	if len(enabled) != len(cfg.Items) {
		t.Errorf("expected %d enabled items, got %d", len(cfg.Items), len(enabled))
	}
}

func TestDefaultItems_Content(t *testing.T) {
	items := DefaultItems()

	expected := []struct {
		itemType   ItemType
		sourcePath string
		targetPath string
	}{
		{ItemTypeDirectory, "skills", ".claude/skills"},
		{ItemTypeFile, "hooks.json", ".claude/hooks.json"},
		{ItemTypeFile, ".envrc", ".envrc"},
		{ItemTypeFile, ".env", ".env"},
	}

	if len(items) != len(expected) {
		t.Fatalf("expected %d default items, got %d", len(expected), len(items))
	}

	for i, want := range expected {
		got := items[i]
		if got.Type != want.itemType {
			t.Errorf("item[%d] type = %q, want %q", i, got.Type, want.itemType)
		}
		if got.SourcePath != want.sourcePath {
			t.Errorf("item[%d] source = %q, want %q", i, got.SourcePath, want.sourcePath)
		}
		if got.TargetPath != want.targetPath {
			t.Errorf("item[%d] target = %q, want %q", i, got.TargetPath, want.targetPath)
		}
		if !got.Enabled {
			t.Errorf("item[%d] should be enabled by default", i)
		}
	}
}

func TestLoad_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	_, err := Load()
	if err == nil {
		t.Error("Load without saved config should return an error")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	// Create the config directory and write invalid YAML
	configDir := filepath.Join(tmpDir, ".config", "repoinjector")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("{{invalid yaml:::"), 0644)

	_, err := Load()
	if err == nil {
		t.Error("Load should return an error for invalid YAML")
	}
}

func TestConfigPath_Consistency(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	path1, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	path2, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath second call failed: %v", err)
	}
	if path1 != path2 {
		t.Errorf("ConfigPath should return consistent path, got %q and %q", path1, path2)
	}
}

func TestDefaultConfig_HasSourceDir(t *testing.T) {
	cfg := DefaultConfig()
	// SourceDir is initially empty in DefaultConfig
	if cfg.SourceDir != "" {
		t.Errorf("expected empty SourceDir in default config, got %q", cfg.SourceDir)
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	got := ExpandPath("~")
	if got != home {
		t.Errorf("ExpandPath(~) = %q, want %q", got, home)
	}
}

func TestExpandPath_TildeSlash(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	got := ExpandPath("~/my-config")
	want := filepath.Join(home, "my-config")
	if got != want {
		t.Errorf("ExpandPath(~/my-config) = %q, want %q", got, want)
	}
}

func TestExpandPath_EnvVar(t *testing.T) {
	t.Setenv("TEST_EXPAND_DIR", "/tmp/test-dir")

	got := ExpandPath("$TEST_EXPAND_DIR")
	if got != "/tmp/test-dir" {
		t.Errorf("ExpandPath($TEST_EXPAND_DIR) = %q, want %q", got, "/tmp/test-dir")
	}
}

func TestExpandPath_EnvVarBraces(t *testing.T) {
	t.Setenv("TEST_EXPAND_DIR", "/tmp/test-dir")

	got := ExpandPath("${TEST_EXPAND_DIR}/subdir")
	if got != "/tmp/test-dir/subdir" {
		t.Errorf("ExpandPath(${TEST_EXPAND_DIR}/subdir) = %q, want %q", got, "/tmp/test-dir/subdir")
	}
}

func TestExpandPath_AbsoluteUnchanged(t *testing.T) {
	got := ExpandPath("/usr/local/bin")
	if got != "/usr/local/bin" {
		t.Errorf("ExpandPath(/usr/local/bin) = %q, want %q", got, "/usr/local/bin")
	}
}

func TestExpandPath_EmptyString(t *testing.T) {
	got := ExpandPath("")
	if got != "" {
		t.Errorf("ExpandPath('') = %q, want empty string", got)
	}
}

func TestExpandPath_HomeEnvVar(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")

	got := ExpandPath("$HOME/projects")
	if got != "/tmp/fakehome/projects" {
		t.Errorf("ExpandPath($HOME/projects) = %q, want %q", got, "/tmp/fakehome/projects")
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Override HOME so os.UserConfigDir() resolves to a temp location.
	// On macOS UserConfigDir returns $HOME/Library/Application Support,
	// on Linux it uses $XDG_CONFIG_HOME or $HOME/.config.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	cfg := DefaultConfig()
	cfg.SourceDir = "/some/test/path"

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file was created at the expected path
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created at %s", path)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.SourceDir != cfg.SourceDir {
		t.Errorf("expected source_dir %q, got %q", cfg.SourceDir, loaded.SourceDir)
	}
	if loaded.Mode != cfg.Mode {
		t.Errorf("expected mode %q, got %q", cfg.Mode, loaded.Mode)
	}
	if len(loaded.Items) != len(cfg.Items) {
		t.Errorf("expected %d items, got %d", len(cfg.Items), len(loaded.Items))
	}
}

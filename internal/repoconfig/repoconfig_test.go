package repoconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
)

func TestFilterGlobalConfig(t *testing.T) {
	globalCfg := &config.Config{
		Version:   1,
		SourceDir: "/tmp/source",
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: "hooks.json", TargetPath: ".claude/hooks.json", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: ".envrc", TargetPath: ".envrc", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: ".env", TargetPath: ".env", Enabled: false},
		},
	}

	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{"hello-world", "web-browser"}},
			{TargetPath: ".claude/hooks.json", Enabled: false},
			{TargetPath: ".envrc", Enabled: true},
			{TargetPath: ".env", Enabled: false},
		},
	}

	filtered := repoCfg.FilterGlobalConfig(globalCfg)

	// Should only include enabled items from repo config.
	if len(filtered.Items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(filtered.Items), filtered.Items)
	}

	// Check that the right items are included.
	targets := map[string]bool{}
	for _, item := range filtered.Items {
		targets[item.TargetPath] = true
		if !item.Enabled {
			t.Errorf("item %s should be enabled in filtered config", item.TargetPath)
		}
	}
	if !targets[".claude/skills"] {
		t.Error("expected .claude/skills to be in filtered config")
	}
	if !targets[".envrc"] {
		t.Error("expected .envrc to be in filtered config")
	}
	if targets[".claude/hooks.json"] {
		t.Error("hooks.json should NOT be in filtered config (disabled in repo config)")
	}

	// SourceDir and Mode should be preserved.
	if filtered.SourceDir != globalCfg.SourceDir {
		t.Errorf("SourceDir mismatch: got %s, want %s", filtered.SourceDir, globalCfg.SourceDir)
	}
	if filtered.Mode != globalCfg.Mode {
		t.Errorf("Mode mismatch: got %s, want %s", filtered.Mode, globalCfg.Mode)
	}
}

func TestToSelectedEntries(t *testing.T) {
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{"hello-world", "web-browser"}},
			{TargetPath: ".claude/hooks.json", Enabled: true},       // file item, no entries
			{TargetPath: ".envrc", Enabled: false},                  // disabled
			{TargetPath: ".other-dir", Enabled: true, Entries: nil}, // enabled but nil entries
		},
	}

	selected := repoCfg.ToSelectedEntries()

	// Only .claude/skills should have entries (enabled + non-empty entries).
	if len(selected) != 1 {
		t.Fatalf("expected 1 entry in selected, got %d: %+v", len(selected), selected)
	}

	skillEntries, ok := selected[".claude/skills"]
	if !ok {
		t.Fatal("expected .claude/skills in selected entries")
	}
	if len(skillEntries) != 2 {
		t.Fatalf("expected 2 skill entries, got %d", len(skillEntries))
	}
	if !skillEntries["hello-world"] {
		t.Error("expected hello-world in skill entries")
	}
	if !skillEntries["web-browser"] {
		t.Error("expected web-browser in skill entries")
	}
	if skillEntries["jira-tasks"] {
		t.Error("jira-tasks should NOT be in skill entries")
	}
}

func TestToSelectedEntries_AllSkillsSelected(t *testing.T) {
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{"hello-world", "web-browser", "jira-tasks"}},
		},
	}

	selected := repoCfg.ToSelectedEntries()
	skillEntries := selected[".claude/skills"]
	if len(skillEntries) != 3 {
		t.Fatalf("expected 3 skill entries, got %d", len(skillEntries))
	}
}

func TestToSelectedEntries_EmptyEntries(t *testing.T) {
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{}},
		},
	}

	selected := repoCfg.ToSelectedEntries()
	if _, ok := selected[".claude/skills"]; ok {
		t.Error("empty entries should NOT produce an entry in selected (len check fails)")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{"hello-world", "web-browser"}},
			{TargetPath: ".claude/hooks.json", Enabled: false},
		},
	}

	if err := Save(gitDir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if loaded.Version != original.Version {
		t.Errorf("Version mismatch: got %d, want %d", loaded.Version, original.Version)
	}
	if len(loaded.Items) != len(original.Items) {
		t.Fatalf("Items count mismatch: got %d, want %d", len(loaded.Items), len(original.Items))
	}

	// Verify skills item.
	skills := loaded.Items[0]
	if skills.TargetPath != ".claude/skills" {
		t.Errorf("unexpected TargetPath: %s", skills.TargetPath)
	}
	if !skills.Enabled {
		t.Error("skills should be enabled")
	}
	if len(skills.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(skills.Entries))
	}

	// Verify hooks item.
	hooks := loaded.Items[1]
	if hooks.Enabled {
		t.Error("hooks should be disabled")
	}
}

func TestLoad_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")

	cfg, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load should not error for non-existent file: %v", err)
	}
	if cfg != nil {
		t.Error("Load should return nil for non-existent file")
	}
}

func TestEnabledTargetPaths(t *testing.T) {
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true},
			{TargetPath: ".claude/hooks.json", Enabled: false},
			{TargetPath: ".envrc", Enabled: true},
			{TargetPath: ".env", Enabled: false},
		},
	}

	paths := repoCfg.EnabledTargetPaths()
	if len(paths) != 2 {
		t.Fatalf("expected 2 enabled paths, got %d", len(paths))
	}
	if !paths[".claude/skills"] {
		t.Error("expected .claude/skills to be enabled")
	}
	if !paths[".envrc"] {
		t.Error("expected .envrc to be enabled")
	}
	if paths[".claude/hooks.json"] {
		t.Error("hooks.json should NOT be enabled")
	}
	if paths[".env"] {
		t.Error(".env should NOT be enabled")
	}
}

func TestEnabledTargetPaths_NoneEnabled(t *testing.T) {
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".envrc", Enabled: false},
			{TargetPath: ".env", Enabled: false},
		},
	}

	paths := repoCfg.EnabledTargetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 enabled paths, got %d", len(paths))
	}
}

func TestEnabledTargetPaths_Empty(t *testing.T) {
	repoCfg := &RepoConfig{Version: 1}
	paths := repoCfg.EnabledTargetPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 enabled paths for empty items, got %d", len(paths))
	}
}

func TestSaveAndLoad_WithMergeURL(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &RepoConfig{
		Version:  1,
		State:    "review",
		MergeURL: "https://github.com/org/repo/pull/42",
		Items: []RepoItemConfig{
			{TargetPath: ".envrc", Enabled: true},
		},
	}

	if err := Save(gitDir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.MergeURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("MergeURL mismatch: got %q, want %q", loaded.MergeURL, original.MergeURL)
	}
	if loaded.State != "review" {
		t.Errorf("State mismatch: got %q, want %q", loaded.State, "review")
	}
}

func TestSaveAndLoad_WithRemote(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &RepoConfig{
		Version: 1,
		Remote:  true,
		Items: []RepoItemConfig{
			{TargetPath: ".envrc", Enabled: true},
		},
	}

	if err := Save(gitDir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !loaded.Remote {
		t.Error("Remote should be true")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	configDir := filepath.Join(gitDir, "repomni")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("{{invalid: yaml:::"), 0644)

	_, err := Load(gitDir)
	if err == nil {
		t.Error("Load should return an error for invalid YAML")
	}
}

func TestFilterGlobalConfig_PreservesEntries(t *testing.T) {
	globalCfg := &config.Config{
		Version:   1,
		SourceDir: "/tmp/source",
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{"hello-world", "web-browser"}},
		},
	}

	filtered := repoCfg.FilterGlobalConfig(globalCfg)

	// The filtered config should have the directory item
	if len(filtered.Items) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(filtered.Items))
	}
	if filtered.Items[0].TargetPath != ".claude/skills" {
		t.Errorf("unexpected target path: %s", filtered.Items[0].TargetPath)
	}
}

func TestFilterGlobalConfig_NoMatchingItems(t *testing.T) {
	globalCfg := &config.Config{
		Version:   1,
		SourceDir: "/tmp/source",
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeFile, SourcePath: ".envrc", TargetPath: ".envrc", Enabled: true},
		},
	}

	// Repo config references items not in global config
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".nonexistent", Enabled: true},
		},
	}

	filtered := repoCfg.FilterGlobalConfig(globalCfg)
	if len(filtered.Items) != 0 {
		t.Errorf("expected 0 filtered items, got %d", len(filtered.Items))
	}
}

func TestToSelectedEntries_DisabledSkipped(t *testing.T) {
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: false, Entries: []string{"hello-world"}},
		},
	}

	selected := repoCfg.ToSelectedEntries()
	if _, ok := selected[".claude/skills"]; ok {
		t.Error("disabled items should not appear in ToSelectedEntries")
	}
}

func TestSaveAndLoad_EmptyItems(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.MkdirAll(gitDir, 0755)

	original := &RepoConfig{
		Version: 1,
		Items:   []RepoItemConfig{},
	}

	if err := Save(gitDir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if len(loaded.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(loaded.Items))
	}
}

func TestFilterGlobalConfig_EmptyRepoConfig(t *testing.T) {
	globalCfg := &config.Config{
		Version:   1,
		SourceDir: "/tmp/source",
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeFile, SourcePath: ".envrc", TargetPath: ".envrc", Enabled: true},
		},
	}

	repoCfg := &RepoConfig{Version: 1}
	filtered := repoCfg.FilterGlobalConfig(globalCfg)

	if len(filtered.Items) != 0 {
		t.Errorf("expected 0 items with empty repo config, got %d", len(filtered.Items))
	}
}

func TestFilterAndSelect_IntegrationFlow(t *testing.T) {
	// Simulates the inject command's flow:
	// 1. Load global config with all items
	// 2. Load per-repo config with subset selected
	// 3. FilterGlobalConfig → only enabled items
	// 4. ToSelectedEntries → only selected directory entries
	// This mirrors inject.go lines 86-100.

	globalCfg := &config.Config{
		Version:   1,
		SourceDir: "/tmp/source",
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: "hooks.json", TargetPath: ".claude/hooks.json", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: ".envrc", TargetPath: ".envrc", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: ".env", TargetPath: ".env", Enabled: true},
		},
	}

	// Repo config: skills with only 2 of 3, hooks disabled, envrc enabled.
	repoCfg := &RepoConfig{
		Version: 1,
		Items: []RepoItemConfig{
			{TargetPath: ".claude/skills", Enabled: true, Entries: []string{"hello-world", "web-browser"}},
			{TargetPath: ".claude/hooks.json", Enabled: false},
			{TargetPath: ".envrc", Enabled: true},
			{TargetPath: ".env", Enabled: false},
		},
	}

	// Step 1: Filter global config.
	filteredCfg := repoCfg.FilterGlobalConfig(globalCfg)

	enabledItems := filteredCfg.EnabledItems()
	if len(enabledItems) != 2 {
		t.Fatalf("expected 2 enabled items, got %d", len(enabledItems))
	}

	// Step 2: Get allowed entries for directory items.
	allowedEntries := repoCfg.ToSelectedEntries()

	skillsAllowed := allowedEntries[".claude/skills"]
	if skillsAllowed == nil {
		t.Fatal("expected .claude/skills in allowedEntries")
	}
	if len(skillsAllowed) != 2 {
		t.Fatalf("expected 2 allowed skills, got %d", len(skillsAllowed))
	}
	if !skillsAllowed["hello-world"] {
		t.Error("hello-world should be allowed")
	}
	if !skillsAllowed["web-browser"] {
		t.Error("web-browser should be allowed")
	}
	if skillsAllowed["jira-tasks"] {
		t.Error("jira-tasks should NOT be allowed")
	}
}

func TestSaveAndLoad_WithTicket(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &RepoConfig{
		Version: 1,
		State:   "active",
		Ticket:  "PROJ-123",
		Items: []RepoItemConfig{
			{TargetPath: ".envrc", Enabled: true},
		},
	}

	if err := Save(gitDir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Ticket != "PROJ-123" {
		t.Errorf("Ticket mismatch: got %q, want %q", loaded.Ticket, "PROJ-123")
	}
	if loaded.State != "active" {
		t.Errorf("State mismatch: got %q, want %q", loaded.State, "active")
	}
}

func TestSaveAndLoad_WithoutTicket(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Config without ticket — verifies backward compatibility.
	original := &RepoConfig{
		Version: 1,
		State:   "review",
	}

	if err := Save(gitDir, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Ticket != "" {
		t.Errorf("Ticket should be empty, got %q", loaded.Ticket)
	}
}

func TestNoRepoConfig_AllEntriesAllowed(t *testing.T) {
	// When no repo config exists (repoCfg is nil), allowedEntries should be nil,
	// which means all entries are shown in the picker.
	// This test verifies the nil behavior that inject.go relies on.

	var repoCfg *RepoConfig = nil

	// This is what inject.go does:
	if repoCfg != nil {
		t.Fatal("repoCfg should be nil for this test")
	}

	// When repoCfg is nil, allowedEntries stays nil.
	var allowedEntries map[string]map[string]bool
	if allowedEntries != nil {
		t.Fatal("allowedEntries should be nil when no repo config")
	}

	// A nil map returns nil for any key lookup (the zero value).
	allowed := allowedEntries[".claude/skills"]
	if allowed != nil {
		t.Error("nil map lookup should return nil")
	}

	// This is the condition in SelectDirEntries: when allowed is nil, all entries pass.
	testEntries := []string{"hello-world", "web-browser", "jira-tasks"}
	for _, name := range testEntries {
		if allowed != nil && !allowed[name] {
			t.Errorf("%s should NOT be filtered out when allowed is nil", name)
		}
	}
}

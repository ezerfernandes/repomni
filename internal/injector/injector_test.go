package injector

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ezer/repoinjector/internal/config"
)

func setupTestEnv(t *testing.T) (sourceDir, targetDir string, cfg *config.Config) {
	t.Helper()

	// Create a root directory that contains both source and target as subdirs.
	// .env and .envrc are placed in rootDir (parent of target) so that
	// findEnvInParents discovers them when walking up from targetDir.
	rootDir := t.TempDir()
	sourceDir = filepath.Join(rootDir, "source")
	targetDir = filepath.Join(rootDir, "target")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Init git repo in target
	cmd := exec.Command("git", "init", targetDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Create source files (defaults: skills/, hooks.json at source root)
	os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644)

	// Place .env and .envrc in rootDir (parent of target) for parent search
	os.WriteFile(filepath.Join(rootDir, ".envrc"), []byte("export FOO=bar"), 0644)
	os.WriteFile(filepath.Join(rootDir, ".env"), []byte("SECRET=123"), 0644)

	cfg = &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items:     config.DefaultItems(),
	}

	return
}

func TestInjectSymlink(t *testing.T) {
	sourceDir, targetDir, cfg := setupTestEnv(t)

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "created" {
			t.Errorf("expected action 'created' for %s, got %q: %s", r.Item.TargetPath, r.Action, r.Detail)
		}
	}

	// Verify .envrc symlinks to parent directory (found via parent search)
	link, err := os.Readlink(filepath.Join(targetDir, ".envrc"))
	if err != nil {
		t.Fatalf("cannot read symlink: %v", err)
	}
	rootDir := filepath.Dir(targetDir)
	expected := filepath.Join(rootDir, ".envrc")
	if link != expected {
		t.Errorf("expected symlink to %q, got %q", expected, link)
	}

	// Verify per-entry skill symlink (not whole directory)
	skillPath := filepath.Join(targetDir, ".claude", "skills", "test.md")
	link, err = os.Readlink(skillPath)
	if err != nil {
		t.Fatalf("cannot read skill symlink: %v", err)
	}
	expected = filepath.Join(sourceDir, "skills", "test.md")
	if link != expected {
		t.Errorf("expected symlink to %q, got %q", expected, link)
	}

	// Verify .claude/skills is a real directory, not a symlink
	info, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills"))
	if err != nil {
		t.Fatalf("cannot stat .claude/skills: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error(".claude/skills should be a real directory, not a symlink")
	}
}

func TestInjectIdempotent(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("second Inject failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "skipped" {
			t.Errorf("expected 'skipped' on second inject for %s, got %q", r.Item.TargetPath, r.Action)
		}
	}
}

func TestInjectCopy(t *testing.T) {
	sourceDir, targetDir, cfg := setupTestEnv(t)

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
	if err != nil {
		t.Fatalf("Inject copy failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "created" {
			t.Errorf("expected 'created' for %s, got %q", r.Item.TargetPath, r.Action)
		}
	}

	// Verify .envrc is a real file, not a symlink
	info, err := os.Lstat(filepath.Join(targetDir, ".envrc"))
	if err != nil {
		t.Fatalf("cannot stat .envrc: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error(".envrc should be a regular file, not a symlink")
	}

	// Verify content matches (source is in parent directory)
	rootDir := filepath.Dir(targetDir)
	src, _ := os.ReadFile(filepath.Join(rootDir, ".envrc"))
	dst, _ := os.ReadFile(filepath.Join(targetDir, ".envrc"))
	if string(src) != string(dst) {
		t.Error("copied file content does not match source")
	}

	// Verify per-entry skill copy
	skillDst := filepath.Join(targetDir, ".claude", "skills", "test.md")
	info, err = os.Lstat(skillDst)
	if err != nil {
		t.Fatalf("cannot stat copied skill: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("skill should be a regular file in copy mode, not a symlink")
	}

	srcContent, _ := os.ReadFile(filepath.Join(sourceDir, "skills", "test.md"))
	dstContent, _ := os.ReadFile(skillDst)
	if string(srcContent) != string(dstContent) {
		t.Error("copied skill content does not match source")
	}
}

func TestInjectDryRun(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	results, err := Inject(cfg, targetDir, Options{DryRun: true})
	if err != nil {
		t.Fatalf("Inject dry-run failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "dry-run" {
			t.Errorf("expected 'dry-run' for %s, got %q", r.Item.TargetPath, r.Action)
		}
	}

	// Verify nothing was created
	if _, err := os.Stat(filepath.Join(targetDir, ".envrc")); err == nil {
		t.Error(".envrc should not exist after dry run")
	}
}

func TestInjectRejectsSameDir(t *testing.T) {
	sourceDir, _, cfg := setupTestEnv(t)
	cfg.SourceDir = sourceDir

	// Make source a git repo too
	exec.Command("git", "init", sourceDir).Run()

	_, err := Inject(cfg, sourceDir, Options{})
	if err == nil {
		t.Error("expected error when source == target")
	}
}

func TestInjectSelectedEntries(t *testing.T) {
	sourceDir, targetDir, cfg := setupTestEnv(t)

	// Add a second skill to the source
	os.WriteFile(filepath.Join(sourceDir, "skills", "another.md"), []byte("another skill"), 0644)

	// Only select "test.md", deselect "another.md"
	opts := Options{
		Mode: config.ModeSymlink,
		SelectedEntries: map[string]map[string]bool{
			".claude/skills": {"test.md": true},
		},
	}

	results, err := Inject(cfg, targetDir, opts)
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// test.md should be created
	var foundCreated, foundAnother bool
	for _, r := range results {
		if r.Item.TargetPath == ".claude/skills/test.md" && r.Action == "created" {
			foundCreated = true
		}
		if r.Item.TargetPath == ".claude/skills/another.md" {
			foundAnother = true
		}
	}
	if !foundCreated {
		t.Error("expected test.md to be created")
	}
	if foundAnother {
		t.Error("another.md should not appear in results when deselected")
	}

	// Verify test.md symlink exists
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "test.md")); err != nil {
		t.Error("test.md should exist")
	}

	// Verify another.md was NOT created
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "another.md")); err == nil {
		t.Error("another.md should NOT exist when deselected")
	}
}

func TestInjectSelectedEntries_RepoConfigFlow(t *testing.T) {
	// Simulates the full per-repo config flow: configure selects 2 of 3 skills,
	// then inject with SelectedEntries from repo config only injects those 2.
	sourceDir, targetDir, _ := setupTestEnv(t)

	// Create 3 skills in source.
	os.MkdirAll(filepath.Join(sourceDir, "skills", "hello-world"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "hello-world", "README.md"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "web-browser"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "web-browser", "README.md"), []byte("browser"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "jira-tasks"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "jira-tasks", "README.md"), []byte("jira"), 0644)

	// Filtered config (as FilterGlobalConfig would produce): only skills enabled.
	filteredCfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	// SelectedEntries from ToSelectedEntries: only hello-world and web-browser.
	opts := Options{
		Mode: config.ModeSymlink,
		SelectedEntries: map[string]map[string]bool{
			".claude/skills": {
				"hello-world": true,
				"web-browser": true,
			},
		},
	}

	results, err := Inject(filteredCfg, targetDir, opts)
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	injected := map[string]string{}
	for _, r := range results {
		injected[filepath.Base(r.Item.TargetPath)] = r.Action
	}

	if injected["hello-world"] != "created" {
		t.Errorf("expected hello-world to be created, got %q", injected["hello-world"])
	}
	if injected["web-browser"] != "created" {
		t.Errorf("expected web-browser to be created, got %q", injected["web-browser"])
	}
	if _, found := injected["jira-tasks"]; found {
		t.Error("jira-tasks should NOT appear in results at all (excluded by SelectedEntries)")
	}

	// Verify on disk.
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "hello-world")); err != nil {
		t.Error("hello-world should exist on disk")
	}
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "web-browser")); err != nil {
		t.Error("web-browser should exist on disk")
	}
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "jira-tasks")); err == nil {
		t.Error("jira-tasks should NOT exist on disk")
	}
}

func TestInjectWithoutSelectedEntries_InjectsAllSkills(t *testing.T) {
	// When no repo config exists, all skills should be injected.
	sourceDir, targetDir, _ := setupTestEnv(t)

	os.MkdirAll(filepath.Join(sourceDir, "skills", "hello-world"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "hello-world", "README.md"), []byte("hello"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "web-browser"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "web-browser", "README.md"), []byte("browser"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "jira-tasks"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "jira-tasks", "README.md"), []byte("jira"), 0644)

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	// No SelectedEntries (no repo config).
	opts := Options{Mode: config.ModeSymlink}

	results, err := Inject(cfg, targetDir, opts)
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	injected := map[string]string{}
	for _, r := range results {
		injected[filepath.Base(r.Item.TargetPath)] = r.Action
	}

	// All 4 skills should be injected (test.md from setupTestEnv + our 3).
	for _, skill := range []string{"test.md", "hello-world", "web-browser", "jira-tasks"} {
		if injected[skill] != "created" {
			t.Errorf("expected %s to be created, got %q", skill, injected[skill])
		}
	}
}

func TestInjectPreservesExistingSkills(t *testing.T) {
	sourceDir, targetDir, cfg := setupTestEnv(t)

	// Pre-create .claude/skills with an existing skill
	existingSkillDir := filepath.Join(targetDir, ".claude", "skills")
	os.MkdirAll(existingSkillDir, 0755)
	os.WriteFile(filepath.Join(existingSkillDir, "existing.md"), []byte("existing skill"), 0644)

	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// The existing skill should still be there (untouched)
	content, err := os.ReadFile(filepath.Join(existingSkillDir, "existing.md"))
	if err != nil {
		t.Fatal("existing skill was removed")
	}
	if string(content) != "existing skill" {
		t.Error("existing skill content was modified")
	}

	// The injected skill should be a symlink
	link, err := os.Readlink(filepath.Join(existingSkillDir, "test.md"))
	if err != nil {
		t.Fatalf("injected skill symlink not created: %v", err)
	}
	expected := filepath.Join(sourceDir, "skills", "test.md")
	if link != expected {
		t.Errorf("expected symlink to %q, got %q", expected, link)
	}
}

func TestInjectConflictingSkillWarning(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	// Pre-create a skill with the SAME name as one in source
	existingSkillDir := filepath.Join(targetDir, ".claude", "skills")
	os.MkdirAll(existingSkillDir, 0755)
	os.WriteFile(filepath.Join(existingSkillDir, "test.md"), []byte("my local skill"), 0644)

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Find the result for the conflicting skill
	var skillResult *Result
	for _, r := range results {
		if r.Item.TargetPath == ".claude/skills/test.md" {
			skillResult = &r
			break
		}
	}

	if skillResult == nil {
		t.Fatal("no result for .claude/skills/test.md")
	}

	if skillResult.Action != "warning" {
		t.Errorf("expected 'warning' for conflicting skill, got %q: %s", skillResult.Action, skillResult.Detail)
	}

	// The original file should be preserved
	content, err := os.ReadFile(filepath.Join(existingSkillDir, "test.md"))
	if err != nil {
		t.Fatal("conflicting skill was removed")
	}
	if string(content) != "my local skill" {
		t.Error("conflicting skill content was modified")
	}
}

func TestEject(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})

	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "removed" {
			t.Errorf("expected 'removed' for %s, got %q: %s", r.Item.TargetPath, r.Action, r.Detail)
		}
	}

	// Verify files are gone
	if _, err := os.Stat(filepath.Join(targetDir, ".envrc")); err == nil {
		t.Error(".envrc should be removed after eject")
	}

	// Verify individual skill symlink is gone
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "test.md")); err == nil {
		t.Error(".claude/skills/test.md should be removed after eject")
	}

	// Verify .claude dir was cleaned up (skills dir empty -> removed, then .claude empty -> removed)
	if _, err := os.Stat(filepath.Join(targetDir, ".claude")); err == nil {
		t.Error(".claude should be cleaned up after eject")
	}
}

func TestEjectPreservesExistingSkills(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	// Pre-create an existing skill
	existingSkillDir := filepath.Join(targetDir, ".claude", "skills")
	os.MkdirAll(existingSkillDir, 0755)
	os.WriteFile(filepath.Join(existingSkillDir, "existing.md"), []byte("keep me"), 0644)

	// Inject (will add test.md symlink alongside existing.md)
	Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})

	// Eject
	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	// The injected skill should be removed
	foundRemoved := false
	for _, r := range results {
		if r.Item.TargetPath == ".claude/skills/test.md" && r.Action == "removed" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Error("expected injected skill test.md to be removed")
	}

	// The existing skill should still be there
	content, err := os.ReadFile(filepath.Join(existingSkillDir, "existing.md"))
	if err != nil {
		t.Fatal("existing skill was removed during eject")
	}
	if string(content) != "keep me" {
		t.Error("existing skill content was modified during eject")
	}

	// .claude/skills should NOT be removed (still has existing.md)
	if _, err := os.Stat(existingSkillDir); err != nil {
		t.Error(".claude/skills should still exist because it has non-injected content")
	}
}

func TestInjectEnvNotFoundInParentGitRepo(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	targetDir := filepath.Join(rootDir, "target")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	// Make rootDir a git repo (parent git repo boundary) with no .env/.envrc
	exec.Command("git", "init", rootDir).Run()
	exec.Command("git", "init", targetDir).Run()

	os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644)

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items:     config.DefaultItems(),
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	for _, r := range results {
		if r.Item.TargetPath == ".env" || r.Item.TargetPath == ".envrc" {
			if r.Action != "skipped" {
				t.Errorf("expected 'skipped' for %s, got %q", r.Item.TargetPath, r.Action)
			}
			if r.Detail != "not found in parent git repository" {
				t.Errorf("expected 'not found in parent git repository' for %s, got %q", r.Item.TargetPath, r.Detail)
			}
		}
	}
}

func TestInjectEnvPartialFind(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	targetDir := filepath.Join(rootDir, "target")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)

	exec.Command("git", "init", targetDir).Run()

	// Only place .env in rootDir, not .envrc
	os.WriteFile(filepath.Join(rootDir, ".env"), []byte("SECRET=123"), 0644)

	os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644)

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items:     config.DefaultItems(),
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	for _, r := range results {
		if r.Item.TargetPath == ".env" && r.Action != "created" {
			t.Errorf("expected 'created' for .env, got %q: %s", r.Action, r.Detail)
		}
		if r.Item.TargetPath == ".envrc" && r.Action != "skipped" {
			t.Errorf("expected 'skipped' for .envrc, got %q: %s", r.Action, r.Detail)
		}
	}
}

func TestStatus(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	// Before inject
	statuses, err := Status(cfg, targetDir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	for _, s := range statuses {
		if s.Present {
			t.Errorf("%s should not be present before inject", s.Item.TargetPath)
		}
	}

	// After inject
	Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	statuses, err = Status(cfg, targetDir)
	if err != nil {
		t.Fatalf("Status after inject failed: %v", err)
	}
	for _, s := range statuses {
		if !s.Present {
			t.Errorf("%s should be present after inject", s.Item.TargetPath)
		}
		if !s.Current {
			t.Errorf("%s should be current after inject", s.Item.TargetPath)
		}
		if !s.Excluded {
			t.Errorf("%s should be excluded after inject", s.Item.TargetPath)
		}
	}
}

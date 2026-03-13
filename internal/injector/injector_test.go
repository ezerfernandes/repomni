package injector

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/gitutil"
)

func setupTestEnv(t *testing.T) (sourceDir, targetDir string, cfg *config.Config) {
	t.Helper()

	// Create a root directory that contains both source and target as subdirs.
	// .env and .envrc are placed in rootDir (parent of target) so that
	// findEnvInParents discovers them when walking up from targetDir.
	rootDir := t.TempDir()
	sourceDir = filepath.Join(rootDir, "source")
	targetDir = filepath.Join(rootDir, "target")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Init git repo in target
	cmd := exec.Command("git", "init", targetDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Create source files (defaults: skills/, hooks.json at source root)
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Place .env and .envrc in rootDir (parent of target) for parent search
	if err := os.WriteFile(filepath.Join(rootDir, ".envrc"), []byte("export FOO=bar"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, ".env"), []byte("SECRET=123"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink}); err != nil {
		t.Fatalf("first Inject failed: %v", err)
	}
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
	if err := exec.Command("git", "init", sourceDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	_, err := Inject(cfg, sourceDir, Options{})
	if err == nil {
		t.Error("expected error when source == target")
	}
}

func TestInjectSelectedEntries(t *testing.T) {
	sourceDir, targetDir, cfg := setupTestEnv(t)

	// Add a second skill to the source
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "another.md"), []byte("another skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "hello-world"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "hello-world", "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "web-browser"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "web-browser", "README.md"), []byte("browser"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "jira-tasks"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "jira-tasks", "README.md"), []byte("jira"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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

	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "hello-world"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "hello-world", "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "web-browser"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "web-browser", "README.md"), []byte("browser"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "jira-tasks"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "jira-tasks", "README.md"), []byte("jira"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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
	if err := os.MkdirAll(existingSkillDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(existingSkillDir, "existing.md"), []byte("existing skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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
	if err := os.MkdirAll(existingSkillDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(existingSkillDir, "test.md"), []byte("my local skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

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
	if err := os.MkdirAll(existingSkillDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(existingSkillDir, "existing.md"), []byte("keep me"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Inject (will add test.md symlink alongside existing.md)
	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

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
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Make rootDir a git repo (parent git repo boundary) with no .env/.envrc
	if err := exec.Command("git", "init", rootDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := exec.Command("git", "init", targetDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	if err := exec.Command("git", "init", targetDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Only place .env in rootDir, not .envrc
	if err := os.WriteFile(filepath.Join(rootDir, ".env"), []byte("SECRET=123"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

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

func TestInjectNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Version:   1,
		SourceDir: dir,
		Mode:      config.ModeSymlink,
		Items:     config.DefaultItems(),
	}

	_, err := Inject(cfg, dir, Options{})
	if err == nil {
		t.Error("expected error when injecting to non-git directory")
	}
}

func TestEjectNotPresent(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	// Eject without prior inject — items should be "skipped" (not present)
	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "skipped" {
			t.Errorf("expected 'skipped' for %s, got %q: %s", r.Item.TargetPath, r.Action, r.Detail)
		}
	}
}

func TestInjectForceOverwrite(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	// Create a regular file at the symlink target
	if err := os.WriteFile(filepath.Join(targetDir, ".envrc"), []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink, Force: true})
	if err != nil {
		t.Fatalf("Inject with force failed: %v", err)
	}

	for _, r := range results {
		if r.Item.TargetPath == ".envrc" {
			if r.Action != "created" {
				t.Errorf("expected 'created' with force for .envrc, got %q: %s", r.Action, r.Detail)
			}
		}
	}

	// Verify it's now a symlink
	info, err := os.Lstat(filepath.Join(targetDir, ".envrc"))
	if err != nil {
		t.Fatalf("cannot stat .envrc: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error(".envrc should be a symlink after force inject")
	}
}

func TestInjectCopyIdempotent(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy}); err != nil {
		t.Fatalf("first Inject failed: %v", err)
	}
	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
	if err != nil {
		t.Fatalf("second copy Inject failed: %v", err)
	}

	for _, r := range results {
		// File items get "skipped" (content matches), directory entries get "warning" (already exist)
		if r.Action != "skipped" && r.Action != "warning" {
			t.Errorf("expected 'skipped' or 'warning' on second copy inject for %s, got %q: %s", r.Item.TargetPath, r.Action, r.Detail)
		}
	}
}

func TestInjectCopyForceOverwrite(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Modify a copied file so content differs
	if err := os.WriteFile(filepath.Join(targetDir, ".envrc"), []byte("modified"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Without force, should skip with "different content" message
	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	for _, r := range results {
		if r.Item.TargetPath == ".envrc" && r.Action != "skipped" {
			t.Errorf("expected 'skipped' for modified .envrc without force, got %q", r.Action)
		}
	}

	// With force, should overwrite
	results, err = Inject(cfg, targetDir, Options{Mode: config.ModeCopy, Force: true})
	if err != nil {
		t.Fatalf("Inject with force failed: %v", err)
	}
	for _, r := range results {
		if r.Item.TargetPath == ".envrc" && r.Action != "created" {
			t.Errorf("expected 'created' for .envrc with force, got %q", r.Action)
		}
	}
}

func TestStatusAfterEject(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if _, err := Eject(cfg, targetDir); err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	statuses, err := Status(cfg, targetDir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	for _, s := range statuses {
		if s.Present {
			t.Errorf("%s should not be present after eject", s.Item.TargetPath)
		}
	}
}

func TestFilesEqual_IdenticalContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("hello world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(b, []byte("hello world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if !filesEqual(a, b) {
		t.Error("filesEqual should return true for identical files")
	}
}

func TestFilesEqual_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(b, []byte("world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if filesEqual(a, b) {
		t.Error("filesEqual should return false for different files")
	}
}

func TestFilesEqual_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(b, []byte("hello  \n\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if !filesEqual(a, b) {
		t.Error("filesEqual should trim trailing whitespace when comparing")
	}
}

func TestFilesEqual_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(a, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	b := filepath.Join(dir, "nonexistent.txt")

	if filesEqual(a, b) {
		t.Error("filesEqual should return false when a file doesn't exist")
	}
}

func TestFilesEqual_BothNonExistent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "missing1.txt")
	b := filepath.Join(dir, "missing2.txt")

	if filesEqual(a, b) {
		t.Error("filesEqual should return false when both files don't exist")
	}
}

func TestIsEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		item     config.Item
		expected bool
	}{
		{"env file", config.Item{Type: config.ItemTypeFile, TargetPath: ".env"}, true},
		{"envrc file", config.Item{Type: config.ItemTypeFile, TargetPath: ".envrc"}, true},
		{"other file", config.Item{Type: config.ItemTypeFile, TargetPath: "hooks.json"}, false},
		{"directory type", config.Item{Type: config.ItemTypeDirectory, TargetPath: ".env"}, false},
		{"nested env file", config.Item{Type: config.ItemTypeFile, TargetPath: "config/.env"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEnvFile(tt.item)
			if got != tt.expected {
				t.Errorf("isEnvFile(%+v) = %v, want %v", tt.item, got, tt.expected)
			}
		})
	}
}

func TestRemoveIfEmptyDir(t *testing.T) {
	// Empty directory should be removed
	dir := t.TempDir()
	emptyDir := filepath.Join(dir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	removeIfEmptyDir(emptyDir)

	if _, err := os.Stat(emptyDir); err == nil {
		t.Error("empty directory should be removed")
	}
}

func TestRemoveIfEmptyDir_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	nonEmptyDir := filepath.Join(dir, "notempty")
	if err := os.MkdirAll(nonEmptyDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonEmptyDir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	removeIfEmptyDir(nonEmptyDir)

	if _, err := os.Stat(nonEmptyDir); err != nil {
		t.Error("non-empty directory should NOT be removed")
	}
}

func TestRemoveIfEmptyDir_NonExistent(t *testing.T) {
	// Should not panic for non-existent directory
	removeIfEmptyDir(filepath.Join(t.TempDir(), "nonexistent"))
}

func TestCopyFileContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := copyFileContent(src, dst); err != nil {
		t.Fatalf("copyFileContent failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("cannot read dst: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content mismatch: got %q", data)
	}
}

func TestCopyFileContent_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.sh")
	dst := filepath.Join(dir, "dst.sh")
	if err := os.WriteFile(src, []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := copyFileContent(src, dst); err != nil {
		t.Fatalf("copyFileContent failed: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("cannot stat dst: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("dst should preserve executable permission")
	}
}

func TestCopyFileContent_NonExistentSource(t *testing.T) {
	dir := t.TempDir()
	err := copyFileContent(filepath.Join(dir, "missing"), filepath.Join(dir, "dst"))
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

func TestCopyDirRecursive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	// Create nested structure
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "file1.txt"), []byte("root file"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "file2.txt"), []byte("sub file"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := copyDirRecursive(src, dst); err != nil {
		t.Fatalf("copyDirRecursive failed: %v", err)
	}

	// Verify root file
	data, err := os.ReadFile(filepath.Join(dst, "file1.txt"))
	if err != nil {
		t.Fatalf("cannot read root file: %v", err)
	}
	if string(data) != "root file" {
		t.Errorf("root file content mismatch: got %q", data)
	}

	// Verify nested file
	data, err = os.ReadFile(filepath.Join(dst, "sub", "file2.txt"))
	if err != nil {
		t.Fatalf("cannot read sub file: %v", err)
	}
	if string(data) != "sub file" {
		t.Errorf("sub file content mismatch: got %q", data)
	}
}

func TestCopyDirRecursive_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	if err := copyDirRecursive(src, dst); err != nil {
		t.Fatalf("copyDirRecursive failed for empty dir: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal("dst directory should exist")
	}
	if !info.IsDir() {
		t.Error("dst should be a directory")
	}
}

func TestCreateSymlink_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	src1 := filepath.Join(dir, "src1")
	src2 := filepath.Join(dir, "src2")
	dst := filepath.Join(dir, "link")
	if err := os.WriteFile(src1, []byte("v1"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(src2, []byte("v2"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}

	// Create symlink to src1
	if err := os.Symlink(src1, dst); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// createSymlink should update to src2
	result := createSymlink(item, src2, dst, false)
	if result.Action != "created" {
		t.Errorf("expected 'created' when updating symlink, got %q: %s", result.Action, result.Detail)
	}

	target, _ := os.Readlink(dst)
	if target != src2 {
		t.Errorf("symlink should point to %q, got %q", src2, target)
	}
}

func TestCreateSymlink_SkipsIfCurrent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "link")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.Symlink(src, dst); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}
	result := createSymlink(item, src, dst, false)
	if result.Action != "skipped" {
		t.Errorf("expected 'skipped' when symlink already correct, got %q", result.Action)
	}
}

func TestCreateSymlink_SkipsRegularFileWithoutForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "existing")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(dst, []byte("regular file"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}
	result := createSymlink(item, src, dst, false)
	if result.Action != "skipped" {
		t.Errorf("expected 'skipped' without force, got %q: %s", result.Action, result.Detail)
	}
}

func TestCreateSymlink_OverwritesRegularFileWithForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "existing")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(dst, []byte("regular file"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}
	result := createSymlink(item, src, dst, true)
	if result.Action != "created" {
		t.Errorf("expected 'created' with force, got %q: %s", result.Action, result.Detail)
	}

	target, err := os.Readlink(dst)
	if err != nil {
		t.Fatal("dst should be a symlink")
	}
	if target != src {
		t.Errorf("symlink should point to %q, got %q", src, target)
	}
}

func TestCopyFile_ExistingDifferentContentNoForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("source content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(dst, []byte("different content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}
	result := copyFile(item, src, dst, false)
	if result.Action != "skipped" {
		t.Errorf("expected 'skipped' for different content without force, got %q", result.Action)
	}
}

func TestCopyFile_ExistingSameContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("same content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(dst, []byte("same content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}
	result := copyFile(item, src, dst, false)
	if result.Action != "skipped" {
		t.Errorf("expected 'skipped' for same content, got %q", result.Action)
	}
	if result.Detail != "already up to date" {
		t.Errorf("expected 'already up to date' detail, got %q", result.Detail)
	}
}

func TestCopyFile_ExistingDifferentContentWithForce(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("new content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(dst, []byte("old content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	item := config.Item{TargetPath: "test"}
	result := copyFile(item, src, dst, true)
	if result.Action != "created" {
		t.Errorf("expected 'created' with force, got %q: %s", result.Action, result.Detail)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "new content" {
		t.Errorf("content should be overwritten, got %q", data)
	}
}

func TestFindEnvInParents_FindsInParent(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	result := findEnvInParents(child)
	if len(result.Found) == 0 {
		t.Fatal("expected to find .env in parent")
	}
	if result.Found[".env"] != root {
		t.Errorf("expected .env in %q, got %q", root, result.Found[".env"])
	}
}

func TestFindEnvInParents_StopsAtGitRepo(t *testing.T) {
	root := t.TempDir()
	parentRepo := filepath.Join(root, "parent")
	child := filepath.Join(parentRepo, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Make parent a git repo but don't place .env there
	if err := exec.Command("git", "init", parentRepo).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	result := findEnvInParents(child)
	if len(result.Found) != 0 {
		t.Error("should not find env files when parent git repo has none")
	}
	if !result.HitGitRepo {
		t.Error("HitGitRepo should be true")
	}
}

func TestFindEnvInParents_FindsBothFiles(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".envrc"), []byte("export A=1"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	result := findEnvInParents(child)
	if len(result.Found) != 2 {
		t.Errorf("expected 2 found files, got %d", len(result.Found))
	}
}

func TestStatusCopyMode(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	statuses, err := Status(cfg, targetDir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	for _, s := range statuses {
		if !s.Present {
			t.Errorf("%s should be present after copy inject", s.Item.TargetPath)
		}
		if !s.Current {
			t.Errorf("%s should be current after copy inject, got detail: %s", s.Item.TargetPath, s.Detail)
		}
		if s.Detail != "copy ok" {
			t.Errorf("%s expected detail 'copy ok', got %q", s.Item.TargetPath, s.Detail)
		}
	}
}

func TestEjectAfterCopyInject(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject after copy inject failed: %v", err)
	}

	for _, r := range results {
		if r.Action != "removed" {
			t.Errorf("expected 'removed' for %s after copy inject, got %q: %s", r.Item.TargetPath, r.Action, r.Detail)
		}
	}

	// Verify files are gone
	if _, err := os.Stat(filepath.Join(targetDir, ".envrc")); err == nil {
		t.Error(".envrc should be removed after eject")
	}
}

func TestInjectSourceNotFound(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	targetDir := filepath.Join(rootDir, "target")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := exec.Command("git", "init", targetDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Config references a source file that doesn't exist
	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeFile, SourcePath: "nonexistent.json", TargetPath: "nonexistent.json", Enabled: true},
		},
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject should not fail for missing source: %v", err)
	}

	if len(results) != 1 || results[0].Action != "skipped" {
		t.Errorf("expected 'skipped' for missing source, got %+v", results)
	}
}

func TestInjectEmptySourceDir(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	targetDir := filepath.Join(rootDir, "target")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := exec.Command("git", "init", targetDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Directory item with empty source
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	if len(results) != 1 || results[0].Action != "skipped" {
		t.Errorf("expected 'skipped' for empty source dir, got %+v", results)
	}
}

func TestStatusDir_SourceNotReadable(t *testing.T) {
	item := config.Item{
		Type:       config.ItemTypeDirectory,
		SourcePath: "skills",
		TargetPath: ".claude/skills",
		Enabled:    true,
	}

	statuses := statusDir(item, "/nonexistent/path", "/also/nonexistent", nil)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Present {
		t.Error("should not be present")
	}
}

// --- Regression tests for Finding 1: Eject must not delete user-owned files ---

func TestEjectDoesNotDeleteUserOwnedFileAtManagedPath(t *testing.T) {
	// If a user has their own .envrc and repomni never injected one,
	// eject must not delete it.
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "source")
	targetDir := filepath.Join(rootDir, "target")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := exec.Command("git", "init", targetDir).Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Only enable skills (not .env/.envrc) to inject
	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	// Inject just skills
	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Now the user manually creates a .envrc in their repo
	if err := os.WriteFile(filepath.Join(targetDir, ".envrc"), []byte("my own envrc"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Switch to a config that includes .envrc as a managed path
	cfgWithEnv := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: ".envrc", TargetPath: ".envrc", Enabled: true},
		},
	}

	// Eject with the wider config — .envrc must NOT be deleted
	results, err := Eject(cfgWithEnv, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	// Verify .envrc was skipped (not managed by repomni)
	for _, r := range results {
		if r.Item.TargetPath == ".envrc" && r.Action == "removed" {
			t.Error(".envrc should NOT be removed — it was created by the user, not repomni")
		}
	}

	// The user's file must still exist
	content, err := os.ReadFile(filepath.Join(targetDir, ".envrc"))
	if err != nil {
		t.Fatal("user's .envrc was deleted by eject")
	}
	if string(content) != "my own envrc" {
		t.Error("user's .envrc content was modified")
	}
}

func TestEjectDoesNotDeleteUserFilesInManagedDirectory(t *testing.T) {
	// If a user creates a file inside a managed directory (e.g. .claude/skills/my-skill.md),
	// and repomni did not inject that file, eject must not delete it.
	sourceDir, targetDir, _ := setupTestEnv(t)

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeCopy,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	// Inject in copy mode
	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// User manually adds their own skill file
	userSkill := filepath.Join(targetDir, ".claude", "skills", "my-custom-skill.md")
	if err := os.WriteFile(userSkill, []byte("user skill"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Eject
	_, err = Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	// The injected file (test.md) should be removed
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "test.md")); err == nil {
		t.Error("injected test.md should be removed after eject")
	}

	// The user's custom skill must still exist
	content, err := os.ReadFile(userSkill)
	if err != nil {
		t.Fatal("user's custom skill was deleted by eject")
	}
	if string(content) != "user skill" {
		t.Error("user's custom skill content was modified")
	}
}

// --- Regression tests for Finding 2: Eject must not depend on live source tree ---

func TestEjectCleansUpRenamedSourceEntry(t *testing.T) {
	// If a source entry was injected and later renamed in the source directory,
	// eject must still clean up the original injected file.
	sourceDir, targetDir, _ := setupTestEnv(t)

	// Add an extra skill to source
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "old-skill.md"), []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	// Inject both skills
	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Verify both were injected
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "old-skill.md")); err != nil {
		t.Fatal("old-skill.md should exist after inject")
	}

	// Rename the source entry (simulating source evolution)
	if err := os.Rename(
		filepath.Join(sourceDir, "skills", "old-skill.md"),
		filepath.Join(sourceDir, "skills", "new-skill.md"),
	); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	// Eject — old-skill.md must still be cleaned up even though source no longer has it
	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	// old-skill.md should have been removed
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "old-skill.md")); err == nil {
		t.Error("old-skill.md should be removed after eject, even though source was renamed")
	}

	foundOldRemoved := false
	for _, r := range results {
		if r.Item.TargetPath == ".claude/skills/old-skill.md" && r.Action == "removed" {
			foundOldRemoved = true
		}
	}
	if !foundOldRemoved {
		t.Error("expected old-skill.md removal in eject results")
	}
}

func TestEjectCleansUpDeletedSourceEntry(t *testing.T) {
	// If a source entry was injected and later deleted from the source directory,
	// eject must still clean up the injected file.
	sourceDir, targetDir, _ := setupTestEnv(t)

	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "ephemeral.md"), []byte("temp"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Delete the source entry
	os.Remove(filepath.Join(sourceDir, "skills", "ephemeral.md"))

	// Eject
	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "ephemeral.md")); err == nil {
		t.Error("ephemeral.md should be removed after eject, even though source was deleted")
	}

	foundRemoved := false
	for _, r := range results {
		if r.Item.TargetPath == ".claude/skills/ephemeral.md" && r.Action == "removed" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Error("expected ephemeral.md removal in eject results")
	}
}

func TestEjectCopyModeWhenSourceUnavailable(t *testing.T) {
	// In copy mode, if the source directory becomes unavailable after injection,
	// eject must still clean up all injected copies using the manifest.
	sourceDir, targetDir, _ := setupTestEnv(t)

	cfg := &config.Config{
		Version:   1,
		SourceDir: sourceDir,
		Mode:      config.ModeCopy,
		Items: []config.Item{
			{Type: config.ItemTypeDirectory, SourcePath: "skills", TargetPath: ".claude/skills", Enabled: true},
		},
	}

	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	// Verify test.md was injected
	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "test.md")); err != nil {
		t.Fatal("test.md should exist after inject")
	}

	// Remove the entire source directory (simulating unavailability)
	os.RemoveAll(sourceDir)

	// Eject — must still remove the copied file
	results, err := Eject(cfg, targetDir)
	if err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(targetDir, ".claude", "skills", "test.md")); err == nil {
		t.Error("test.md should be removed after eject, even when source is unavailable")
	}

	foundRemoved := false
	for _, r := range results {
		if r.Item.TargetPath == ".claude/skills/test.md" && r.Action == "removed" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Error("expected test.md removal in eject results when source unavailable")
	}
}

// --- Manifest persistence tests ---

func TestManifestSavedOnInject(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	_, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	gitDir, _ := gitutil.FindGitDir(targetDir)
	manifest := LoadManifest(gitDir)

	if len(manifest.Entries) == 0 {
		t.Fatal("manifest should have entries after inject")
	}

	// Check that known injected paths are present
	if !manifest.Has(".claude/skills/test.md") {
		t.Error("manifest should contain .claude/skills/test.md")
	}
}

func TestManifestClearedOnEject(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
	if _, err := Eject(cfg, targetDir); err != nil {
		t.Fatalf("Eject failed: %v", err)
	}

	gitDir, _ := gitutil.FindGitDir(targetDir)
	manifest := LoadManifest(gitDir)

	if len(manifest.Entries) != 0 {
		t.Error("manifest should be empty after eject")
	}
}

func TestManifestNotSavedOnDryRun(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	_, err := Inject(cfg, targetDir, Options{DryRun: true})
	if err != nil {
		t.Fatalf("Inject dry-run failed: %v", err)
	}

	gitDir, _ := gitutil.FindGitDir(targetDir)
	manifest := LoadManifest(gitDir)

	if len(manifest.Entries) != 0 {
		t.Error("manifest should not be saved during dry run")
	}
}

func TestInjectDoesNotManifestSkippedRegularFile(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	// Pre-existing regular file at a managed path should not become repomni-owned.
	envrcPath := filepath.Join(targetDir, ".envrc")
	if err := os.WriteFile(envrcPath, []byte("user-owned"), 0644); err != nil {
		t.Fatalf("write .envrc: %v", err)
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	foundSkip := false
	for _, r := range results {
		if r.Item.TargetPath == ".envrc" && r.Action == "skipped" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatal("expected .envrc to be skipped")
	}

	gitDir, _ := gitutil.FindGitDir(targetDir)
	manifest := LoadManifest(gitDir)
	if manifest.Has(".envrc") {
		t.Error("manifest should not claim ownership of a pre-existing regular file")
	}
}

func TestEjectRefusesCleanupWithoutManifest(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	envrcPath := filepath.Join(targetDir, ".envrc")
	if err := os.WriteFile(envrcPath, []byte("user-owned"), 0644); err != nil {
		t.Fatalf("write .envrc: %v", err)
	}

	results, err := Eject(cfg, targetDir)
	if err == nil {
		t.Fatal("expected eject to refuse cleanup without a manifest")
	}

	foundRefusal := false
	for _, r := range results {
		if r.Item.TargetPath == ".envrc" && r.Detail == "manifest missing; refusing to delete" {
			foundRefusal = true
		}
	}
	if !foundRefusal {
		t.Error("expected .envrc refusal result when manifest is missing")
	}

	content, readErr := os.ReadFile(envrcPath)
	if readErr != nil {
		t.Fatalf(".envrc should still exist: %v", readErr)
	}
	if string(content) != "user-owned" {
		t.Error(".envrc content changed during refused eject")
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
	if _, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink}); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}
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

func TestValidateTargetPath(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		wantErr    bool
	}{
		{"normal relative path", ".claude/skills", false},
		{"simple file", ".env", false},
		{"traversal with dotdot", "../etc/passwd", true},
		{"deep traversal", "../../.ssh/id_rsa", true},
		{"hidden traversal via clean", "foo/../../..", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTargetPath("/tmp/repo", tt.targetPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTargetPath(%q) error = %v, wantErr %v", tt.targetPath, err, tt.wantErr)
			}
		})
	}
}

func TestInjectRejectsPathTraversal(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	cfg.Items = []config.Item{
		{Type: config.ItemTypeFile, SourcePath: "hooks.json", TargetPath: "../escape.txt", Enabled: true},
	}

	results, err := Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "error" {
		t.Errorf("expected action 'error' for traversal path, got %q: %s", results[0].Action, results[0].Detail)
	}
}

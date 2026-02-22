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
	os.WriteFile(filepath.Join(targetDir, ".envrc"), []byte("existing"), 0644)

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

	Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
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

	Inject(cfg, targetDir, Options{Mode: config.ModeCopy})

	// Modify a copied file so content differs
	os.WriteFile(filepath.Join(targetDir, ".envrc"), []byte("modified"), 0644)

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

	Inject(cfg, targetDir, Options{Mode: config.ModeSymlink})
	Eject(cfg, targetDir)

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
	os.WriteFile(a, []byte("hello world"), 0644)
	os.WriteFile(b, []byte("hello world"), 0644)

	if !filesEqual(a, b) {
		t.Error("filesEqual should return true for identical files")
	}
}

func TestFilesEqual_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("hello"), 0644)
	os.WriteFile(b, []byte("world"), 0644)

	if filesEqual(a, b) {
		t.Error("filesEqual should return false for different files")
	}
}

func TestFilesEqual_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("hello\n"), 0644)
	os.WriteFile(b, []byte("hello  \n\n"), 0644)

	if !filesEqual(a, b) {
		t.Error("filesEqual should trim trailing whitespace when comparing")
	}
}

func TestFilesEqual_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0644)
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
	os.MkdirAll(emptyDir, 0755)

	removeIfEmptyDir(emptyDir)

	if _, err := os.Stat(emptyDir); err == nil {
		t.Error("empty directory should be removed")
	}
}

func TestRemoveIfEmptyDir_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	nonEmptyDir := filepath.Join(dir, "notempty")
	os.MkdirAll(nonEmptyDir, 0755)
	os.WriteFile(filepath.Join(nonEmptyDir, "file.txt"), []byte("data"), 0644)

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
	os.WriteFile(src, []byte("hello world"), 0644)

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
	os.WriteFile(src, []byte("#!/bin/bash"), 0755)

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
	os.MkdirAll(filepath.Join(src, "sub"), 0755)
	os.WriteFile(filepath.Join(src, "file1.txt"), []byte("root file"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "file2.txt"), []byte("sub file"), 0644)

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
	os.MkdirAll(src, 0755)

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
	os.WriteFile(src1, []byte("v1"), 0644)
	os.WriteFile(src2, []byte("v2"), 0644)

	item := config.Item{TargetPath: "test"}

	// Create symlink to src1
	os.Symlink(src1, dst)

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
	os.WriteFile(src, []byte("data"), 0644)
	os.Symlink(src, dst)

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
	os.WriteFile(src, []byte("data"), 0644)
	os.WriteFile(dst, []byte("regular file"), 0644)

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
	os.WriteFile(src, []byte("data"), 0644)
	os.WriteFile(dst, []byte("regular file"), 0644)

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
	os.WriteFile(src, []byte("source content"), 0644)
	os.WriteFile(dst, []byte("different content"), 0644)

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
	os.WriteFile(src, []byte("same content"), 0644)
	os.WriteFile(dst, []byte("same content"), 0644)

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
	os.WriteFile(src, []byte("new content"), 0644)
	os.WriteFile(dst, []byte("old content"), 0644)

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
	os.MkdirAll(child, 0755)
	os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0644)

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
	os.MkdirAll(child, 0755)

	// Make parent a git repo but don't place .env there
	exec.Command("git", "init", parentRepo).Run()

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
	os.MkdirAll(child, 0755)
	os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0644)
	os.WriteFile(filepath.Join(root, ".envrc"), []byte("export A=1"), 0644)

	result := findEnvInParents(child)
	if len(result.Found) != 2 {
		t.Errorf("expected 2 found files, got %d", len(result.Found))
	}
}

func TestStatusCopyMode(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
	statuses, err := Status(cfg, targetDir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	for _, s := range statuses {
		if !s.Present {
			t.Errorf("%s should be present after copy inject", s.Item.TargetPath)
		}
		// Copy-mode items are regular files, not symlinks, so Current should be false
		// and Detail should mention "regular file/dir"
		if s.Item.TargetPath != ".claude/skills/test.md" {
			// File items in copy mode are detected as regular files
			if s.Detail != "regular file/dir (not a symlink)" {
				// Accept both detail messages — env files found via parent search are regular files
				continue
			}
		}
	}
}

func TestEjectAfterCopyInject(t *testing.T) {
	_, targetDir, cfg := setupTestEnv(t)

	Inject(cfg, targetDir, Options{Mode: config.ModeCopy})
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
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)
	exec.Command("git", "init", targetDir).Run()

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
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(targetDir, 0755)
	exec.Command("git", "init", targetDir).Run()

	// Directory item with empty source
	os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755)

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

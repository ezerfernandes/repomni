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

	sourceDir = t.TempDir()
	targetDir = t.TempDir()

	// Init git repo in target
	cmd := exec.Command("git", "init", targetDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Create source files (defaults: skills/, hooks.json at source root)
	os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "test.md"), []byte("skill"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "hooks.json"), []byte(`{"hooks":[]}`), 0644)
	os.WriteFile(filepath.Join(sourceDir, ".envrc"), []byte("export FOO=bar"), 0644)
	os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SECRET=123"), 0644)

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

	// Verify file symlinks
	link, err := os.Readlink(filepath.Join(targetDir, ".envrc"))
	if err != nil {
		t.Fatalf("cannot read symlink: %v", err)
	}
	expected := filepath.Join(sourceDir, ".envrc")
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

	// Verify content matches
	src, _ := os.ReadFile(filepath.Join(sourceDir, ".envrc"))
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

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

	// Verify symlinks
	link, err := os.Readlink(filepath.Join(targetDir, ".envrc"))
	if err != nil {
		t.Fatalf("cannot read symlink: %v", err)
	}
	expected := filepath.Join(sourceDir, ".envrc")
	if link != expected {
		t.Errorf("expected symlink to %q, got %q", expected, link)
	}

	// Verify directory symlink
	link, err = os.Readlink(filepath.Join(targetDir, ".claude", "skills"))
	if err != nil {
		t.Fatalf("cannot read skills symlink: %v", err)
	}
	expected = filepath.Join(sourceDir, "skills")
	if link != expected {
		t.Errorf("expected symlink to %q, got %q", expected, link)
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

	// Verify it's a real file, not a symlink
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

	// Verify .claude dir was cleaned up
	if _, err := os.Stat(filepath.Join(targetDir, ".claude")); err == nil {
		t.Error(".claude should be cleaned up after eject")
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

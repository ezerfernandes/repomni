package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
)

func TestBuildDirSelectionPlans_PreselectedEntriesStillShowNewEntries(t *testing.T) {
	sourceDir := t.TempDir()
	skillsDir := filepath.Join(sourceDir, "skills")
	for _, name := range []string{"hello-world", "new-skill", "web-browser"} {
		if err := os.MkdirAll(filepath.Join(skillsDir, name), 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		Items: []config.Item{{
			Type:       config.ItemTypeDirectory,
			SourcePath: "skills",
			TargetPath: ".claude/skills",
			Enabled:    true,
		}},
	}

	plans, err := buildDirSelectionPlans(cfg, map[string]map[string]bool{
		".claude/skills": {
			"hello-world": true,
			"web-browser": true,
		},
	})
	if err != nil {
		t.Fatalf("buildDirSelectionPlans failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	plan := plans[0]
	if plan.targetPath != ".claude/skills" {
		t.Fatalf("unexpected target path: %q", plan.targetPath)
	}

	gotOptions := make(map[string]bool)
	for _, option := range plan.options {
		gotOptions[option.name] = true
	}
	for _, want := range []string{"hello-world", "new-skill", "web-browser"} {
		if !gotOptions[want] {
			t.Errorf("expected option %q to be shown", want)
		}
	}

	gotSelected := make(map[string]bool)
	for _, name := range plan.selected {
		gotSelected[name] = true
	}
	if !gotSelected["hello-world"] {
		t.Error("hello-world should be preselected")
	}
	if !gotSelected["web-browser"] {
		t.Error("web-browser should be preselected")
	}
	if gotSelected["new-skill"] {
		t.Error("new-skill should be shown but not preselected")
	}
}

func TestBuildDirSelectionPlans_NoPreselectedEntriesSelectsAll(t *testing.T) {
	sourceDir := t.TempDir()
	skillsDir := filepath.Join(sourceDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	for _, name := range []string{"hello-world", "web-browser"} {
		if err := os.WriteFile(filepath.Join(skillsDir, name+".md"), []byte(name), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		Items: []config.Item{{
			Type:       config.ItemTypeDirectory,
			SourcePath: "skills",
			TargetPath: ".claude/skills",
			Enabled:    true,
		}},
	}

	plans, err := buildDirSelectionPlans(cfg, nil)
	if err != nil {
		t.Fatalf("buildDirSelectionPlans failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	gotSelected := make(map[string]bool)
	for _, name := range plans[0].selected {
		gotSelected[name] = true
	}
	if !gotSelected["hello-world.md"] {
		t.Error("hello-world.md should be selected by default")
	}
	if !gotSelected["web-browser.md"] {
		t.Error("web-browser.md should be selected by default")
	}
}

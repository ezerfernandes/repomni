package ui

import "testing"

func TestGetSourcePath_Existing(t *testing.T) {
	existing := map[string]string{
		".claude/skills": "my-skills",
	}

	got := getSourcePath(existing, ".claude/skills", "skills")
	if got != "my-skills" {
		t.Errorf("expected 'my-skills', got %q", got)
	}
}

func TestGetSourcePath_Missing(t *testing.T) {
	existing := map[string]string{}

	got := getSourcePath(existing, ".claude/skills", "skills")
	if got != "skills" {
		t.Errorf("expected default 'skills', got %q", got)
	}
}

func TestGetSourcePath_EmptyValue(t *testing.T) {
	existing := map[string]string{
		".claude/skills": "",
	}

	got := getSourcePath(existing, ".claude/skills", "skills")
	if got != "skills" {
		t.Errorf("expected default 'skills' for empty value, got %q", got)
	}
}

func TestGetSourcePath_NilMap(t *testing.T) {
	got := getSourcePath(nil, ".claude/skills", "skills")
	if got != "skills" {
		t.Errorf("expected default 'skills' for nil map, got %q", got)
	}
}

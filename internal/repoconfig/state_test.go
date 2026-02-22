package repoconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateState_KnownStates(t *testing.T) {
	for _, s := range KnownStates() {
		if err := ValidateState(string(s)); err != nil {
			t.Errorf("ValidateState(%q) should pass: %v", s, err)
		}
	}
}

func TestValidateState_CustomState(t *testing.T) {
	valid := []string{"my-state", "wip", "ready-for-qa", "v2"}
	for _, s := range valid {
		if err := ValidateState(s); err != nil {
			t.Errorf("ValidateState(%q) should pass: %v", s, err)
		}
	}
}

func TestValidateState_Empty(t *testing.T) {
	if err := ValidateState(""); err == nil {
		t.Error("ValidateState(\"\") should fail")
	}
}

func TestValidateState_InvalidChars(t *testing.T) {
	invalid := []string{"Active", "DONE", "in progress", "state!", "review/done"}
	for _, s := range invalid {
		if err := ValidateState(s); err == nil {
			t.Errorf("ValidateState(%q) should fail", s)
		}
	}
}

func TestKnownStates_Count(t *testing.T) {
	states := KnownStates()
	if len(states) != 8 {
		t.Errorf("expected 8 known states, got %d", len(states))
	}
}

func TestIsKnownState_AllKnown(t *testing.T) {
	for _, s := range KnownStates() {
		if !IsKnownState(string(s)) {
			t.Errorf("IsKnownState(%q) should return true", s)
		}
	}
}

func TestIsKnownState(t *testing.T) {
	if !IsKnownState("active") {
		t.Error("active should be a known state")
	}
	if !IsKnownState("review") {
		t.Error("review should be a known state")
	}
	if !IsKnownState("done") {
		t.Error("done should be a known state")
	}
	if !IsKnownState("paused") {
		t.Error("paused should be a known state")
	}
	if IsKnownState("custom") {
		t.Error("custom should NOT be a known state")
	}
	if IsKnownState("") {
		t.Error("empty should NOT be a known state")
	}
}

func TestValidateState_TooLong(t *testing.T) {
	long := "a-very-long-state-name-that-exceeds-reasonable-limits-aaaaaaaaaa"
	// Should still pass if it matches the regex pattern
	err := ValidateState(long)
	// The function only checks for lowercase alphanumeric and hyphens
	if err != nil {
		t.Errorf("ValidateState(%q) should pass (matches pattern): %v", long, err)
	}
}

func TestValidateState_SingleChar(t *testing.T) {
	if err := ValidateState("a"); err != nil {
		t.Errorf("ValidateState(\"a\") should pass: %v", err)
	}
}

func TestValidateState_WithNumbers(t *testing.T) {
	valid := []string{"v1", "phase-2", "123", "a1b2c3"}
	for _, s := range valid {
		if err := ValidateState(s); err != nil {
			t.Errorf("ValidateState(%q) should pass: %v", s, err)
		}
	}
}

func TestValidateState_SpecialChars(t *testing.T) {
	invalid := []string{"hello_world", "foo.bar", "a@b", "tab\there", "new\nline"}
	for _, s := range invalid {
		if err := ValidateState(s); err == nil {
			t.Errorf("ValidateState(%q) should fail", s)
		}
	}
}

func TestIsKnownState_NotCaseSensitive(t *testing.T) {
	// IsKnownState should be case-sensitive (only lowercase known states)
	if IsKnownState("Active") {
		t.Error("IsKnownState should be case-sensitive")
	}
	if IsKnownState("REVIEW") {
		t.Error("IsKnownState should be case-sensitive")
	}
}

func TestSaveAndLoad_WithState(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &RepoConfig{
		Version: 1,
		State:   "review",
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
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.State != "review" {
		t.Errorf("State mismatch: got %q, want %q", loaded.State, "review")
	}
}

func TestSaveAndLoad_NoState(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	original := &RepoConfig{
		Version: 1,
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
	if loaded.State != "" {
		t.Errorf("State should be empty, got %q", loaded.State)
	}
}

func TestLoad_LegacyConfig(t *testing.T) {
	// Simulate a config file written before the state field existed.
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	configDir := filepath.Join(gitDir, "repoinjector")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	legacy := []byte("version: 1\nitems:\n  - target_path: .envrc\n    enabled: true\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), legacy, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.State != "" {
		t.Errorf("State should be empty for legacy config, got %q", loaded.State)
	}
	if len(loaded.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(loaded.Items))
	}
}

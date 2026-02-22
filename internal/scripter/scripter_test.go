package scripter

import (
	"os"
	"path/filepath"
	"testing"
)

func setupGitDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestScriptPath(t *testing.T) {
	gitDir := "/fake/.git"
	got := ScriptPath(gitDir, ScriptSetup)
	expected := filepath.Join("/fake/.git", "repoinjector", "scripts", "setup.sh")
	if got != expected {
		t.Errorf("ScriptPath() = %q, want %q", got, expected)
	}
}

func TestScriptPath_CustomType(t *testing.T) {
	gitDir := "/fake/.git"
	got := ScriptPath(gitDir, "teardown")
	expected := filepath.Join("/fake/.git", "repoinjector", "scripts", "teardown.sh")
	if got != expected {
		t.Errorf("ScriptPath() = %q, want %q", got, expected)
	}
}

func TestGetScript_NotExists(t *testing.T) {
	gitDir := setupGitDir(t)

	content, exists := GetScript(gitDir, ScriptSetup)
	if exists {
		t.Error("expected exists=false for missing script")
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestSaveAndGetScript(t *testing.T) {
	gitDir := setupGitDir(t)
	scriptContent := "#!/bin/bash\necho hello"

	if err := SaveScript(gitDir, ScriptSetup, scriptContent); err != nil {
		t.Fatalf("SaveScript failed: %v", err)
	}

	// Verify file is executable
	info, err := os.Stat(ScriptPath(gitDir, ScriptSetup))
	if err != nil {
		t.Fatalf("script file not created: %v", err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Error("script file should be executable")
	}

	// Verify content via GetScript
	got, exists := GetScript(gitDir, ScriptSetup)
	if !exists {
		t.Error("expected exists=true after save")
	}
	if got != scriptContent {
		t.Errorf("GetScript() = %q, want %q", got, scriptContent)
	}
}

func TestSaveScript_CreatesDirectories(t *testing.T) {
	gitDir := setupGitDir(t)

	if err := SaveScript(gitDir, ScriptSetup, "#!/bin/bash"); err != nil {
		t.Fatalf("SaveScript failed: %v", err)
	}

	dir := filepath.Dir(ScriptPath(gitDir, ScriptSetup))
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("script directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, not file")
	}
}

func TestDeleteScript(t *testing.T) {
	gitDir := setupGitDir(t)

	// Save then delete
	SaveScript(gitDir, ScriptSetup, "#!/bin/bash")

	if err := DeleteScript(gitDir, ScriptSetup); err != nil {
		t.Fatalf("DeleteScript failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(ScriptPath(gitDir, ScriptSetup)); !os.IsNotExist(err) {
		t.Error("script file should be deleted")
	}
}

func TestDeleteScript_NotExists(t *testing.T) {
	gitDir := setupGitDir(t)

	// Deleting a nonexistent script should not error
	if err := DeleteScript(gitDir, ScriptSetup); err != nil {
		t.Fatalf("DeleteScript on nonexistent file should not error: %v", err)
	}
}

func TestRunScript_NotExists(t *testing.T) {
	gitDir := setupGitDir(t)
	workDir := t.TempDir()

	// RunScript should return nil when no script exists
	if err := RunScript(gitDir, ScriptSetup, workDir); err != nil {
		t.Fatalf("RunScript should return nil for nonexistent script: %v", err)
	}
}

func TestRunScript_Executes(t *testing.T) {
	gitDir := setupGitDir(t)
	workDir := t.TempDir()
	markerFile := filepath.Join(workDir, "marker.txt")

	// Create a script that writes a marker file
	script := "#!/bin/bash\necho done > " + markerFile
	if err := SaveScript(gitDir, ScriptSetup, script); err != nil {
		t.Fatalf("SaveScript failed: %v", err)
	}

	if err := RunScript(gitDir, ScriptSetup, workDir); err != nil {
		t.Fatalf("RunScript failed: %v", err)
	}

	// Verify the marker file was created
	if _, err := os.Stat(markerFile); err != nil {
		t.Error("script did not execute: marker file not found")
	}
}

func TestRunScript_UsesWorkDir(t *testing.T) {
	gitDir := setupGitDir(t)
	workDir := t.TempDir()

	// Create a script that writes pwd to a file
	outFile := filepath.Join(t.TempDir(), "pwd.txt")
	script := "#!/bin/bash\npwd > " + outFile
	if err := SaveScript(gitDir, ScriptSetup, script); err != nil {
		t.Fatalf("SaveScript failed: %v", err)
	}

	if err := RunScript(gitDir, ScriptSetup, workDir); err != nil {
		t.Fatalf("RunScript failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("cannot read pwd output: %v", err)
	}

	// The working directory should match workDir
	got := filepath.Clean(string(data[:len(data)-1])) // strip newline
	want := filepath.Clean(workDir)
	if got != want {
		t.Errorf("script ran in %q, want %q", got, want)
	}
}

func TestSaveScript_OverwritesExisting(t *testing.T) {
	gitDir := setupGitDir(t)

	SaveScript(gitDir, ScriptSetup, "#!/bin/bash\necho v1")
	SaveScript(gitDir, ScriptSetup, "#!/bin/bash\necho v2")

	got, exists := GetScript(gitDir, ScriptSetup)
	if !exists {
		t.Fatal("script should exist")
	}
	if got != "#!/bin/bash\necho v2" {
		t.Errorf("expected overwritten content, got %q", got)
	}
}

func TestRunScript_FailingScript(t *testing.T) {
	gitDir := setupGitDir(t)
	workDir := t.TempDir()

	// Create a script that exits with non-zero code
	script := "#!/bin/bash\nexit 1"
	if err := SaveScript(gitDir, ScriptSetup, script); err != nil {
		t.Fatalf("SaveScript failed: %v", err)
	}

	err := RunScript(gitDir, ScriptSetup, workDir)
	if err == nil {
		t.Error("RunScript should return error for failing script")
	}
}

func TestRunScript_ScriptWithArgs(t *testing.T) {
	gitDir := setupGitDir(t)
	workDir := t.TempDir()
	outFile := filepath.Join(workDir, "output.txt")

	// Create a script that uses environment from the workdir
	script := "#!/bin/bash\necho success > " + outFile + " && exit 0"
	if err := SaveScript(gitDir, ScriptSetup, script); err != nil {
		t.Fatalf("SaveScript failed: %v", err)
	}

	if err := RunScript(gitDir, ScriptSetup, workDir); err != nil {
		t.Fatalf("RunScript failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal("output file should exist")
	}
	if string(data) != "success\n" {
		t.Errorf("unexpected output: %q", data)
	}
}

func TestScriptPath_DifferentTypes(t *testing.T) {
	tests := []struct {
		name       string
		scriptType string
		suffix     string
	}{
		{"setup", ScriptSetup, "setup.sh"},
		{"teardown", "teardown", "teardown.sh"},
		{"deploy", "deploy", "deploy.sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScriptPath("/fake/.git", tt.scriptType)
			if !filepath.IsAbs(got) {
				t.Error("ScriptPath should return absolute path")
			}
			if filepath.Base(got) != tt.suffix {
				t.Errorf("expected suffix %q, got %q", tt.suffix, filepath.Base(got))
			}
		})
	}
}

func TestGetScript_AfterDelete(t *testing.T) {
	gitDir := setupGitDir(t)

	SaveScript(gitDir, ScriptSetup, "#!/bin/bash\necho hello")
	DeleteScript(gitDir, ScriptSetup)

	content, exists := GetScript(gitDir, ScriptSetup)
	if exists {
		t.Error("script should not exist after delete")
	}
	if content != "" {
		t.Errorf("content should be empty after delete, got %q", content)
	}
}

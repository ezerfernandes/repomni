package scripter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// ScriptSetup is the script type for post-branch-creation setup scripts.
	ScriptSetup = "setup"
)

// ScriptPath returns the full path to a script file within the git directory.
func ScriptPath(gitDir string, scriptType string) string {
	return filepath.Join(gitDir, "repomni", "scripts", scriptType+".sh")
}

// GetScript reads the script content for the given type.
// Returns the content and true if the script exists, or empty string and false otherwise.
func GetScript(gitDir string, scriptType string) (string, bool) {
	data, err := os.ReadFile(ScriptPath(gitDir, scriptType))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// SaveScript writes the script content and makes it executable.
func SaveScript(gitDir string, scriptType string, content string) error {
	dir := filepath.Dir(ScriptPath(gitDir, scriptType))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create script directory: %w", err)
	}
	return os.WriteFile(ScriptPath(gitDir, scriptType), []byte(content), 0700)
}

// DeleteScript removes a script file.
func DeleteScript(gitDir string, scriptType string) error {
	path := ScriptPath(gitDir, scriptType)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete script: %w", err)
	}
	return nil
}

// RunScript executes a script in the given working directory.
// Returns nil without doing anything if no script exists for the given type.
// Stdout and stderr are connected to the terminal.
func RunScript(gitDir string, scriptType string, workDir string) error {
	path := ScriptPath(gitDir, scriptType)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	cmd := exec.Command("bash", path)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

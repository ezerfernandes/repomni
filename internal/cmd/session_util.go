package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
)

// resolveProjectPath determines the Claude Code project path from the
// current working directory. If the CWD is inside a git repo, the
// repository root is used; otherwise the CWD itself is used.
func resolveProjectPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine current directory: %w", err)
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Walk up to find git root.
	dir := cwd
	for {
		if gitutil.IsGitRepo(dir) {
			root, err := gitutil.RunGit(dir, "rev-parse", "--show-toplevel")
			if err == nil && root != "" {
				return root, nil
			}
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Not in a git repo, use CWD as-is.
	return cwd, nil
}

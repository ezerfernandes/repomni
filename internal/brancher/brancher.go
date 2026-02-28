package brancher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
)

// Result holds information about a successful branch operation.
type Result struct {
	ParentRepo string
	RemoteURL  string
	TargetDir  string
	Branch     string
}

// ValidateBranchName checks that name is both a valid directory name and a valid git branch name.
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("branch name cannot be %q", name)
	}
	if name == "@" {
		return fmt.Errorf("branch name cannot be %q", name)
	}

	// Must be a simple name (no path separators) so it works as a directory name.
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("branch name cannot contain path separators")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("branch name cannot contain null bytes")
	}

	// Git ref-format rules (for refs/heads/<name>).
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name cannot contain '..'")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("branch name cannot start with '.'")
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("branch name cannot end with '.'")
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("branch name cannot end with '.lock'")
	}
	if strings.Contains(name, "@{") {
		return fmt.Errorf("branch name cannot contain '@{'")
	}

	for _, r := range name {
		if r < 32 || r == 127 {
			return fmt.Errorf("branch name cannot contain control characters")
		}
		switch r {
		case ' ', '~', '^', ':', '?', '*', '[':
			return fmt.Errorf("branch name cannot contain %q", string(r))
		}
	}

	return nil
}

// FindParentGitRepo walks up from startDir to find the first directory that is a git repository.
func FindParentGitRepo(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if gitutil.IsGitRepo(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository found in any parent of %s", startDir)
		}
		dir = parent
	}
}

// SanitizeBranchName replaces characters that are forbidden in directory names with '-'.
func SanitizeBranchName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '?', '*', '[', '~', '^', ' ':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Branch clones the repository found in a parent directory and checks out a new branch.
// workDir is the directory where the clone will be created.
func Branch(workDir, branchName string) (*Result, error) {
	if err := ValidateBranchName(branchName); err != nil {
		return nil, err
	}

	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}

	parentRepo, err := FindParentGitRepo(workDir)
	if err != nil {
		return nil, err
	}

	url, err := gitutil.RunGit(parentRepo, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("cannot get remote URL from %s: %w", parentRepo, err)
	}

	targetDir := filepath.Join(workDir, branchName)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", targetDir)
	}

	if _, err := gitutil.RunGit(workDir, "clone", url, branchName); err != nil {
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	if _, err := gitutil.RunGit(targetDir, "checkout", "-b", branchName); err != nil {
		os.RemoveAll(targetDir)
		return nil, fmt.Errorf("git checkout failed: %w", err)
	}

	return &Result{
		ParentRepo: parentRepo,
		RemoteURL:  url,
		TargetDir:  targetDir,
		Branch:     branchName,
	}, nil
}

// Clone clones the repository found in a parent directory and checks out an existing remote branch.
// The local directory name is derived by sanitizing the branch name (replacing forbidden chars with '-').
func Clone(workDir, branchName string) (*Result, error) {
	if branchName == "" {
		return nil, fmt.Errorf("branch name cannot be empty")
	}

	dirName := SanitizeBranchName(branchName)

	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}

	parentRepo, err := FindParentGitRepo(workDir)
	if err != nil {
		return nil, err
	}

	url, err := gitutil.RunGit(parentRepo, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("cannot get remote URL from %s: %w", parentRepo, err)
	}

	targetDir := filepath.Join(workDir, dirName)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", targetDir)
	}

	if _, err := gitutil.RunGit(workDir, "clone", url, dirName); err != nil {
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	if _, err := gitutil.RunGit(targetDir, "checkout", branchName); err != nil {
		os.RemoveAll(targetDir)
		return nil, fmt.Errorf("git checkout failed: %w", err)
	}

	return &Result{
		ParentRepo: parentRepo,
		RemoteURL:  url,
		TargetDir:  targetDir,
		Branch:     branchName,
	}, nil
}

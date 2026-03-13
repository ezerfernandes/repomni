package gitutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// FindGitDir returns the path to the .git directory for a repository.
// Handles both regular repos (.git is a directory) and worktrees (.git is a file).
func FindGitDir(repoRoot string) (string, error) {
	gitPath := filepath.Join(repoRoot, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return "", fmt.Errorf("not a git repository (no .git found): %w", err)
	}

	if info.IsDir() {
		return gitPath, nil
	}

	// .git is a file — worktree pointer
	content, err := os.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("cannot read .git file: %w", err)
	}

	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("unexpected .git file format: %s", line)
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(repoRoot, gitdir)
	}

	return filepath.Clean(gitdir), nil
}

// IsGitRepo checks if the given directory is a git repository.
func IsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// FindGitRepos returns all immediate subdirectories of parentDir that are git repos.
func FindGitRepos(parentDir string) ([]string, error) {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %s: %w", parentDir, err)
	}

	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(parentDir, e.Name())
		if IsGitRepo(candidate) {
			repos = append(repos, candidate)
		}
	}
	return repos, nil
}

// sanitizeStderr strips potential credentials from stderr output before
// including it in error messages. Git URLs may contain embedded passwords
// (e.g. https://user:token@host/repo) that should not leak.
func sanitizeStderr(s string) string {
	// Match URLs with embedded credentials: scheme://user:pass@host
	re := regexp.MustCompile(`(https?://)([^:@]+):([^@]+)@`)
	return re.ReplaceAllString(s, "${1}${2}:***@")
}

// RunGit executes a git command in the given directory and returns trimmed stdout.
func RunGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), sanitizeStderr(strings.TrimSpace(stderr.String())), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CurrentBranch returns the current branch name, or "" if HEAD is detached.
func CurrentBranch(dir string) (string, error) {
	out, err := RunGit(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", nil // detached HEAD
	}
	return out, nil
}

// UpstreamRef returns the upstream tracking ref (e.g. "origin/main") for the current branch.
// Returns ("", nil) if no upstream is configured.
func UpstreamRef(dir string) (string, error) {
	out, err := RunGit(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", nil
	}
	return out, nil
}

// AheadBehind returns how many commits the current branch is ahead/behind its upstream.
func AheadBehind(dir string) (ahead, behind int, err error) {
	out, err := RunGit(dir, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0, err
	}
	_, err = fmt.Sscanf(out, "%d\t%d", &ahead, &behind)
	return
}

// IsDirty returns true if the working tree has uncommitted changes
// (staged, unstaged, or untracked files).
func IsDirty(dir string) (bool, error) {
	out, err := RunGit(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// Fetch runs git fetch --quiet on the repo.
func Fetch(dir string) error {
	_, err := RunGit(dir, "fetch", "--quiet")
	return err
}

// Pull runs git pull with the given strategy.
// strategy can be "ff-only" (default), "rebase", or "merge".
// When autoStash is true and strategy is "rebase", git's --autostash is used.
// For other strategies, stash/unstash is performed manually.
func Pull(dir string, strategy string, autoStash bool) (string, error) {
	if autoStash && strategy != "rebase" {
		// Manual stash for non-rebase strategies (git --autostash only works with --rebase)
		stashed, err := stash(dir)
		if err != nil {
			return "", fmt.Errorf("stash before pull: %w", err)
		}
		out, pullErr := pullInner(dir, strategy, false)
		if stashed {
			if err := stashPop(dir); err != nil {
				if pullErr != nil {
					return "", fmt.Errorf("pull failed: %w; also failed to pop stash: %v", pullErr, err)
				}
				return "", fmt.Errorf("stash pop after pull: %w", err)
			}
		}
		return out, pullErr
	}
	return pullInner(dir, strategy, autoStash)
}

func pullInner(dir string, strategy string, autoStash bool) (string, error) {
	args := []string{"pull", "--quiet"}
	switch strategy {
	case "rebase":
		args = append(args, "--rebase")
	case "merge":
		// default merge behavior
	default:
		args = append(args, "--ff-only")
	}
	if autoStash {
		args = append(args, "--autostash")
	}
	return RunGit(dir, args...)
}

func stash(dir string) (bool, error) {
	out, err := RunGit(dir, "stash")
	if err != nil {
		return false, err
	}
	return !strings.Contains(out, "No local changes"), nil
}

func stashPop(dir string) error {
	_, err := RunGit(dir, "stash", "pop")
	return err
}

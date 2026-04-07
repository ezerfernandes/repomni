package gitutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

// credentialRe matches URLs with embedded credentials: scheme://user:pass@host
var credentialRe = regexp.MustCompile(`(https?://)([^:@]+):([^@]+)@`)

// sanitizeStderr strips potential credentials from stderr output before
// including it in error messages. Git URLs may contain embedded passwords
// (e.g. https://user:token@host/repo) that should not leak.
func sanitizeStderr(s string) string {
	return credentialRe.ReplaceAllString(s, "${1}${2}:***@")
}

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// gitEnv is the cached environment for git commands, set once on init.
var gitEnv = func() []string {
	env := os.Environ()
	return append(env,
		"GIT_TERMINAL_PROMPT=0",
	)
}()

// gitPath caches the resolved git binary path to avoid repeated PATH lookups.
var gitPath = func() string {
	p, err := exec.LookPath("git")
	if err != nil {
		return "git" // fallback to default
	}
	return p
}()

// RunGit executes a git command in the given directory and returns trimmed stdout.
func RunGit(dir string, args ...string) (string, error) {
	cmd := exec.Command(gitPath, args...)
	cmd.Dir = dir
	cmd.Env = gitEnv

	stdout := bufPool.Get().(*bytes.Buffer)
	stderr := bufPool.Get().(*bytes.Buffer)
	stdout.Reset()
	stderr.Reset()

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	result := strings.TrimSpace(stdout.String())
	stderrStr := stderr.String()

	bufPool.Put(stdout)
	bufPool.Put(stderr)

	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), sanitizeStderr(strings.TrimSpace(stderrStr)), err)
	}
	return result, nil
}

// RepoInfo holds combined status information obtained from a single git call.
type RepoInfo struct {
	Branch   string // current branch name, or "" if detached
	Upstream string // upstream tracking ref (e.g. "origin/main"), or ""
	Ahead    int
	Behind   int
	Dirty    bool
}

// GetRepoInfo retrieves branch, upstream, ahead/behind, and dirty status
// in a single git process using "git status -b --porcelain=v2".
func GetRepoInfo(dir string) (RepoInfo, error) {
	out, err := RunGit(dir, "--no-optional-locks", "status", "-b", "--porcelain=v2")
	if err != nil {
		return RepoInfo{}, err
	}
	return parseStatusV2(out), nil
}

func parseStatusV2(out string) RepoInfo {
	var info RepoInfo
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			head := strings.TrimPrefix(line, "# branch.head ")
			if head != "(detached)" {
				info.Branch = head
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			info.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &info.Ahead, &info.Behind)
		case line != "" && !strings.HasPrefix(line, "#"):
			// Any non-header, non-empty line means dirty
			info.Dirty = true
		}
	}
	return info
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

// Fetch runs git fetch --quiet on the repo with optimized settings.
// When noTags is true, --no-tags is appended to skip fetching tags.
func Fetch(dir string, noTags bool) error {
	args := []string{
		"-c", "gc.auto=0",
		"-c", "fetch.negotiationAlgorithm=skipping",
		"fetch", "--quiet", "--no-auto-maintenance",
	}
	if noTags {
		args = append(args, "--no-tags")
	}
	_, err := RunGit(dir, args...)
	return err
}

// MergeUpstream integrates upstream changes without fetching (assumes fetch already done).
// strategy can be "ff-only" (default), "rebase", or "merge".
// When autoStash is true and strategy is "rebase", git's --autostash is used.
// For other strategies, stash/unstash is performed manually.
func MergeUpstream(dir string, strategy string, autoStash bool) (string, error) {
	if autoStash && strategy != "rebase" {
		stashed, err := stash(dir)
		if err != nil {
			return "", fmt.Errorf("stash before merge: %w", err)
		}
		out, mergeErr := mergeUpstreamInner(dir, strategy, false)
		if stashed {
			if err := stashPop(dir); err != nil {
				if mergeErr != nil {
					return "", fmt.Errorf("merge failed: %w; also failed to pop stash: %v", mergeErr, err)
				}
				return "", fmt.Errorf("stash pop after merge: %w", err)
			}
		}
		return out, mergeErr
	}
	return mergeUpstreamInner(dir, strategy, autoStash)
}

func mergeUpstreamInner(dir string, strategy string, autoStash bool) (string, error) {
	switch strategy {
	case "rebase":
		args := []string{"-c", "gc.auto=0", "rebase", "--quiet", "@{u}"}
		if autoStash {
			args = append(args, "--autostash")
		}
		return RunGit(dir, args...)
	case "merge":
		return RunGit(dir, "-c", "gc.auto=0", "merge", "--quiet", "@{u}")
	default:
		return RunGit(dir, "-c", "gc.auto=0", "merge", "--quiet", "--ff-only", "@{u}")
	}
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

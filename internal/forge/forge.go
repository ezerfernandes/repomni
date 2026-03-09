// Package forge provides utilities for running GitHub CLI (gh) and GitLab CLI (glab) commands.
package forge

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
)

// Platform re-exports mergestatus.Platform to avoid circular dependencies.
type Platform = mergestatus.Platform

const (
	PlatformGitHub = mergestatus.PlatformGitHub
	PlatformGitLab = mergestatus.PlatformGitLab
)

// cliName returns "gh" or "glab" for the given platform.
func cliName(platform Platform) string {
	if platform == PlatformGitHub {
		return "gh"
	}
	return "glab"
}

// RunForge executes a gh or glab command and returns trimmed stdout.
func RunForge(platform Platform, args ...string) (string, error) {
	name := cliName(platform)
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %s: %w", name, strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunForgeDir executes a gh or glab command in the given directory and returns trimmed stdout.
func RunForgeDir(dir string, platform Platform, args ...string) (string, error) {
	name := cliName(platform)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %s: %w", name, strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunForgePassthrough executes a gh or glab command with stdio attached to the terminal.
// Used for interactive commands like "checks --watch".
func RunForgePassthrough(dir string, platform Platform, args ...string) error {
	name := cliName(platform)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckCLI verifies that the CLI for the given platform is available in PATH.
func CheckCLI(platform Platform) error {
	name := cliName(platform)
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s CLI not found in PATH; install it to use this command with %s", name, platform)
	}
	return nil
}

// DetectPlatformFromRemote determines the hosting platform by inspecting
// the origin remote URL of the git repository at dir.
func DetectPlatformFromRemote(dir string) (Platform, error) {
	remoteURL, err := gitutil.RunGit(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("cannot detect platform: %w", err)
	}
	host := extractHost(remoteURL)
	if host == "" {
		return "", fmt.Errorf("cannot parse host from remote URL: %s", remoteURL)
	}
	host = strings.ToLower(host)
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		return PlatformGitHub, nil
	}
	return PlatformGitLab, nil
}

// extractHost parses the hostname from a git remote URL.
// Handles HTTPS (https://github.com/org/repo.git) and
// SSH (git@github.com:org/repo.git) formats.
func extractHost(remoteURL string) string {
	// Try HTTPS first
	if parsed, err := url.Parse(remoteURL); err == nil && parsed.Host != "" {
		return parsed.Hostname()
	}
	// SSH format: [user@]host:path
	if _, rest, ok := strings.Cut(remoteURL, "@"); ok {
		if host, _, ok := strings.Cut(rest, ":"); ok {
			return host
		}
	}
	return ""
}

// ParseMergeNumber extracts the PR/MR number from a merge URL.
// Works for both GitHub (/pull/42) and GitLab (/merge_requests/42) URLs.
var mergeNumberRe = regexp.MustCompile(`/(?:pull|merge_requests)/(\d+)`)

func ParseMergeNumber(mergeURL string) int {
	m := mergeNumberRe.FindStringSubmatch(mergeURL)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

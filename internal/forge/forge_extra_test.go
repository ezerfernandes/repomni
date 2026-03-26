package forge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- sanitizeStderr ---

func TestSanitizeStderr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no credentials", "error: not found", "error: not found"},
		{"https with credentials", "https://user:secret-token@github.com/org/repo", "https://user:***@github.com/org/repo"},
		{"http with credentials", "http://deploy:abc123@gitlab.com/group/project", "http://deploy:***@gitlab.com/group/project"},
		{"multiple credentials", "https://a:b@x.com and https://c:d@y.com", "https://a:***@x.com and https://c:***@y.com"},
		{"no scheme", "just plain text", "just plain text"},
		{"empty string", "", ""},
		{"url without credentials", "https://github.com/org/repo", "https://github.com/org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeStderr(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeStderr(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- CheckCLI ---

func TestCheckCLI_GitHub(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	err := CheckCLI(PlatformGitHub)
	if err != nil {
		t.Errorf("CheckCLI(GitHub) should succeed when gh is installed: %v", err)
	}
}

func TestCheckCLI_GitLab(t *testing.T) {
	if _, err := exec.LookPath("glab"); err != nil {
		t.Skip("glab not installed")
	}

	err := CheckCLI(PlatformGitLab)
	if err != nil {
		t.Errorf("CheckCLI(GitLab) should succeed when glab is installed: %v", err)
	}
}

func TestCheckCLI_MissingGitHub(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := CheckCLI(PlatformGitHub)
	if err == nil {
		t.Error("expected error when gh is not in PATH")
	}
	if err != nil && !strings.Contains(err.Error(), "gh CLI not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckCLI_MissingGitLab(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := CheckCLI(PlatformGitLab)
	if err == nil {
		t.Error("expected error when glab is not in PATH")
	}
	if err != nil && !strings.Contains(err.Error(), "glab CLI not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- RunForge ---

func TestRunForge_GitHub_Success(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	// gh --version always succeeds.
	out, err := RunForge(PlatformGitHub, "--version")
	if err != nil {
		t.Fatalf("RunForge(gh --version) error: %v", err)
	}
	if !strings.Contains(out, "gh version") {
		t.Errorf("expected version output, got %q", out)
	}
}

func TestRunForge_GitLab_Success(t *testing.T) {
	if _, err := exec.LookPath("glab"); err != nil {
		t.Skip("glab not installed")
	}

	out, err := RunForge(PlatformGitLab, "--version")
	if err != nil {
		t.Fatalf("RunForge(glab --version) error: %v", err)
	}
	if !strings.Contains(out, "glab") {
		t.Errorf("expected glab version output, got %q", out)
	}
}

func TestRunForge_Error(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	// gh with an invalid subcommand should fail.
	_, err := RunForge(PlatformGitHub, "definitely-not-a-real-subcommand")
	if err == nil {
		t.Error("expected error for invalid gh subcommand")
	}
	// Error should contain the cli name.
	if err != nil && !strings.Contains(err.Error(), "gh") {
		t.Errorf("error should mention gh, got: %v", err)
	}
}

// --- RunForgeDir ---

func TestRunForgeDir_Success(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	dir := t.TempDir()

	out, err := RunForgeDir(dir, PlatformGitHub, "--version")
	if err != nil {
		t.Fatalf("RunForgeDir(gh --version) error: %v", err)
	}
	if !strings.Contains(out, "gh version") {
		t.Errorf("expected version output, got %q", out)
	}
}

func TestRunForgeDir_Error(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	dir := t.TempDir()

	_, err := RunForgeDir(dir, PlatformGitHub, "definitely-not-a-real-subcommand")
	if err == nil {
		t.Error("expected error for invalid gh subcommand")
	}
}

// --- RunForgePassthrough ---

func TestRunForgePassthrough_Error(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}

	dir := t.TempDir()

	err := RunForgePassthrough(dir, PlatformGitHub, "definitely-not-a-real-subcommand")
	if err == nil {
		t.Error("expected error for invalid gh subcommand")
	}
}

// --- DetectPlatformFromRemote ---

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDetectPlatformFromRemote_GitHub(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "https://github.com/org/repo.git")

	platform, err := DetectPlatformFromRemote(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if platform != PlatformGitHub {
		t.Errorf("platform = %q, want %q", platform, PlatformGitHub)
	}
}

func TestDetectPlatformFromRemote_GitHubSSH(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "git@github.com:org/repo.git")

	platform, err := DetectPlatformFromRemote(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if platform != PlatformGitHub {
		t.Errorf("platform = %q, want %q", platform, PlatformGitHub)
	}
}

func TestDetectPlatformFromRemote_GitLab(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "https://gitlab.com/group/project.git")

	platform, err := DetectPlatformFromRemote(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if platform != PlatformGitLab {
		t.Errorf("platform = %q, want %q", platform, PlatformGitLab)
	}
}

func TestDetectPlatformFromRemote_GitLabSSH(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "git@gitlab.com:group/project.git")

	platform, err := DetectPlatformFromRemote(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if platform != PlatformGitLab {
		t.Errorf("platform = %q, want %q", platform, PlatformGitLab)
	}
}

func TestDetectPlatformFromRemote_SelfHostedGitHub(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "https://git.github.com/org/repo.git")

	platform, err := DetectPlatformFromRemote(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if platform != PlatformGitHub {
		t.Errorf("platform = %q, want %q", platform, PlatformGitHub)
	}
}

func TestDetectPlatformFromRemote_SelfHostedDefault(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	runGit(t, dir, "remote", "add", "origin", "https://git.company.com/team/repo.git")

	platform, err := DetectPlatformFromRemote(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-github hosts default to GitLab.
	if platform != PlatformGitLab {
		t.Errorf("platform = %q, want %q", platform, PlatformGitLab)
	}
}

func TestDetectPlatformFromRemote_NoRemote(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	initGitRepo(t, dir)
	// No remote added.

	_, err := DetectPlatformFromRemote(dir)
	if err == nil {
		t.Error("expected error for repo with no remote")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot detect platform") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDetectPlatformFromRemote_NotGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectPlatformFromRemote(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

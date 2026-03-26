package mergestatus

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCheckCLI_GitHub(t *testing.T) {
	_, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh CLI not installed, skipping")
	}

	if err := checkCLI(PlatformGitHub); err != nil {
		t.Errorf("checkCLI(GitHub) returned error despite gh being installed: %v", err)
	}
}

func TestCheckCLI_GitLab(t *testing.T) {
	_, err := exec.LookPath("glab")
	if err != nil {
		t.Skip("glab CLI not installed, skipping")
	}

	if err := checkCLI(PlatformGitLab); err != nil {
		t.Errorf("checkCLI(GitLab) returned error despite glab being installed: %v", err)
	}
}

func TestQueryGitHub_InvalidURL(t *testing.T) {
	_, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh CLI not installed, skipping")
	}

	// An invalid PR URL should cause gh to fail.
	_, err = queryGitHub("https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	if err == nil {
		t.Error("expected error for invalid GitHub PR URL")
	}
}

func TestQueryGitLab_InvalidURL(t *testing.T) {
	_, err := exec.LookPath("glab")
	if err != nil {
		t.Skip("glab CLI not installed, skipping")
	}

	// An invalid MR URL should cause glab to fail.
	_, err = queryGitLab("https://gitlab.com/nonexistent-group-xyzzy/nonexistent-project-xyzzy/-/merge_requests/999999")
	if err == nil {
		t.Error("expected error for invalid GitLab MR URL")
	}
}

func TestQueryGitLab_UnparseableURL(t *testing.T) {
	_, err := exec.LookPath("glab")
	if err != nil {
		t.Skip("glab CLI not installed, skipping")
	}

	// A URL that can't be parsed by parseGitLabURL.
	_, err = queryGitLab("https://gitlab.com/no-merge-requests-path")
	if err == nil {
		t.Error("expected error for unparseable GitLab URL")
	}
}

func TestQueryMergeStatus_GitHubError(t *testing.T) {
	_, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh CLI not installed, skipping")
	}

	// Should fail at queryGitHub with an invalid PR.
	state, platform, err := QueryMergeStatus("https://github.com/nonexistent-org-xyzzy/nonexistent-repo-xyzzy/pull/999999")
	if err == nil {
		t.Errorf("expected error, got state=%q", state)
	}
	if platform != PlatformGitHub {
		t.Errorf("platform = %q, want %q", platform, PlatformGitHub)
	}
}

func TestQueryMergeStatus_GitLabError(t *testing.T) {
	_, err := exec.LookPath("glab")
	if err != nil {
		t.Skip("glab CLI not installed, skipping")
	}

	// Should fail at queryGitLab with an invalid MR.
	state, platform, err := QueryMergeStatus("https://gitlab.com/nonexistent-group-xyzzy/nonexistent-project-xyzzy/-/merge_requests/999999")
	if err == nil {
		t.Errorf("expected error, got state=%q", state)
	}
	if platform != PlatformGitLab {
		t.Errorf("platform = %q, want %q", platform, PlatformGitLab)
	}
}

func TestCheckCLI_MissingGitHub(t *testing.T) {
	// Temporarily override PATH so gh is not found.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	err := checkCLI(PlatformGitHub)
	if err == nil {
		t.Error("expected error when gh is not in PATH")
	}
	if err != nil && !strings.Contains(err.Error(), "gh CLI not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckCLI_MissingGitLab(t *testing.T) {
	// Temporarily override PATH so glab is not found.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	err := checkCLI(PlatformGitLab)
	if err == nil {
		t.Error("expected error when glab is not in PATH")
	}
	if err != nil && !strings.Contains(err.Error(), "glab CLI not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestQueryMergeStatus_MissingCLI(t *testing.T) {
	// With empty PATH, checkCLI should fail before reaching the query.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	_, platform, err := QueryMergeStatus("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when gh is not in PATH")
	}
	if platform != PlatformGitHub {
		t.Errorf("platform = %q, want %q", platform, PlatformGitHub)
	}
}

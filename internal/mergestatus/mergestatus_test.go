package mergestatus

import (
	"testing"

	"github.com/ezer/repoinjector/internal/repoconfig"
)

func TestReviewStates(t *testing.T) {
	states := ReviewStates()

	expected := []string{"review", "approved", "review-blocked"}
	for _, s := range expected {
		if !states[s] {
			t.Errorf("expected ReviewStates to contain %q", s)
		}
	}

	if len(states) != len(expected) {
		t.Errorf("expected %d review states, got %d", len(expected), len(states))
	}

	// Non-review states should not be present
	for _, s := range []string{"active", "merged", "closed", "done", "paused"} {
		if states[s] {
			t.Errorf("ReviewStates should not contain %q", s)
		}
	}
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected Platform
	}{
		{"github.com", "https://github.com/org/repo/pull/42", PlatformGitHub},
		{"github enterprise subdomain", "https://git.github.com/org/repo/pull/1", PlatformGitHub},
		{"gitlab.com", "https://gitlab.com/group/project/-/merge_requests/42", PlatformGitLab},
		{"self-hosted gitlab", "https://git.company.com/group/project/-/merge_requests/1", PlatformGitLab},
		{"bitbucket falls back to gitlab", "https://bitbucket.org/org/repo/pull-requests/5", PlatformGitLab},
		{"invalid URL falls back to gitlab", "://broken", PlatformGitLab},
		{"empty string falls back to gitlab", "", PlatformGitLab},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPlatform(tt.url)
			if got != tt.expected {
				t.Errorf("DetectPlatform(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

func TestMapGitHubState(t *testing.T) {
	tests := []struct {
		name     string
		pr       ghPRView
		expected string
	}{
		{
			"merged",
			ghPRView{State: "MERGED"},
			string(repoconfig.StateMerged),
		},
		{
			"closed",
			ghPRView{State: "CLOSED"},
			string(repoconfig.StateClosed),
		},
		{
			"open with no review decision",
			ghPRView{State: "OPEN"},
			string(repoconfig.StateReview),
		},
		{
			"open and approved",
			ghPRView{State: "OPEN", ReviewDecision: "APPROVED"},
			string(repoconfig.StateApproved),
		},
		{
			"open approved lowercase",
			ghPRView{State: "open", ReviewDecision: "approved"},
			string(repoconfig.StateApproved),
		},
		{
			"open with failing checks",
			ghPRView{
				State: "OPEN",
				StatusCheckRollup: []ghCheckStatus{
					{Conclusion: "FAILURE"},
				},
			},
			string(repoconfig.StateReviewBlocked),
		},
		{
			"open with failing checks takes precedence over approved",
			ghPRView{
				State:          "OPEN",
				ReviewDecision: "APPROVED",
				StatusCheckRollup: []ghCheckStatus{
					{Conclusion: "FAILURE"},
				},
			},
			string(repoconfig.StateReviewBlocked),
		},
		{
			"open with changes requested",
			ghPRView{State: "OPEN", ReviewDecision: "CHANGES_REQUESTED"},
			string(repoconfig.StateReview),
		},
		{
			"unknown state defaults to review",
			ghPRView{State: "UNKNOWN"},
			string(repoconfig.StateReview),
		},
		{
			"empty state defaults to review",
			ghPRView{},
			string(repoconfig.StateReview),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitHubState(tt.pr)
			if got != tt.expected {
				t.Errorf("mapGitHubState() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHasFailingChecks(t *testing.T) {
	tests := []struct {
		name     string
		checks   []ghCheckStatus
		expected bool
	}{
		{"nil checks", nil, false},
		{"empty checks", []ghCheckStatus{}, false},
		{"all passing", []ghCheckStatus{{Conclusion: "SUCCESS"}}, false},
		{"failure conclusion", []ghCheckStatus{{Conclusion: "FAILURE"}}, true},
		{"timed out conclusion", []ghCheckStatus{{Conclusion: "TIMED_OUT"}}, true},
		{"cancelled conclusion", []ghCheckStatus{{Conclusion: "CANCELLED"}}, true},
		{"failure state", []ghCheckStatus{{State: "FAILURE"}}, true},
		{"error state", []ghCheckStatus{{State: "ERROR"}}, true},
		{
			"mixed - one failing among passing",
			[]ghCheckStatus{
				{Conclusion: "SUCCESS"},
				{Conclusion: "FAILURE"},
				{Conclusion: "SUCCESS"},
			},
			true,
		},
		{
			"all success",
			[]ghCheckStatus{
				{Conclusion: "SUCCESS"},
				{Conclusion: "SUCCESS"},
			},
			false,
		},
		{"pending is not failing", []ghCheckStatus{{Conclusion: "PENDING"}}, false},
		{"neutral is not failing", []ghCheckStatus{{Conclusion: "NEUTRAL"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFailingChecks(tt.checks)
			if got != tt.expected {
				t.Errorf("hasFailingChecks() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMapGitLabState(t *testing.T) {
	tests := []struct {
		name     string
		mr       glabMRView
		expected string
	}{
		{"merged", glabMRView{State: "merged"}, string(repoconfig.StateMerged)},
		{"closed", glabMRView{State: "closed"}, string(repoconfig.StateClosed)},
		{"opened and approved", glabMRView{State: "opened", Approved: true}, string(repoconfig.StateApproved)},
		{"opened not approved", glabMRView{State: "opened", Approved: false}, string(repoconfig.StateReview)},
		{"unknown state defaults to review", glabMRView{State: "unknown"}, string(repoconfig.StateReview)},
		{"empty state defaults to review", glabMRView{}, string(repoconfig.StateReview)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitLabState(tt.mr)
			if got != tt.expected {
				t.Errorf("mapGitLabState() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseGitLabURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantMRID     string
		wantRepoSlug string
		wantErr      bool
	}{
		{
			"simple project",
			"https://gitlab.com/group/project/-/merge_requests/42",
			"42",
			"https://gitlab.com/group/project",
			false,
		},
		{
			"subgroup project",
			"https://gitlab.com/group/sub/project/-/merge_requests/99",
			"99",
			"https://gitlab.com/group/sub/project",
			false,
		},
		{
			"self-hosted",
			"https://git.company.com/team/repo/-/merge_requests/7",
			"7",
			"https://git.company.com/team/repo",
			false,
		},
		{
			"deeply nested subgroups",
			"https://gitlab.com/a/b/c/d/-/merge_requests/123",
			"123",
			"https://gitlab.com/a/b/c/d",
			false,
		},
		{
			"missing merge_requests path",
			"https://gitlab.com/group/project",
			"",
			"",
			true,
		},
		{
			"merge_requests without ID",
			"https://gitlab.com/group/project/-/merge_requests",
			"",
			"",
			true,
		},
		{
			"invalid URL",
			"://bad",
			"",
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrID, repoSlug, err := parseGitLabURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitLabURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if mrID != tt.wantMRID {
				t.Errorf("parseGitLabURL() mrID = %q, want %q", mrID, tt.wantMRID)
			}
			if repoSlug != tt.wantRepoSlug {
				t.Errorf("parseGitLabURL() repoSlug = %q, want %q", repoSlug, tt.wantRepoSlug)
			}
		})
	}
}

func TestCheckCLI_UnknownPlatform(t *testing.T) {
	err := checkCLI(Platform("unknown"))
	if err == nil {
		t.Error("expected error for unknown platform")
	}
}

func TestParseGitLabURL_WithQueryParams(t *testing.T) {
	mrID, repoSlug, err := parseGitLabURL("https://gitlab.com/group/project/-/merge_requests/42?tab=changes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mrID != "42" {
		t.Errorf("mrID = %q, want %q", mrID, "42")
	}
	if repoSlug != "https://gitlab.com/group/project" {
		t.Errorf("repoSlug = %q, want %q", repoSlug, "https://gitlab.com/group/project")
	}
}

func TestParseGitLabURL_WithFragment(t *testing.T) {
	mrID, repoSlug, err := parseGitLabURL("https://gitlab.com/group/project/-/merge_requests/7#note_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mrID != "7" {
		t.Errorf("mrID = %q, want %q", mrID, "7")
	}
	if repoSlug != "https://gitlab.com/group/project" {
		t.Errorf("repoSlug = %q, want %q", repoSlug, "https://gitlab.com/group/project")
	}
}

func TestParseGitLabURL_WithoutDashSeparator(t *testing.T) {
	// Some older GitLab URLs don't have the dash separator
	mrID, repoSlug, err := parseGitLabURL("https://gitlab.com/group/project/merge_requests/15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mrID != "15" {
		t.Errorf("mrID = %q, want %q", mrID, "15")
	}
	if repoSlug != "https://gitlab.com/group/project" {
		t.Errorf("repoSlug = %q, want %q", repoSlug, "https://gitlab.com/group/project")
	}
}

func TestParseGitLabURL_HTTPScheme(t *testing.T) {
	mrID, repoSlug, err := parseGitLabURL("http://git.internal.com/team/repo/-/merge_requests/3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mrID != "3" {
		t.Errorf("mrID = %q, want %q", mrID, "3")
	}
	if repoSlug != "http://git.internal.com/team/repo" {
		t.Errorf("repoSlug = %q, want %q", repoSlug, "http://git.internal.com/team/repo")
	}
}

func TestDetectPlatform_WithPort(t *testing.T) {
	got := DetectPlatform("https://github.com:443/org/repo/pull/1")
	if got != PlatformGitHub {
		t.Errorf("DetectPlatform with port 443 = %q, want %q", got, PlatformGitHub)
	}
}

func TestDetectPlatform_GitLabWithPort(t *testing.T) {
	got := DetectPlatform("https://gitlab.example.com:8443/group/project/-/merge_requests/5")
	if got != PlatformGitLab {
		t.Errorf("DetectPlatform with gitlab port = %q, want %q", got, PlatformGitLab)
	}
}

func TestHasFailingChecks_LowercaseValues(t *testing.T) {
	// hasFailingChecks uses ToUpper internally, so lowercase should also be detected
	tests := []struct {
		name     string
		checks   []ghCheckStatus
		expected bool
	}{
		{"lowercase failure", []ghCheckStatus{{Conclusion: "failure"}}, true},
		{"lowercase timed_out", []ghCheckStatus{{Conclusion: "timed_out"}}, true},
		{"lowercase cancelled", []ghCheckStatus{{Conclusion: "cancelled"}}, true},
		{"lowercase state failure", []ghCheckStatus{{State: "failure"}}, true},
		{"lowercase state error", []ghCheckStatus{{State: "error"}}, true},
		{"lowercase success", []ghCheckStatus{{Conclusion: "success"}}, false},
		{"mixed case failure", []ghCheckStatus{{Conclusion: "Failure"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFailingChecks(tt.checks)
			if got != tt.expected {
				t.Errorf("hasFailingChecks() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMapGitHubState_LowercaseInput(t *testing.T) {
	// mapGitHubState uses ToUpper, so lowercase should work
	tests := []struct {
		name     string
		pr       ghPRView
		expected string
	}{
		{"lowercase merged", ghPRView{State: "merged"}, string(repoconfig.StateMerged)},
		{"lowercase closed", ghPRView{State: "closed"}, string(repoconfig.StateClosed)},
		{"lowercase open", ghPRView{State: "open"}, string(repoconfig.StateReview)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitHubState(tt.pr)
			if got != tt.expected {
				t.Errorf("mapGitHubState() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMapGitLabState_MixedCase(t *testing.T) {
	// mapGitLabState uses ToLower, so mixed case should work
	tests := []struct {
		name     string
		mr       glabMRView
		expected string
	}{
		{"uppercase MERGED", glabMRView{State: "MERGED"}, string(repoconfig.StateMerged)},
		{"uppercase CLOSED", glabMRView{State: "CLOSED"}, string(repoconfig.StateClosed)},
		{"uppercase OPENED", glabMRView{State: "OPENED", Approved: true}, string(repoconfig.StateApproved)},
		{"mixed case Opened", glabMRView{State: "Opened"}, string(repoconfig.StateReview)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitLabState(tt.mr)
			if got != tt.expected {
				t.Errorf("mapGitLabState() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReviewStates_DoesNotContainNonReviewStates(t *testing.T) {
	states := ReviewStates()
	nonReview := []string{"active", "merged", "closed", "done", "paused", ""}
	for _, s := range nonReview {
		if states[s] {
			t.Errorf("ReviewStates should not contain %q", s)
		}
	}
}

func TestResult_FieldsExist(t *testing.T) {
	r := Result{
		Path:          "/path/to/repo",
		Name:          "my-repo",
		MergeURL:      "https://github.com/org/repo/pull/1",
		Platform:      PlatformGitHub,
		PreviousState: "review",
		NewState:      "approved",
		Changed:       true,
		Error:         "",
	}
	if r.Path != "/path/to/repo" {
		t.Error("Path field")
	}
	if r.Platform != PlatformGitHub {
		t.Error("Platform field")
	}
	if !r.Changed {
		t.Error("Changed field")
	}
}

func TestSummary_Fields(t *testing.T) {
	s := Summary{Total: 10, Updated: 3, Unchanged: 6, Errors: 1}
	if s.Total != 10 || s.Updated != 3 || s.Unchanged != 6 || s.Errors != 1 {
		t.Error("Summary fields mismatch")
	}
}

func TestHasFailingChecks_MultipleChecks(t *testing.T) {
	// First failing check should cause true regardless of subsequent checks
	checks := []ghCheckStatus{
		{Conclusion: "SUCCESS"},
		{Conclusion: "SUCCESS"},
		{State: "ERROR"},
		{Conclusion: "SUCCESS"},
	}
	if !hasFailingChecks(checks) {
		t.Error("should detect failing check among multiple passing ones")
	}
}

func TestHasFailingChecks_AllPassing(t *testing.T) {
	checks := []ghCheckStatus{
		{Conclusion: "SUCCESS"},
		{Conclusion: "SUCCESS"},
		{Conclusion: "NEUTRAL"},
		{Conclusion: "SKIPPED"},
	}
	if hasFailingChecks(checks) {
		t.Error("should not report failing when all pass")
	}
}

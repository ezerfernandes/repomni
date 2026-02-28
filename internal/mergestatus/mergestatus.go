package mergestatus

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
)

// Platform represents a git hosting platform.
type Platform string

const (
	PlatformGitHub Platform = "github"
	PlatformGitLab Platform = "gitlab"
)

// ReviewStates returns the set of states that represent an active review.
// sync-status only checks branches in these states.
func ReviewStates() map[string]bool {
	return map[string]bool{
		string(repoconfig.StateReview):        true,
		string(repoconfig.StateApproved):      true,
		string(repoconfig.StateReviewBlocked): true,
	}
}

// Result holds the outcome of checking one repo's merge status.
type Result struct {
	Path          string   `json:"path"`
	Name          string   `json:"name"`
	MergeURL      string   `json:"merge_url"`
	Platform      Platform `json:"platform"`
	PreviousState string   `json:"previous_state"`
	NewState      string   `json:"new_state"`
	Changed       bool     `json:"changed"`
	Error         string   `json:"error,omitempty"`
}

// Summary aggregates sync-status results.
type Summary struct {
	Total     int `json:"total"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Errors    int `json:"errors"`
}

// DetectPlatform determines the hosting platform from a merge URL.
// github.com URLs use gh; everything else uses glab.
func DetectPlatform(mergeURL string) Platform {
	parsed, err := url.Parse(mergeURL)
	if err != nil {
		return PlatformGitLab
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		return PlatformGitHub
	}
	return PlatformGitLab
}

// QueryMergeStatus queries the platform for the current PR/MR state
// and returns the corresponding repoinjector workflow state.
func QueryMergeStatus(mergeURL string) (string, Platform, error) {
	platform := DetectPlatform(mergeURL)

	if err := checkCLI(platform); err != nil {
		return "", platform, err
	}

	var newState string
	var err error
	switch platform {
	case PlatformGitHub:
		newState, err = queryGitHub(mergeURL)
	case PlatformGitLab:
		newState, err = queryGitLab(mergeURL)
	default:
		return "", platform, fmt.Errorf("unsupported platform for URL: %s", mergeURL)
	}

	return newState, platform, err
}

func checkCLI(platform Platform) error {
	var name string
	switch platform {
	case PlatformGitHub:
		name = "gh"
	case PlatformGitLab:
		name = "glab"
	default:
		return fmt.Errorf("unknown platform")
	}
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s CLI not found in PATH; install it to use sync-status with %s URLs", name, platform)
	}
	return nil
}

// --- GitHub ---

type ghPRView struct {
	State             string          `json:"state"`
	ReviewDecision    string          `json:"reviewDecision"`
	StatusCheckRollup []ghCheckStatus `json:"statusCheckRollup"`
}

type ghCheckStatus struct {
	State      string `json:"state"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

func queryGitHub(mergeURL string) (string, error) {
	cmd := exec.Command("gh", "pr", "view", mergeURL,
		"--json", "state,reviewDecision,statusCheckRollup")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr view failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	var pr ghPRView
	if err := json.Unmarshal(out, &pr); err != nil {
		return "", fmt.Errorf("cannot parse gh output: %w", err)
	}

	return mapGitHubState(pr), nil
}

func mapGitHubState(pr ghPRView) string {
	switch strings.ToUpper(pr.State) {
	case "MERGED":
		return string(repoconfig.StateMerged)
	case "CLOSED":
		return string(repoconfig.StateClosed)
	case "OPEN":
		if hasFailingChecks(pr.StatusCheckRollup) {
			return string(repoconfig.StateReviewBlocked)
		}
		if strings.ToUpper(pr.ReviewDecision) == "APPROVED" {
			return string(repoconfig.StateApproved)
		}
		return string(repoconfig.StateReview)
	default:
		return string(repoconfig.StateReview)
	}
}

func hasFailingChecks(checks []ghCheckStatus) bool {
	for _, c := range checks {
		conclusion := strings.ToUpper(c.Conclusion)
		state := strings.ToUpper(c.State)
		if conclusion == "FAILURE" || conclusion == "TIMED_OUT" || conclusion == "CANCELLED" {
			return true
		}
		if state == "FAILURE" || state == "ERROR" {
			return true
		}
	}
	return false
}

// --- GitLab ---

type glabMRView struct {
	State    string `json:"state"`
	Approved bool   `json:"approved"`
}

func queryGitLab(mergeURL string) (string, error) {
	mrID, repoSlug, err := parseGitLabURL(mergeURL)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("glab", "mr", "view", mrID,
		"--repo", repoSlug, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("glab mr view failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	var mr glabMRView
	if err := json.Unmarshal(out, &mr); err != nil {
		return "", fmt.Errorf("cannot parse glab output: %w", err)
	}

	return mapGitLabState(mr), nil
}

func parseGitLabURL(mergeURL string) (mrID string, repoSlug string, err error) {
	parsed, err := url.Parse(mergeURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid GitLab URL: %w", err)
	}

	// Path: /group/project/-/merge_requests/42
	// or:   /group/subgroup/project/-/merge_requests/42
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	mrIdx := -1
	for i, p := range parts {
		if p == "merge_requests" && i+1 < len(parts) {
			mrIdx = i
			break
		}
	}
	if mrIdx < 0 || mrIdx+1 >= len(parts) {
		return "", "", fmt.Errorf("cannot parse merge request ID from URL: %s", mergeURL)
	}

	mrID = parts[mrIdx+1]

	// Everything before "/-/merge_requests" is the project slug
	dashIdx := -1
	for i, p := range parts {
		if p == "-" && i < mrIdx {
			dashIdx = i
			break
		}
	}

	var projectParts []string
	if dashIdx >= 0 {
		projectParts = parts[:dashIdx]
	} else {
		projectParts = parts[:mrIdx]
	}

	repoSlug = parsed.Scheme + "://" + parsed.Host + "/" + strings.Join(projectParts, "/")
	return mrID, repoSlug, nil
}

func mapGitLabState(mr glabMRView) string {
	switch strings.ToLower(mr.State) {
	case "merged":
		return string(repoconfig.StateMerged)
	case "closed":
		return string(repoconfig.StateClosed)
	case "opened":
		if mr.Approved {
			return string(repoconfig.StateApproved)
		}
		return string(repoconfig.StateReview)
	default:
		return string(repoconfig.StateReview)
	}
}

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach [url]",
	Short: "Attach an existing PR/MR to the current branch repo",
	Long: `Associate a pull request (GitHub) or merge request (GitLab) with the current
branch repo. The PR/MR state is queried and stored along with the URL.

Provide a URL directly:
  repomni branch attach https://github.com/org/repo/pull/42

Or discover the PR/MR for the current branch automatically:
  repomni branch attach --current`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAttach,
}

var (
	attachCurrent bool
	attachJSON    bool
)

func init() {
	branchCmd.AddCommand(attachCmd)
	attachCmd.Flags().BoolVar(&attachCurrent, "current", false, "discover PR/MR for the current branch")
	attachCmd.Flags().BoolVar(&attachJSON, "json", false, "output as JSON")
}

// ghPRViewFull is used to parse the full pr view JSON for attach and clone.
type ghPRViewFull struct {
	URL               string `json:"url"`
	Number            int    `json:"number"`
	State             string `json:"state"`
	IsDraft           bool   `json:"isDraft"`
	BaseRefName       string `json:"baseRefName"`
	HeadRefName       string `json:"headRefName"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	ReviewDecision    string `json:"reviewDecision"`
}

// glabMRViewFull is used to parse the full mr view JSON for attach and clone.
type glabMRViewFull struct {
	WebURL          string `json:"web_url"`
	IID             int    `json:"iid"`
	State           string `json:"state"`
	Draft           bool   `json:"draft"`
	TargetBranch    string `json:"target_branch"`
	SourceBranch    string `json:"source_branch"`
	SourceProjectID int    `json:"source_project_id"`
	TargetProjectID int    `json:"target_project_id"`
	Approved        bool   `json:"approved"`
}

// mergeRequestInfo is the platform-neutral view of a PR/MR queried by URL.
type mergeRequestInfo struct {
	Number     int
	State      string // repomni workflow state
	Draft      bool
	BaseBranch string
	HeadBranch string
	// CrossRepo is true when the head branch lives in a fork rather than
	// in the repository the PR/MR targets.
	CrossRepo bool
}

func runAttach(cmd *cobra.Command, args []string) error {
	if !attachCurrent && len(args) == 0 {
		return fmt.Errorf("provide a PR/MR URL, or use --current to auto-discover")
	}
	if attachCurrent && len(args) > 0 {
		return fmt.Errorf("--current and a URL are mutually exclusive")
	}

	repoRoot, err := gitutil.RunGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	gitDir, err := gitutil.FindGitDir(repoRoot)
	if err != nil {
		return err
	}

	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &repoconfig.RepoConfig{Version: 1}
	}

	if attachCurrent {
		return attachFromCurrentBranch(repoRoot, gitDir, cfg)
	}
	return attachFromURL(args[0], repoRoot, gitDir, cfg)
}

func attachFromURL(mergeURL, repoRoot, gitDir string, cfg *repoconfig.RepoConfig) error {
	if err := validateMergeURL(mergeURL); err != nil {
		return err
	}

	platform := mergestatus.DetectPlatform(mergeURL)
	if err := forge.CheckCLI(platform); err != nil {
		return err
	}

	state, err := queryAndPopulate(platform, mergeURL, repoRoot, cfg)
	if err != nil {
		return err
	}

	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	if attachJSON {
		return printAttachJSON(cfg, state)
	}

	fmt.Printf("Attached: %s (state: %s)\n", cfg.MergeURL, state)
	return nil
}

func printAttachJSON(cfg *repoconfig.RepoConfig, state string) error {
	return ui.PrintJSON(struct {
		MergeURL    string `json:"merge_url"`
		MergeNumber int    `json:"merge_number"`
		State       string `json:"state"`
		Draft       bool   `json:"draft"`
		BaseBranch  string `json:"base_branch,omitempty"`
	}{
		MergeURL:    cfg.MergeURL,
		MergeNumber: cfg.MergeNumber,
		State:       state,
		Draft:       cfg.Draft,
		BaseBranch:  cfg.BaseBranch,
	})
}

func attachFromCurrentBranch(repoRoot, gitDir string, cfg *repoconfig.RepoConfig) error {
	platform, err := forge.DetectPlatformFromRemote(repoRoot)
	if err != nil {
		return err
	}
	if err := forge.CheckCLI(platform); err != nil {
		return err
	}

	var mergeURL, state string

	switch platform {
	case forge.PlatformGitHub:
		out, err := forge.RunForgeDir(repoRoot, platform,
			"pr", "view", "--json", "url,number,state,isDraft,baseRefName,reviewDecision")
		if err != nil {
			return fmt.Errorf("no PR found for current branch; create one with \"branch submit\"")
		}
		var pr ghPRViewFull
		if err := json.Unmarshal([]byte(out), &pr); err != nil {
			return fmt.Errorf("cannot parse gh output: %w", err)
		}
		mergeURL = pr.URL
		cfg.MergeURL = pr.URL
		cfg.MergeNumber = pr.Number
		cfg.BaseBranch = pr.BaseRefName
		cfg.Draft = pr.IsDraft
		state = mapGHState(pr.State, pr.ReviewDecision)

	case forge.PlatformGitLab:
		out, err := forge.RunForgeDir(repoRoot, platform,
			"mr", "view", "--output", "json")
		if err != nil {
			return fmt.Errorf("no MR found for current branch; create one with \"branch submit\"")
		}
		var mr glabMRViewFull
		if err := json.Unmarshal([]byte(out), &mr); err != nil {
			return fmt.Errorf("cannot parse glab output: %w", err)
		}
		mergeURL = mr.WebURL
		cfg.MergeURL = mr.WebURL
		cfg.MergeNumber = mr.IID
		cfg.BaseBranch = mr.TargetBranch
		cfg.Draft = mr.Draft
		state = mapGLState(mr.State, mr.Approved)
	}

	cfg.State = state

	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	if attachJSON {
		return printAttachJSON(cfg, state)
	}

	fmt.Printf("Attached: %s (state: %s)\n", mergeURL, state)
	return nil
}

func queryAndPopulate(platform forge.Platform, mergeURL, repoRoot string, cfg *repoconfig.RepoConfig) (string, error) {
	info, err := queryMergeRequest(platform, mergeURL, repoRoot)
	if err != nil {
		return "", err
	}
	populateMergeConfig(cfg, mergeURL, info)
	return info.State, nil
}

// populateMergeConfig records the PR/MR identified by mergeURL in cfg.
func populateMergeConfig(cfg *repoconfig.RepoConfig, mergeURL string, info *mergeRequestInfo) {
	cfg.MergeURL = mergeURL
	cfg.MergeNumber = info.Number
	cfg.BaseBranch = info.BaseBranch
	cfg.Draft = info.Draft
	cfg.State = info.State
}

// queryMergeRequest fetches the PR/MR at mergeURL with gh or glab, run from repoRoot.
func queryMergeRequest(platform forge.Platform, mergeURL, repoRoot string) (*mergeRequestInfo, error) {
	switch platform {
	case forge.PlatformGitHub:
		out, err := forge.RunForgeDir(repoRoot, platform,
			"pr", "view", mergeURL, "--json", "url,number,state,isDraft,baseRefName,headRefName,isCrossRepository,reviewDecision")
		if err != nil {
			return nil, fmt.Errorf("cannot query PR: %w", err)
		}
		var pr ghPRViewFull
		if err := json.Unmarshal([]byte(out), &pr); err != nil {
			return nil, fmt.Errorf("cannot parse gh output: %w", err)
		}
		return &mergeRequestInfo{
			Number:     pr.Number,
			State:      mapGHState(pr.State, pr.ReviewDecision),
			Draft:      pr.IsDraft,
			BaseBranch: pr.BaseRefName,
			HeadBranch: pr.HeadRefName,
			CrossRepo:  pr.IsCrossRepository,
		}, nil

	case forge.PlatformGitLab:
		mrID := fmt.Sprintf("%d", forge.ParseMergeNumber(mergeURL))
		if mrID == "0" {
			return nil, fmt.Errorf("cannot parse MR number from URL: %s", mergeURL)
		}
		out, err := forge.RunForgeDir(repoRoot, platform,
			"mr", "view", mrID, "--output", "json")
		if err != nil {
			return nil, fmt.Errorf("cannot query MR: %w", err)
		}
		var mr glabMRViewFull
		if err := json.Unmarshal([]byte(out), &mr); err != nil {
			return nil, fmt.Errorf("cannot parse glab output: %w", err)
		}
		return &mergeRequestInfo{
			Number:     mr.IID,
			State:      mapGLState(mr.State, mr.Approved),
			Draft:      mr.Draft,
			BaseBranch: mr.TargetBranch,
			HeadBranch: mr.SourceBranch,
			CrossRepo:  mr.SourceProjectID != mr.TargetProjectID,
		}, nil
	}
	return nil, fmt.Errorf("unsupported platform for URL: %s", mergeURL)
}

func mapGHState(state, reviewDecision string) string {
	switch state {
	case "MERGED":
		return string(repoconfig.StateMerged)
	case "CLOSED":
		return string(repoconfig.StateClosed)
	default:
		if reviewDecision == "APPROVED" {
			return string(repoconfig.StateApproved)
		}
		return string(repoconfig.StateReview)
	}
}

func mapGLState(state string, approved bool) string {
	switch state {
	case "merged":
		return string(repoconfig.StateMerged)
	case "closed":
		return string(repoconfig.StateClosed)
	default:
		if approved {
			return string(repoconfig.StateApproved)
		}
		return string(repoconfig.StateReview)
	}
}

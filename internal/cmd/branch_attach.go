package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/mergestatus"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
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

var attachCurrent bool

func init() {
	branchCmd.AddCommand(attachCmd)
	attachCmd.Flags().BoolVar(&attachCurrent, "current", false, "discover PR/MR for the current branch")
}

// ghPRViewFull is used to parse the full pr view JSON for attach.
type ghPRViewFull struct {
	URL            string `json:"url"`
	Number         int    `json:"number"`
	State          string `json:"state"`
	IsDraft        bool   `json:"isDraft"`
	BaseRefName    string `json:"baseRefName"`
	ReviewDecision string `json:"reviewDecision"`
}

// glabMRViewFull is used to parse the full mr view JSON for attach.
type glabMRViewFull struct {
	WebURL       string `json:"web_url"`
	IID          int    `json:"iid"`
	State        string `json:"state"`
	Draft        bool   `json:"draft"`
	TargetBranch string `json:"target_branch"`
	Approved     bool   `json:"approved"`
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

	fmt.Printf("Attached: %s (state: %s)\n", cfg.MergeURL, state)
	return nil
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

	fmt.Printf("Attached: %s (state: %s)\n", mergeURL, state)
	return nil
}

func queryAndPopulate(platform forge.Platform, mergeURL, repoRoot string, cfg *repoconfig.RepoConfig) (string, error) {
	var state string

	switch platform {
	case forge.PlatformGitHub:
		out, err := forge.RunForgeDir(repoRoot, platform,
			"pr", "view", mergeURL, "--json", "url,number,state,isDraft,baseRefName,reviewDecision")
		if err != nil {
			return "", fmt.Errorf("cannot query PR: %w", err)
		}
		var pr ghPRViewFull
		if err := json.Unmarshal([]byte(out), &pr); err != nil {
			return "", fmt.Errorf("cannot parse gh output: %w", err)
		}
		cfg.MergeURL = mergeURL
		cfg.MergeNumber = pr.Number
		cfg.BaseBranch = pr.BaseRefName
		cfg.Draft = pr.IsDraft
		state = mapGHState(pr.State, pr.ReviewDecision)

	case forge.PlatformGitLab:
		mrID := fmt.Sprintf("%d", forge.ParseMergeNumber(mergeURL))
		if mrID == "0" {
			return "", fmt.Errorf("cannot parse MR number from URL: %s", mergeURL)
		}
		out, err := forge.RunForgeDir(repoRoot, platform,
			"mr", "view", mrID, "--output", "json")
		if err != nil {
			return "", fmt.Errorf("cannot query MR: %w", err)
		}
		var mr glabMRViewFull
		if err := json.Unmarshal([]byte(out), &mr); err != nil {
			return "", fmt.Errorf("cannot parse glab output: %w", err)
		}
		cfg.MergeURL = mergeURL
		cfg.MergeNumber = mr.IID
		cfg.BaseBranch = mr.TargetBranch
		cfg.Draft = mr.Draft
		state = mapGLState(mr.State, mr.Approved)
	}

	cfg.State = state
	return state, nil
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

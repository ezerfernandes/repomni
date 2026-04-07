package syncer

import (
	"fmt"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/gitutil"
	"golang.org/x/sync/errgroup"
)

// RepoState represents the git sync state of a repository.
type RepoState string

const (
	StateCurrent    RepoState = "current"
	StateBehind     RepoState = "behind"
	StateAhead      RepoState = "ahead"
	StateDiverged   RepoState = "diverged"
	StateDirty      RepoState = "dirty"
	StateNoUpstream RepoState = "no-upstream"
	StateError      RepoState = "error"
	StatePulled     RepoState = "pulled"
)

// RepoStatus holds the full git status of a single repository.
type RepoStatus struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Branch   string    `json:"branch"`
	Upstream string    `json:"upstream,omitempty"`
	Ahead    int       `json:"ahead"`
	Behind   int       `json:"behind"`
	Dirty    bool      `json:"dirty"`
	State    RepoState `json:"state"`
	Detail   string    `json:"detail"`
	Error    error     `json:"-"`
}

// SyncResult is the outcome of a sync operation on one repo.
type SyncResult struct {
	RepoStatus
	Action     string `json:"action"`
	PostDetail string `json:"post_detail"`
}

// SyncOptions controls the sync operation.
type SyncOptions struct {
	DryRun    bool
	AutoStash bool
	Jobs      int
	NoFetch   bool
	NoTags    bool
	Strategy  string
}

// SyncSummary aggregates results across all repos.
type SyncSummary struct {
	Total     int `json:"total"`
	Pulled    int `json:"pulled"`
	Current   int `json:"current"`
	Skipped   int `json:"skipped"`
	Conflicts int `json:"conflicts"`
	Errors    int `json:"errors"`
}

// CheckStatus determines the git sync status of a single repo.
func CheckStatus(repoDir string, noFetch bool, noTags bool) RepoStatus {
	status := RepoStatus{
		Path: repoDir,
		Name: filepath.Base(repoDir),
	}

	if noFetch {
		// Single git call gets everything: branch, upstream, dirty, ahead/behind.
		info, err := gitutil.GetRepoInfo(repoDir)
		if err != nil {
			status.State = StateError
			status.Error = err
			status.Detail = fmt.Sprintf("cannot determine status: %v", err)
			return status
		}
		if info.Branch == "" {
			status.State = StateError
			status.Detail = "detached HEAD"
			return status
		}
		status.Branch = info.Branch
		if info.Upstream == "" {
			status.State = StateNoUpstream
			status.Detail = "no upstream tracking ref"
			return status
		}
		status.Upstream = info.Upstream
		status.Dirty = info.Dirty
		status.Ahead = info.Ahead
		status.Behind = info.Behind
	} else {
		// With fetch: fetch first, then single GetRepoInfo for everything.
		// This is 2 git processes instead of 3+ in the original code.
		if err := gitutil.Fetch(repoDir, noTags); err != nil {
			status.State = StateError
			status.Error = err
			status.Detail = fmt.Sprintf("fetch failed: %v", err)
			return status
		}

		info, err := gitutil.GetRepoInfo(repoDir)
		if err != nil {
			status.State = StateError
			status.Error = err
			status.Detail = fmt.Sprintf("cannot determine status: %v", err)
			return status
		}
		if info.Branch == "" {
			status.State = StateError
			status.Detail = "detached HEAD"
			return status
		}
		status.Branch = info.Branch
		if info.Upstream == "" {
			status.State = StateNoUpstream
			status.Detail = "no upstream tracking ref"
			return status
		}
		status.Upstream = info.Upstream
		status.Dirty = info.Dirty
		status.Ahead = info.Ahead
		status.Behind = info.Behind
	}

	switch {
	case status.Ahead > 0 && status.Behind > 0:
		status.State = StateDiverged
		status.Detail = fmt.Sprintf("%d ahead, %d behind", status.Ahead, status.Behind)
	case status.Behind > 0 && status.Dirty:
		status.State = StateDirty
		status.Detail = fmt.Sprintf("%d behind, has uncommitted changes", status.Behind)
	case status.Behind > 0:
		status.State = StateBehind
		status.Detail = fmt.Sprintf("%d behind", status.Behind)
	case status.Ahead > 0:
		status.State = StateAhead
		status.Detail = fmt.Sprintf("%d ahead", status.Ahead)
	case status.Dirty:
		status.State = StateDirty
		status.Detail = "uncommitted changes"
	default:
		status.State = StateCurrent
		status.Detail = "up to date"
	}

	return status
}

// SyncRepo attempts to pull updates for a single repository.
func SyncRepo(repoDir string, opts SyncOptions) SyncResult {
	strategy := opts.Strategy
	if strategy == "" {
		strategy = "ff-only"
	}

	status := CheckStatus(repoDir, opts.NoFetch, opts.NoTags)
	result := SyncResult{RepoStatus: status}

	if status.State == StateError || status.State == StateNoUpstream {
		result.Action = "skipped"
		result.PostDetail = status.Detail
		return result
	}

	if status.State == StateCurrent || status.State == StateAhead {
		result.Action = "skipped"
		result.PostDetail = status.Detail
		return result
	}

	if opts.DryRun {
		result.Action = "dry-run"
		switch status.State {
		case StateBehind:
			result.PostDetail = fmt.Sprintf("would pull %d commits", status.Behind)
		case StateDirty:
			if opts.AutoStash {
				result.PostDetail = fmt.Sprintf("would stash, pull %d commits, unstash", status.Behind)
			} else {
				result.PostDetail = "dirty working tree, would skip (use --autostash)"
			}
		case StateDiverged:
			result.PostDetail = "diverged, would skip"
		}
		return result
	}

	if status.State == StateDiverged {
		result.Action = "skipped"
		result.PostDetail = "diverged; manual resolution required"
		return result
	}

	if status.Dirty && !opts.AutoStash {
		result.Action = "skipped"
		result.PostDetail = "dirty working tree (use --autostash to stash before pull)"
		return result
	}

	// If we already fetched in CheckStatus, use MergeUpstream to avoid a
	// redundant fetch. Otherwise fall back to Pull which includes its own fetch.
	var err error
	if !opts.NoFetch {
		_, err = gitutil.MergeUpstream(repoDir, strategy, opts.AutoStash && status.Dirty)
	} else {
		_, err = gitutil.Pull(repoDir, strategy, opts.AutoStash && status.Dirty)
	}
	if err != nil {
		result.Action = "error"
		result.PostDetail = fmt.Sprintf("pull failed: %v", err)
		result.State = StateError
		result.Error = err
		return result
	}

	result.Action = "pulled"
	result.State = StatePulled
	result.PostDetail = fmt.Sprintf("pulled %d commits", status.Behind)
	return result
}

// SyncAll runs sync across multiple repos, optionally in parallel.
func SyncAll(repos []string, opts SyncOptions) ([]SyncResult, SyncSummary) {
	results := make([]SyncResult, len(repos))

	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = 1
	}

	g := new(errgroup.Group)
	g.SetLimit(jobs)

	for i, repo := range repos {
		g.Go(func() error {
			results[i] = SyncRepo(repo, opts)
			return nil
		})
	}

	_ = g.Wait()

	var summary SyncSummary
	summary.Total = len(repos)
	for i := range results {
		r := &results[i]
		switch r.Action {
		case "pulled":
			summary.Pulled++
		case "skipped":
			if r.State == StateDiverged || r.State == StateDirty {
				summary.Conflicts++
			} else {
				summary.Current++
			}
		case "dry-run":
			summary.Skipped++
		case "error":
			summary.Errors++
		default:
			summary.Current++
		}
	}

	return results, summary
}

// StatusAll checks status for multiple repos (no pull), optionally in parallel.
func StatusAll(repos []string, noFetch bool, noTags bool, jobs int) []RepoStatus {
	statuses := make([]RepoStatus, len(repos))

	if jobs <= 0 {
		jobs = 1
	}

	g := new(errgroup.Group)
	g.SetLimit(jobs)

	for i, repo := range repos {
		g.Go(func() error {
			statuses[i] = CheckStatus(repo, noFetch, noTags)
			return nil
		})
	}

	_ = g.Wait()
	return statuses
}

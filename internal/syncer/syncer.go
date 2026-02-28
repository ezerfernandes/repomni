package syncer

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
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
func CheckStatus(repoDir string, noFetch bool) RepoStatus {
	status := RepoStatus{
		Path: repoDir,
		Name: filepath.Base(repoDir),
	}

	branch, err := gitutil.CurrentBranch(repoDir)
	if err != nil {
		status.State = StateError
		status.Error = err
		status.Detail = fmt.Sprintf("cannot determine branch: %v", err)
		return status
	}
	if branch == "" {
		status.State = StateError
		status.Detail = "detached HEAD"
		return status
	}
	status.Branch = branch

	if !noFetch {
		if err := gitutil.Fetch(repoDir); err != nil {
			status.State = StateError
			status.Error = err
			status.Detail = fmt.Sprintf("fetch failed: %v", err)
			return status
		}
	}

	upstream, err := gitutil.UpstreamRef(repoDir)
	if err != nil || upstream == "" {
		status.State = StateNoUpstream
		status.Detail = "no upstream tracking ref"
		return status
	}
	status.Upstream = upstream

	dirty, err := gitutil.IsDirty(repoDir)
	if err != nil {
		status.State = StateError
		status.Error = err
		status.Detail = fmt.Sprintf("cannot check working tree: %v", err)
		return status
	}
	status.Dirty = dirty

	ahead, behind, err := gitutil.AheadBehind(repoDir)
	if err != nil {
		status.State = StateError
		status.Error = err
		status.Detail = fmt.Sprintf("cannot determine ahead/behind: %v", err)
		return status
	}
	status.Ahead = ahead
	status.Behind = behind

	switch {
	case ahead > 0 && behind > 0:
		status.State = StateDiverged
		status.Detail = fmt.Sprintf("%d ahead, %d behind", ahead, behind)
	case behind > 0 && dirty:
		status.State = StateDirty
		status.Detail = fmt.Sprintf("%d behind, has uncommitted changes", behind)
	case behind > 0:
		status.State = StateBehind
		status.Detail = fmt.Sprintf("%d behind", behind)
	case ahead > 0:
		status.State = StateAhead
		status.Detail = fmt.Sprintf("%d ahead", ahead)
	case dirty:
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
	status := CheckStatus(repoDir, opts.NoFetch)
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

	strategy := opts.Strategy
	if strategy == "" {
		strategy = "ff-only"
	}

	_, err := gitutil.Pull(repoDir, strategy, opts.AutoStash && status.Dirty)
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

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(jobs)

	var mu sync.Mutex
	for i, repo := range repos {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			result := SyncRepo(repo, opts)
			mu.Lock()
			results[i] = result
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

	var summary SyncSummary
	summary.Total = len(repos)
	for _, r := range results {
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
func StatusAll(repos []string, noFetch bool, jobs int) []RepoStatus {
	statuses := make([]RepoStatus, len(repos))

	if jobs <= 0 {
		jobs = 1
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(jobs)

	var mu sync.Mutex
	for i, repo := range repos {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			s := CheckStatus(repo, noFetch)
			mu.Lock()
			statuses[i] = s
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()
	return statuses
}

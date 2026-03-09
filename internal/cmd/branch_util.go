package cmd

import (
	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

// resolveMergeNumber returns the MR/PR number from the cached config field,
// falling back to parsing it from MergeURL. This handles repos that were
// attached via the old "set-state review <url>" path before the cache fields
// existed.
func resolveMergeNumber(cfg *repoconfig.RepoConfig) int {
	if cfg.MergeNumber != 0 {
		return cfg.MergeNumber
	}
	return forge.ParseMergeNumber(cfg.MergeURL)
}

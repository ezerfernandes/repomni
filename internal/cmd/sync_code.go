package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/syncer"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var syncCodeCmd = &cobra.Command{
	Use:   "code [directory]",
	Short: "Pull updates for git repos in a directory",
	Long: `Fetch and pull updates for all git repositories that are immediate
subdirectories of the target directory.

If no directory is specified, the current directory is used. Each repo is
checked for upstream changes, and repos that are behind are pulled.

Repos with dirty working trees are skipped unless --autostash is used.
Diverged repos are always skipped (manual resolution required).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSyncCode,
}

var (
	syncCodeDryRun    bool
	syncCodeAutoStash bool
	syncCodeJobs      int
	syncCodeNoFetch   bool
	syncCodeNoTags    bool
	syncCodeStrategy  string
	syncCodeJSON      bool
)

func init() {
	syncCodeCmd.Flags().BoolVar(&syncCodeDryRun, "dry-run", false, "show what would be done without pulling")
	syncCodeCmd.Flags().BoolVar(&syncCodeAutoStash, "autostash", false, "stash dirty working trees before pull")
	syncCodeCmd.Flags().IntVarP(&syncCodeJobs, "jobs", "j", 1, "number of parallel sync workers")
	syncCodeCmd.Flags().BoolVar(&syncCodeNoFetch, "no-fetch", false, "skip git fetch (local status only)")
	syncCodeCmd.Flags().BoolVar(&syncCodeNoTags, "no-tags", false, "do not fetch tags")
	syncCodeCmd.Flags().StringVar(&syncCodeStrategy, "strategy", "ff-only", "pull strategy: ff-only, rebase, merge")
	syncCodeCmd.Flags().BoolVar(&syncCodeJSON, "json", false, "output as JSON")
}

// gatherSyncCode runs the sync-code operation and returns the raw results
// without printing anything. Used by runSyncAll for combined JSON output.
func gatherSyncCode(args []string) ([]syncer.SyncResult, syncer.SyncSummary, error) {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return nil, syncer.SyncSummary{}, err
	}

	repos, err := gitutil.FindGitRepos(target)
	if err != nil {
		return nil, syncer.SyncSummary{}, err
	}
	if len(repos) == 0 {
		return nil, syncer.SyncSummary{}, fmt.Errorf("no git repositories found under %s", target)
	}

	opts := syncer.SyncOptions{
		DryRun:    syncCodeDryRun,
		AutoStash: syncCodeAutoStash,
		Jobs:      syncCodeJobs,
		NoFetch:   syncCodeNoFetch,
		NoTags:    syncCodeNoTags,
		Strategy:  syncCodeStrategy,
	}

	results, summary := syncer.SyncAll(repos, opts)
	return results, summary, nil
}

func runSyncCode(cmd *cobra.Command, args []string) error {
	results, summary, err := gatherSyncCode(args)
	if err != nil {
		return err
	}

	if syncCodeJSON {
		return ui.PrintSyncJSON(results, summary)
	}

	ui.PrintSyncResults(results, summary)

	if syncCodeDryRun {
		fmt.Println("\nDry run \u2014 no changes were made.")
	}

	if summary.Errors > 0 {
		return fmt.Errorf("%d repos had errors", summary.Errors)
	}
	if summary.Conflicts > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d repo(s) have conflicts requiring manual resolution\n", summary.Conflicts)
	}

	return nil
}

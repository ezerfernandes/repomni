package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var syncAllCmd = &cobra.Command{
	Use:   "sync [directory]",
	Short: "Pull code updates and refresh PR/MR states",
	Long: `Run sync-code and sync-state together.

First fetches and pulls git updates for all repos under the target directory,
then queries GitHub/GitLab for PR/MR status changes and updates workflow states.

If no directory is specified, the current directory is used.
All flags from sync-code are supported. Use sync-code or sync-state directly
if you only need one operation.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSyncAll,
}

var (
	syncAllDryRun    bool
	syncAllAutoStash bool
	syncAllJobs      int
	syncAllNoFetch   bool
	syncAllStrategy  string
	syncAllJSON      bool
)

func init() {
	rootCmd.AddCommand(syncAllCmd)
	syncAllCmd.Flags().BoolVar(&syncAllDryRun, "dry-run", false, "show what would be done without making changes")
	syncAllCmd.Flags().BoolVar(&syncAllAutoStash, "autostash", false, "stash dirty working trees before pull")
	syncAllCmd.Flags().IntVarP(&syncAllJobs, "jobs", "j", 1, "number of parallel sync workers")
	syncAllCmd.Flags().BoolVar(&syncAllNoFetch, "no-fetch", false, "skip git fetch (local status only)")
	syncAllCmd.Flags().StringVar(&syncAllStrategy, "strategy", "ff-only", "pull strategy: ff-only, rebase, merge")
	syncAllCmd.Flags().BoolVar(&syncAllJSON, "json", false, "output as JSON")
}

func runSyncAll(cmd *cobra.Command, args []string) error {
	// Forward flags to sync-code.
	syncCodeDryRun = syncAllDryRun
	syncCodeAutoStash = syncAllAutoStash
	syncCodeJobs = syncAllJobs
	syncCodeNoFetch = syncAllNoFetch
	syncCodeStrategy = syncAllStrategy
	syncCodeJSON = syncAllJSON

	// Forward flags to sync-state.
	syncStateDryRun = syncAllDryRun
	syncStateJSON = syncAllJSON

	var errs []string

	if err := runSyncCode(cmd, args); err != nil {
		errs = append(errs, fmt.Sprintf("sync-code: %v", err))
	}

	if !syncAllJSON {
		fmt.Println()
	}

	if err := runSyncState(cmd, args); err != nil {
		errs = append(errs, fmt.Sprintf("sync-state: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil
}

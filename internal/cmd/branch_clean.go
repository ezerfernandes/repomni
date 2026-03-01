package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/injector"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean [directory]",
	Short: "Remove branch repos in terminal states (merged, closed)",
	Long: `Find branch repos in merged or closed states and optionally delete them.

Before deletion, branch metadata is archived as a JSON file in the parent
directory (.repoinjector-archive.json), injected files are ejected, and the
directory is removed.

If no directory is specified, the current directory is used.

By default, only repos in "merged" or "closed" states are selected. Use
--state to override which states to clean. Repos with uncommitted changes
are skipped unless --force is used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runClean,
}

var (
	cleanDryRun bool
	cleanJSON   bool
	cleanForce  bool
	cleanStates []string
)

func init() {
	branchCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false,
		"show what would be deleted without making changes")
	cleanCmd.Flags().BoolVar(&cleanJSON, "json", false, "output as JSON")
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false,
		"skip confirmation and delete dirty repos")
	cleanCmd.Flags().StringSliceVar(&cleanStates, "state", []string{"merged", "closed"},
		"workflow states to clean (comma-separated)")
}

func runClean(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	repos, err := gitutil.FindGitRepos(target)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no git repositories found under %s", target)
	}

	stateSet := make(map[string]bool)
	for _, s := range cleanStates {
		stateSet[s] = true
	}

	var candidates []ui.CleanCandidate
	for _, repo := range repos {
		info := collectBranchInfo(repo)
		if !stateSet[info.State] {
			continue
		}

		c := ui.CleanCandidate{Info: info}
		c.Size = dirSize(repo)
		c.SizeHuman = formatSize(c.Size)

		if info.Dirty && !cleanForce {
			c.Skipped = true
			c.Reason = "uncommitted changes"
		}

		candidates = append(candidates, c)
	}

	if len(candidates) == 0 {
		fmt.Println("No branches in the selected states found.")
		return nil
	}

	if cleanJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(candidates)
	}

	ui.PrintCleanCandidates(candidates)

	deletable := countDeletable(candidates)
	if deletable == 0 {
		fmt.Println("Nothing to delete (all candidates are skipped).")
		return nil
	}

	if cleanDryRun {
		fmt.Println("Dry run -- no changes were made.")
		return nil
	}

	if !cleanForce {
		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Delete these branch directories?").
					Description(fmt.Sprintf(
						"%d director%s will be permanently removed.",
						deletable, pluralY(deletable))).
					Value(&confirm),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := archiveBranches(target, candidates); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not archive metadata: %v\n", err)
	}

	cfg, cfgErr := config.Load()

	hasErrors := false
	var totalFreed int64
	for _, c := range candidates {
		if c.Skipped {
			continue
		}

		if cfgErr == nil {
			if _, ejectErr := injector.Eject(cfg, c.Info.Path); ejectErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: eject failed for %s: %v\n", c.Info.Name, ejectErr)
			}
		}

		if err := os.RemoveAll(c.Info.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot remove %s: %v\n", c.Info.Name, err)
			hasErrors = true
			continue
		}

		totalFreed += c.Size
		fmt.Printf("  Removed %s (%s)\n", c.Info.Name, c.SizeHuman)
	}

	cleaned := cleanDanglingSymlinks(target)
	if cleaned > 0 {
		fmt.Printf("\nCleaned %d dangling symlink(s).\n", cleaned)
	}

	fmt.Printf("\nFreed %s total.\n", formatSize(totalFreed))

	if hasErrors {
		return fmt.Errorf("some directories could not be removed")
	}
	return nil
}

func countDeletable(candidates []ui.CleanCandidate) int {
	n := 0
	for _, c := range candidates {
		if !c.Skipped {
			n++
		}
	}
	return n
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// dirSize calculates the total size of a directory tree.
func dirSize(path string) int64 {
	var size int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			size += info.Size()
		}
		return nil
	})
	return size
}

// formatSize returns a human-readable size string.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// archiveEntry records metadata about a cleaned branch.
type archiveEntry struct {
	ArchivedAt string        `json:"archived_at"`
	Info       ui.BranchInfo `json:"info"`
}

// archiveBranches appends branch metadata to .repoinjector-archive.json in parentDir.
func archiveBranches(parentDir string, candidates []ui.CleanCandidate) error {
	archivePath := filepath.Join(parentDir, ".repoinjector-archive.json")

	var entries []archiveEntry
	if data, err := os.ReadFile(archivePath); err == nil {
		_ = json.Unmarshal(data, &entries)
	}

	now := time.Now().Format(time.RFC3339)
	for _, c := range candidates {
		if c.Skipped {
			continue
		}
		entries = append(entries, archiveEntry{
			ArchivedAt: now,
			Info:       c.Info,
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(archivePath, data, 0644)
}

// cleanDanglingSymlinks finds and removes broken symlinks in the given directory.
// Returns the count of symlinks removed.
func cleanDanglingSymlinks(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	cleaned := 0
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if _, err := os.Stat(path); err != nil {
				if os.Remove(path) == nil {
					cleaned++
				}
			}
		}
	}
	return cleaned
}

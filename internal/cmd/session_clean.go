package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repoinjector/internal/session"
	"github.com/spf13/cobra"
)

var sessionCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove old or empty session files",
	Long: `Find and remove session files that are empty (0 bytes) or older than
a specified duration. By default, only targets empty sessions.`,
	Args: cobra.NoArgs,
	RunE: runSessionClean,
}

var (
	sessionCleanJSON      bool
	sessionCleanDryRun    bool
	sessionCleanOlderThan string
	sessionCleanEmpty     bool
	sessionCleanForce     bool
)

func init() {
	sessionCmd.AddCommand(sessionCleanCmd)
	sessionCleanCmd.Flags().BoolVar(&sessionCleanJSON, "json", false, "output as JSON")
	sessionCleanCmd.Flags().BoolVar(&sessionCleanDryRun, "dry-run", false,
		"show what would be deleted without making changes")
	sessionCleanCmd.Flags().StringVar(&sessionCleanOlderThan, "older-than", "",
		`delete sessions older than duration (e.g., "30d", "7d")`)
	sessionCleanCmd.Flags().BoolVar(&sessionCleanEmpty, "empty", false,
		"only remove 0-byte session files")
	sessionCleanCmd.Flags().BoolVar(&sessionCleanForce, "force", false,
		"skip confirmation prompt")
}

type cleanCandidate struct {
	SessionID string `json:"session_id"`
	FilePath  string `json:"file_path"`
	SizeBytes int64  `json:"size_bytes"`
	Reason    string `json:"reason"`
}

func runSessionClean(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	dir, err := session.ProjectSessionDir(projectPath)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No session directory found.")
			return nil
		}
		return fmt.Errorf("cannot read session directory: %w", err)
	}

	var olderThanDuration time.Duration
	if sessionCleanOlderThan != "" {
		d, err := parseDayDuration(sessionCleanOlderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than value: %w", err)
		}
		olderThanDuration = d
	}

	var candidates []cleanCandidate
	cutoff := time.Now().Add(-olderThanDuration)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")

		if info.Size() == 0 {
			candidates = append(candidates, cleanCandidate{
				SessionID: sessionID,
				FilePath:  filePath,
				SizeBytes: 0,
				Reason:    "empty (0 bytes)",
			})
			continue
		}

		if sessionCleanEmpty {
			continue // only targeting empty files
		}

		if olderThanDuration > 0 && info.ModTime().Before(cutoff) {
			candidates = append(candidates, cleanCandidate{
				SessionID: sessionID,
				FilePath:  filePath,
				SizeBytes: info.Size(),
				Reason:    fmt.Sprintf("older than %s", sessionCleanOlderThan),
			})
		}
	}

	if len(candidates) == 0 {
		fmt.Println("No sessions match the cleanup criteria.")
		return nil
	}

	if sessionCleanJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(candidates)
	}

	fmt.Printf("\nFound %d session(s) to clean:\n\n", len(candidates))
	for _, c := range candidates {
		sizeStr := "0 B"
		if c.SizeBytes > 0 {
			sizeStr = formatCleanSize(c.SizeBytes)
		}
		fmt.Printf("  %s  %6s  %s\n", c.SessionID[:8], sizeStr, c.Reason)
	}
	fmt.Println()

	if sessionCleanDryRun {
		fmt.Println("Dry run -- no changes were made.")
		return nil
	}

	if !sessionCleanForce {
		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Delete these session files?").
					Description(fmt.Sprintf(
						"%d session file(s) will be permanently removed.",
						len(candidates))).
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

	removed := 0
	for _, c := range candidates {
		if err := os.Remove(c.FilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot remove %s: %v\n", c.SessionID[:8], err)
			continue
		}

		// Also remove corresponding UUID directory if it exists.
		uuidDir := strings.TrimSuffix(c.FilePath, ".jsonl")
		if info, err := os.Stat(uuidDir); err == nil && info.IsDir() {
			_ = os.RemoveAll(uuidDir)
		}

		removed++
	}

	fmt.Printf("Removed %d session file(s).\n", removed)
	return nil
}

func parseDayDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid day count: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func formatCleanSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

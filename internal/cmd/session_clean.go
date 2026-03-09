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
	"github.com/ezerfernandes/repomni/internal/session"
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

	var olderThanDuration time.Duration
	if sessionCleanOlderThan != "" {
		d, err := parseDayDuration(sessionCleanOlderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than value: %w", err)
		}
		olderThanDuration = d
	}

	cutoff := time.Now().Add(-olderThanDuration)
	var candidates []cleanCandidate

	// Collect Claude Code candidates.
	if sessionCLIFilter == "" || sessionCLIFilter == "claude" {
		c := collectClaudeCleanCandidates(projectPath, olderThanDuration, cutoff)
		candidates = append(candidates, c...)
	}

	// Collect Codex candidates.
	if sessionCLIFilter == "" || sessionCLIFilter == "codex" {
		c := collectCodexCleanCandidates(projectPath, olderThanDuration, cutoff)
		candidates = append(candidates, c...)
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

func collectClaudeCleanCandidates(projectPath string, olderThanDuration time.Duration, cutoff time.Time) []cleanCandidate {
	dir, err := session.ProjectSessionDir(projectPath)
	if err != nil {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var candidates []cleanCandidate
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
			continue
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
	return candidates
}

func collectCodexCleanCandidates(projectPath string, olderThanDuration time.Duration, cutoff time.Time) []cleanCandidate {
	dir, err := session.CodexSessionsDir()
	if err != nil {
		return nil
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	var candidates []cleanCandidate

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

		// For Codex, filter by project path first (when possible).
		if projectPath != "" && info.Size() > 0 {
			meta, err := session.ExtractCodexMeta(path)
			if err != nil {
				return nil
			}
			if !session.PathContains(projectPath, meta.ProjectPath) {
				return nil
			}
		}

		if info.Size() == 0 {
			// Can't verify project ownership for empty files, so only
			// include them when we can't filter by project (no projectPath)
			// or when projectPath is empty.
			if projectPath != "" {
				return nil
			}
			candidates = append(candidates, cleanCandidate{
				SessionID: sessionID,
				FilePath:  path,
				SizeBytes: 0,
				Reason:    "empty (0 bytes)",
			})
			return nil
		}

		if sessionCleanEmpty {
			return nil
		}

		if olderThanDuration > 0 && info.ModTime().Before(cutoff) {
			candidates = append(candidates, cleanCandidate{
				SessionID: sessionID,
				FilePath:  path,
				SizeBytes: info.Size(),
				Reason:    fmt.Sprintf("older than %s", sessionCleanOlderThan),
			})
		}
		return nil
	})

	return candidates
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

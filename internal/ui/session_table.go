package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ezerfernandes/repoinjector/internal/session"
)

var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#2e8b57")).Bold(true) // green
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)       // blue
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))                  // gray
	idStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))                  // cyan
	labelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))                  // yellow
)

// PrintSessionsList renders sessions as a vertical list with each field on its own line.
func PrintSessionsList(sessions []session.SessionMeta) {
	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Println()
	for i, s := range sessions {
		fmt.Printf("  %s  %s\n", labelStyle.Render("ID:      "), idStyle.Render(s.SessionID))
		fmt.Printf("  %s  %s\n", labelStyle.Render("Message: "), s.FirstMessage)
		fmt.Printf("  %s  %d\n", labelStyle.Render("Messages:"), s.MessageCount)
		fmt.Printf("  %s  %s\n", labelStyle.Render("Duration:"), formatDuration(s.DurationSecs))
		total := s.Tokens.InputTokens + s.Tokens.OutputTokens
		fmt.Printf("  %s  %s total (%s input / %s output)\n", labelStyle.Render("Tokens:  "),
			formatCount(total),
			formatCount(s.Tokens.InputTokens),
			formatCount(s.Tokens.OutputTokens))
		fmt.Printf("  %s  %s\n", labelStyle.Render("Modified:"), formatTimeAgo(s.ModifiedAt))

		if i < len(sessions)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

// PrintSessionMessages renders color-coded messages from a session.
func PrintSessionMessages(meta session.SessionMeta, messages []session.Message, full bool) {
	fmt.Println()
	fmt.Printf("  Session: %s\n", idStyle.Render(meta.SessionID))
	fmt.Printf("  Project: %s\n", meta.ProjectPath)
	fmt.Printf("  Created: %s   Messages: %d   Duration: %s\n",
		meta.CreatedAt.Format(time.RFC3339),
		meta.MessageCount,
		formatDuration(meta.DurationSecs))
	fmt.Println()

	for _, msg := range messages {
		var label string
		switch msg.Type {
		case "user":
			label = userStyle.Render("User")
		case "assistant":
			label = assistantStyle.Render("Assistant")
		default:
			label = msg.Type
		}

		ts := dimStyle.Render(msg.Timestamp.Format("15:04:05"))
		fmt.Printf("  %s %s\n", label, ts)

		lines := strings.Split(msg.Content, "\n")
		for _, line := range lines {
			fmt.Printf("    %s\n", line)
		}
		fmt.Println()
	}
}

// PrintSessionStats renders aggregate statistics.
func PrintSessionStats(stats session.Stats) {
	fmt.Println()
	fmt.Printf("  Sessions:  %d\n", stats.TotalSessions)
	fmt.Printf("  Messages:  %d\n", stats.TotalMessages)
	fmt.Printf("  Duration:  %s\n", formatDuration(stats.TotalDurationSecs))
	fmt.Printf("  Tokens:    %s input / %s output\n",
		formatCount(stats.TotalTokens.InputTokens),
		formatCount(stats.TotalTokens.OutputTokens))
	if stats.TotalTokens.CacheReadTokens > 0 {
		fmt.Printf("  Cache:     %s read / %s created\n",
			formatCount(stats.TotalTokens.CacheReadTokens),
			formatCount(stats.TotalTokens.CacheCreationTokens))
	}
	fmt.Printf("  Size:      %s\n", formatSize(stats.TotalSizeBytes))
	if !stats.OldestSession.IsZero() {
		fmt.Printf("  Oldest:    %s\n", stats.OldestSession.Format("2006-01-02"))
	}
	if !stats.NewestSession.IsZero() {
		fmt.Printf("  Newest:    %s\n", stats.NewestSession.Format("2006-01-02"))
	}
	fmt.Println()
}

// PrintSearchResults renders search matches.
func PrintSearchResults(results []session.SearchResult) {
	if len(results) == 0 {
		fmt.Println("No matches found.")
		return
	}

	fmt.Println()
	for _, r := range results {
		id := idStyle.Render(truncateStr(r.Meta.SessionID, 8))
		preview := truncateStr(r.Meta.FirstMessage, 50)
		fmt.Printf("  %s  %s  (%d matches)\n", id, preview, len(r.Matches))

		for _, m := range r.Matches {
			typeLabel := dimStyle.Render(m.Type)
			fmt.Printf("    [%s] %s\n", typeLabel, m.Preview)
		}
		fmt.Println()
	}
}

func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(secs float64) string {
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	d := time.Duration(secs) * time.Second
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatTokens(t session.TokenUsage) string {
	total := t.InputTokens + t.OutputTokens
	return formatCount(total)
}

func formatCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

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

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

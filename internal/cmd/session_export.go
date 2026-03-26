package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ezerfernandes/repomni/internal/session"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var sessionExportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a session as markdown",
	Long: `Export a Claude Code session conversation as a markdown document.
Output goes to stdout by default, or to a file with --output.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionExport,
}

var (
	sessionExportOutput  string
	sessionExportFull    bool
	sessionExportNoTools bool
	sessionExportJSON    bool
)

func init() {
	sessionCmd.AddCommand(sessionExportCmd)
	sessionExportCmd.Flags().StringVar(&sessionExportOutput, "output", "",
		"output file path (default: stdout)")
	sessionExportCmd.Flags().BoolVar(&sessionExportFull, "full", false,
		"include full tool_use/tool_result blocks")
	sessionExportCmd.Flags().BoolVar(&sessionExportNoTools, "no-tools", false,
		"omit messages that only contain tool calls")
	sessionExportCmd.Flags().BoolVar(&sessionExportJSON, "json", false, "output as JSON")
}

func runSessionExport(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	meta, err := session.FindSessionAll(projectPath, args[0], sessionCLIFilter)
	if err != nil {
		return err
	}

	full := sessionExportFull && !sessionExportNoTools
	messages, err := session.ReadMessagesForSession(meta, 0, 0, full)
	if err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Session %s\n\n", meta.SessionID)
	fmt.Fprintf(&b, "**Project:** %s\n\n", meta.ProjectPath)
	fmt.Fprintf(&b, "**Created:** %s\n\n", meta.CreatedAt.Format(time.RFC3339))
	b.WriteString(fmt.Sprintf("**Duration:** %s\n\n", formatExportDuration(meta.DurationSecs)))
	b.WriteString(fmt.Sprintf("**Messages:** %d\n\n", meta.MessageCount))
	b.WriteString("---\n\n")

	for _, msg := range messages {
		if sessionExportNoTools && msg.Type == "assistant" && isToolOnly(msg.Content) {
			continue
		}

		content := msg.Content
		if sessionExportNoTools {
			content = stripToolLines(content)
		}

		switch msg.Type {
		case "user":
			b.WriteString("### User\n\n")
			for _, line := range strings.Split(content, "\n") {
				b.WriteString("> " + line + "\n")
			}
		case "assistant":
			b.WriteString("### Assistant\n\n")
			b.WriteString(content + "\n")
		}
		b.WriteString("\n---\n\n")
	}

	output := b.String()

	if sessionExportJSON {
		outPath := sessionExportOutput
		if outPath != "" {
			if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
				return fmt.Errorf("cannot write to %s: %w", outPath, err)
			}
		}
		return ui.PrintJSON(struct {
			SessionID    string  `json:"session_id"`
			ProjectPath  string  `json:"project_path"`
			FilePath     string  `json:"file_path"`
			CreatedAt    string  `json:"created_at"`
			DurationSecs float64 `json:"duration_seconds"`
			MessageCount int     `json:"message_count"`
			SizeBytes    int64   `json:"size_bytes"`
			OutputPath   string  `json:"output_path,omitempty"`
			OutputBytes  int     `json:"output_bytes"`
		}{
			SessionID:    meta.SessionID,
			ProjectPath:  meta.ProjectPath,
			FilePath:     meta.FilePath,
			CreatedAt:    meta.CreatedAt.Format(time.RFC3339),
			DurationSecs: meta.DurationSecs,
			MessageCount: meta.MessageCount,
			SizeBytes:    meta.SizeBytes,
			OutputPath:   outPath,
			OutputBytes:  len(output),
		})
	}

	if sessionExportOutput != "" {
		if err := os.WriteFile(sessionExportOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("cannot write to %s: %w", sessionExportOutput, err)
		}
		fmt.Printf("Exported to %s\n", sessionExportOutput)
		return nil
	}

	fmt.Print(output)
	return nil
}

// isToolOnly returns true if the content consists entirely of [tool: ...] lines.
func isToolOnly(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[tool:") && !strings.HasPrefix(line, "[result]") {
			return false
		}
	}
	return true
}

// stripToolLines removes [tool: ...] and [result] lines from content,
// returning the remaining text. If only whitespace remains, returns "".
func stripToolLines(content string) string {
	var kept []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[tool:") || strings.HasPrefix(trimmed, "[result]") {
			continue
		}
		kept = append(kept, line)
	}
	result := strings.TrimSpace(strings.Join(kept, "\n"))
	return result
}

func formatExportDuration(secs float64) string {
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

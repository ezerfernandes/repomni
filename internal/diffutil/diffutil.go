package diffutil

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pmezard/go-difflib/difflib"
)

var (
	addedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#2e8b57"))
	removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#b22222"))
	hunkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	headerStyle  = lipgloss.NewStyle().Bold(true)
)

// UnifiedDiff computes a unified diff between oldText and newText.
// Returns an empty string if the texts are identical.
func UnifiedDiff(oldLabel, newLabel, oldText, newText string) string {
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldText),
		B:        difflib.SplitLines(newText),
		FromFile: oldLabel,
		ToFile:   newLabel,
		Context:  3,
	})
	return diff
}

// ColorDiff applies ANSI colors to a unified diff string.
func ColorDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			b.WriteString(headerStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(addedStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(removedStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(hunkStyle.Render(line))
		default:
			b.WriteString(line)
		}
	}
	return b.String()
}

// SummaryLine returns a one-line summary comparing oldText and newText.
func SummaryLine(oldText, newText string) string {
	if oldText == newText {
		return "Outputs are identical"
	}

	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:       difflib.SplitLines(oldText),
		B:       difflib.SplitLines(newText),
		Context: 0,
	})

	var added, removed int
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// skip headers
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		}
	}

	return fmt.Sprintf("Outputs differ (%d lines added, %d lines removed)", added, removed)
}

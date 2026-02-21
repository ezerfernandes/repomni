package ui

import "github.com/charmbracelet/lipgloss"

// StateStyle returns a lipgloss.Style for the given workflow state.
// Unknown states return an unstyled base.
func StateStyle(state string) lipgloss.Style {
	base := lipgloss.NewStyle()
	switch state {
	case "active":
		return base.Foreground(lipgloss.Color("#2e8b57")) // dark green
	case "review":
		return base.Foreground(lipgloss.Color("3")) // yellow
	case "approved":
		return base.Foreground(lipgloss.Color("#32cd32")) // lime green
	case "review-blocked":
		return base.Foreground(lipgloss.Color("1")) // red
	case "merged":
		return base.Foreground(lipgloss.Color("5")) // magenta/purple
	case "closed":
		return base.Foreground(lipgloss.Color("#b22222")) // dim red
	case "done":
		return base.Foreground(lipgloss.Color("8")) // gray
	case "paused":
		return base.Foreground(lipgloss.Color("4")) // blue
	default:
		return base
	}
}

// RenderState returns the state string styled for terminal output.
// Empty state renders as a dim placeholder.
func RenderState(state string) string {
	if state == "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("--")
	}
	return StateStyle(state).Render(state)
}

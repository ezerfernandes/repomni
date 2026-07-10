package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BranchInfo holds the collected information about one branch repo.
type BranchInfo struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Branch      string `json:"branch"`
	State       string `json:"state"`
	MergeURL    string `json:"merge_url,omitempty"`
	Ticket      string `json:"ticket,omitempty"`
	Description string `json:"description,omitempty"`
	LastCommit  string `json:"last_commit,omitempty"`
	Dirty       bool   `json:"dirty"`
	Remote      bool   `json:"remote"`
}

// PrintBranchesTable renders a colored table of branch repos.
func PrintBranchesTable(infos []BranchInfo) {
	nameW := len("Name")
	stateW := len("State")
	ticketW := 0
	hasCommit := false
	hasDiffers := false
	hasRemote := false
	hasTicket := false
	for _, info := range infos {
		if info.LastCommit != "" {
			hasCommit = true
		}
		display := info.Name
		if info.Remote {
			display += "*"
			hasRemote = true
		}
		if info.Branch != "" && info.Name != info.Branch {
			hasDiffers = true
		}
		if len(display) > nameW {
			nameW = len(display)
		}
		stateDisplay := info.State
		if stateDisplay == "" {
			stateDisplay = "--"
		}
		if len(stateDisplay) > stateW {
			stateW = len(stateDisplay)
		}
		if info.Ticket != "" {
			hasTicket = true
			if len(info.Ticket) > ticketW {
				ticketW = len(info.Ticket)
			}
		}
	}
	if hasTicket && ticketW < len("Ticket") {
		ticketW = len("Ticket")
	}

	fmt.Println()
	commitSuffix := ""
	commitSepSuffix := ""
	if hasCommit {
		commitSuffix = "  Last Commit"
		commitSepSuffix = "  " + strings.Repeat("─", 11)
	}
	if hasTicket {
		hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%-5s", nameW, stateW, ticketW)
		fmt.Printf(hdrFmt+commitSuffix+"\n", "Name", "State", "Ticket", "Dirty")
		fmt.Printf(hdrFmt+commitSepSuffix+"\n",
			strings.Repeat("─", nameW),
			strings.Repeat("─", stateW),
			strings.Repeat("─", ticketW),
			strings.Repeat("─", 5))
	} else {
		hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-5s", nameW, stateW)
		fmt.Printf(hdrFmt+commitSuffix+"\n", "Name", "State", "Dirty")
		fmt.Printf(hdrFmt+commitSepSuffix+"\n",
			strings.Repeat("─", nameW),
			strings.Repeat("─", stateW),
			strings.Repeat("─", 5))
	}

	for _, info := range infos {
		dirty := " "
		if info.Dirty {
			dirty = "x"
		}

		display := info.Name
		if info.Remote {
			display += "*"
		}

		stateDisplay := RenderState(info.State)

		rawState := info.State
		if rawState == "" {
			rawState = "--"
		}
		pad := stateW - len(rawState)
		if pad < 0 {
			pad = 0
		}

		commitCol := ""
		if hasCommit {
			commitCol = "  " + info.LastCommit
		}

		if hasTicket {
			fmt.Printf("  %-*s  %s%s  %-*s  %-5s%s\n",
				nameW, display,
				stateDisplay, strings.Repeat(" ", pad),
				ticketW, info.Ticket,
				dirty, commitCol)
		} else {
			fmt.Printf("  %-*s  %s%s  %-5s%s\n",
				nameW, display,
				stateDisplay, strings.Repeat(" ", pad),
				dirty, commitCol)
		}
	}

	if hasRemote || hasDiffers {
		fmt.Println()
		if hasRemote {
			fmt.Println("  * Cloned from an existing remote branch")
		}
		if hasDiffers {
			fmt.Println("  * Name and Branch differs")
		}
	}
	fmt.Println()
}

// branchLabelStyle is used for field labels in the detailed list view.
var branchLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow

// PrintBranchesList renders branch repos as a vertical list with each field on its own line.
func PrintBranchesList(infos []BranchInfo) {
	if len(infos) == 0 {
		fmt.Println("No branches found.")
		return
	}

	fmt.Println()
	for i, info := range infos {
		name := info.Name
		if info.Remote {
			name += "*"
		}
		fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Name:       "), name)
		fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Branch:     "), info.Branch)
		fmt.Printf("  %s  %s\n", branchLabelStyle.Render("State:      "), RenderState(info.State))
		if info.Ticket != "" {
			fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Ticket:     "), info.Ticket)
		}
		if info.MergeURL != "" {
			fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Merge URL:  "), info.MergeURL)
		}
		if info.Description != "" {
			fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Description:"), info.Description)
		}
		if info.LastCommit != "" {
			fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Last Commit:"), info.LastCommit)
		}
		dirty := "no"
		if info.Dirty {
			dirty = "yes"
		}
		fmt.Printf("  %s  %s\n", branchLabelStyle.Render("Dirty:      "), dirty)

		if i < len(infos)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

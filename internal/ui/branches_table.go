package ui

import (
	"fmt"
	"strings"
)

// BranchInfo holds the collected information about one branch repo.
type BranchInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
	State    string `json:"state"`
	MergeURL string `json:"merge_url,omitempty"`
	Ticket   string `json:"ticket,omitempty"`
	LastCommit string `json:"last_commit,omitempty"`
	Dirty      bool   `json:"dirty"`
	Remote     bool   `json:"remote"`
}

// PrintBranchesTable renders a colored table of branch repos.
func PrintBranchesTable(infos []BranchInfo) {
	nameW := len("Name")
	stateW := len("State")
	ticketW := 0
	commitW := 0
	hasDiffers := false
	hasRemote := false
	hasTicket := false
	hasCommit := false
	for _, info := range infos {
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
		if info.LastCommit != "" {
			hasCommit = true
			if len(info.LastCommit) > commitW {
				commitW = len(info.LastCommit)
			}
		}
	}
	if hasTicket && ticketW < len("Ticket") {
		ticketW = len("Ticket")
	}
	if hasCommit && commitW < len("Last Commit") {
		commitW = len("Last Commit")
	}
	// Cap commit column width to keep the table readable.
	const maxCommitW = 50
	if commitW > maxCommitW {
		commitW = maxCommitW
	}

	fmt.Println()
	printHeader(nameW, stateW, ticketW, commitW, hasTicket, hasCommit)

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

		commit := info.LastCommit
		if len(commit) > commitW && commitW > 0 {
			commit = commit[:commitW-1] + "…"
		}

		parts := fmt.Sprintf("  %-*s  %s%s", nameW, display, stateDisplay, strings.Repeat(" ", pad))
		if hasTicket {
			parts += fmt.Sprintf("  %-*s", ticketW, info.Ticket)
		}
		parts += fmt.Sprintf("  %s", dirty)
		if hasCommit {
			parts += fmt.Sprintf("  %s", commit)
		}
		fmt.Println(parts)
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

func printHeader(nameW, stateW, ticketW, commitW int, hasTicket, hasCommit bool) {
	hdr := fmt.Sprintf("  %-*s  %-*s", nameW, "Name", stateW, "State")
	sep := fmt.Sprintf("  %s  %s", strings.Repeat("─", nameW), strings.Repeat("─", stateW))
	if hasTicket {
		hdr += fmt.Sprintf("  %-*s", ticketW, "Ticket")
		sep += fmt.Sprintf("  %s", strings.Repeat("─", ticketW))
	}
	hdr += fmt.Sprintf("  %s", "Dirty")
	sep += fmt.Sprintf("  %s", strings.Repeat("─", 5))
	if hasCommit {
		hdr += fmt.Sprintf("  %s", "Last Commit")
		sep += fmt.Sprintf("  %s", strings.Repeat("─", commitW))
	}
	fmt.Println(hdr)
	fmt.Println(sep)
}

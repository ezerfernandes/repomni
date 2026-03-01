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
	Dirty    bool   `json:"dirty"`
	Remote   bool   `json:"remote"`
}

// PrintBranchesTable renders a colored table of branch repos.
func PrintBranchesTable(infos []BranchInfo) {
	nameW := len("Name")
	stateW := len("State")
	ticketW := 0
	hasDiffers := false
	hasRemote := false
	hasTicket := false
	for _, info := range infos {
		display := info.Name
		if info.Remote {
			display += "*"
			hasRemote = true
		}
		if info.Name != info.Branch {
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
	if hasTicket {
		hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, stateW, ticketW)
		fmt.Printf(hdrFmt, "Name", "State", "Ticket", "Dirty")
		fmt.Printf(hdrFmt,
			strings.Repeat("─", nameW),
			strings.Repeat("─", stateW),
			strings.Repeat("─", ticketW),
			strings.Repeat("─", 5))
	} else {
		hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", nameW, stateW)
		fmt.Printf(hdrFmt, "Name", "State", "Dirty")
		fmt.Printf(hdrFmt,
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

		if hasTicket {
			fmt.Printf("  %-*s  %s%s  %-*s  %s\n",
				nameW, display,
				stateDisplay, strings.Repeat(" ", pad),
				ticketW, info.Ticket,
				dirty)
		} else {
			fmt.Printf("  %-*s  %s%s  %s\n",
				nameW, display,
				stateDisplay, strings.Repeat(" ", pad),
				dirty)
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

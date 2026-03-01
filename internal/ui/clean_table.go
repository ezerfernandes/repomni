package ui

import (
	"fmt"
	"strings"
)

// CleanCandidate holds information about a branch that is a candidate for cleaning.
type CleanCandidate struct {
	Info      BranchInfo `json:"info"`
	Size      int64      `json:"size"`
	SizeHuman string     `json:"size_human"`
	Skipped   bool       `json:"skipped"`
	Reason    string     `json:"reason,omitempty"`
}

// PrintCleanCandidates renders a table of branches that will be cleaned.
func PrintCleanCandidates(candidates []CleanCandidate) {
	nameW := len("Name")
	stateW := len("State")
	sizeW := len("Size")

	for _, c := range candidates {
		if len(c.Info.Name) > nameW {
			nameW = len(c.Info.Name)
		}
		stateDisplay := c.Info.State
		if stateDisplay == "" {
			stateDisplay = "--"
		}
		if len(stateDisplay) > stateW {
			stateW = len(stateDisplay)
		}
		if len(c.SizeHuman) > sizeW {
			sizeW = len(c.SizeHuman)
		}
	}

	fmt.Println()
	hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%s\n", nameW, stateW, sizeW)
	fmt.Printf(hdrFmt, "Name", "State", "Size", "Status")
	fmt.Printf(hdrFmt,
		strings.Repeat("─", nameW),
		strings.Repeat("─", stateW),
		strings.Repeat("─", sizeW),
		strings.Repeat("─", 10))

	deletable := 0
	for _, c := range candidates {
		stateDisplay := RenderState(c.Info.State)
		rawState := c.Info.State
		if rawState == "" {
			rawState = "--"
		}
		pad := stateW - len(rawState)
		if pad < 0 {
			pad = 0
		}

		status := "will delete"
		if c.Skipped {
			status = "skip: " + c.Reason
		} else {
			deletable++
		}

		fmt.Printf("  %-*s  %s%s  %-*s  %s\n",
			nameW, c.Info.Name,
			stateDisplay, strings.Repeat(" ", pad),
			sizeW, c.SizeHuman,
			status)
	}

	fmt.Printf("\n  %d of %d branch(es) will be deleted.\n\n", deletable, len(candidates))
}

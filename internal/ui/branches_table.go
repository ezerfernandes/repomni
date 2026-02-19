package ui

import (
	"fmt"
	"strings"
)

// BranchInfo holds the collected information about one branch repo.
type BranchInfo struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Branch string `json:"branch"`
	State  string `json:"state"`
	Dirty  bool   `json:"dirty"`
}

// PrintBranchesTable renders a colored table of branch repos.
func PrintBranchesTable(infos []BranchInfo) {
	nameW := len("Name")
	stateW := len("State")
	hasDiffers := false
	for _, info := range infos {
		display := info.Name
		if info.Name != info.Branch {
			display = info.Name + "*"
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
	}

	fmt.Println()
	hdrFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", nameW, stateW)
	fmt.Printf(hdrFmt, "Name", "State", "Dirty")
	fmt.Printf(hdrFmt,
		strings.Repeat("─", nameW),
		strings.Repeat("─", stateW),
		strings.Repeat("─", 5))

	for _, info := range infos {
		dirty := " "
		if info.Dirty {
			dirty = "*"
		}

		display := info.Name
		if info.Name != info.Branch {
			display = info.Name + "*"
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

		fmt.Printf("  %-*s  %s%s  %s\n",
			nameW, display,
			stateDisplay, strings.Repeat(" ", pad),
			dirty)
	}

	if hasDiffers {
		fmt.Println()
		fmt.Println("  * Name and Branch differs")
	}
	fmt.Println()
}

package ui

import (
	"strings"
	"testing"
)

func TestPrintBranchesTable_Basic(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat-a", Name: "feat-a", Branch: "feat-a", State: "active", Dirty: false},
		{Path: "/repos/feat-b", Name: "feat-b", Branch: "feat-b", State: "review", Dirty: true},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "feat-a") {
		t.Error("expected feat-a in output")
	}
	if !strings.Contains(output, "feat-b") {
		t.Error("expected feat-b in output")
	}
	if !strings.Contains(output, "Name") {
		t.Error("expected header row with 'Name'")
	}
	if !strings.Contains(output, "State") {
		t.Error("expected header row with 'State'")
	}
}

func TestPrintBranchesTable_EmptyState(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat-a", Name: "feat-a", Branch: "feat-a", State: ""},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	// Empty state should render as "--"
	if !strings.Contains(output, "--") {
		t.Error("expected '--' placeholder for empty state")
	}
}

func TestPrintBranchesTable_RemoteBranch(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat-a", Name: "feat-a", Branch: "feat-a", Remote: true},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "feat-a*") {
		t.Error("expected asterisk for remote branch")
	}
	if !strings.Contains(output, "Cloned from an existing remote branch") {
		t.Error("expected remote branch footnote")
	}
}

func TestPrintBranchesTable_DifferentNameAndBranch(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat-a", Name: "feat-a", Branch: "different-branch"},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "Name and Branch differs") {
		t.Error("expected name/branch differs footnote")
	}
}

func TestPrintBranchesTable_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		PrintBranchesTable(nil)
	})

	// Should still print header
	if !strings.Contains(output, "Name") {
		t.Error("expected header even with empty list")
	}
}

func TestPrintBranchesTable_DirtyFlag(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/clean", Name: "clean", Branch: "clean", State: "active", Dirty: false},
		{Path: "/repos/dirty", Name: "dirty", Branch: "dirty", State: "active", Dirty: true},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "Dirty") {
		t.Error("expected Dirty column header")
	}

	lines := strings.Split(output, "\n")
	// Find the data rows and check dirty flags
	var cleanLine, dirtyLine string
	for _, line := range lines {
		if strings.Contains(line, "clean") && !strings.Contains(line, "Name") {
			cleanLine = line
		}
		if strings.Contains(line, "dirty") && !strings.Contains(line, "Name") && !strings.Contains(line, "Dirty") {
			dirtyLine = line
		}
	}

	// Clean repo should end with space (not "x")
	if cleanLine == "" {
		t.Fatal("expected line containing 'clean'")
	}
	if strings.HasSuffix(strings.TrimRight(cleanLine, " \t"), "x") {
		t.Error("clean repo should not show dirty flag 'x'")
	}

	// Dirty repo should have "x" dirty marker
	if dirtyLine == "" {
		t.Fatal("expected line containing 'dirty'")
	}
	if !strings.Contains(dirtyLine, "x") {
		t.Error("dirty repo should show 'x' dirty flag")
	}
}

func TestPrintBranchesTable_BothFootnotes(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat-a", Name: "feat-a", Branch: "feat-a", Remote: true},
		{Path: "/repos/feat-b", Name: "feat-b", Branch: "different-branch"},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "Cloned from an existing remote branch") {
		t.Error("expected remote footnote")
	}
	if !strings.Contains(output, "Name and Branch differs") {
		t.Error("expected name/branch differs footnote")
	}
}

func TestPrintBranchesTable_NoFootnotes(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat-a", Name: "feat-a", Branch: "feat-a", State: "active"},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if strings.Contains(output, "Cloned from an existing remote branch") {
		t.Error("should not have remote footnote")
	}
	if strings.Contains(output, "Name and Branch differs") {
		t.Error("should not have name/branch differs footnote")
	}
}

func TestPrintBranchesTable_LongNames(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/very-long-feature-branch-name", Name: "very-long-feature-branch-name", Branch: "very-long-feature-branch-name", State: "review-blocked"},
		{Path: "/repos/x", Name: "x", Branch: "x", State: "active"},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "very-long-feature-branch-name") {
		t.Error("expected long name in output")
	}
	if !strings.Contains(output, "x") {
		t.Error("expected short name in output")
	}
}

func TestPrintBranchesTable_AllStates(t *testing.T) {
	states := []string{"active", "review", "approved", "review-blocked", "merged", "closed", "done", "paused"}
	var infos []BranchInfo
	for _, s := range states {
		infos = append(infos, BranchInfo{
			Path: "/repos/" + s, Name: s, Branch: s, State: s,
		})
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	// All names should appear
	for _, s := range states {
		if !strings.Contains(output, s) {
			t.Errorf("expected state name %q in output", s)
		}
	}
}

func TestPrintBranchesTable_WithMergeURL(t *testing.T) {
	infos := []BranchInfo{
		{
			Path:     "/repos/feat-a",
			Name:     "feat-a",
			Branch:   "feat-a",
			State:    "review",
			MergeURL: "https://github.com/org/repo/pull/42",
		},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	// The table should still render even though MergeURL is set
	// (MergeURL is a data field, not displayed in table)
	if !strings.Contains(output, "feat-a") {
		t.Error("expected feat-a in output")
	}
}

func TestPrintBranchesTable_MultipleRemotes(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/a", Name: "a", Branch: "a", Remote: true},
		{Path: "/repos/b", Name: "b", Branch: "b", Remote: true},
		{Path: "/repos/c", Name: "c", Branch: "c", Remote: false},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "a*") {
		t.Error("expected a* in output")
	}
	if !strings.Contains(output, "b*") {
		t.Error("expected b* in output")
	}
	// c should not have asterisk
	if strings.Contains(output, "c*") {
		t.Error("c should not have asterisk")
	}
}

func TestPrintBranchesTable_CustomState(t *testing.T) {
	infos := []BranchInfo{
		{Path: "/repos/feat", Name: "feat", Branch: "feat", State: "custom-state"},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "custom-state") {
		t.Error("expected custom-state in output")
	}
}

func TestPrintBranchesTable_RemoteExpandsColumn(t *testing.T) {
	// The asterisk suffix for remote should be factored into column width
	infos := []BranchInfo{
		{Path: "/repos/abcd", Name: "abcd", Branch: "abcd", Remote: true},
	}

	output := captureStdout(t, func() {
		PrintBranchesTable(infos)
	})

	if !strings.Contains(output, "abcd*") {
		t.Error("expected 'abcd*' in output")
	}
}

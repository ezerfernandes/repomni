package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/injector"
)

// captureStdout captures stdout output from a function.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("cannot create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestActionIcon(t *testing.T) {
	tests := []struct {
		action   string
		expected string
	}{
		{"created", "[ok]"},
		{"updated", "[ok]"},
		{"removed", "[ok]"},
		{"skipped", "[--]"},
		{"dry-run", "[--]"},
		{"warning", "[!!]"},
		{"error", "[!!]"},
		{"unknown", "[??]"},
		{"", "[??]"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := actionIcon(tt.action)
			if got != tt.expected {
				t.Errorf("actionIcon(%q) = %q, want %q", tt.action, got, tt.expected)
			}
		})
	}
}

func TestBoolIcon(t *testing.T) {
	if got := boolIcon(true); got != "Yes" {
		t.Errorf("boolIcon(true) = %q, want \"Yes\"", got)
	}
	if got := boolIcon(false); got != "No" {
		t.Errorf("boolIcon(false) = %q, want \"No\"", got)
	}
}

func TestPrintResults(t *testing.T) {
	results := []injector.Result{
		{Action: "created", Detail: "symlinked", Item: config.Item{TargetPath: ".envrc"}},
		{Action: "skipped", Detail: "already up to date", Item: config.Item{TargetPath: ".env"}},
		{Action: "error", Detail: "cannot copy", Item: config.Item{TargetPath: "hooks.json"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if !strings.Contains(output, "1 changed") {
		t.Errorf("expected summary with '1 changed', got: %s", output)
	}
	if !strings.Contains(output, "1 skipped") {
		t.Errorf("expected summary with '1 skipped', got: %s", output)
	}
	if !strings.Contains(output, "1 errors") {
		t.Errorf("expected summary with '1 errors', got: %s", output)
	}
	if !strings.Contains(output, ".envrc") {
		t.Errorf("expected output to contain '.envrc', got: %s", output)
	}
}

func TestPrintResults_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		PrintResults(nil)
	})

	if !strings.Contains(output, "0 changed") {
		t.Errorf("expected summary with '0 changed', got: %s", output)
	}
}

func TestPrintResults_WithWarnings(t *testing.T) {
	results := []injector.Result{
		{Action: "warning", Detail: "file exists", Item: config.Item{TargetPath: "test.md"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if !strings.Contains(output, "1 warnings") {
		t.Errorf("expected summary with '1 warnings', got: %s", output)
	}
}

func TestPrintStatusTable(t *testing.T) {
	statuses := []injector.ItemStatus{
		{
			Item:     config.Item{TargetPath: ".envrc"},
			Present:  true,
			Current:  true,
			Excluded: true,
		},
		{
			Item:     config.Item{TargetPath: ".env"},
			Present:  false,
			Current:  false,
			Excluded: false,
		},
	}

	output := captureStdout(t, func() {
		PrintStatusTable("/path/to/repo", statuses)
	})

	if !strings.Contains(output, "Repository: /path/to/repo") {
		t.Error("expected repository path in output")
	}
	if !strings.Contains(output, ".envrc") {
		t.Error("expected .envrc in output")
	}
	if !strings.Contains(output, ".env") {
		t.Error("expected .env in output")
	}
	if !strings.Contains(output, "Present") {
		t.Error("expected header row with 'Present'")
	}
}

func TestPrintStatusTable_NotPresentShowsDash(t *testing.T) {
	statuses := []injector.ItemStatus{
		{
			Item:    config.Item{TargetPath: ".envrc"},
			Present: false,
			Current: false,
		},
	}

	output := captureStdout(t, func() {
		PrintStatusTable("/repo", statuses)
	})

	// When not present, Current column should show "-"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, ".envrc") {
			if !strings.Contains(line, "-") {
				t.Error("expected '-' for current when item is not present")
			}
			break
		}
	}
}

func TestPrintStatusTable_PresentNotCurrentShowsNo(t *testing.T) {
	statuses := []injector.ItemStatus{
		{
			Item:    config.Item{TargetPath: ".envrc"},
			Present: true,
			Current: false,
		},
	}

	output := captureStdout(t, func() {
		PrintStatusTable("/repo", statuses)
	})

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, ".envrc") {
			// Should have "Yes" for present and "No" for current
			if !strings.Contains(line, "Yes") {
				t.Error("expected 'Yes' for present")
			}
			if !strings.Contains(line, "No") {
				t.Error("expected 'No' for not-current")
			}
			break
		}
	}
}

func TestPrintStatusTable_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		PrintStatusTable("/empty/repo", nil)
	})

	if !strings.Contains(output, "Repository: /empty/repo") {
		t.Error("expected repository path in output")
	}
	if !strings.Contains(output, "Present") {
		t.Error("expected header row even with empty statuses")
	}
}

func TestPrintResults_AllActions(t *testing.T) {
	results := []injector.Result{
		{Action: "created", Detail: "symlinked", Item: config.Item{TargetPath: "a"}},
		{Action: "updated", Detail: "refreshed", Item: config.Item{TargetPath: "b"}},
		{Action: "removed", Detail: "deleted symlink", Item: config.Item{TargetPath: "c"}},
		{Action: "skipped", Detail: "already up to date", Item: config.Item{TargetPath: "d"}},
		{Action: "dry-run", Detail: "would create", Item: config.Item{TargetPath: "e"}},
		{Action: "warning", Detail: "file exists", Item: config.Item{TargetPath: "f"}},
		{Action: "error", Detail: "permission denied", Item: config.Item{TargetPath: "g"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if !strings.Contains(output, "3 changed") {
		t.Errorf("expected '3 changed' (created+updated+removed), got: %s", output)
	}
	if !strings.Contains(output, "2 skipped") {
		t.Errorf("expected '2 skipped' (skipped+dry-run), got: %s", output)
	}
	if !strings.Contains(output, "1 warnings") {
		t.Errorf("expected '1 warnings', got: %s", output)
	}
	if !strings.Contains(output, "1 errors") {
		t.Errorf("expected '1 errors', got: %s", output)
	}
	// Check icons appear
	if !strings.Contains(output, "[ok]") {
		t.Error("expected [ok] icon in output")
	}
	if !strings.Contains(output, "[--]") {
		t.Error("expected [--] icon in output")
	}
	if !strings.Contains(output, "[!!]") {
		t.Error("expected [!!] icon in output")
	}
}

func TestPrintResults_DetailText(t *testing.T) {
	results := []injector.Result{
		{Action: "created", Detail: "symlinked from source", Item: config.Item{TargetPath: ".envrc"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if !strings.Contains(output, "symlinked from source") {
		t.Error("expected detail text in output")
	}
	if !strings.Contains(output, ".envrc") {
		t.Error("expected target path in output")
	}
}

func TestPrintStatusTable_AllPresentAndCurrent(t *testing.T) {
	statuses := []injector.ItemStatus{
		{Item: config.Item{TargetPath: ".envrc"}, Present: true, Current: true, Excluded: true, Detail: "symlink ok"},
		{Item: config.Item{TargetPath: ".env"}, Present: true, Current: true, Excluded: true, Detail: "symlink ok"},
		{Item: config.Item{TargetPath: ".claude/hooks.json"}, Present: true, Current: true, Excluded: true, Detail: "symlink ok"},
	}

	output := captureStdout(t, func() {
		PrintStatusTable("/repo", statuses)
	})

	// All items should show "Yes" for Present and Current
	lines := strings.Split(output, "\n")
	dataLines := 0
	for _, line := range lines {
		if strings.Contains(line, ".envrc") || strings.Contains(line, ".env") || strings.Contains(line, "hooks.json") {
			dataLines++
			yesCount := strings.Count(line, "Yes")
			if yesCount < 2 {
				t.Errorf("expected at least 2 'Yes' in line %q (Present + Current), got %d", line, yesCount)
			}
		}
	}
	if dataLines < 3 {
		t.Errorf("expected 3 data lines, found %d", dataLines)
	}
}

func TestPrintStatusTable_PresentCurrentExcludedColumns(t *testing.T) {
	statuses := []injector.ItemStatus{
		{
			Item:     config.Item{TargetPath: ".envrc"},
			Present:  true,
			Current:  false,
			Excluded: true,
			Detail:   "symlink points to wrong path",
		},
	}

	output := captureStdout(t, func() {
		PrintStatusTable("/repo", statuses)
	})

	// The table shows Present=Yes, Current=No, Excluded=Yes
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, ".envrc") {
			if !strings.Contains(line, "Yes") {
				t.Error("expected 'Yes' for present column")
			}
			if !strings.Contains(line, "No") {
				t.Error("expected 'No' for current column")
			}
			break
		}
	}
}

func TestPrintResults_OnlySkipped(t *testing.T) {
	results := []injector.Result{
		{Action: "skipped", Detail: "already up to date", Item: config.Item{TargetPath: "a"}},
		{Action: "skipped", Detail: "not present", Item: config.Item{TargetPath: "b"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if !strings.Contains(output, "2 skipped") {
		t.Errorf("expected '2 skipped', got: %s", output)
	}
	if !strings.Contains(output, "0 changed") {
		t.Errorf("expected '0 changed', got: %s", output)
	}
}

func TestPrintResults_OnlyErrors(t *testing.T) {
	results := []injector.Result{
		{Action: "error", Detail: "permission denied", Item: config.Item{TargetPath: "a"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if !strings.Contains(output, "1 errors") {
		t.Errorf("expected '1 errors', got: %s", output)
	}
	if !strings.Contains(output, "permission denied") {
		t.Error("expected error detail in output")
	}
}

func TestPrintResults_NoWarningsHidesWarningCount(t *testing.T) {
	results := []injector.Result{
		{Action: "created", Detail: "ok", Item: config.Item{TargetPath: "a"}},
	}

	output := captureStdout(t, func() {
		PrintResults(results)
	})

	if strings.Contains(output, "warnings") {
		t.Error("should not include 'warnings' in summary when there are none")
	}
}

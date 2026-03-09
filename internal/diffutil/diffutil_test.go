package diffutil

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_Identical(t *testing.T) {
	text := "line1\nline2\nline3\n"
	diff := UnifiedDiff("a", "b", text, text)
	if diff != "" {
		t.Errorf("expected empty diff for identical inputs, got:\n%s", diff)
	}
}

func TestUnifiedDiff_SingleLineDifference(t *testing.T) {
	old := "hello\n"
	new := "world\n"
	diff := UnifiedDiff("main", "branch", old, new)
	if !strings.Contains(diff, "-hello") {
		t.Errorf("diff should contain removed line, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+world") {
		t.Errorf("diff should contain added line, got:\n%s", diff)
	}
	if !strings.Contains(diff, "--- main") {
		t.Errorf("diff should contain old label, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+++ branch") {
		t.Errorf("diff should contain new label, got:\n%s", diff)
	}
}

func TestUnifiedDiff_MultiLine(t *testing.T) {
	old := "a\nb\nc\nd\n"
	new := "a\nc\nd\ne\n"
	diff := UnifiedDiff("old", "new", old, new)
	if !strings.Contains(diff, "-b") {
		t.Errorf("diff should show removed line 'b', got:\n%s", diff)
	}
	if !strings.Contains(diff, "+e") {
		t.Errorf("diff should show added line 'e', got:\n%s", diff)
	}
}

func TestColorDiff_PreservesContent(t *testing.T) {
	diff := "--- main\n+++ branch\n@@ -1 +1 @@\n-old\n+new\n context\n"
	colored := ColorDiff(diff)
	// Should preserve all text content regardless of whether ANSI codes are added
	// (lipgloss may skip ANSI in non-TTY environments).
	if !strings.Contains(colored, "old") || !strings.Contains(colored, "new") {
		t.Error("ColorDiff should preserve text content")
	}
	if !strings.Contains(colored, "context") {
		t.Error("ColorDiff should preserve context lines")
	}
}

func TestSummaryLine_Identical(t *testing.T) {
	text := "same\n"
	got := SummaryLine(text, text)
	if got != "Outputs are identical" {
		t.Errorf("got %q, want %q", got, "Outputs are identical")
	}
}

func TestSummaryLine_Different(t *testing.T) {
	old := "a\nb\n"
	new := "a\nc\nd\n"
	got := SummaryLine(old, new)
	if !strings.Contains(got, "Outputs differ") {
		t.Errorf("expected 'Outputs differ' prefix, got %q", got)
	}
	if !strings.Contains(got, "added") || !strings.Contains(got, "removed") {
		t.Errorf("expected counts in summary, got %q", got)
	}
}

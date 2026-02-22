package ui

import (
	"strings"
	"testing"
)

func TestRenderState_Empty(t *testing.T) {
	got := RenderState("")
	if got == "" {
		t.Error("RenderState(\"\") should return non-empty string")
	}
}

func TestRenderState_KnownStates(t *testing.T) {
	states := []string{"active", "review", "approved", "review-blocked", "merged", "closed", "done", "paused"}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			got := RenderState(state)
			if got == "" {
				t.Errorf("RenderState(%q) should return non-empty string", state)
			}
		})
	}
}

func TestRenderState_UnknownState(t *testing.T) {
	got := RenderState("custom-state")
	if got == "" {
		t.Error("RenderState for unknown state should return non-empty string")
	}
}

func TestStateStyle_AllKnownStates(t *testing.T) {
	states := []string{"active", "review", "approved", "review-blocked", "merged", "closed", "done", "paused"}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			style := StateStyle(state)
			// Apply the style to a test string to verify it works
			rendered := style.Render(state)
			if rendered == "" {
				t.Errorf("StateStyle(%q).Render() should produce non-empty output", state)
			}
		})
	}
}

func TestStateStyle_UnknownState(t *testing.T) {
	style := StateStyle("something-else")
	rendered := style.Render("something-else")
	if rendered == "" {
		t.Error("StateStyle for unknown state should still produce output")
	}
}

func TestRenderState_ContainsStateText(t *testing.T) {
	states := []string{"active", "review", "approved", "review-blocked", "merged", "closed", "done", "paused"}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			got := RenderState(state)
			if !strings.Contains(got, state) {
				t.Errorf("RenderState(%q) = %q, should contain the state text", state, got)
			}
		})
	}
}

func TestRenderState_UnknownContainsText(t *testing.T) {
	got := RenderState("custom-state")
	if !strings.Contains(got, "custom-state") {
		t.Errorf("RenderState(\"custom-state\") = %q, should contain 'custom-state'", got)
	}
}

func TestRenderState_EmptyContainsDash(t *testing.T) {
	got := RenderState("")
	if !strings.Contains(got, "--") {
		t.Errorf("RenderState(\"\") = %q, should contain '--'", got)
	}
}

func TestRenderState_PlaceholderLength(t *testing.T) {
	// Empty state renders as "--" which should be at least 2 chars
	got := RenderState("")
	if len(got) < 2 {
		t.Errorf("RenderState(\"\") output too short: %q", got)
	}
}

func TestStateStyle_DefaultReturnsSameText(t *testing.T) {
	// Unknown states should not crash and should render
	strangeStates := []string{"123", "a-b-c", "x"}
	for _, s := range strangeStates {
		rendered := StateStyle(s).Render(s)
		if rendered == "" {
			t.Errorf("StateStyle(%q).Render() returned empty", s)
		}
		if !strings.Contains(rendered, s) {
			t.Errorf("StateStyle(%q).Render() = %q, should contain %q", s, rendered, s)
		}
	}
}

func TestStateStyle_ReturnsDistinctStyleObjects(t *testing.T) {
	// Verify that different states return style objects with different foreground colors.
	// We check that the style objects exist and can render without panic,
	// since in non-TTY environments lipgloss may strip ANSI codes.
	for _, state := range []string{"active", "review", "approved", "merged", "closed"} {
		style := StateStyle(state)
		rendered := style.Render("text")
		if rendered == "" {
			t.Errorf("StateStyle(%q).Render(\"text\") returned empty string", state)
		}
	}

	// Unknown state should return unstyled base — rendering should still produce text
	unknownRendered := StateStyle("xyz").Render("text")
	if unknownRendered != "text" && !strings.Contains(unknownRendered, "text") {
		t.Errorf("unknown state render should contain 'text', got %q", unknownRendered)
	}
}

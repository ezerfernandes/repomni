package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestHighlightQuery(t *testing.T) {
	bold := lipgloss.NewStyle().Bold(true)

	tests := []struct {
		name  string
		text  string
		query string
		want  string
	}{
		{"empty query", "hello world", "", "hello world"},
		{"no match", "hello world", "xyz", "hello world"},
		{"case insensitive", "Hello World", "hello", bold.Render("Hello") + " World"},
		{"multiple matches", "abcabc", "abc", bold.Render("abc") + bold.Render("abc")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highlightQuery(tt.text, tt.query, &bold)
			if got != tt.want {
				t.Errorf("highlightQuery(%q, %q) = %q, want %q", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

func TestHighlightQuery_NilStyle(t *testing.T) {
	got := highlightQuery("Hello World", "hello", nil)
	if got != "Hello World" {
		t.Errorf("highlightQuery with nil style = %q, want original text", got)
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hi", 10, "hi"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"newlines replaced", "a\nb\nc", 10, "a b c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		secs float64
		want string
	}{
		{"seconds", 45, "45s"},
		{"minutes", 125, "2m"},
		{"hours", 3725, "1h 2m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.secs)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.secs, got, tt.want)
			}
		})
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"small", 42, "42"},
		{"thousands", 1500, "1.5K"},
		{"millions", 2500000, "2.5M"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCount(tt.n)
			if got != tt.want {
				t.Errorf("formatCount(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"bytes", 500, "500 B"},
		{"kilobytes", 2048, "2.0 KB"},
		{"megabytes", 5 * 1024 * 1024, "5.0 MB"},
		{"gigabytes", 3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "--"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimeAgo(tt.t)
			if got != tt.want {
				t.Errorf("formatTimeAgo(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}

	// Test relative times with approximate checks
	now := time.Now()

	got := formatTimeAgo(now.Add(-30 * time.Minute))
	if got != "30m ago" {
		t.Errorf("30m ago: got %q", got)
	}

	got = formatTimeAgo(now.Add(-5 * time.Hour))
	if got != "5h ago" {
		t.Errorf("5h ago: got %q", got)
	}

	got = formatTimeAgo(now.Add(-3 * 24 * time.Hour))
	if got != "3d ago" {
		t.Errorf("3d ago: got %q", got)
	}

	old := now.Add(-60 * 24 * time.Hour)
	got = formatTimeAgo(old)
	if got != old.Format("2006-01-02") {
		t.Errorf("old date: got %q, want %q", got, old.Format("2006-01-02"))
	}
}

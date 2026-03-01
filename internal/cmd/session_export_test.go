package cmd

import "testing"

func TestIsToolOnly(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty string", "", true},
		{"blank lines only", "\n\n", true},
		{"single tool line", "[tool: read_file]", true},
		{"tool and result", "[tool: read_file]\n[result]", true},
		{"tool with surrounding whitespace", "  [tool: read_file]  ", true},
		{"tool lines with blank lines between", "[tool: read_file]\n\n[result]\n", true},
		{"plain text", "hello world", false},
		{"mixed content", "[tool: read_file]\nhello world", false},
		{"text before tool", "hello\n[tool: read_file]", false},
		{"partial tool prefix", "[tool", false},
		{"result line only", "[result]", true},
		{"result with trailing text", "[result] extra", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isToolOnly(tt.content)
			if got != tt.want {
				t.Errorf("isToolOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripToolLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"no tool lines", "hello world", "hello world"},
		{"all tool lines", "[tool: read_file]\n[result]", ""},
		{"mixed lines", "hello\n[tool: read_file]\nworld", "hello\nworld"},
		{"tool at start", "[tool: write]\nsome text", "some text"},
		{"tool at end", "some text\n[result]", "some text"},
		{"empty string", "", ""},
		{"indented tool line", "hello\n  [tool: read_file]\nworld", "hello\nworld"},
		{"multiple consecutive tools", "[tool: a]\n[result]\n[tool: b]\n[result]", ""},
		{"preserves non-tool lines", "line1\n[tool: x]\nline2\n[result]\nline3", "line1\nline2\nline3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripToolLines(tt.content)
			if got != tt.want {
				t.Errorf("stripToolLines() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatExportDuration(t *testing.T) {
	tests := []struct {
		secs float64
		want string
	}{
		{0, "0s"},
		{1, "1s"},
		{30, "30s"},
		{59, "59s"},
		{60, "1m"},
		{90, "1m"},
		{300, "5m"},
		{3599, "59m"},
		{3600, "1h 0m"},
		{3661, "1h 1m"},
		{7200, "2h 0m"},
		{9000, "2h 30m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatExportDuration(tt.secs)
			if got != tt.want {
				t.Errorf("formatExportDuration(%v) = %q, want %q", tt.secs, got, tt.want)
			}
		})
	}
}

package cmd

import (
	"testing"
	"time"
)

func TestParseDayDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1 day", "1d", 24 * time.Hour, false},
		{"7 days", "7d", 7 * 24 * time.Hour, false},
		{"30 days", "30d", 30 * 24 * time.Hour, false},
		{"0 days", "0d", 0, false},
		{"go duration hours", "72h", 72 * time.Hour, false},
		{"go duration minutes", "30m", 30 * time.Minute, false},
		{"with leading whitespace", "  7d", 7 * 24 * time.Hour, false},
		{"with trailing whitespace", "7d  ", 7 * 24 * time.Hour, false},
		{"invalid day count", "xd", 0, true},
		{"empty string", "", 0, true},
		{"invalid go duration", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDayDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDayDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseDayDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatCleanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{10240, "10.0 KB"},
		{1048576, "1.0 MB"},
		{2621440, "2.5 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatCleanSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatCleanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

package cmd

import (
	"testing"

	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

func TestResolveMergeNumber(t *testing.T) {
	tests := []struct {
		name        string
		mergeNumber int
		mergeURL    string
		want        int
	}{
		{
			name:        "cached number",
			mergeNumber: 42,
			mergeURL:    "https://github.com/org/repo/pull/42",
			want:        42,
		},
		{
			name:        "fallback to github URL",
			mergeNumber: 0,
			mergeURL:    "https://github.com/org/repo/pull/99",
			want:        99,
		},
		{
			name:        "fallback to gitlab URL",
			mergeNumber: 0,
			mergeURL:    "https://gitlab.com/group/project/-/merge_requests/55",
			want:        55,
		},
		{
			name:        "no number anywhere",
			mergeNumber: 0,
			mergeURL:    "https://github.com/org/repo",
			want:        0,
		},
		{
			name:        "cached takes precedence over URL",
			mergeNumber: 10,
			mergeURL:    "https://github.com/org/repo/pull/99",
			want:        10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &repoconfig.RepoConfig{
				MergeNumber: tt.mergeNumber,
				MergeURL:    tt.mergeURL,
			}
			got := resolveMergeNumber(cfg)
			if got != tt.want {
				t.Errorf("resolveMergeNumber() = %d, want %d", got, tt.want)
			}
		})
	}
}

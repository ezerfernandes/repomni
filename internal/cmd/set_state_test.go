package cmd

import "testing"

func TestValidateMergeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https github", "https://github.com/org/repo/pull/42", false},
		{"valid https gitlab", "https://gitlab.com/group/project/-/merge_requests/1", false},
		{"valid http", "http://gitlab.com/group/project/-/merge_requests/1", false},
		{"no scheme", "github.com/org/repo/pull/42", true},
		{"ftp scheme", "ftp://example.com/something", true},
		{"no host", "https:///path", true},
		{"invalid url", "://broken", true},
		{"empty string", "", true},
		{"self-hosted github", "https://github.internal.com/team/repo/pull/99", false},
		{"self-hosted gitlab", "https://git.company.com/group/project/-/merge_requests/5", false},
		{"with query params", "https://github.com/org/repo/pull/42?tab=files", false},
		{"with fragment", "https://github.com/org/repo/pull/42#discussion", false},
		{"with port", "https://gitlab.com:8443/group/project/-/merge_requests/1", false},
		{"just scheme and host", "https://github.com", false},
		{"ssh scheme", "ssh://github.com/org/repo", true},
		{"file scheme", "file:///path/to/file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMergeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMergeURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

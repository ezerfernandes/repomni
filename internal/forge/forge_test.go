package forge

import "testing"

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https github", "https://github.com/org/repo.git", "github.com"},
		{"https gitlab", "https://gitlab.com/org/repo.git", "gitlab.com"},
		{"https enterprise", "https://github.mycompany.com/org/repo.git", "github.mycompany.com"},
		{"ssh github", "git@github.com:org/repo.git", "github.com"},
		{"ssh gitlab", "git@gitlab.com:org/repo.git", "gitlab.com"},
		{"ssh enterprise", "git@github.mycompany.com:org/repo.git", "github.mycompany.com"},
		{"ssh no user", "github.com:org/repo.git", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHost(tt.url)
			if got != tt.want {
				t.Errorf("extractHost(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseMergeNumber(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		{"github pr", "https://github.com/org/repo/pull/42", 42},
		{"gitlab mr", "https://gitlab.com/org/repo/-/merge_requests/99", 99},
		{"no number", "https://github.com/org/repo", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMergeNumber(tt.url)
			if got != tt.want {
				t.Errorf("ParseMergeNumber(%q) = %d, want %d", tt.url, got, tt.want)
			}
		})
	}
}

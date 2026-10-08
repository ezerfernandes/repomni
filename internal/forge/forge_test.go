package forge

import (
	"testing"
)

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

func TestRepoSlugFromRemote(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https", "https://github.com/Org/Repo.git", "org/repo"},
		{"https no suffix", "https://github.com/org/repo", "org/repo"},
		{"https credentials", "https://user:token@gitlab.com/group/sub/project.git", "group/sub/project"},
		{"ssh scheme", "ssh://git@github.com:22/org/repo.git", "org/repo"},
		{"scp-like", "git@github.com:org/repo.git", "org/repo"},
		{"scp-like host alias", "work-gh:org/repo.git", "org/repo"},
		{"local path", "/tmp/origin.git", ""},
		{"relative path with colon", "repos/a:b", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RepoSlugFromRemote(tt.url); got != tt.want {
				t.Errorf("RepoSlugFromRemote(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestRepoSlugFromMergeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"github pr", "https://github.com/Org/Repo/pull/42", "org/repo"},
		{"github pr files tab", "https://github.com/org/repo/pull/42/files", "org/repo"},
		{"gitlab mr", "https://gitlab.com/group/project/-/merge_requests/7", "group/project"},
		{"gitlab subgroup", "https://gitlab.com/group/sub/project/-/merge_requests/7", "group/sub/project"},
		{"not a pr", "https://github.com/org/repo", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RepoSlugFromMergeURL(tt.url); got != tt.want {
				t.Errorf("RepoSlugFromMergeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestCliName(t *testing.T) {
	tests := []struct {
		platform Platform
		want     string
	}{
		{PlatformGitHub, "gh"},
		{PlatformGitLab, "glab"},
	}
	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			got := cliName(tt.platform)
			if got != tt.want {
				t.Errorf("cliName(%q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

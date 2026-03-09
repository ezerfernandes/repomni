package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

func TestBuildCreateArgs_GitHub(t *testing.T) {
	tests := []struct {
		name      string
		fill      bool
		draft     bool
		reviewers []string
		base      string
		title     string
		body      string
		want      []string
	}{
		{
			name: "no flags",
			want: []string{"pr", "create"},
		},
		{
			name: "fill",
			fill: true,
			want: []string{"pr", "create", "--fill"},
		},
		{
			name:  "draft",
			draft: true,
			want:  []string{"pr", "create", "--draft"},
		},
		{
			name: "base branch",
			base: "main",
			want: []string{"pr", "create", "--base", "main"},
		},
		{
			name:  "title and body",
			title: "Add feature",
			body:  "This adds a feature",
			want:  []string{"pr", "create", "--title", "Add feature", "--body", "This adds a feature"},
		},
		{
			name:      "reviewers",
			reviewers: []string{"alice", "bob"},
			want:      []string{"pr", "create", "--reviewer", "alice", "--reviewer", "bob"},
		},
		{
			name:      "all flags",
			fill:      true,
			draft:     true,
			base:      "develop",
			title:     "My PR",
			body:      "Description",
			reviewers: []string{"carol"},
			want:      []string{"pr", "create", "--base", "develop", "--fill", "--draft", "--title", "My PR", "--body", "Description", "--reviewer", "carol"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore package-level vars.
			origFill, origDraft, origReviewers := submitFill, submitDraft, submitReviewers
			origBase, origTitle, origBody := submitBase, submitTitle, submitBody
			defer func() {
				submitFill, submitDraft, submitReviewers = origFill, origDraft, origReviewers
				submitBase, submitTitle, submitBody = origBase, origTitle, origBody
			}()

			submitFill = tt.fill
			submitDraft = tt.draft
			submitReviewers = tt.reviewers
			submitBase = tt.base
			submitTitle = tt.title
			submitBody = tt.body

			got := buildCreateArgs(forge.PlatformGitHub)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildCreateArgs(GitHub) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCreateArgs_GitLab(t *testing.T) {
	tests := []struct {
		name      string
		fill      bool
		draft     bool
		reviewers []string
		base      string
		title     string
		body      string
		want      []string
	}{
		{
			name: "no flags",
			want: []string{"mr", "create"},
		},
		{
			name: "base uses target-branch",
			base: "main",
			want: []string{"mr", "create", "--target-branch", "main"},
		},
		{
			name: "body uses description flag",
			body: "MR description text",
			want: []string{"mr", "create", "--description", "MR description text"},
		},
		{
			name:      "all flags",
			fill:      true,
			draft:     true,
			base:      "develop",
			title:     "My MR",
			body:      "Description",
			reviewers: []string{"carol"},
			want:      []string{"mr", "create", "--target-branch", "develop", "--fill", "--draft", "--title", "My MR", "--description", "Description", "--reviewer", "carol"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origFill, origDraft, origReviewers := submitFill, submitDraft, submitReviewers
			origBase, origTitle, origBody := submitBase, submitTitle, submitBody
			defer func() {
				submitFill, submitDraft, submitReviewers = origFill, origDraft, origReviewers
				submitBase, submitTitle, submitBody = origBase, origTitle, origBody
			}()

			submitFill = tt.fill
			submitDraft = tt.draft
			submitReviewers = tt.reviewers
			submitBase = tt.base
			submitTitle = tt.title
			submitBody = tt.body

			got := buildCreateArgs(forge.PlatformGitLab)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildCreateArgs(GitLab) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunSubmit_AlreadyHasMergeURL(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{
		Version:  1,
		State:    "review",
		MergeURL: "https://github.com/org/repo/pull/42",
	}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "PR/MR already exists") {
		t.Errorf("error = %q, want it to mention PR/MR already exists", err.Error())
	}
}

func TestRunSubmit_OnMainBranch(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	// The default branch from initGitRepo is usually "master" or "main".
	// Ensure we're on one of them by checking the current branch.
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	// Rename branch to "main" so the test is deterministic.
	runGitCmd(t, repoDir, "branch", "-M", "main")

	err := runSubmit(submitCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot submit from main") {
		t.Errorf("error = %q, want it to mention cannot submit from main", err.Error())
	}
}

func TestLastNonEmptyLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single line", "https://github.com/org/repo/pull/1", "https://github.com/org/repo/pull/1"},
		{"trailing newline", "https://github.com/org/repo/pull/1\n", "https://github.com/org/repo/pull/1"},
		{"multiple lines", "Creating pull request...\nhttps://github.com/org/repo/pull/1", "https://github.com/org/repo/pull/1"},
		{"trailing blank lines", "url\n\n\n", "url"},
		{"empty string", "", ""},
		{"only whitespace", "   \n  \n  ", ""},
		{"leading blank lines", "\n\nhttps://example.com/pull/5", "https://example.com/pull/5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastNonEmptyLine(tt.input)
			if got != tt.want {
				t.Errorf("lastNonEmptyLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

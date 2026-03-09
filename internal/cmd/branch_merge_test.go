package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ezerfernandes/repomni/internal/forge"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

func TestBuildMergeArgs_GitHub(t *testing.T) {
	tests := []struct {
		name         string
		squash       bool
		rebase       bool
		deleteBranch bool
		want         []string
	}{
		{
			name: "no flags",
			want: []string{"pr", "merge"},
		},
		{
			name:   "squash",
			squash: true,
			want:   []string{"pr", "merge", "--squash"},
		},
		{
			name:   "rebase",
			rebase: true,
			want:   []string{"pr", "merge", "--rebase"},
		},
		{
			name:         "delete branch",
			deleteBranch: true,
			want:         []string{"pr", "merge", "--delete-branch"},
		},
		{
			name:         "all flags",
			squash:       true,
			rebase:       true,
			deleteBranch: true,
			want:         []string{"pr", "merge", "--squash", "--rebase", "--delete-branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origSquash, origRebase, origDelete := mergeSquash, mergeRebase, mergeDeleteBranch
			defer func() { mergeSquash, mergeRebase, mergeDeleteBranch = origSquash, origRebase, origDelete }()

			mergeSquash = tt.squash
			mergeRebase = tt.rebase
			mergeDeleteBranch = tt.deleteBranch

			got := buildMergeArgs(forge.PlatformGitHub)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMergeArgs(GitHub) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildMergeArgs_GitLab(t *testing.T) {
	tests := []struct {
		name         string
		squash       bool
		rebase       bool
		deleteBranch bool
		want         []string
	}{
		{
			name: "no flags",
			want: []string{"mr", "merge"},
		},
		{
			name:   "squash",
			squash: true,
			want:   []string{"mr", "merge", "--squash"},
		},
		{
			name:   "rebase",
			rebase: true,
			want:   []string{"mr", "merge", "--rebase"},
		},
		{
			name:         "delete branch uses remove-source-branch",
			deleteBranch: true,
			want:         []string{"mr", "merge", "--remove-source-branch"},
		},
		{
			name:         "all flags",
			squash:       true,
			rebase:       true,
			deleteBranch: true,
			want:         []string{"mr", "merge", "--squash", "--rebase", "--remove-source-branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origSquash, origRebase, origDelete := mergeSquash, mergeRebase, mergeDeleteBranch
			defer func() { mergeSquash, mergeRebase, mergeDeleteBranch = origSquash, origRebase, origDelete }()

			mergeSquash = tt.squash
			mergeRebase = tt.rebase
			mergeDeleteBranch = tt.deleteBranch

			got := buildMergeArgs(forge.PlatformGitLab)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMergeArgs(GitLab) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunMerge_NoMergeURL(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	gitDir := filepath.Join(repoDir, ".git")
	cfg := &repoconfig.RepoConfig{Version: 1, State: "active"}
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runMerge(mergeCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no PR/MR attached; use \"branch submit\" or \"branch attach\" first"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunMerge_NoConfig(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	err := runMerge(mergeCmd, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "no PR/MR attached; use \"branch submit\" or \"branch attach\" first"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

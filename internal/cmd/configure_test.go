package cmd

import (
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
)

func TestMergeRepoConfigPreservesWorkflowState(t *testing.T) {
	existing := &repoconfig.RepoConfig{
		Version:     1,
		State:       "review",
		MergeURL:    "https://github.com/o/r/pull/42",
		MergeNumber: 42,
		Ticket:      "PROJ-7",
		BaseBranch:  "main",
		Draft:       true,
		Description: "wip",
		Remote:      true,
		Items:       []repoconfig.RepoItemConfig{{TargetPath: ".env", Enabled: true}},
	}
	// Simulates what the editor/form returns: only version + items, as if the
	// user had deleted the workflow-state lines from the buffer.
	edited := &repoconfig.RepoConfig{
		Version: 1,
		Items:   []repoconfig.RepoItemConfig{{TargetPath: ".env", Enabled: false}},
	}

	got := mergeRepoConfig(existing, edited)

	if got.State != "review" || got.MergeURL != existing.MergeURL || got.MergeNumber != 42 ||
		got.Ticket != "PROJ-7" || got.BaseBranch != "main" || !got.Draft ||
		got.Description != "wip" || !got.Remote {
		t.Fatalf("workflow state not preserved: %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].Enabled {
		t.Fatalf("edited items not applied: %+v", got.Items)
	}
}

func TestMergeRepoConfigNilExistingDefaultsVersion(t *testing.T) {
	edited := &repoconfig.RepoConfig{
		Version: 0, // user deleted the version line
		Items:   []repoconfig.RepoItemConfig{{TargetPath: ".env", Enabled: true}},
	}

	got := mergeRepoConfig(nil, edited)

	if got.Version != 1 {
		t.Fatalf("Version = %d, want 1 (default)", got.Version)
	}
	if got.State != "" || got.MergeURL != "" {
		t.Fatalf("fresh config should have no workflow state: %+v", got)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items not carried over: %+v", got.Items)
	}
}

func TestEditableRepoSeedExcludesWorkflowState(t *testing.T) {
	existing := &repoconfig.RepoConfig{
		Version:  1,
		State:    "review",
		MergeURL: "https://github.com/o/r/pull/42",
		Ticket:   "PROJ-7",
		Items:    []repoconfig.RepoItemConfig{{TargetPath: ".env", Enabled: true}},
	}

	seed := editableRepoSeed(existing, nil)

	if seed.State != "" || seed.MergeURL != "" || seed.Ticket != "" {
		t.Fatalf("seed must not expose workflow state: %+v", seed)
	}
	if seed.Version != 1 || len(seed.Items) != 1 {
		t.Fatalf("seed should carry version + items: %+v", seed)
	}
}

func TestEditableRepoSeedNilBuildsTemplate(t *testing.T) {
	globalCfg := &config.Config{
		Version:   1,
		SourceDir: t.TempDir(),
		Items: []config.Item{
			{Type: config.ItemTypeFile, SourcePath: "a", TargetPath: ".a", Enabled: true},
			{Type: config.ItemTypeFile, SourcePath: "b", TargetPath: ".b", Enabled: false},
		},
	}

	seed := editableRepoSeed(nil, globalCfg)

	if len(seed.Items) != 2 {
		t.Fatalf("template should list every global item, got %+v", seed.Items)
	}
	if seed.State != "" || seed.MergeURL != "" {
		t.Fatalf("template seed must not carry workflow state: %+v", seed)
	}
}

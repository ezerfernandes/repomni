package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/editor"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "repo",
	Short: "Configure injection settings for this repository",
	Long: `Interactively select which items and entries to inject into this repository.

The configuration is saved to .git/repomni/config.yaml and is used by
"repomni inject" and "repomni branch" to filter which items get injected.`,
	RunE: runConfigure,
}

func init() {
	configCmd.AddCommand(configureCmd)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	repoRoot, err := gitutil.RunGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	gitDir, err := gitutil.FindGitDir(repoRoot)
	if err != nil {
		return err
	}

	globalCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("global config not found (run 'repomni config global' first): %w", err)
	}

	existingRepoCfg, err := repoconfig.Load(gitDir)
	if err != nil {
		return fmt.Errorf("cannot read existing repo config: %w", err)
	}

	// Prefer editing the raw YAML in an external editor (vim/$EDITOR) when one
	// is available; fall back to the guided TUI form otherwise.
	if editor.Available() {
		return configureRepoViaEditor(gitDir, globalCfg, existingRepoCfg)
	}

	repoCfg, err := ui.RunConfigureRepoForm(globalCfg, existingRepoCfg)
	if err != nil {
		return fmt.Errorf("configuration cancelled: %w", err)
	}

	merged := mergeRepoConfig(existingRepoCfg, repoCfg)
	if err := repoconfig.Save(gitDir, merged); err != nil {
		return err
	}

	fmt.Printf("\nRepository configuration saved to %s\n", repoconfig.ConfigPath(gitDir))
	return nil
}

// configureRepoViaEditor opens the per-repo config as raw YAML in an external
// editor. Only the editable injection settings are shown; workflow-state fields
// are preserved via mergeRepoConfig. Quitting with the buffer unchanged (or
// empty) aborts without writing, so merely opening a config-less repo to look
// no longer materializes a pinned config.
func configureRepoViaEditor(gitDir string, globalCfg *config.Config, existing *repoconfig.RepoConfig) error {
	seed := editableRepoSeed(existing, globalCfg)

	header := "# repomni per-repo config.\n" +
		"# Set enabled: true/false per item. For directory items, list the\n" +
		"# entries to inject under 'entries' (omit to inject all entries).\n" +
		"#\n" +
		"# Save with no changes (or an empty buffer) to abort without writing.\n" +
		"# Branch workflow state (state, PR/MR URL, ticket) is preserved for you.\n\n"

	edited, err := editYAMLConfig(seed, header)
	if err != nil {
		if errors.Is(err, editor.ErrAborted) {
			fmt.Println("No changes made; repository configuration left unchanged.")
			return nil
		}
		// editYAMLConfig already describes the failure (e.g. a YAML syntax
		// error); don't relabel a fixable typo as a cancellation.
		return err
	}

	repoCfg := mergeRepoConfig(existing, edited)
	if err := repoconfig.Save(gitDir, repoCfg); err != nil {
		return err
	}

	fmt.Printf("\nRepository configuration saved to %s\n", repoconfig.ConfigPath(gitDir))
	return nil
}

// editableRepoSeed returns the subset of a per-repo config meant to be
// hand-edited: the version and per-item injection settings. Workflow-state
// fields (state, merge URL, ticket, etc.) are deliberately excluded so an
// editor round-trip cannot wipe them. For a repo without an existing config the
// seed is a template listing every global item.
func editableRepoSeed(existing *repoconfig.RepoConfig, globalCfg *config.Config) repoconfig.RepoConfig {
	if existing != nil {
		return repoconfig.RepoConfig{Version: existing.Version, Items: existing.Items}
	}
	tmpl := buildRepoConfigTemplate(globalCfg)
	return repoconfig.RepoConfig{Version: tmpl.Version, Items: tmpl.Items}
}

// mergeRepoConfig carries edited's injection settings (version + items) into a
// copy of existing while preserving every workflow-state field, so re-running
// "config repo" never clobbers branch state, PR/MR URLs, tickets, or related
// metadata written by the branch workflow commands.
func mergeRepoConfig(existing, edited *repoconfig.RepoConfig) *repoconfig.RepoConfig {
	result := &repoconfig.RepoConfig{}
	if existing != nil {
		*result = *existing
	}
	result.Items = edited.Items
	if edited.Version > 0 {
		result.Version = edited.Version
	}
	if result.Version == 0 {
		result.Version = 1
	}
	return result
}

// buildRepoConfigTemplate constructs a per-repo config pre-populated from the
// global config: every item is listed with its default enablement, and enabled
// directory items list all their available entries.
func buildRepoConfigTemplate(globalCfg *config.Config) *repoconfig.RepoConfig {
	sourceDir, _ := filepath.Abs(globalCfg.SourceDir)

	var items []repoconfig.RepoItemConfig
	for _, item := range globalCfg.Items {
		rc := repoconfig.RepoItemConfig{
			TargetPath: item.TargetPath,
			Enabled:    item.Enabled,
		}
		if item.Enabled && item.Type == config.ItemTypeDirectory {
			if entries, err := os.ReadDir(filepath.Join(sourceDir, item.SourcePath)); err == nil {
				for _, e := range entries {
					rc.Entries = append(rc.Entries, e.Name())
				}
			}
		}
		items = append(items, rc)
	}

	return &repoconfig.RepoConfig{Version: 1, Items: items}
}

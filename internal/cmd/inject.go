package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ezerfernandes/repoinjector/internal/config"
	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/ezerfernandes/repoinjector/internal/injector"
	"github.com/ezerfernandes/repoinjector/internal/repoconfig"
	"github.com/ezerfernandes/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var injectCmd = &cobra.Command{
	Use:   "inject [target]",
	Short: "Inject configured files into target repo(s)",
	Long: `Symlinks or copies configured files from the source directory into the target
repository. Injected files are added to .git/info/exclude to keep them
invisible to git.

If no target is specified, the current directory is used.
Use --all to inject into all git repos under the target directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInject,
}

var (
	injectAll     bool
	injectDryRun  bool
	injectForce   bool
	injectCopy    bool
	injectSymlink bool
)

func init() {
	rootCmd.AddCommand(injectCmd)
	injectCmd.Flags().BoolVar(&injectAll, "all", false, "inject into all git repos under target directory")
	injectCmd.Flags().BoolVar(&injectDryRun, "dry-run", false, "show what would be done without doing it")
	injectCmd.Flags().BoolVar(&injectForce, "force", false, "overwrite existing regular files")
	injectCmd.Flags().BoolVar(&injectCopy, "copy", false, "use copy mode for this run")
	injectCmd.Flags().BoolVar(&injectSymlink, "symlink", false, "use symlink mode for this run")
}

// loadRepoConfig attempts to load a per-repo config from the target's git directory.
// Returns (nil, nil) if the target is not a git repo or has no per-repo config.
func loadRepoConfig(target string) (*repoconfig.RepoConfig, error) {
	gitDir, err := gitutil.FindGitDir(target)
	if err != nil {
		return nil, nil
	}
	return repoconfig.Load(gitDir)
}

func runInject(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}

	mode := cfg.Mode
	if injectCopy {
		mode = config.ModeCopy
	} else if injectSymlink {
		mode = config.ModeSymlink
	}

	opts := injector.Options{
		DryRun: injectDryRun,
		Force:  injectForce,
		Mode:   mode,
	}

	// For single-target runs, apply per-repo config filtering and show picker.
	injectCfg := cfg
	if !injectAll {
		var allowedEntries map[string]map[string]bool
		repoCfg, _ := loadRepoConfig(target)
		if repoCfg != nil {
			injectCfg = repoCfg.FilterGlobalConfig(cfg)
			allowedEntries = repoCfg.ToSelectedEntries()
		}

		selected, err := ui.SelectDirEntries(injectCfg, allowedEntries)
		if err != nil {
			return err
		}
		if selected != nil {
			opts.SelectedEntries = selected
		}
	}

	var targets []string
	if injectAll {
		targets, err = gitutil.FindGitRepos(target)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("no git repositories found under %s", target)
		}
	} else {
		targets = []string{target}
	}

	hasErrors := false
	for _, t := range targets {
		targetCfg := injectCfg

		// For --all, apply per-repo config per target.
		if injectAll {
			fmt.Printf("\nInjecting into %s...\n", t)
			repoCfg, _ := loadRepoConfig(t)
			if repoCfg != nil {
				targetCfg = repoCfg.FilterGlobalConfig(cfg)
				opts.SelectedEntries = repoCfg.ToSelectedEntries()
			} else {
				targetCfg = cfg
				opts.SelectedEntries = nil
			}
		}

		results, err := injector.Inject(targetCfg, t, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			hasErrors = true
			continue
		}

		ui.PrintResults(results)

		for _, r := range results {
			if r.Action == "error" {
				hasErrors = true
			}
		}
	}

	if injectDryRun {
		fmt.Println("\nDry run — no changes were made.")
	}

	if hasErrors {
		return fmt.Errorf("some items had errors")
	}

	return nil
}

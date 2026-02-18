package cmd

import (
	"fmt"
	"os"

	"github.com/ezer/repoinjector/internal/brancher"
	"github.com/ezer/repoinjector/internal/config"
	"github.com/ezer/repoinjector/internal/gitutil"
	"github.com/ezer/repoinjector/internal/injector"
	"github.com/ezer/repoinjector/internal/repoconfig"
	"github.com/ezer/repoinjector/internal/scripter"
	"github.com/ezer/repoinjector/internal/ui"
	"github.com/spf13/cobra"
)

var branchCmd = &cobra.Command{
	Use:   "branch <branch-name>",
	Short: "Clone the parent repo and create a new branch",
	Long: `Finds the closest parent directory that is a git repository, clones it
into the current directory using the branch name, and checks out a new branch
with that name.

This is useful for creating isolated working copies for feature branches.`,
	Args: cobra.ExactArgs(1),
	RunE: runBranch,
}

var branchNoInject bool

func init() {
	rootCmd.AddCommand(branchCmd)
	branchCmd.Flags().BoolVar(&branchNoInject, "no-inject", false, "skip automatic injection into the new branch")
}

func runBranch(cmd *cobra.Command, args []string) error {
	result, err := brancher.Branch(".", args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Cloned %s into %s\n", result.RemoteURL, result.TargetDir)
	fmt.Printf("Checked out new branch: %s\n", result.Branch)

	parentGitDir, parentGitDirErr := gitutil.FindGitDir(result.ParentRepo)

	// Auto-inject into the new repo unless --no-inject is set.
	if !branchNoInject {
		if err := autoInject(result, parentGitDir, parentGitDirErr); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: auto-injection failed: %v\n", err)
		}
	}

	// Run setup script if configured in the parent repo.
	if parentGitDirErr == nil {
		if _, exists := scripter.GetScript(parentGitDir, scripter.ScriptSetup); exists {
			fmt.Println("Running setup script...")
			if err := scripter.RunScript(parentGitDir, scripter.ScriptSetup, result.TargetDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: setup script failed: %v\n", err)
			}
		}
	}

	return nil
}

func autoInject(result *brancher.Result, parentGitDir string, parentGitDirErr error) error {
	globalCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cannot load global config: %w", err)
	}

	effectiveCfg := globalCfg
	var selectedEntries map[string]map[string]bool
	var repoCfg *repoconfig.RepoConfig

	if parentGitDirErr == nil {
		repoCfg, _ = repoconfig.Load(parentGitDir)
		if repoCfg != nil {
			effectiveCfg = repoCfg.FilterGlobalConfig(globalCfg)
			selectedEntries = repoCfg.ToSelectedEntries()
		}
	}

	opts := injector.Options{
		Mode:            effectiveCfg.Mode,
		SelectedEntries: selectedEntries,
	}

	fmt.Println("Injecting configured files...")
	results, err := injector.Inject(effectiveCfg, result.TargetDir, opts)
	if err != nil {
		return err
	}

	ui.PrintResults(results)

	// Copy parent's per-repo config to the new repo so future inject runs
	// in the branch use the same settings.
	if repoCfg != nil {
		newGitDir, gErr := gitutil.FindGitDir(result.TargetDir)
		if gErr == nil {
			if err := repoconfig.Save(newGitDir, repoCfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not copy repo config to branch: %v\n", err)
			}
		}
	}

	return nil
}

package cmd

import (
	"fmt"
	"os"

	"github.com/ezerfernandes/repomni/internal/brancher"
	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/injector"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/scripter"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

// branchResult is the JSON output for branch create/clone commands.
type branchResult struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	RemoteURL  string `json:"remote_url"`
	ParentRepo string `json:"parent_repo"`
	State      string `json:"state"`
	Remote     bool   `json:"remote"`
	Ticket     string `json:"ticket,omitempty"`
}

var createCmd = &cobra.Command{
	Use:   "create <branch-name>",
	Short: "Clone the parent repo and create a new branch",
	Long: `Finds the closest parent directory that is a git repository, clones it
into the current directory using the branch name, and checks out a new branch
with that name.

This is useful for creating isolated working copies for feature branches.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

var (
	createNoInject bool
	createTicket   string
	createJSON     bool
)

func init() {
	branchCmd.AddCommand(createCmd)
	createCmd.Flags().BoolVar(&createNoInject, "no-inject", false, "skip automatic injection into the new branch")
	createCmd.Flags().StringVar(&createTicket, "ticket", "", "associate a ticket identifier (e.g., PROJ-123)")
	createCmd.Flags().BoolVar(&createJSON, "json", false, "output as JSON")
}

func runCreate(cmd *cobra.Command, args []string) error {
	result, err := brancher.Branch(".", args[0])
	if err != nil {
		return err
	}
	if !createJSON {
		fmt.Printf("Cloned %s into %s\n", result.RemoteURL, result.TargetDir)
		fmt.Printf("Checked out new branch: %s\n", result.Branch)
	}

	parentGitDir, parentGitDirErr := gitutil.FindGitDir(result.ParentRepo)

	// Auto-inject into the new repo unless --no-inject is set.
	if !createNoInject {
		if err := autoInject(result, parentGitDir, parentGitDirErr, createJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: auto-injection failed: %v\n", err)
		}
	}

	// Set initial workflow state for the new branch.
	if newGitDir, err := gitutil.FindGitDir(result.TargetDir); err == nil {
		newCfg, _ := repoconfig.Load(newGitDir)
		if newCfg == nil {
			newCfg = &repoconfig.RepoConfig{Version: 1}
		}
		newCfg.State = string(repoconfig.StateActive)
		newCfg.Remote = false
		newCfg.Ticket = createTicket
		if err := repoconfig.Save(newGitDir, newCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set initial state: %v\n", err)
		}
	}

	// Run setup script if configured in the parent repo.
	if parentGitDirErr == nil {
		if _, exists := scripter.GetScript(parentGitDir, scripter.ScriptSetup); exists {
			if !createJSON {
				fmt.Println("Running setup script...")
			}
			if err := scripter.RunScript(parentGitDir, scripter.ScriptSetup, result.TargetDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: setup script failed: %v\n", err)
			}
		}
	}

	if createJSON {
		return ui.PrintJSON(branchResult{
			Path:       result.TargetDir,
			Branch:     result.Branch,
			RemoteURL:  result.RemoteURL,
			ParentRepo: result.ParentRepo,
			State:      string(repoconfig.StateActive),
			Remote:     false,
			Ticket:     createTicket,
		})
	}

	return nil
}

func autoInject(result *brancher.Result, parentGitDir string, parentGitDirErr error, quiet bool) error {
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

	if !quiet {
		fmt.Println("Injecting configured files...")
	}
	results, err := injector.Inject(effectiveCfg, result.TargetDir, opts)
	if err != nil {
		return err
	}

	if !quiet {
		ui.PrintResults(results)
	}

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

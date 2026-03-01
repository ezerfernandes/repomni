package cmd

import (
	"fmt"
	"os"

	"github.com/ezerfernandes/repomni/internal/brancher"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/scripter"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <branch-name>",
	Short: "Clone the parent repo and check out an existing remote branch",
	Long: `Finds the closest parent directory that is a git repository, clones it
into the current directory, and checks out an existing remote branch.

The local directory name is derived from the branch name by replacing
forbidden characters (such as '/') with '-'. For example,
"feature/my-thing" becomes "feature-my-thing".

This is useful for creating isolated working copies for existing branches.`,
	Args: cobra.ExactArgs(1),
	RunE: runClone,
}

var (
	cloneNoInject bool
	cloneTicket   string
)

func init() {
	branchCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().BoolVar(&cloneNoInject, "no-inject", false, "skip automatic injection into the new clone")
	cloneCmd.Flags().StringVar(&cloneTicket, "ticket", "", "associate a ticket identifier (e.g., PROJ-123)")
}

func runClone(cmd *cobra.Command, args []string) error {
	result, err := brancher.Clone(".", args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Cloned %s into %s\n", result.RemoteURL, result.TargetDir)
	fmt.Printf("Checked out branch: %s\n", result.Branch)

	parentGitDir, parentGitDirErr := gitutil.FindGitDir(result.ParentRepo)

	// Auto-inject into the new repo unless --no-inject is set.
	if !cloneNoInject {
		if err := autoInject(result, parentGitDir, parentGitDirErr); err != nil {
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
		newCfg.Remote = true
		newCfg.Ticket = cloneTicket
		if err := repoconfig.Save(newGitDir, newCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set initial state: %v\n", err)
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

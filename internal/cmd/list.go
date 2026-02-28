package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/ezerfernandes/repoinjector/internal/gitutil"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [directory]",
	Short: "List git repositories in a directory",
	Long: `List all git repositories that are immediate subdirectories of the target
directory, one per line. Useful for piping to other tools.

If no directory is specified, the current directory is used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

var listNames bool

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listNames, "names", false, "output only repo directory names")
}

func runList(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	repos, err := gitutil.FindGitRepos(target)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no git repositories found under %s", target)
	}

	for _, repo := range repos {
		if listNames {
			fmt.Println(filepath.Base(repo))
		} else {
			fmt.Println(repo)
		}
	}

	return nil
}

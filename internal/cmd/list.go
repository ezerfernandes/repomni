package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/ui"
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

var (
	listNames bool
	listJSON  bool
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listNames, "names", false, "output only repo directory names")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
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

	if listJSON {
		type listEntry struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		entries := make([]listEntry, len(repos))
		for i, repo := range repos {
			entries[i] = listEntry{Path: repo, Name: filepath.Base(repo)}
		}
		return ui.PrintJSON(entries)
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

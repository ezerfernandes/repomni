package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

var setDescriptionCmd = &cobra.Command{
	Use:   "set-description <description>",
	Short: "Set a description for the current branch repo",
	Long: `Associate a free-text description with the current repository. The description
is stored in .git/repomni/config.yaml and displayed by "branch list --detailed".

  repomni branch set-description "working on auth refactor"

Use "set-description --clear" to remove the description.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetDescription,
}

var (
	setDescriptionClear bool
	setDescriptionJSON  bool
)

func init() {
	branchCmd.AddCommand(setDescriptionCmd)
	setDescriptionCmd.Flags().BoolVar(&setDescriptionClear, "clear", false, "remove the description")
	setDescriptionCmd.Flags().BoolVar(&setDescriptionJSON, "json", false, "output as JSON")
}

func runSetDescription(cmd *cobra.Command, args []string) error {
	if !setDescriptionClear && len(args) == 0 {
		return fmt.Errorf("provide a description, or use --clear to remove")
	}

	repoRoot, err := gitutil.RunGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	gitDir, err := gitutil.FindGitDir(repoRoot)
	if err != nil {
		return err
	}

	cfg, err := repoconfig.Load(gitDir)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &repoconfig.RepoConfig{Version: 1}
	}

	if setDescriptionClear {
		cfg.Description = ""
		if err := repoconfig.Save(gitDir, cfg); err != nil {
			return err
		}
		if setDescriptionJSON {
			return ui.PrintJSON(struct {
				Description string `json:"description"`
			}{Description: ""})
		}
		fmt.Println("Description cleared.")
		return nil
	}

	cfg.Description = args[0]
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	if setDescriptionJSON {
		return ui.PrintJSON(struct {
			Description string `json:"description"`
		}{Description: cfg.Description})
	}

	fmt.Printf("Description set to: %s\n", cfg.Description)
	return nil
}

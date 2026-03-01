package cmd

import (
	"fmt"

	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/repoconfig"
	"github.com/spf13/cobra"
)

var setTicketCmd = &cobra.Command{
	Use:   "set-ticket <ticket>",
	Short: "Set the ticket identifier for the current branch repo",
	Long: `Associate a ticket identifier (e.g., PROJ-123, a URL, or any string) with
the current repository. The ticket is stored in .git/repomni/config.yaml and
displayed by the "branch list" command.

  repomni branch set-ticket PROJ-123
  repomni branch set-ticket https://linear.app/team/issue/LIN-42

Use "set-ticket --clear" to remove the ticket.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetTicket,
}

var setTicketClear bool

func init() {
	branchCmd.AddCommand(setTicketCmd)
	setTicketCmd.Flags().BoolVar(&setTicketClear, "clear", false, "remove the ticket identifier")
}

func runSetTicket(cmd *cobra.Command, args []string) error {
	if !setTicketClear && len(args) == 0 {
		return fmt.Errorf("provide a ticket identifier, or use --clear to remove")
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

	if setTicketClear {
		cfg.Ticket = ""
		if err := repoconfig.Save(gitDir, cfg); err != nil {
			return err
		}
		fmt.Println("Ticket cleared.")
		return nil
	}

	cfg.Ticket = args[0]
	if err := repoconfig.Save(gitDir, cfg); err != nil {
		return err
	}

	fmt.Printf("Ticket set to: %s\n", cfg.Ticket)
	return nil
}

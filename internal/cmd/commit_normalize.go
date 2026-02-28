package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ezerfernandes/repoinjector/internal/commitnorm"
	"github.com/spf13/cobra"
)

var commitNormalizeCmd = &cobra.Command{
	Use:   "commit-normalize [file]",
	Short: "Normalize a git commit message",
	Long: `Normalize a git commit message by stripping comment lines, trimming
whitespace, capitalizing the subject, removing a trailing period, enforcing a
72-character subject limit, and ensuring a blank line between subject and body.

When a file path is given the file is read, normalized, and rewritten in place.
This makes the command suitable as a git commit-msg hook:

  repoinjector commit-normalize "$1"

When no file is given the message is read from stdin and the normalized result
is written to stdout.

Use --check to verify that a message is already normalized without modifying
it. The command exits with code 1 if the message would change, making it
useful for CI or linting.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommitNormalize,
}

var commitNormalizeCheck bool

func init() {
	rootCmd.AddCommand(commitNormalizeCmd)
	commitNormalizeCmd.Flags().BoolVar(&commitNormalizeCheck, "check", false, "exit 1 if the message would change (no modifications)")
}

func runCommitNormalize(cmd *cobra.Command, args []string) error {
	var raw []byte
	var err error

	if len(args) == 1 {
		raw, err = os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading commit message file: %w", err)
		}
	} else {
		raw, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
	}

	original := string(raw)
	normalized := commitnorm.Normalize(original)

	if commitNormalizeCheck {
		if normalized != original {
			cmd.SilenceUsage = true
			return errors.New("commit message is not normalized")
		}
		return nil
	}

	if len(args) == 1 {
		if err := os.WriteFile(args[0], []byte(normalized), 0644); err != nil {
			return fmt.Errorf("writing commit message file: %w", err)
		}
	} else {
		fmt.Fprint(cmd.OutOrStdout(), normalized)
	}
	return nil
}

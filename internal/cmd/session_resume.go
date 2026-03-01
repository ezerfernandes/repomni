package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ezerfernandes/repoinjector/internal/session"
	"github.com/spf13/cobra"
)

var sessionResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Resume a Claude Code session",
	Long: `Launch Claude Code with --resume to continue a previous session.
Supports prefix matching on the session ID.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionResume,
}

var sessionResumeContinue bool

func init() {
	sessionCmd.AddCommand(sessionResumeCmd)
	sessionResumeCmd.Flags().BoolVar(&sessionResumeContinue, "continue", false,
		"also pass --continue to Claude Code")
}

func runSessionResume(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath()
	if err != nil {
		return err
	}

	meta, err := session.FindSession(projectPath, args[0])
	if err != nil {
		return err
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH; install Claude Code CLI first")
	}

	claudeArgs := []string{claudePath, "--resume", meta.SessionID}
	if sessionResumeContinue {
		claudeArgs = append(claudeArgs, "--continue")
	}

	proc := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}

	p, err := os.StartProcess(claudePath, claudeArgs, proc)
	if err != nil {
		return fmt.Errorf("cannot start claude: %w", err)
	}

	state, err := p.Wait()
	if err != nil {
		return fmt.Errorf("claude exited with error: %w", err)
	}
	if !state.Success() {
		return fmt.Errorf("claude exited with code %d", state.ExitCode())
	}
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ezerfernandes/repomni/internal/session"
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

	meta, err := session.FindSessionAll(projectPath, args[0], sessionCLIFilter)
	if err != nil {
		return err
	}

	if meta.CLI == "codex" {
		return resumeCodex(meta)
	}
	return resumeClaude(meta)
}

func resumeClaude(meta *session.SessionMeta) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH; install Claude Code CLI first")
	}

	claudeArgs := []string{claudePath, "--resume", meta.SessionID}
	if sessionResumeContinue {
		claudeArgs = append(claudeArgs, "--continue")
	}

	return startProcess(claudePath, claudeArgs)
}

func resumeCodex(meta *session.SessionMeta) error {
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("codex not found in PATH; install Codex CLI first")
	}

	codexArgs := []string{codexPath, "resume", meta.SessionID}
	return startProcess(codexPath, codexArgs)
}

func startProcess(binPath string, args []string) error {
	proc := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}

	p, err := os.StartProcess(binPath, args, proc)
	if err != nil {
		return fmt.Errorf("cannot start %s: %w", filepath.Base(binPath), err)
	}

	state, err := p.Wait()
	if err != nil {
		return fmt.Errorf("%s exited with error: %w", filepath.Base(binPath), err)
	}
	if !state.Success() {
		return fmt.Errorf("%s exited with code %d", filepath.Base(binPath), state.ExitCode())
	}
	return nil
}

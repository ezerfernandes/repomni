package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ezerfernandes/repomni/internal/brancher"
	"github.com/ezerfernandes/repomni/internal/diffutil"
	"github.com/ezerfernandes/repomni/internal/gitutil"
	"github.com/ezerfernandes/repomni/internal/ui"
	"github.com/spf13/cobra"
)

// commandResult holds the output and execution status of a command.
type commandResult struct {
	Output   string
	ExitCode int
	Err      error // non-nil when the command failed to start (e.g., missing executable)
}

// diffOutcome holds the result of comparing two command executions.
type diffOutcome struct {
	Stdout   string // text to print to stdout (includes trailing newlines where appropriate)
	Stderr   string // text to print to stderr (includes trailing newlines where appropriate)
	ExitCode int    // 0 = identical, 1 = different or error
}

var (
	execDiffNoSync   bool
	execDiffNameOnly bool
	execDiffMainDir  string
	execDiffJSON     bool
)

var execDiffCmd = &cobra.Command{
	Use:   "diff [flags] -- <command> [args...]",
	Short: "Diff command output between main and branch repos",
	Long: `Run a command in both the main (parent) repo and the current branch repo,
then display a unified diff of their outputs.

Especially useful for linters and test commands to see how many errors the
branch is adding or removing compared to main.

Use -- to separate repomni flags from the command to run:

  repomni exec diff -- make lint
  repomni exec diff --no-sync -- go vet ./...
  repomni exec diff --name-only -- npm test`,
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,
	RunE:         runExecDiff,
}

func init() {
	execCmd.AddCommand(execDiffCmd)
	execDiffCmd.Flags().BoolVar(&execDiffNoSync, "no-sync", false, "skip fetch+pull on the main repo")
	execDiffCmd.Flags().BoolVar(&execDiffNameOnly, "name-only", false, "only show whether outputs differ")
	execDiffCmd.Flags().StringVar(&execDiffMainDir, "main-dir", "", "explicit path to the main repo")
	execDiffCmd.Flags().BoolVar(&execDiffJSON, "json", false, "output as JSON")
}

func runExecDiff(cmd *cobra.Command, args []string) error {
	// Parse the user command from after --.
	userCmd, err := parseUserCommand(cmd, args)
	if err != nil {
		return err
	}

	// Find current (branch) repo root.
	branchDir, err := gitutil.RunGit(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	// Find main repo directory.
	mainDir, err := resolveMainDir(branchDir)
	if err != nil {
		return err
	}

	// Guard: must not be running from the main repo itself.
	absBranch, _ := filepath.Abs(branchDir)
	absMain, _ := filepath.Abs(mainDir)
	if absBranch == absMain {
		return fmt.Errorf("already in the main repo; run from a branch repo instead")
	}

	// Sync main repo.
	if !execDiffNoSync {
		syncMainRepo(mainDir, execDiffJSON)
	}

	// Run command in both repos.
	if !execDiffJSON {
		fmt.Fprintf(os.Stderr, "Running command in main repo (%s)...\n", filepath.Base(mainDir))
	}
	mainRes := captureCommand(mainDir, userCmd[0], userCmd[1:]...)

	if !execDiffJSON {
		fmt.Fprintf(os.Stderr, "Running command in branch repo (%s)...\n", filepath.Base(branchDir))
	}
	branchRes := captureCommand(branchDir, userCmd[0], userCmd[1:]...)

	if execDiffJSON {
		return outputExecDiffJSON(mainRes, branchRes, mainDir, branchDir, userCmd)
	}

	outcome := compareResults(mainRes, branchRes, execDiffNameOnly)
	if outcome.Stderr != "" {
		fmt.Fprint(os.Stderr, outcome.Stderr)
	}
	if outcome.Stdout != "" {
		fmt.Print(outcome.Stdout)
	}
	if outcome.ExitCode != 0 {
		os.Exit(outcome.ExitCode)
	}
	return nil
}

func outputExecDiffJSON(mainRes, branchRes commandResult, mainDir, branchDir string, userCmd []string) error {
	type cmdOutput struct {
		Dir      string `json:"dir"`
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
		Error    string `json:"error,omitempty"`
	}

	mainErr := ""
	if mainRes.Err != nil {
		mainErr = mainRes.Err.Error()
	}
	branchErr := ""
	if branchRes.Err != nil {
		branchErr = branchRes.Err.Error()
	}

	identical := mainRes.Output == branchRes.Output && mainRes.ExitCode == branchRes.ExitCode && mainRes.Err == nil && branchRes.Err == nil

	diff := ""
	if !identical && mainRes.Err == nil && branchRes.Err == nil {
		diff = diffutil.UnifiedDiff("main", "branch", mainRes.Output, branchRes.Output)
	}

	out := struct {
		Command   string    `json:"command"`
		Identical bool      `json:"identical"`
		Diff      string    `json:"diff,omitempty"`
		Main      cmdOutput `json:"main"`
		Branch    cmdOutput `json:"branch"`
	}{
		Command:   strings.Join(userCmd, " "),
		Identical: identical,
		Diff:      diff,
		Main:      cmdOutput{Dir: mainDir, Output: mainRes.Output, ExitCode: mainRes.ExitCode, Error: mainErr},
		Branch:    cmdOutput{Dir: branchDir, Output: branchRes.Output, ExitCode: branchRes.ExitCode, Error: branchErr},
	}
	if err := ui.PrintJSON(out); err != nil {
		return err
	}
	if !identical {
		os.Exit(1)
	}
	return nil
}

// parseUserCommand extracts the command to run from args after the -- separator.
func parseUserCommand(cmd *cobra.Command, args []string) ([]string, error) {
	dashIdx := cmd.ArgsLenAtDash()
	if dashIdx == -1 {
		return nil, fmt.Errorf("usage: repomni exec diff [flags] -- <command> [args...]\n\nUse -- to separate flags from the command to run")
	}
	userCmd := args[dashIdx:]
	if len(userCmd) == 0 {
		return nil, fmt.Errorf("no command specified after --")
	}
	return userCmd, nil
}

// resolveMainDir determines the main repo directory.
func resolveMainDir(branchDir string) (string, error) {
	if execDiffMainDir != "" {
		if !gitutil.IsGitRepo(execDiffMainDir) {
			return "", fmt.Errorf("--main-dir %q is not a git repository", execDiffMainDir)
		}
		return execDiffMainDir, nil
	}

	parentDir := filepath.Dir(branchDir)
	mainDir, err := brancher.FindParentGitRepo(parentDir)
	if err != nil {
		return "", fmt.Errorf("could not find main repo: %w\nUse --main-dir to specify it explicitly", err)
	}
	return mainDir, nil
}

// syncMainRepo fetches and pulls the main repo, printing warnings on failure.
func syncMainRepo(mainDir string, quiet bool) {
	if !quiet {
		fmt.Fprintf(os.Stderr, "Syncing main repo...\n")
	}
	if err := gitutil.Fetch(mainDir, false); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: fetch failed: %v\n", err)
		}
		return
	}
	if _, err := gitutil.Pull(mainDir, "ff-only", false); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: pull failed: %v (continuing with current state)\n", err)
		}
	}
}

// compareResults decides the outcome of comparing two command executions.
func compareResults(mainRes, branchRes commandResult, nameOnly bool) diffOutcome {
	// If the command failed to start in either repo, report the errors.
	if mainRes.Err != nil || branchRes.Err != nil {
		var stderr string
		if mainRes.Err != nil {
			stderr += fmt.Sprintf("Error in main repo: %v\n", mainRes.Err)
		}
		if branchRes.Err != nil {
			stderr += fmt.Sprintf("Error in branch repo: %v\n", branchRes.Err)
		}
		return diffOutcome{Stderr: stderr, ExitCode: 1}
	}

	if nameOnly {
		if mainRes.Output == branchRes.Output && mainRes.ExitCode != branchRes.ExitCode {
			return diffOutcome{
				Stdout:   fmt.Sprintf("Outputs are identical but exit codes differ (main=%d, branch=%d)\n", mainRes.ExitCode, branchRes.ExitCode),
				ExitCode: 1,
			}
		}
		exitCode := 0
		if mainRes.Output != branchRes.Output {
			exitCode = 1
		}
		return diffOutcome{
			Stdout:   diffutil.SummaryLine(mainRes.Output, branchRes.Output) + "\n",
			ExitCode: exitCode,
		}
	}

	diff := diffutil.UnifiedDiff("main", "branch", mainRes.Output, branchRes.Output)
	if diff == "" && mainRes.ExitCode != branchRes.ExitCode {
		return diffOutcome{
			Stdout:   fmt.Sprintf("Outputs are identical but exit codes differ (main=%d, branch=%d)\n", mainRes.ExitCode, branchRes.ExitCode),
			ExitCode: 1,
		}
	}
	if diff == "" {
		return diffOutcome{Stdout: "Outputs are identical\n", ExitCode: 0}
	}

	return diffOutcome{Stdout: diffutil.ColorDiff(diff), ExitCode: 1}
}

// captureCommand runs a command in dir and returns its combined output and execution status.
// A non-zero exit code is preserved (common for linters) but distinguished from a failure
// to start the process (e.g., missing executable).
func captureCommand(dir string, name string, args ...string) commandResult {
	c := exec.Command(name, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()

	res := commandResult{Output: string(out)}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.Err = err
		}
	}
	return res
}

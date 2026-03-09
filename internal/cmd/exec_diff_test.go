package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExecDiff_NoDash(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	repoDir := filepath.Join(dir, "repo")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repoDir)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	// Simulate no -- by setting ArgsLenAtDash to -1 (default when no -- present).
	execDiffCmd.SetArgs([]string{"echo", "hello"})
	err := runExecDiff(execDiffCmd, []string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error for missing --")
	}
	if !strings.Contains(err.Error(), "Use --") {
		t.Errorf("error should mention --, got: %v", err)
	}
}

func TestRunExecDiff_EmptyCommand(t *testing.T) {
	// ArgsLenAtDash returns -1 when no -- is present, so parseUserCommand returns an error.
	_, err := parseUserCommand(execDiffCmd, []string{})
	if err == nil {
		t.Fatal("expected error for empty args with no --")
	}
}

// --- compareResults unit tests ---

func TestCompareResults_IdenticalOutput(t *testing.T) {
	main := commandResult{Output: "hello\n", ExitCode: 0}
	branch := commandResult{Output: "hello\n", ExitCode: 0}

	outcome := compareResults(main, branch, false)
	if outcome.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "Outputs are identical") {
		t.Errorf("expected identical message, got %q", outcome.Stdout)
	}
}

func TestCompareResults_DifferentOutput(t *testing.T) {
	main := commandResult{Output: "hello\n", ExitCode: 0}
	branch := commandResult{Output: "world\n", ExitCode: 0}

	outcome := compareResults(main, branch, false)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if outcome.Stdout == "" {
		t.Error("expected diff output")
	}
}

func TestCompareResults_MissingExecutable(t *testing.T) {
	main := commandResult{Err: fmt.Errorf("exec: \"nope\": executable file not found in $PATH")}
	branch := commandResult{Output: "hello\n", ExitCode: 0}

	outcome := compareResults(main, branch, false)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stderr, "Error in main repo") {
		t.Errorf("expected error message, got stderr=%q", outcome.Stderr)
	}
}

func TestCompareResults_BothMissingExecutable(t *testing.T) {
	main := commandResult{Err: fmt.Errorf("exec: not found")}
	branch := commandResult{Err: fmt.Errorf("exec: not found")}

	outcome := compareResults(main, branch, false)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stderr, "Error in main repo") {
		t.Errorf("expected main error, got stderr=%q", outcome.Stderr)
	}
	if !strings.Contains(outcome.Stderr, "Error in branch repo") {
		t.Errorf("expected branch error, got stderr=%q", outcome.Stderr)
	}
}

func TestCompareResults_SameOutputDifferentExitCodes(t *testing.T) {
	main := commandResult{Output: "", ExitCode: 0}
	branch := commandResult{Output: "", ExitCode: 1}

	outcome := compareResults(main, branch, false)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "exit codes differ") {
		t.Errorf("expected exit code diff message, got %q", outcome.Stdout)
	}
}

func TestCompareResults_NameOnly_Identical(t *testing.T) {
	main := commandResult{Output: "hello\n", ExitCode: 0}
	branch := commandResult{Output: "hello\n", ExitCode: 0}

	outcome := compareResults(main, branch, true)
	if outcome.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "Outputs are identical") {
		t.Errorf("expected identical message, got %q", outcome.Stdout)
	}
}

func TestCompareResults_NameOnly_Different(t *testing.T) {
	main := commandResult{Output: "hello\n", ExitCode: 0}
	branch := commandResult{Output: "world\n", ExitCode: 0}

	outcome := compareResults(main, branch, true)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "Outputs differ") {
		t.Errorf("expected differ message, got %q", outcome.Stdout)
	}
}

func TestCompareResults_NameOnly_SameOutputDifferentExitCodes(t *testing.T) {
	main := commandResult{Output: "", ExitCode: 0}
	branch := commandResult{Output: "", ExitCode: 1}

	outcome := compareResults(main, branch, true)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "exit codes differ") {
		t.Errorf("expected exit code diff message, got %q", outcome.Stdout)
	}
}

func TestCompareResults_NameOnly_MissingExecutable(t *testing.T) {
	main := commandResult{Err: fmt.Errorf("exec: not found")}
	branch := commandResult{Err: fmt.Errorf("exec: not found")}

	outcome := compareResults(main, branch, true)
	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if outcome.Stderr == "" {
		t.Error("expected error messages for missing executable")
	}
	if outcome.Stdout != "" {
		t.Errorf("expected no stdout for error case, got %q", outcome.Stdout)
	}
}

// --- End-to-end tests: captureCommand + compareResults ---

func TestExecDiff_EndToEnd_IdenticalFiles(t *testing.T) {
	dir := t.TempDir()

	mainDir := filepath.Join(dir, "main")
	os.Mkdir(mainDir, 0755)
	os.WriteFile(filepath.Join(mainDir, "file.txt"), []byte("hello\n"), 0644)

	branchDir := filepath.Join(dir, "branch")
	os.Mkdir(branchDir, 0755)
	os.WriteFile(filepath.Join(branchDir, "file.txt"), []byte("hello\n"), 0644)

	mainRes := captureCommand(mainDir, "cat", "file.txt")
	branchRes := captureCommand(branchDir, "cat", "file.txt")
	outcome := compareResults(mainRes, branchRes, true)

	if outcome.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "Outputs are identical") {
		t.Errorf("expected identical message, got %q", outcome.Stdout)
	}
}

func TestExecDiff_EndToEnd_DifferentFiles(t *testing.T) {
	dir := t.TempDir()

	mainDir := filepath.Join(dir, "main")
	os.Mkdir(mainDir, 0755)
	os.WriteFile(filepath.Join(mainDir, "file.txt"), []byte("hello\n"), 0644)

	branchDir := filepath.Join(dir, "branch")
	os.Mkdir(branchDir, 0755)
	os.WriteFile(filepath.Join(branchDir, "file.txt"), []byte("world\n"), 0644)

	mainRes := captureCommand(mainDir, "cat", "file.txt")
	branchRes := captureCommand(branchDir, "cat", "file.txt")
	outcome := compareResults(mainRes, branchRes, false)

	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if outcome.Stdout == "" {
		t.Error("expected diff output")
	}
}

func TestExecDiff_EndToEnd_MissingExecutable(t *testing.T) {
	dir := t.TempDir()

	mainRes := captureCommand(dir, "definitely-not-a-real-command")
	branchRes := captureCommand(dir, "definitely-not-a-real-command")
	outcome := compareResults(mainRes, branchRes, true)

	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1 for missing executable, got %d", outcome.ExitCode)
	}
	if outcome.Stderr == "" {
		t.Error("expected error messages for missing executable")
	}
}

func TestExecDiff_EndToEnd_SameOutputDifferentExitCodes(t *testing.T) {
	dir := t.TempDir()

	mainRes := captureCommand(dir, "sh", "-c", "exit 0")
	branchRes := captureCommand(dir, "sh", "-c", "exit 1")
	outcome := compareResults(mainRes, branchRes, true)

	if outcome.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", outcome.ExitCode)
	}
	if !strings.Contains(outcome.Stdout, "exit codes differ") {
		t.Errorf("expected exit code diff message, got %q", outcome.Stdout)
	}
}

func TestResolveMainDir_ExplicitDir(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)

	mainDir := filepath.Join(dir, "main")
	if err := os.Mkdir(mainDir, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, mainDir)

	execDiffMainDir = mainDir
	defer func() { execDiffMainDir = "" }()

	got, err := resolveMainDir("/some/branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != mainDir {
		t.Errorf("got %q, want %q", got, mainDir)
	}
}

func TestResolveMainDir_NotARepo(t *testing.T) {
	dir := t.TempDir()

	execDiffMainDir = dir
	defer func() { execDiffMainDir = "" }()

	_, err := resolveMainDir("/some/branch")
	if err == nil {
		t.Fatal("expected error for non-repo directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error should mention not a git repository, got: %v", err)
	}
}

func TestCaptureCommand(t *testing.T) {
	res := captureCommand(".", "echo", "hello")
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if strings.TrimSpace(res.Output) != "hello" {
		t.Errorf("got %q, want %q", strings.TrimSpace(res.Output), "hello")
	}
}

func TestCaptureCommand_NonZeroExit(t *testing.T) {
	res := captureCommand(".", "sh", "-c", "echo error output; exit 1")
	if res.Err != nil {
		t.Fatalf("unexpected start error: %v", res.Err)
	}
	if res.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Output, "error output") {
		t.Errorf("expected output even on non-zero exit, got %q", res.Output)
	}
}

func TestCaptureCommand_MissingExecutable(t *testing.T) {
	res := captureCommand(".", "definitely-not-a-real-command")
	if res.Err == nil {
		t.Fatal("expected error for missing executable")
	}
}

func TestCaptureCommand_DifferentExitCodes(t *testing.T) {
	res0 := captureCommand(".", "sh", "-c", "exit 0")
	res1 := captureCommand(".", "sh", "-c", "exit 1")

	if res0.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res0.ExitCode)
	}
	if res1.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", res1.ExitCode)
	}
	if res0.Output != res1.Output {
		t.Errorf("expected identical output, got %q vs %q", res0.Output, res1.Output)
	}
}

func TestParseUserCommand_NoDash(t *testing.T) {
	// When ArgsLenAtDash returns -1, we should get an error.
	_, err := parseUserCommand(execDiffCmd, []string{"echo"})
	if err == nil {
		t.Fatal("expected error")
	}
}

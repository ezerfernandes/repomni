# Review Findings

## 1. False success when the compared command fails

Severity: High

Files:
- `internal/cmd/exec_diff.go:79`
- `internal/cmd/exec_diff.go:147`

### Problem

`exec diff` ignores command execution errors entirely. `captureCommand` returns only
`CombinedOutput()` and discards the `error`, which means `runExecDiff` compares only
stdout/stderr text and has no visibility into whether the command actually ran
successfully in either repo.

That produces incorrect success cases:

- If the command does not exist in either repo, both executions fail in the same way,
  both outputs are empty, and the command reports `Outputs are identical` with exit
  code `0`.
- If the command succeeds in one repo and fails in the other, but neither side prints
  output, the command still reports `Outputs are identical` with exit code `0`.

This is not just a UX issue; it changes the meaning of the command. The current
implementation can claim there is no difference even when the underlying tool failed
to run or produced different outcomes.

### Reproduction

Observed with the built binary from this branch:

1. Missing binary in both repos

   Command:

   ```sh
   /tmp/repomni-review exec diff --no-sync --name-only -- definitely-not-a-real-command
   ```

   Output:

   ```text
   Running command in main repo (main)...
   Running command in branch repo (feature)...
   Outputs are identical
   ```

   Exit code: `0`

2. Success in main repo, failure in branch repo, no output on either side

   Command:

   ```sh
   /tmp/repomni-review exec diff --no-sync --name-only -- sh -c 'test -f marker'
   ```

   Setup:
- `marker` exists in the main repo
- `marker` does not exist in the branch repo

   Output:

   ```text
   Running command in main repo (main)...
   Running command in branch repo (feature)...
   Outputs are identical
   ```

   Exit code: `0`

### Why it happens

Current implementation:

```go
func captureCommand(dir string, name string, args ...string) string {
	c := exec.Command(name, args...)
	c.Dir = dir
	out, _ := c.CombinedOutput()
	return string(out)
}
```

The command error is discarded, so callers cannot distinguish:

- success with empty output
- failure with empty output
- missing executable
- non-zero exit with diagnostics only in process status

### Expected behavior

The comparison should include execution status, not just output text. At minimum, the
result object should preserve:

- combined output
- whether the process started successfully
- exit code or error

Then `runExecDiff` should treat differing execution status as a diff, and should not
report success when the command failed to execute.

### Suggested fix direction

Replace `captureCommand` with something that returns structured results, for example:

```go
type commandResult struct {
	Output   string
	ExitCode int
	Err      error
}
```

Comparison logic should consider both `Output` and exit status. If the executable is
missing, surface that clearly instead of treating it as an identical output case.

## 2. Tests do not cover the real command behavior

Severity: Medium

Files:
- `internal/cmd/exec_diff_test.go:45`
- `internal/cmd/exec_diff_test.go:85`

### Problem

The new test file passes, but it does not meaningfully validate the end-to-end behavior
of `exec diff`.

The most important tests avoid executing `runExecDiff` through Cobra and instead test
helpers directly:

- `TestRunExecDiff_IdenticalOutput` only compares two `captureCommand` calls
- `TestRunExecDiff_DifferentOutput` only compares two `captureCommand` calls
- the file explicitly notes that it is not exercising the full `ArgsLenAtDash` path

Because of that, the tests miss the actual regression above. The suite is green while
the command still returns a false success for missing executables and status-only
differences.

### Evidence

This comment in the staged tests is the core issue:

```go
// We can't easily test the full runExecDiff with ArgsLenAtDash,
// so test the core helpers instead.
```

That tradeoff is too weak for a command whose correctness depends on Cobra parsing,
repo resolution, process execution, and final exit behavior.

### Expected coverage

The test suite should include black-box or near-black-box coverage for:

- command invoked with `--` and a valid executable
- command invoked without `--`
- identical output with success on both sides
- different output
- executable missing
- same output but different exit codes

### Suggested fix direction

Use one of these approaches:

- execute the built Cobra command in tests and assert on stdout/stderr and exit code
- refactor `runExecDiff` so the core logic is in a pure helper that accepts an
  explicit command result runner and returns a structured result

The important part is that tests must validate the actual decision logic, not only the
output-capture helper.

## Verification performed

- `go test ./...` passed on the staged branch
- Additional black-box runs against a built binary reproduced the incorrect success
  cases described above

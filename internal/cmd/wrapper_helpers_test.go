package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/session"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func captureStdoutAndStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldOut, oldErr := os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
	}()

	fn()

	if err := stdoutW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrW.Close(); err != nil {
		t.Fatal(err)
	}

	stdout, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatal(err)
	}

	return string(stdout), string(stderr)
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func writeJSONL(t *testing.T, path string, entries []map[string]interface{}) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
}

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func prependPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func setupSessionProject(t *testing.T) (string, string) {
	t.Helper()

	home := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	t.Setenv("HOME", home)

	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(projectDir); err == nil {
		projectDir = resolved
	}
	chdirForTest(t, projectDir)

	return home, projectDir
}

func writeClaudeSessionFixture(t *testing.T, home, projectPath, sessionID string, entries []map[string]interface{}) string {
	t.Helper()

	dir := filepath.Join(home, ".claude", "projects", session.EncodePath(projectPath))
	path := filepath.Join(dir, sessionID+".jsonl")
	writeJSONL(t, path, entries)
	return path
}

func writeCodexSessionFixture(t *testing.T, home, projectPath, sessionID, body string) string {
	t.Helper()

	dir := filepath.Join(home, ".codex", "sessions", "2026", "03", "01")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	if body == "" {
		body = fmt.Sprintf(`{"timestamp":"2026-03-01T10:00:00Z","type":"session_meta","payload":{"id":"%s","cwd":%q}}
{"timestamp":"2026-03-01T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Hello from Codex"}]}}
`, sessionID, projectPath)
	}

	path := filepath.Join(dir, fmt.Sprintf("rollout-2026-03-01T10-00-00-%s.jsonl", sessionID))
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func saveGlobalConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
}

func setupSourceDir(t *testing.T, files map[string]string) string {
	t.Helper()

	sourceDir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(sourceDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return sourceDir
}

func resetSessionCommandFlags(t *testing.T) {
	t.Helper()

	oldCLIFilter := sessionCLIFilter
	oldListJSON, oldListLimit := sessionListJSON, sessionListLimit
	oldShowJSON, oldShowLimit, oldShowOffset, oldShowFull := sessionShowJSON, sessionShowLimit, sessionShowOffset, sessionShowFull
	oldSearchJSON, oldSearchMode, oldSearchLimit := sessionSearchJSON, sessionSearchMode, sessionSearchLimit
	oldStatsJSON := sessionStatsJSON
	oldExportOutput, oldExportFull, oldExportNoTools, oldExportJSON := sessionExportOutput, sessionExportFull, sessionExportNoTools, sessionExportJSON
	oldResumeContinue := sessionResumeContinue
	oldCleanJSON, oldCleanDryRun := sessionCleanJSON, sessionCleanDryRun
	oldCleanOlderThan, oldCleanEmpty, oldCleanForce := sessionCleanOlderThan, sessionCleanEmpty, sessionCleanForce

	sessionCLIFilter = ""
	sessionListJSON, sessionListLimit = false, 0
	sessionShowJSON, sessionShowLimit, sessionShowOffset, sessionShowFull = false, 0, 0, false
	sessionSearchJSON, sessionSearchMode, sessionSearchLimit = false, "all", 10
	sessionStatsJSON = false
	sessionExportOutput, sessionExportFull, sessionExportNoTools, sessionExportJSON = "", false, false, false
	sessionResumeContinue = false
	sessionCleanJSON, sessionCleanDryRun = false, false
	sessionCleanOlderThan, sessionCleanEmpty, sessionCleanForce = "", false, false

	t.Cleanup(func() {
		sessionCLIFilter = oldCLIFilter
		sessionListJSON, sessionListLimit = oldListJSON, oldListLimit
		sessionShowJSON, sessionShowLimit, sessionShowOffset, sessionShowFull = oldShowJSON, oldShowLimit, oldShowOffset, oldShowFull
		sessionSearchJSON, sessionSearchMode, sessionSearchLimit = oldSearchJSON, oldSearchMode, oldSearchLimit
		sessionStatsJSON = oldStatsJSON
		sessionExportOutput, sessionExportFull, sessionExportNoTools, sessionExportJSON = oldExportOutput, oldExportFull, oldExportNoTools, oldExportJSON
		sessionResumeContinue = oldResumeContinue
		sessionCleanJSON, sessionCleanDryRun = oldCleanJSON, oldCleanDryRun
		sessionCleanOlderThan, sessionCleanEmpty, sessionCleanForce = oldCleanOlderThan, oldCleanEmpty, oldCleanForce
	})
}

func resetRepoWrapperFlags(t *testing.T) {
	t.Helper()

	oldInjectAll, oldInjectDryRun, oldInjectForce := injectAll, injectDryRun, injectForce
	oldInjectCopy, oldInjectSymlink := injectCopy, injectSymlink
	oldInjectJSON, oldInjectYes := injectJSON, injectYes
	oldEjectAll, oldEjectJSON := ejectAll, ejectJSON
	oldStatusAll, oldStatusJSON, oldStatusGit, oldStatusNoFetch := statusAll, statusJSON, statusGit, statusNoFetch
	oldSyncStateDryRun, oldSyncStateJSON := syncStateDryRun, syncStateJSON

	injectAll, injectDryRun, injectForce = false, false, false
	injectCopy, injectSymlink = false, false
	injectJSON, injectYes = false, false
	ejectAll, ejectJSON = false, false
	statusAll, statusJSON, statusGit, statusNoFetch = false, false, false, false
	syncStateDryRun, syncStateJSON = false, false

	t.Cleanup(func() {
		injectAll, injectDryRun, injectForce = oldInjectAll, oldInjectDryRun, oldInjectForce
		injectCopy, injectSymlink = oldInjectCopy, oldInjectSymlink
		injectJSON, injectYes = oldInjectJSON, oldInjectYes
		ejectAll, ejectJSON = oldEjectAll, oldEjectJSON
		statusAll, statusJSON, statusGit, statusNoFetch = oldStatusAll, oldStatusJSON, oldStatusGit, oldStatusNoFetch
		syncStateDryRun, syncStateJSON = oldSyncStateDryRun, oldSyncStateJSON
	})
}

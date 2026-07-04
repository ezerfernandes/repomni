package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/editor"
)

// stubEditor installs a fake $EDITOR that runs script against the temp file
// (passed as $1) and returns its path. VISUAL is cleared so EDITOR wins.
func stubEditor(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", path)
}

func sampleConfig() config.Config {
	return config.Config{
		Version:   1,
		SourceDir: "/tmp/src",
		Mode:      config.ModeSymlink,
		Items: []config.Item{
			{Type: config.ItemTypeFile, SourcePath: "a", TargetPath: ".a", Enabled: true},
		},
	}
}

func TestEditYAMLConfigAppliesValidEdit(t *testing.T) {
	stubEditor(t, `sed -i 's/^mode: symlink/mode: copy/' "$1"`)

	got, err := editYAMLConfig(sampleConfig(), "# header\n\n")
	if err != nil {
		t.Fatalf("editYAMLConfig() error = %v", err)
	}
	if got.Mode != config.ModeCopy {
		t.Fatalf("Mode = %q, want %q", got.Mode, config.ModeCopy)
	}
}

func TestEditYAMLConfigRejectsUnknownField(t *testing.T) {
	stubEditor(t, `printf '\nbogus_key: 1\n' >> "$1"`)

	_, err := editYAMLConfig(sampleConfig(), "# header\n\n")
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if errors.Is(err, editor.ErrAborted) {
		t.Fatalf("unknown field must be an error, not an abort: %v", err)
	}
}

func TestEditYAMLConfigUnchangedAborts(t *testing.T) {
	stubEditor(t, `true`) // leave the buffer untouched

	_, err := editYAMLConfig(sampleConfig(), "# header\n\n")
	if !errors.Is(err, editor.ErrAborted) {
		t.Fatalf("error = %v, want ErrAborted", err)
	}
}

func TestEditYAMLConfigCommentsOnlyAborts(t *testing.T) {
	stubEditor(t, `printf '# everything deleted but a comment\n' > "$1"`)

	_, err := editYAMLConfig(sampleConfig(), "# header\n\n")
	if !errors.Is(err, editor.ErrAborted) {
		t.Fatalf("error = %v, want ErrAborted for a content-free buffer", err)
	}
}

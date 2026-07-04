package editor

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestResolvePrefersVisual(t *testing.T) {
	t.Setenv("VISUAL", "myvisual")
	t.Setenv("EDITOR", "myeditor")

	got, ok := Resolve()
	if !ok || got != "myvisual" {
		t.Fatalf("Resolve() = (%q, %v), want (\"myvisual\", true)", got, ok)
	}
}

func TestResolveFallsBackToEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "myeditor")

	got, ok := Resolve()
	if !ok || got != "myeditor" {
		t.Fatalf("Resolve() = (%q, %v), want (\"myeditor\", true)", got, ok)
	}
}

func TestResolveIgnoresBlankEnv(t *testing.T) {
	t.Setenv("VISUAL", "   ")
	t.Setenv("EDITOR", "  myeditor  ")

	got, ok := Resolve()
	if !ok || got != "myeditor" {
		t.Fatalf("Resolve() = (%q, %v), want (\"myeditor\", true)", got, ok)
	}
}

func TestResolveVimFallback(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	got, ok := Resolve()

	if _, err := exec.LookPath("vim"); err == nil {
		if !ok || got != "vim" {
			t.Fatalf("Resolve() = (%q, %v), want (\"vim\", true)", got, ok)
		}
	} else {
		if ok {
			t.Fatalf("Resolve() = (%q, %v), want (\"\", false) when vim is absent", got, ok)
		}
	}
}

func TestEditContentRoundTrip(t *testing.T) {
	// Use a stub editor that appends a marker line, exercising EditContent's
	// seed/read round-trip without a real interactive editor.
	stub := writeStubEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", stub)

	got, err := EditContent("original\n", ".sh")
	if err != nil {
		t.Fatalf("EditContent() error = %v", err)
	}
	want := "original\nedited\n"
	if got != want {
		t.Fatalf("EditContent() = %q, want %q", got, want)
	}
}

func TestEditContentOrAbortChanged(t *testing.T) {
	stub := writeStubEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", stub)

	got, err := EditContentOrAbort("original\n", ".sh")
	if err != nil {
		t.Fatalf("EditContentOrAbort() error = %v", err)
	}
	if want := "original\nedited\n"; got != want {
		t.Fatalf("EditContentOrAbort() = %q, want %q", got, want)
	}
}

func TestEditContentOrAbortUnchanged(t *testing.T) {
	// A no-op editor leaves the buffer identical to the seed — the same thing a
	// non-blocking GUI editor does when it returns before the user saves.
	stub := writeNoopEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", stub)

	_, err := EditContentOrAbort("original\n", ".sh")
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("EditContentOrAbort() error = %v, want ErrAborted", err)
	}
}

func TestEditContentOrAbortEmptied(t *testing.T) {
	// Emptying the buffer aborts even though it differs from the seed.
	stub := writeEmptyingEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", stub)

	_, err := EditContentOrAbort("original\n", ".sh")
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("EditContentOrAbort() error = %v, want ErrAborted", err)
	}
}

func TestEnsureBlocking(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"code", "code --wait"},
		{"code-insiders", "code-insiders --wait"},
		{"/usr/local/bin/code", "/usr/local/bin/code --wait"},
		{"subl", "subl --wait"},
		{"code --wait", "code --wait"}, // already blocking
		{"code -w", "code -w"},         // short form respected
		{"code --new-window", "code --new-window --wait"},
		{"vim", "vim"},   // not a GUI editor
		{"nano", "nano"}, // not in the known set
		{"", ""},         // empty
	}
	for _, c := range cases {
		if got := ensureBlocking(c.in); got != c.want {
			t.Errorf("ensureBlocking(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// writeStubEditor writes a tiny shell script that appends "edited" to the file
// it is given, and returns a shell command string that invokes it.
func writeStubEditor(t *testing.T) string {
	t.Helper()
	return writeEditorScript(t, "stub-editor.sh", "#!/bin/sh\nprintf 'edited\\n' >> \"$1\"\n")
}

// writeNoopEditor writes a shell script that leaves its file untouched.
func writeNoopEditor(t *testing.T) string {
	t.Helper()
	return writeEditorScript(t, "noop-editor.sh", "#!/bin/sh\nexit 0\n")
}

// writeEmptyingEditor writes a shell script that truncates its file to empty.
func writeEmptyingEditor(t *testing.T) string {
	t.Helper()
	return writeEditorScript(t, "empty-editor.sh", "#!/bin/sh\n: > \"$1\"\n")
}

func writeEditorScript(t *testing.T, name, script string) string {
	t.Helper()
	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	return "sh " + path
}

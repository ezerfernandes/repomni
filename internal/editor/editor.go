// Package editor launches an external text editor for editing scripts and
// config files, falling back to the caller's in-process TUI when no editor is
// available.
package editor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNoEditor is returned when no external editor can be resolved.
var ErrNoEditor = errors.New("no external editor available")

// ErrAborted is returned by EditContentOrAbort when the editor exits with the
// buffer left unchanged from the seed or containing no non-whitespace content.
// Callers should treat it as a cancellation (mirroring how `git commit` aborts
// on an unchanged or empty message). It also neutralizes non-blocking GUI
// editors, which return the seed untouched instead of blocking for the edit.
var ErrAborted = errors.New("edit aborted (buffer unchanged or empty)")

// guiEditorsNeedingWait maps the base command of common GUI editors to the flag
// that makes them block until the file is closed. Without it these editors fork
// and return immediately, so the file would be read back unedited. Only editors
// known to accept the flag are listed, so appending it is always safe.
var guiEditorsNeedingWait = map[string]string{
	"code":          "--wait",
	"code-insiders": "--wait",
	"codium":        "--wait",
	"subl":          "--wait",
	"sublime_text":  "--wait",
	"atom":          "--wait",
	"mate":          "--wait",
	"zed":           "--wait",
}

// Resolve returns the external editor command to use and whether one is
// available. The preference order is $VISUAL, then $EDITOR, then "vim" on PATH.
// The returned string may contain arguments (e.g. "code --wait").
func Resolve() (string, bool) {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, true
		}
	}
	if _, err := exec.LookPath("vim"); err == nil {
		return "vim", true
	}
	return "", false
}

// Available reports whether an external editor can be launched.
func Available() bool {
	_, ok := Resolve()
	return ok
}

// ensureBlocking appends a wait flag to known GUI editor commands that would
// otherwise fork and return immediately. It never appends to editors not in the
// known set, and never appends a flag that is already present.
func ensureBlocking(cmdStr string) string {
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return cmdStr
	}
	flag, ok := guiEditorsNeedingWait[filepath.Base(fields[0])]
	if !ok {
		return cmdStr
	}
	for _, f := range fields[1:] {
		if f == "--wait" || f == "-w" {
			return cmdStr
		}
	}
	return cmdStr + " " + flag
}

// EditFile opens the resolved editor on path, connected to the terminal, and
// returns once the editor exits. Returns ErrNoEditor if none is available.
func EditFile(path string) error {
	cmdStr, ok := Resolve()
	if !ok {
		return ErrNoEditor
	}
	cmdStr = ensureBlocking(cmdStr)

	// Run through the shell so editor values containing arguments (e.g.
	// "code --wait") are honored. Passing the path as a positional argument
	// keeps it correctly quoted even when it contains spaces.
	c := exec.Command("sh", "-c", cmdStr+` "$@"`, "sh", path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// EditContent opens the editor on a temporary file seeded with initial and
// returns the edited content. The suffix (e.g. ".sh" or ".yaml") is applied to
// the temp file so editors can pick appropriate syntax highlighting. Returns
// ErrNoEditor if no editor is available.
func EditContent(initial, suffix string) (string, error) {
	if !Available() {
		return "", ErrNoEditor
	}

	f, err := os.CreateTemp("", "repomni-*"+suffix)
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer os.Remove(path)

	if _, err := f.WriteString(initial); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	if err := EditFile(path); err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// EditContentOrAbort behaves like EditContent but returns ErrAborted when the
// editor exits with the buffer byte-identical to initial or containing no
// non-whitespace content. This makes "quit without changing" a clean cancel and
// keeps non-blocking GUI editors (which return the seed untouched) from silently
// persisting an unedited buffer.
func EditContentOrAbort(initial, suffix string) (string, error) {
	edited, err := EditContent(initial, suffix)
	if err != nil {
		return "", err
	}
	if edited == initial || strings.TrimSpace(edited) == "" {
		return "", ErrAborted
	}
	return edited, nil
}

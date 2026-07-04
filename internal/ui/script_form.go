package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repomni/internal/editor"
	"github.com/ezerfernandes/repomni/internal/scripter"
)

// newScriptTemplate seeds the external editor for a brand-new setup script so a
// first-time user sees the file's purpose and a shebang instead of a blank
// buffer. Saving it unchanged aborts (see runEditScriptExternal), so the
// boilerplate alone is never persisted as a script.
const newScriptTemplate = `#!/usr/bin/env bash
# repomni setup script — runs after 'branch create' and 'branch clone'.
# Add the commands needed to set up a fresh clone of this repo below.
# Save with no changes (or an empty buffer) to abort without writing a script.
`

// ScriptAction describes what the user chose to do.
type ScriptAction string

const (
	ScriptSaved   ScriptAction = "saved"
	ScriptDeleted ScriptAction = "deleted"
)

// RunScriptForm runs the interactive TUI for managing the setup script.
// Returns the action taken (saved or deleted) on success.
func RunScriptForm(gitDir string) (ScriptAction, error) {
	content, exists := scripter.GetScript(gitDir, scripter.ScriptSetup)

	if exists {
		return runExistingScriptForm(gitDir, content)
	}
	return runEditScriptForm(gitDir, "")
}

func runExistingScriptForm(gitDir string, content string) (ScriptAction, error) {
	var action string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Current Setup Script").
				Description(content),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Edit script", "edit"),
					huh.NewOption("Delete script", "delete"),
					huh.NewOption("Cancel", "cancel"),
				).
				Value(&action),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	switch action {
	case "edit":
		return runEditScriptForm(gitDir, content)
	case "delete":
		return runDeleteScriptForm(gitDir)
	default:
		return "", fmt.Errorf("cancelled by user")
	}
}

func runEditScriptForm(gitDir string, initialContent string) (ScriptAction, error) {
	// Prefer an external editor (vim/$EDITOR) when one is available; fall back
	// to the in-process TUI text field otherwise.
	if editor.Available() {
		return runEditScriptExternal(gitDir, initialContent)
	}

	content := initialContent
	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Setup Script").
				Description("Commands to run when creating a new branch for this repo").
				Value(&content),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save this script?").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	if !confirm {
		return "", fmt.Errorf("cancelled by user")
	}

	if content == "" {
		return "", fmt.Errorf("script content cannot be empty")
	}

	return ScriptSaved, scripter.SaveScript(gitDir, scripter.ScriptSetup, content)
}

// runEditScriptExternal edits the setup script in an external editor. Making a
// change and saving is the confirmation; quitting with the buffer unchanged or
// empty is treated as a cancellation (including when a non-blocking GUI editor
// returns the seed untouched).
func runEditScriptExternal(gitDir string, initialContent string) (ScriptAction, error) {
	seed := initialContent
	if strings.TrimSpace(seed) == "" {
		seed = newScriptTemplate
	}

	content, err := editor.EditContentOrAbort(seed, ".sh")
	if err != nil {
		if errors.Is(err, editor.ErrAborted) {
			return "", fmt.Errorf("cancelled by user")
		}
		return "", fmt.Errorf("edit script: %w", err)
	}

	return ScriptSaved, scripter.SaveScript(gitDir, scripter.ScriptSetup, content)
}

func runDeleteScriptForm(gitDir string) (ScriptAction, error) {
	var confirm bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Delete setup script?").
				Description("This cannot be undone.").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	if !confirm {
		return "", fmt.Errorf("cancelled by user")
	}

	return ScriptDeleted, scripter.DeleteScript(gitDir, scripter.ScriptSetup)
}

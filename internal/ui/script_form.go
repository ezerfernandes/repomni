package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/ezerfernandes/repoinjector/internal/scripter"
)

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

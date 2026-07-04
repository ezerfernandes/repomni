# Configuration

Repomni uses a two-tier configuration system: a global config shared across all repos and optional per-repo overrides stored inside each clone's `.git` directory.

## Global Config

The global configuration lives at `~/.config/repomni/config.yaml` and controls the source directory, injection mode, and which items to inject. Managed by [[internal/config/config.go#Config]].

Key fields:
- **source**: Path to the directory containing shared files (supports `~` and env vars via [[internal/config/config.go#ExpandPath]])
- **mode**: `symlink` (default) or `copy`
- **items**: List of file/directory mappings, each with source path, target path, type, and enabled flag

Default items when no config exists: `.claude/skills` (directory), `.claude/hooks.json` (file), `.envrc` (file), `.env` (file).

## Per-Repo Config

Per-repo configuration lives at `.git/repomni/config.yaml` and lets individual clones override which items and directory entries are active. Managed by [[internal/repoconfig/repoconfig.go#RepoConfig]].

Fields include enabled-items map, workflow state, merge URL, ticket ID, description, and draft status. The `config repo` command edits this config — in an external editor when available (see [[config#External Editor]]), otherwise via an interactive form to select items and entries per clone.

[[internal/repoconfig/repoconfig.go#RepoConfig#FilterGlobalConfig]] merges per-repo overrides with the global config, producing a filtered config that only contains enabled items with selected entries.

## Workflow States

Each branch repo tracks a workflow state stored in per-repo config. States drive the color-coded `branch list` display and determine which repos are checked by `sync state`. Defined in [[internal/repoconfig/state.go]].

Predefined states: `active` (green), `review` (yellow), `approved` (lime), `review-blocked` (red), `merged` (purple), `closed` (red), `paused` (blue). Custom states are also accepted (lowercase letters, digits, hyphens).

Review-related states (`review`, `approved`, `review-blocked`) are the only ones queried by [[sync#State Sync]].

## Interactive Wizard

The `config global` command launches an interactive wizard using the huh library when no external editor is available. It collects source directory (validated for existence), injection mode, and item selection. Implemented in [[internal/ui/settings_form.go#RunSettingsForm]].

Non-interactive mode (`--non-interactive --source <path>`) bypasses both the wizard and the editor for scripted setup.

## External Editor

Script and config editing prefer an external editor when one is available, falling back to the in-process huh TUI otherwise. Editor resolution is `$VISUAL`, then `$EDITOR`, then `vim` on PATH, implemented in [[internal/editor/editor.go#Resolve]].

`config global` and `config repo` open their raw YAML in the editor via [[internal/editor/editor.go#EditContentOrAbort]] — seeded from the current config, or for a fresh per-repo config a template listing every global item. `config repo` shows only the editable injection settings and merges them back over the existing config, so branch workflow state (state, PR/MR URL, ticket) is preserved across edits. Quitting with the buffer left unchanged or empty aborts without writing anything; otherwise the edited YAML is strictly re-parsed — syntax errors and unknown/misspelled keys are rejected before saving so a typo can't silently drop a field, and `config global` additionally rejects an injection mode that is not `symlink` or `copy`. `config script` edits the shell script the same way, seeding a brand-new script with a commented shebang template.

Known GUI editors (VS Code, Sublime, etc.) are launched with a wait flag so they block until the file is closed instead of returning immediately with an unedited buffer; see [[internal/editor/editor.go#EditFile]].

When no editor is resolvable, each command falls back to its original huh form: [[internal/ui/settings_form.go#RunSettingsForm]], [[internal/ui/configure_repo_form.go#RunConfigureRepoForm]], and [[internal/ui/script_form.go#RunScriptForm]].

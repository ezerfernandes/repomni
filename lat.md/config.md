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

Fields include enabled-items map, workflow state, merge URL, ticket ID, description, and draft status. The `config repo` command provides an interactive form to select items and entries per clone.

[[internal/repoconfig/repoconfig.go#RepoConfig#FilterGlobalConfig]] merges per-repo overrides with the global config, producing a filtered config that only contains enabled items with selected entries.

## Workflow States

Each branch repo tracks a workflow state stored in per-repo config. States drive the color-coded `branch list` display and determine which repos are checked by `sync state`. Defined in [[internal/repoconfig/state.go]].

Predefined states: `active` (green), `review` (yellow), `approved` (lime), `review-blocked` (red), `merged` (purple), `closed` (red), `paused` (blue). Custom states are also accepted (lowercase letters, digits, hyphens).

Review-related states (`review`, `approved`, `review-blocked`) are the only ones queried by [[sync#State Sync]].

## Interactive Wizard

The `config global` command launches an interactive wizard using the huh library. It collects source directory (validated for existence), injection mode, and item selection. Implemented in [[internal/ui/settings_form.go#RunSettingsForm]].

Non-interactive mode (`--non-interactive --source <path>`) bypasses the wizard for scripted setup.

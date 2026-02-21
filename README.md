# repoinjector

test

A CLI tool that injects shared configuration files into multiple repository clones, keeping them invisible to git.

When working with multiple branch copies of the same repo (e.g., running parallel AI agents across branches), you need the same `.envrc`, `.env`, Claude skills, and hooks in every clone. Repoinjector symlinks them from a single source directory so you configure once and every clone stays in sync.

## What it does

- Symlinks (or copies) files from a central source into target repos
- Hides injected files from git using `.git/info/exclude` (no tracked files modified)
- Supports batch operations across all repos in a directory
- Track workflow state per branch (`active`, `review`, `done`, `paused`) with color-coded overview
- Interactive TUI for configuration, simple CLI for daily use

### Injected items (defaults)

| Source path | Target path in repo | Type |
|---|---|---|
| `skills/` | `.claude/skills/` | directory |
| `hooks.json` | `.claude/hooks.json` | file |
| `.envrc` | `.envrc` | file |
| `.env` | `.env` | file |

Source paths are configurable via `repoinjector settings`.

## Installation

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- Git

### From source (recommended)

```sh
git clone https://github.com/ezerfernandes/repoinjector.git
cd repoinjector
sudo make install
```

This builds the binary and installs it to `/usr/local/bin/repoinjector`.

To install to a different location:

```sh
make install PREFIX=$HOME/.local
```

This puts the binary in `~/.local/bin/repoinjector` — make sure `~/.local/bin` is in your `PATH`.

### With `go install`

```sh
go install github.com/ezer/repoinjector/cmd/repoinjector@latest
```

This installs to `$GOPATH/bin` (usually `~/go/bin`). Make sure it's in your `PATH`.

### Verify installation

```sh
repoinjector --version
```

### Uninstall

```sh
# If installed with make install
sudo make uninstall

# If installed with go install
rm $(which repoinjector)
```

## Quick start

### 1. Set up your source directory

Create a directory with the files you want to share across repos:

```
my-agent-config/
  skills/
    my-skill.md
  hooks.json
  .envrc
  .env
```

### 2. Configure repoinjector

```sh
repoinjector settings
```

This walks you through an interactive wizard to set the source directory, injection mode, and which items to inject. Configuration is saved to `~/.config/repoinjector/config.yaml`.

For scripted setup:

```sh
repoinjector settings --non-interactive --source /path/to/my-agent-config
```

### 3. Inject into a repo

```sh
cd /path/to/my-repo-clone
repoinjector inject
```

Or inject into all repos under a parent directory:

```sh
repoinjector inject --all /path/to/my-clones
```

### 4. Check status

```sh
repoinjector status --all /path/to/my-clones
```

Output:

```
Repository: /path/to/my-clones/feature-a
  Item                   Present   Current   Excluded
  ─────────────────────  ────────  ────────  ────────
  .claude/skills         Yes       Yes       Yes
  .claude/hooks.json     Yes       Yes       Yes
  .envrc                 Yes       Yes       Yes
  .env                   Yes       Yes       Yes
```

### 5. Clean up when done

```sh
repoinjector eject /path/to/my-clones/feature-a
```

Or eject from all at once:

```sh
repoinjector eject --all /path/to/my-clones
```

## Commands

| Command | Description |
|---|---|
| `settings` | Interactive global setup wizard |
| `configure` | Configure injection settings for the current repository |
| `inject [target]` | Inject files into target repo (default: current dir) |
| `eject [target]` | Remove injected files from target repo |
| `status [target]` | Show injection or git status of target repo(s) |
| `branch <name>` | Clone parent repo and create a new branch |
| `set-state <state>` | Set workflow state for the current branch repo |
| `branches [dir]` | List branch repos with their workflow states |
| `sync [dir]` | Pull updates for all repos in a directory |
| `list [dir]` | List git repos in a directory (for scripting) |
| `script` | Manage per-repo setup scripts |

### `settings`

Interactively configure the source directory, injection mode, and items. Saved to `~/.config/repoinjector/config.yaml`.

```sh
repoinjector settings
repoinjector settings --source ~/shared-config
repoinjector settings --source ~/shared-config --non-interactive
```

| Flag | Description |
|---|---|
| `--source` | Source directory (skip interactive prompt) |
| `--non-interactive` | Use defaults without prompting (requires `--source`) |

### `configure`

Interactively select which items and directory entries to inject into the current repository. Saved to `.git/repoinjector/config.yaml`.

```sh
repoinjector configure
```

### `inject`

Symlink or copy configured files into the target repo. Injected paths are added to `.git/info/exclude`.

```sh
repoinjector inject
repoinjector inject /path/to/repo
repoinjector inject --all
repoinjector inject --dry-run
repoinjector inject --force
```

| Flag | Description |
|---|---|
| `--all` | Inject into all git repos under the target directory |
| `--dry-run` | Show what would be done without making changes |
| `--force` | Overwrite existing regular files |
| `--copy` | Use copy mode for this run |
| `--symlink` | Use symlink mode for this run |

### `eject`

Remove all injected files/symlinks and clean up `.git/info/exclude`.

```sh
repoinjector eject
repoinjector eject --all
```

| Flag | Description |
|---|---|
| `--all` | Eject from all git repos under the target directory |

### `status`

Show injection status or git sync status for target repos.

```sh
repoinjector status
repoinjector status --all
repoinjector status --git
repoinjector status --json
```

| Flag | Description |
|---|---|
| `--all` | Check all git repos under the target directory |
| `--git` | Show git sync status (branch, ahead/behind, dirty) |
| `--no-fetch` | Skip `git fetch` when checking git status |
| `--json` | Output as JSON |

### `branch`

Clone the parent repository and check out a new branch. Automatically injects configured files, runs the setup script, and sets state to `active`.

```sh
repoinjector branch my-feature
repoinjector branch my-feature --no-inject
```

| Flag | Description |
|---|---|
| `--no-inject` | Skip automatic injection into the new branch |

### `set-state`

Set a workflow state for the current repo. Stored in `.git/repoinjector/config.yaml`.

Predefined states: `active`, `review`, `done`, `paused`. Custom states are also accepted (lowercase letters, digits, hyphens).

```sh
repoinjector set-state active
repoinjector set-state review
repoinjector set-state done
repoinjector set-state my-custom-state
repoinjector set-state --clear
```

| Flag | Description |
|---|---|
| `--clear` | Remove the workflow state |

### `branches`

List branch repos with state, git branch, and dirty status. States are color-coded: `active` (green), `review` (yellow), `done` (gray), `paused` (blue).

```sh
repoinjector branches
repoinjector branches /path/to/branches
repoinjector branches --state review
repoinjector branches --json
```

| Flag | Description |
|---|---|
| `--state` | Filter by workflow state |
| `--json` | Output as JSON |

Example output:

```
  Name           Branch          State    Dirty
  ─────────────  ──────────────  ───────  ─────
  feat-auth      feat-auth       active
  fix-bug-123    fix-bug-123     review   *
  old-feature    old-feature     done
```

### `sync`

Fetch and pull updates for all repos under a directory. Dirty repos are skipped unless `--autostash` is used. Diverged repos are always skipped.

```sh
repoinjector sync
repoinjector sync --dry-run
repoinjector sync --autostash
repoinjector sync -j 4
repoinjector sync --strategy rebase
repoinjector sync --json
```

| Flag | Description |
|---|---|
| `--dry-run` | Show what would be done without pulling |
| `--autostash` | Stash dirty working trees before pull |
| `-j`, `--jobs` | Number of parallel sync workers (default: 1) |
| `--no-fetch` | Skip `git fetch` (local status only) |
| `--strategy` | Pull strategy: `ff-only` (default), `rebase`, `merge` |
| `--json` | Output as JSON |

### `list`

List git repositories under a directory, one per line. Useful for piping to other tools.

```sh
repoinjector list
repoinjector list --names
```

| Flag | Description |
|---|---|
| `--names` | Output directory names only instead of full paths |

### `script`

Interactively create or edit a setup script that runs when you create a new branch with `repoinjector branch`. Stored in `.git` and never committed.

```sh
repoinjector script
```

## How it works

- **Symlink mode** (default): Creates symbolic links from target paths to source files. Changes to source are instantly reflected in all targets.
- **Copy mode**: Copies files from source to target. Targets are independent snapshots.
- **Git exclusion**: All injected paths are added to `.git/info/exclude` inside a managed block. This file is local to each clone and never tracked by git, so `git status` stays clean.
- **Worktree support**: Works with both regular git repos and `git worktree` directories.

## Development

```sh
make build    # Build to bin/repoinjector
make test     # Run all tests
make run      # Build and run
make clean    # Remove build artifacts
```

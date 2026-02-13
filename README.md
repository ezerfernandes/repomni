# repoinjector

A CLI tool that injects shared configuration files into multiple repository clones, keeping them invisible to git.

When working with multiple branch copies of the same repo (e.g., running parallel AI agents across branches), you need the same `.envrc`, `.env`, Claude skills, and hooks in every clone. Repoinjector symlinks them from a single source directory so you configure once and every clone stays in sync.

## What it does

- Symlinks (or copies) files from a central source into target repos
- Hides injected files from git using `.git/info/exclude` (no tracked files modified)
- Supports batch operations across all repos in a directory
- Interactive TUI for configuration, simple CLI for daily use

### Injected items (defaults)

| Source path | Target path in repo | Type |
|---|---|---|
| `skills/` | `.claude/skills/` | directory |
| `hooks.json` | `.claude/hooks.json` | file |
| `.envrc` | `.envrc` | file |
| `.env` | `.env` | file |

Source paths are configurable via `repoinjector configure`.

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
repoinjector configure
```

This walks you through an interactive wizard to set the source directory, injection mode, and which items to inject. Configuration is saved to `~/.config/repoinjector/config.yaml`.

For scripted setup:

```sh
repoinjector configure --non-interactive --source /path/to/my-agent-config
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
| `configure` | Interactive setup wizard |
| `inject [target]` | Inject files into target repo (default: current dir) |
| `status [target]` | Show injection state of target repo |
| `eject [target]` | Remove injected files from target repo |

### Common flags

| Flag | Available on | Description |
|---|---|---|
| `--all` | inject, status, eject | Operate on all git repos under target directory |
| `--dry-run` | inject | Show what would be done without making changes |
| `--force` | inject | Overwrite existing regular files |
| `--copy` | inject | Use copy mode instead of symlink for this run |
| `--symlink` | inject | Use symlink mode for this run |
| `--json` | status | Output as JSON |
| `--non-interactive` | configure | Skip interactive prompts, use defaults |
| `--source <dir>` | configure | Set source directory without prompting |

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

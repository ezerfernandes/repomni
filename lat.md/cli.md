# CLI

The command-line interface is built with Cobra and organized into command groups. Entry point is [[main.go#main]] which calls [[internal/cmd/root.go#Execute]].

## Command Tree

The root command is defined in [[internal/cmd/root.go]] with version injection via linker flags (`-X`). Command groups:

- **config**: `config global`, `config repo`, `config script` — configuration wizards
- **inject / eject**: File injection and cleanup
- **status**: Injection status or git sync status (`--git` flag)
- **branch**: Full PR lifecycle — `create`, `clone`, `list`, `set-state`, `submit`, `attach`, `checks`, `open`, `ready`, `review`, `merge`, `clean`
- **sync**: `sync`, `sync code`, `sync state` — pull updates and track PR/MR status
- **exec**: `exec diff` — compare command output between main and branch repos
- **session**: `list`, `show`, `search`, `export`, `resume`, `stats`, `clean`
- **list**: Utility to list git repos in a directory

## Common Patterns

Most commands follow these conventions:
- `--all` flag for batch operations across sibling repos
- `--json` flag for machine-readable output via [[internal/ui/json.go]]
- `--dry-run` flag for preview without side effects
- Default target is current working directory
- Git repo validation before operations

## Exec Diff

Runs a command in both the main repo and the current branch repo, then diffs their stdout. Useful for comparing lint or test results.

Implemented in [[internal/cmd/exec_diff.go]], with diff rendering via [[internal/diffutil/diffutil.go#UnifiedDiff]]. Character-level highlighting shows exactly what changed between outputs.

## UI Layer

Interactive forms use the huh library. Table output uses lipgloss for styling. Color mapping for workflow states is centralized in [[internal/ui/colors.go]]. All table renderers live in `internal/ui/` with `_table.go` suffix.

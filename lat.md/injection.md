# Injection

The injection system is the core mechanism of repomni. It places shared files into target repos via symlinks or copies, tracks what was injected for safe cleanup, and hides injected paths from git.

## Inject Operation

[[internal/injector/injector.go#Inject]] processes each enabled item from the config, creating symlinks or copies at the target paths. It accumulates results per item and continues on failure.

Key behaviors:
- Parent directories are created automatically
- Symlinks use absolute paths and are replaced atomically (temp file + rename)
- Directory items create per-entry symlinks rather than symlinking the whole directory, enabling selective entry injection
- Missing source items are skipped (not errors)
- Environment files (`.env`, `.envrc`) search parent directories instead of the source directory
- Path validation rejects traversal attempts (`../` escapes)

## Eject Operation

[[internal/injector/injector.go#Eject]] reads the [[injection#Manifest Tracking|manifest]] and removes every recorded item. Empty parent directories (e.g., `.claude/`) are cleaned up afterward. The managed block in `.git/info/exclude` is also removed.

## Manifest Tracking

The injector records every injected path in `.git/repomni/manifest.json` via [[internal/injector/manifest.go]]. Each entry stores the target path, source path, injection mode, and (for directories) selected entries.

The manifest makes eject reliable regardless of config changes — it always knows exactly what was injected. Status checks also use the manifest to verify currency.

## Git Exclusion

All injected target paths are written to `.git/info/exclude` inside a managed block delimited by `# BEGIN repomni managed block` / `# END repomni managed block`. Managed by [[internal/injector/exclude.go#UpdateExclude]].

The block is replaced atomically on each inject, preserving all user-written exclusions outside the markers. On eject, the entire managed block is removed.

## Status Check

[[internal/injector/injector.go#Status]] reports each item's presence, currency (symlink points to correct source), and exclusion state. This drives the `status` command's table output.

## Batch Operations

The `--all` flag discovers all immediate-subdirectory git repos via [[internal/gitutil/gitutil.go#FindGitRepos]] and injects/ejects/checks each one. No nested discovery — only depth-1 subdirectories.

# Architecture

Go CLI that injects shared config files into multiple repo clones, keeping them invisible to git. Targets parallel AI-agent workflows across branch clones.

## Package Layering

The `cmd` layer orchestrates via Cobra, `ui` renders forms and tables, domain packages implement logic. All git operations funnel through `gitutil`.

```
cmd/                      CLI commands (Cobra)
├── ui/                   TUI forms, tables, colors (huh, lipgloss)
│
├── injector/             Inject/eject files
├── syncer/               Git fetch/pull across repos
├── brancher/             Branch creation and cloning
├── mergestatus/          PR/MR state queries
├── session/              Claude Code / Codex session analysis
├── diffutil/             Unified diff with character highlighting
├── scripter/             Per-repo setup scripts
│
├── config/               Global config (~/.config/repomni/config.yaml)
├── repoconfig/           Per-repo config (.git/repomni/config.yaml)
├── forge/                GitHub/GitLab CLI abstraction
│
└── gitutil/              Low-level git wrappers (foundational)
```

## State Locations

Repomni persists state in several locations, none of which are tracked by git.

| What | Where | Format |
|---|---|---|
| Global config | `~/.config/repomni/config.yaml` | YAML |
| Per-repo config | `.git/repomni/config.yaml` | YAML |
| Injection manifest | `.git/repomni/manifest.json` | JSON |
| Git exclusions | `.git/info/exclude` (managed block) | Text |
| Setup scripts | `.git/repomni/scripts/setup.sh` | Shell |
| Clean archive | `.repomni-archive.json` (parent dir) | JSON |

## Key Design Decisions

Core architectural choices that shape how repomni works.

### Symlinks over copies

Symlink mode is the default because changes to the source directory propagate instantly to all clones. Copy mode exists for environments where symlinks are impractical (e.g., Docker volumes).

### Git-invisible injection

All injected paths go into `.git/info/exclude` inside a managed block delimited by `BEGIN/END repomni managed block` markers. This file is local to each clone and never committed, so `git status` stays clean. See [[injection#Git Exclusion]].

### Manifest-based idempotency

The injector records every injected path in `.git/repomni/manifest.json`. Eject reads the manifest rather than re-deriving paths, which makes cleanup reliable even if the global config changes between inject and eject. See [[injection#Manifest Tracking]].

### Credential sanitization

Every git and forge CLI call redacts `user:password@` patterns from error messages via [[internal/gitutil/gitutil.go#sanitizeStderr]]. This prevents accidental exposure of tokens in logs or terminal output.

### Buffer pooling

[[internal/gitutil/gitutil.go#RunGit]] uses `sync.Pool` for stdout/stderr buffers, reducing GC pressure when running many git commands in parallel during sync operations.

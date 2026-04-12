# Forge

The forge package abstracts the differences between GitHub and GitLab CLIs (`gh` and `glab`), providing a unified interface for PR/MR operations throughout repomni.

## Platform Detection

[[internal/forge/forge.go#DetectPlatformFromRemote]] determines the platform from the git remote URL. URLs containing `github.com` map to GitHub; everything else maps to GitLab. [[internal/mergestatus/mergestatus.go#DetectPlatform]] does the same from a merge URL.

## CLI Execution

Three execution modes via [[internal/forge/forge.go]]:
- `RunForge()`: Run `gh`/`glab` command globally
- `RunForgeDir()`: Run in a specific directory (variable for test injection)
- `RunForgePassthrough()`: Run with terminal I/O attached (for interactive commands)

All modes sanitize credentials from error output. `CheckCLI()` verifies the chosen CLI is available before operations.

## State Mapping

[[internal/mergestatus/mergestatus.go#QueryMergeStatus]] queries the platform and maps native states to repomni [[config#Workflow States|workflow states]]:

**GitHub** (via `gh pr view`):
- MERGED → `merged`, CLOSED → `closed`
- OPEN + APPROVED review → `approved`
- OPEN + failing checks → `review-blocked`
- OPEN otherwise → `review`

**GitLab** (via `glab mr view`):
- merged → `merged`, closed → `closed`
- opened + approved → `approved`
- opened otherwise → `review`

Results are returned as [[internal/mergestatus/mergestatus.go#Result]] with previous/new state and changed flag, aggregated into [[internal/mergestatus/mergestatus.go#Summary]].

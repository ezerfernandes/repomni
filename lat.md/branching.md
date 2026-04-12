# Branching

The branch subsystem manages the lifecycle of parallel branch clones — from creation through PR submission to cleanup. It is the primary workflow for running multiple AI agents on separate branches of the same repo.

## Branch Creation

[[internal/brancher/brancher.go#Branch]] clones the parent repository (found by walking up the directory tree) into a sibling directory and creates a new local branch. On success, the command auto-injects configured files and runs the setup script.

[[internal/brancher/brancher.go#Clone]] does the same but checks out an existing remote branch instead of creating a new one. Directory name is derived from the branch name with slashes replaced by hyphens.

Both operations validate branch names against git ref-format rules and filesystem constraints via [[internal/brancher/brancher.go#ValidateBranchName]]. On failure, partially cloned directories are cleaned up.

## PR/MR Submission

`branch submit` pushes the current branch and creates a pull request (GitHub) or merge request (GitLab). Platform is detected from the origin remote URL via [[internal/forge/forge.go#DetectPlatformFromRemote]]. After creation, the PR/MR URL is stored in per-repo config and state is set to `review`.

Related commands: `branch attach` (link existing PR/MR), `branch checks` (CI status), `branch open` (browser), `branch ready` (undraft), `branch review` (approve/comment), `branch merge`.

## Branch Cleanup

`branch clean` finds repos in terminal states and removes them after archiving metadata and ejecting injected files.

Default terminal states are `merged` and `closed` (configurable via `--state`). Before deletion, metadata is archived to `.repomni-archive.json` in the parent directory. Repos with uncommitted changes are skipped unless `--force`.

## Setup Scripts

Per-repo setup scripts stored at `.git/repomni/scripts/setup.sh` run automatically after `branch create` and `branch clone`. Managed by [[internal/scripter/scripter.go]]. The `config script` command provides an interactive editor.

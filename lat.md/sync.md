# Sync

The sync system keeps branch clones up to date with their remotes and tracks PR/MR status changes. It operates in two phases that can run independently or together.

## Code Sync

[[internal/syncer/syncer.go#SyncAll]] fetches and pulls updates for all repos under a directory. Each repo is classified by [[internal/syncer/syncer.go#CheckStatus]] into one of: `current`, `behind`, `ahead`, `diverged`, `dirty`, `no-upstream`, or `error`.

Pull strategies:
- **ff-only** (default): Fast-forward only, safest
- **rebase**: Rebase local commits onto upstream
- **merge**: Regular merge commit

Dirty repos are skipped unless `--autostash` is set. Diverged repos are always skipped (requires manual resolution). The `--jobs` flag controls parallelism via `errgroup`.

### Git Optimization

Status detection uses a single `git status -b --porcelain=v2` call per repo to extract branch, upstream, ahead/behind counts, and dirty state simultaneously. Fetch uses optimized settings (`gc.auto=0`, `negotiationAlgorithm=skipping`) via [[internal/gitutil/gitutil.go#Fetch]].

## State Sync

[[internal/mergestatus/mergestatus.go#QueryMergeStatus]] queries GitHub/GitLab for PR/MR status and maps results to repomni workflow states. Only repos with a stored merge URL and a review-related state are checked. See [[forge#State Mapping]] for platform-specific mapping rules.

When the queried state differs from the stored state, the per-repo config is updated automatically (unless `--dry-run`).

## Umbrella Command

`repomni sync` runs code sync followed by state sync on the same directory. Errors from either phase are accumulated.

Flags are forwarded appropriately: `--dry-run` and `--json` go to both phases, sync-specific flags (`--autostash`, `--jobs`, `--strategy`) go to code sync only.

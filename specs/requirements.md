# Repomni Requirements (EARS Notation)

## 1. Configuration

### REQ-CFG-001: Configuration persistence
The system shall store configuration in `~/.config/repomni/config.yaml` using YAML format.

### REQ-CFG-002: Default configuration
When no configuration file exists, the system shall use symlink mode with all default items enabled (.claude/skills, .claude/hooks.json, .envrc, .env).

### REQ-CFG-003: Configuration schema
The system shall persist the following in the configuration: version number, source directory path, injection mode, and a list of items each having a type, source path, target path, and enabled flag.

### REQ-CFG-004: Item types
The system shall support two item types: `file` for single files and `directory` for entire directories.

### REQ-CFG-005: Injection modes
The system shall support two injection modes: `symlink` and `copy`.

## 2. Settings Command

### REQ-CONF-001: Interactive wizard
When the user runs `repomni config global`, the system shall present an interactive multi-step form collecting: source directory path, injection mode, items to inject, and a confirmation prompt.

### REQ-CONF-002: Source directory validation
When the user provides a source directory during configuration, the system shall validate that the path exists and is a directory.

### REQ-CONF-003: Non-interactive mode
Where the `--non-interactive` flag is provided, the system shall skip the interactive form and use defaults.

### REQ-CONF-004: Non-interactive source requirement
If the `--non-interactive` flag is provided without `--source`, then the system shall return an error.

### REQ-CONF-005: Source flag override
Where the `--source` flag is provided, the system shall use its value as the source directory, skipping the source directory prompt.

### REQ-CONF-006: Existing config as defaults
When a configuration file already exists, the system shall load it and use its values as defaults in the interactive form.

## 3. Inject Command

### REQ-INJ-001: Default target
When the user runs `repomni inject` without a target argument, the system shall use the current working directory as the target.

### REQ-INJ-002: Explicit target
When the user runs `repomni inject <target>`, the system shall use the provided path as the target directory.

### REQ-INJ-003: Git repository validation
If the target directory is not a git repository, then the system shall return an error.

### REQ-INJ-004: Self-injection prevention
If the source and target directories resolve to the same path, then the system shall return an error.

### REQ-INJ-005: Symlink injection
While the injection mode is `symlink`, the system shall create symbolic links from target paths to source paths using absolute paths.

### REQ-INJ-006: Copy injection
While the injection mode is `copy`, the system shall copy files and directories from source to target, preserving file permissions.

### REQ-INJ-007: Directory copy
While the injection mode is `copy` and the item type is `directory`, the system shall recursively copy all files within the source directory to the target directory.

### REQ-INJ-008: Parent directory creation
When an item's target path requires parent directories that do not exist, the system shall create them.

### REQ-INJ-009: Git exclusion
When items are injected into a target repository, the system shall add all injected target paths to a managed block in `.git/info/exclude`.

### REQ-INJ-010: Managed block markers
The system shall delimit the managed block in `.git/info/exclude` with `# BEGIN repomni managed block` and `# END repomni managed block` markers.

### REQ-INJ-011: Preserve existing excludes
When updating `.git/info/exclude`, the system shall preserve all content outside the managed block.

### REQ-INJ-012: Idempotent symlinks
When a symlink already exists and points to the correct source, the system shall skip it and report "already up to date".

### REQ-INJ-013: Stale symlink replacement
When a symlink exists but points to a different source, the system shall remove it and create a new symlink to the correct source.

### REQ-INJ-014: Regular file conflict
If a regular file or directory exists at the target path and the `--force` flag is not set, then the system shall skip the item and report the conflict.

### REQ-INJ-015: Force overwrite
Where the `--force` flag is provided, the system shall remove existing regular files or directories at target paths before injecting.

### REQ-INJ-016: Idempotent copy
While the injection mode is `copy` and a target file already has identical content to the source, the system shall skip it and report "already up to date".

### REQ-INJ-017: Missing source tolerance
If a source file or directory does not exist for an enabled item, then the system shall skip that item and continue processing remaining items.

### REQ-INJ-018: Error accumulation
When an item fails to inject, the system shall continue processing remaining items and report all errors at the end.

### REQ-INJ-019: Dry run
Where the `--dry-run` flag is provided, the system shall report what would be done without creating any files, symlinks, or modifying `.git/info/exclude`.

### REQ-INJ-020: Batch injection
Where the `--all` flag is provided, the system shall discover all git repositories as immediate subdirectories (depth 1) of the target directory and inject into each.

### REQ-INJ-021: No repos found
If the `--all` flag is provided and no git repositories are found under the target directory, then the system shall return an error.

### REQ-INJ-022: Mode override flags
Where the `--copy` or `--symlink` flag is provided, the system shall override the configured injection mode for that run.

### REQ-INJ-023: Only enabled items
The system shall only inject items whose `enabled` field is `true` in the configuration.

### REQ-INJ-024: Managed block idempotency
When updating `.git/info/exclude`, the system shall replace the existing managed block rather than appending a duplicate.

### REQ-INJ-025: Config required
If no configuration file exists when running `inject`, then the system shall return an error suggesting the user run `repomni config global`.

## 4. Status Command

### REQ-STA-001: Status table
When the user runs `repomni status`, the system shall display a table showing each enabled item's target path, presence, currency, and exclusion state.

### REQ-STA-002: Presence check
The system shall report an item as "present" if a file, directory, or symlink exists at its target path.

### REQ-STA-003: Currency check
While an item is present and is a symlink, the system shall report it as "current" if the symlink points to the expected source path.

### REQ-STA-004: Exclusion check
The system shall report an item as "excluded" if its target path appears in the managed block of `.git/info/exclude`.

### REQ-STA-005: Batch status
Where the `--all` flag is provided, the system shall display a status table for each git repository found as an immediate subdirectory of the target directory.

### REQ-STA-006: JSON output
Where the `--json` flag is provided, the system shall output status information as JSON to stdout.

### REQ-STA-007: Default target
When the user runs `repomni status` without a target argument, the system shall use the current working directory as the target.

## 5. Eject Command

### REQ-EJE-001: Symlink removal
When the user runs `repomni eject`, the system shall remove all injected symlinks from the target repository.

### REQ-EJE-002: File removal
When injected items are regular files or directories (from copy mode), the system shall remove them from the target repository.

### REQ-EJE-003: Exclude cleanup
When ejecting from a target repository, the system shall remove the managed block from `.git/info/exclude`.

### REQ-EJE-004: Parent directory cleanup
When ejecting leaves an empty parent directory (e.g., `.claude/`), the system shall remove that empty directory.

### REQ-EJE-005: Missing item tolerance
If an injected item is not present in the target during eject, then the system shall skip it and continue processing remaining items.

### REQ-EJE-006: Batch eject
Where the `--all` flag is provided, the system shall eject from all git repositories found as immediate subdirectories of the target directory.

### REQ-EJE-007: Default target
When the user runs `repomni eject` without a target argument, the system shall use the current working directory as the target.

## 6. Git Compatibility

### REQ-GIT-001: Standard repository support
The system shall detect git repositories by the presence of a `.git` directory.

### REQ-GIT-002: Worktree support
When the target repository is a git worktree (`.git` is a file containing a `gitdir:` pointer), the system shall resolve the actual git directory and write to the correct `.git/info/exclude` location.

### REQ-GIT-003: No tracked file modification
The system shall not modify any git-tracked file in the target repository. All exclusion patterns shall be written to `.git/info/exclude` only.

## 7. Output

### REQ-OUT-001: Action reporting
The system shall report each item's result with an action indicator: `[ok]` for created/updated/removed, `[--]` for skipped/dry-run, and `[!!]` for errors.

### REQ-OUT-002: Summary line
When injection, ejection, or status completes, the system shall print a summary line with counts of changed, skipped, and errored items.

### REQ-OUT-003: Exit code
If any item encounters an error during injection or ejection, then the system shall exit with a non-zero exit code.

## 8. Sync-Code Command

### REQ-SYC-001: Default target
When the user runs `repomni sync-code` without a directory argument, the system shall use the current working directory as the target.

### REQ-SYC-002: Repository discovery
The system shall discover all git repositories as immediate subdirectories of the target directory.

### REQ-SYC-003: No repos found
If no git repositories are found under the target directory, then the system shall return an error.

### REQ-SYC-004: Fetch and pull
The system shall run `git fetch` followed by `git pull` for each discovered repository.

### REQ-SYC-005: Dirty working tree skip
When a repository has a dirty working tree and `--autostash` is not set, the system shall skip it.

### REQ-SYC-006: Autostash
Where the `--autostash` flag is provided, the system shall stash dirty working trees before pulling.

### REQ-SYC-007: Diverged skip
When a repository has diverged from its upstream, the system shall always skip it.

### REQ-SYC-008: Pull strategy
The system shall support three pull strategies via the `--strategy` flag: `ff-only` (default), `rebase`, and `merge`.

### REQ-SYC-009: Parallel workers
Where the `-j` or `--jobs` flag is provided, the system shall run the specified number of sync workers in parallel.

### REQ-SYC-010: No fetch
Where the `--no-fetch` flag is provided, the system shall skip `git fetch` and check local status only.

### REQ-SYC-011: Dry run
Where the `--dry-run` flag is provided, the system shall report what would be done without pulling.

### REQ-SYC-012: JSON output
Where the `--json` flag is provided, the system shall output sync results as JSON to stdout.

### REQ-SYC-013: Summary line
When sync completes, the system shall print a summary with counts of pulled, current, skipped, conflicts, and errors.

### REQ-SYC-014: Error exit code
If any repository encounters an error during sync, then the system shall exit with a non-zero exit code.

### REQ-SYC-015: Conflict warning
If any repository has conflicts requiring manual resolution, the system shall print a warning to stderr.

## 9. Sync-State Command

### REQ-SYS-001: Default target
When the user runs `repomni sync-state` without a directory argument, the system shall use the current working directory as the target.

### REQ-SYS-002: Repository discovery
The system shall discover all git repositories as immediate subdirectories of the target directory.

### REQ-SYS-003: No repos found
If no git repositories are found under the target directory, then the system shall return an error.

### REQ-SYS-004: Review state filter
The system shall only check repositories with a stored merge URL and a review-related state (`review`, `approved`, `review-blocked`).

### REQ-SYS-005: Platform detection
The system shall detect the platform from the merge URL: `github.com` URLs use the `gh` CLI, all other URLs use the `glab` CLI.

### REQ-SYS-006: GitHub state mapping
When querying GitHub, the system shall map PR states as follows: MERGED to `merged`, CLOSED to `closed`, OPEN with APPROVED review decision to `approved`, OPEN with failing checks to `review-blocked`, and OPEN otherwise to `review`.

### REQ-SYS-007: GitLab state mapping
When querying GitLab, the system shall map MR states as follows: merged to `merged`, closed to `closed`, opened with approved=true to `approved`, and opened otherwise to `review`.

### REQ-SYS-008: State update persistence
When the queried state differs from the stored state and `--dry-run` is not set, the system shall save the new state to the repository config.

### REQ-SYS-009: No active merge requests
If no repositories with active merge requests are found, the system shall print a message and exit successfully.

### REQ-SYS-010: Dry run
Where the `--dry-run` flag is provided, the system shall report what would change without updating configs.

### REQ-SYS-011: JSON output
Where the `--json` flag is provided, the system shall output sync-state results as JSON to stdout.

### REQ-SYS-012: Summary line
When sync-state completes, the system shall print a summary with counts of updated, unchanged, and errors.

## 10. Sync Command (Umbrella)

### REQ-SYN-001: Combined execution
When the user runs `repomni sync`, the system shall execute `sync-code` followed by `sync-state` on the same target directory.

### REQ-SYN-002: Default target
When the user runs `repomni sync` without a directory argument, the system shall use the current working directory as the target.

### REQ-SYN-003: Flag forwarding
The system shall forward `--dry-run` and `--json` to both `sync-code` and `sync-state`, and forward `--autostash`, `--jobs`, `--no-fetch`, and `--strategy` to `sync-code`.

### REQ-SYN-004: Error accumulation
If either `sync-code` or `sync-state` returns an error, the system shall continue executing the other and report all errors at the end.

### REQ-SYN-005: Error exit code
If either sub-command encounters an error, the system shall exit with a non-zero exit code.

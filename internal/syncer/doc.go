// Package syncer checks and synchronizes the git state of multiple
// repositories. It fetches upstream changes, computes ahead/behind counts,
// and optionally pulls updates with configurable strategies (fast-forward,
// rebase, or merge) and auto-stash support.
package syncer

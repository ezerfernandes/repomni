// Package forge provides utilities for running GitHub CLI (gh) and GitLab CLI (glab) commands.
// It abstracts platform differences behind a common interface, allowing callers
// to execute forge commands, detect the hosting platform from a remote URL,
// and parse PR/MR numbers without coupling to a specific provider.
package forge

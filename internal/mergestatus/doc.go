// Package mergestatus queries GitHub and GitLab for the current state of pull
// requests and merge requests. It maps platform-specific states (open,
// merged, closed, approved) to repoinjector workflow states, using the gh and
// glab CLIs respectively.
package mergestatus

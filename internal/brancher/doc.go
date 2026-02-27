// Package brancher creates branch-based clones of a git repository. It locates
// the nearest parent git repo, clones it into the working directory, and
// checks out either a new or existing branch. Branch names are validated
// against git ref-format rules and filesystem constraints.
package brancher

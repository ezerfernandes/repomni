// Package scripter manages per-repo setup scripts stored inside the git
// directory at .git/repoinjector/scripts/. Scripts are shell files that run
// automatically after branch creation to perform project-specific setup such
// as installing dependencies.
package scripter

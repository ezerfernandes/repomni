// Package injector manages the injection and ejection of configuration files
// into target git repositories. It supports symlink and copy modes, merges
// directory entries without clobbering existing repo files, and maintains a
// managed block in .git/info/exclude so that injected paths stay invisible to
// git.
package injector

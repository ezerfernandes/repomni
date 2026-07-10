package commitfmt

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var prefixRe = regexp.MustCompile(`(?i)^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([^)]*\))?:\s*`)

// Normalize standardizes a conventional-commit subject line:
//   - trims whitespace
//   - lowercases conventional commit prefixes (with optional scope)
//   - capitalizes the first letter of the description after the prefix
//   - removes trailing periods
//   - truncates to 72 characters (with ellipsis)
func Normalize(subject string) string {
	s := strings.TrimSpace(subject)
	if s == "" {
		return ""
	}

	loc := prefixRe.FindStringIndex(s)
	if loc != nil {
		prefix := strings.ToLower(s[:loc[1]])
		desc := s[loc[1]:]
		desc = capitalizeFirst(desc)
		s = prefix + desc
	}

	s = strings.TrimRight(s, ".")

	if utf8.RuneCountInString(s) > 72 {
		runes := []rune(s)
		s = string(runes[:69]) + "..."
	}

	return s
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

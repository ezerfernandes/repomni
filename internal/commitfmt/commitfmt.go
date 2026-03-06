package commitfmt

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxSubjectLen is the maximum length for a normalized commit subject.
const MaxSubjectLen = 72

// knownPrefixes lists conventional commit types that get lowercased.
var knownPrefixes = []string{
	"feat", "fix", "docs", "style", "refactor",
	"perf", "test", "build", "ci", "chore", "revert",
}

// Normalize standardizes a commit message subject line:
//   - trims whitespace
//   - lowercases conventional commit prefixes (e.g. FIX: -> fix:)
//   - capitalizes the first letter of the description
//   - removes a trailing period
//   - truncates to MaxSubjectLen
func Normalize(subject string) string {
	s := strings.TrimSpace(subject)
	if s == "" {
		return ""
	}

	s = normalizePrefix(s)
	s = strings.TrimSuffix(s, ".")
	if len(s) > MaxSubjectLen {
		s = s[:MaxSubjectLen]
	}
	return s
}

// normalizePrefix lowercases known conventional-commit prefixes and
// capitalizes the first letter of the description after the prefix.
func normalizePrefix(s string) string {
	for _, prefix := range knownPrefixes {
		// Check for "PREFIX:" or "PREFIX(scope):" case-insensitively.
		if len(s) <= len(prefix) {
			continue
		}
		candidate := s[:len(prefix)]
		rest := s[len(prefix):]

		if !strings.EqualFold(candidate, prefix) {
			continue
		}

		// Must be followed by '(' (scoped) or ':'
		if rest[0] != ':' && rest[0] != '(' {
			continue
		}

		var colonIdx int
		if rest[0] == '(' {
			ci := strings.Index(rest, "):")
			if ci < 0 {
				continue
			}
			colonIdx = ci + 1
		}

		beforeDesc := strings.ToLower(candidate) + rest[:colonIdx+1]
		desc := strings.TrimSpace(rest[colonIdx+1:])
		desc = capitalizeFirst(desc)
		if desc == "" {
			return beforeDesc
		}
		return beforeDesc + " " + desc
	}

	// No conventional prefix — just capitalize first letter.
	return capitalizeFirst(s)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if unicode.IsUpper(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

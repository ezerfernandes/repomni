package commitnorm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxSubjectLen is the maximum allowed length for a commit subject line.
const MaxSubjectLen = 72

// Normalize cleans up a raw commit message:
//   - Strips lines starting with '#' (git comment lines)
//   - Trims leading/trailing blank lines
//   - Capitalizes the first letter of the subject
//   - Removes a trailing period from the subject
//   - Truncates the subject to MaxSubjectLen characters
//   - Ensures a blank line between subject and body (if body exists)
func Normalize(msg string) string {
	lines := strings.Split(msg, "\n")

	// Strip comment lines.
	var filtered []string
	for _, l := range lines {
		if !strings.HasPrefix(strings.TrimLeft(l, " \t"), "#") {
			filtered = append(filtered, l)
		}
	}
	lines = filtered

	// Trim leading blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	// Trim trailing blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return ""
	}

	// Normalize subject line.
	subject := strings.TrimSpace(lines[0])
	subject = capitalize(subject)
	subject = strings.TrimRight(subject, ".")
	subject = truncate(subject, MaxSubjectLen)

	if len(lines) == 1 {
		return subject + "\n"
	}

	// Build body: everything after the subject.
	body := lines[1:]

	// Ensure blank line between subject and body.
	if len(body) > 0 && strings.TrimSpace(body[0]) != "" {
		body = append([]string{""}, body...)
	}

	var b strings.Builder
	b.WriteString(subject)
	b.WriteByte('\n')
	for _, l := range body {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// capitalize upper-cases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// truncate returns s trimmed to at most maxLen characters. If truncation
// happens, it tries to break at the last space to avoid splitting a word.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := s[:maxLen]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut
}

package commitfmt

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain subject", "add user login", "add user login"},
		{"plain with trailing period", "add user login.", "add user login"},
		{"whitespace trimming", "  add user login  ", "add user login"},
		{"empty input", "", ""},
		{"only whitespace", "   ", ""},
		{"feat prefix lowercase", "feat: add login", "feat: Add login"},
		{"feat prefix mixed case", "Feat: add login", "feat: Add login"},
		{"fix prefix uppercase", "FIX: resolve crash", "fix: Resolve crash"},
		{"docs prefix", "DOCS: update readme", "docs: Update readme"},
		{"chore prefix", "Chore: bump deps", "chore: Bump deps"},
		{"revert prefix", "Revert: bad commit", "revert: Bad commit"},
		{"scoped prefix", "fix(auth): resolve token bug", "fix(auth): Resolve token bug"},
		{"scoped prefix mixed case", "Fix(Auth): resolve token bug", "fix(auth): Resolve token bug"},
		{"prefix with trailing period", "feat: add login.", "feat: Add login"},
		{"already normalized", "feat: Add login", "feat: Add login"},
		{"exactly 72 chars", "abcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijAB", "abcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijAB"},
		{"73 chars truncated", "abcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABC", "abcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghi..."},
		{"long with prefix", "feat: " + "abcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABC", "feat: AbcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabcdefghijABCDEFGHIJabc..."},
		{"build prefix", "BUILD: update dockerfile", "build: Update dockerfile"},
		{"ci prefix", "CI: add pipeline", "ci: Add pipeline"},
		{"perf prefix", "PERF: optimize query", "perf: Optimize query"},
		{"style prefix", "Style: fix formatting", "style: Fix formatting"},
		{"refactor prefix", "REFACTOR: extract method", "refactor: Extract method"},
		{"test prefix", "Test: add unit tests", "test: Add unit tests"},
		{"not a prefix", "feature: not a real prefix", "feature: not a real prefix"},
		{"multiple periods", "feat: add login...", "feat: Add login"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.in)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

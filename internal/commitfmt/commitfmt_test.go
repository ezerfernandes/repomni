package commitfmt

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
		{name: "trim whitespace", in: "  hello world  ", want: "Hello world"},
		{name: "already correct", in: "fix: Add login validation", want: "fix: Add login validation"},
		{name: "uppercase prefix", in: "FIX: add login validation", want: "fix: Add login validation"},
		{name: "mixed case prefix", in: "Feat: add new feature", want: "feat: Add new feature"},
		{name: "capitalize description", in: "docs: update readme", want: "docs: Update readme"},
		{name: "trailing period", in: "fix: Resolve bug.", want: "fix: Resolve bug"},
		{name: "scoped prefix", in: "FIX(auth): handle timeout", want: "fix(auth): Handle timeout"},
		{name: "no prefix", in: "update readme file", want: "Update readme file"},
		{name: "no prefix already capitalized", in: "Update readme file", want: "Update readme file"},
		{name: "trailing period no prefix", in: "Update readme.", want: "Update readme"},
		{name: "prefix only no description", in: "fix:", want: "fix:"},
		{name: "chore prefix", in: "CHORE: bump deps", want: "chore: Bump deps"},
		{name: "revert prefix", in: "Revert: undo changes", want: "revert: Undo changes"},
		{name: "truncate long message", in: "feat: " + string(make([]byte, 80)), want: "feat: " + string(make([]byte, 80)[:66])},
		{name: "ci prefix", in: "CI: update pipeline", want: "ci: Update pipeline"},
		{name: "perf prefix", in: "PERF: optimize query", want: "perf: Optimize query"},
		{name: "build prefix", in: "BUILD: add docker", want: "build: Add docker"},
		{name: "test prefix", in: "TEST: add unit tests", want: "test: Add unit tests"},
		{name: "style prefix", in: "STYLE: format code", want: "style: Format code"},
		{name: "refactor prefix", in: "REFACTOR: simplify logic", want: "refactor: Simplify logic"},
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

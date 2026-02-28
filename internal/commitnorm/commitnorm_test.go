package commitnorm

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple subject",
			in:   "fix bug\n",
			want: "Fix bug\n",
		},
		{
			name: "strips trailing period",
			in:   "Fix bug.\n",
			want: "Fix bug\n",
		},
		{
			name: "capitalizes first letter",
			in:   "add feature\n",
			want: "Add feature\n",
		},
		{
			name: "already capitalized",
			in:   "Add feature\n",
			want: "Add feature\n",
		},
		{
			name: "strips comment lines",
			in:   "Fix bug\n# This is a comment\n# Another comment\n",
			want: "Fix bug\n",
		},
		{
			name: "strips comment lines with leading whitespace",
			in:   "Fix bug\n  # indented comment\n",
			want: "Fix bug\n",
		},
		{
			name: "subject and body",
			in:   "Fix bug\n\nThis fixes the thing.\n",
			want: "Fix bug\n\nThis fixes the thing.\n",
		},
		{
			name: "inserts blank line between subject and body",
			in:   "Fix bug\nThis is the body.\n",
			want: "Fix bug\n\nThis is the body.\n",
		},
		{
			name: "trims leading blank lines",
			in:   "\n\nFix bug\n",
			want: "Fix bug\n",
		},
		{
			name: "trims trailing blank lines",
			in:   "Fix bug\n\n\n",
			want: "Fix bug\n",
		},
		{
			name: "trims subject whitespace",
			in:   "  Fix bug  \n",
			want: "Fix bug\n",
		},
		{
			name: "empty message",
			in:   "",
			want: "",
		},
		{
			name: "only comments",
			in:   "# comment 1\n# comment 2\n",
			want: "",
		},
		{
			name: "only whitespace",
			in:   "  \n  \n",
			want: "",
		},
		{
			name: "long subject truncated at word boundary",
			in:   "This is a very long commit message subject line that goes well beyond the seventy-two character limit\n",
			want: "This is a very long commit message subject line that goes well beyond\n",
		},
		{
			name: "subject with body and comments mixed",
			in: "fix authentication bug.\n" +
				"# Please enter the commit message for your changes.\n" +
				"# Lines starting with '#' will be ignored.\n" +
				"\n" +
				"The login endpoint was not checking token expiration.\n",
			want: "Fix authentication bug\n" +
				"\n" +
				"The login endpoint was not checking token expiration.\n",
		},
		{
			name: "preserves body formatting",
			in: "Fix bug\n\n" +
				"First paragraph.\n\n" +
				"Second paragraph.\n",
			want: "Fix bug\n\n" +
				"First paragraph.\n\n" +
				"Second paragraph.\n",
		},
		{
			name: "no trailing newline in input",
			in:   "fix bug",
			want: "Fix bug\n",
		},
		{
			name: "subject exactly 72 chars",
			in:   "This is exactly seventy-two characters long, padded to fill the spacing!\n",
			want: "This is exactly seventy-two characters long, padded to fill the spacing!\n",
		},
		{
			name: "multiple trailing periods",
			in:   "Fix bug...\n",
			want: "Fix bug\n",
		},
		{
			name: "body with comments interleaved",
			in: "Fix bug\n\n" +
				"Details here.\n" +
				"# comment in body\n" +
				"More details.\n",
			want: "Fix bug\n\n" +
				"Details here.\n" +
				"More details.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.in)
			if got != tt.want {
				t.Errorf("Normalize(%q)\n got: %q\nwant: %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"", ""},
		{"123abc", "123abc"},
	}
	for _, tt := range tests {
		got := capitalize(tt.in)
		if got != tt.want {
			t.Errorf("capitalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"short", 72, "short"},
		{"this is a longer string that needs truncation at a word", 30, "this is a longer string that"},
		{"nospaces-in-this-very-long-string-at-all", 10, "nospaces-i"},
	}
	for _, tt := range tests {
		got := truncate(tt.in, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
		}
	}
}

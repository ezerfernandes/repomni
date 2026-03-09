package cmd

import "testing"

func TestMapGHState(t *testing.T) {
	tests := []struct {
		name           string
		state          string
		reviewDecision string
		want           string
	}{
		{"merged", "MERGED", "", "merged"},
		{"closed", "CLOSED", "", "closed"},
		{"open no review", "OPEN", "", "review"},
		{"open approved", "OPEN", "APPROVED", "approved"},
		{"open changes requested", "OPEN", "CHANGES_REQUESTED", "review"},
		{"open review required", "OPEN", "REVIEW_REQUIRED", "review"},
		{"unknown state", "UNKNOWN", "", "review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGHState(tt.state, tt.reviewDecision)
			if got != tt.want {
				t.Errorf("mapGHState(%q, %q) = %q, want %q", tt.state, tt.reviewDecision, got, tt.want)
			}
		})
	}
}

func TestMapGLState(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		approved bool
		want     string
	}{
		{"merged", "merged", false, "merged"},
		{"closed", "closed", false, "closed"},
		{"opened not approved", "opened", false, "review"},
		{"opened approved", "opened", true, "approved"},
		{"unknown state", "unknown", false, "review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGLState(tt.state, tt.approved)
			if got != tt.want {
				t.Errorf("mapGLState(%q, %v) = %q, want %q", tt.state, tt.approved, got, tt.want)
			}
		})
	}
}

func TestRunAttach_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		current bool
		wantErr string
	}{
		{
			name:    "no args no current",
			args:    []string{},
			current: false,
			wantErr: "provide a PR/MR URL, or use --current to auto-discover",
		},
		{
			name:    "current with url",
			args:    []string{"https://github.com/org/repo/pull/1"},
			current: true,
			wantErr: "--current and a URL are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCurrent := attachCurrent
			defer func() { attachCurrent = origCurrent }()
			attachCurrent = tt.current

			err := runAttach(attachCmd, tt.args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

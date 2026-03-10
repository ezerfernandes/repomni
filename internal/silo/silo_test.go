package silo

import (
	"testing"
)

const mockGitLog = `Alice

src/main.go
src/utils.go

Bob

src/main.go
src/api.go

Alice

src/main.go
src/utils.go

Alice

src/utils.go

Bob

src/api.go

Bob

src/api.go
`

func TestParseGitLogOutput(t *testing.T) {
	files := ParseGitLogOutput(mockGitLog)

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	// src/main.go: Alice=2, Bob=1 → 2 contributors
	if fi, ok := files["src/main.go"]; !ok {
		t.Fatal("missing src/main.go")
	} else {
		if fi.contributors["Alice"] != 2 {
			t.Errorf("src/main.go Alice commits: got %d, want 2", fi.contributors["Alice"])
		}
		if fi.contributors["Bob"] != 1 {
			t.Errorf("src/main.go Bob commits: got %d, want 1", fi.contributors["Bob"])
		}
	}

	// src/utils.go: Alice=3 → 1 contributor (silo)
	if fi, ok := files["src/utils.go"]; !ok {
		t.Fatal("missing src/utils.go")
	} else {
		if len(fi.contributors) != 1 {
			t.Errorf("src/utils.go contributors: got %d, want 1", len(fi.contributors))
		}
		if fi.contributors["Alice"] != 3 {
			t.Errorf("src/utils.go Alice commits: got %d, want 3", fi.contributors["Alice"])
		}
	}

	// src/api.go: Bob=3 → 1 contributor (silo)
	if fi, ok := files["src/api.go"]; !ok {
		t.Fatal("missing src/api.go")
	} else {
		if len(fi.contributors) != 1 {
			t.Errorf("src/api.go contributors: got %d, want 1", len(fi.contributors))
		}
		if fi.contributors["Bob"] != 3 {
			t.Errorf("src/api.go Bob commits: got %d, want 3", fi.contributors["Bob"])
		}
	}
}

func TestAnalyzeFiles(t *testing.T) {
	files := ParseGitLogOutput(mockGitLog)
	opts := Options{MinCommits: 3, Threshold: 1}
	silos := analyzeFiles(files, opts)

	if len(silos) != 2 {
		t.Fatalf("expected 2 silos, got %d", len(silos))
	}

	siloMap := make(map[string]FileSilo)
	for _, s := range silos {
		siloMap[s.Path] = s
	}

	utils, ok := siloMap["src/utils.go"]
	if !ok {
		t.Fatal("missing src/utils.go in silos")
	}
	if utils.Owner != "Alice" {
		t.Errorf("src/utils.go owner: got %q, want Alice", utils.Owner)
	}
	if utils.Commits != 3 {
		t.Errorf("src/utils.go commits: got %d, want 3", utils.Commits)
	}
	if utils.BusFactor != 1 {
		t.Errorf("src/utils.go bus factor: got %d, want 1", utils.BusFactor)
	}

	api, ok := siloMap["src/api.go"]
	if !ok {
		t.Fatal("missing src/api.go in silos")
	}
	if api.Owner != "Bob" {
		t.Errorf("src/api.go owner: got %q, want Bob", api.Owner)
	}
}

func TestAnalyzeFilesMinCommitsFilter(t *testing.T) {
	files := ParseGitLogOutput(mockGitLog)
	opts := Options{MinCommits: 5, Threshold: 1}
	silos := analyzeFiles(files, opts)

	if len(silos) != 0 {
		t.Errorf("expected 0 silos with min-commits=5, got %d", len(silos))
	}
}

func TestAnalyzeFilesThresholdFilter(t *testing.T) {
	files := ParseGitLogOutput(mockGitLog)
	opts := Options{MinCommits: 3, Threshold: 2}
	silos := analyzeFiles(files, opts)

	// With threshold=2, src/main.go (2 contributors, 3 commits) is also included
	if len(silos) != 3 {
		t.Errorf("expected 3 silos with threshold=2, got %d", len(silos))
	}
}

func TestAnalyzeDirs(t *testing.T) {
	files := ParseGitLogOutput(mockGitLog)
	opts := Options{MinCommits: 3, Threshold: 1}
	silos := analyzeDirs(files, opts)

	// src/ has Alice + Bob → 2 contributors, not a silo with threshold=1
	if len(silos) != 0 {
		t.Errorf("expected 0 dir silos, got %d", len(silos))
	}
}

func TestRiskLevel(t *testing.T) {
	tests := []struct {
		busFactor int
		commits   int
		want      string
	}{
		{1, 10, "high"},
		{1, 15, "high"},
		{1, 3, "medium"},
		{1, 9, "medium"},
		{2, 20, "low"},
	}
	for _, tt := range tests {
		got := riskLevel(tt.busFactor, tt.commits)
		if got != tt.want {
			t.Errorf("riskLevel(%d, %d) = %q, want %q", tt.busFactor, tt.commits, got, tt.want)
		}
	}
}

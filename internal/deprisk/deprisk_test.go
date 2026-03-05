package deprisk

import (
	"testing"
	"time"
)

func TestParseGoListOutput(t *testing.T) {
	input := []byte(`{
	"Path": "example.com/main",
	"Main": true,
	"GoVersion": "1.21"
}
{
	"Path": "github.com/foo/bar",
	"Version": "v1.2.0",
	"Time": "2023-01-15T00:00:00Z",
	"Update": {
		"Path": "github.com/foo/bar",
		"Version": "v1.5.0",
		"Time": "2024-06-15T00:00:00Z"
	},
	"Indirect": false
}
{
	"Path": "github.com/baz/qux",
	"Version": "v0.3.1",
	"Indirect": true
}`)

	modules, err := ParseGoListOutput(input)
	if err != nil {
		t.Fatalf("ParseGoListOutput: %v", err)
	}
	if len(modules) != 3 {
		t.Fatalf("expected 3 modules, got %d", len(modules))
	}

	if !modules[0].Main {
		t.Error("expected first module to be Main")
	}

	m := modules[1]
	if m.Path != "github.com/foo/bar" {
		t.Errorf("expected foo/bar, got %s", m.Path)
	}
	if m.Version != "v1.2.0" {
		t.Errorf("expected v1.2.0, got %s", m.Version)
	}
	if m.Update == nil {
		t.Fatal("expected Update to be non-nil")
	}
	if m.Update.Version != "v1.5.0" {
		t.Errorf("expected update v1.5.0, got %s", m.Update.Version)
	}

	if !modules[2].Indirect {
		t.Error("expected third module to be indirect")
	}
}

func TestParseGovulncheckOutput(t *testing.T) {
	// Test line-delimited format (govulncheck v1)
	input := []byte(`{"config": {}}
{"vulnerability": {"osv": {"id": "GO-2023-0001", "summary": "Test vuln"}, "modules": [{"path": "github.com/foo/bar"}]}}
{"vulnerability": {"osv": {"id": "GO-2023-0002", "summary": "Another vuln"}, "modules": [{"path": "github.com/foo/bar"}, {"path": "github.com/baz/qux"}]}}
`)

	result := ParseGovulncheckOutput(input)

	fooVulns := result["github.com/foo/bar"]
	if len(fooVulns) != 2 {
		t.Fatalf("expected 2 vulns for foo/bar, got %d", len(fooVulns))
	}
	if fooVulns[0].ID != "GO-2023-0001" {
		t.Errorf("expected GO-2023-0001, got %s", fooVulns[0].ID)
	}

	bazVulns := result["github.com/baz/qux"]
	if len(bazVulns) != 1 {
		t.Fatalf("expected 1 vuln for baz/qux, got %d", len(bazVulns))
	}
}

func TestComputeRiskScore(t *testing.T) {
	tests := []struct {
		name     string
		dep      DepRisk
		wantMin  float64
		wantMax  float64
	}{
		{
			name:    "no risk",
			dep:     DepRisk{},
			wantMin: 0,
			wantMax: 0.1,
		},
		{
			name:    "outdated only",
			dep:     DepRisk{MonthsBehind: 12},
			wantMin: 2,
			wantMax: 6,
		},
		{
			name: "vuln only",
			dep: DepRisk{
				Vulnerabilities: []VulnInfo{{ID: "GO-2023-0001"}},
			},
			wantMin: 2,
			wantMax: 4,
		},
		{
			name: "both outdated and vulns",
			dep: DepRisk{
				MonthsBehind:    24,
				Vulnerabilities: []VulnInfo{{ID: "GO-2023-0001"}, {ID: "GO-2023-0002"}},
			},
			wantMin: 5,
			wantMax: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ComputeRiskScore(tt.dep)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("ComputeRiskScore() = %f, want between %f and %f", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  RiskLevel
	}{
		{0, RiskLow},
		{1.5, RiskLow},
		{2, RiskMedium},
		{3.9, RiskMedium},
		{4, RiskHigh},
		{6.9, RiskHigh},
		{7, RiskCritical},
		{10, RiskCritical},
	}
	for _, tt := range tests {
		got := ScoreToLevel(tt.score)
		if got != tt.want {
			t.Errorf("ScoreToLevel(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFilterByMinRisk(t *testing.T) {
	deps := []DepRisk{
		{Path: "a", RiskScore: 1},
		{Path: "b", RiskScore: 3},
		{Path: "c", RiskScore: 5},
		{Path: "d", RiskScore: 8},
	}

	filtered := FilterByMinRisk(deps, RiskHigh)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(filtered))
	}
	if filtered[0].Path != "c" || filtered[1].Path != "d" {
		t.Errorf("unexpected filter results: %v", filtered)
	}
}

func TestMonthsBetween(t *testing.T) {
	a := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	b := time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)

	got := monthsBetween(a, b)
	if got != 18 {
		t.Errorf("monthsBetween = %d, want 18", got)
	}

	// Order shouldn't matter
	got2 := monthsBetween(b, a)
	if got2 != 18 {
		t.Errorf("monthsBetween reversed = %d, want 18", got2)
	}
}

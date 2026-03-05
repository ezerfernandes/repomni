// Package deprisk scans Go module dependencies for security and maintenance risks.
package deprisk

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// RiskLevel categorizes a dependency's overall risk.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// DepRisk holds the risk assessment for a single dependency.
type DepRisk struct {
	Path           string    `json:"path"`
	Version        string    `json:"version"`
	LatestVersion  string    `json:"latest_version,omitempty"`
	UpdateAvail    bool      `json:"update_available"`
	MonthsBehind   int       `json:"months_behind"`
	Vulnerabilities []VulnInfo `json:"vulnerabilities,omitempty"`
	RiskScore      float64   `json:"risk_score"`
	Risk           RiskLevel `json:"risk"`
	Indirect       bool      `json:"indirect"`
}

// VulnInfo holds info about a known vulnerability.
type VulnInfo struct {
	ID      string `json:"id"`
	Summary string `json:"summary,omitempty"`
}

// goListModule is the JSON shape returned by `go list -m -json -u`.
type goListModule struct {
	Path      string        `json:"Path"`
	Version   string        `json:"Version"`
	Update    *goListUpdate `json:"Update,omitempty"`
	Indirect  bool          `json:"Indirect"`
	Time      *time.Time    `json:"Time,omitempty"`
	Main      bool          `json:"Main"`
	GoVersion string        `json:"GoVersion,omitempty"`
}

type goListUpdate struct {
	Path    string     `json:"Path"`
	Version string     `json:"Version"`
	Time    *time.Time `json:"Time,omitempty"`
}

// govulncheckOutput is a simplified view of govulncheck JSON output.
type govulncheckVuln struct {
	OSV struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"osv"`
	Modules []struct {
		Path string `json:"path"`
	} `json:"modules"`
}

type govulncheckOutput struct {
	Vulns []govulncheckVuln `json:"vulns"`
}

// ScanResult is the output of a full scan.
type ScanResult struct {
	RepoPath string    `json:"repo_path"`
	Deps     []DepRisk `json:"deps"`
	Error    string    `json:"error,omitempty"`
}

// Scan runs dependency risk analysis on the Go module at dir.
func Scan(dir string) (*ScanResult, error) {
	result := &ScanResult{RepoPath: dir}

	modules, err := runGoList(dir)
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	vulns := runGovulncheck(dir)

	for _, mod := range modules {
		if mod.Main {
			continue
		}
		dep := assessModule(mod, vulns)
		result.Deps = append(result.Deps, dep)
	}

	sort.Slice(result.Deps, func(i, j int) bool {
		return result.Deps[i].RiskScore > result.Deps[j].RiskScore
	})

	return result, nil
}

func runGoList(dir string) ([]goListModule, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "-u", "all")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseGoListOutput(out)
}

// ParseGoListOutput parses the concatenated JSON objects from `go list -m -json`.
func ParseGoListOutput(data []byte) ([]goListModule, error) {
	var modules []goListModule
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var m goListModule
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("decode module: %w", err)
		}
		modules = append(modules, m)
	}
	return modules, nil
}

func runGovulncheck(dir string) map[string][]VulnInfo {
	result := make(map[string][]VulnInfo)

	path, err := exec.LookPath("govulncheck")
	if err != nil {
		return result
	}

	cmd := exec.Command(path, "-json", "./...")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// govulncheck exits non-zero when vulns are found; try parsing anyway
		if out == nil {
			return result
		}
	}

	parsed := ParseGovulncheckOutput(out)
	for modPath, vulns := range parsed {
		result[modPath] = vulns
	}
	return result
}

// ParseGovulncheckOutput parses govulncheck JSON and returns vulns keyed by module path.
func ParseGovulncheckOutput(data []byte) map[string][]VulnInfo {
	result := make(map[string][]VulnInfo)

	var output govulncheckOutput
	if err := json.Unmarshal(data, &output); err != nil {
		// Try line-delimited JSON messages (govulncheck v1 format)
		return parseGovulncheckMessages(data)
	}

	for _, v := range output.Vulns {
		info := VulnInfo{ID: v.OSV.ID, Summary: v.OSV.Summary}
		for _, m := range v.Modules {
			result[m.Path] = append(result[m.Path], info)
		}
	}
	return result
}

// govulncheck v1 emits line-delimited JSON messages with different types.
type govulncheckMessage struct {
	Vulnerability *struct {
		OSV struct {
			ID      string `json:"id"`
			Summary string `json:"summary"`
		} `json:"osv"`
		Modules []struct {
			Path string `json:"path"`
		} `json:"modules"`
	} `json:"vulnerability,omitempty"`
}

func parseGovulncheckMessages(data []byte) map[string][]VulnInfo {
	result := make(map[string][]VulnInfo)
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var msg govulncheckMessage
		if err := dec.Decode(&msg); err != nil {
			break
		}
		if msg.Vulnerability == nil {
			continue
		}
		v := msg.Vulnerability
		info := VulnInfo{ID: v.OSV.ID, Summary: v.OSV.Summary}
		for _, m := range v.Modules {
			result[m.Path] = append(result[m.Path], info)
		}
	}
	return result
}

func assessModule(mod goListModule, vulns map[string][]VulnInfo) DepRisk {
	dep := DepRisk{
		Path:     mod.Path,
		Version:  mod.Version,
		Indirect: mod.Indirect,
	}

	if mod.Update != nil {
		dep.UpdateAvail = true
		dep.LatestVersion = mod.Update.Version
		if mod.Time != nil && mod.Update.Time != nil {
			months := monthsBetween(*mod.Time, *mod.Update.Time)
			dep.MonthsBehind = months
		}
	}

	if v, ok := vulns[mod.Path]; ok {
		dep.Vulnerabilities = v
	}

	dep.RiskScore = ComputeRiskScore(dep)
	dep.Risk = ScoreToLevel(dep.RiskScore)
	return dep
}

func monthsBetween(a, b time.Time) int {
	if b.Before(a) {
		a, b = b, a
	}
	months := (b.Year()-a.Year())*12 + int(b.Month()-a.Month())
	if months < 0 {
		return 0
	}
	return months
}

// ComputeRiskScore calculates a 0-10 risk score for a dependency.
func ComputeRiskScore(dep DepRisk) float64 {
	score := 0.0

	// Outdatedness: up to 5 points, logarithmic scale
	if dep.MonthsBehind > 0 {
		score += math.Min(5.0, math.Log2(float64(dep.MonthsBehind)+1)*1.5)
	}

	// Vulnerabilities: 3 points per vuln, up to 5 points
	vulnCount := len(dep.Vulnerabilities)
	if vulnCount > 0 {
		score += math.Min(5.0, float64(vulnCount)*3.0)
	}

	return math.Min(10.0, math.Round(score*10)/10)
}

// ScoreToLevel converts a numeric risk score to a risk level.
func ScoreToLevel(score float64) RiskLevel {
	switch {
	case score >= 7:
		return RiskCritical
	case score >= 4:
		return RiskHigh
	case score >= 2:
		return RiskMedium
	default:
		return RiskLow
	}
}

// FilterByMinRisk filters deps to only those at or above the given level.
func FilterByMinRisk(deps []DepRisk, minLevel RiskLevel) []DepRisk {
	minScore := levelToMinScore(minLevel)
	var filtered []DepRisk
	for _, d := range deps {
		if d.RiskScore >= minScore {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

func levelToMinScore(level RiskLevel) float64 {
	switch level {
	case RiskCritical:
		return 7
	case RiskHigh:
		return 4
	case RiskMedium:
		return 2
	case RiskLow:
		return 0
	default:
		return 0
	}
}

package silo

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FileSilo represents a file or directory that is a knowledge silo.
type FileSilo struct {
	Path      string `json:"path"`
	Owner     string `json:"owner"`
	Commits   int    `json:"commits"`
	BusFactor int    `json:"bus_factor"`
	IsDir     bool   `json:"is_dir"`
	RiskLevel string `json:"risk_level"`
}

// Summary holds aggregate statistics about the silo analysis.
type Summary struct {
	TotalFiles int    `json:"total_files"`
	TotalSilos int    `json:"total_silos"`
	MostAtRisk string `json:"most_at_risk"`
	MaxCommits int    `json:"max_commits"`
}

// Result holds the complete analysis output.
type Result struct {
	Silos   []FileSilo `json:"silos"`
	Summary Summary    `json:"summary"`
}

// Options configures the silo analysis.
type Options struct {
	MinCommits int  // minimum commits for a file to be considered
	Threshold  int  // max unique contributors to count as silo
	ByDir      bool // aggregate by directory
}

// fileInfo tracks contributors and commit counts per file.
type fileInfo struct {
	contributors map[string]int // contributor -> commit count
}

// Analyze runs the silo detection on the given git repository path.
func Analyze(repoPath string, opts Options) (*Result, error) {
	if opts.MinCommits <= 0 {
		opts.MinCommits = 3
	}
	if opts.Threshold <= 0 {
		opts.Threshold = 1
	}

	files, err := parseGitLog(repoPath)
	if err != nil {
		return nil, err
	}

	var silos []FileSilo

	if opts.ByDir {
		silos = analyzeDirs(files, opts)
	} else {
		silos = analyzeFiles(files, opts)
	}

	sort.Slice(silos, func(i, j int) bool {
		if silos[i].BusFactor != silos[j].BusFactor {
			return silos[i].BusFactor < silos[j].BusFactor
		}
		return silos[i].Commits > silos[j].Commits
	})

	summary := Summary{
		TotalFiles: len(files),
		TotalSilos: len(silos),
	}
	if len(silos) > 0 {
		summary.MostAtRisk = silos[0].Path
		summary.MaxCommits = silos[0].Commits
	}

	return &Result{Silos: silos, Summary: summary}, nil
}

// parseGitLog runs git log and builds a map of file -> contributor info.
func parseGitLog(repoPath string) (map[string]*fileInfo, error) {
	cmd := exec.Command("git", "log", "--format=%aN", "--name-only")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return ParseGitLogOutput(string(out)), nil
}

// ParseGitLogOutput parses raw git log output into file contributor data.
// Exported for testing.
func ParseGitLogOutput(output string) map[string]*fileInfo {
	files := make(map[string]*fileInfo)
	scanner := bufio.NewScanner(strings.NewReader(output))

	var currentAuthor string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Lines that don't contain path separators and don't look like file paths
		// are author names. File paths typically contain / or have extensions.
		if !strings.Contains(line, "/") && !strings.Contains(line, ".") {
			currentAuthor = line
			continue
		}
		if currentAuthor == "" {
			continue
		}
		// This is a file path
		fi, ok := files[line]
		if !ok {
			fi = &fileInfo{contributors: make(map[string]int)}
			files[line] = fi
		}
		fi.contributors[currentAuthor]++
	}
	return files
}

func analyzeFiles(files map[string]*fileInfo, opts Options) []FileSilo {
	var silos []FileSilo
	for path, fi := range files {
		totalCommits := 0
		for _, c := range fi.contributors {
			totalCommits += c
		}
		if totalCommits < opts.MinCommits {
			continue
		}
		busFactor := len(fi.contributors)
		if busFactor > opts.Threshold {
			continue
		}
		owner := topContributor(fi.contributors)
		silos = append(silos, FileSilo{
			Path:      path,
			Owner:     owner,
			Commits:   totalCommits,
			BusFactor: busFactor,
			RiskLevel: riskLevel(busFactor, totalCommits),
		})
	}
	return silos
}

func analyzeDirs(files map[string]*fileInfo, opts Options) []FileSilo {
	// Aggregate by top-level directory
	dirs := make(map[string]*fileInfo)
	for path, fi := range files {
		dir := filepath.Dir(path)
		if dir == "." {
			dir = "/"
		}
		d, ok := dirs[dir]
		if !ok {
			d = &fileInfo{contributors: make(map[string]int)}
			dirs[dir] = d
		}
		for author, count := range fi.contributors {
			d.contributors[author] += count
		}
	}

	var silos []FileSilo
	for dir, fi := range dirs {
		totalCommits := 0
		for _, c := range fi.contributors {
			totalCommits += c
		}
		if totalCommits < opts.MinCommits {
			continue
		}
		busFactor := len(fi.contributors)
		if busFactor > opts.Threshold {
			continue
		}
		owner := topContributor(fi.contributors)
		silos = append(silos, FileSilo{
			Path:      dir,
			Owner:     owner,
			Commits:   totalCommits,
			BusFactor: busFactor,
			IsDir:     true,
			RiskLevel: riskLevel(busFactor, totalCommits),
		})
	}
	return silos
}

func topContributor(contributors map[string]int) string {
	best := ""
	max := 0
	for name, count := range contributors {
		if count > max {
			max = count
			best = name
		}
	}
	return best
}

func riskLevel(busFactor, commits int) string {
	if busFactor == 1 && commits >= 10 {
		return "high"
	}
	if busFactor == 1 {
		return "medium"
	}
	return "low"
}

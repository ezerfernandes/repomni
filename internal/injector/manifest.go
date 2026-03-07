package injector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const manifestDir = "repomni"
const manifestFile = "manifest.json"

// ManifestEntry records a single path injected by repomni.
type ManifestEntry struct {
	TargetPath string `json:"target_path"` // relative to repo root
	SourcePath string `json:"source_path"` // absolute source path used
	Mode       string `json:"mode"`        // "symlink" or "copy"
}

// Manifest tracks all paths injected into a repository.
type Manifest struct {
	Entries []ManifestEntry `json:"entries"`
}

// Has reports whether the manifest contains an entry for the given target path.
func (m *Manifest) Has(targetPath string) bool {
	for _, e := range m.Entries {
		if e.TargetPath == targetPath {
			return true
		}
	}
	return false
}

// EntriesUnder returns all manifest entries whose TargetPath is a direct child
// of the given directory prefix (e.g. ".claude/skills").
func (m *Manifest) EntriesUnder(dirPrefix string) []ManifestEntry {
	prefix := dirPrefix + "/"
	var result []ManifestEntry
	for _, e := range m.Entries {
		if strings.HasPrefix(e.TargetPath, prefix) {
			// Only direct children (no further slashes after prefix)
			rest := e.TargetPath[len(prefix):]
			if !strings.Contains(rest, "/") {
				result = append(result, e)
			}
		}
	}
	return result
}

// Add adds an entry to the manifest if it doesn't already exist.
func (m *Manifest) Add(entry ManifestEntry) {
	for i, e := range m.Entries {
		if e.TargetPath == entry.TargetPath {
			m.Entries[i] = entry
			return
		}
	}
	m.Entries = append(m.Entries, entry)
}

// Remove deletes the entry for targetPath if present.
func (m *Manifest) Remove(targetPath string) {
	filtered := m.Entries[:0]
	for _, e := range m.Entries {
		if e.TargetPath != targetPath {
			filtered = append(filtered, e)
		}
	}
	m.Entries = filtered
}

// TargetPaths returns the distinct managed target paths in manifest order.
func (m *Manifest) TargetPaths() []string {
	seen := make(map[string]bool)
	var paths []string
	for _, e := range m.Entries {
		if seen[e.TargetPath] {
			continue
		}
		seen[e.TargetPath] = true
		paths = append(paths, e.TargetPath)
	}
	return paths
}

func manifestPath(gitDir string) string {
	return filepath.Join(gitDir, manifestDir, manifestFile)
}

// LoadManifest reads the manifest from .git/repomni/manifest.json.
// Returns an empty manifest if the file does not exist.
func LoadManifest(gitDir string) *Manifest {
	data, err := os.ReadFile(manifestPath(gitDir))
	if err != nil {
		return &Manifest{}
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &Manifest{}
	}
	return &m
}

// SaveManifest writes the manifest to .git/repomni/manifest.json.
func SaveManifest(gitDir string, m *Manifest) error {
	dir := filepath.Join(gitDir, manifestDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(gitDir), data, 0644)
}

// ClearManifest removes the manifest file.
func ClearManifest(gitDir string) error {
	err := os.Remove(manifestPath(gitDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Try to clean up the directory if empty
	dir := filepath.Join(gitDir, manifestDir)
	removeIfEmptyDir(dir)
	return nil
}

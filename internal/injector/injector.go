package injector

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ezerfernandes/repomni/internal/config"
	"github.com/ezerfernandes/repomni/internal/gitutil"
)

// validateTargetPath checks that targetPath does not escape baseDir via path traversal.
func validateTargetPath(baseDir, targetPath string) error {
	joined := filepath.Join(baseDir, targetPath)
	rel, err := filepath.Rel(baseDir, joined)
	if err != nil {
		return fmt.Errorf("invalid target path %q: %w", targetPath, err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("target path %q escapes target directory", targetPath)
	}
	return nil
}

// Result describes the outcome of injecting or ejecting a single item.
type Result struct {
	Item   config.Item
	Action string // "created", "updated", "skipped", "error"
	Detail string
}

// Options controls the behavior of an [Inject] operation.
type Options struct {
	DryRun bool
	Force  bool
	Mode   config.InjectionMode
	// SelectedEntries filters directory item entries. Key is the item's TargetPath,
	// value is a set of entry names to include. If nil or missing for an item,
	// all entries are included.
	SelectedEntries map[string]map[string]bool
}

// isEnvFile returns true if the item represents a .env or .envrc file.
func isEnvFile(item config.Item) bool {
	return item.Type == config.ItemTypeFile && (item.TargetPath == ".env" || item.TargetPath == ".envrc")
}

type envSearchResult struct {
	// Found maps filename (.env or .envrc) to the directory where it was found.
	Found map[string]string
	// HitGitRepo is true if the search stopped at a parent git repo that had neither file.
	HitGitRepo bool
}

// findEnvInParents searches parent directories starting from the parent of startDir
// for .env and .envrc files. It walks upward until it finds at least one of them,
// encounters a git repository that has neither, or reaches the filesystem root.
func findEnvInParents(startDir string) envSearchResult {
	dir := filepath.Dir(startDir)

	for {
		found := make(map[string]string)

		for _, name := range []string{".env", ".envrc"} {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				found[name] = dir
			}
		}

		if len(found) > 0 {
			return envSearchResult{Found: found}
		}

		if gitutil.IsGitRepo(dir) {
			return envSearchResult{HitGitRepo: true}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return envSearchResult{}
		}
		dir = parent
	}
}

// Inject places each enabled config item into targetDir using symlinks or copies.
func Inject(cfg *config.Config, targetDir string, opts Options) ([]Result, error) {
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve target path: %w", err)
	}

	if !gitutil.IsGitRepo(targetDir) {
		return nil, fmt.Errorf("%s is not a git repository", targetDir)
	}

	gitDir, err := gitutil.FindGitDir(targetDir)
	if err != nil {
		return nil, err
	}

	sourceDir, err := filepath.Abs(cfg.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve source path: %w", err)
	}

	if sourceDir == targetDir {
		return nil, fmt.Errorf("source and target are the same directory: %s", targetDir)
	}

	mode := opts.Mode
	if mode == "" {
		mode = cfg.Mode
	}

	var results []Result
	var excludePaths []string
	manifest := LoadManifest(gitDir)

	// Search parent directories for .env/.envrc files
	envSearch := findEnvInParents(targetDir)

	for _, item := range cfg.EnabledItems() {
		// Validate target path does not escape targetDir
		if err := validateTargetPath(targetDir, item.TargetPath); err != nil {
			results = append(results, Result{Item: item, Action: "error", Detail: err.Error()})
			continue
		}

		src := filepath.Join(sourceDir, item.SourcePath)
		dst := filepath.Join(targetDir, item.TargetPath)

		var envFoundDir string

		// For .env/.envrc files, search parent directories instead of source dir
		if isEnvFile(item) {
			fileName := item.TargetPath
			if foundDir, ok := envSearch.Found[fileName]; ok {
				src = filepath.Join(foundDir, fileName)
				envFoundDir = foundDir
			} else {
				detail := "not found in any parent directory"
				if envSearch.HitGitRepo {
					detail = "not found in parent git repository"
				}
				results = append(results, Result{Item: item, Action: "skipped", Detail: detail})
				continue
			}
		}

		// Check source exists
		srcInfo, err := os.Stat(src)
		if err != nil {
			results = append(results, Result{Item: item, Action: "skipped", Detail: fmt.Sprintf("source not found: %s", src)})
			continue
		}

		// Directory items get per-entry merging
		if srcInfo.IsDir() {
			dirResults, dirExcludes := injectDirMerged(item, src, dst, mode, opts, manifest)
			results = append(results, dirResults...)
			excludePaths = append(excludePaths, dirExcludes...)
			continue
		}

		if opts.DryRun {
			action := "symlink"
			if mode == config.ModeCopy {
				action = "copy"
			}
			detail := fmt.Sprintf("would %s %s -> %s", action, src, dst)
			if envFoundDir != "" {
				detail = fmt.Sprintf("found at %s, %s", envFoundDir, detail)
			}
			results = append(results, Result{Item: item, Action: "dry-run", Detail: detail})
			excludePaths = append(excludePaths, item.TargetPath)
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			results = append(results, Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot create parent dir: %v", err)})
			continue
		}

		var result Result
		if mode == config.ModeSymlink {
			result = createSymlink(item, src, dst, opts.Force)
		} else {
			result = copyFile(item, src, dst, opts.Force)
		}

		if envFoundDir != "" {
			result.Detail = fmt.Sprintf("found at %s, %s", envFoundDir, result.Detail)
		}

		results = append(results, result)
		excludePaths = append(excludePaths, item.TargetPath)

		// Record in manifest only for paths we can safely attribute to repomni.
		if shouldRecordManagedPath(result, mode) {
			manifest.Add(ManifestEntry{
				TargetPath: item.TargetPath,
				SourcePath: src,
				Mode:       string(mode),
			})
		}
	}

	// Update .git/info/exclude
	if !opts.DryRun && len(excludePaths) > 0 {
		if err := UpdateExclude(gitDir, excludePaths); err != nil {
			return results, fmt.Errorf("failed to update .git/info/exclude: %w", err)
		}
	}

	// Persist manifest
	if !opts.DryRun {
		if err := SaveManifest(gitDir, manifest); err != nil {
			return results, fmt.Errorf("failed to save manifest: %w", err)
		}
	}

	return results, nil
}

// Eject removes all previously injected items from targetDir and cleans up git excludes.
func Eject(cfg *config.Config, targetDir string) ([]Result, error) {
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve target path: %w", err)
	}

	gitDir, err := gitutil.FindGitDir(targetDir)
	if err != nil {
		return nil, err
	}

	manifest := LoadManifest(gitDir)
	if len(manifest.Entries) == 0 {
		return refuseEjectWithoutManifest(cfg, targetDir)
	}

	var results []Result

	for _, item := range cfg.EnabledItems() {
		// Validate target path does not escape targetDir
		if err := validateTargetPath(targetDir, item.TargetPath); err != nil {
			results = append(results, Result{Item: item, Action: "error", Detail: err.Error()})
			continue
		}

		dst := filepath.Join(targetDir, item.TargetPath)

		// Directory items get per-entry ejection
		if item.Type == config.ItemTypeDirectory {
			dirResults := ejectDirByManifest(item, targetDir, manifest)
			results = append(results, dirResults...)
			reconcileManifest(dirResults, manifest)
			// Clean up the directory itself if now empty
			removeIfEmptyDir(dst)
			continue
		}

		// For file items, check manifest first
		if !manifest.Has(item.TargetPath) {
			results = append(results, Result{Item: item, Action: "skipped", Detail: "not managed by repomni"})
			continue
		}

		info, err := os.Lstat(dst)
		if err != nil {
			results = append(results, Result{Item: item, Action: "skipped", Detail: "not present"})
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dst); err != nil {
				results = append(results, Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot remove symlink: %v", err)})
			} else {
				results = append(results, Result{Item: item, Action: "removed", Detail: "symlink removed"})
				manifest.Remove(item.TargetPath)
			}
		} else {
			if err := os.Remove(dst); err != nil {
				results = append(results, Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot remove file: %v", err)})
			} else {
				results = append(results, Result{Item: item, Action: "removed", Detail: "file removed"})
				manifest.Remove(item.TargetPath)
			}
		}
	}

	// Clean up empty parent directories (.claude/ if empty)
	for _, item := range cfg.EnabledItems() {
		dst := filepath.Join(targetDir, item.TargetPath)
		parent := filepath.Dir(dst)
		if parent != targetDir {
			removeIfEmptyDir(parent)
		}
	}

	if len(manifest.Entries) == 0 {
		if err := CleanExclude(gitDir); err != nil {
			return results, fmt.Errorf("failed to clean .git/info/exclude: %w", err)
		}
		if err := ClearManifest(gitDir); err != nil {
			return results, fmt.Errorf("failed to clear manifest: %w", err)
		}
		return results, nil
	}

	if err := UpdateExclude(gitDir, manifest.TargetPaths()); err != nil {
		return results, fmt.Errorf("failed to update .git/info/exclude: %w", err)
	}
	if err := SaveManifest(gitDir, manifest); err != nil {
		return results, fmt.Errorf("failed to save manifest: %w", err)
	}

	return results, nil
}

func refuseEjectWithoutManifest(cfg *config.Config, targetDir string) ([]Result, error) {
	var (
		results     []Result
		foundTarget bool
	)

	for _, item := range cfg.EnabledItems() {
		dst := filepath.Join(targetDir, item.TargetPath)
		if _, err := os.Lstat(dst); err != nil {
			results = append(results, Result{Item: item, Action: "skipped", Detail: "not present"})
			continue
		}

		foundTarget = true
		results = append(results, Result{
			Item:   item,
			Action: "skipped",
			Detail: "manifest missing; refusing to delete",
		})
	}

	if foundTarget {
		return results, fmt.Errorf(
			"refusing to eject from %s without repomni manifest; run inject again to recreate metadata or remove managed paths manually",
			targetDir,
		)
	}

	return results, nil
}

// ItemStatus reports the current state of a single injected item in a target repo.
type ItemStatus struct {
	Item     config.Item
	Present  bool
	Current  bool // symlink points to correct source, or copy matches
	Excluded bool
	Detail   string
}

// Status checks each enabled config item in targetDir and reports whether it is
// present, current, and excluded from git.
func Status(cfg *config.Config, targetDir string) ([]ItemStatus, error) {
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve target path: %w", err)
	}

	gitDir, err := gitutil.FindGitDir(targetDir)
	if err != nil {
		return nil, err
	}

	sourceDir, err := filepath.Abs(cfg.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve source path: %w", err)
	}

	excludedPaths := GetExcludedPaths(gitDir)
	excludeSet := make(map[string]bool)
	for _, p := range excludedPaths {
		excludeSet[p] = true
	}

	var statuses []ItemStatus

	// Search parent directories for .env/.envrc files
	envSearch := findEnvInParents(targetDir)

	for _, item := range cfg.EnabledItems() {
		src := filepath.Join(sourceDir, item.SourcePath)
		dst := filepath.Join(targetDir, item.TargetPath)

		// For .env/.envrc files, use parent directory search
		if isEnvFile(item) {
			fileName := item.TargetPath
			if foundDir, ok := envSearch.Found[fileName]; ok {
				src = filepath.Join(foundDir, fileName)
			}
		}

		// Directory items get per-entry status
		if item.Type == config.ItemTypeDirectory {
			dirStatuses := statusDir(item, src, dst, excludeSet)
			statuses = append(statuses, dirStatuses...)
			continue
		}

		status := ItemStatus{
			Item:     item,
			Excluded: excludeSet[item.TargetPath],
		}

		info, err := os.Lstat(dst)
		if err != nil {
			status.Detail = "not present"
			statuses = append(statuses, status)
			continue
		}

		status.Present = true

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(dst)
			if err == nil && target == src {
				status.Current = true
				status.Detail = "symlink ok"
			} else if err == nil {
				status.Detail = fmt.Sprintf("symlink points to %s (expected %s)", target, src)
			} else {
				status.Detail = "cannot read symlink"
			}
		} else if filesEqual(src, dst) {
			status.Current = true
			status.Detail = "copy ok"
		} else {
			status.Detail = "copy out of date"
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// injectDirMerged creates per-entry symlinks or copies inside the target directory,
// merging with any existing content. Entries that already exist in the target
// with the same name are skipped with a warning.
func injectDirMerged(item config.Item, src, dst string, mode config.InjectionMode, opts Options, manifest *Manifest) ([]Result, []string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return []Result{{Item: item, Action: "error", Detail: fmt.Sprintf("cannot read source directory: %v", err)}}, nil
	}

	if len(entries) == 0 {
		return []Result{{Item: item, Action: "skipped", Detail: "source directory is empty"}}, nil
	}

	var results []Result
	var excludes []string

	// Check if a selection filter applies to this directory item
	selectionFilter := opts.SelectedEntries[item.TargetPath]

	for _, entry := range entries {
		entryName := entry.Name()

		// Skip entries not in the selection filter (when filter is set)
		if selectionFilter != nil && !selectionFilter[entryName] {
			continue
		}

		entrySrc := filepath.Join(src, entryName)
		entryDst := filepath.Join(dst, entryName)
		entryExclude := filepath.Join(item.TargetPath, entryName)

		subItem := config.Item{
			Type:       item.Type,
			SourcePath: filepath.Join(item.SourcePath, entryName),
			TargetPath: filepath.Join(item.TargetPath, entryName),
			Enabled:    true,
		}

		if opts.DryRun {
			action := "symlink"
			if mode == config.ModeCopy {
				action = "copy"
			}
			results = append(results, Result{Item: subItem, Action: "dry-run", Detail: fmt.Sprintf("would %s %s -> %s", action, entrySrc, entryDst)})
			excludes = append(excludes, entryExclude)
			continue
		}

		// Ensure target directory exists (as a real directory, not a symlink)
		if err := os.MkdirAll(dst, 0755); err != nil {
			results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot create directory %s: %v", dst, err)})
			continue
		}

		// Check if something already exists at the destination
		info, lstatErr := os.Lstat(entryDst)
		if lstatErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				// It's a symlink — check if it points to our source
				target, _ := os.Readlink(entryDst)
				if target == entrySrc {
					results = append(results, Result{Item: subItem, Action: "skipped", Detail: "already up to date"})
					if mode == config.ModeSymlink {
						manifest.Add(ManifestEntry{
							TargetPath: subItem.TargetPath,
							SourcePath: entrySrc,
							Mode:       string(mode),
						})
					}
					excludes = append(excludes, entryExclude)
					continue
				}
			}
			// Something else exists with the same name — warn and skip
			results = append(results, Result{Item: subItem, Action: "warning", Detail: fmt.Sprintf("%s already exists in repo, skipping", entryName)})
			continue
		}

		if mode == config.ModeSymlink {
			if err := os.Symlink(entrySrc, entryDst); err != nil {
				results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot create symlink: %v", err)})
				continue
			}
			results = append(results, Result{Item: subItem, Action: "created", Detail: fmt.Sprintf("symlinked -> %s", entrySrc)})
		} else {
			if entry.IsDir() {
				if err := copyDirRecursive(entrySrc, entryDst); err != nil {
					results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot copy directory: %v", err)})
					continue
				}
			} else {
				if err := copyFileContent(entrySrc, entryDst); err != nil {
					results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot copy: %v", err)})
					continue
				}
			}
			results = append(results, Result{Item: subItem, Action: "created", Detail: "copied"})
		}
		manifest.Add(ManifestEntry{
			TargetPath: subItem.TargetPath,
			SourcePath: entrySrc,
			Mode:       string(mode),
		})
		excludes = append(excludes, entryExclude)
	}

	return results, excludes
}

// ejectDirByManifest removes injected entries from a directory item using the
// persisted manifest instead of the live source tree. This ensures cleanup is
// correct even when source entries have been renamed or removed after injection.
func ejectDirByManifest(item config.Item, targetDir string, manifest *Manifest) []Result {
	dst := filepath.Join(targetDir, item.TargetPath)

	// Check if target exists at all
	info, err := os.Lstat(dst)
	if err != nil {
		return []Result{{Item: item, Action: "skipped", Detail: "not present"}}
	}

	// Old-style: whole directory is a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		if manifest.Has(item.TargetPath) {
			if err := os.Remove(dst); err != nil {
				return []Result{{Item: item, Action: "error", Detail: fmt.Sprintf("cannot remove symlink: %v", err)}}
			}
			return []Result{{Item: item, Action: "removed", Detail: "directory symlink removed"}}
		}
		return []Result{{Item: item, Action: "skipped", Detail: "not managed by repomni"}}
	}

	// New-style: iterate manifest entries for this directory
	entries := manifest.EntriesUnder(item.TargetPath)
	if len(entries) == 0 {
		return []Result{{Item: item, Action: "skipped", Detail: "no managed entries found"}}
	}

	var results []Result
	for _, me := range entries {
		entryDst := filepath.Join(targetDir, me.TargetPath)

		subItem := config.Item{
			Type:       item.Type,
			SourcePath: strings.TrimPrefix(me.TargetPath, item.TargetPath+"/"),
			TargetPath: me.TargetPath,
			Enabled:    true,
		}
		subItem.SourcePath = filepath.Join(item.SourcePath, subItem.SourcePath)

		entryInfo, err := os.Lstat(entryDst)
		if err != nil {
			results = append(results, Result{Item: subItem, Action: "skipped", Detail: "not present"})
			continue
		}

		if entryInfo.Mode()&os.ModeSymlink != 0 {
			// Only remove if it points to our recorded source
			target, _ := os.Readlink(entryDst)
			if target == me.SourcePath {
				if err := os.Remove(entryDst); err != nil {
					results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot remove symlink: %v", err)})
				} else {
					results = append(results, Result{Item: subItem, Action: "removed", Detail: "symlink removed"})
				}
			} else {
				results = append(results, Result{Item: subItem, Action: "skipped", Detail: "symlink points elsewhere, not ours"})
			}
		} else if entryInfo.IsDir() {
			if err := os.RemoveAll(entryDst); err != nil {
				results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot remove directory: %v", err)})
			} else {
				results = append(results, Result{Item: subItem, Action: "removed", Detail: "directory removed"})
			}
		} else {
			if err := os.Remove(entryDst); err != nil {
				results = append(results, Result{Item: subItem, Action: "error", Detail: fmt.Sprintf("cannot remove file: %v", err)})
			} else {
				results = append(results, Result{Item: subItem, Action: "removed", Detail: "file removed"})
			}
		}
	}

	if len(results) == 0 {
		return []Result{{Item: item, Action: "skipped", Detail: "no managed entries found"}}
	}
	return results
}

// statusDir reports per-entry status for a directory item.
func statusDir(item config.Item, src, dst string, excludeSet map[string]bool) []ItemStatus {
	entries, err := os.ReadDir(src)
	if err != nil {
		return []ItemStatus{{
			Item:   item,
			Detail: fmt.Sprintf("source not readable: %v", err),
		}}
	}

	if len(entries) == 0 {
		return []ItemStatus{{
			Item:   item,
			Detail: "source directory is empty",
		}}
	}

	var statuses []ItemStatus
	for _, entry := range entries {
		entryName := entry.Name()
		entrySrc := filepath.Join(src, entryName)
		entryDst := filepath.Join(dst, entryName)
		entryExclude := filepath.Join(item.TargetPath, entryName)

		subItem := config.Item{
			Type:       item.Type,
			SourcePath: filepath.Join(item.SourcePath, entryName),
			TargetPath: filepath.Join(item.TargetPath, entryName),
			Enabled:    true,
		}

		status := ItemStatus{
			Item:     subItem,
			Excluded: excludeSet[entryExclude],
		}

		info, err := os.Lstat(entryDst)
		if err != nil {
			status.Detail = "not present"
			statuses = append(statuses, status)
			continue
		}

		status.Present = true

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(entryDst)
			if err == nil && target == entrySrc {
				status.Current = true
				status.Detail = "symlink ok"
			} else if err == nil {
				status.Detail = fmt.Sprintf("symlink points to %s (expected %s)", target, entrySrc)
			} else {
				status.Detail = "cannot read symlink"
			}
		} else if filesEqual(entrySrc, entryDst) {
			status.Current = true
			status.Detail = "copy ok"
		} else {
			status.Detail = "copy out of date"
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func createSymlink(item config.Item, src, dst string, force bool) Result {
	existing, err := os.Readlink(dst)
	if err == nil {
		if existing == src {
			return Result{Item: item, Action: "skipped", Detail: "already up to date"}
		}
		// Symlink exists but points elsewhere — atomic replace via temp symlink + rename
		return atomicSymlink(item, src, dst)
	}

	// Check if a regular file/dir exists
	if _, statErr := os.Lstat(dst); statErr == nil {
		if !force {
			return Result{Item: item, Action: "skipped", Detail: "regular file exists (use --force to overwrite)"}
		}
		os.RemoveAll(dst)
	}

	if err := os.Symlink(src, dst); err != nil {
		return Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot create symlink: %v", err)}
	}

	return Result{Item: item, Action: "created", Detail: fmt.Sprintf("symlinked -> %s", src)}
}

// atomicSymlink replaces dst with a symlink to src using a temp file + rename
// to avoid TOCTOU race conditions.
func atomicSymlink(item config.Item, src, dst string) Result {
	tmp := dst + ".repomni.tmp"
	os.Remove(tmp) // clean up any stale temp
	if err := os.Symlink(src, tmp); err != nil {
		return Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot create temp symlink: %v", err)}
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot replace symlink: %v", err)}
	}
	return Result{Item: item, Action: "created", Detail: fmt.Sprintf("symlinked -> %s", src)}
}

func copyFile(item config.Item, src, dst string, force bool) Result {
	if _, err := os.Lstat(dst); err == nil {
		if !force {
			// Check if content matches
			if filesEqual(src, dst) {
				return Result{Item: item, Action: "skipped", Detail: "already up to date"}
			}
			return Result{Item: item, Action: "skipped", Detail: "file exists with different content (use --force to overwrite)"}
		}
		os.Remove(dst)
	}

	if err := copyFileContent(src, dst); err != nil {
		return Result{Item: item, Action: "error", Detail: fmt.Sprintf("cannot copy: %v", err)}
	}

	return Result{Item: item, Action: "created", Detail: "copied"}
}

func copyFileContent(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		return copyFileContent(path, target)
	})
}

func filesEqual(a, b string) bool {
	dataA, errA := os.ReadFile(a)
	dataB, errB := os.ReadFile(b)
	if errA != nil || errB != nil {
		return false
	}
	return strings.TrimSpace(string(dataA)) == strings.TrimSpace(string(dataB))
}

func removeIfEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
}

func shouldRecordManagedPath(result Result, mode config.InjectionMode) bool {
	if result.Action == "created" {
		return true
	}

	return mode == config.ModeSymlink &&
		result.Action == "skipped" &&
		result.Detail == "already up to date"
}

func reconcileManifest(results []Result, manifest *Manifest) {
	for _, result := range results {
		switch {
		case result.Action == "removed":
			manifest.Remove(result.Item.TargetPath)
		case result.Action == "skipped" && result.Detail == "not present":
			manifest.Remove(result.Item.TargetPath)
		}
	}
}

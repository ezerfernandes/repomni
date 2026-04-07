package syncer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BenchmarkSyncAll measures the time to sync multiple repos that are behind.
// This is the primary metric for the autoresearch optimization loop.
func BenchmarkSyncAll(b *testing.B) {
	for _, repoCount := range []int{5, 10} {
		for _, jobs := range []int{1, 4} {
			name := fmt.Sprintf("repos=%d/jobs=%d", repoCount, jobs)
			b.Run(name, func(b *testing.B) {
				// Setup: create bare repos and clones that are behind
				type repoEnv struct {
					bareDir  string
					cloneDir string
				}
				envs := make([]repoEnv, repoCount)

				for i := range repoCount {
					bareDir := filepath.Join(b.TempDir(), fmt.Sprintf("bare-%d.git", i))
					runB(b, "", "git", "init", "--bare", bareDir)

					cloneDir := filepath.Join(b.TempDir(), fmt.Sprintf("clone-%d", i))
					runB(b, "", "git", "clone", bareDir, cloneDir)

					writeFileB(b, filepath.Join(cloneDir, "README.md"), "init")
					runB(b, cloneDir, "git", "add", ".")
					runB(b, cloneDir, "git", "commit", "-m", "init")
					runB(b, cloneDir, "git", "push")

					envs[i] = repoEnv{bareDir: bareDir, cloneDir: cloneDir}
				}

				iter := 0
				b.ResetTimer()
				for range b.N {
					// Before each iteration, push a new commit from second clones
					// and re-create the "behind" state
					b.StopTimer()
					repos := make([]string, repoCount)
					for i, env := range envs {
						// Push from a fresh second clone to make the primary behind
						clone2 := filepath.Join(b.TempDir(), fmt.Sprintf("clone2-%d-%d", i, iter))
						runB(b, "", "git", "clone", env.bareDir, clone2)
						writeFileB(b, filepath.Join(clone2, fmt.Sprintf("file-%d-%d.txt", i, iter)), fmt.Sprintf("content-%d-%d", i, iter))
						runB(b, clone2, "git", "add", ".")
						runB(b, clone2, "git", "commit", "-m", "upstream commit")
						runB(b, clone2, "git", "push")
						repos[i] = env.cloneDir
					}
					b.StartTimer()

					results, _ := SyncAll(repos, SyncOptions{Jobs: jobs, NoFetch: false})

					b.StopTimer()
					for j, r := range results {
						if r.Action != "pulled" {
							b.Errorf("repo %d: expected pulled, got %s: %s", j, r.Action, r.PostDetail)
						}
					}
					iter++
					b.StartTimer()
				}
			})
		}
	}
}

// BenchmarkCheckStatus measures the time to check status of a single repo.
func BenchmarkCheckStatus(b *testing.B) {
	bareDir := filepath.Join(b.TempDir(), "bare.git")
	runB(b, "", "git", "init", "--bare", bareDir)

	cloneDir := filepath.Join(b.TempDir(), "clone")
	runB(b, "", "git", "clone", bareDir, cloneDir)

	writeFileB(b, filepath.Join(cloneDir, "README.md"), "init")
	runB(b, cloneDir, "git", "add", ".")
	runB(b, cloneDir, "git", "commit", "-m", "init")
	runB(b, cloneDir, "git", "push")

	b.Run("noFetch", func(b *testing.B) {
		for range b.N {
			s := CheckStatus(cloneDir, true, false)
			if s.State != StateCurrent {
				b.Errorf("expected current, got %s", s.State)
			}
		}
	})

	b.Run("withFetch", func(b *testing.B) {
		for range b.N {
			s := CheckStatus(cloneDir, false, false)
			if s.State != StateCurrent {
				b.Errorf("expected current, got %s", s.State)
			}
		}
	})
}

func runB(b *testing.B, dir string, name string, args ...string) string {
	b.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func writeFileB(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatal(err)
	}
}

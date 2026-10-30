package buildstore

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/kb"
)

func TestOperationLockPathFollowsAGitFile(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "wt")
	gitdir := filepath.Join(base, "main", ".git", "worktrees", "wt")
	for _, tc := range []struct{ name, dotGit, want string }{
		{"a .git directory", "", filepath.Join(repo, ".git", OperationLockFile)},
		{"a relative gitdir", "gitdir: ../main/.git/worktrees/wt\n", filepath.Join(gitdir, OperationLockFile)},
		{"an absolute gitdir", "gitdir: " + gitdir + "\n", filepath.Join(gitdir, OperationLockFile)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.RemoveAll(repo); err != nil {
				t.Fatal(err)
			}
			if tc.dotGit == "" {
				if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				storetest.WriteFile(t, filepath.Join(repo, ".git"), tc.dotGit)
			}
			if got := OperationLockPath(filepath.Join(repo, kb.KBDir)); got != tc.want {
				t.Errorf("OperationLockPath = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestAnswerCacheCountsTheTrailDirectories: the answer cache's entries and
// bytes are the regular files of every trail directory's answers; a directory
// under one is no answer, the flat directory an earlier kbase wrote counts
// for nothing, and a store with no cache holds none.
func TestAnswerCacheCountsTheTrailDirectories(t *testing.T) {
	dir := t.TempDir()
	if entries, size, err := answerCache(dir); err != nil || entries != 0 || size != 0 {
		t.Errorf("answerCache with no cache = %d entries, %d bytes, %v; want none", entries, size, err)
	}
	trails := filepath.Join(dir, ScratchDir, TrailsDir)
	first, second := filepath.Join(trails, "c1", asks.AnswersDir), filepath.Join(trails, "c2", asks.AnswersDir)
	storetest.WriteFile(t, filepath.Join(first, "a.json"), "xy")
	storetest.WriteFile(t, filepath.Join(first, "b.json"), "xyz")
	storetest.WriteFile(t, filepath.Join(second, "c.json"), "w")
	storetest.WriteFile(t, filepath.Join(dir, ScratchDir, asks.AnswersDir, "flat.json"), "ignored")
	if err := os.Mkdir(filepath.Join(first, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if entries, size, err := answerCache(dir); err != nil || entries != 3 || size != 6 {
		t.Errorf("answerCache = %d entries, %d bytes, %v; want 3, 6", entries, size, err)
	}
}

// TestPruneCapturesKeepsTheNewestStemsOfItsPrefix: captures are counted by
// the stem their two files share, a lone half counting as one; the oldest
// stems by name go with both files, and a file of another prefix or with no
// capture suffix stands. Fewer stems than the count remove nothing, and no
// reports directory is no error.
func TestPruneCapturesKeepsTheNewestStemsOfItsPrefix(t *testing.T) {
	if err := PruneCaptures(filepath.Join(t.TempDir(), ReportsDir), "p-", 1); err != nil {
		t.Errorf("PruneCaptures over no reports directory = %v, want nil", err)
	}
	all := []string{
		"p-1.stderr.log", "p-1.stdout.yaml", "p-2.stderr.log", "p-2.stdout.yaml",
		"p-3.stderr.log", "p-3.stdout.yaml", "p-4.stderr.log", "p-4.stdout.yaml",
		"p-5.stdout.yaml", "p-x.yaml", "q-0.stdout.yaml",
	}
	for _, tc := range []struct {
		name  string
		keep  int
		stand []string
	}{
		{"three of five stems", 3, []string{
			"p-3.stderr.log", "p-3.stdout.yaml", "p-4.stderr.log", "p-4.stdout.yaml",
			"p-5.stdout.yaml", "p-x.yaml", "q-0.stdout.yaml",
		}},
		{"more than there are", 6, all},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reports := t.TempDir()
			for _, name := range all {
				storetest.WriteFile(t, filepath.Join(reports, name), "x")
			}
			if err := PruneCaptures(reports, "p-", tc.keep); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(reports)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, e := range entries {
				got = append(got, e.Name())
			}
			if !slices.Equal(got, tc.stand) {
				t.Errorf("standing = %q, want %q", got, tc.stand)
			}
		})
	}
}

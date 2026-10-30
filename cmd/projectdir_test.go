package main

import (
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/kb"
	toolresult "kbase/internal/result"
)

// resetProjectDir puts --project-dir back as a new process would find it once
// the test ends: the command tree is shared between tests, and ResetFlags
// leaves persistent flags alone.
func resetProjectDir(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		flagProjectDir = ""
		rootCmd.PersistentFlags().Lookup("project-dir").Changed = false
	})
}

// realPath is dir with its symlinks resolved, as a verb reports a path.
func realPath(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// TestProjectDirNamesTheRepository: from a directory in no repository,
// --project-dir is where the verb finds its KB, and where init births one.
func TestProjectDirNamesTheRepository(t *testing.T) {
	t.Run("verify", func(t *testing.T) {
		resetProjectDir(t)
		repo := realPath(t, maintenanceRepo(t))
		out, code := kbase(t, t.TempDir(), "--project-dir", repo, "verify")
		d := checkDocument(t, "verify", out)
		if d.Outcome != toolresult.Done || code != 0 || d.Values["kb-root"] != filepath.Join(repo, kb.KBDir) {
			t.Errorf("outcome %s, exit %d, kb-root %v; want done, 0, the repository's kb-root\n%s", d.Outcome, code, d.Values["kb-root"], out)
		}
	})
	t.Run("init", func(t *testing.T) {
		resetProjectDir(t)
		repo := gitWorktree(t)
		out, code := kbase(t, t.TempDir(), "--project-dir", repo, "init")
		if d := parseResult(t, out); d.Outcome != toolresult.Done || code != 0 {
			t.Fatalf("outcome %s, exit %d\n%s", d.Outcome, code, out)
		}
		if _, err := os.Stat(filepath.Join(repo, kb.KBDir, kb.EntryPointFile)); err != nil {
			t.Errorf("init under --project-dir wrote no entry point: %v", err)
		}
	})
}

// TestProjectDirRefusesWhatIsNoWorktreeRoot: a subdirectory of a worktree and a
// missing path are refused, naming the flag and the absolute path.
func TestProjectDirRefusesWhatIsNoWorktreeRoot(t *testing.T) {
	repo := realPath(t, maintenanceRepo(t))
	for name, dir := range map[string]string{
		"subdirectory": filepath.Join(repo, kb.KBDir),
		"missing":      filepath.Join(repo, "nowhere"),
	} {
		t.Run(name, func(t *testing.T) {
			resetProjectDir(t)
			out, code := kbase(t, t.TempDir(), "--project-dir", dir, "verify")
			d := parseResult(t, out)
			items := d.items(toolresult.RefusalsKey)
			if d.Outcome != toolresult.Refused || code != 1 || len(items) != 1 {
				t.Fatalf("outcome %s, exit %d, %d items; want refused, 1, one item\n%s", d.Outcome, code, len(items), out)
			}
			item := items[0]
			if item["check"] != checkWorktree || item["key"] != "--project-dir" || item["path"] != dir {
				t.Errorf("the refusal item = %v", item)
			}
		})
	}
}

// TestRelativeValuesIsTheProjectRoots: from a subdirectory, a relative
// --values names a file under the repository root.
func TestRelativeValuesIsTheProjectRoots(t *testing.T) {
	resetProjectDir(t)
	repo := realPath(t, maintenanceRepo(t))
	gitRepo(t, repo)
	writeValues(t, repo, "values.yaml", "entry:\n- title: A Result\n  rigor: 0.5\n  rationale: Shown.\n")
	out, _ := kbase(t, filepath.Join(repo, kb.KBDir), "insert-claim-entry", "--no-refresh", "--values", "values.yaml")
	if d := parseResult(t, out); d.Outcome != toolresult.Done {
		t.Errorf("outcome %s, want %s\n%s", d.Outcome, toolresult.Done, out)
	}
}

// TestRelativeStateDirOfWaitIsTheProjectRoots: from a subdirectory, wait
// resolves a relative --state-dir against the repository root, as every other
// verb does. The replay's leg of this is in `score_replay_test.go`, its verb
// being dev-only.
func TestRelativeStateDirOfWaitIsTheProjectRoots(t *testing.T) {
	resetProjectDir(t)
	repo := realPath(t, maintenanceRepo(t))
	gitRepo(t, repo)
	sub := filepath.Join(repo, kb.KBDir)
	want := filepath.Join(repo, "store")
	out, _ := kbase(t, sub, "wait", "--state-dir", "store", "--pid", "999999")
	if got := parseResult(t, out).Values["state-dir"]; got != want {
		t.Errorf("state-dir = %v, want %s\n%s", got, want, out)
	}
}

// TestRelativeExportOutIsTheProjectRoots: from a subdirectory, a relative
// export-latex --out lands under the repository root.
func TestRelativeExportOutIsTheProjectRoots(t *testing.T) {
	requirePandoc(t)
	resetProjectDir(t)
	dir := exportDir(t)
	outDir := filepath.Join(dir, "paper")
	writeFileAt(t, outDir, "gen-tex/notes.txt", "mine")
	out, _ := kbase(t, filepath.Join(dir, kb.KBDir, "vol1"), "export-latex", "--out", "paper")
	items := parseResult(t, out).items(toolresult.RefusalsKey)
	if want := filepath.Join(outDir, "gen-tex"); len(items) != 1 || items[0]["path"] != want {
		t.Errorf("refusal items %v, want one naming %s", items, want)
	}
}

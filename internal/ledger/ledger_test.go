package ledger

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
)

var owned = []string{"kb-root", "record.yaml"}

// newRepo is a fresh repository with git's identity and global configuration
// fixed for the test.
func newRepo(t *testing.T) *Repo {
	t.Helper()
	isolateGit(t)
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	r, err := Open(dir, owned, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func isolateGit(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(t.TempDir(), "gitconfig"),
	} {
		t.Setenv(k, v)
	}
}

func write(t *testing.T, r *Repo, rel, text string) {
	t.Helper()
	p := filepath.Join(r.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, r *Repo, rel string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func TestOpenOutsideAWorktree(t *testing.T) {
	isolateGit(t)
	_, err := Open(t.TempDir(), owned, log.Discard())
	var nw NotWorktreeError
	if !errors.As(err, &nw) || !strings.Contains(err.Error(), "git init") {
		t.Errorf("Open outside a worktree = %v, want a NotWorktreeError naming git init", err)
	}
}

func TestRecordScopesTheCommitAndTrailReadsItBack(t *testing.T) {
	r := newRepo(t)
	if trail, err := r.Trail(); err != nil || len(trail) != 0 {
		t.Fatalf("Trail on an unborn branch = %v, %v", trail, err)
	}
	write(t, r, "src.tex", "source\n")
	if _, err := r.run("add", "src.tex"); err != nil {
		t.Fatal(err)
	}
	start, err := r.Record("start", "build started", "charter: none")
	if err != nil {
		t.Fatal(err)
	}
	write(t, r, "kb-root/entry-point.md", "# entry\n")
	write(t, r, "record.yaml", "a: 1\n")
	graph, err := r.Record("document-graph", "document tree derived", "")
	if err != nil {
		t.Fatal(err)
	}
	files, err := r.run("show", "--name-only", "--format=", graph)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(files); !slices.Equal(got, []string{"kb-root/entry-point.md", "record.yaml"}) {
		t.Errorf("document-graph commit holds %q, want the owned paths only", got)
	}
	if staged, _ := r.run("diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "src.tex" {
		t.Errorf("staged after the boundaries = %q, want the user's src.tex left staged", staged)
	}
	if _, err := r.run("commit", "--quiet", "--allow-empty", "-m", "unrelated", "-m", "kb-build: forged | in a body"); err != nil {
		t.Fatal(err)
	}
	trail, err := r.Trail()
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{"document-graph", graph, ""}, {"start", start, "charter: none"}}
	if !slices.Equal(trail, want) {
		t.Errorf("Trail = %+v, want %+v", trail, want)
	}
}

func TestDirtyAndRestore(t *testing.T) {
	r := newRepo(t)
	write(t, r, "kb-root/a.md", "a\n")
	write(t, r, "kb-root/b.md", "b\n")
	commit, err := r.Record("document-graph", "document tree derived", "")
	if err != nil {
		t.Fatal(err)
	}
	if dirty, err := r.Dirty(); err != nil || len(dirty) != 0 {
		t.Fatalf("Dirty after a boundary = %q, %v", dirty, err)
	}
	write(t, r, "kb-root/a.md", "changed\n")
	if err := os.Remove(filepath.Join(r.Root, "kb-root", "b.md")); err != nil {
		t.Fatal(err)
	}
	write(t, r, "kb-root/c.md", "new\n")
	write(t, r, "record.yaml", "new\n")
	write(t, r, "elsewhere.txt", "not owned\n")
	dirty, err := r.Dirty()
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(dirty)
	if want := []string{"kb-root/a.md", "kb-root/b.md", "kb-root/c.md", "record.yaml"}; !slices.Equal(dirty, want) {
		t.Errorf("Dirty = %q, want %q", dirty, want)
	}
	if err := r.Restore(commit); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{"kb-root/a.md": "a\n", "kb-root/b.md": "b\n", "elsewhere.txt": "not owned\n"} {
		if got, ok := read(t, r, rel); !ok || got != want {
			t.Errorf("%s after Restore = %q (present %t), want %q", rel, got, ok, want)
		}
	}
	for _, rel := range []string{"kb-root/c.md", "record.yaml"} {
		if _, ok := read(t, r, rel); ok {
			t.Errorf("%s survived Restore", rel)
		}
	}
	if dirty, err := r.Dirty(); err != nil || len(dirty) != 0 {
		t.Errorf("Dirty after Restore = %q, %v", dirty, err)
	}
}

func TestRestoreToACommitWithoutTheTree(t *testing.T) {
	r := newRepo(t)
	start, err := r.Record("start", "build started", "")
	if err != nil {
		t.Fatal(err)
	}
	write(t, r, "kb-root/a.md", "a\n")
	if _, err := r.run("add", "kb-root"); err != nil {
		t.Fatal(err)
	}
	write(t, r, "kb-root/b.md", "b\n")
	if err := r.Restore(start); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, "kb-root")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("kb-root/ survived a restore to a commit without it: %v", err)
	}
	if dirty, err := r.Dirty(); err != nil || len(dirty) != 0 {
		t.Errorf("Dirty after Restore = %q, %v", dirty, err)
	}
}

func TestRestoreWithNoCommitYet(t *testing.T) {
	r := newRepo(t)
	write(t, r, "record.yaml", "staged\n")
	if _, err := r.run("add", "record.yaml"); err != nil {
		t.Fatal(err)
	}
	write(t, r, "kb-root/a.md", "a\n")
	write(t, r, "elsewhere.txt", "not owned\n")
	if err := r.Restore(""); err != nil {
		t.Fatal(err)
	}
	if dirty, err := r.Dirty(); err != nil || len(dirty) != 0 {
		t.Errorf("Dirty after Restore on an unborn branch = %q, %v", dirty, err)
	}
	if _, ok := read(t, r, "elsewhere.txt"); !ok {
		t.Error("Restore removed a path the build does not own")
	}
}

func TestFiles(t *testing.T) {
	r := newRepo(t)
	write(t, r, "kb-root/a.md", "a\n")
	write(t, r, "kb-root/vol/index.md", "i\n")
	write(t, r, "record.yaml", "r\n")
	commit, err := r.Record("document-graph", "document tree derived", "")
	if err != nil {
		t.Fatal(err)
	}
	files, err := r.Files(commit, "kb-root")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || string(files["a.md"]) != "a\n" || string(files["vol/index.md"]) != "i\n" {
		t.Errorf("Files = %q", files)
	}
}

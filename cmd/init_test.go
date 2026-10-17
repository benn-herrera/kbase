package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/ledger"
	toolresult "kbase/internal/result"
)

// gitWorktree is a fresh git worktree named repo under a temp dir, its
// .git laid down by hand rather than git-init: cmd may not exec git
// (execpolicy), and init commits through ledger, the one package that may.
// The hand-built objects/refs/HEAD is the whole layout a birth commit needs.
func gitWorktree(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	for _, dir := range []string{".git/objects", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "config"), []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME":     "init test",
		"GIT_AUTHOR_EMAIL":    "init@test.invalid",
		"GIT_COMMITTER_NAME":  "init test",
		"GIT_COMMITTER_EMAIL": "init@test.invalid",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   filepath.Join(repo, "gitconfig"),
	} {
		t.Setenv(k, v)
	}
	return repo
}

// TestInitRefusesOutsideAWorktree: nothing is created, the document carries
// no kb-root key, and the remedy is the git init the tool never runs.
func TestInitRefusesOutsideAWorktree(t *testing.T) {
	dir := t.TempDir()
	out, code := kbase(t, dir, "init")
	d := checkDocument(t, "init", out)
	if d.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] {
		t.Fatalf("outcome %s exit %d, want refused exit %d\n%s", d.Outcome, code, exitCodes[toolresult.Refused], out)
	}
	if slices.Contains(d.Keys, "kb-root") {
		t.Errorf("refusal before birth carries a kb-root key: %q", d.Keys)
	}
	if !strings.Contains(out, `check: "worktree"`) || !strings.Contains(out, `remedy: "git init"`) {
		t.Errorf("refusal names no worktree check with remedy git init:\n%s", out)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Errorf("nothing may be created outside a worktree, found %q (%v)", entries, err)
	}
}

// TestInitRefusesAnyExistingKBRoot: creation is birth-once and the refusal
// fires at every state an existing kb-root can be in — empty included.
func TestInitRefusesAnyExistingKBRoot(t *testing.T) {
	t.Run("empty tree", func(t *testing.T) {
		repo := gitWorktree(t)
		if err := os.Mkdir(filepath.Join(repo, "kb-root"), 0o755); err != nil {
			t.Fatal(err)
		}
		out, code := kbase(t, repo, "init")
		d := checkDocument(t, "init", out)
		if d.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] {
			t.Fatalf("empty kb-root: outcome %s exit %d\n%s", d.Outcome, code, out)
		}
		if !strings.Contains(out, `check: "kb-root"`) {
			t.Errorf("empty kb-root refused under no kb-root check:\n%s", out)
		}
	})
	for _, name := range []string{"file", "populated"} {
		t.Run(name, func(t *testing.T) {
			repo := gitWorktree(t)
			p := filepath.Join(repo, "kb-root")
			if name == "file" {
				if err := os.WriteFile(p, []byte("not a KB\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(filepath.Join(p, "docs"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(p, "docs", "x.md"), []byte("x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out, code := kbase(t, repo, "init")
			d := checkDocument(t, "init", out)
			if d.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] {
				t.Fatalf("existing kb-root (%s): outcome %s exit %d\n%s", name, d.Outcome, code, out)
			}
			if !strings.Contains(out, `check: "kb-root"`) {
				t.Errorf("existing kb-root (%s) refused under no kb-root check:\n%s", name, out)
			}
		})
	}
}

// slotResidue matches an unfilled {slot} marker of the shape kbdocs' fill
// consumes; a stamped document holds none.
var slotResidue = regexp.MustCompile(`\{[a-z][a-z0-9]*(?:-[a-z0-9]+)*\}`)

// treeFiles walks root and returns every regular file beneath it,
// root-relative, slash-separated, path-sorted.
func treeFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	return files
}

// TestInitBirthWritesTheBornTreeAndCommitsIt: the born tree's file set is
// exactly the written list, both directions; the bytes init is the sole
// author of stand byte-exact — entry-point.md and the zero-node
// subtree-aggregates row against their fixtures, CLAUDE.md against the
// redirect constant it exists to be (the unmigrated_agents_file reader);
// AGENTS.md and CONVENTIONS.md stand non-empty and hold no unfilled slot;
// the authored pin is asserted where the template puts it, AGENTS.md's
// {scope-pin}; the derived index's six files stand and are named
// in written, the five empty ones' bytes not pinned. The birth commit lands —
// its branch ref standing, since cmd may not exec git to read the subject
// back — and re-issue refuses. The subject is held off the trail by
// TestBirthSubjectIsOffTheTrail.
func TestInitBirthWritesTheBornTreeAndCommitsIt(t *testing.T) {
	// Resolve the fixture dir to an absolute path before kbase() chdirs the
	// test into the staged repo.
	fixture, err := filepath.Abs(filepath.Join("..", "test_data", "fixtures", "init", "skeleton"))
	if err != nil {
		t.Fatal(err)
	}
	repo := gitWorktree(t)
	out, code := kbase(t, repo, "init")
	d := checkDocument(t, "init", out)
	if d.Outcome != toolresult.Done || code != 0 {
		t.Fatalf("birth: outcome %s exit %d\n%s", d.Outcome, code, out)
	}
	written := anyStrings(d.Values["written"])
	sorted := slices.Clone(written)
	if !slices.IsSorted(sorted) || !slices.Equal(sorted, written) {
		t.Errorf("written is not path-sorted: %q", written)
	}
	asSet := slices.Clone(written)
	slices.Sort(asSet)
	if !slices.Equal(asSet, slices.Compact(asSet)) {
		t.Errorf("written holds duplicates: %q", written)
	}
	if removed := anyStrings(d.Values["removed"]); len(removed) != 0 {
		t.Errorf("birth removes nothing, written removed %q", removed)
	}
	if born := treeFiles(t, filepath.Join(repo, "kb-root")); !slices.Equal(born, written) {
		t.Errorf("born tree and written differ, born %q written %q", born, written)
	}
	for _, name := range kb.IndexFiles {
		rel := kb.IndexDir + "/" + kb.IndexFileName(name)
		if _, err := os.Stat(filepath.Join(repo, "kb-root", filepath.FromSlash(rel))); err != nil {
			t.Errorf("the derived index lacks %s: %v", rel, err)
		}
		if !slices.Contains(written, rel) {
			t.Errorf("written lacks %s: %q", rel, written)
		}
	}
	for _, rel := range []string{kb.EntryPointFile, kb.IndexDir + "/" + kb.IndexFileName(kb.IndexSubtreeAggregates)} {
		got, err := os.ReadFile(filepath.Join(repo, "kb-root", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("born tree lacks %s: %v", rel, err)
		}
		want, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s differs from its fixture", rel)
		}
	}
	redirect, err := os.ReadFile(filepath.Join(repo, "kb-root", kb.AgentsRedirectFile))
	if err != nil {
		t.Fatalf("born tree lacks %s: %v", kb.AgentsRedirectFile, err)
	}
	if string(redirect) != kb.AgentsRedirect+"\n" {
		t.Errorf("%s is not the redirect constant plus newline: %q", kb.AgentsRedirectFile, redirect)
	}
	for _, doc := range []string{kb.AgentsFile, kbdocs.ConventionsFile} {
		text, err := os.ReadFile(filepath.Join(repo, "kb-root", doc))
		if err != nil {
			t.Fatalf("born tree lacks %s: %v", doc, err)
		}
		if len(text) == 0 {
			t.Errorf("%s is empty at birth", doc)
		}
		if m := slotResidue.FindString(string(text)); m != "" {
			t.Errorf("%s holds an unfilled slot %s", doc, m)
		}
	}
	agents, err := os.ReadFile(filepath.Join(repo, "kb-root", kb.AgentsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), kbdocs.AuthoredKBPin) {
		t.Errorf("%s lacks the authored pin", kb.AgentsFile)
	}
	// The birth commit landed where git writes a first commit: the branch
	// ref. Reading the commit back needs git, which only ledger may exec, so
	// the ref standing is the exec-free proof commit ran.
	if _, err := os.Stat(filepath.Join(repo, ".git", "refs", "heads", "main")); err != nil {
		t.Errorf("the birth commit left no branch ref: %v", err)
	}

	again, code := kbase(t, repo, "init")
	d = checkDocument(t, "init", again)
	if d.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] || !strings.Contains(again, `check: "kb-root"`) {
		t.Fatalf("re-issue at a born KB: outcome %s exit %d\n%s", d.Outcome, code, again)
	}
}

// TestBirthSubjectIsOffTheTrail: ledger's Trail reads only subjects opening
// with ledger.Prefix, so a birth subject that opened with it would put the
// birth commit on build's trail and lift build's refusal over a created KB.
// ledger_test.go pins that a plain subject leaves Trail empty.
func TestBirthSubjectIsOffTheTrail(t *testing.T) {
	if strings.HasPrefix(birthSubject, ledger.Prefix) {
		t.Errorf("birth subject %q opens with the trail prefix %q", birthSubject, ledger.Prefix)
	}
}

// TestInitRefusesATitleSpanningLines: a --title holding a line break is a
// usage refusal and nothing is written, kb-root/ included; where kb-root/
// already stands, the refusal lists both items.
func TestInitRefusesATitleSpanningLines(t *testing.T) {
	repo := gitWorktree(t)
	if err := os.Mkdir(filepath.Join(repo, "kb-root"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, _ := kbase(t, repo, "init", "--title", "X\nY")
	if d := checkDocument(t, "init", out); d.Outcome != toolresult.Refused || !strings.Contains(out, `check: "usage"`) || !strings.Contains(out, `check: "kb-root"`) {
		t.Errorf("--title spanning lines over a standing kb-root/: outcome %s, want both refusal items\n%s", d.Outcome, out)
	}

	for _, title := range []string{"X\n- [v](v/index.md)", "X\r\nY", "X\rY", "X\n"} {
		repo := gitWorktree(t)
		out, code := kbase(t, repo, "init", "--title", title)
		d := checkDocument(t, "init", out)
		if d.Outcome != toolresult.Refused || code != exitCodes[toolresult.Refused] || !strings.Contains(out, `check: "usage"`) || !strings.Contains(out, `key: "--title"`) {
			t.Errorf("--title %q: outcome %s exit %d, want a usage refusal naming --title\n%s", title, d.Outcome, code, out)
		}
		if _, err := os.Lstat(filepath.Join(repo, "kb-root")); !os.IsNotExist(err) {
			t.Errorf("--title %q: kb-root/ stands after the refusal (%v)", title, err)
		}
	}
}

// findOnPATH is exec.LookPath without exec: the first executable of name on
// PATH, by stat, or "" when there is none. cmd may not exec; this lets a
// test relocate a binary it only needs to exist on PATH for another package
// to exec.
func findOnPATH(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if p := filepath.Join(dir, name); isExecutable(p) {
			return p
		}
	}
	return ""
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}

// TestInitTitleBecomesTheHeading: --title names the entry point's heading;
// the born default in TestInitBirthWritesTheBornTreeAndCommitsIt is the
// repository's directory name.
func TestInitTitleBecomesTheHeading(t *testing.T) {
	repo := gitWorktree(t)
	out, code := kbase(t, repo, "init", "--title", "Scratch Named")
	if d := checkDocument(t, "init", out); d.Outcome != toolresult.Done || code != 0 {
		t.Fatalf("named birth: outcome %s exit %d\n%s", d.Outcome, code, out)
	}
	text, err := os.ReadFile(filepath.Join(repo, "kb-root", "entry-point.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(text), "# Scratch Named\n") {
		t.Errorf("entry point does not end with the named heading: %q", text)
	}
}

// TestInitWithoutDotWritesThePlaceholder: PATH keeps git and loses dot, so
// the birth still lands and claim-graph.svg stands as the placeholder, named
// in written, with the lack said on stderr.
func TestInitWithoutDotWritesThePlaceholder(t *testing.T) {
	repo := gitWorktree(t)
	git := findOnPATH("git")
	if git == "" {
		t.Skip("no git to serve the birth commit")
	}
	// A PATH holding git alone: dot shares git's directory here, so git is
	// linked into a dir of its own and that becomes the whole PATH. git
	// resolves its core programs off its own real path, not PATH.
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	out, code := kbase(t, repo, "init")
	d := checkDocument(t, "init", out)
	if d.Outcome != toolresult.Done || code != 0 {
		t.Fatalf("birth without dot: outcome %s exit %d\n%s", d.Outcome, code, out)
	}
	if !slices.Contains(anyStrings(d.Values["written"]), "claim-graph.svg") {
		t.Errorf("the placeholder is not named in written: %q", anyStrings(d.Values["written"]))
	}
	svg, err := os.ReadFile(filepath.Join(repo, "kb-root", "claim-graph.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(svg), "no Graphviz dot on PATH") {
		t.Errorf("claim-graph.svg is not the placeholder: %q", svg)
	}
}

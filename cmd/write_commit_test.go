package main

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
)

// The pre-flight commits' labels, as page §2.2 spells them.
const (
	outsideIndexLabel = "kb-write: kb-root/ changes outside .index/ made before this batch"
	recordsLabel      = "kb-write: record changes made before this batch"
)

// authorEdit is a.md of maintenanceRepo as an author edits it by hand.
const authorEdit = "[↑ KB](entry-point.md)\n\n# A\n\nAn author's edit.\n"

// committedKB is maintenanceRepo's KB brought up to date by update-format and
// committed, so the worktree is clean.
func committedKB(t *testing.T) string {
	t.Helper()
	repo := maintenanceRepo(t)
	updateFormatKB(t, repo)
	commitKB(t, repo)
	return repo
}

// gitIn runs git in repo, failing t unless it succeeds, and returns its
// stdout.
func gitIn(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// refuseCommits installs a pre-commit hook in repo that refuses every commit.
func refuseCommits(t *testing.T, repo string) {
	t.Helper()
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\necho the hook refuses >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// writeBatch runs op's batch from repo, values on stdin and --commit as given,
// and reads its result against the documented keys.
func writeBatch(t *testing.T, repo, op, values string, commit bool) resultDoc {
	t.Helper()
	var out bytes.Buffer
	if _, err := runWriteOp(writeOpOptions{Op: op, WorkDir: repo, Stdin: strings.NewReader(values), Commit: commit,
		Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()}); err != nil {
		t.Fatal(err)
	}
	return checkDocument(t, op, out.String())
}

// committedPaths is the paths commit changed, sorted, a rename as its two
// sides.
func committedPaths(t *testing.T, repo, commit string) []string {
	t.Helper()
	return strings.Fields(gitIn(t, repo, "show", "--name-only", "--no-renames", "--format=", commit))
}

// TestACommittingBatchCommitsBeforeAndAfter: a hand-edited leaf, and the
// records an earlier batch run without commit left, each get their own
// pre-flight commit; the batch commit holds exactly the batch's paths, and a
// revert of it restores kb-root/ as it stood before the batch.
func TestACommittingBatchCommitsBeforeAndAfter(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{"kb-root/a.md": authorEdit})
	if d := writeBatch(t, repo, "add-axiom", "entry:\n- label: S1\n  title: A rule\n", false); d.Outcome != toolresult.Done {
		t.Fatalf("the uncommitted batch: %s\n%v", d.Outcome, d.Values)
	}
	prior := tree(t, repo)

	d := writeBatch(t, repo, "write", "commit: true\nentry:\n- operation: add-axiom\n  label: S2\n  title: Another rule\n", false)
	commits := d.items("commits")
	var labels []string
	for _, c := range commits {
		labels = append(labels, c["label"].(string))
	}
	var batchLabel string
	if entries := d.items("entries"); len(entries) == 1 {
		batchLabel = "kb-write: add-axiom " + entries[0]["id"].(string)
	}
	if d.Outcome != toolresult.Done || !slices.Equal(labels, []string{outsideIndexLabel, recordsLabel, batchLabel}) {
		t.Fatalf("outcome %s, commit labels %q; want done with the two pre-flight commits and the batch's\n%v", d.Outcome, labels, d.Values)
	}
	index := kb.KBDir + "/" + kb.IndexDir + "/"
	for i, c := range commits {
		paths := anyStrings(c["paths"])
		if got := committedPaths(t, repo, c["commit"].(string)); !slices.Equal(got, paths) {
			t.Errorf("commit %d holds %q, its result names %q", i+1, got, paths)
		}
		if subject := strings.TrimSpace(gitIn(t, repo, "log", "-1", "--format=%s", c["commit"].(string))); subject != labels[i] {
			t.Errorf("commit %d's subject is %q, want its label %q", i+1, subject, labels[i])
		}
		inIndex := slices.ContainsFunc(paths, func(p string) bool { return strings.HasPrefix(p, index) })
		switch i {
		case 0:
			if !slices.Contains(paths, kb.KBDir+"/a.md") || inIndex {
				t.Errorf("the first pre-flight commit took %q, want a.md and nothing under %s", paths, index)
			}
		case 1:
			if len(paths) == 0 || slices.ContainsFunc(paths, func(p string) bool { return !strings.HasPrefix(p, index) }) {
				t.Errorf("the second pre-flight commit took %q, want the records alone", paths)
			}
		}
	}
	var batch []string
	for _, p := range append(anyStrings(d.Values["written"]), anyStrings(d.Values["removed"])...) {
		batch = append(batch, kb.KBDir+"/"+p)
	}
	slices.Sort(batch)
	if got := committedPaths(t, repo, commits[2]["commit"].(string)); !slices.Equal(got, batch) {
		t.Errorf("the batch commit holds %q, want exactly the batch's %q", got, batch)
	}
	if status := gitIn(t, repo, "status", "--porcelain"); status != "" {
		t.Errorf("git status after the committing batch:\n%s", status)
	}
	gitIn(t, repo, "revert", "--no-edit", commits[2]["commit"].(string))
	if after := tree(t, repo); !maps.Equal(after, prior) {
		t.Errorf("reverting the batch commit left kb-root/ other than it stood before the batch")
	}
}

// TestARefusedBatchCommitLeavesTheBatchWritten: a hook refusing the batch
// commit leaves the batch written, outcome done, with uncommitted saying why
// and no commit made.
func TestARefusedBatchCommitLeavesTheBatchWritten(t *testing.T) {
	repo := committedKB(t)
	head := gitIn(t, repo, "rev-parse", "HEAD")
	refuseCommits(t, repo)
	d := writeBatch(t, repo, "add-axiom", "entry:\n- label: S1\n  title: A rule\n", true)
	uncommitted, _ := d.Values["uncommitted"].(string)
	if d.Outcome != toolresult.Done || len(d.items("commits")) != 0 || !strings.Contains(uncommitted, "the hook refuses") {
		t.Errorf("outcome %s, commits %v, uncommitted %q; want done, none, and the hook's refusal\n%v", d.Outcome, d.Values["commits"], uncommitted, d.Values)
	}
	if gitIn(t, repo, "rev-parse", "HEAD") != head {
		t.Error("a commit was made")
	}
	if !strings.Contains(tree(t, repo)[kb.KBDir+"/"+kb.GraphNodesFile], `label: "S1"`) {
		t.Error("the batch was not written")
	}
}

// TestARefusedPreflightWritesNothing: a hook refusing a pre-flight commit
// stops the batch before it writes: outcome failed, kb-root/ as it stood, no
// commit.
func TestARefusedPreflightWritesNothing(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{"kb-root/a.md": authorEdit})
	head := gitIn(t, repo, "rev-parse", "HEAD")
	before := tree(t, repo)
	refuseCommits(t, repo)
	d := writeBatch(t, repo, "add-axiom", "entry:\n- label: S1\n  title: A rule\n", true)
	if d.Outcome != toolresult.Failed || len(d.items("commits")) != 0 {
		t.Errorf("outcome %s, commits %v; want failed with none\n%v", d.Outcome, d.Values["commits"], d.Values)
	}
	if !maps.Equal(tree(t, repo), before) {
		t.Error("the batch wrote though its pre-flight commit failed")
	}
	if gitIn(t, repo, "rev-parse", "HEAD") != head {
		t.Error("a commit was made")
	}
}

// TestACommittingDryRunCommitsNothing: dry-run beside commit commits nothing,
// the author's edit included, and writes nothing.
func TestACommittingDryRunCommitsNothing(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{"kb-root/a.md": authorEdit})
	head := gitIn(t, repo, "rev-parse", "HEAD")
	before := tree(t, repo)
	d := writeBatch(t, repo, "add-axiom", "commit: true\ndry-run: true\nentry:\n- label: S1\n  title: A rule\n", false)
	if d.Outcome != toolresult.Done || len(d.items("commits")) != 0 {
		t.Errorf("outcome %s, commits %v; want done with none", d.Outcome, d.Values["commits"])
	}
	if gitIn(t, repo, "rev-parse", "HEAD") != head || !maps.Equal(tree(t, repo), before) {
		t.Error("a committing dry run committed or wrote")
	}
}

// TestACommittingBatchOutsideAWorktreeIsRefused: where git finds no worktree,
// a committing batch is refused, check worktree, remedy git init, with
// nothing written.
func TestACommittingBatchOutsideAWorktreeIsRefused(t *testing.T) {
	repo := committedKB(t)
	if err := os.RemoveAll(filepath.Join(repo, ".git", "objects")); err != nil {
		t.Fatal(err)
	}
	before := tree(t, repo)
	d := writeBatch(t, repo, "add-axiom", "entry:\n- label: S1\n  title: A rule\n", true)
	if items := d.items(toolresult.RefusalsKey); d.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != checkWorktree || items[0]["remedy"] != "git init" {
		t.Errorf("outcome %s, refusals %v; want one worktree refusal, remedy git init", d.Outcome, d.Values[toolresult.RefusalsKey])
	}
	if !maps.Equal(tree(t, repo), before) {
		t.Error("the refused batch wrote")
	}
	d = writeBatch(t, repo, "add-axiom", "dry-run: true\nentry:\n- label: S1\n  title: A rule\n", true)
	if items := d.items(toolresult.RefusalsKey); d.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != checkWorktree || d.Values["dry-run"] != true {
		t.Errorf("a committing dry run: outcome %s, dry-run %v, refusals %v; want one worktree refusal, dry-run true", d.Outcome, d.Values["dry-run"], d.Values[toolresult.RefusalsKey])
	}
	if !maps.Equal(tree(t, repo), before) {
		t.Error("the refused dry run wrote")
	}
	d = writeBatch(t, t.TempDir(), "add-axiom", "entry: []\n", true)
	if items := d.items(toolresult.RefusalsKey); len(items) != 1 || items[0]["check"] != checkWorktree || items[0]["remedy"] != "git init" {
		t.Errorf("a batch with no repository: refusals %v, want one worktree refusal, remedy git init", d.Values[toolresult.RefusalsKey])
	}
}

// nestedKB is an outer repository git reads holding inner/, whose .git is an
// empty directory git does not accept and, where withKB, whose kb-root/ is a
// committed KB's: from inner/, git reads the outer repository and kbase the
// inner one.
func nestedKB(t *testing.T, withKB bool) (outer, inner string) {
	t.Helper()
	src := committedKB(t)
	outer = t.TempDir()
	gitRepo(t, outer)
	inner = filepath.Join(outer, "inner")
	if err := os.MkdirAll(filepath.Join(inner, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withKB {
		if err := os.CopyFS(filepath.Join(inner, kb.KBDir), os.DirFS(filepath.Join(src, kb.KBDir))); err != nil {
			t.Fatal(err)
		}
	}
	return outer, inner
}

// sameDir is whether path, a result's path value, names dir, symbolic links
// resolved.
func sameDir(t *testing.T, path any, dir string) bool {
	t.Helper()
	p, ok := path.(string)
	if !ok {
		return false
	}
	got, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return got == want
}

// TestEveryGitCallerIsRefusedWhereGitReadsAnotherRepository: from inner/ of
// nestedKB, every command that runs git over the KB is refused, one worktree
// item at inner/, remedy git init there, and writes nothing.
func TestEveryGitCallerIsRefusedWhereGitReadsAnotherRepository(t *testing.T) {
	for _, tc := range []struct {
		name   string
		withKB bool
		run    func(t *testing.T, inner string) resultDoc
	}{
		{"a committing batch", true, func(t *testing.T, inner string) resultDoc {
			return writeBatch(t, inner, "write", "commit: true\nentry: []\n", false)
		}},
		{"a committing dry run", true, func(t *testing.T, inner string) resultDoc {
			d := writeBatch(t, inner, "write", "commit: true\ndry-run: true\nentry: []\n", false)
			if d.Values["dry-run"] != true {
				t.Errorf("dry-run %v, want true", d.Values["dry-run"])
			}
			return d
		}},
		{"update-format", true, func(t *testing.T, inner string) resultDoc {
			_, out := runMaintenance(t, runUpdateFormat, inner)
			return checkDocument(t, "update-format", out)
		}},
		{"init", false, func(t *testing.T, inner string) resultDoc {
			out, _ := kbase(t, inner, "init")
			return checkDocument(t, "init", out)
		}},
		{"build", true, func(t *testing.T, inner string) resultDoc {
			out, _ := kbase(t, inner, "build", "paper.tex", "--config-dir", t.TempDir(), "--no-inference", "--state-dir", filepath.Join(t.TempDir(), "state"))
			return checkDocument(t, "build", out)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outer, inner := nestedKB(t, tc.withKB)
			before := tree(t, outer)
			d := tc.run(t, inner)
			items := d.items(toolresult.RefusalsKey)
			if d.Outcome != toolresult.Refused || len(items) != 1 || items[0]["check"] != checkWorktree || !sameDir(t, items[0]["path"], inner) || items[0]["remedy"] != "git init "+items[0]["path"].(string) {
				t.Errorf("outcome %s, refusals %v; want one worktree refusal at %s, remedy git init there\n%v", d.Outcome, d.Values[toolresult.RefusalsKey], inner, d.Values)
			}
			if !maps.Equal(tree(t, outer), before) {
				t.Error("the refused command wrote")
			}
		})
	}
}

// TestCommittingBatchLeavesOutsideKBRootAlone: a committing batch commits
// kb-root/ alone, leaving a staged and an unstaged change outside it as they
// stood.
func TestCommittingBatchLeavesOutsideKBRootAlone(t *testing.T) {
	repo := committedKB(t)
	writeRepo(t, repo, map[string]string{"notes.txt": "staged\n", "other.txt": "unstaged\n", "kb-root/a.md": authorEdit})
	gitIn(t, repo, "add", "notes.txt")
	d := writeBatch(t, repo, "write", "commit: true\nentry:\n- operation: add-axiom\n  label: S9\n  title: A probe rule\n", false)
	commits := d.items("commits")
	if len(commits) != 2 {
		t.Fatalf("commits %v, want the outside-.index pre-flight and the batch's\n%v", d.Values["commits"], d.Values)
	}
	for _, c := range commits {
		for _, p := range committedPaths(t, repo, c["commit"].(string)) {
			if !strings.HasPrefix(p, "kb-root/") {
				t.Errorf("commit %q holds %s, outside kb-root/", c["label"], p)
			}
		}
	}
	if staged := strings.TrimSpace(gitIn(t, repo, "diff", "--cached", "--name-only")); staged != "notes.txt" {
		t.Errorf("staged after the batch: %q, want notes.txt alone", staged)
	}
}

// TestUpdateFormatCommit: --commit makes the conversion one commit, label
// kb-write: update-format <from> → <to>, the moved build record's rename
// included, leaving the worktree clean; a re-run commits nothing.
func TestUpdateFormatCommit(t *testing.T) {
	repo := fixtureRepo(t, "score-kb")
	from := buildrecords.RootRecordFile(buildrecords.UnmarkedFile)
	writeRepo(t, repo, map[string]string{from: "about: the unmarked record\nplanned: null\npairs: []\n"})
	commitKB(t, repo)
	stamped := formatOf(t, repo)

	run := func() resultDoc {
		t.Helper()
		var out bytes.Buffer
		if _, err := runUpdateFormat(maintenanceOptions{WorkDir: repo, Commit: true, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()}); err != nil {
			t.Fatal(err)
		}
		return checkDocument(t, "update-format", out.String())
	}
	d := run()
	commits := d.items("commits")
	label := "kb-write: update-format " + stamped + " → " + kb.FormatVersion
	if d.Outcome != toolresult.Done || len(commits) != 1 || commits[0]["label"] != label {
		t.Fatalf("outcome %s, commits %v; want done with one commit labelled %q", d.Outcome, d.Values["commits"], label)
	}
	paths := anyStrings(commits[0]["paths"])
	if !slices.Contains(paths, from) || !slices.Equal(paths, committedPaths(t, repo, commits[0]["commit"].(string))) {
		t.Errorf("the conversion's commit names %q, want what it holds, the moved record's root path %s among them", paths, from)
	}
	if status := gitIn(t, repo, "status", "--porcelain"); status != "" {
		t.Errorf("git status after update-format --commit:\n%s", status)
	}
	if d := run(); d.Outcome != toolresult.Unchanged || len(d.items("commits")) != 0 {
		t.Errorf("the re-run: outcome %s, commits %v; want unchanged with none", d.Outcome, d.Values["commits"])
	}
}

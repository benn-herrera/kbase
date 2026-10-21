package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kbase/internal/buildrecords"
	"kbase/internal/claimgraph"
	"kbase/internal/config"
	"kbase/internal/filelock"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/latex/pandoc"
	"kbase/internal/ledger"
	"kbase/internal/log"
	"kbase/internal/records"
	"kbase/internal/result"
	"kbase/internal/write"
)

func TestResolveStage(t *testing.T) {
	for name, want := range map[string]string{
		"phase-3a": "phase-3a", "  Validation   Gate ": "phase-3a", "DEPENDS-ATTRIBUTED": "depends-attributed",
		"claim discovery":         "claims-discovered",
		"co-linear claims culled": "claims-culled",
	} {
		if s, ok := ResolveStage(name); !ok || s.ID != want {
			t.Errorf("ResolveStage(%q) = %v, %t; want %s", name, s, ok, want)
		}
	}
	if _, ok := ResolveStage("phase-5"); ok {
		t.Error("a name outside the vocabulary must resolve to no stage")
	}
}

func TestDroppedRowsFollowInference(t *testing.T) {
	var dropped []string
	for _, r := range rows {
		if !r.applies(true) {
			dropped = append(dropped, r.id)
		}
	}
	if strings.Join(dropped, ",") != "discover.build,unmarked.build,cull.build,ov.docs" {
		t.Errorf("rows dropped under --no-inference = %q, want the four that spend it", dropped)
	}
}

func TestKBRootState(t *testing.T) {
	kbRoot := filepath.Join(t.TempDir(), "kb-root")
	check := func(want string) {
		t.Helper()
		if got, err := kbRootState(kbRoot); err != nil || got != want {
			t.Errorf("kbRootState = %q, %v; want %q", got, err, want)
		}
	}
	check(kbRootAbsent)
	if err := os.MkdirAll(filepath.Join(kbRoot, ".index"), 0o755); err != nil {
		t.Fatal(err)
	}
	check(kbRootSpineOnly)
	if err := os.WriteFile(filepath.Join(kbRoot, "entry-point.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check(kbRootPopulated)
}

func TestShellJoin(t *testing.T) {
	got := ShellJoin([]string{"kbase", "build", "/a b/p.tex", "--state-dir", "/s/it's"})
	if want := `kbase build '/a b/p.tex' --state-dir '/s/it'"'"'s'`; got != want {
		t.Errorf("ShellJoin = %s, want %s", got, want)
	}
}

// paper is a volume root whose build mints a lemma, a theorem resting on it
// and a labelled equation the theorem references.
const paper = `\documentclass{article}
\newtheorem{theorem}{Theorem}
\newtheorem{lemma}{Lemma}
\title{A Small Paper}
\author{A. Author}
\begin{document}
\maketitle
\begin{abstract}
We prove a small theorem.
\end{abstract}
\section{Introduction}
Some opening prose.
\section{Results}
\begin{lemma}\label{lem:base}
Every widget is a gadget.
\end{lemma}
\begin{proof}
Obvious.
\end{proof}
\begin{equation}\label{eq:main}
a + b = c
\end{equation}
\begin{theorem}\label{thm:main}
By Lemma~\ref{lem:base} and \eqref{eq:main}, every gadget is a widget.
\end{theorem}
\begin{proof}
Apply Lemma~\ref{lem:base} to \eqref{eq:main}.
\end{proof}
\end{document}
`

// fixture is a repository holding the paper and a state store beside it.
type fixture struct {
	repo, state string
}

// newFixture is a fresh repository with the paper, git's identity and global
// configuration fixed. The repository is made as git init makes one, so
// nothing here starts a process.
func newFixture(t *testing.T) fixture {
	t.Helper()
	if _, err := pandoc.Preflight(context.Background()); err != nil {
		t.Skipf("pandoc is not usable here: %v", err)
	}
	base := t.TempDir()
	gitEnv(t, base)
	f := fixture{repo: filepath.Join(base, "repo"), state: filepath.Join(base, "state")}
	makeRepo(t, f.repo)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), paper)
	return f
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitEnv is git's identity, its global configuration and the state store's
// home, all under base.
func gitEnv(t *testing.T, base string) {
	t.Helper()
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(base, "gitconfig"), "XDG_STATE_HOME": filepath.Join(base, "xdg"),
	} {
		t.Setenv(k, v)
	}
}

// makeRepo is the repository git init makes, without starting a process.
func makeRepo(t *testing.T, repo string) {
	t.Helper()
	for _, d := range []string{".git/objects", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
}

func (f fixture) options() Options {
	return Options{VolumeRoots: []string{filepath.Join(f.repo, "paper.tex")}, NoInference: true, StateDir: f.state, WorkDir: f.repo}
}

func (f fixture) build(t *testing.T, opts Options) (string, []result.Field) {
	t.Helper()
	if opts.now == nil {
		opts.now = time.Now
	}
	return Run(context.Background(), opts)
}

func (f fixture) ledger(t *testing.T) *ledger.Repo {
	t.Helper()
	r, err := ledger.Open(f.repo, ownedPaths, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f fixture) trailStages(t *testing.T) []string {
	t.Helper()
	trail, err := f.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	var stages []string
	for _, e := range trail {
		stages = append([]string{e.Stage}, stages...)
	}
	return stages
}

func field(fields []result.Field, key string) any {
	for _, f := range fields {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// roleValue is one key's value in a role record, nil where it carries none.
func roleValue(record result.Record, key string) any {
	for _, f := range record {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// carries is whether the document holds key at all, an absent key told apart
// from one holding an empty value.
func carries(fields []result.Field, key string) bool {
	return slices.ContainsFunc(fields, func(f result.Field) bool { return f.Key == key })
}

// details is the details of the items under key.
func details(fields []result.Field, key string) []string {
	items, _ := field(fields, key).([]result.Item)
	var out []string
	for _, it := range items {
		out = append(out, it.Detail)
	}
	return out
}

func stageIDs(through string) []string {
	var ids []string
	for _, s := range Stages {
		ids = append(ids, s.ID)
		if s.ID == through {
			break
		}
	}
	return ids
}

// snapshot is every file a build owns, by repository-relative path.
func snapshot(t *testing.T, repo string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, owned := range ownedPaths {
		root := filepath.Join(repo, owned)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && p == root {
				return nil
			}
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p)
			rel, _ := filepath.Rel(repo, p)
			files[filepath.ToSlash(rel)] = string(b)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// sameModuloIDs fails on every way the build in repo differs from the
// uninterrupted build in want, compared modulo node ids.
func sameModuloIDs(t *testing.T, repo, want string) {
	t.Helper()
	checks, err := claimgraph.CompareModuloIDs(claimgraph.BuiltBy("this build", repo), claimgraph.BuiltBy("the uninterrupted build", want))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range checks {
		for _, d := range c.Differences {
			t.Errorf("%s: %s", c.Name, d)
		}
	}
}

// referenceFiles is the uninterrupted build's owned files, built once per
// test binary.
var referenceFiles map[string]string

// reference is a repository holding the uninterrupted build's owned files.
func reference(t *testing.T) string {
	t.Helper()
	if referenceFiles == nil {
		f := newFixture(t)
		if outcome, fields := f.build(t, f.options()); outcome != result.Done {
			t.Fatalf("the reference build = %s %v", outcome, fields)
		}
		referenceFiles = snapshot(t, f.repo)
	}
	dir := t.TempDir()
	for p, text := range referenceFiles {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(p)), text)
	}
	return dir
}

func TestBuildEndToEnd(t *testing.T) {
	f := newFixture(t)
	outcome, fields := f.build(t, f.options())
	if outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if through, _ := field(fields, "through").(string); through != Stages[len(Stages)-1].ID || carries(fields, "resume") {
		t.Errorf("a build walked to the end reports through %q and resume %v, want the last stage and none: it has nothing left to continue", through, field(fields, "resume"))
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer", "roles"} {
		if carries(fields, key) {
			t.Errorf("a build spending no inference = %v, want no %s: it resolved no endpoint", fields, key)
		}
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("")) {
		t.Errorf("trail = %q, want every stage in order", got)
	}
	trail, _ := f.ledger(t).Trail()
	bodies := map[string]string{}
	for _, e := range trail {
		bodies[e.Stage] = e.Body
	}
	for _, s := range stageIDs("") {
		if in, ok := recordedInputs(bodies[s]); !ok || !slices.Equal(in.roots, []string{"paper.tex"}) || in.bibliographies != nil {
			t.Errorf("%s body = %q, want it to record the one volume root and no bibliography", s, bodies[s])
		}
	}
	for _, s := range []string{"claims-discovered", "references-found", "claims-culled", "overview-drafted"} {
		if !strings.HasPrefix(bodies[s], "--no-inference: ") {
			t.Errorf("%s body = %q, want the dropped rows named", s, bodies[s])
		}
	}
	files := snapshot(t, f.repo)
	if want := "\nplanned: null\npairs: []\n"; !strings.HasSuffix(files[buildrecords.UnmarkedFile], want) {
		t.Errorf("%s = %q, want it as the declared pass wrote it, ending %q", buildrecords.UnmarkedFile, files[buildrecords.UnmarkedFile], want)
	}
	if want := "\nentries: []\nculls: []\nlinks: []\n"; !strings.HasSuffix(files[buildrecords.CullFile], want) {
		t.Errorf("%s = %q, want it as the declared pass wrote it, ending %q", buildrecords.CullFile, files[buildrecords.CullFile], want)
	}
	if files["kb-root/CLAUDE.md"] != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q, want exactly the redirect", files["kb-root/CLAUDE.md"])
	}
	if strings.Contains(files["kb-root/AGENTS.md"], "charter") {
		t.Error("AGENTS.md mentions a charter")
	}
	if strings.Contains(files["kb-root/AGENTS.md"], "\n\n\n") {
		t.Error("AGENTS.md carries a run of blank lines")
	}
	if _, ok := files["kb-root/README.md"]; ok {
		t.Error("a build spending no inference wrote the overview document, which it must not")
	}
	if dirty, err := f.ledger(t).Dirty(); err != nil || len(dirty) != 0 {
		t.Errorf("owned paths after the build: dirty %q, %v", dirty, err)
	}
	sameModuloIDs(t, f.repo, reference(t))

	if outcome, fields := f.build(t, f.options()); outcome != result.Unchanged {
		t.Errorf("a re-run over a finished build = %s %v, want unchanged", outcome, fields)
	} else if carries(fields, "through") || carries(fields, "resume") {
		t.Errorf("an unchanged build = %v, want neither through nor resume: it entered no stage", fields)
	}
	opts := f.options()
	opts.VolumeRoots = append(opts.VolumeRoots, filepath.Join(f.repo, "more.tex"))
	writeFile(t, opts.VolumeRoots[1], paper)
	if outcome, fields := f.build(t, opts); outcome != result.Refused || details(fields, "refusals") == nil {
		t.Errorf("a re-run over a finished build with another volume root = %s %v, want refused", outcome, fields)
	} else if carries(fields, "through") || carries(fields, "resume") {
		t.Errorf("a refusal before the walk = %v, want neither through nor resume: it recorded no boundary", fields)
	}
}

func TestStampLeavesAnAuthoredDocument(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "depends-attributed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	const authored = "# The project's own conventions\n"
	writeFile(t, filepath.Join(f.repo, kb.KBDir, kbdocs.ConventionsFile), authored)
	if _, err := f.ledger(t).Record("authored", "the project's own conventions", ""); err != nil {
		t.Fatal(err)
	}
	opts.Through = ""
	if outcome, fields := f.build(t, opts); outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	files := snapshot(t, f.repo)
	if files["kb-root/CONVENTIONS.md"] != authored {
		t.Errorf("an authored CONVENTIONS.md was rewritten: %q", files["kb-root/CONVENTIONS.md"])
	}
	if files["kb-root/AGENTS.md"] == "" || files["kb-root/CLAUDE.md"] != "@AGENTS.md\n" {
		t.Error("the absent readiness documents were not stamped beside it")
	}
}

func TestDoubleRunGuardRefusesAWrittenTree(t *testing.T) {
	f := newFixture(t)
	if outcome, fields := f.build(t, f.options()); outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	g := newFixture(t)
	for p, text := range snapshot(t, f.repo) {
		writeFile(t, filepath.Join(g.repo, filepath.FromSlash(p)), text)
	}
	outcome, fields := g.build(t, g.options())
	if items := details(fields, "refusals"); outcome != result.Refused || len(items) != 1 || !strings.Contains(items[0], kbRootPopulated) {
		t.Errorf("a fresh build over a written tree = %s %v, want refused naming it populated", outcome, fields)
	}
	if got := g.trailStages(t); len(got) != 0 {
		t.Errorf("the refused build recorded %q", got)
	}
}

func TestLockRefusesASecondBuilder(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := takeLock(f.state, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	outcome, fields := f.build(t, f.options())
	if items := details(fields, "refusals"); outcome != result.Retry || len(items) != 1 || !strings.Contains(items[0], "holds the state store") {
		t.Errorf("a second builder = %s %v, want retry naming the holder", outcome, fields)
	}
	if events, _ := readProgress(f.state); len(events) != 0 {
		t.Errorf("the refused builder wrote %d progress events", len(events))
	}
}

func TestRunLockRefusesABuildThroughAnotherStateDir(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(filepath.Dir(f.state), "other-state")
	held, err := takeRunLock(filepath.Join(f.repo, kb.KBDir), other)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	outcome, fields := f.build(t, f.options())
	items, _ := field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkLock || items[0].Path != other ||
		items[0].Remedy != "" || items[0].Detail != "a build is running; its state-dir is "+other {
		t.Errorf("a second build through another state-dir = %s %v, want refused naming %s", outcome, fields, other)
	}
	if events, _ := readProgress(f.state); len(events) != 0 {
		t.Errorf("the refused build wrote %d progress events", len(events))
	}
	if got := f.trailStages(t); len(got) != 0 {
		t.Errorf("the refused build recorded %q", got)
	}
}

// TestRunLockHeldBeforeItsStateDirIsWritten: a run lock taken and not yet
// holding its state directory is a build starting — a retry for a second
// build, no refusal for a writer — never a refusal naming an empty path.
func TestRunLockHeldBeforeItsStateDirIsWritten(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	kbRoot := filepath.Join(repo, kb.KBDir)
	held, err := filelock.Acquire(runLockPath(kbRoot), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	var r retry
	if l, err := takeRunLock(kbRoot, t.TempDir()); !errors.As(err, &r) {
		if l != nil {
			l.Release()
		}
		t.Errorf("a second build against a starting one: %v, want retry", err)
	}
	if items, err := RunningBuild(kbRoot); items != nil || err != nil {
		t.Errorf("a writer against a starting build: %v, %v; want neither refusal nor error", items, err)
	}
}

// TestStatusSeesABuildWaitingOutAWriter: a build that finds a writer holding
// the KB write lock is running to status while it waits, and walks once the
// writer lets go.
func TestStatusSeesABuildWaitingOutAWriter(t *testing.T) {
	f := newFixture(t)
	release, err := write.LockKB(filepath.Join(f.repo, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	opts := f.options()
	opts.Through = "start"
	done := make(chan string, 1)
	go func() {
		outcome, _ := f.build(t, opts)
		done <- outcome
	}()
	running := false
	for deadline := time.Now().Add(write.LockWait / 2); !running && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(f.state, progressFile)); err == nil {
			running = statusOf(t, f).state == stateRunning
		}
	}
	release()
	if outcome := <-done; outcome != result.Bounded {
		t.Errorf("the build once the writer let go = %s, want bounded", outcome)
	}
	if !running {
		t.Error("status never saw the build waiting for the writer as running")
	}
}

func TestRunLockPathFollowsAGitFile(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "wt")
	gitdir := filepath.Join(base, "main", ".git", "worktrees", "wt")
	for _, tc := range []struct{ name, dotGit, want string }{
		{"a .git directory", "", filepath.Join(repo, ".git", runLockFile)},
		{"a relative gitdir", "gitdir: ../main/.git/worktrees/wt\n", filepath.Join(gitdir, runLockFile)},
		{"an absolute gitdir", "gitdir: " + gitdir + "\n", filepath.Join(gitdir, runLockFile)},
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
				writeFile(t, filepath.Join(repo, ".git"), tc.dotGit)
			}
			if got := runLockPath(filepath.Join(repo, kb.KBDir)); got != tc.want {
				t.Errorf("runLockPath = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDirtRefusedUnlessAnInterruptedStageAccountsForIt(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	stray := filepath.Join(f.repo, kb.KBDir, "stray.md")
	writeFile(t, stray, "a hand edit\n")
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	if items, _ := field(fields, "refusals").([]result.Item); outcome != result.Refused || len(items) != 1 || items[0].Check != checkDirtyPaths || items[0].Path != "kb-root/stray.md" {
		t.Errorf("a build over unaccounted dirt = %s %v, want refused naming it", outcome, fields)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("the refusal touched the dirt: %v", err)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("spine-seed")) {
		t.Errorf("trail after the refusal = %q", got)
	}
}

// TestSpineStampsTheFormat: the spine's boundary commits an entry point
// stamped with the format version; a resume over a KB a newer kbase stamped
// is refused, naming the format.
func TestSpineStampsTheFormat(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	entry := filepath.Join(f.repo, kb.KBDir, kb.EntryPointFile)
	b, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if stamp, _, err := kb.FormatStamp(string(b)); err != nil || stamp != kb.FormatVersion {
		t.Errorf("the spine left the entry point stamped %q (%v), want %s:\n%s", stamp, err, kb.FormatVersion, b)
	}
	if dirty, err := f.ledger(t).Dirty(); err != nil || len(dirty) > 0 {
		t.Errorf("the stamp is not in the spine's boundary: dirty %q, %v", dirty, err)
	}
	writeFile(t, entry, strings.Replace(string(b), `"`+kb.FormatVersion+`"`, `"3.0.0"`, 1))
	if _, err := f.ledger(t).Record("spine-seed", "claim-graph spine seeded", ""); err != nil {
		t.Fatal(err)
	}
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	if items, _ := field(fields, "refusals").([]result.Item); outcome != result.Refused || len(items) != 1 || items[0].Check != "kb-format" {
		t.Errorf("a resume over a KB stamped 3.0.0 = %s %v, want refused naming the format", outcome, fields)
	}
}

// TestKillAndResume reproduces what a build killed inside claims-declared
// leaves — its progress entering the stage and never ending, the stage's
// writes uncommitted and one of them torn — and resumes it.
func TestKillAndResume(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	p, err := openProgress(f.state, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []event{{Event: eventRun, PID: 1, NoInference: true}, {Event: eventStage, Stage: "claims-declared", Total: 1}} {
		if err := p.emit(e); err != nil {
			t.Fatal(err)
		}
	}
	p.close()
	kbRoot := filepath.Join(f.repo, kb.KBDir)
	if _, err := claimgraph.Declared(claimgraph.Options{KBRoot: kbRoot, RepoRoot: f.repo, Records: records.Path(f.state), Logger: log.Discard()}); err != nil {
		t.Fatal(err)
	}
	torn := filepath.Join(kbRoot, kb.EntryPointFile)
	b, err := os.ReadFile(torn)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, torn, string(b[:len(b)/2]))
	dirty, err := f.ledger(t).Dirty()
	if err != nil || len(dirty) == 0 {
		t.Fatalf("the interrupted stage left no dirt: %q, %v", dirty, err)
	}

	if state := statusOf(t, f); state.state != stateInterrupted || state.current != "claims-declared" {
		t.Errorf("status after the kill = %+v, want interrupted in claims-declared", state)
	}
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	if outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	if restored, _ := field(fields, "restored").([]string); !slices.Equal(restored, dirty) {
		t.Errorf("restored %q, want the interrupted stage's dirt %q", restored, dirty)
	}
	sameModuloIDs(t, f.repo, reference(t))
}

func TestCancelLosesOnlyTheUnitInFlight(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := f.options()
	opts.now = time.Now
	opts.beforeUnit = func(stage, unit string) {
		if unit == "declared.build" {
			cancel()
		}
	}
	outcome, fields := Run(ctx, opts)
	if outcome != result.Cancelled {
		t.Fatalf("build = %s %v, want cancelled", outcome, fields)
	}
	if got := field(fields, "cancelled").(result.Record); got[0].Value != "claims-declared" || got[1].Value != "declared.build" {
		t.Errorf("cancelled = %v, want claims-declared at declared.build", got)
	}
	if resume, _ := field(fields, "resume").(string); !carries(fields, "through") || !strings.HasPrefix(resume, "kbase build ") {
		t.Errorf("a cancelled build's through = %v and resume = %q, want both: an identical re-issue continues it", field(fields, "through"), resume)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("spine-seed")) {
		t.Errorf("trail after the cancel = %q, want every stage before the one in flight", got)
	}
	if _, err := os.Stat(filepath.Join(f.state, lockFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the holder lock file stands after the cancel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", runLockFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the repository's run lock stands after the cancel: %v", err)
	}
	if s := statusOf(t, f); s.state != stateCancelled || s.current != "claims-declared" || s.done != 0 || s.resume == "" {
		t.Errorf("status after the cancel = %+v", s)
	}
	if outcome, fields := f.build(t, f.options()); outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	sameModuloIDs(t, f.repo, reference(t))
}

func TestMissingRecordsAreRegenerated(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if err := os.RemoveAll(filepath.Dir(records.Path(f.state))); err != nil {
		t.Fatal(err)
	}
	opts.Through = ""
	if outcome, fields := f.build(t, opts); outcome != result.Done {
		t.Fatalf("resume without records = %s %v", outcome, fields)
	}
	sameModuloIDs(t, f.repo, reference(t))
}

func TestMissingRecordsOverAChangedSourceFail(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if err := os.RemoveAll(filepath.Dir(records.Path(f.state))); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "Some other prose.", 1))
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	if items := details(fields, "failures"); outcome != result.Failed || len(items) != 1 || !strings.Contains(items[0], "kb-root/a-small-paper/introduction.md") {
		t.Errorf("resume over a changed source = %s %v, want failed naming the first differing path", outcome, fields)
	}
}

func TestLaunchRefusals(t *testing.T) {
	noProvider := func() (Provider, error) { return Provider{}, errors.New("no usable provider in providers.toml") }
	for _, tc := range []struct {
		name     string
		through  string
		provider func() (Provider, error)
		want     []string
	}{
		{"a model call with no provider", "claims-discovered", noProvider, []string{"discover.build (claims-discovered) calls a model, and no usable provider"}},
		{"no provider configuration at all", "depends-attributed", nil, []string{"discover.build (claims-discovered) calls a model, and no provider is configured;"}},
		{"provider-free stages", "claims-declared", noProvider, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			opts := f.options()
			opts.NoInference, opts.Through, opts.Provider = false, tc.through, tc.provider
			outcome, fields := f.build(t, opts)
			if tc.want == nil {
				if outcome != result.Bounded {
					t.Errorf("build = %s %v, want bounded", outcome, fields)
				}
				if through, _ := field(fields, "through").(string); through != tc.through {
					t.Errorf("bounded build's through = %q, want the bound it was given, %s", through, tc.through)
				}
				if carries(fields, "resume") {
					t.Errorf("bounded build = %v, want no resume: its own command repeats its --through and advances nothing", fields)
				}
				return
			}
			items := details(fields, "refusals")
			if outcome != result.Refused || len(items) != len(tc.want) {
				t.Fatalf("build = %s %q, want one refusal per %q", outcome, items, tc.want)
			}
			if carries(fields, "through") || carries(fields, "resume") {
				t.Errorf("a launch refusal = %v, want neither through nor resume: it recorded no boundary", fields)
			}
			for i, w := range tc.want {
				if !strings.Contains(items[i], w) {
					t.Errorf("refusal %q does not name %s", items[i], w)
				}
			}
			if refusals, _ := field(fields, "refusals").([]result.Item); refusals[0].Remedy != "" {
				t.Errorf("the provider refusal carries remedy %q, want none: no command clears an unconfigured provider", refusals[0].Remedy)
			}
			if got := f.trailStages(t); len(got) != 0 {
				t.Errorf("a launch refusal recorded %q", got)
			}
		})
	}
	f := newFixture(t)
	opts := f.options()
	opts.NoInference, opts.Through = false, "claims-discovered"
	const baseURLKey = "baseUrl"
	opts.Provider = func() (Provider, error) {
		return Provider{}, config.Refusal{Key: baseURLKey, Path: config.ProvidersFileName, Detail: "KBASE_API_BASE_URL is unset"}
	}
	if _, fields := f.build(t, opts); func() bool {
		items, _ := field(fields, "refusals").([]result.Item)
		return len(items) != 1 || items[0].Key != baseURLKey || items[0].Path != config.ProvidersFileName
	}() {
		t.Errorf("a provider lacking its base URL = %v, want the refusal keyed by the pool's baseUrl and providers.toml", fields)
	}
	opts = f.options()
	opts.Through = "phase-5"
	if outcome, fields := f.build(t, opts); outcome != result.Refused || !strings.Contains(details(fields, "refusals")[0], "names no stage") {
		t.Errorf("--through phase-5 = %s %v", outcome, fields)
	}
}

// TestRefusalInsideDocumentGraphCarriesThroughAndResume: a refusal a stage's
// row raises leaves the start boundary walked, so the result names the bound it
// was given and the command that continues it — unlike a launch refusal, which
// records no boundary and names neither.
func TestRefusalInsideDocumentGraphCarriesThroughAndResume(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "\\input{missing.tex}\nSome opening prose.", 1))
	outcome, fields := f.build(t, f.options())
	if items := details(fields, "refusals"); outcome != result.Refused || len(items) != 1 || !strings.Contains(items[0], "could not load include missing.tex") {
		t.Fatalf("a build over an unloadable include = %s %v, want refused naming it", outcome, fields)
	}
	if through, _ := field(fields, "through").(string); through != Stages[len(Stages)-1].ID {
		t.Errorf("through = %q, want the bound the build was given, %s", through, Stages[len(Stages)-1].ID)
	}
	if resume, _ := field(fields, "resume").(string); !strings.HasPrefix(resume, "kbase build ") {
		t.Errorf("resume = %q, want the command that continues the refused walk", resume)
	}
	if stages, _ := field(fields, "stages").([]result.Record); len(stages) != 2 || stages[0][1].Value == nil ||
		stages[1][0].Value != stageDocumentGraph || stages[1][1].Value != nil {
		t.Errorf("stages = %v, want the start boundary and the stage the refusal stopped in, uncommitted", stages)
	}
}

// TestIncludeOutsideTheVolumeRefused: an include resolving outside the volume
// root's directory is the include refusal, naming the include path, never a
// file the reader would have inlined.
func TestIncludeOutsideTheVolumeRefused(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(filepath.Dir(f.repo), "secret.tex"), "Outside the volume.\n")
	writeFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "\\input{../secret.tex}\nSome opening prose.", 1))
	outcome, fields := f.build(t, f.options())
	items, _ := field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkInclude || items[0].Path != "../secret.tex" {
		t.Fatalf("a build over an include outside the volume = %s %v, want one include refusal naming it", outcome, fields)
	}
	if !strings.Contains(items[0].Detail, "outside the volume's directory") {
		t.Errorf("the refusal = %q, want it naming the volume's directory", items[0].Detail)
	}
}

// TestFailedMidWalkCarriesThroughAndResume: a build that fails inside a stage
// has walked boundaries, so its result names the bound and the command that
// re-issues it.
func TestFailedMidWalkCarriesThroughAndResume(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	// A document under kb-root/ that the document-graph row writes nothing of:
	// the stage's reachability check finds it carrying no up-link and fails.
	opts.beforeUnit = func(_, unit string) {
		if unit == "dg.build" {
			writeFile(t, filepath.Join(f.repo, kb.KBDir, "stray.md"), "# Stray\n")
		}
	}
	outcome, fields := f.build(t, opts)
	if items := details(fields, "failures"); outcome != result.Failed || len(items) == 0 {
		t.Fatalf("a build whose document graph found a stray document = %s %v, want failed naming it", outcome, fields)
	}
	if !carries(fields, "through") || !carries(fields, "resume") {
		t.Errorf("a failure inside a stage = %v, want through and resume: the walk it stopped in is resumable", fields)
	}
}

// TestStaleIndexLockRefusedByName: a git index lock a killed commit left is
// refused, naming it, before anything is restored or walked.
func TestStaleIndexLockRefusedByName(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	lock := filepath.Join(f.repo, ".git", "index.lock")
	writeFile(t, lock, "")
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	items, _ := field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkLock || !strings.HasSuffix(items[0].Path, filepath.Join(".git", "index.lock")) {
		t.Fatalf("a build over a stale index lock = %s %+v, want refused naming the lock", outcome, items)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if outcome, fields := f.build(t, opts); outcome != result.Done {
		t.Fatalf("the build once the lock is gone = %s %v", outcome, fields)
	}
}

func TestInputsRefusal(t *testing.T) {
	trail := inputs{roots: []string{"a.tex", "b.tex"}, bibliographies: []string{"x.bib", "y.bib"}}
	for _, tc := range []struct {
		name   string
		given  inputs
		key    string
		change string
	}{
		{"the same inputs", trail, "", ""},
		{"a volume root added", inputs{[]string{"a.tex", "b.tex", "c.tex"}, trail.bibliographies}, "<volume-root>", "added c.tex"},
		{"a volume root removed", inputs{[]string{"b.tex"}, trail.bibliographies}, "<volume-root>", "removed a.tex"},
		{"the bibliographies reordered", inputs{trail.roots, []string{"y.bib", "x.bib"}}, "--bibliography", "the same paths in another order"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := inputsRefusal("/repo", trail, tc.given)
			if tc.key == "" {
				if len(items) != 0 {
					t.Errorf("refusal = %+v, want none", items)
				}
				return
			}
			if len(items) != 1 || items[0].Check != checkInputs || items[0].Key != tc.key || items[0].Path != "/repo" ||
				!strings.HasSuffix(items[0].Detail, ": "+tc.change) || items[0].Remedy == "" {
				t.Errorf("refusal = %+v, want one inputs item keyed %s ending %q", items, tc.key, tc.change)
			}
		})
	}
	if in, ok := recordedInputs(trail.body()); !ok ||
		!slices.Equal(in.roots, trail.roots) || !slices.Equal(in.bibliographies, trail.bibliographies) {
		t.Errorf("recorded inputs read back as %+v (%t), want %+v", in, ok, trail)
	}
	for _, body := range []string{"", "note: something", "--no-inference: this build spent no model call"} {
		if _, ok := recordedInputs(body); ok {
			t.Errorf("a body recording no inputs, %q, reads as recording some", body)
		}
	}
}

// TestInputsSpelledOtherwise: a volume root and a bibliography spelled
// absolute, relative to the working directory, through a symlinked parent or
// a symlinked repository root record as one repository-relative form, so a
// resume spelling them otherwise than the launch raises no inputs refusal.
func TestInputsSpelledOtherwise(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	writeFile(t, filepath.Join(repo, "papers", "paper.tex"), paper)
	writeFile(t, filepath.Join(repo, "refs.bib"), "")
	link := filepath.Join(base, "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	recorded := inputs{roots: []string{"papers/paper.tex"}, bibliographies: []string{"refs.bib"}}
	for _, c := range []struct{ name, root, workDir, volumeRoot, bibliography string }{
		{"absolute", repo, base, filepath.Join(repo, "papers", "paper.tex"), filepath.Join(repo, "refs.bib")},
		{"relative", repo, filepath.Join(repo, "papers"), "paper.tex", "../refs.bib"},
		{"through a symlinked parent", repo, base, filepath.Join("link", "papers", "paper.tex"), filepath.Join(link, "refs.bib")},
		{"from a symlinked working directory", repo, filepath.Join(link, "papers"), "paper.tex", "../refs.bib"},
		{"the repository root through a symlink", link, repo, "papers/paper.tex", "refs.bib"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := &walk{repo: &ledger.Repo{Root: c.root}, opts: Options{WorkDir: c.workDir,
				VolumeRoots: []string{c.volumeRoot}, Bibliographies: []string{c.bibliography}}}
			given := w.inputs()
			if !slices.Equal(given.roots, recorded.roots) || !slices.Equal(given.bibliographies, recorded.bibliographies) {
				t.Errorf("inputs = %+v, want %+v", given, recorded)
			}
			trail, _ := recordedInputs(recorded.body())
			if items := inputsRefusal(repo, trail, given); len(items) != 0 {
				t.Errorf("refusal = %+v, want none", items)
			}
		})
	}
}

// TestResumeWithOtherInputsRefused: a resume given a volume root the trail
// does not record is refused before anything is restored or committed; a
// trail whose newest boundary records no inputs resumes, and its next
// boundary records them.
func TestResumeWithOtherInputsRefused(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	stray := filepath.Join(f.repo, kb.KBDir, "stray.md")
	writeFile(t, stray, "a hand edit\n")
	before := snapshot(t, f.repo)

	more := filepath.Join(f.repo, "more.tex")
	writeFile(t, more, paper)
	opts.Through = ""
	opts.VolumeRoots = append(opts.VolumeRoots, more)
	outcome, fields := f.build(t, opts)
	items, _ := field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkInputs || items[0].Key != "<volume-root>" ||
		items[0].Path != f.ledger(t).Root || !strings.Contains(items[0].Detail, "added more.tex") {
		t.Fatalf("a resume with another volume root = %s %+v, want refused naming it", outcome, items)
	}
	if after := snapshot(t, f.repo); !maps.Equal(after, before) {
		t.Errorf("the refusal touched the owned paths")
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("spine-seed")) {
		t.Errorf("trail after the refusal = %q", got)
	}

	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ledger(t).Record("spine-seed", "claim-graph spine seeded", ""); err != nil {
		t.Fatal(err)
	}
	opts.Through = "claims-declared"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("a resume over a trail recording no inputs = %s %v, want it walked", outcome, fields)
	}
	trail, err := f.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	if in, ok := recordedInputs(trail[0].Body); !ok || !slices.Equal(in.roots, []string{"paper.tex", "more.tex"}) {
		t.Errorf("the next boundary's body = %q, want it to record both volume roots", trail[0].Body)
	}
}

// TestTrailOutOfOrderRefused: a trail recording a stage past one it leaves
// unrecorded was written by a build walking another stage order, and a resume
// over it is refused before anything is restored.
func TestTrailOutOfOrderRefused(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "references-found"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if _, err := f.ledger(t).Record("depends-attributed", "dependency attribution", ""); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.repo, kb.KBDir, "stray.md"), "a hand edit\n")
	before := snapshot(t, f.repo)

	opts.Through = ""
	outcome, fields := f.build(t, opts)
	items, _ := field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkTrail || items[0].Path != f.ledger(t).Root ||
		!strings.Contains(items[0].Detail, "depends-attributed") || !strings.Contains(items[0].Detail, "claims-culled") {
		t.Fatalf("a resume over an out-of-order trail = %s %+v, want refused naming both stages", outcome, items)
	}
	if restored := field(fields, "restored"); restored != nil {
		t.Errorf("the refusal restored %v", restored)
	}
	if after := snapshot(t, f.repo); !maps.Equal(after, before) {
		t.Errorf("the refusal touched the owned paths")
	}
}

// TestResumeCarriesEveryLaunchFlag: a walk refused inside its first stage hands
// out the command that continues it, restating the bibliographies,
// --no-inference, the store and the configuration directory, and leaving the
// bound off.
func TestResumeCarriesEveryLaunchFlag(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through, opts.ConfigDir = stageDocumentGraph, filepath.Join(f.repo, "config")
	opts.Bibliographies = []string{filepath.Join(f.repo, "refs.bib")}
	writeFile(t, opts.Bibliographies[0], "")
	writeFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "\\input{missing.tex}\nSome opening prose.", 1))
	outcome, fields := f.build(t, opts)
	if outcome != result.Refused {
		t.Fatalf("build = %s %v, want refused inside %s", outcome, fields, stageDocumentGraph)
	}
	resume, _ := field(fields, "resume").(string)
	for _, want := range []string{"--bibliography " + opts.Bibliographies[0], "--no-inference", "--state-dir " + f.state,
		"--config-dir " + opts.ConfigDir} {
		if !strings.Contains(resume, want) {
			t.Errorf("resume = %q, want it to carry %q", resume, want)
		}
	}
	if strings.Contains(resume, "--through") {
		t.Errorf("resume = %q carries the bound", resume)
	}
}

func TestOutsideAWorktree(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.WorkDir = t.TempDir()
	outcome, fields := f.build(t, opts)
	if items := details(fields, "refusals"); outcome != result.Refused || len(items) != 1 || !strings.Contains(items[0], "git init") {
		t.Errorf("build outside a worktree = %s %v, want refused naming git init", outcome, fields)
	}
}

type statusSnapshot struct {
	state, current, resume string
	done, total            int
	pid                    any
	recorded               int
}

func statusOf(t *testing.T, f fixture) statusSnapshot {
	t.Helper()
	outcome, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if outcome != result.Done {
		t.Fatalf("status = %s %v", outcome, fields)
	}
	s := statusSnapshot{state: field(fields, "state").(string), pid: field(fields, "pid")}
	if r, ok := field(fields, "resume").(string); ok {
		s.resume = r
	}
	if c, ok := field(fields, "current").(result.Record); ok {
		s.current, s.done, s.total = c[0].Value.(string), c[1].Value.(int), c[2].Value.(int)
	}
	for _, st := range field(fields, "stages").([]result.Record) {
		if st[1].Value != nil {
			s.recorded++
		}
	}
	return s
}

func TestStatusAtEveryState(t *testing.T) {
	f := newFixture(t)
	if s := statusOf(t, f); s.state != stateNone || s.pid != nil || s.current != stageStart || s.recorded != 0 {
		t.Errorf("before any build: %+v", s)
	}

	opts := f.options()
	opts.Through = "claims-declared"
	if outcome, _ := f.build(t, opts); outcome != result.Bounded {
		t.Fatal(outcome)
	}
	s := statusOf(t, f)
	if s.state != stateBounded || s.recorded != 4 || s.current != "claims-discovered" || s.resume != "" {
		t.Errorf("bounded: %+v, want bounded with no resume to hand out", s)
	}

	opts.Through = "nonsense"
	f.build(t, opts)
	if s := statusOf(t, f); s.state != stateBounded {
		t.Errorf("a refused launch moved the state: %+v", s)
	}

	if err := os.MkdirAll(f.state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := takeLock(f.state, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p, err := openProgress(f.state, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []event{{Event: eventRun, PID: os.Getpid(), NoInference: true}, {Event: eventStage, Stage: "claims-discovered", Total: 0}} {
		if err := p.emit(e); err != nil {
			t.Fatal(err)
		}
	}
	if s := statusOf(t, f); s.state != stateRunning || s.pid != os.Getpid() || s.resume != "" {
		t.Errorf("running: %+v", s)
	}
	held.Release()
	if s := statusOf(t, f); s.state != stateInterrupted || s.current != "claims-discovered" {
		t.Errorf("a holder gone with no end recorded: %+v", s)
	}
	if err := p.emit(event{Event: eventEnd, Outcome: result.Failed, Items: []result.Item{{Check: checkError, Detail: "a check failed"}}}); err != nil {
		t.Fatal(err)
	}
	p.close()
	_, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if items := details(fields, "recent-refusals"); !slices.Contains(items, "a check failed") {
		t.Errorf("recent refusals = %q", items)
	}

	opts.Through = ""
	if outcome, _ := f.build(t, opts); outcome != result.Done {
		t.Fatal(outcome)
	}
	if s := statusOf(t, f); s.state != stateFinished || s.recorded != len(Stages) || s.current != "" || s.resume != "" {
		t.Errorf("finished: %+v", s)
	}
}

// TestStatusReportsAKilledBuild: a build killed inside a stage leaves the
// state store's holder record naming a process that is gone and a progress
// record that never ended. status reports interrupted, naming the stage, its
// pid and the command that resumes it, and reports failed once an end event
// says how the run ended.
func TestStatusReportsAKilledBuild(t *testing.T) {
	const deadPID = 1 << 30 // larger than any pid a running process holds
	base := t.TempDir()
	gitEnv(t, base)
	repo := filepath.Join(base, "repo")
	makeRepo(t, repo)
	writeFile(t, filepath.Join(repo, kb.KBDir, kb.EntryPointFile), "# Knowledge Base\n")
	f := fixture{repo: repo, state: filepath.Join(base, "state")}
	if _, err := f.ledger(t).Record(Stages[0].ID, Stages[0].Display, ""); err != nil {
		t.Fatal(err)
	}
	resume := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", f.state}
	b, err := json.Marshal(holder{PID: deadPID, Started: "2026-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.state, lockFile), string(b))
	p, err := openProgress(f.state, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []event{
		{Event: eventRun, PID: deadPID, NoInference: true, Resume: resume},
		{Event: eventStage, Stage: stageDocumentGraph, Total: 2},
	} {
		if err := p.emit(e); err != nil {
			t.Fatal(err)
		}
	}
	s := statusOf(t, f)
	if s.state != stateInterrupted || s.current != stageDocumentGraph || s.done != 0 || s.total != 2 || s.pid != deadPID || s.resume != ShellJoin(resume) {
		t.Errorf("status of a build killed in %s = %+v, want interrupted there with pid %d and its resume command", stageDocumentGraph, s, deadPID)
	}
	if err := p.emit(event{Event: eventEnd, Outcome: result.Failed, Items: []result.Item{{Check: checkError, Detail: "a check failed"}}}); err != nil {
		t.Fatal(err)
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	if s := statusOf(t, f); s.state != stateFailed {
		t.Errorf("status after the run's failed end event = %+v, want failed", s)
	}
}

// TestReadStateNamesTheRun: ReadState is status's derivation without the
// commit trail — the state, and the pid and start stamp that name one run and
// no other — read from the store's holder lock and progress record alone.
func TestReadStateNamesTheRun(t *testing.T) {
	const deadPID = 1 << 30 // larger than any pid a running process holds
	state := t.TempDir()
	if got, err := ReadState(state); err != nil || got != (State{State: stateNone}) {
		t.Fatalf("a store never written: %+v, %v; want state none and no run", got, err)
	}
	held, err := takeLock(state, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p, err := openProgress(state, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.emit(event{Event: eventRun, PID: deadPID}); err != nil {
		t.Fatal(err)
	}
	started := func() string {
		events, err := readProgress(state)
		if err != nil || len(events) == 0 {
			t.Fatalf("the progress record: %v", err)
		}
		return events[0].Time
	}()
	if got, err := ReadState(state); err != nil || got != (State{State: stateRunning, PID: os.Getpid(), Started: started}) {
		t.Fatalf("a held store: %+v, %v; want running under the holder's pid %d from %s", got, err, os.Getpid(), started)
	}
	held.Release()
	if got, err := ReadState(state); err != nil || got != (State{State: stateInterrupted, PID: deadPID, Started: started}) {
		t.Fatalf("a holder gone with no end recorded: %+v, %v; want interrupted, the run's own pid %d from %s", got, err, deadPID, started)
	}
	if err := p.emit(event{Event: eventEnd, Outcome: result.Done}); err != nil {
		t.Fatal(err)
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadState(state); err != nil || got != (State{State: stateFinished, PID: deadPID, Started: started}) {
		t.Fatalf("a finished run: %+v, %v; want finished, the same run and start", got, err)
	}
	for state, want := range map[string]bool{stateNone: false, stateRunning: false, stateFinished: true,
		stateBounded: true, stateCancelled: true, stateFailed: true, stateInterrupted: true} {
		if got := Ended(state); got != want {
			t.Errorf("Ended(%s) = %v, want %v", state, got, want)
		}
	}
}

// TestWaitState: the wait ends the moment the store no longer stands at the
// run's progress — another run has superseded it, or its own run ended — and
// at once where the store never stood at that run at all.
func TestWaitState(t *testing.T) {
	const (
		runPID   = 1 << 30 // larger than any pid a running process holds
		otherPID = runPID + 1
	)
	// started is a store holding one run under pid, its lock held as a build
	// holds it while it progresses.
	started := func(t *testing.T, pid int) (string, *filelock.Lock, *progress) {
		t.Helper()
		dir := t.TempDir()
		held, err := takeLock(dir, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		p, err := openProgress(dir, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.emit(event{Event: eventRun, PID: pid}); err != nil {
			t.Fatal(err)
		}
		return dir, held, p
	}
	// end records the run's end and lets the lock go, as a build does on its
	// way out.
	end := func(t *testing.T, held *filelock.Lock, p *progress) {
		t.Helper()
		if err := p.emit(event{Event: eventEnd, Outcome: result.Done}); err != nil {
			t.Fatal(err)
		}
		held.Release()
		if err := p.close(); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("a run that already ended returns at once", func(t *testing.T) {
		dir, held, p := started(t, runPID)
		end(t, held, p)
		got, err := WaitState(dir, runPID, time.Hour)
		if err != nil || got.State != stateFinished || got.PID != runPID {
			t.Errorf("WaitState over an ended run = %+v, %v; want finished under pid %d", got, err, runPID)
		}
	})
	t.Run("a running run returns when it ends", func(t *testing.T) {
		// The holder's pid is the one the store reports while the lock is
		// held, so the run being waited for is this process's.
		pid := os.Getpid()
		dir, held, p := started(t, pid)
		done := make(chan error, 1)
		go func() {
			time.Sleep(20 * time.Millisecond)
			err := p.emit(event{Event: eventEnd, Outcome: result.Done})
			if err == nil {
				err = p.close()
			}
			held.Release()
			done <- err
		}()
		got, err := WaitState(dir, pid, 5*time.Millisecond)
		if err != nil || got.State != stateFinished || got.PID != pid {
			t.Errorf("WaitState over a running run = %+v, %v; want the finished run under pid %d", got, err, pid)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a superseded run returns", func(t *testing.T) {
		dir, held, p := started(t, runPID)
		got, err := WaitState(dir, otherPID, time.Hour)
		if err != nil || got.State != stateRunning || got.PID != os.Getpid() {
			t.Errorf("WaitState over a superseded run = %+v, %v; want running under the holder's pid %d", got, err, os.Getpid())
		}
		held.Release()
		if err := p.close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a pid the store does not hold returns with its standing run", func(t *testing.T) {
		dir, held, p := started(t, runPID)
		end(t, held, p)
		got, err := WaitState(dir, otherPID, time.Hour)
		if err != nil || got.State != stateFinished || got.PID != runPID {
			t.Errorf("WaitState for an unheld pid = %+v, %v; want the store's finished run under pid %d", got, err, runPID)
		}
	})
}

func TestProgressRecordIsJSONLines(t *testing.T) {
	f := newFixture(t)
	if outcome, _ := f.build(t, f.options()); outcome != result.Done {
		t.Fatal(outcome)
	}
	b, err := os.ReadFile(filepath.Join(f.state, progressFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	units := 0
	for _, l := range lines {
		var e event
		if err := json.Unmarshal(l, &e); err != nil {
			t.Fatalf("line %q: %v", l, err)
		}
		if e.Event == eventUnit {
			units++
		}
	}
	want := 0
	for _, r := range rows {
		if r.applies(true) {
			want++
		}
	}
	if units != want {
		t.Errorf("unit events = %d, want one per row run (%d)", units, want)
	}
}

func TestOverviewAsksOnceMoreAndAssembles(t *testing.T) {
	kbRoot := filepath.Join(t.TempDir(), "proj", kb.KBDir)
	writeFile(t, filepath.Join(kbRoot, kb.EntryPointFile), "# Knowledge Base\n\n- [Volume](vol/index.md)\n")
	writeFile(t, filepath.Join(kbRoot, "vol", "index.md"), "[↑ Knowledge Base](../entry-point.md)\n\n# Volume\n")
	for _, tc := range []struct {
		name    string
		replies []string
		ok      bool
	}{
		{"a clean passage", []string{"  A corpus about widgets.\n"}, true},
		{"a heading, then prose", []string{"# Overview\nA corpus.", "A corpus about widgets."}, true},
		{"empty, then prose", []string{"   ", "A corpus about widgets."}, true},
		{"a link twice", []string{"see [x](a.md)", "still [x](a.md)"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Remove(filepath.Join(kbRoot, kbdocs.OverviewFile))
			var asked [][]string
			w := &walk{ctx: context.Background(), kbRoot: kbRoot, repo: &ledger.Repo{Root: filepath.Dir(kbRoot)}, opts: Options{Logger: log.Discard()},
				passage: func(_ context.Context, excerpts string, rejected []string, attempt int) (string, error) {
					if !strings.HasPrefix(excerpts, "==> Knowledge Base <==") {
						t.Errorf("excerpts = %q", excerpts)
					}
					asked = append(asked, rejected)
					if attempt != len(asked) {
						t.Errorf("ask %d carried attempt %d", len(asked), attempt)
					}
					return tc.replies[len(asked)-1], nil
				},
			}
			// The digest predicate reads the claims stream; this fixture
			// carries none, and a one-volume KB is the shape it must give.
			writeFile(t, filepath.Join(kbRoot, kb.IndexDir, "claims.yaml"), "")
			_, err := overview(w)
			if (err == nil) != tc.ok {
				t.Fatalf("overview = %v, want ok %t", err, tc.ok)
			}
			if len(asked) != len(tc.replies) {
				t.Errorf("asked %d times, want %d", len(asked), len(tc.replies))
			}
			if len(asked) == 2 && strings.HasPrefix(tc.replies[0], "#") && !slices.Equal(asked[1], []string{"# Overview"}) {
				t.Errorf("the re-ask quoted %q, want the rejected line", asked[1])
			}
			readme, err := os.ReadFile(filepath.Join(kbRoot, kbdocs.OverviewFile))
			if tc.ok && (err != nil || !strings.Contains(string(readme), "# proj Knowledge Base\n\nA corpus about widgets.\n")) {
				t.Errorf("README.md = %q, %v", readme, err)
			}
			if !tc.ok && err == nil {
				t.Error("a refused passage was written")
			}
		})
	}
}

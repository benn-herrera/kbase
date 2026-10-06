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
		"claim discovery": "claims-discovered",
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
	if strings.Join(dropped, ",") != "discover.build,unmarked.build,ov.docs" {
		t.Errorf("rows dropped under --no-inference = %q, want the three that spend it", dropped)
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

func TestStateKeyResolvesTheUnbuiltTree(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	before, err := stateKey(filepath.Join(link, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(real, kb.KBDir), 0o755); err != nil {
		t.Fatal(err)
	}
	after, err := stateKey(filepath.Join(real, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	if before != after || len(before) != stateKeyHexDigits {
		t.Errorf("state keys %q (through a link, kb-root absent) and %q (resolved, present) must be one 16-digit key", before, after)
	}
}

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"kbase", "build", "/a b/p.tex", "--state-dir", "/s/it's"})
	if want := `kbase build '/a b/p.tex' --state-dir '/s/it'"'"'s'`; got != want {
		t.Errorf("shellJoin = %s, want %s", got, want)
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
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@invalid", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@invalid",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": filepath.Join(base, "gitconfig"), "XDG_STATE_HOME": filepath.Join(base, "xdg"),
	} {
		t.Setenv(k, v)
	}
	f := fixture{repo: filepath.Join(base, "repo"), state: filepath.Join(base, "state")}
	for _, d := range []string{".git/objects", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(f.repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(f.repo, ".git", "HEAD"), "ref: refs/heads/main\n")
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
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("")) {
		t.Errorf("trail = %q, want every stage in order", got)
	}
	trail, _ := f.ledger(t).Trail()
	bodies := map[string]string{}
	for _, e := range trail {
		bodies[e.Stage] = e.Body
	}
	if !strings.HasPrefix(bodies[stageStart], noCharter+"\n\n") {
		t.Errorf("start body = %q, want the stated absence of a charter", bodies[stageStart])
	}
	for _, s := range stageIDs("") {
		if in, ok := recordedInputs(bodies[s]); !ok || !slices.Equal(in.roots, []string{"paper.tex"}) || in.bibliographies != nil {
			t.Errorf("%s body = %q, want it to record the one volume root and no bibliography", s, bodies[s])
		}
	}
	for _, s := range []string{"claims-discovered", "references-found", "overview-drafted"} {
		if !strings.HasPrefix(bodies[s], "--no-inference: ") {
			t.Errorf("%s body = %q, want the dropped rows named", s, bodies[s])
		}
	}
	files := snapshot(t, f.repo)
	if want := "\nplanned: null\npairs: []\n"; !strings.HasSuffix(files[buildrecords.UnmarkedFile], want) {
		t.Errorf("%s = %q, want it as the declared pass wrote it, ending %q", buildrecords.UnmarkedFile, files[buildrecords.UnmarkedFile], want)
	}
	if files["kb-root/CLAUDE.md"] != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q, want exactly the redirect", files["kb-root/CLAUDE.md"])
	}
	if !strings.Contains(files["kb-root/AGENTS.md"], kbdocs.NoCharterPin) {
		t.Error("AGENTS.md does not carry the scope pin")
	}
	if _, ok := files["kb-root/README.md"]; ok {
		t.Error("a build spending no inference wrote the overview document, which kb_tools' does not")
	}
	if dirty, err := f.ledger(t).Dirty(); err != nil || len(dirty) != 0 {
		t.Errorf("owned paths after the build: dirty %q, %v", dirty, err)
	}
	sameModuloIDs(t, f.repo, reference(t))

	if outcome, fields := f.build(t, f.options()); outcome != result.Unchanged {
		t.Errorf("a re-run over a finished build = %s %v, want unchanged", outcome, fields)
	}
	opts := f.options()
	opts.VolumeRoots = append(opts.VolumeRoots, filepath.Join(f.repo, "more.tex"))
	writeFile(t, opts.VolumeRoots[1], paper)
	if outcome, fields := f.build(t, opts); outcome != result.Refused || details(fields, "refusals") == nil {
		t.Errorf("a re-run over a finished build with another volume root = %s %v, want refused", outcome, fields)
	}
}

func TestCharterBecomesTheScopePin(t *testing.T) {
	f := newFixture(t)
	charterPath := filepath.Join(t.TempDir(), "scope.md")
	writeFile(t, charterPath, "This KB distills one small paper about widgets.\n")
	opts := f.options()
	opts.Charter, opts.Through = charterPath, "phase-3a"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	trail, _ := f.ledger(t).Trail()
	if start := trail[len(trail)-1]; !strings.HasPrefix(start.Body, "charter: kb-build-charter.md\n\n") {
		t.Errorf("start body = %q", start.Body)
	}
	files := snapshot(t, f.repo)
	if files[charterFile] != "This KB distills one small paper about widgets.\n" {
		t.Errorf("%s = %q", charterFile, files[charterFile])
	}
	if !strings.Contains(files["kb-root/AGENTS.md"], "This KB distills one small paper about widgets.") {
		t.Error("AGENTS.md does not carry the charter as its scope pin")
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
	writeFile(t, entry, strings.Replace(string(b), `"`+kb.FormatVersion+`"`, `"2.0.0"`, 1))
	if _, err := f.ledger(t).Record("spine-seed", "claim-graph spine seeded", ""); err != nil {
		t.Fatal(err)
	}
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	if items, _ := field(fields, "refusals").([]result.Item); outcome != result.Refused || len(items) != 1 || items[0].Check != "kb-format" {
		t.Errorf("a resume over a KB stamped 2.0.0 = %s %v, want refused naming the format", outcome, fields)
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

	if state := statusOf(t, f); state.state != stateFailed || state.current != "claims-declared" {
		t.Errorf("status after the kill = %+v, want failed in claims-declared", state)
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
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("spine-seed")) {
		t.Errorf("trail after the cancel = %q, want every stage before the one in flight", got)
	}
	if held, err := filelock.Held(filepath.Join(f.state, lockFile)); err != nil || held {
		t.Errorf("the holder lock is still held after the cancel: %t, %v", held, err)
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
				return
			}
			items := details(fields, "refusals")
			if outcome != result.Refused || len(items) != len(tc.want) {
				t.Fatalf("build = %s %q, want one refusal per %q", outcome, items, tc.want)
			}
			for i, w := range tc.want {
				if !strings.Contains(items[i], w) {
					t.Errorf("refusal %q does not name %s", items[i], w)
				}
			}
			if got := f.trailStages(t); len(got) != 0 {
				t.Errorf("a launch refusal recorded %q", got)
			}
		})
	}
	f := newFixture(t)
	opts := f.options()
	opts.NoInference, opts.Through = false, "claims-discovered"
	opts.Provider = func() (Provider, error) {
		return Provider{}, config.LackError{Key: config.EnvAPIBaseURL, Path: config.ProvidersFileName, Detail: "KBASE_API_BASE_URL is unset"}
	}
	if _, fields := f.build(t, opts); func() bool {
		items, _ := field(fields, "refusals").([]result.Item)
		return len(items) != 1 || items[0].Key != config.EnvAPIBaseURL || items[0].Path != config.ProvidersFileName
	}() {
		t.Errorf("a provider lacking its base URL = %v, want the refusal keyed by the variable and the file", fields)
	}
	opts = f.options()
	opts.Through = "phase-5"
	if outcome, fields := f.build(t, opts); outcome != result.Refused || !strings.Contains(details(fields, "refusals")[0], "names no stage") {
		t.Errorf("--through phase-5 = %s %v", outcome, fields)
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
	for _, body := range []string{"", noCharter, "charter: kb-build-charter.md", "--no-inference: this build spent no model call"} {
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

// TestResumeCarriesEveryLaunchFlag: the resume command restates the charter,
// the bibliographies, --no-inference, the store and the configuration
// directory, and leaves the bound off.
func TestResumeCarriesEveryLaunchFlag(t *testing.T) {
	f := newFixture(t)
	charterPath := filepath.Join(t.TempDir(), "scope.md")
	writeFile(t, charterPath, "Widgets.\n")
	opts := f.options()
	opts.Charter, opts.Through, opts.ConfigDir = charterPath, "start", filepath.Join(f.repo, "config")
	opts.Bibliographies = []string{filepath.Join(f.repo, "refs.bib")}
	writeFile(t, opts.Bibliographies[0], "")
	outcome, fields := f.build(t, opts)
	if outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	resume, _ := field(fields, "resume").(string)
	for _, want := range []string{"--charter " + charterPath, "--bibliography " + opts.Bibliographies[0], "--no-inference", "--state-dir " + f.state,
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
	if s.state != stateBounded || s.recorded != 4 || s.current != "claims-discovered" || !strings.Contains(s.resume, "--no-inference") || strings.Contains(s.resume, "--through") {
		t.Errorf("bounded: %+v", s)
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
	if s := statusOf(t, f); s.state != stateFailed || s.current != "claims-discovered" {
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

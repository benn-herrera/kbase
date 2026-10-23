package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/build"
	"kbase/internal/kb"
	"kbase/internal/log/logtest"
	"kbase/internal/query"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// Two starts far enough apart that every case orders them the same way however
// the stamps are read back.
const (
	statusEarlier = "2026-10-01T10:00:00Z"
	statusLater   = "2026-10-02T10:00:00Z"
)

// statusStore is a worktree and the state store a status read resolves over it,
// the store empty. The kb-root/ is the directory alone: status reads no KB.
func statusStore(t *testing.T) (repo, state string) {
	t.Helper()
	repo, state = t.TempDir(), t.TempDir()
	gitRepo(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, kb.KBDir), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo, state
}

// statusProgress is one line of the store's progress record, as the fields a
// status read takes from it: when it happened, and what it was.
type statusProgress struct {
	Time    string `json:"time"`
	Event   string `json:"event"`
	PID     int    `json:"pid,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

// buildRan records one build invocation in the store's progress record, started
// at at and ended done.
func buildRan(t *testing.T, state, at string) {
	t.Helper()
	path := filepath.Join(state, "progress.jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, e := range []statusProgress{
		{Time: at, Event: "run", PID: 1},
		{Time: at, Event: "end", Outcome: toolresult.Done},
	} {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
}

// scoreLedger opens a run in the store at state, records its two items and the
// line the first landed, and returns the closed ledger — the run a waiter finds
// a result document for, or finds abandoned.
func scoreLedger(t *testing.T, state, kbRoot, run, started string) *score.Ledger {
	t.Helper()
	l, err := score.Create(state, score.RunHeader{Run: run, KBRoot: kbRoot, Pass: query.PassRigor, Started: started})
	if err != nil {
		t.Fatal(err)
	}
	items := []query.Item{
		{Node: &query.ScoringNode{Kind: kb.NodeKindClaim, ID: "clm-aaaaaa"}},
		{Node: &query.ScoringNode{Kind: kb.NodeKindClaim, ID: "clm-bbbbbb"}},
	}
	if err := l.WriteItems(items); err != nil {
		t.Fatal(err)
	}
	landed := score.Entry{Status: score.StatusDone, Value: "0.65", Faults: []score.Charge{}, Started: started, Ended: started}
	if err := l.Append(items[0], landed); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return l
}

// scoreRan records a score run in the store: its header started at, its two
// items, the line the first landed and — where outcome is not "" — the result
// document the verb writes beside them. A run left with no result document is
// one whose process ended before writing it.
func scoreRan(t *testing.T, state, kbRoot, run, started, outcome string) {
	t.Helper()
	l := scoreLedger(t, state, kbRoot, run, started)
	if outcome == "" {
		return
	}
	if err := score.WriteResultDocument(l.Dir(), outcome, nil); err != nil {
		t.Fatal(err)
	}
}

// scoreHeld is a run of the store at state whose process is pid: the record a
// waiter finds its run by.
func scoreHeld(t *testing.T, state, run string, pid int) *score.Ledger {
	t.Helper()
	l := scoreLedger(t, state, t.TempDir(), run, statusEarlier)
	if err := l.WritePID(pid); err != nil {
		t.Fatal(err)
	}
	return l
}

// TestWaitScoreReturnsTheRunsDocument: the run wrote its result document, so the
// waiter reads that document rather than inventing one, and exits on the outcome
// it names — cancelled here, so the code 4 is the document's own.
func TestWaitScoreReturnsTheRunsDocument(t *testing.T) {
	state, pid := t.TempDir(), os.Getpid()
	l := scoreHeld(t, state, "run-cancelled", pid)
	if err := score.WriteResultDocument(l.Dir(), toolresult.Cancelled, nil); err != nil {
		t.Fatal(err)
	}

	doc, code, err := waitScore(state, pid, time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(l.Dir(), score.ResultFile))
	if err != nil {
		t.Fatal(err)
	}
	if doc != string(want) {
		t.Errorf("wait reported\n%s\nwant the run's own document\n%s", doc, want)
	}
	if code != exitCodes[toolresult.Cancelled] {
		t.Errorf("exit %d, want the cancelled outcome's %d", code, exitCodes[toolresult.Cancelled])
	}
}

// holdScoreRun takes the store at state for the run named, its holder record
// naming this process, and returns the release. The returned function releases
// once however often it is called, so a test that must let a polling waiter
// unwind can release it itself and still register it for cleanup.
func holdScoreRun(t *testing.T, repo, state, run string) func() {
	t.Helper()
	release, err := build.Hold(filepath.Join(repo, kb.KBDir), state, build.KindScore, run, time.Now())
	if err != nil {
		t.Fatalf("hold the store for a score run: %v", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if err := release(); err != nil {
				t.Errorf("release the store: %v", err)
			}
		})
	}
}

// TestWaitScoreWaitsWhileTheRunHoldsTheStore: a holder record naming the run's
// own process is a run still in flight, so the waiter polls rather than
// reporting the fallback, and returns the document the run writes while it
// waits — a document that did not stand when the first pass read there.
func TestWaitScoreWaitsWhileTheRunHoldsTheStore(t *testing.T) {
	repo, state := statusStore(t)
	l := scoreHeld(t, state, "run-live", os.Getpid())
	t.Cleanup(holdScoreRun(t, repo, state, "run-live"))

	written := make(chan error, 1)
	go func() {
		time.Sleep(5 * time.Millisecond)
		written <- score.WriteResultDocument(l.Dir(), toolresult.Cancelled, nil)
	}()

	doc, code, err := waitScore(state, os.Getpid(), time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if err := <-written; err != nil {
		t.Fatalf("the run's document: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(l.Dir(), score.ResultFile))
	if err != nil {
		t.Fatalf("reading the document the run wrote: %v", err)
	}
	if doc != string(want) {
		t.Errorf("wait reported\n%s\nwant the document the run wrote\n%s", doc, want)
	}
	if code != exitCodes[toolresult.Cancelled] {
		t.Errorf("exit %d, want the cancelled outcome's %d", code, exitCodes[toolresult.Cancelled])
	}
}

// TestWaitScoreStopsBehindAnotherProcessHolder: a holder record naming a
// process other than the run's is a later run holding the store — a build
// started after this one ended — and is no reason to wait, so the waiter
// reports the run's end at once. A waiter that polled behind it would never
// return, so the watchdog turns that into a failure rather than a hung suite,
// releasing the store first so the polling goroutine unwinds.
func TestWaitScoreStopsBehindAnotherProcessHolder(t *testing.T) {
	repo, state := statusStore(t)
	release := holdScoreRun(t, repo, state, "run-live")
	t.Cleanup(release)
	mine := os.Getpid() + 1
	scoreHeld(t, state, "run-superseded", mine)

	type waited struct {
		doc  string
		code int
		err  error
	}
	got := make(chan waited, 1)
	go func() {
		doc, code, err := waitScore(state, mine, time.Millisecond)
		got <- waited{doc, code, err}
	}()

	var res waited
	select {
	case res = <-got:
	case <-time.After(2 * time.Second):
		release()
		t.Fatalf("waitScore is still polling behind a holder of another process")
	}
	if res.err != nil {
		t.Fatalf("wait: %v", res.err)
	}
	if res.code != exitCodes[toolresult.Failed] || res.code != 3 {
		t.Errorf("exit %d, want the run's end, which is failed's %d (3)", res.code, exitCodes[toolresult.Failed])
	}
	if d := parseResult(t, res.doc); d.Outcome != toolresult.Failed || d.Values["run"] != "run-superseded" {
		t.Errorf("document names outcome %v, run %v; want failed, run-superseded\n%s",
			d.Outcome, d.Values["run"], res.doc)
	}
}

// TestWaitScoreReportsARunItsProcessLeft: no document and no live holder is a
// run whose process ended without writing its result. The waiter reports it from
// the run's own ledger — the counts and the one item that landed — with the
// failure naming the end, and exits on failed, which is 3.
func TestWaitScoreReportsARunItsProcessLeft(t *testing.T) {
	state, pid := t.TempDir(), os.Getpid()
	scoreHeld(t, state, "run-abandoned", pid)

	doc, code, err := waitScore(state, pid, time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if code != exitCodes[toolresult.Failed] || code != 3 {
		t.Errorf("exit %d, want failed's %d (3)", code, exitCodes[toolresult.Failed])
	}
	d := parseResult(t, doc)
	if d.Outcome != toolresult.Failed || d.Values["run"] != "run-abandoned" ||
		d.Values["state-dir"] != state || d.Values["pass"] != query.PassRigor {
		t.Errorf("document names outcome %v, run %v, state-dir %v, pass %v; want failed, run-abandoned, %s, %s\n%s",
			d.Outcome, d.Values["run"], d.Values["state-dir"], d.Values["pass"], state, query.PassRigor, doc)
	}
	counts, _ := d.Values["counts"].(map[string]any)
	if counts["total"] != 2 || counts["done"] != 1 || counts["failed"] != 0 || counts["remaining"] != 1 {
		t.Errorf("counts = %v, want the two items with one landed\n%s", d.Values["counts"], doc)
	}
	if scored := anyStrings(d.Values["scored"]); !slices.Equal(scored, []string{"clm-aaaaaa"}) {
		t.Errorf("scored = %q, want the one item whose value landed", scored)
	}
	items := d.items(toolresult.FailuresKey)
	if len(items) != 1 {
		t.Fatalf("failures = %v, want the one item naming the process's end\n%s", items, doc)
	}
	detail, _ := items[0]["detail"].(string)
	if !strings.Contains(detail, "ended without writing its result") {
		t.Errorf("failure = %q, want it to name the process's end", detail)
	}
}

// statusOf is status's document for the store, as the verb reports it.
func statusOf(t *testing.T, repo, state string) resultDoc {
	t.Helper()
	out, code := kbase(t, repo, "status", "--state-dir", state)
	if code != 0 {
		t.Fatalf("status: exit %d\n%s", code, out)
	}
	return parseResult(t, out)
}

// checkScoreDocument fails unless d is the document of the run named run, read
// at the state store.
func checkScoreDocument(t *testing.T, d resultDoc, state, run, started, wantState string) {
	t.Helper()
	if d.Values["run"] != run || d.Values["state"] != wantState || d.Values["pass"] != query.PassRigor {
		t.Errorf("document names run %v, state %v, pass %v; want %s, %s, %s\n%v",
			d.Values["run"], d.Values["state"], d.Values["pass"], run, wantState, query.PassRigor, d.Values)
	}
	if d.Values["state-dir"] != state || d.Values["started"] != started {
		t.Errorf("document names state-dir %v, started %v; want %s, %s", d.Values["state-dir"], d.Values["started"], state, started)
	}
	counts, _ := d.Values["counts"].(map[string]any)
	if counts["total"] != 2 || counts["done"] != 1 || counts["failed"] != 0 || counts["remaining"] != 1 {
		t.Errorf("counts = %v, want the two items with one landed\n%v", d.Values["counts"], d.Values)
	}
	if scored := anyStrings(d.Values["scored"]); !slices.Equal(scored, []string{"clm-aaaaaa"}) {
		t.Errorf("scored = %q, want the one item whose value landed", scored)
	}
}

// checkBuildDocument fails unless d is the build's document, reporting state.
func checkBuildDocument(t *testing.T, d resultDoc, wantState string) {
	t.Helper()
	if d.Values["state"] != wantState || d.Values["stages"] == nil {
		t.Errorf("document reports state %v with stages %v; want the build's %s\n%v",
			d.Values["state"], d.Values["stages"], wantState, d.Values)
	}
	if run, ok := d.Values["run"]; ok {
		t.Errorf("the build's document names a run: %v", run)
	}
}

// TestPersistScoreResultLogsAWriteItCannotMake: the result document is written
// on the way out of a verb that cannot return an error there, so a run
// directory that refuses the write leaves the failure in the run's log, naming
// the ledger, and no document.
func TestPersistScoreResultLogsAWriteItCannotMake(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	capture := &logtest.Capture{}

	persistScoreResult(capture, []toolresult.Field{{Key: "ledger", Value: dir}}, toolresult.Done)

	if !capture.Has(t, "error", "ledger", dir) {
		t.Errorf("the failed write was not logged against the ledger: %+v", capture.Snapshot())
	}
	if _, err := os.Stat(filepath.Join(dir, score.ResultFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the run directory holds a result document: %v", err)
	}
}

// TestPersistScoreResultWritesNothingWithoutALedger: a run that named no
// ledger has nowhere to write its document, and no failure to report.
func TestPersistScoreResultWritesNothingWithoutALedger(t *testing.T) {
	capture := &logtest.Capture{}
	persistScoreResult(capture, []toolresult.Field{{Key: "kb-root", Value: t.TempDir()}}, toolresult.Done)
	if records := capture.Snapshot(); len(records) != 0 {
		t.Errorf("a run without a ledger logged %+v", records)
	}
}

// TestStatusReportsTheNewestScoreRun: with no run holding the store, status
// reports the store's newest run, and a score run that started after the last
// build invocation is that run — its own document, with the counts and state
// its records give.
func TestStatusReportsTheNewestScoreRun(t *testing.T) {
	repo, state := statusStore(t)
	buildRan(t, state, statusEarlier)
	scoreRan(t, state, filepath.Join(repo, kb.KBDir), "run-newer", statusLater, toolresult.Done)

	d := statusOf(t, repo, state)
	if d.Outcome != toolresult.Done {
		t.Fatalf("status: outcome %s\n%v", d.Outcome, d.Values)
	}
	checkScoreDocument(t, d, state, "run-newer", statusLater, toolresult.Done)
	if pid, ok := d.Values["pid"]; !ok || pid != nil {
		t.Errorf("the document names pid %v for a run no process holds", pid)
	}
}

// TestStatusAndWaitReportADryRun: a dry run's dry- id names a run directory as
// a real run's does, so wait finds it by its process and returns the document it
// wrote, and status reports it as the store's newest run.
func TestStatusAndWaitReportADryRun(t *testing.T) {
	repo, state := scoreRepo(t), t.TempDir()
	gitRepo(t, repo)
	buildRan(t, state, statusEarlier)
	got := scoreIn(t, repo, state, scoreOptions{Pass: query.PassRigor, DryRun: true}, nil)
	run, _ := got.doc.Values["run"].(string)
	if got.doc.Outcome != toolresult.Done || !strings.HasPrefix(run, "dry-") {
		t.Fatalf("the dry run: outcome %s, run %q\n%s", got.doc.Outcome, run, got.stdout)
	}

	doc, code, err := waitScore(state, os.Getpid(), time.Millisecond)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if d := parseResult(t, doc); d.Outcome != toolresult.Done || d.Values["run"] != run || code != exitCodes[toolresult.Done] {
		t.Errorf("wait reported outcome %s, run %v, exit %d; want done, %s, %d\n%s",
			d.Outcome, d.Values["run"], code, run, exitCodes[toolresult.Done], doc)
	}

	d := statusOf(t, repo, state)
	if d.Outcome != toolresult.Done || d.Values["run"] != run || d.Values["state"] != toolresult.Done {
		t.Errorf("status reported outcome %s, run %v, state %v; want done, %s, done\n%v",
			d.Outcome, d.Values["run"], d.Values["state"], run, d.Values)
	}
}

// TestStatusReportsTheBuildNewerThanTheNewestScoreRun: a build invocation that
// started after the store's newest score run is the newer run, so status
// reports the build as it did before any score run stood there.
func TestStatusReportsTheBuildNewerThanTheNewestScoreRun(t *testing.T) {
	repo, state := statusStore(t)
	scoreRan(t, state, filepath.Join(repo, kb.KBDir), "run-earlier", statusEarlier, toolresult.Done)
	buildRan(t, state, statusLater)

	d := statusOf(t, repo, state)
	if d.Outcome != toolresult.Done {
		t.Fatalf("status: outcome %s\n%v", d.Outcome, d.Values)
	}
	checkBuildDocument(t, d, "finished")
}

// TestStatusReportsTheHolderWhicheverRunIsNewer: a live holder names the run
// status reports, whether it is the newer run or not.
func TestStatusReportsTheHolderWhicheverRunIsNewer(t *testing.T) {
	t.Run("a live build over a newer score run", func(t *testing.T) {
		repo, state := statusStore(t)
		buildRan(t, state, statusEarlier)
		scoreRan(t, state, filepath.Join(repo, kb.KBDir), "run-newer", statusLater, toolresult.Done)
		release, err := build.Hold(filepath.Join(repo, kb.KBDir), state, build.KindBuild, "", time.Now())
		if err != nil {
			t.Fatalf("hold the store for a build: %v", err)
		}
		t.Cleanup(func() {
			if err := release(); err != nil {
				t.Errorf("release the store: %v", err)
			}
		})

		checkBuildDocument(t, statusOf(t, repo, state), "running")
	})

	t.Run("a live score run over a newer build invocation", func(t *testing.T) {
		repo, state := statusStore(t)
		scoreRan(t, state, filepath.Join(repo, kb.KBDir), "run-live", statusEarlier, "")
		buildRan(t, state, statusLater)
		t.Cleanup(holdScoreRun(t, repo, state, "run-live"))

		d := statusOf(t, repo, state)
		// The run holds the store and has written no result document, so it is
		// running under the holder's process.
		if d.Values["run"] != "run-live" || d.Values["state"] != "running" || d.Values["pid"] != os.Getpid() {
			t.Errorf("document names run %v, state %v, pid %v; want run-live, running, %d\n%v",
				d.Values["run"], d.Values["state"], d.Values["pid"], os.Getpid(), d.Values)
		}
	})
}

// TestStatusReportsTheBuildWhereTheStoreRecordsNoRun: a store no run wrote
// reports the build's own document, its state none.
func TestStatusReportsTheBuildWhereTheStoreRecordsNoRun(t *testing.T) {
	repo, state := statusStore(t)
	checkBuildDocument(t, statusOf(t, repo, state), "none")
}

// TestStatusSkipsScoreRunsWithoutAHeader: a run directory whose run.yaml is
// absent, or does not read as one, left no record of when it started. It is
// skipped and no error: the store's other run is what status reports.
func TestStatusSkipsScoreRunsWithoutAHeader(t *testing.T) {
	repo, state := statusStore(t)
	buildRan(t, state, statusEarlier)
	for run, header := range map[string]string{
		"run-absent":  "",
		"run-garbage": "started: [not a stamp\n",
	} {
		dir := filepath.Join(score.RunsDir(state), run)
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
		if header != "" {
			if err := os.WriteFile(filepath.Join(dir, "run.yaml"), []byte(header), 0o666); err != nil {
				t.Fatal(err)
			}
		}
		// Each skipped run claims an outcome, so a status that reported it
		// would say done instead of the build's finished.
		if err := score.WriteResultDocument(dir, toolresult.Done, nil); err != nil {
			t.Fatal(err)
		}
	}

	d := statusOf(t, repo, state)
	if d.Outcome != toolresult.Done {
		t.Fatalf("status: outcome %s\n%v", d.Outcome, d.Values)
	}
	checkBuildDocument(t, d, "finished")
}

// TestStatusReportsTheBuildWhereAStartDoesNotParse: two starts that do not
// order leave the build the run reported, so an unparsable stamp does not turn
// a score run into the newer one.
func TestStatusReportsTheBuildWhereAStartDoesNotParse(t *testing.T) {
	const unreadable = "yesterday"

	t.Run("the score run's start", func(t *testing.T) {
		repo, state := statusStore(t)
		buildRan(t, state, statusEarlier)
		scoreRan(t, state, filepath.Join(repo, kb.KBDir), "run-unstamped", unreadable, toolresult.Done)

		checkBuildDocument(t, statusOf(t, repo, state), "finished")
	})

	t.Run("the build's start", func(t *testing.T) {
		repo, state := statusStore(t)
		buildRan(t, state, unreadable)
		scoreRan(t, state, filepath.Join(repo, kb.KBDir), "run-stamped", statusLater, toolresult.Done)

		checkBuildDocument(t, statusOf(t, repo, state), "finished")
	})
}

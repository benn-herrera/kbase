package buildstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kbase/internal/buildstore/storetest"
	"kbase/internal/filelock"
	"kbase/internal/kb"
	"kbase/internal/ledger"
	"kbase/internal/log"
	"kbase/internal/result"
)

// fixture is a repository and a state store beside it.
type fixture struct {
	repo, state string
}

// newTrailFixture is a fresh repository whose boundaries the test writes by
// hand through the ledger, and a state store beside it: nothing here runs a
// build, so no test needs pandoc.
func newTrailFixture(t *testing.T) fixture {
	t.Helper()
	base := t.TempDir()
	storetest.GitEnv(t, base)
	f := fixture{repo: filepath.Join(base, "repo"), state: filepath.Join(base, "state")}
	storetest.MakeRepo(t, f.repo)
	return f
}

// ledger opens the fixture's repository over the build's owned paths. build's
// tests keep a copy of their own: it names buildstore.OwnedPaths, which a
// package these in-package tests import cannot reach.
func (f fixture) ledger(t *testing.T) *ledger.Repo {
	t.Helper()
	r, err := ledger.Open(f.repo, OwnedPaths, log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// appendProgress appends events to the progress record in dir as a build's
// progress writer lands them, one stamped JSON line apiece; that writer is the
// runner's, which this package does not import.
func appendProgress(t *testing.T, dir string, events ...Event) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	p, err := os.OpenFile(filepath.Join(dir, ProgressFile), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, e := range events {
		e.Time = time.Now().UTC().Format(time.RFC3339)
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
	}
}

// holdStore takes the state store's holder lock at dir and writes h as its
// record, as a run holds the store while it progresses.
func holdStore(t *testing.T, dir string, h Holder) *filelock.Lock {
	t.Helper()
	l, err := filelock.Acquire(HolderPath(dir), 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Write(b); err != nil {
		t.Fatal(err)
	}
	return l
}

// statusSnapshot and statusOf read Status; build's tests keep a copy of their
// own, which a package these in-package tests import cannot hold.
type statusSnapshot struct {
	state, current, resume string
	done, total            int
	pid                    any
}

func statusOf(t *testing.T, f fixture) statusSnapshot {
	t.Helper()
	outcome, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
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
	return s
}

func TestShellJoin(t *testing.T) {
	got := ShellJoin([]string{"kbase", "build", "/a b/p.tex", "--state-dir", "/s/it's"})
	if want := `kbase build '/a b/p.tex' --state-dir '/s/it'"'"'s'`; got != want {
		t.Errorf("ShellJoin = %s, want %s", got, want)
	}
}

// TestHolderRecordWithoutAKindIsRefused: the kind is what says which verb owns
// the run, so a record naming none is a store another version left — refused,
// never read as a build's.
func TestHolderRecordWithoutAKindIsRefused(t *testing.T) {
	f := newTrailFixture(t)
	record := `{"pid": 4000, "started": "2026-01-01T00:00:00Z"}`
	storetest.WriteFile(t, HolderPath(f.state), record)
	outcome, fields := Status(MonitorOptions{StateDir: f.state, WorkDir: f.repo})
	items, _ := storetest.Field(fields, "refusals").([]result.Item)
	if outcome != result.Refused || len(items) != 1 || items[0].Check != CheckStateDir ||
		items[0].Path != HolderPath(f.state) || !strings.Contains(items[0].Detail, `names kind ""`) {
		t.Errorf("status over a holder record naming no verb = %s %v, want refused naming the record", outcome, fields)
	}
	if _, err := ReadState(f.state); err == nil {
		t.Error("ReadState read a holder record naming no verb")
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
	storetest.GitEnv(t, base)
	repo := filepath.Join(base, "repo")
	storetest.MakeRepo(t, repo)
	storetest.WriteFile(t, filepath.Join(repo, kb.KBDir, kb.EntryPointFile), "# Knowledge Base\n")
	f := fixture{repo: repo, state: filepath.Join(base, "state")}
	if _, err := f.ledger(t).Record(Stages[0].ID, Stages[0].Display, ""); err != nil {
		t.Fatal(err)
	}
	resume := []string{"kbase", "build", "/papers/paper.tex", "--state-dir", f.state}
	b, err := json.Marshal(Holder{PID: deadPID, Started: "2026-01-01T00:00:00Z", Kind: KindBuild})
	if err != nil {
		t.Fatal(err)
	}
	storetest.WriteFile(t, HolderPath(f.state), string(b))
	appendProgress(t, f.state,
		Event{Event: EventRun, PID: deadPID, NoInference: true, Resume: resume},
		Event{Event: EventStage, Stage: StageDocumentGraph, Total: 2},
	)
	s := statusOf(t, f)
	if s.state != StateInterrupted || s.current != StageDocumentGraph || s.done != 0 || s.total != 2 || s.pid != deadPID || s.resume != ShellJoin(resume) {
		t.Errorf("status of a build killed in %s = %+v, want interrupted there with pid %d and its resume command", StageDocumentGraph, s, deadPID)
	}
	appendProgress(t, f.state, Event{Event: EventEnd, Outcome: result.Failed, Items: []result.Item{{Check: CheckError, Detail: "a check failed"}}})
	if s := statusOf(t, f); s.state != StateFailed {
		t.Errorf("status after the run's failed end event = %+v, want failed", s)
	}
}

// TestReadStateNamesTheRun: ReadState is status's derivation without the
// commit trail — the state, and the pid and start stamp that name one run and
// no other — read from the store's holder lock and progress record alone.
func TestReadStateNamesTheRun(t *testing.T) {
	const deadPID = 1 << 30 // larger than any pid a running process holds
	state := t.TempDir()
	if got, err := ReadState(state); err != nil || got != (State{State: StateNone}) {
		t.Fatalf("a store never written: %+v, %v; want state none and no run", got, err)
	}
	held := holdStore(t, state, Holder{PID: os.Getpid(), Started: time.Now().UTC().Format(time.RFC3339), Kind: KindBuild})
	appendProgress(t, state, Event{Event: EventRun, PID: deadPID})
	started := func() string {
		events, err := ReadProgress(state)
		if err != nil || len(events) == 0 {
			t.Fatalf("the progress record: %v", err)
		}
		return events[0].Time
	}()
	if got, err := ReadState(state); err != nil || got != (State{State: StateRunning, PID: os.Getpid(), Started: started}) {
		t.Fatalf("a held store: %+v, %v; want running under the holder's pid %d from %s", got, err, os.Getpid(), started)
	}
	held.Release()
	if got, err := ReadState(state); err != nil || got != (State{State: StateInterrupted, PID: deadPID, Started: started}) {
		t.Fatalf("a holder gone with no end recorded: %+v, %v; want interrupted, the run's own pid %d from %s", got, err, deadPID, started)
	}
	appendProgress(t, state, Event{Event: EventEnd, Outcome: result.Done})
	if got, err := ReadState(state); err != nil || got != (State{State: StateFinished, PID: deadPID, Started: started}) {
		t.Fatalf("a finished run: %+v, %v; want finished, the same run and start", got, err)
	}
	for state, want := range map[string]bool{StateNone: false, StateRunning: false, StateFinished: true,
		StateBounded: true, StateCancelled: true, StateFailed: true, StateInterrupted: true} {
		if got := Ended(state); got != want {
			t.Errorf("Ended(%s) = %v, want %v", state, got, want)
		}
	}
}

// TestAScoreRunLeavesTheBuildWhereItStands: a score run holds the store's
// holder lock and writes no progress event, so it is no build run. While it
// holds the store and after it lets go, the reading and status name the build
// the store stands at, with its own state, pid and start — never the score
// run's pid, which would make the build a new run once the score run ended and
// have the watcher announce it again.
func TestAScoreRunLeavesTheBuildWhereItStands(t *testing.T) {
	const buildPID = 1 << 30 // larger than any pid a running process holds
	f := newTrailFixture(t)
	appendProgress(t, f.state, Event{Event: EventRun, PID: buildPID}, Event{Event: EventEnd, Outcome: result.Done})
	built, err := ReadState(f.state)
	if err != nil || built.State != StateFinished || built.PID != buildPID {
		t.Fatalf("the finished build: %+v, %v", built, err)
	}
	check := func(when string) {
		t.Helper()
		if got, err := ReadState(f.state); err != nil || got != built {
			t.Errorf("%s: ReadState = %+v, %v; want the finished build %+v", when, got, err, built)
		}
		if s := statusOf(t, f); s.state != StateFinished || s.pid != buildPID {
			t.Errorf("%s: status = %+v; want the build finished under pid %d", when, s, buildPID)
		}
	}
	held := holdStore(t, f.state, Holder{PID: os.Getpid(), Started: time.Now().UTC().Format(time.RFC3339), Kind: KindScore, Run: "run-1"})
	check("while a score run holds the store")
	held.Release()
	check("once the score run has let go")
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
	started := func(t *testing.T, pid int) (string, *filelock.Lock) {
		t.Helper()
		dir := t.TempDir()
		appendProgress(t, dir, Event{Event: EventRun, PID: pid})
		return dir, holdStore(t, dir, Holder{PID: os.Getpid(), Kind: KindBuild})
	}
	// end records the run's end and lets the lock go, as a build does on its
	// way out.
	end := func(t *testing.T, dir string, held *filelock.Lock) {
		t.Helper()
		appendProgress(t, dir, Event{Event: EventEnd, Outcome: result.Done})
		held.Release()
	}

	t.Run("a run that already ended returns at once", func(t *testing.T) {
		dir, held := started(t, runPID)
		end(t, dir, held)
		got, err := WaitState(dir, runPID, time.Hour)
		if err != nil || got.State != StateFinished || got.PID != runPID {
			t.Errorf("WaitState over an ended run = %+v, %v; want finished under pid %d", got, err, runPID)
		}
	})
	t.Run("a running run returns when it ends", func(t *testing.T) {
		// The holder's pid is the one the store reports while the lock is
		// held, so the run being waited for is this process's. Its end is
		// recorded while it still holds the store, and the run is over once it
		// lets go.
		pid := os.Getpid()
		dir, held := started(t, pid)
		appendProgress(t, dir, Event{Event: EventEnd, Outcome: result.Done})
		released := make(chan struct{})
		go func() {
			defer close(released)
			time.Sleep(20 * time.Millisecond)
			held.Release()
		}()
		got, err := WaitState(dir, pid, 5*time.Millisecond)
		<-released
		if err != nil || got.State != StateFinished || got.PID != pid {
			t.Errorf("WaitState over a running run = %+v, %v; want the finished run under pid %d", got, err, pid)
		}
	})
	t.Run("a superseded run returns", func(t *testing.T) {
		dir, held := started(t, runPID)
		defer held.Release()
		got, err := WaitState(dir, otherPID, time.Hour)
		if err != nil || got.State != StateRunning || got.PID != os.Getpid() {
			t.Errorf("WaitState over a superseded run = %+v, %v; want running under the holder's pid %d", got, err, os.Getpid())
		}
	})
	t.Run("a pid the store does not hold returns with its standing run", func(t *testing.T) {
		dir, held := started(t, runPID)
		end(t, dir, held)
		got, err := WaitState(dir, otherPID, time.Hour)
		if err != nil || got.State != StateFinished || got.PID != runPID {
			t.Errorf("WaitState for an unheld pid = %+v, %v; want the store's finished run under pid %d", got, err, runPID)
		}
	})
}

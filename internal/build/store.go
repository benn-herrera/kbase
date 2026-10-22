package build

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kbase/internal/asks/call"
	"kbase/internal/atomicfile"
	"kbase/internal/filelock"
	"kbase/internal/result"
	"kbase/internal/write"
)

// The state store holds what a build keeps outside the repository: the
// holder's lock and record, the progress record, the document graph's records, and
// scratch — per-call evidence, the answer cache, and the tree a resume
// regenerates missing records from. Nothing in it is authoritative for
// position; the commit trail is.
const (
	lockFile     = "lock"
	progressFile = "progress.jsonl"
	// ReportsDir holds each stage's report, and the output a build started
	// in its own process leaves.
	ReportsDir = "reports"
	// CapturePrefix begins the name of each file under ReportsDir that
	// captures the output of a build started in its own process.
	CapturePrefix = "build-"
	scratchDir    = "scratch"
	// regenerateDir, under scratch, is where a resume rebuilds the document
	// graph to regenerate missing records; it is removed once read.
	regenerateDir = "regenerate"
	// lockWait bounds the wait for the operation lock and the holder lock, so
	// a monitor probing one does not turn a starting run away.
	lockWait = 2 * time.Second
)

// reportPath is where a stage's report lands in the store at dir.
func reportPath(dir, stage string) string { return filepath.Join(dir, ReportsDir, stage+".yaml") }

// writeReport writes a stage's report: each row's record and what was dropped.
func writeReport(path string, report result.Record) error {
	var b bytes.Buffer
	if err := result.Write(&b, report...); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return atomicfile.Write(path, b.Bytes(), nil)
}

// Kind is the verb that owns a run: what the holder record names, and what
// both runs' refusals call their holder.
type Kind string

const (
	KindBuild Kind = "build"
	KindScore Kind = "score"
)

// noun is the run as a refusal and a document name it: "a build", "a score
// run".
func (k Kind) noun() string {
	if k == KindScore {
		return "score run"
	}
	return "build"
}

// Holder is what the store's holder lock says about the run holding it: the
// process, its start, the verb that owns it, and the id of the score run where
// one holds it.
type Holder struct {
	PID     int    `json:"pid"`
	Started string `json:"started"`
	Kind    Kind   `json:"kind"`
	Run     string `json:"run,omitempty"`
}

// HolderPath is the state store's holder lock at dir.
func HolderPath(dir string) string { return filepath.Join(dir, lockFile) }

// readHolder is the holder record in the store at dir; the zero record where
// none stands. The kind is required: it says which verb owns the run, and a
// record without one this version writes is a store another version left,
// refused rather than read as a build.
func readHolder(dir string) (Holder, error) {
	var h Holder
	path := HolderPath(dir)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err == nil && len(b) == 0 {
		return h, nil
	}
	if err != nil {
		return Holder{}, err
	}
	if err := json.Unmarshal(b, &h); err != nil {
		return Holder{}, fmt.Errorf("%s: %w", path, err)
	}
	if h.Kind != KindBuild && h.Kind != KindScore {
		return Holder{}, result.Refusal{{Check: checkStateDir, Path: path, Key: "--state-dir",
			Remedy: "update kbase, or use another --state-dir",
			Detail: fmt.Sprintf("the holder record names kind %q; this kbase writes %q or %q", h.Kind, KindBuild, KindScore)}}
	}
	return h, nil
}

// HolderOf is the state store's holder lock at dir read: whether a run holds
// it, and the record that run wrote — the zero record where no run is
// recorded, and a record this version did not write refused.
func HolderOf(dir string) (live bool, h Holder, err error) {
	if h, err = readHolder(dir); err != nil {
		return false, Holder{}, err
	}
	if live, err = filelock.Held(HolderPath(dir)); err != nil {
		return false, Holder{}, err
	}
	return live, h, nil
}

// operationLockFile is the operation lock in the repository's git directory,
// so a build and a scoring run over one KB exclude each other whatever their
// state directories and the working tree gains no file. It records the
// holder's state directory; the store's holder record carries the pid, the
// start and the verb status and cancel read.
const operationLockFile = "kbase-operation.lock"

// operationLockPath is the operation lock of the KB at kbRoot. A .git file is
// followed to the worktree's own git directory, read rather than asked of git,
// since a maintenance verb that checks the lock never requires git.
func operationLockPath(kbRoot string) string {
	repo := filepath.Dir(kbRoot)
	dir := filepath.Join(repo, ".git")
	if text, err := os.ReadFile(dir); err == nil {
		if target, ok := strings.CutPrefix(strings.TrimSpace(string(text)), "gitdir:"); ok {
			dir = strings.TrimSpace(target)
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(repo, dir)
			}
		}
	}
	return filepath.Join(dir, operationLockFile)
}

// takeOperationLock is the repository's operation lock, recording stateDir; a
// live holder is refused, naming the run and its state directory.
func takeOperationLock(kbRoot, stateDir string) (*filelock.Lock, error) {
	path := operationLockPath(kbRoot)
	l, err := filelock.Acquire(path, lockWait)
	if errors.Is(err, filelock.ErrHeld) {
		items, err := runningRefusal(path)
		if err != nil {
			return nil, err
		}
		if items == nil {
			return nil, Retry{{Check: checkLock, Path: path, Detail: "another run held the operation lock through the wait and is starting or has just released it"}}
		}
		return nil, result.Refusal(items)
	}
	if err != nil {
		return nil, err
	}
	if err := l.Write([]byte(stateDir)); err != nil {
		return nil, errors.Join(err, l.ReleaseRemoving())
	}
	return l, nil
}

// Hold takes the locks one run holds for its length: the KB's operation lock
// at kbRoot, recording stateDir, then the state store's holder lock at
// stateDir, recording the run — the score run named run, the build naming
// none. A live holder of either is the run's refusal, in that order, as a
// build's. What Hold returns releases both, the store's holder lock first.
func Hold(kbRoot, stateDir string, kind Kind, run string, now time.Time) (func() error, error) {
	op, err := takeOperationLock(kbRoot, stateDir)
	if err != nil {
		return nil, err
	}
	l, err := takeLock(stateDir, Holder{PID: os.Getpid(), Started: stamp(now), Kind: kind, Run: run})
	if err != nil {
		return nil, errors.Join(err, op.ReleaseRemoving())
	}
	return func() error { return errors.Join(l.ReleaseRemoving(), op.ReleaseRemoving()) }, nil
}

// RunningOperation is the refusal a verb that writes the KB at kbRoot meets
// while a run — a build or a score run — holds the operation lock: the run
// writes kb-root/ without the write lock; nil where none does.
func RunningOperation(kbRoot string) ([]result.Item, error) {
	path := operationLockPath(kbRoot)
	held, err := filelock.Held(path)
	if err != nil || !held {
		return nil, err
	}
	return runningRefusal(path)
}

// runningRefusal is the refusal naming the operation lock's holder; nil where
// the holder has removed the lock on its way out, or has taken it and not yet
// written its state directory. A holder that has not written it has not yet
// waited out the KB's writers, so a writer may go ahead. The holder's record
// in that store says which verb holds the lock and which process; one taken
// and not yet recorded is a run starting.
func runningRefusal(path string) ([]result.Item, error) {
	dir, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err == nil && len(dir) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	h, err := readHolder(string(dir))
	if err != nil {
		return nil, err
	}
	if h.PID == 0 {
		return []result.Item{{Check: checkLock, Path: string(dir), Detail: "a run is starting; its state-dir is " + string(dir)}}, nil
	}
	return []result.Item{{Check: checkLock, Path: string(dir),
		Detail: fmt.Sprintf("a %s (pid %d) is running; its state-dir is %s", h.Kind.noun(), h.PID, dir)}}, nil
}

// awaitWriters waits out a write op or refresh already holding the KB's
// write lock; one that takes it later finds the operation lock and refuses.
func awaitWriters(kbRoot string) error {
	release, err := write.LockKB(kbRoot)
	if errors.Is(err, filelock.ErrHeld) {
		return Retry{{Check: checkLock, Path: filepath.Dir(kbRoot), Detail: "a write op or refresh holds the KB's write lock"}}
	}
	if err != nil {
		return err
	}
	release()
	return nil
}

// takeLock is the state store's holder lock, its holder record written; a
// live holder is a retry naming it.
func takeLock(dir string, h Holder) (*filelock.Lock, error) {
	l, err := filelock.Acquire(HolderPath(dir), lockWait)
	if errors.Is(err, filelock.ErrHeld) {
		who := "another run"
		if held, err := readHolder(dir); err == nil && held.PID != 0 {
			who = fmt.Sprintf("another %s (pid %d, started %s)", held.Kind.noun(), held.PID, held.Started)
		}
		return nil, Retry{{Check: checkLock, Path: HolderPath(dir), Detail: who + " holds the state store"}}
	}
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(h)
	if err == nil {
		err = l.Write(b)
	}
	if err != nil {
		return nil, errors.Join(err, l.ReleaseRemoving())
	}
	return l, nil
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// The progress record's events.
const (
	eventRun      = "run"
	eventStage    = "stage"
	eventUnit     = "unit"
	eventUnits    = "units"
	eventRecorded = "recorded"
	eventFallback = "fallback"
	eventEnd      = "end"
)

// event is one line of the progress record. A run event opens each
// invocation that took the lock; a stage event enters a stage with its unit
// count, and a units event recounts them where a row reports finer units of
// its own; a unit event closes one unit, a fallback event names an item
// defaulted to its mechanical draft, a recorded event names the stage's
// boundary commit, and an end event closes the invocation with its outcome.
type event struct {
	Time        string        `json:"time"`
	Event       string        `json:"event"`
	PID         int           `json:"pid,omitempty"`
	NoInference bool          `json:"no-inference,omitempty"`
	Resume      []string      `json:"resume,omitempty"`
	Stage       string        `json:"stage,omitempty"`
	Unit        string        `json:"unit,omitempty"`
	Done        int           `json:"done,omitempty"`
	Total       int           `json:"total,omitempty"`
	Commit      string        `json:"commit,omitempty"`
	Outcome     string        `json:"outcome,omitempty"`
	Items       []result.Item `json:"items,omitempty"`
}

// progress appends events to the record, each synced as it lands so a monitor
// and a resume after a kill read every event that happened.
type progress struct {
	f   *os.File
	now func() time.Time
}

func openProgress(dir string, now func() time.Time) (*progress, error) {
	f, err := os.OpenFile(filepath.Join(dir, progressFile), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		return nil, err
	}
	return &progress{f, now}, nil
}

func (p *progress) emit(e event) error {
	e.Time = stamp(p.now())
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := p.f.Write(append(b, '\n')); err != nil {
		return err
	}
	return p.f.Sync()
}

func (p *progress) close() error { return p.f.Close() }

// readProgress is the record's events in order; a line a killed process left
// half-written is not an event.
func readProgress(dir string) ([]event, error) {
	f, err := os.Open(filepath.Join(dir, progressFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			events = append(events, e)
		}
	}
	return events, sc.Err()
}

// invocation is one run's events, its run event first.
type invocation []event

func invocations(events []event) []invocation {
	var out []invocation
	for _, e := range events {
		if e.Event == eventRun {
			out = append(out, invocation{e})
		} else if len(out) > 0 {
			out[len(out)-1] = append(out[len(out)-1], e)
		}
	}
	return out
}

// end is the invocation's end event, or the zero event where it has none —
// a process that was killed.
func (inv invocation) end() event {
	if last := inv[len(inv)-1]; last.Event == eventEnd {
		return last
	}
	return event{}
}

// recorded reports whether the invocation recorded a stage boundary: the fact
// that decides whether an identical re-issue continues it, since a run that
// recorded none wrote no boundary and leaves the build where the run before it
// left it.
func (inv invocation) recorded() bool {
	for _, e := range inv {
		if e.Event == eventRecorded {
			return true
		}
	}
	return false
}

// stands reports whether the invocation is the run status and rewind-build read
// the build's position from: every run but one that refused or asked to be
// retried without recording a boundary — that one wrote nothing, so the run
// before it is the one the build stands at.
func (inv invocation) stands() bool {
	switch inv.end().Outcome {
	case result.Refused, result.Retry:
		return inv.recorded()
	}
	return true
}

// entered is the last stage the invocation entered and whether it recorded
// it.
func (inv invocation) entered() (stage string, recorded bool) {
	for _, e := range inv {
		switch e.Event {
		case eventStage:
			stage, recorded = e.Stage, false
		case eventRecorded:
			recorded = recorded || e.Stage == stage
		}
	}
	return stage, recorded
}

// uncommittedIn is whether the progress record accounts for uncommitted work
// in stage: the latest invocation that entered a stage and was not refused —
// a refused invocation writes nothing — entered stage last and never
// recorded it. A cancelled invocation counts: its stage's work stands
// uncommitted too, whatever word the state is reported under.
func uncommittedIn(events []event, stage string) bool {
	all := invocations(events)
	for i := len(all) - 1; i >= 0; i-- {
		inv := all[i]
		entered, recorded := inv.entered()
		if entered == "" || inv.end().Outcome == result.Refused {
			continue
		}
		return entered == stage && !recorded
	}
	return false
}

// keptBuilds is how many of the latest builds that ended having walked keep
// their captures in the store.
const keptBuilds = 5

// pruneCaptures removes, from the store at dir, every capture — a started
// build's output under ReportsDir and a call's under scratch — last written
// before the start of the keptBuilds-th latest build that ended neither
// refused nor told to retry. A build killed before its end is not counted.
// The answer cache and the stage reports are kept.
func pruneCaptures(dir string) error {
	events, err := readProgress(dir)
	if err != nil {
		return err
	}
	var starts []string
	for _, inv := range invocations(events) {
		if o := inv.end().Outcome; o != "" && o != result.Refused && o != result.Retry {
			starts = append(starts, inv[0].Time)
		}
	}
	if len(starts) < keptBuilds {
		return nil
	}
	cutoff, err := time.Parse(time.RFC3339, starts[len(starts)-keptBuilds])
	if err != nil {
		return err
	}
	for _, d := range []struct{ dir, prefix string }{
		{filepath.Join(dir, ReportsDir), CapturePrefix},
		{filepath.Join(dir, scratchDir, call.CapturesDir), ""},
	} {
		entries, err := os.ReadDir(d.dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.Type().IsRegular() || !strings.HasPrefix(e.Name(), d.prefix) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				return err
			}
			if info.ModTime().Before(cutoff) {
				if err := os.Remove(filepath.Join(d.dir, e.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// answerCache is how many answers the store at dir caches and the bytes they
// hold.
func answerCache(dir string) (entries, size int, err error) {
	list, err := os.ReadDir(filepath.Join(dir, scratchDir, call.AnswersDir))
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	for _, e := range list {
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return 0, 0, err
		}
		entries++
		size += int(info.Size())
	}
	return entries, size, nil
}

package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"kbase/internal/asks"
	"kbase/internal/atomicfile"
	"kbase/internal/buildstore"
	"kbase/internal/filelock"
	"kbase/internal/result"
	"kbase/internal/write"
)

// regenerateDir, under scratch, is where a resume rebuilds the document graph
// to regenerate missing records; it is removed once read.
const regenerateDir = "regenerate"

// reportPath is where a stage's report lands in the store at dir.
func reportPath(dir, stage string) string {
	return filepath.Join(dir, buildstore.ReportsDir, stage+".yaml")
}

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

// takeOperationLock is the repository's operation lock, recording stateDir; a
// live holder is refused, naming the run and its state directory.
func takeOperationLock(kbRoot, stateDir string) (*filelock.Lock, error) {
	path := buildstore.OperationLockPath(kbRoot)
	l, err := filelock.Acquire(path, buildstore.LockWait)
	if errors.Is(err, filelock.ErrHeld) {
		items, err := runningRefusal(path)
		if err != nil {
			return nil, err
		}
		if items == nil {
			return nil, buildstore.Retry{{Check: buildstore.CheckLock, Path: path, Detail: "another run held the operation lock through the wait and is starting or has just released it"}}
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
func Hold(kbRoot, stateDir string, kind buildstore.Kind, run string, now time.Time) (func() error, error) {
	op, err := takeOperationLock(kbRoot, stateDir)
	if err != nil {
		return nil, err
	}
	l, err := takeLock(stateDir, buildstore.Holder{PID: os.Getpid(), Started: stamp(now), Kind: kind, Run: run})
	if err != nil {
		return nil, errors.Join(err, op.ReleaseRemoving())
	}
	return func() error { return errors.Join(l.ReleaseRemoving(), op.ReleaseRemoving()) }, nil
}

// RunningOperation is the refusal a verb that writes the KB at kbRoot meets
// while a run — a build or a score run — holds the operation lock: the run
// writes kb-root/ without the write lock; nil where none does.
func RunningOperation(kbRoot string) ([]result.Item, error) {
	path := buildstore.OperationLockPath(kbRoot)
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
	h, err := buildstore.ReadHolder(string(dir))
	if err != nil {
		return nil, err
	}
	if h.PID == 0 {
		return []result.Item{{Check: buildstore.CheckLock, Path: string(dir), Detail: "a run is starting; its state-dir is " + string(dir)}}, nil
	}
	return []result.Item{{Check: buildstore.CheckLock, Path: string(dir),
		Detail: fmt.Sprintf("a %s (pid %d) is running; its state-dir is %s", h.Kind.Noun(), h.PID, dir)}}, nil
}

// awaitWriters waits out a write op or refresh already holding the KB's
// write lock; one that takes it later finds the operation lock and refuses.
func awaitWriters(kbRoot string) error {
	release, err := write.LockKB(kbRoot)
	if errors.Is(err, filelock.ErrHeld) {
		return buildstore.Retry{{Check: buildstore.CheckLock, Path: filepath.Dir(kbRoot), Detail: "a write op or refresh holds the KB's write lock"}}
	}
	if err != nil {
		return err
	}
	release()
	return nil
}

// takeLock is the state store's holder lock, its holder record written; a
// live holder is a retry naming it.
func takeLock(dir string, h buildstore.Holder) (*filelock.Lock, error) {
	l, err := filelock.Acquire(buildstore.HolderPath(dir), buildstore.LockWait)
	if errors.Is(err, filelock.ErrHeld) {
		who := "another run"
		if held, err := buildstore.ReadHolder(dir); err == nil && held.PID != 0 {
			who = fmt.Sprintf("another %s (pid %d, started %s)", held.Kind.Noun(), held.PID, held.Started)
		}
		return nil, buildstore.Retry{{Check: buildstore.CheckLock, Path: buildstore.HolderPath(dir), Detail: who + " holds the state store"}}
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

// progress appends events to the record, each synced as it lands so a monitor
// and a resume after a kill read every event that happened.
type progress struct {
	f   *os.File
	now func() time.Time
}

func openProgress(dir string, now func() time.Time) (*progress, error) {
	f, err := os.OpenFile(filepath.Join(dir, buildstore.ProgressFile), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
	if err != nil {
		return nil, err
	}
	return &progress{f, now}, nil
}

func (p *progress) emit(e buildstore.Event) error { return p.emitAll([]buildstore.Event{e}) }

// emitAll appends the events in one write and one sync, each stamped as it is
// written.
func (p *progress) emitAll(events []buildstore.Event) error {
	if len(events) == 0 {
		return nil
	}
	var buf []byte
	for _, e := range events {
		e.Time = stamp(p.now())
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		buf = append(append(buf, b...), '\n')
	}
	if _, err := p.f.Write(buf); err != nil {
		return err
	}
	return p.f.Sync()
}

func (p *progress) close() error { return p.f.Close() }

const (
	// progressRotateLines is how many lines the live progress record may hold
	// before a fresh build moves the earlier runs' lines out of it.
	progressRotateLines = 500
	// keptProgressFiles is how many rotated progress records stand.
	keptProgressFiles = 15
)

// rotateProgress moves, where the live record in dir holds more than
// progressRotateLines lines, every line but the last — this invocation's run
// event, the only line of it — to progress-<stamp>.jsonl, and prunes the
// rotated records to the newest keptProgressFiles. The record is reopened into
// p whatever fails before it, so later events still land.
func (p *progress) rotateProgress(dir string) (err error) {
	live := filepath.Join(dir, buildstore.ProgressFile)
	defer func() {
		f, openErr := os.OpenFile(live, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o666)
		if openErr == nil {
			p.f = f
		}
		err = errors.Join(err, openErr)
	}()
	if err := p.f.Close(); err != nil {
		return err
	}
	data, err := os.ReadFile(live)
	if err != nil {
		return err
	}
	if bytes.Count(data, []byte{'\n'}) <= progressRotateLines {
		return nil
	}
	last := bytes.LastIndexByte(data[:len(data)-1], '\n') + 1
	name := "progress-" + p.now().UTC().Format("20060102T150405.000000000Z") + ".jsonl"
	if err := atomicfile.Write(filepath.Join(dir, name), data[:last], nil); err != nil {
		return err
	}
	if err := atomicfile.Write(live, data[last:], nil); err != nil {
		return err
	}
	rotated, err := filepath.Glob(filepath.Join(dir, "progress-*.jsonl"))
	if err != nil {
		return err
	}
	slices.Sort(rotated)
	for _, path := range rotated[:max(0, len(rotated)-keptProgressFiles)] {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// uncommittedIn is whether the progress record accounts for uncommitted work
// in stage: the latest invocation that entered a stage and was not refused —
// a refused invocation writes nothing — entered stage last and never
// recorded it. A cancelled invocation counts: its stage's work stands
// uncommitted too, whatever word the state is reported under.
func uncommittedIn(events []buildstore.Event, stage string) bool {
	all := buildstore.Invocations(events)
	for i := len(all) - 1; i >= 0; i-- {
		inv := all[i]
		entered, recorded := inv.Entered()
		if entered == "" || inv.End().Outcome == result.Refused {
			continue
		}
		return entered == stage && !recorded
	}
	return false
}

// keptBuilds is how many build trails, beside the current one, keep their
// directories, and how many captured builds, beside the newest, stand.
const keptBuilds = 5

// pruneTrails keeps, of the trail directories under the store at dir's
// scratch, the current one and the keptBuilds newest others by modification
// time, and removes the rest whole. The current directory current — empty
// for a fresh build — is created where it does not stand, and stamped now:
// that stamp is what marks a trail as last built on. The flat answers, group
// records and call captures an earlier kbase wrote are removed.
func pruneTrails(dir, current string) error {
	if current != "" {
		if err := os.MkdirAll(current, 0o777); err != nil {
			return err
		}
		now := time.Now()
		if err := os.Chtimes(current, now, now); err != nil {
			return err
		}
	}
	trails, err := listTrails(dir)
	if err != nil {
		return err
	}
	others := slices.DeleteFunc(trails, func(path string) bool { return path == current })
	for _, path := range others[min(keptBuilds, len(others)):] {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	scratch := filepath.Join(dir, buildstore.ScratchDir)
	for _, flat := range []string{asks.AnswersDir, asks.AskRecordsDir, asks.CapturesDir} {
		if err := os.RemoveAll(filepath.Join(scratch, flat)); err != nil {
			return err
		}
	}
	return nil
}

// listTrails is the trail directories under the store at dir's scratch,
// newest first by modification time; none where no trails directory stands.
func listTrails(dir string) ([]string, error) {
	trails := filepath.Join(dir, buildstore.ScratchDir, buildstore.TrailsDir)
	entries, err := os.ReadDir(trails)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type trail struct {
		path string
		mod  time.Time
	}
	var all []trail
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		all = append(all, trail{filepath.Join(trails, e.Name()), info.ModTime()})
	}
	slices.SortStableFunc(all, func(a, b trail) int { return b.mod.Compare(a.mod) })
	paths := make([]string, len(all))
	for i, t := range all {
		paths[i] = t.path
	}
	return paths, nil
}

// pruneBuildCaptures keeps, under the store at dir's ReportsDir, the newest
// keptBuilds+1 captured builds and removes the rest.
func pruneBuildCaptures(dir string) error {
	return buildstore.PruneCaptures(filepath.Join(dir, buildstore.ReportsDir), buildstore.CapturePrefix, keptBuilds+1)
}

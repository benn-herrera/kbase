package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (p *progress) emit(e buildstore.Event) error {
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

// keptBuilds is how many of the latest builds that ended having walked keep
// their captures in the store.
const keptBuilds = 5

// pruneCaptures removes, from the store at dir, every capture — a started
// build's output under ReportsDir and a call's under scratch — last written
// before the start of the keptBuilds-th latest build that ended neither
// refused nor told to retry. A build killed before its end is not counted.
// The answer cache and the stage reports are kept.
func pruneCaptures(dir string) error {
	events, err := buildstore.ReadProgress(dir)
	if err != nil {
		return err
	}
	var starts []string
	for _, inv := range buildstore.Invocations(events) {
		if o := inv.End().Outcome; o != "" && o != result.Refused && o != result.Retry {
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
		{filepath.Join(dir, buildstore.ReportsDir), buildstore.CapturePrefix},
		{filepath.Join(dir, buildstore.ScratchDir, asks.CapturesDir), ""},
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

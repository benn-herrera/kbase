package build

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	"kbase/internal/kb"
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
	// stateKeyHexDigits is the key's length: the first 16 hex digits of the
	// SHA-256 of the resolved kb-root/ path.
	stateKeyHexDigits = 16
	// lockWait bounds the wait for the run lock and the holder lock, so a
	// monitor probing one does not turn a starting build away.
	lockWait = 2 * time.Second
)

// stateDir is the store for the KB at kbRoot: override, else
// $XDG_STATE_HOME/kbase/<key>, $XDG_STATE_HOME defaulting to
// ~/.local/state. It refuses a store inside kb-root/.
func stateDir(override, kbRoot string) (string, error) {
	dir := override
	if dir == "" {
		key, err := stateKey(kbRoot)
		if err != nil {
			return "", err
		}
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "state")
		}
		dir = filepath.Join(base, "kbase", key)
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if kb.Within(kbRoot, dir) {
		return "", result.Refusal{{Check: checkUsage, Path: dir, Key: "--state-dir",
			Detail: fmt.Sprintf("the state directory %s is inside %s; build state is never written there", dir, kbRoot)}}
	}
	return dir, nil
}

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

// stateKey names the store of the kb-root/ at kbRoot, whether or not it
// exists yet; its directory must.
func stateKey(kbRoot string) (string, error) {
	abs, err := filepath.Abs(kbRoot)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Dir(abs)); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(kb.ResolvePath(abs)))
	return hex.EncodeToString(sum[:])[:stateKeyHexDigits], nil
}

// holder is what the holder lock says about the build holding it.
type holder struct {
	PID     int    `json:"pid"`
	Started string `json:"started"`
}

func readHolder(dir string) (holder, error) {
	var h holder
	b, err := os.ReadFile(filepath.Join(dir, lockFile))
	if err != nil || len(b) == 0 {
		return h, err
	}
	return h, json.Unmarshal(b, &h)
}

// runLockFile is the run lock in the repository's git directory, so two
// builds over one KB exclude each other whatever their state directories and
// the working tree gains no file. It records the holder's state directory;
// the state store's lock still carries the pid and start time status and
// cancel read.
const runLockFile = "kbase-build.lock"

// runLockPath is the run lock of the KB at kbRoot. A .git file is followed to
// the worktree's own git directory, read rather than asked of git, since a
// maintenance verb that checks the lock never requires git.
func runLockPath(kbRoot string) string {
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
	return filepath.Join(dir, runLockFile)
}

// takeRunLock is the repository's run lock, recording stateDir; a live
// holder is refused, naming its state directory.
func takeRunLock(kbRoot, stateDir string) (*filelock.Lock, error) {
	path := runLockPath(kbRoot)
	l, err := filelock.Acquire(path, lockWait)
	if errors.Is(err, filelock.ErrHeld) {
		items, err := runningRefusal(path)
		if err != nil {
			return nil, err
		}
		if items == nil {
			return nil, retry{{Check: checkLock, Path: path, Detail: "another build held the run lock through the wait and is starting or has just released it"}}
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

// RunningBuild is the refusal a verb that writes the KB at kbRoot meets
// while a build holds the run lock — the build writes kb-root/ without the
// write lock; nil where no build does.
func RunningBuild(kbRoot string) ([]result.Item, error) {
	path := runLockPath(kbRoot)
	held, err := filelock.Held(path)
	if err != nil || !held {
		return nil, err
	}
	return runningRefusal(path)
}

// runningRefusal is the refusal naming the run lock's holder; nil where the
// holder has removed the lock on its way out, or has taken it and not yet
// written its state directory. A holder that has not written it has not yet
// waited out the KB's writers, so a writer may go ahead.
func runningRefusal(path string) ([]result.Item, error) {
	dir, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err == nil && len(dir) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []result.Item{{Check: checkLock, Path: string(dir), Detail: "a build is running; its state-dir is " + string(dir)}}, nil
}

// awaitWriters waits out a write op or refresh already holding the KB's
// write lock; one that takes it later finds the run lock and refuses.
func awaitWriters(kbRoot string) error {
	release, err := write.LockKB(kbRoot)
	if errors.Is(err, filelock.ErrHeld) {
		return retry{{Check: checkLock, Path: filepath.Dir(kbRoot), Detail: "a write op or refresh holds the KB's write lock"}}
	}
	if err != nil {
		return err
	}
	release()
	return nil
}

// takeLock is the state store's lock, its holder record written; a live
// holder is a retry naming it.
func takeLock(dir string, now time.Time) (*filelock.Lock, error) {
	l, err := filelock.Acquire(filepath.Join(dir, lockFile), lockWait)
	if errors.Is(err, filelock.ErrHeld) {
		who := "another build"
		if h, err := readHolder(dir); err == nil && h.PID != 0 {
			who = fmt.Sprintf("another build (pid %d, started %s)", h.PID, h.Started)
		}
		return nil, retry{{Check: checkLock, Path: filepath.Join(dir, lockFile), Detail: who + " holds the state store"}}
	}
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(holder{PID: os.Getpid(), Started: stamp(now)})
	if err == nil {
		err = l.Write(b)
	}
	if err != nil {
		l.Release()
		return nil, err
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

// interrupted is whether the progress record accounts for uncommitted work
// in stage: the latest invocation that entered a stage and was not refused —
// a refused invocation writes nothing — entered stage last and never
// recorded it.
func interrupted(events []event, stage string) bool {
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

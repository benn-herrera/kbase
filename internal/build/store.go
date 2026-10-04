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
	"time"

	"kbase/internal/atomicfile"
	"kbase/internal/filelock"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// The state store holds what a build keeps outside the repository: the run
// lock and its holder, the progress record, the document graph's records, and
// scratch — per-call evidence, the answer cache, and the tree a resume
// regenerates missing records from. Nothing in it is authoritative for
// position; the commit trail is.
const (
	lockFile     = "lock"
	progressFile = "progress.jsonl"
	reportsDir   = "reports"
	scratchDir   = "scratch"
	// regenerateDir, under scratch, is where a resume rebuilds the document
	// graph to regenerate missing records; it is removed once read.
	regenerateDir = "regenerate"
	// stateKeyHexDigits is the key's length: the first 16 hex digits of the
	// SHA-256 of the resolved kb-root/ path.
	stateKeyHexDigits = 16
	// lockWait bounds the wait for the run lock, so a monitor probing it
	// does not turn a starting build away.
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
		return "", refusal{{Check: checkUsage, Path: dir, Key: "--state-dir",
			Detail: fmt.Sprintf("the state directory %s is inside %s; build state is never written there", dir, kbRoot)}}
	}
	return dir, nil
}

// reportPath is where a stage's report lands in the store at dir.
func reportPath(dir, stage string) string { return filepath.Join(dir, reportsDir, stage+".yaml") }

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
// exists yet.
func stateKey(kbRoot string) (string, error) {
	resolved, err := filepath.EvalSymlinks(kbRoot)
	if errors.Is(err, os.ErrNotExist) {
		var parent string
		if parent, err = filepath.EvalSymlinks(filepath.Dir(kbRoot)); err == nil {
			resolved = filepath.Join(parent, filepath.Base(kbRoot))
		}
	}
	if err != nil {
		return "", err
	}
	if resolved, err = filepath.Abs(resolved); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(resolved))
	return hex.EncodeToString(sum[:])[:stateKeyHexDigits], nil
}

// holder is what the run lock says about the build holding it.
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

// takeLock is the run lock, its holder record written; a live holder is a
// retry naming it.
func takeLock(dir string, now time.Time) (*filelock.Lock, error) {
	l, err := filelock.Acquire(filepath.Join(dir, lockFile), lockWait)
	if errors.Is(err, filelock.ErrHeld) {
		who := "another build"
		if h, err := readHolder(dir); err == nil && h.PID != 0 {
			who = fmt.Sprintf("another build (pid %d, started %s)", h.PID, h.Started)
		}
		return nil, retry{{Check: checkLock, Path: filepath.Join(dir, lockFile), Detail: who + " holds the run lock"}}
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

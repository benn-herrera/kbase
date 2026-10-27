package buildstore

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"kbase/internal/asks"
	"kbase/internal/filelock"
	"kbase/internal/result"
)

// The state store holds what a build keeps outside the repository: the
// holder's lock and record, the progress record, the document graph's records, and
// scratch — per-call evidence, the answer cache, and the tree a resume
// regenerates missing records from. Nothing in it is authoritative for
// position; the commit trail is.
const (
	lockFile     = "lock"
	ProgressFile = "progress.jsonl"
	// ReportsDir holds each stage's report, and the output a build started
	// in its own process leaves.
	ReportsDir = "reports"
	// CapturePrefix begins the name of each file under ReportsDir that
	// captures the output of a build started in its own process.
	CapturePrefix = "build-"
	ScratchDir    = "scratch"
	// CaptureStdoutSuffix and CaptureStderrSuffix end the two files of one
	// captured run under ReportsDir, which share a stem.
	CaptureStdoutSuffix = ".stdout.yaml"
	CaptureStderrSuffix = ".stderr.log"
	// TrailsDir is where, under ScratchDir, each build trail keeps its
	// answers and group records, in a directory named by the trail's start
	// commit.
	TrailsDir = "trails"
	// LockWait bounds the wait for the operation lock and the holder lock, so
	// a monitor probing one does not turn a starting run away.
	LockWait = 2 * time.Second
)

// Kind is the verb that owns a run: what the holder record names, and what
// both runs' refusals call their holder.
type Kind string

const (
	KindBuild Kind = "build"
	KindScore Kind = "score"
)

// Noun is the run as a refusal and a document name it: "a build", "a score
// run".
func (k Kind) Noun() string {
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

// ReadHolder is the holder record in the store at dir; the zero record where
// none stands. The kind is required: it says which verb owns the run, and a
// record without one this version writes is a store another version left,
// refused rather than read as a build.
func ReadHolder(dir string) (Holder, error) {
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
		return Holder{}, result.Refusal{{Check: CheckStateDir, Path: path, Key: "--state-dir",
			Remedy: "update kbase, or use another --state-dir",
			Detail: fmt.Sprintf("the holder record names kind %q; this kbase writes %q or %q", h.Kind, KindBuild, KindScore)}}
	}
	return h, nil
}

// HolderOf is the state store's holder lock at dir read: whether a run holds
// it, and the record that run wrote — the zero record where no run is
// recorded, and a record this version did not write refused.
func HolderOf(dir string) (live bool, h Holder, err error) {
	if h, err = ReadHolder(dir); err != nil {
		return false, Holder{}, err
	}
	if live, err = filelock.Held(HolderPath(dir)); err != nil {
		return false, Holder{}, err
	}
	return live, h, nil
}

// The progress record's events.
const (
	EventRun      = "run"
	EventStage    = "stage"
	EventUnit     = "unit"
	EventUnits    = "units"
	EventRecorded = "recorded"
	EventFallback = "fallback"
	EventEnd      = "end"
)

// Event is one line of the progress record. A run event opens each
// invocation that took the lock; a stage event enters a stage with its unit
// count, and a units event recounts them where a row reports finer units of
// its own; a unit event closes one unit, a fallback event names an item
// defaulted to its mechanical draft, a recorded event names the stage's
// boundary commit, and an end event closes the invocation with its outcome.
type Event struct {
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

// ReadProgress is the record's events in order; a line a killed process left
// half-written is not an event.
func ReadProgress(dir string) ([]Event, error) {
	f, err := os.Open(filepath.Join(dir, ProgressFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			events = append(events, e)
		}
	}
	return events, sc.Err()
}

// Invocation is one run's events, its run event first.
type Invocation []Event

// Invocations is the record's events grouped by the run event opening each.
func Invocations(events []Event) []Invocation {
	var out []Invocation
	for _, e := range events {
		if e.Event == EventRun {
			out = append(out, Invocation{e})
		} else if len(out) > 0 {
			out[len(out)-1] = append(out[len(out)-1], e)
		}
	}
	return out
}

// End is the invocation's end event, or the zero event where it has none —
// a process that was killed.
func (inv Invocation) End() Event {
	if last := inv[len(inv)-1]; last.Event == EventEnd {
		return last
	}
	return Event{}
}

// recorded reports whether the invocation recorded a stage boundary: the fact
// that decides whether an identical re-issue continues it, since a run that
// recorded none wrote no boundary and leaves the build where the run before it
// left it.
func (inv Invocation) recorded() bool {
	for _, e := range inv {
		if e.Event == EventRecorded {
			return true
		}
	}
	return false
}

// stands reports whether the invocation is the run status and rewind-build read
// the build's position from: every run but one that refused or asked to be
// retried without recording a boundary — that one wrote nothing, so the run
// before it is the one the build stands at.
func (inv Invocation) stands() bool {
	switch inv.End().Outcome {
	case result.Refused, result.Retry:
		return inv.recorded()
	}
	return true
}

// Entered is the last stage the invocation entered and whether it recorded
// it.
func (inv Invocation) Entered() (stage string, recorded bool) {
	for _, e := range inv {
		switch e.Event {
		case EventStage:
			stage, recorded = e.Stage, false
		case EventRecorded:
			recorded = recorded || e.Stage == stage
		}
	}
	return stage, recorded
}

// answerCache is how many answers the store at dir caches across its trail
// directories and the bytes they hold.
func answerCache(dir string) (entries, size int, err error) {
	trails, err := filepath.Glob(filepath.Join(dir, ScratchDir, TrailsDir, "*", asks.AnswersDir))
	if err != nil {
		return 0, 0, err
	}
	for _, answers := range trails {
		list, err := os.ReadDir(answers)
		if errors.Is(err, os.ErrNotExist) {
			continue
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
	}
	return entries, size, nil
}

// PruneCaptures keeps, under reports, the newest keep captured runs whose
// stem begins prefix, and removes the rest. A capture is the
// CaptureStdoutSuffix and CaptureStderrSuffix files sharing a stem; the
// stem's UTC stamp makes name order start order.
func PruneCaptures(reports, prefix string, keep int) error {
	suffixes := []string{CaptureStdoutSuffix, CaptureStderrSuffix}
	entries, err := os.ReadDir(reports)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the captures under %s: %w", reports, err)
	}
	var stems []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		for _, suffix := range suffixes {
			if stem, ok := strings.CutSuffix(name, suffix); ok && !slices.Contains(stems, stem) {
				stems = append(stems, stem)
			}
		}
	}
	slices.Sort(stems)
	for _, stem := range stems[:max(0, len(stems)-keep)] {
		for _, suffix := range suffixes {
			if err := os.Remove(filepath.Join(reports, stem+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove the capture %s: %w", stem, err)
			}
		}
	}
	return nil
}

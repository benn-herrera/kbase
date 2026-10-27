package buildstore

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"kbase/internal/gitledger"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
	"kbase/internal/statedir"
	"kbase/internal/write"
)

// The states status reports.
const (
	StateNone        = "none"
	StateRunning     = "running"
	StateCancelled   = "cancelled"
	StateInterrupted = "interrupted"
	StateFailed      = "failed"
	StateBounded     = "bounded"
	StateFinished    = "finished"
)

// recentItems is how many refusals and fallbacks status lists.
const recentItems = 10

// MonitorOptions is one status read or cancel of the build over the KB the
// working directory is in.
type MonitorOptions struct {
	StateDir string
	WorkDir  string
	Logger   log.Logger
}

func (o MonitorOptions) repo() (*gitledger.Repo, error) {
	if o.Logger == nil {
		o.Logger = log.Discard()
	}
	repo, err := gitledger.Open(o.WorkDir, OwnedPaths, o.Logger)
	var nw gitledger.NotWorktreeError
	if errors.As(err, &nw) {
		return nil, result.Refusal{{Check: CheckWorktree, Path: nw.Dir, Remedy: "git init", Detail: err.Error()}}
	}
	return repo, err
}

// Open is the repository, kb-root/ and the state store, a store newer than
// this kbase refused.
func (o MonitorOptions) Open() (repo *gitledger.Repo, kbRoot, dir string, err error) {
	if repo, err = o.repo(); err != nil {
		return nil, "", "", err
	}
	kbRoot = filepath.Join(repo.Root, kb.KBDir)
	if dir, err = statedir.Dir(o.StateDir, kbRoot); err != nil {
		return repo, kbRoot, dir, err
	}
	_, err = StoreVersion(dir)
	return repo, kbRoot, dir, err
}

// State is the build's state and the run it stands at, as the state store's
// holder lock and progress record report them: one run, named by its pid and
// its start stamp, and whether it is still progressing.
type State struct {
	State   string
	PID     int
	Started string
}

// ReadState is the build's state in the state store at dir, and the run it
// stands at. It reads the store alone, never the commit trail, so a monitor
// may read it as often as it likes.
func ReadState(dir string) (State, error) {
	r, err := readStore(dir)
	if err != nil {
		return State{}, err
	}
	return State{State: r.state(), PID: r.pid(), Started: r.started()}, nil
}

// Ended reports whether state is one a build stops at: the run is over when it
// is one of these, and no further progress is coming.
func Ended(state string) bool {
	switch state {
	case StateFinished, StateBounded, StateCancelled, StateFailed, StateInterrupted:
		return true
	}
	return false
}

// WaitState polls the state store at dir every interval until the run pid is
// no longer progressing — the store stands at another run, or at that run with
// a state it stopped at — and returns the reading that ended the wait. A pid
// the store does not hold returns at once with the store's standing run.
func WaitState(dir string, pid int, interval time.Duration) (State, error) {
	for {
		state, err := ReadState(dir)
		if err != nil {
			return State{}, err
		}
		if state.PID != pid || Ended(state.State) {
			return state, nil
		}
		time.Sleep(interval)
	}
}

// storeReading is what the state store at dir records of the build's run:
// whether one holds it, the run the store stands at, the holder's record, and
// every invocation the progress record holds. status's state and the MCP watcher's
// both derive from it; the commit trail, which costs a git log against the
// repository the build commits to, is status's alone.
type storeReading struct {
	live   bool
	last   Invocation
	all    []Invocation
	holder Holder
}

// readStore is the state store at dir read: its progress record, its holder
// lock and the holder's record. A score run holding the store is no build run:
// it writes no progress event, so the build the store stands at is read as
// though nothing held it.
func readStore(dir string) (storeReading, error) {
	events, err := ReadProgress(dir)
	if err != nil {
		return storeReading{}, err
	}
	live, h, err := HolderOf(dir)
	if err != nil {
		return storeReading{}, err
	}
	if h.Kind == KindScore {
		live = false
	}
	// The run the build stands at is the newest one: a refusal or retry that
	// recorded no boundary wrote nothing, so the run before it stands.
	all := Invocations(events)
	var last Invocation
	for i := len(all) - 1; i >= 0; i-- {
		if live || all[i].stands() {
			last = all[i]
			break
		}
	}
	return storeReading{live: live, last: last, all: all, holder: h}, nil
}

// state is the state the reading reports.
func (r storeReading) state() string {
	switch {
	case r.live:
		return StateRunning
	case r.last != nil:
		end := r.last.End()
		state := map[string]string{
			result.Done: StateFinished, result.Unchanged: StateFinished, result.Bounded: StateBounded,
			result.Cancelled: StateCancelled,
		}[end.Outcome]
		switch {
		case end.Event == "":
			return StateInterrupted
		case state == "":
			return StateFailed
		}
		return state
	}
	return StateNone
}

// pid is the run's process: the holder's while a run holds the store, else
// the run's own; zero where the store records no run.
func (r storeReading) pid() int {
	if r.live {
		return r.holder.PID
	}
	if r.last != nil {
		return r.last[0].PID
	}
	return 0
}

// started is the run's start stamp; "" where the store records no run.
func (r storeReading) started() string {
	if r.last != nil {
		return r.last[0].Time
	}
	return ""
}

// Status is the build's state read from the holder lock, the progress record
// and the commit trail, and the size of the answer cache.
func Status(opts MonitorOptions) (string, []result.Field) {
	repo, kbRoot, dir, err := opts.Open()
	if err != nil {
		return MonitorEnd(nil, err)
	}
	placement := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	trail, err := repo.Trail()
	if err != nil {
		return MonitorEnd(placement, err)
	}
	read, err := readStore(dir)
	if err != nil {
		return MonitorEnd(placement, err)
	}
	last, state := read.last, read.state()
	recorded := RecordedStages(trail)
	stages := make([]result.Record, len(Stages))
	var current any
	for i, s := range Stages {
		var commit any
		if c := recorded[s.ID]; c != "" {
			commit = c
		}
		stages[i] = result.Record{{Key: "stage", Value: s.ID}, {Key: "commit", Value: commit}}
		if current == nil && recorded[s.ID] == "" {
			current = currentStage(s, last)
		}
	}
	var run Event
	if last != nil {
		run = last[0]
	}
	pid := read.pid()
	// resume is the command that continues the work: none while a build is
	// running it, none for a finished or bounded build, whose own command
	// repeats where it stopped, and none where the record's command does not
	// read as a kbase build invocation. The run this reads is the one the build
	// stands at, so a refusal that recorded no boundary hands out nothing: it
	// is not the run reported.
	command, confirmed := RecordedCommand(run.Resume)
	if !confirmed || state == StateFinished || state == StateRunning || state == StateBounded {
		command = ""
	}
	cacheEntries, cacheBytes, err := answerCache(dir)
	if err != nil {
		return MonitorEnd(placement, err)
	}
	return result.Done, append(placement,
		result.Field{Key: "state", Value: state},
		result.Field{Key: "pid", Value: Nullable(pid)},
		result.Field{Key: "started", Value: Nullable(run.Time)},
		result.Field{Key: "updated", Value: Nullable(lastTime(last))},
		result.Field{Key: "ended", Value: Nullable(endTime(last))},
		result.Field{Key: "no-inference", Value: run.NoInference},
		result.Field{Key: "stages", Value: stages},
		result.Field{Key: "current", Value: current},
		result.Field{Key: "recent-refusals", Value: recent(read.all, func(inv Invocation) []result.Item {
			if e := inv.End(); e.Outcome == result.Refused || e.Outcome == result.Retry || e.Outcome == result.Failed {
				return e.Items
			}
			return nil
		})},
		result.Field{Key: "recent-fallbacks", Value: recent(read.all, func(inv Invocation) []result.Item {
			var items []result.Item
			for _, e := range inv {
				if e.Event == EventFallback {
					items = append(items, e.Items...)
				}
			}
			return items
		})},
		result.Field{Key: "cache-entries", Value: cacheEntries},
		result.Field{Key: "cache-bytes", Value: cacheBytes},
		result.Field{Key: "resume", Value: Nullable(command)},
	)
}

// MonitorEnd is a status read or cancel that stopped on err, with what it had
// resolved.
func MonitorEnd(fields []result.Field, err error) (string, []result.Field) {
	end := EndingOf("", err)
	return end.Outcome, append(fields, end.Fields()...)
}

// currentStage is the first unrecorded stage and its units done of total,
// as the last invocation that entered it left them.
func currentStage(s Stage, last Invocation) result.Record {
	noInference := len(last) > 0 && last[0].NoInference
	run, _ := StageSteps(s.ID, noInference)
	done, total := 0, len(run)
	for _, e := range last {
		switch {
		case e.Stage != s.ID:
		case e.Event == EventStage, e.Event == EventUnits:
			done, total = 0, e.Total
		case e.Event == EventUnit:
			done, total = e.Done, e.Total
		}
	}
	return result.Record{{Key: "stage", Value: s.ID}, {Key: "units-done", Value: done}, {Key: "units-total", Value: total}}
}

func recent(all []Invocation, items func(Invocation) []result.Item) []result.Item {
	var out []result.Item
	for _, inv := range all {
		out = append(out, items(inv)...)
	}
	if len(out) > recentItems {
		out = out[len(out)-recentItems:]
	}
	return append([]result.Item{}, out...)
}

func lastTime(inv Invocation) string {
	if len(inv) == 0 {
		return ""
	}
	return inv[len(inv)-1].Time
}

func endTime(inv Invocation) string {
	if len(inv) == 0 {
		return ""
	}
	return inv.End().Time
}

// Nullable is v, or nil where it is its type's zero: what a monitor states of
// a value the store does not record, such as the pid of a run no process holds.
func Nullable[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}
	return v
}

// RecordedCommand renders a recorded invocation for a document, and reports
// whether it reads as a kbase build invocation at all: a record naming another
// program was not written by kbase, and is never rendered for a harness to run.
func RecordedCommand(argv []string) (string, bool) {
	if len(argv) < 2 || argv[0] != "kbase" || argv[1] != "build" {
		return "", false
	}
	return ShellJoin(argv), true
}

// ShellJoin quotes each word a POSIX shell would split or expand.
func ShellJoin(argv []string) string {
	words := make([]string, len(argv))
	for i, a := range argv {
		if a != "" && strings.Trim(a, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@%+=:,./-_") == "" {
			words[i] = a
		} else {
			words[i] = "'" + strings.ReplaceAll(a, "'", `'"'"'`) + "'"
		}
	}
	return strings.Join(words, " ")
}

// Locate is the kb-root and the state store a build started from
// opts.WorkDir works in, a relative opts.StateDir being the repository
// root's; outcome is "" there, else it and fields are the document saying why
// there are none.
func Locate(opts MonitorOptions) (kbRoot, dir, outcome string, fields []result.Field) {
	if opts.StateDir != "" && !filepath.IsAbs(opts.StateDir) {
		repo, err := opts.repo()
		if err != nil {
			outcome, fields = MonitorEnd(nil, err)
			return "", "", outcome, fields
		}
		opts.StateDir = filepath.Join(repo.Root, opts.StateDir)
	}
	_, kbRoot, dir, err := opts.Open()
	if err != nil {
		outcome, fields = MonitorEnd(nil, err)
	}
	return kbRoot, dir, outcome, fields
}

// Launched reads the latest invocation the process pid opened in the state store
// at dir: whether it has passed its launch checks and entered a stage, and
// the command that resumes it, empty where the record's command does not read
// as a kbase build invocation.
func Launched(dir string, pid int) (entered bool, resume string, err error) {
	events, err := ReadProgress(dir)
	if err != nil {
		return false, "", err
	}
	all := Invocations(events)
	for i := len(all) - 1; i >= 0; i-- {
		if all[i][0].PID != pid {
			continue
		}
		entered = slices.ContainsFunc(all[i], func(e Event) bool { return e.Event == EventStage })
		command, _ := RecordedCommand(all[i][0].Resume)
		return entered, command, nil
	}
	return false, "", nil
}

// StartWait bounds how long the starter of a build in its own process waits
// for it to enter its first stage or exit: past the build's own waits for
// the operation lock, the holder lock and a writer in progress, and its launch
// checks.
const StartWait = write.LockWait + 2*LockWait + 6*time.Second

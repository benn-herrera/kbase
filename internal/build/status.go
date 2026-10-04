package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kbase/internal/filelock"
	"kbase/internal/kb"
	"kbase/internal/ledger"
	"kbase/internal/log"
	"kbase/internal/result"
)

// The states status reports.
const (
	stateNone      = "none"
	stateRunning   = "running"
	stateCancelled = "cancelled"
	stateFailed    = "failed"
	stateBounded   = "bounded"
	stateFinished  = "finished"
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

func (o MonitorOptions) open() (repo *ledger.Repo, kbRoot, dir string, err error) {
	if o.Logger == nil {
		o.Logger = log.Discard()
	}
	repo, err = ledger.Open(o.WorkDir, ownedPaths, o.Logger)
	var nw ledger.NotWorktreeError
	if errors.As(err, &nw) {
		return nil, "", "", refusal{{Check: checkWorktree, Path: nw.Dir, Remedy: "git init", Detail: err.Error()}}
	}
	if err != nil {
		return nil, "", "", err
	}
	kbRoot = filepath.Join(repo.Root, kb.KBDir)
	dir, err = stateDir(o.StateDir, kbRoot)
	return repo, kbRoot, dir, err
}

// Status is the build's state read from the run lock, the progress record
// and the commit trail.
func Status(opts MonitorOptions) (string, []result.Field) {
	repo, kbRoot, dir, err := opts.open()
	if err != nil {
		return monitorEnd(nil, err)
	}
	placement := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	trail, err := repo.Trail()
	if err != nil {
		return monitorEnd(placement, err)
	}
	events, err := readProgress(dir)
	if err != nil {
		return monitorEnd(placement, err)
	}
	live, err := filelock.Held(filepath.Join(dir, lockFile))
	if err != nil {
		return monitorEnd(placement, err)
	}
	all := invocations(events)
	var last invocation
	for i := len(all) - 1; i >= 0; i-- {
		if live || all[i].end().Outcome != result.Refused {
			last = all[i]
			break
		}
	}
	state := stateNone
	switch {
	case live:
		state = stateRunning
	case last != nil:
		state = map[string]string{
			result.Done: stateFinished, result.Unchanged: stateFinished, result.Bounded: stateBounded,
			result.Cancelled: stateCancelled,
		}[last.end().Outcome]
		if state == "" {
			state = stateFailed
		}
	}
	w := &walk{trail: trail}
	recorded := w.recorded()
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
	var run event
	if last != nil {
		run = last[0]
	}
	h, err := readHolder(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return monitorEnd(placement, err)
	}
	pid := run.PID
	if live {
		pid = h.PID
	}
	var resume any
	if state != stateFinished && state != stateRunning && run.Resume != nil {
		resume = shellJoin(run.Resume)
	}
	return result.Done, append(placement,
		result.Field{Key: "state", Value: state},
		result.Field{Key: "pid", Value: nullable(pid)},
		result.Field{Key: "started", Value: nullable(run.Time)},
		result.Field{Key: "updated", Value: nullable(lastTime(last))},
		result.Field{Key: "ended", Value: nullable(endTime(last))},
		result.Field{Key: "no-inference", Value: run.NoInference},
		result.Field{Key: "stages", Value: stages},
		result.Field{Key: "current", Value: current},
		result.Field{Key: "recent-refusals", Value: recent(all, func(inv invocation) []result.Item {
			if e := inv.end(); e.Outcome == result.Refused || e.Outcome == result.Retry || e.Outcome == result.Failed {
				return e.Items
			}
			return nil
		})},
		result.Field{Key: "recent-fallbacks", Value: recent(all, func(inv invocation) []result.Item {
			var items []result.Item
			for _, e := range inv {
				if e.Event == eventFallback {
					items = append(items, e.Items...)
				}
			}
			return items
		})},
		result.Field{Key: "resume", Value: resume},
	)
}

// monitorEnd is a status read or cancel that stopped on err, with what it had
// resolved.
func monitorEnd(fields []result.Field, err error) (string, []result.Field) {
	end := ended("", err)
	return end.outcome, append(fields, end.fields()...)
}

// currentStage is the first unrecorded stage and its units done of total,
// as the last invocation that entered it left them.
func currentStage(s Stage, last invocation) result.Record {
	noInference := len(last) > 0 && last[0].NoInference
	run, _ := stageRows(s.ID, noInference)
	done, total := 0, len(run)
	for _, e := range last {
		switch {
		case e.Stage != s.ID:
		case e.Event == eventStage, e.Event == eventUnits:
			done, total = 0, e.Total
		case e.Event == eventUnit:
			done, total = e.Done, e.Total
		}
	}
	return result.Record{{Key: "stage", Value: s.ID}, {Key: "units-done", Value: done}, {Key: "units-total", Value: total}}
}

func recent(all []invocation, items func(invocation) []result.Item) []result.Item {
	var out []result.Item
	for _, inv := range all {
		out = append(out, items(inv)...)
	}
	if len(out) > recentItems {
		out = out[len(out)-recentItems:]
	}
	return append([]result.Item{}, out...)
}

func lastTime(inv invocation) string {
	if len(inv) == 0 {
		return ""
	}
	return inv[len(inv)-1].Time
}

func endTime(inv invocation) string {
	if len(inv) == 0 {
		return ""
	}
	return inv.end().Time
}

func nullable[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}
	return v
}

// shellJoin quotes each word a POSIX shell would split or expand.
func shellJoin(argv []string) string {
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

// cancelWait bounds how long cancel waits for the holder to let the lock go.
const cancelWait = 20 * time.Second

// Cancel signals the build holding the run lock and waits for it to stop:
// the unit in flight is abandoned with nothing written for it, and the build
// stays resumable.
func Cancel(opts MonitorOptions) (string, []result.Field) {
	_, kbRoot, dir, err := opts.open()
	if err != nil {
		return monitorEnd(nil, err)
	}
	fields := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	lock := filepath.Join(dir, lockFile)
	live, err := filelock.Held(lock)
	if err != nil {
		return monitorEnd(fields, err)
	}
	if !live {
		return monitorEnd(fields, refusal{{Check: checkLock, Path: lock, Detail: "no build holds the run lock"}})
	}
	h, err := readHolder(dir)
	if err != nil {
		return monitorEnd(fields, err)
	}
	fields = append(fields, result.Field{Key: "pid", Value: h.PID})
	p, err := os.FindProcess(h.PID)
	if err == nil {
		err = p.Signal(os.Interrupt)
	}
	if err != nil {
		return monitorEnd(fields, fmt.Errorf("signalling pid %d: %w", h.PID, err))
	}
	for deadline := time.Now().Add(cancelWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if live, err = filelock.Held(lock); err != nil || !live {
			break
		}
	}
	if err != nil {
		return monitorEnd(fields, err)
	}
	if live {
		return monitorEnd(fields, retry{{Check: checkLock, Path: lock,
			Detail: fmt.Sprintf("pid %d was signalled and has not let the run lock go within %v", h.PID, cancelWait)}})
	}
	resume, err := lastResume(dir)
	if err != nil {
		return monitorEnd(fields, err)
	}
	return result.Done, append(fields, result.Field{Key: "resume", Value: resume})
}

// lastResume is the command that resumes the build the progress record
// names last, or nil where it names none.
func lastResume(dir string) (any, error) {
	events, err := readProgress(dir)
	if err != nil {
		return nil, err
	}
	all := invocations(events)
	if len(all) == 0 || all[len(all)-1][0].Resume == nil {
		return nil, nil
	}
	return shellJoin(all[len(all)-1][0].Resume), nil
}

package score

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/buildstore"
	"kbase/internal/result"
)

// Status reports the run the state store stands at, as the status command
// reads it: the run the holder record names, else — where no run holds the
// store — whichever of its newest runs started later, a score run or a build
// invocation, and the build's own result where the store records neither or
// the two starts cannot be ordered. It reads the store's records alone.
func Status(opts buildstore.MonitorOptions) (string, []result.Field) {
	_, stateDir, outcome, fields := buildstore.Locate(opts)
	if outcome != "" {
		return outcome, fields
	}
	opts.StateDir = stateDir
	placement := []result.Field{{Key: "state-dir", Value: stateDir}}
	live, h, err := buildstore.HolderOf(stateDir)
	if err != nil {
		return buildstore.MonitorEnd(placement, err)
	}
	if h.Kind == buildstore.KindScore {
		return runResult(stateDir, h, live)
	}
	if live {
		return buildstore.Status(opts)
	}
	run, started, err := NewestRun(stateDir)
	if err != nil {
		return buildstore.MonitorEnd(placement, err)
	}
	if run == "" {
		return buildstore.Status(opts)
	}
	buildState, err := buildstore.ReadState(stateDir)
	if err != nil {
		return buildstore.MonitorEnd(placement, err)
	}
	if startedAfter(started, buildState.Started) {
		return runResult(stateDir, buildstore.Holder{Run: run, Started: started}, false)
	}
	return buildstore.Status(opts)
}

// startedAfter reports whether the start stamp a is later than b. Two starts
// that do not parse cannot be ordered, so a missing or unreadable stamp on
// either side is no later start: status then reports the build, and a resume
// is not refused for one.
func startedAfter(a, b string) bool {
	at, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return false
	}
	bt, err := time.Parse(time.RFC3339, b)
	if err != nil {
		return false
	}
	return at.After(bt)
}

// runResult reports the score run the holder record h names in the state
// store at stateDir: the state a build's ending takes from the outcome its
// stored result records, else running
// while a process holds the store and interrupted where none does; the counts
// from its ledger; and the command that continues it where the run has not
// finished. h is the store's own record where one names the run, and where the
// store names none and the run is reported as its newest, a record carrying the
// run and the start its header records, with no process.
func runResult(stateDir string, h buildstore.Holder, live bool) (string, []result.Field) {
	placement := []result.Field{{Key: "state-dir", Value: stateDir}}
	run := h.Run
	st, err := ReadRun(stateDir, run)
	if errors.Is(err, os.ErrNotExist) {
		// The holder names a run whose ledger has not opened yet: nothing is
		// counted, and the run is still starting.
		st = RunStatus{Run: run}
	} else if err != nil {
		return buildstore.MonitorEnd(placement, err)
	}
	state := buildstore.StateRunning
	if !live {
		state = buildstore.StateInterrupted
	}
	_, doc, wrote := readResult(stateDir, run)
	if wrote {
		state = buildstore.StateOf(outcomeOf(doc))
	}
	fields := []result.Field{
		{Key: "state-dir", Value: stateDir},
		{Key: "state", Value: state},
		{Key: "pid", Value: buildstore.Nullable(h.PID)},
		{Key: "started", Value: buildstore.Nullable(h.Started)},
		{Key: "run", Value: run},
		{Key: "pass", Value: st.Pass},
		{Key: "counts", Value: CountsRecord(st.Counts)},
	}
	if len(st.Scored) > 0 {
		fields = append(fields, result.Field{Key: "scored", Value: st.Scored})
	}
	if len(st.FailedItems) > 0 {
		fields = append(fields, result.Field{Key: "failed-items", Value: st.FailedItems})
	}
	if state != buildstore.StateFinished && state != buildstore.StateRunning {
		if resume := doc["resume"]; resume != nil {
			fields = append(fields, result.Field{Key: "resume", Value: resume})
		}
	}
	return result.Done, fields
}

// ResumeOf is the resume command the stored result of the run in the state
// store at stateDir carries; nil where the run wrote none.
func ResumeOf(stateDir, run string) any {
	_, doc, _ := readResult(stateDir, run)
	return doc["resume"]
}

// readResult is the stored result the run in the state store at stateDir
// wrote: its bytes and its decoding, and wrote false where none can be read. A
// stored result that does not decode keeps its bytes and decodes to nil.
func readResult(stateDir, run string) (b []byte, doc map[string]any, wrote bool) {
	b, err := os.ReadFile(filepath.Join(RunsDir(stateDir), run, ResultFile))
	if err != nil {
		return nil, nil, false
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return b, nil, true
	}
	return b, doc, true
}

// WaitRun polls the score run named by pid until it ends, and returns the run's
// stored result and the exit code its outcome maps to. A run whose process
// ended without writing a result reports the failure the ledger supports.
func WaitRun(stateDir string, pid int, interval time.Duration) (string, int, error) {
	run, err := RunByPID(stateDir, pid)
	if err != nil {
		return "", 0, err
	}
	for {
		if b, doc, wrote := readResult(stateDir, run); wrote {
			return string(b), result.ExitCode(outcomeOf(doc)), nil
		}
		// The run is live while the store's holder record names it: a build
		// started after it ended holds the store too, and is no reason to wait.
		live, h, err := buildstore.HolderOf(stateDir)
		if err != nil {
			return "", 0, err
		}
		if !live || h.PID != pid {
			break
		}
		time.Sleep(interval)
	}
	doc, err := interruptedResult(stateDir, run)
	if err != nil {
		return "", 0, err
	}
	return doc, result.ExitCode(result.Failed), nil
}

// outcomeOf is a decoded stored result's outcome, result.Failed where it
// names none.
func outcomeOf(doc map[string]any) string {
	if outcome, _ := doc["outcome"].(string); outcome != "" {
		return outcome
	}
	return result.Failed
}

// interruptedResult is the result reported for a score run that ended without
// writing its own: the ledger's counts and failures, and the failure naming the
// process's end. It reuses the run's ledger, the record the run wrote, rather
// than inventing a parallel one.
func interruptedResult(stateDir, run string) (string, error) {
	st, err := ReadRun(stateDir, run)
	if err != nil {
		return "", err
	}
	fields := []result.Field{
		{Key: "state-dir", Value: stateDir},
		{Key: "pass", Value: st.Pass},
		{Key: "run", Value: run},
		{Key: "ledger", Value: filepath.Join(RunsDir(stateDir), run)},
		{Key: "counts", Value: CountsRecord(st.Counts)},
	}
	if len(st.Scored) > 0 {
		fields = append(fields, result.Field{Key: "scored", Value: st.Scored})
	}
	fields = append(fields, result.Field{Key: result.FailuresKey, Value: []result.Item{{
		Check: checkError, Path: filepath.Join(RunsDir(stateDir), run),
		Detail: "the run's process ended without writing its result; its ledger records what it attempted",
	}}})
	var b bytes.Buffer
	if err := result.Emit(&b, result.Failed, fields...); err != nil {
		return "", err
	}
	return b.String(), nil
}

// CountsRecord is a run's counts as the result states them.
func CountsRecord(c Counts) result.Record {
	return result.Record{
		{Key: "total", Value: c.Total},
		{Key: "done", Value: c.Done},
		{Key: "failed", Value: c.Failed},
		{Key: "skipped", Value: c.Skipped},
		{Key: "remaining", Value: c.Remaining},
	}
}

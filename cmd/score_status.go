package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"

	"kbase/internal/build"
	"kbase/internal/filelock"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// persistScoreResult writes the run's result document beside its ledger, so a
// waiter reads the document the verb reported. A run that named no ledger
// writes nothing.
func persistScoreResult(fields []toolresult.Field, outcome string) {
	var dir string
	for _, f := range fields {
		if f.Key == "ledger" {
			dir, _ = f.Value.(string)
			break
		}
	}
	if dir == "" {
		return
	}
	_ = score.WriteResultDocument(dir, outcome, fields)
}

// waitScore polls the score run named by pid until it ends, and returns the
// run's result document and the exit code its outcome maps to. A run whose
// process ended without writing a result reports the failure the ledger
// supports.
func waitScore(stateDir string, pid int, interval time.Duration) (string, int, error) {
	run, err := score.RunByPID(stateDir, pid)
	if err != nil {
		return "", 0, err
	}
	dir := filepath.Join(score.RunsDir(stateDir), run)
	for {
		if b, err := os.ReadFile(filepath.Join(dir, score.ResultFile)); err == nil {
			outcome := outcomeOf(b)
			return string(b), exitCodes[outcome], nil
		}
		// The run is live while the store's holder record names it: a build
		// started after it ended holds the store too, and is no reason to wait.
		live, h, err := build.HolderOf(stateDir)
		if err != nil {
			return "", 0, err
		}
		if !live || h.PID != pid {
			break
		}
		time.Sleep(interval)
	}
	doc, err := interruptedScoreDocument(stateDir, run)
	if err != nil {
		return "", 0, err
	}
	return doc, exitCodes[toolresult.Failed], nil
}

// outcomeOf reads a result document's outcome.
func outcomeOf(b []byte) string {
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return toolresult.Failed
	}
	outcome, _ := doc["outcome"].(string)
	if outcome == "" {
		return toolresult.Failed
	}
	return outcome
}

// interruptedScoreDocument is the document a score run that ended without
// writing its result reports: the ledger's counts and failures, and the failure
// naming the process's end. It reuses the run's ledger, the record the run
// wrote, rather than inventing a parallel one.
func interruptedScoreDocument(stateDir, run string) (string, error) {
	st, err := score.ReadRun(stateDir, run)
	if err != nil {
		return "", err
	}
	fields := []toolresult.Field{
		{Key: "state-dir", Value: stateDir},
		{Key: "pass", Value: st.Pass},
		{Key: "run", Value: run},
		{Key: "ledger", Value: filepath.Join(score.RunsDir(stateDir), run)},
		{Key: "counts", Value: scoreCountsRecord(st.Counts)},
	}
	if len(st.Scored) > 0 {
		fields = append(fields, toolresult.Field{Key: "scored", Value: st.Scored})
	}
	fields = append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: []toolresult.Item{{
		Check: checkError, Path: filepath.Join(score.RunsDir(stateDir), run),
		Detail: "the run's process ended without writing its result; its ledger records what it attempted",
	}}})
	var b bytes.Buffer
	if err := toolresult.Emit(&b, toolresult.Failed, fields...); err != nil {
		return "", err
	}
	return b.String(), nil
}

// scoreCountsRecord is a run's counts as the document states them.
func scoreCountsRecord(c score.Counts) toolresult.Record {
	return toolresult.Record{
		{Key: "total", Value: c.Total},
		{Key: "done", Value: c.Done},
		{Key: "failed", Value: c.Failed},
		{Key: "remaining", Value: c.Remaining},
	}
}

// nullableInt is v, or nil where it is zero: what a monitor states of a run it
// does not hold.
func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// nullableString is s, or nil where it is empty.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// scoreStatusDocument reports a score run's status for the state store at dir,
// as the status command reads it: the state from the store's holder record and
// the run's result, the counts from its ledger, and the command that continues
// it where the run has not finished. A store whose holder record names a build
// is the build's, and reports as the build does.
func scoreStatusDocument(opts build.MonitorOptions, stateDir string) (string, []toolresult.Field) {
	placement := []toolresult.Field{{Key: "state-dir", Value: stateDir}}
	live, h, err := build.HolderOf(stateDir)
	if err != nil {
		return monitorFailure(placement, err)
	}
	if h.Kind != build.KindScore {
		return build.Status(opts)
	}
	run := h.Run
	st, err := score.ReadRun(stateDir, run)
	if errors.Is(err, os.ErrNotExist) {
		// The holder names a run whose ledger has not opened yet: nothing is
		// counted, and the run is still starting.
		st = score.RunStatus{Run: run}
	} else if err != nil {
		return monitorFailure(placement, err)
	}
	state := "running"
	if !live {
		state = "interrupted"
	}
	if b, err := os.ReadFile(filepath.Join(score.RunsDir(stateDir), run, score.ResultFile)); err == nil {
		if o := outcomeOf(b); o != "" {
			state = o
		}
	}
	fields := []toolresult.Field{
		{Key: "state-dir", Value: stateDir},
		{Key: "state", Value: state},
		{Key: "pid", Value: nullableInt(h.PID)},
		{Key: "started", Value: nullableString(h.Started)},
		{Key: "run", Value: run},
		{Key: "pass", Value: st.Pass},
		{Key: "counts", Value: scoreCountsRecord(st.Counts)},
	}
	if len(st.Scored) > 0 {
		fields = append(fields, toolresult.Field{Key: "scored", Value: st.Scored})
	}
	if len(st.Failures) > 0 {
		fields = append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: st.Failures})
	}
	if state != "done" && state != "bounded" && state != "running" {
		if resume := resumeFromResult(stateDir, run); resume != nil {
			fields = append(fields, toolresult.Field{Key: "resume", Value: resume})
		}
	}
	return toolresult.Done, fields
}

// monitorFailure is a status or cancel read that failed, as the monitor
// document states it.
func monitorFailure(fields []toolresult.Field, err error) (string, []toolresult.Field) {
	var refusal toolresult.Refusal
	if errors.As(err, &refusal) {
		return toolresult.Refused, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: []toolresult.Item(refusal)})
	}
	return toolresult.Failed, append(fields, toolresult.Field{Key: toolresult.FailuresKey, Value: []toolresult.Item{{
		Check: "error", Detail: err.Error(),
	}}})
}

// resumeFromResult reads the resume command the run's result document carries.
func resumeFromResult(stateDir, run string) any {
	b, err := os.ReadFile(filepath.Join(score.RunsDir(stateDir), run, score.ResultFile))
	if err != nil {
		return nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil
	}
	return doc["resume"]
}

// scoreOrBuildStatus is the status command's verb: the score run the store's
// holder record names, else the build's status.
func scoreOrBuildStatus(opts build.MonitorOptions) (string, []toolresult.Field) {
	_, dir, outcome, fields := build.Locate(opts)
	if outcome != "" {
		return outcome, fields
	}
	return scoreStatusDocument(opts, dir)
}

// cancelScore stops the scoring run holding the state store: it signals the
// process the store's holder record names and waits for the holder lock to be
// let go. A store whose record names a build, or no run, is build's cancel to
// answer.
func cancelScore(opts build.MonitorOptions) (string, []toolresult.Field) {
	kbRoot, dir, outcome, fields := build.Locate(opts)
	if outcome != "" {
		return outcome, fields
	}
	placement := []toolresult.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	live, h, err := build.HolderOf(dir)
	if err != nil {
		return monitorFailure(placement, err)
	}
	if h.Kind != build.KindScore || !live {
		return build.Cancel(opts)
	}
	lock := build.HolderPath(dir)
	fields = append(placement, toolresult.Field{Key: "pid", Value: h.PID})
	p, err := os.FindProcess(h.PID)
	if err == nil {
		err = p.Signal(os.Interrupt)
	}
	if err != nil {
		return monitorFailure(fields, fmt.Errorf("signalling pid %d: %w", h.PID, err))
	}
	for deadline := time.Now().Add(cancelWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if live, err = filelock.Held(lock); err != nil || !live {
			break
		}
	}
	if err != nil {
		return monitorFailure(fields, err)
	}
	if live {
		return toolresult.Retry, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: []toolresult.Item{{
			Check: checkLock, Path: lock,
			Detail: fmt.Sprintf("pid %d was signalled and has not let go of the state store within %v", h.PID, cancelWait)}}})
	}
	resume := resumeFromResult(dir, h.Run)
	return toolresult.Done, append(fields, toolresult.Field{Key: "resume", Value: resume})
}

// cancelWait bounds how long cancel waits for the run it signalled to let the
// store's holder lock go.
const cancelWait = 20 * time.Second

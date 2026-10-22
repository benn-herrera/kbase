package main

import (
	"bytes"
	"encoding/json"
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

// scoreHolderFile is the record of the score run a process is walking, in the
// runs directory: the pid, the run and the start stamp a monitor reads. It is
// written when a run takes the lock and removed when the run lets go, so its
// presence names a run in flight.
const scoreHolderFile = "holder"

// scoreHolder is what a running score run records of itself.
type scoreHolder struct {
	PID     int    `json:"pid"`
	Started string `json:"started"`
	Run     string `json:"run"`
}

// writeScoreHolder records the run this process is walking, so status and
// cancel find it without scanning every run.
func writeScoreHolder(stateDir, run string) error {
	h := scoreHolder{PID: os.Getpid(), Started: time.Now().UTC().Format(time.RFC3339), Run: run}
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(score.RunsDir(stateDir), scoreHolderFile), b, 0o666)
}

// removeScoreHolder drops the holder record when the run lets go of the lock.
func removeScoreHolder(stateDir string) error {
	err := os.Remove(filepath.Join(score.RunsDir(stateDir), scoreHolderFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// readScoreHolder reads the run a process is walking, or os.ErrNotExist where
// no run is in flight.
func readScoreHolder(stateDir string) (scoreHolder, error) {
	var h scoreHolder
	b, err := os.ReadFile(filepath.Join(score.RunsDir(stateDir), scoreHolderFile))
	if err != nil {
		return h, err
	}
	return h, json.Unmarshal(b, &h)
}

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
		live, err := filelock.Held(score.RunsDir(stateDir))
		if err != nil {
			return "", 0, err
		}
		if !live {
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
// as the status command reads it: the state from the run's holder and result,
// the counts from its ledger, and the command that continues it where the run
// has not finished.
func scoreStatusDocument(opts build.MonitorOptions, stateDir string) (string, []toolresult.Field) {
	placement := []toolresult.Field{{Key: "state-dir", Value: stateDir}}
	h, err := readScoreHolder(stateDir)
	if errors.Is(err, os.ErrNotExist) {
		return build.Status(opts)
	}
	if err != nil {
		return monitorFailure(placement, err)
	}
	run := h.Run
	live, err := filelock.Held(score.RunsDir(stateDir))
	if err != nil {
		return monitorFailure(placement, err)
	}
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

// scoreOrBuildStatus is the status command's verb: the score run in flight where
// one holds the store, else the build's status.
func scoreOrBuildStatus(opts build.MonitorOptions) (string, []toolresult.Field) {
	_, dir, outcome, fields := build.Locate(opts)
	if outcome != "" {
		return outcome, fields
	}
	return scoreStatusDocument(opts, dir)
}

// cancelScore stops the score run holding the state store: it signals the
// process the holder names and waits for the runs directory's lock to be let
// go. A store no score run holds falls back to build's cancel.
func cancelScore(opts build.MonitorOptions) (string, []toolresult.Field) {
	kbRoot, dir, outcome, fields := build.Locate(opts)
	if outcome != "" {
		return outcome, fields
	}
	placement := []toolresult.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	h, err := readScoreHolder(dir)
	if errors.Is(err, os.ErrNotExist) {
		return build.Cancel(opts)
	}
	if err != nil {
		return monitorFailure(placement, err)
	}
	live, err := filelock.Held(score.RunsDir(dir))
	if err != nil {
		return monitorFailure(placement, err)
	}
	if !live {
		return monitorFailure(placement, toolresult.Refusal{{Check: "lock", Path: score.RunsDir(dir), Detail: "no score run holds the state store"}})
	}
	fields = append(placement, toolresult.Field{Key: "pid", Value: h.PID})
	p, err := os.FindProcess(h.PID)
	if err == nil {
		err = p.Signal(os.Interrupt)
	}
	if err != nil {
		return monitorFailure(fields, fmt.Errorf("signalling pid %d: %w", h.PID, err))
	}
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if live, err = filelock.Held(score.RunsDir(dir)); err != nil || !live {
			break
		}
	}
	if err != nil {
		return monitorFailure(fields, err)
	}
	if live {
		return toolresult.Retry, append(fields, toolresult.Field{Key: toolresult.RefusalsKey, Value: []toolresult.Item{{
			Check: "lock", Path: score.RunsDir(dir),
			Detail: fmt.Sprintf("pid %d was signalled and has not let go of the runs directory within 20s", h.PID)}}})
	}
	resume := resumeFromResult(dir, h.Run)
	return toolresult.Done, append(fields, toolresult.Field{Key: "resume", Value: resume})
}

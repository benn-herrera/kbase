package main

import (
	"kbase/internal/build"
	"kbase/internal/buildstore"
	"kbase/internal/log"
	toolresult "kbase/internal/result"
	"kbase/internal/score"
)

// persistScoreResult writes the run's result document beside its ledger, so a
// waiter reads the document the verb reported. A run that named no ledger
// writes nothing. It runs on the way out of a verb that cannot report an error
// there and that has already built the document it emits, so a write that fails
// is recorded against the run rather than lost.
func persistScoreResult(lg log.Logger, fields []toolresult.Field, outcome string) {
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
	if err := score.WriteResultDocument(dir, outcome, fields); err != nil {
		lg.Error("the run's result document was not written", "ledger", dir, "error", err)
	}
}

// cancelScore stops the scoring run holding the state store: it signals the
// process the store's holder record names and waits for the holder lock to be
// let go. A store whose record names a build, or no run, is build's cancel to
// answer.
func cancelScore(opts buildstore.MonitorOptions) (string, []toolresult.Field) {
	kbRoot, dir, outcome, fields := buildstore.Locate(opts)
	if outcome != "" {
		return outcome, fields
	}
	opts.StateDir = dir
	placement := []toolresult.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	live, h, err := buildstore.HolderOf(dir)
	if err != nil {
		return buildstore.MonitorEnd(placement, err)
	}
	if h.Kind != buildstore.KindScore || !live {
		return build.Cancel(opts)
	}
	fields = append(placement, toolresult.Field{Key: "pid", Value: h.PID})
	if err := build.StopHolder(dir, h.PID); err != nil {
		return buildstore.MonitorEnd(fields, err)
	}
	resume := score.ResumeOf(dir, h.Run)
	return toolresult.Done, append(fields, toolresult.Field{Key: "resume", Value: resume})
}

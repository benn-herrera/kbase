package build

import (
	"fmt"
	"os"
	"time"

	"kbase/internal/buildstore"
	"kbase/internal/filelock"
	"kbase/internal/result"
)

// cancelWait bounds how long cancel waits for the holder to let the lock go.
const cancelWait = 20 * time.Second

// Cancel signals the run holding the state store and waits for it to stop:
// the unit in flight is abandoned with nothing written for it, and the run
// stays resumable.
func Cancel(opts buildstore.MonitorOptions) (string, []result.Field) {
	_, kbRoot, dir, err := opts.Open()
	if err != nil {
		return buildstore.MonitorEnd(nil, err)
	}
	fields := []result.Field{{Key: "kb-root", Value: kbRoot}, {Key: "state-dir", Value: dir}}
	lock := buildstore.HolderPath(dir)
	live, err := filelock.Held(lock)
	if err != nil {
		return buildstore.MonitorEnd(fields, err)
	}
	if !live {
		return buildstore.MonitorEnd(fields, result.Refusal{{Check: buildstore.CheckLock, Path: lock, Detail: "no run holds the state store"}})
	}
	h, err := buildstore.ReadHolder(dir)
	if err != nil {
		return buildstore.MonitorEnd(fields, err)
	}
	if h.PID == 0 {
		return buildstore.MonitorEnd(fields, buildstore.Retry{{Check: buildstore.CheckLock, Path: lock, Detail: "a run holds the state store and has not written its record yet"}})
	}
	fields = append(fields, result.Field{Key: "pid", Value: h.PID})
	if err := StopHolder(dir, h.PID); err != nil {
		return buildstore.MonitorEnd(fields, err)
	}
	resume, err := lastResume(dir)
	if err != nil {
		return buildstore.MonitorEnd(fields, err)
	}
	return result.Done, append(fields, result.Field{Key: "resume", Value: resume})
}

// StopHolder interrupts pid, the run holding the state store at dir, and waits
// for it to let the store's holder lock go: a buildstore.Retry where it still
// holds it after cancelWait. Whichever run the holder record names — a build or
// a scoring run — stops at its own boundary on the signal.
func StopHolder(dir string, pid int) error {
	p, err := os.FindProcess(pid)
	if err == nil {
		err = p.Signal(os.Interrupt)
	}
	if err != nil {
		return fmt.Errorf("signalling pid %d: %w", pid, err)
	}
	lock := buildstore.HolderPath(dir)
	for deadline := time.Now().Add(cancelWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		held, err := filelock.Held(lock)
		if err != nil {
			return err
		}
		if !held {
			return nil
		}
	}
	return buildstore.Retry{{Check: buildstore.CheckLock, Path: lock,
		Detail: fmt.Sprintf("pid %d was signalled and has not let go of the state store within %v", pid, cancelWait)}}
}

// lastResume is the command that resumes the build the progress record names
// last, or nil where it names none or names a command that does not read as a
// kbase build invocation.
func lastResume(dir string) (any, error) {
	events, err := buildstore.ReadProgress(dir)
	if err != nil {
		return nil, err
	}
	all := buildstore.Invocations(events)
	if len(all) == 0 {
		return nil, nil
	}
	command, confirmed := buildstore.RecordedCommand(all[len(all)-1][0].Resume)
	if !confirmed {
		return nil, nil
	}
	return command, nil
}

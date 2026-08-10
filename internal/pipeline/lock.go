package pipeline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kbase/internal/log"
	"kbase/internal/version"
)

const (
	// LockFileName is the single-writer lockfile in the job directory
	// (ARCHITECTURE.md §9, §12). Two kbase runs over one job dir would
	// interleave stage artifacts and stamps, and the result would look
	// perfectly valid to a later resume — every unit stamped, every hash
	// consistent, the content assembled from two different intentions. So
	// the second writer is refused rather than serialized.
	LockFileName = "job.lock"

	// lockTimeFormat is how the holder's start time is recorded. RFC3339 in
	// the local zone: the reader of this file is a human deciding whether
	// the run that wrote it is still going.
	lockTimeFormat = time.RFC3339
)

// Lock is a held single-writer lock on a job directory.
type Lock struct {
	path string
}

// AcquireLock takes the job directory's lockfile, creating the directory if
// it is missing. On contention it returns a LockedError carrying what the
// existing lockfile says.
//
// Stale-lock policy: there is none, deliberately. A lock is never broken
// automatically — not on a pid that looks dead, not on an age threshold.
// Both heuristics are wrong in the case that matters: pids are reused and
// mean nothing across hosts or containers, and a "surely it's dead by now"
// timeout is a race against exactly the long stage a big corpus produces,
// where breaking the lock corrupts the job it was protecting. The failure
// mode of refusing is that a user deletes one file after a crash; the failure
// mode of guessing is two writers in one job dir. So the error names the file
// and quotes its contents, and the human makes the call in about a second.
func AcquireLock(jobDir string, lg log.Logger) (*Lock, error) {
	if err := os.MkdirAll(jobDir, artifactDirMode); err != nil {
		return nil, fmt.Errorf("pipeline: create %s: %w", jobDir, err)
	}
	path := filepath.Join(jobDir, LockFileName)

	// O_EXCL is the whole mechanism: the create either wins or reports that
	// someone else already did. There is no read-then-write window to lose.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, artifactFileMode)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, LockedError{Path: path, Holder: readHolder(path)}
		}
		return nil, fmt.Errorf("pipeline: create %s: %w", path, err)
	}

	host, hostErr := os.Hostname()
	if hostErr != nil {
		host = "unknown"
	}
	body := fmt.Sprintf("pid = %d\nhost = %q\nstarted = %q\nversion = %q\n",
		os.Getpid(), host, time.Now().Format(lockTimeFormat), version.Current)
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("pipeline: write %s: %w", path, err)
	}
	// Synced for the same reason artifacts are: the value of this file is
	// entirely in what it tells a human after a crash, and an empty lockfile
	// tells them nothing.
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("pipeline: write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("pipeline: write %s: %w", path, err)
	}

	lg.Debug("pipeline job lock acquired", "path", path, "pid", os.Getpid())
	return &Lock{path: path}, nil
}

// Release removes the lockfile. A lockfile already gone is not an error: that
// is the documented remedy for a stale lock, and a job that ran to completion
// should not fail at the last step because someone cleaned up in front of it.
//
// The lock is advisory in the only sense that matters — it is a file, and
// deleting it while a run holds it defeats it. That is the price of never
// breaking one automatically.
func (l *Lock) Release() error {
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("pipeline: release %s: %w", l.path, err)
	}
	return nil
}

// readHolder returns the existing lockfile's contents for the contention
// message, flattened onto one line. It is quoted verbatim rather than parsed:
// the file's only consumer is a human reading the error, and a parser would
// be a second format to keep in sync for no gain — plus an unparseable
// lockfile must still produce a useful message.
func readHolder(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(unreadable: %v)", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return "(empty)"
	}
	return strings.Join(fields, " ")
}

// LockedError reports a job directory another writer holds. Its message is
// the whole user experience of the contention case, so it says what is held,
// who says they hold it, and the one action available.
type LockedError struct {
	Path   string
	Holder string
}

func (e LockedError) Error() string {
	return fmt.Sprintf("pipeline: this job directory is locked by another kbase run [%s]; "+
		"if no kbase is running, delete %s and try again", e.Holder, e.Path)
}

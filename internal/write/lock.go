package write

import (
	"path/filepath"
	"time"

	"kbase/internal/filelock"
)

// LockWait bounds the wait for another writer — its read, its write,
// its trailing refresh and sheets — so a stopped peer turns a write, or a
// starting build, into a retry rather than a hang.
const LockWait = 30 * time.Second

// LockKB takes the write lock of the KB at kbRoot: the exclusive lock on the
// directory kb-root/ sits in, which exists before kb-root/ does and leaves
// nothing to clean up. A holder that keeps it past the wait is
// filelock.ErrHeld.
func LockKB(kbRoot string) (release func(), err error) {
	l, err := filelock.Acquire(filepath.Dir(kbRoot), LockWait)
	if err != nil {
		return nil, err
	}
	return l.Release, nil
}

// RetryRemedy is the remedy of op turned away by a concurrent writer.
func RetryRemedy(op string) string {
	return "re-run kbase " + op + " with these values unchanged — never re-author values that were already right"
}

package write

import (
	"errors"
	"path/filepath"
	"time"

	"kbase/internal/filelock"
	"kbase/internal/kb"
	"kbase/internal/result"
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

// HeldLockItem is the item a writer answers with where err is LockKB's
// filelock.ErrHeld for the KB at kbRoot, remedy the writer's own; held is false
// for any other err, nil included.
func HeldLockItem(err error, kbRoot, remedy string) (item result.Item, held bool) {
	if !errors.Is(err, filelock.ErrHeld) {
		return result.Item{}, false
	}
	return result.Item{Check: kb.CheckLock, Path: filepath.Dir(kbRoot), Remedy: remedy,
		Detail: "another write op or refresh held the KB's write lock past the wait; nothing was written"}, true
}

// RetryRemedy is the remedy of op turned away by a concurrent writer.
func RetryRemedy(op string) string {
	return "re-run kbase " + op + " with these values unchanged — never re-author values that were already right"
}

package write

import (
	"errors"
	"fmt"
	"time"

	"kbase/internal/filelock"
)

// lockTimeout bounds the wait for another writer's critical section — a
// re-read, a proof and a rename — so a stopped peer turns a write into a
// retry rather than a hang.
const lockTimeout = 5 * time.Second

// lockDir holds the exclusive lock on the directory itself: it exists before
// a creation, survives every rename into it, and leaves nothing to clean up.
// Exhausting the wait is a retry, the answer a moved file gets.
func lockDir(dir, subject string) (func(), error) {
	l, err := filelock.Acquire(dir, lockTimeout)
	if errors.Is(err, filelock.ErrHeld) {
		return nil, &storeError{reason: reasonContended, subject: subject, contended: true,
			detail: fmt.Sprintf("is held by another writer that did not finish within %v", lockTimeout)}
	}
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", dir, err)
	}
	return l.Release, nil
}

// Package filelock is kbase's one advisory lock: an exclusive lock on a
// directory or on a lock file, held by an open descriptor, so the operating
// system releases it when the holder exits however it exits.
package filelock

import (
	"errors"
	"os"
	"time"
)

// ErrHeld is the answer when another holder kept the lock past the wait.
var ErrHeld = errors.New("held by another process")

// poll is the interval between attempts while waiting for a holder.
const poll = 5 * time.Millisecond

// Lock is a held lock.
type Lock struct{ f *os.File }

// Acquire takes the exclusive lock on path — an existing directory, or a lock
// file it creates — waiting up to wait for another holder to let go.
func Acquire(path string, wait time.Duration) (*Lock, error) {
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		held, err := tryExclusive(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if !held {
			return &Lock{f}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, ErrHeld
		}
		time.Sleep(poll)
	}
}

// Held is whether another holder has path's lock. A path that does not exist
// has no holder.
func Held(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	return probe(f)
}

// Write replaces the lock file's contents with data: what a status reader
// learns about the holder.
func (l *Lock) Write(data []byte) error {
	if err := l.f.Truncate(0); err != nil {
		return err
	}
	if _, err := l.f.WriteAt(data, 0); err != nil {
		return err
	}
	return l.f.Sync()
}

// Release lets the lock go.
func (l *Lock) Release() {
	unlock(l.f)
	l.f.Close()
}

func open(path string) (*os.File, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return os.Open(path)
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
}

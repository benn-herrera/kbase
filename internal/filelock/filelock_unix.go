//go:build unix

package filelock

import (
	"errors"
	"os"
	"syscall"
)

// tryExclusive takes the exclusive lock without blocking; held reports
// another holder.
func tryExclusive(f *os.File) (held bool, err error) {
	return contended(syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))
}

// probe takes and drops a shared lock: refused only while an exclusive
// holder has it.
func probe(f *os.File) (bool, error) {
	held, err := contended(syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB))
	if err == nil && !held {
		unlock(f)
	}
	return held, err
}

func unlock(f *os.File) { syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

func contended(err error) (bool, error) {
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return true, nil
	}
	return false, err
}

//go:build unix

package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockExcludesASecondHolder(t *testing.T) {
	for name, path := range map[string]string{"file": filepath.Join(t.TempDir(), "lock"), "directory": t.TempDir()} {
		t.Run(name, func(t *testing.T) {
			if held, err := Held(path); err != nil || held {
				t.Fatalf("Held before any holder = %t, %v", held, err)
			}
			first, err := Acquire(path, 0)
			if err != nil {
				t.Fatal(err)
			}
			if held, err := Held(path); err != nil || !held {
				t.Errorf("Held under a holder = %t, %v", held, err)
			}
			if _, err := Acquire(path, 0); !errors.Is(err, ErrHeld) {
				t.Errorf("second Acquire = %v, want ErrHeld", err)
			}
			first.Release()
			if held, err := Held(path); err != nil || held {
				t.Errorf("Held after release = %t, %v", held, err)
			}
			second, err := Acquire(path, 0)
			if err != nil {
				t.Fatalf("Acquire after release: %v", err)
			}
			second.Release()
		})
	}
}

// A waiter that opened the file before the holder removed it must not come
// away holding the removed file while a newcomer locks the new one.
func TestReleaseRemovingLeavesOneHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, err := Acquire(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	waited := make(chan *Lock)
	go func() {
		l, err := Acquire(path, 5*time.Second)
		if err != nil {
			t.Error(err)
		}
		waited <- l
	}()
	time.Sleep(50 * time.Millisecond)
	if err := first.ReleaseRemoving(); err != nil {
		t.Fatal(err)
	}
	second := <-waited
	if second == nil {
		t.FailNow()
	}
	defer second.Release()
	if held, err := Held(path); err != nil || !held {
		t.Errorf("Held after the waiter took over = %t, %v; want the waiter seen at path", held, err)
	}
	if _, err := Acquire(path, 0); !errors.Is(err, ErrHeld) {
		t.Errorf("a newcomer's Acquire = %v, want ErrHeld", err)
	}
	if err := second.ReleaseRemoving(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the lock file stands after ReleaseRemoving: %v", err)
	}
}

func TestHeldOnAMissingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	if held, err := Held(path); err != nil || held {
		t.Errorf("Held(absent) = %t, %v", held, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Held created %s", path)
	}
}

func TestWriteReplacesTheContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	l, err := Acquire(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	for _, s := range []string{"a long first holder record", "short"} {
		if err := l.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != s {
			t.Errorf("contents = %q, %v; want %q", got, err, s)
		}
	}
}

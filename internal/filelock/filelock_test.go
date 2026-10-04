//go:build unix

package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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

package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/log/logtest"
)

func TestLockRefusesASecondWriter(t *testing.T) {
	dir := t.TempDir()
	lg := &logtest.Capture{}

	held, err := AcquireLock(dir, lg)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	_, err = AcquireLock(dir, log.Discard())
	var locked LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("second AcquireLock = %v, want a LockedError", err)
	}
	// The message is the whole user experience of contention, so it has to
	// carry both halves of the decision: who holds it, and what to delete.
	if !strings.Contains(err.Error(), fmt.Sprintf("pid = %d", os.Getpid())) {
		t.Errorf("refusal %q does not identify the holder", err)
	}
	if !strings.Contains(err.Error(), LockFileName) {
		t.Errorf("refusal %q does not name the lockfile to delete", err)
	}

	if err := held.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	again, err := AcquireLock(dir, lg)
	if err != nil {
		t.Fatalf("AcquireLock after release: %v", err)
	}
	if err := again.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// TestLockReleaseIsIdempotent: deleting the lockfile is the documented remedy
// for a stale lock, so a run that finds it already gone must not fail at its
// last step.
func TestLockReleaseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	held, err := AcquireLock(dir, &logtest.Capture{})
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err != nil {
		t.Errorf("second Release = %v, want nil", err)
	}
}

func TestLockCreatesTheJobDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "job", "nested")
	held, err := AcquireLock(dir, &logtest.Capture{})
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer held.Release()

	body, err := os.ReadFile(filepath.Join(dir, LockFileName))
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	for _, want := range []string{"pid =", "host =", "started =", "version ="} {
		if !strings.Contains(string(body), want) {
			t.Errorf("lockfile %q carries no %s line", body, want)
		}
	}
}

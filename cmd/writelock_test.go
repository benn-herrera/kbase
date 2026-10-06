//go:build unix

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/filelock"
	"kbase/internal/log"
	"kbase/internal/write"
)

// lockedRepo is maintenanceRepo refreshed, with a claim register created.
func lockedRepo(t *testing.T) string {
	t.Helper()
	repo := maintenanceRepo(t)
	if code, out := runMaintenance(t, runRefresh, repo); code != 0 {
		t.Fatalf("refresh: exit %d\n%s", code, out)
	}
	if out, err := insertClaim(repo, "Seed"); err != nil || !strings.Contains(out, `outcome: "done"`) {
		t.Fatalf("seed insert: %v\n%s", err, out)
	}
	return repo
}

// insertClaim inserts a claim titled title into the root register; it
// reports through its return so a goroutine may call it.
func insertClaim(repo, title string) (string, error) {
	values := filepath.Join(repo, title+".yaml")
	if err := os.WriteFile(values, fmt.Appendf(nil, "entry:\n- register: claim-quality.md\n  title: %s\n  rigor: 0.5\n  rationale: Shown.\n", title), 0o644); err != nil {
		return "", err
	}
	var out bytes.Buffer
	_, err := runWriteOp(writeOpOptions{Op: "insert-claim-entry", WorkDir: repo, ValuesFile: values, Create: true,
		Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	return out.String(), err
}

func runQuiet(run func(maintenanceOptions) (int, error), repo string) (string, error) {
	var out bytes.Buffer
	_, err := run(maintenanceOptions{WorkDir: repo, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	return out.String(), err
}

func register(t *testing.T, repo string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "kb-root", "claim-quality.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// concurrently runs each call at once and returns their results in order.
func concurrently(calls ...func() (string, error)) ([]string, []error) {
	outs, errs := make([]string, len(calls)), make([]error, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outs[i], errs[i] = call()
		}()
	}
	wg.Wait()
	return outs, errs
}

// landedOrRetried checks a racing insert: done with its title in the
// register, or retry with the lock item; never done and lost.
func landedOrRetried(t *testing.T, repo, title, out string, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	d := checkDocument(t, "insert-claim-entry", out)
	switch d.Outcome {
	case "done":
		if !strings.Contains(register(t, repo), title) {
			t.Errorf("insert of %q reported done and the register lacks it:\n%s", title, register(t, repo))
		}
	case "retry":
		if items := d.items("refusals"); len(items) != 1 || items[0]["check"] != checkLock {
			t.Errorf("insert of %q retried with %v, want one lock item", title, items)
		}
	default:
		t.Errorf("insert of %q = %s, want done or retry\n%s", title, d.Outcome, out)
	}
}

func TestRacingWriteOpsLoseNoEdit(t *testing.T) {
	repo := lockedRepo(t)
	for round := range 3 {
		a, b := fmt.Sprintf("Left %d", round), fmt.Sprintf("Right %d", round)
		outs, errs := concurrently(func() (string, error) { return insertClaim(repo, a) }, func() (string, error) { return insertClaim(repo, b) })
		landedOrRetried(t, repo, a, outs[0], errs[0])
		landedOrRetried(t, repo, b, outs[1], errs[1])
	}
}

func TestRefreshRacingAWriteOpLosesNoEdit(t *testing.T) {
	repo := lockedRepo(t)
	for round := range 3 {
		title := fmt.Sprintf("Raced %d", round)
		outs, errs := concurrently(func() (string, error) { return insertClaim(repo, title) }, func() (string, error) { return runQuiet(runRefresh, repo) })
		landedOrRetried(t, repo, title, outs[0], errs[0])
		if errs[1] != nil {
			t.Fatal(errs[1])
		}
		if d := checkDocument(t, "refresh", outs[1]); d.Outcome != "done" && d.Outcome != "unchanged" && d.Outcome != "retry" {
			t.Errorf("racing refresh = %s\n%s", d.Outcome, outs[1])
		}
	}
}

func TestWriteVerbsRefuseWhileABuildRuns(t *testing.T) {
	repo := lockedRepo(t)
	before := register(t, repo)
	verified, err := runQuiet(runVerify, repo)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "state")
	held, err := filelock.Acquire(filepath.Join(repo, ".git", "kbase-build.lock"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if err := held.Write([]byte(state)); err != nil {
		t.Fatal(err)
	}

	insert, insertErr := insertClaim(repo, "Blocked")
	refreshed, refreshErr := runQuiet(runRefresh, repo)
	rendered, renderErr := runQuiet(runRenderClaimGraph, repo)
	for _, c := range []struct {
		verb, out string
		err       error
	}{{"insert-claim-entry", insert, insertErr}, {"refresh", refreshed, refreshErr}, {"render-claim-graph", rendered, renderErr}} {
		if c.err != nil {
			t.Fatal(c.err)
		}
		d := checkDocument(t, c.verb, c.out)
		items := d.items("refusals")
		if d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkLock || items[0]["path"] != state ||
			items[0]["detail"] != "a build is running; its state-dir is "+state || items[0]["remedy"] != nil {
			t.Errorf("%s under a running build:\n%s", c.verb, c.out)
		}
	}
	if after := register(t, repo); after != before {
		t.Errorf("a refused write changed the register:\n%s", after)
	}
	if out, err := runQuiet(runVerify, repo); err != nil || out != verified {
		t.Errorf("verify under a running build: %v\n%s\nwant as before the build:\n%s", err, out, verified)
	}
	if out, _ := kbase(t, repo, "stats"); checkDocument(t, "stats", out).Outcome != "done" {
		t.Errorf("stats under a running build:\n%s", out)
	}
}

// TestWriterRefusedAtOnceWhileABuildWaitsOutAnother: with a build holding the
// run lock and another writer holding the write lock, a writer is refused
// without waiting for the write lock.
func TestWriterRefusedAtOnceWhileABuildWaitsOutAnother(t *testing.T) {
	repo := lockedRepo(t)
	run, err := filelock.Acquire(filepath.Join(repo, ".git", "kbase-build.lock"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Release()
	if err := run.Write([]byte(filepath.Join(t.TempDir(), "state"))); err != nil {
		t.Fatal(err)
	}
	writer, err := filelock.Acquire(repo, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Release()
	start := time.Now()
	out, err := insertClaim(repo, "Blocked")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); checkDocument(t, "insert-claim-entry", out).Outcome != "refused" || took > write.LockWait/2 {
		t.Errorf("a writer behind a waiting build took %v:\n%s", took, out)
	}
}

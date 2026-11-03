//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"kbase/internal/buildstore"
	"kbase/internal/filelock"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/write"
)

// lockedRepo is maintenanceRepo committed and through update-format, with a
// work recorded.
func lockedRepo(t *testing.T) string {
	t.Helper()
	repo := maintenanceRepo(t)
	updateFormatKB(t, repo)
	if out, err := addWork(repo, "Seed"); err != nil || !strings.Contains(out, `outcome: "done"`) {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	return repo
}

// addWork records a work titled title, its label the title without spaces;
// it reports through its return so a goroutine may call it.
func addWork(repo, title string) (string, error) {
	values := filepath.Join(repo, title+".yaml")
	if err := os.WriteFile(values, fmt.Appendf(nil, "entry:\n- label: %s\n  title: %s\n  rationale: Shown.\n",
		strings.ReplaceAll(title, " ", ""), title), 0o644); err != nil {
		return "", err
	}
	var out bytes.Buffer
	_, err := runWriteOp(writeOpOptions{Op: write.AddWork, WorkDir: repo, ValuesFile: values,
		Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	return out.String(), err
}

func runQuiet(run func(maintenanceOptions) (int, error), repo string) (string, error) {
	var out bytes.Buffer
	_, err := run(maintenanceOptions{WorkDir: repo, Stdout: &out, Stderr: &bytes.Buffer{}, Logger: log.Discard()})
	return out.String(), err
}

// nodeRecords is the KB's node record file as it stands.
func nodeRecords(t *testing.T, repo string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, kb.KBDir, filepath.FromSlash(kb.GraphNodesFile)))
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

// landedOrRetried checks a racing add: done with its title in the node
// records, or retry with the lock item; never done and lost.
func landedOrRetried(t *testing.T, repo, title, out string, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	d := checkDocument(t, write.AddWork, out)
	switch d.Outcome {
	case "done":
		if records := nodeRecords(t, repo); !strings.Contains(records, title) {
			t.Errorf("add of %q reported done and the node records lack it:\n%s", title, records)
		}
	case "retry":
		if items := d.items("refusals"); len(items) != 1 || items[0]["check"] != checkLock {
			t.Errorf("add of %q retried with %v, want one lock item", title, items)
		}
	default:
		t.Errorf("add of %q = %s, want done or retry\n%s", title, d.Outcome, out)
	}
}

func TestRacingWriteOpsLoseNoEdit(t *testing.T) {
	repo := lockedRepo(t)
	for round := range 3 {
		a, b := fmt.Sprintf("Left %d", round), fmt.Sprintf("Right %d", round)
		outs, errs := concurrently(func() (string, error) { return addWork(repo, a) }, func() (string, error) { return addWork(repo, b) })
		landedOrRetried(t, repo, a, outs[0], errs[0])
		landedOrRetried(t, repo, b, outs[1], errs[1])
	}
}

// TestUpdateFormatRacingAWriteOpLosesNoEdit: update-format over a committed
// kb-root/ and a write op race for the write lock. The write lands or is told
// to retry; update-format runs, is told to retry, or — taking the lock after
// the write landed — finds kb-root/ dirty and refuses.
func TestUpdateFormatRacingAWriteOpLosesNoEdit(t *testing.T) {
	repo := lockedRepo(t)
	for round := range 3 {
		commitKB(t, repo)
		title := fmt.Sprintf("Raced %d", round)
		outs, errs := concurrently(func() (string, error) { return addWork(repo, title) }, func() (string, error) { return runQuiet(runUpdateFormat, repo) })
		landedOrRetried(t, repo, title, outs[0], errs[0])
		if errs[1] != nil {
			t.Fatal(errs[1])
		}
		d := checkDocument(t, "update-format", outs[1])
		dirty := d.Outcome == "refused" && !slices.ContainsFunc(d.items("refusals"), func(it map[string]any) bool { return it["check"] != checkDirtyPaths })
		if d.Outcome != "done" && d.Outcome != "unchanged" && d.Outcome != "retry" && !dirty {
			t.Errorf("racing update-format = %s\n%s", d.Outcome, outs[1])
		}
	}
}

func TestWriteVerbsRefuseWhileABuildRuns(t *testing.T) {
	repo := lockedRepo(t)
	before := nodeRecords(t, repo)
	verified, err := runQuiet(runVerify, repo)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := filelock.Acquire(filepath.Join(repo, ".git", "kbase-operation.lock"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if err := held.Write([]byte(state)); err != nil {
		t.Fatal(err)
	}
	record, err := json.Marshal(buildstore.Holder{PID: os.Getpid(), Started: time.Now().UTC().Format(time.RFC3339), Kind: buildstore.KindBuild})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(buildstore.HolderPath(state), record, 0o666); err != nil {
		t.Fatal(err)
	}

	added, addErr := addWork(repo, "Blocked")
	updated, updateErr := runQuiet(runUpdateFormat, repo)
	detail := fmt.Sprintf("a build (pid %d) is running; its state-dir is %s", os.Getpid(), state)
	for _, c := range []struct {
		verb, out string
		err       error
	}{{write.AddWork, added, addErr}, {"update-format", updated, updateErr}} {
		if c.err != nil {
			t.Fatal(c.err)
		}
		d := checkDocument(t, c.verb, c.out)
		items := d.items("refusals")
		if d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkLock || items[0]["path"] != state ||
			items[0]["detail"] != detail || items[0]["remedy"] != nil {
			t.Errorf("%s under a running build:\n%s", c.verb, c.out)
		}
	}
	dryRun := filepath.Join(repo, "dry-run.yaml")
	if err := os.WriteFile(dryRun, []byte("dry-run: true\nentry:\n- label: Dry\n  title: Dry\n  rationale: Shown.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var dryOut bytes.Buffer
	if _, err := runWriteOp(writeOpOptions{Op: write.AddWork, WorkDir: repo, ValuesFile: dryRun,
		Stdout: &dryOut, Stderr: &bytes.Buffer{}, Logger: log.Discard()}); err != nil {
		t.Fatal(err)
	}
	d := checkDocument(t, write.AddWork, dryOut.String())
	if items := d.items("refusals"); d.Outcome != "refused" || len(items) != 1 || items[0]["check"] != checkLock || d.Values["dry-run"] != true {
		t.Errorf("a dry run under a running build:\n%s", dryOut.String())
	}
	if after := nodeRecords(t, repo); after != before {
		t.Errorf("a refused write changed the node records:\n%s", after)
	}
	if out, err := runQuiet(runVerify, repo); err != nil || out != verified {
		t.Errorf("verify under a running build: %v\n%s\nwant as before the build:\n%s", err, out, verified)
	}
	if out, _ := kbase(t, repo, "stats"); checkDocument(t, "stats", out).Outcome != "done" {
		t.Errorf("stats under a running build:\n%s", out)
	}
}

// TestWriterRefusedAtOnceWhileABuildWaitsOutAnother: with a run holding the
// operation lock and another writer holding the write lock, a writer is
// refused without waiting for the write lock.
func TestWriterRefusedAtOnceWhileABuildWaitsOutAnother(t *testing.T) {
	repo := lockedRepo(t)
	run, err := filelock.Acquire(filepath.Join(repo, ".git", "kbase-operation.lock"), 0)
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
	out, err := addWork(repo, "Blocked")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); checkDocument(t, write.AddWork, out).Outcome != "refused" || took > write.LockWait/2 {
		t.Errorf("a writer behind a waiting build took %v:\n%s", took, out)
	}
}

package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/log"
)

// The sweep deletes recursively, so what stops it reaching an operator's files
// is the whole of this file's subject: it is rooted in a directory kbase made,
// one level below whatever `--out` named.

// TestNothingOutsideTempWorkIsTouched is the case that made this structural
// (ARCHITECTURE.md §12): an output directory the operator already keeps files
// in, pointed at by a run that completes and therefore sweeps.
//
// The foreign file is not a stray — it is the shape of `--out notes` against a
// notes directory, or `--out .`, which are the two an operator reaches for
// first on a development verb.
func TestNothingOutsideTempWorkIsTouched(t *testing.T) {
	out := t.TempDir()
	const foreign = "sync.md"
	want := "the operator's own document\n"
	if err := os.WriteFile(filepath.Join(out, foreign), []byte(want), 0o600); err != nil {
		t.Fatalf("write the operator's file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(out, "notes"), 0o700); err != nil {
		t.Fatalf("make the operator's directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "notes", "deeper.md"), []byte(want), 0o600); err != nil {
		t.Fatalf("write the operator's nested file: %v", err)
	}

	client, res := runToCompletion(t, out, ModeResume)
	if client.callCount() != synthCalls {
		t.Fatalf("%d calls, want a complete run of %d", client.callCount(), synthCalls)
	}
	if res.Scan.Stages != res.Stages {
		t.Fatalf("the run did not describe its whole chain, so it never swept: %+v", res)
	}

	for _, rel := range []string{foreign, filepath.Join("notes", "deeper.md")} {
		got, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Errorf("%s did not survive the run: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want it untouched (%q)", rel, got, want)
		}
	}
	// And the run's own output is where it belongs, all of it.
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("read the output directory: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 3 {
		t.Errorf("the output directory holds %v, want the two operator entries plus %s",
			names, TempWorkDirName)
	}
	if len(storeState(t, out)) != 2*synthUnits {
		t.Errorf("the store holds %d files, want an artifact and a stamp per unit under %s",
			len(storeState(t, out)), TempWorkDirName)
	}
}

// TestSweepRefusesARootItDidNotMake is the tripwire behind the structure: one
// string comparison against the day a store is rooted somewhere kbase did not
// create for this purpose.
func TestSweepRefusesARootItDidNotMake(t *testing.T) {
	dir := t.TempDir()
	err := NewStore(dir, log.Discard()).sweep(synthPlan(t).Chain())
	if err == nil {
		t.Fatal("a sweep over a root that is not a temp-work directory must be refused")
	}
	if !strings.Contains(err.Error(), TempWorkDirName) {
		t.Errorf("err = %v, want it to name what a sweepable root is", err)
	}
}

// TestTempWorkMirrorsTheOutputTreeOnDemand: a store-relative path means the
// same thing on both sides of the scratch boundary, and the scratch side is
// created by the write that needs it rather than planned up front.
func TestTempWorkMirrorsTheOutputTreeOnDemand(t *testing.T) {
	out := t.TempDir()
	work, err := OpenTempWork(out, log.Discard())
	if err != nil {
		t.Fatalf("OpenTempWork: %v", err)
	}
	if want := filepath.Join(out, TempWorkDirName); work.Root() != want {
		t.Errorf("root = %s, want %s", work.Root(), want)
	}

	const rel = "ch1/sec2/leaf.md"
	if err := work.Store().Put(rel, []byte("body\n"), []Input{corpusInput}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work.Root(), filepath.FromSlash(rel))); err != nil {
		t.Errorf("the mirror path was not created on demand: %v", err)
	}
	// The output side is NOT created as a side effect: only a delivery puts
	// anything there, and a scratch write is not one.
	if _, err := os.Stat(filepath.Join(out, "ch1")); !os.IsNotExist(err) {
		t.Errorf("a scratch write created %s in the output tree (err = %v)", "ch1", err)
	}

	if err := work.Discard(); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(work.Root()); !os.IsNotExist(err) {
		t.Errorf("%s survived Discard (err = %v)", TempWorkDirName, err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("Discard removed the output directory itself: %v", err)
	}
}

// TestOpenTempWorkRefusesNoOutputDirectory: temp-work is defined relative to
// an output directory, so there is no such thing as one without it — an empty
// path would silently root the scratch tree at ./temp-work.
func TestOpenTempWorkRefusesNoOutputDirectory(t *testing.T) {
	if _, err := OpenTempWork("  ", log.Discard()); err == nil {
		t.Fatal("temporary work with nowhere to sit must be refused")
	}
}

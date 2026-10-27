package build

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kbase/internal/asks"
	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
)

func trailDirs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, buildstore.ScratchDir, buildstore.TrailsDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestPruneTrailsKeepsTheCurrentAndTheNewestFive: of eight trail directories
// the current one, though the oldest, and the five newest others stand whole,
// captures included, and the flat answers, group records and call captures an
// earlier kbase wrote are gone.
func TestPruneTrailsKeepsTheCurrentAndTheNewestFive(t *testing.T) {
	dir := t.TempDir()
	scratch := filepath.Join(dir, buildstore.ScratchDir)
	trails := filepath.Join(scratch, buildstore.TrailsDir)
	base := time.Now().Add(-24 * time.Hour)
	names := []string{"c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7"}
	for i, name := range names {
		storetest.WriteFile(t, filepath.Join(trails, name, asks.AnswersDir, "a.json"), "x")
		storetest.WriteFile(t, filepath.Join(trails, name, asks.AskRecordsDir, "r.yaml"), "x")
		storetest.WriteFile(t, filepath.Join(trails, name, asks.CapturesDir, "c.capture.jsonl"), "x")
		when := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(filepath.Join(trails, name), when, when); err != nil {
			t.Fatal(err)
		}
	}
	storetest.WriteFile(t, filepath.Join(scratch, asks.AnswersDir, "flat.json"), "x")
	storetest.WriteFile(t, filepath.Join(scratch, asks.AskRecordsDir, "flat.yaml"), "x")
	storetest.WriteFile(t, filepath.Join(scratch, asks.CapturesDir, "flat.capture.jsonl"), "x")

	if err := pruneTrails(dir, filepath.Join(trails, "c0")); err != nil {
		t.Fatal(err)
	}
	if got, want := trailDirs(t, dir), []string{"c0", "c3", "c4", "c5", "c6", "c7"}; !slices.Equal(got, want) {
		t.Errorf("trail directories = %q, want the current and the five newest others: %q", got, want)
	}
	for _, name := range []string{"c0", "c3", "c7"} {
		for _, p := range []string{asks.AnswersDir + "/a.json", asks.AskRecordsDir + "/r.yaml", asks.CapturesDir + "/c.capture.jsonl"} {
			if _, err := os.Stat(filepath.Join(trails, name, p)); err != nil {
				t.Errorf("%s/%s did not stand whole: %v", name, p, err)
			}
		}
	}
	for _, flat := range []string{asks.AnswersDir, asks.AskRecordsDir, asks.CapturesDir} {
		if _, err := os.Stat(filepath.Join(scratch, flat)); !os.IsNotExist(err) {
			t.Errorf("the flat %s stands: %v", flat, err)
		}
	}
}

// TestPruneTrailsCreatesAMissingCurrentDirectory: a trail that recorded start
// and wrote nothing gets its directory, stamped with the prune's time.
func TestPruneTrailsCreatesAMissingCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, buildstore.ScratchDir, buildstore.TrailsDir, "c9")
	before := time.Now().Add(-time.Second)
	if err := pruneTrails(dir, current); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(current)
	if err != nil || !info.IsDir() {
		t.Fatalf("the current trail's directory did not stand: %v", err)
	}
	if info.ModTime().Before(before) {
		t.Errorf("modification time %v, want the prune's time", info.ModTime())
	}
}

package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/result"
)

// prefillProgress puts n earlier lines ahead of whatever the live record holds.
func prefillProgress(t *testing.T, state string, n int) {
	t.Helper()
	live := filepath.Join(state, buildstore.ProgressFile)
	old, err := os.ReadFile(live)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	storetest.WriteFile(t, live, strings.Repeat("{\"event\":\"earlier\"}\n", n)+string(old))
}

func rotatedProgress(t *testing.T, state string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(state, "progress-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func lineCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

// TestFreshBuildOverALongRecordRotatesIt: the earlier lines move whole to a
// rotated record, and the live record opens with this build's run event.
func TestFreshBuildOverALongRecordRotatesIt(t *testing.T) {
	f := newFixture(t)
	prefillProgress(t, f.state, progressRotateLines+1)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	rotated := rotatedProgress(t, f.state)
	if len(rotated) != 1 {
		t.Fatalf("rotated records = %q, want one", rotated)
	}
	if got := lineCount(t, rotated[0]); got != progressRotateLines+1 {
		t.Errorf("rotated record holds %d lines, want the %d earlier ones", got, progressRotateLines+1)
	}
	if b, _ := os.ReadFile(rotated[0]); strings.Contains(string(b), `"event":"run"`) {
		t.Errorf("the rotated record holds a run event of the rotating build")
	}
	live, err := os.ReadFile(filepath.Join(f.state, buildstore.ProgressFile))
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(live), "\n")
	if !strings.Contains(first, `"event":"run"`) || strings.Contains(string(live), "earlier") {
		t.Errorf("live record starts %q, want this build's run event and none of the earlier lines", first)
	}
	if !strings.Contains(string(live), `"event":"end"`) {
		t.Errorf("live record = %q, want the reopened file to take the end event", live)
	}
}

// TestFreshBuildUnderTheThresholdRotatesNothing: the live record's lines,
// run event included, at the threshold leave it whole.
func TestFreshBuildUnderTheThresholdRotatesNothing(t *testing.T) {
	f := newFixture(t)
	prefillProgress(t, f.state, progressRotateLines-1)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	if rotated := rotatedProgress(t, f.state); len(rotated) != 0 {
		t.Errorf("rotated records = %q, want none", rotated)
	}
}

// TestResumeRotatesNothing: a build whose trail already recorded start leaves
// a long record in place.
func TestResumeRotatesNothing(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	prefillProgress(t, f.state, progressRotateLines+1)
	opts.Through = ""
	if outcome, fields := f.build(t, opts); outcome != result.Done {
		t.Fatalf("resume = %s %v", outcome, fields)
	}
	if rotated := rotatedProgress(t, f.state); len(rotated) != 0 {
		t.Errorf("a resume rotated %q", rotated)
	}
}

// TestRefusedLaunchRotatesNothing: a build refused at its launch checks leaves
// the long record as it was.
func TestRefusedLaunchRotatesNothing(t *testing.T) {
	f := newFixture(t)
	prefillProgress(t, f.state, progressRotateLines+1)
	opts := f.options()
	opts.VolumeRoots = []string{f.repo}
	if outcome, fields := f.build(t, opts); outcome != result.Refused {
		t.Fatalf("build = %s %v, want refused", outcome, fields)
	}
	if rotated := rotatedProgress(t, f.state); len(rotated) != 0 {
		t.Errorf("a refused build rotated %q", rotated)
	}
}

// TestRotationKeepsTheNewestFifteen: with sixteen rotated records standing, a
// rotation leaves the newest fifteen by name.
func TestRotationKeepsTheNewestFifteen(t *testing.T) {
	f := newFixture(t)
	var names []string
	for i := range keptProgressFiles + 1 {
		name := "progress-2020010" + string(rune('a'+i)) + ".jsonl"
		names = append(names, name)
		storetest.WriteFile(t, filepath.Join(f.state, name), "x\n")
	}
	prefillProgress(t, f.state, progressRotateLines+1)
	opts := f.options()
	opts.Through = "spine-seed"
	if outcome, fields := f.build(t, opts); outcome != result.Bounded {
		t.Fatalf("build = %s %v", outcome, fields)
	}
	var got []string
	for _, p := range rotatedProgress(t, f.state) {
		got = append(got, filepath.Base(p))
	}
	if len(got) != keptProgressFiles {
		t.Fatalf("rotated records = %q, want %d", got, keptProgressFiles)
	}
	if slices.Contains(got, names[0]) || slices.Contains(got, names[1]) {
		t.Errorf("rotated records = %q, want the oldest pruned", got)
	}
}

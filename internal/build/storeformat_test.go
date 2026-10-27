package build

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kbase/internal/buildstore"
	"kbase/internal/buildstore/storetest"
	"kbase/internal/result"
)

// TestStoreAtFormat1Opens: a store with no stamp is version 1 and a build
// over it stamps it; one stamped 1 is read by status.
func TestStoreAtFormat1Opens(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.state, 0o755); err != nil {
		t.Fatal(err)
	}
	if outcome, fields := f.build(t, f.options()); outcome != result.Done {
		t.Fatalf("build over an unstamped store = %s %v", outcome, fields)
	}
	b, err := os.ReadFile(filepath.Join(f.state, buildstore.StoreFormatFile))
	if err != nil || string(b) != "1\n" {
		t.Fatalf("the store's stamp = %q, %v; want \"1\\n\"", b, err)
	}
	if outcome, fields := buildstore.Status(buildstore.MonitorOptions{StateDir: f.state, WorkDir: f.repo}); outcome != result.Done {
		t.Errorf("status over a store at format 1 = %s %v", outcome, fields)
	}
}

// TestNewerStoreRefusedByEveryVerb: build, status, cancel and the MCP build
// tool's Locate each refuse a store stamped past this kbase's format, naming
// both versions, and none of them writes into it.
func TestNewerStoreRefusedByEveryVerb(t *testing.T) {
	f := newFixture(t)
	storetest.WriteFile(t, filepath.Join(f.state, buildstore.StoreFormatFile), "2\n")
	locate := func() (string, []result.Field) {
		_, _, outcome, fields := buildstore.Locate(buildstore.MonitorOptions{StateDir: f.state, WorkDir: f.repo})
		return outcome, fields
	}
	for verb, run := range map[string]func() (string, []result.Field){
		"build": func() (string, []result.Field) { return f.build(t, f.options()) },
		"status": func() (string, []result.Field) {
			return buildstore.Status(buildstore.MonitorOptions{StateDir: f.state, WorkDir: f.repo})
		},
		"cancel": func() (string, []result.Field) {
			return Cancel(buildstore.MonitorOptions{StateDir: f.state, WorkDir: f.repo})
		},
		"mcp": locate,
	} {
		outcome, fields := run()
		items, _ := storetest.Field(fields, result.RefusalsKey).([]result.Item)
		if outcome != result.Refused || len(items) != 1 || items[0].Check != buildstore.CheckStateDir ||
			items[0].Remedy != "update kbase, or use another --state-dir" ||
			!strings.Contains(items[0].Detail, "format 2") || !strings.Contains(items[0].Detail, "format 1") {
			t.Errorf("%s over a store at format 2 = %s %v; want refused, check state-dir, both versions named", verb, outcome, fields)
		}
	}
	entries, err := os.ReadDir(f.state)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the refused store holds %d entries, want only its stamp", len(entries))
	}
}

// TestStoreUpgradeRefusesAGap: with an upgrade from 1 to 2 and none beyond,
// an upgrade to 3 stops at 2 and names it, the store stamped 2.
func TestStoreUpgradeRefusesAGap(t *testing.T) {
	dir := t.TempDir()
	steps := map[int]func(string) error{1: func(string) error { return nil }}
	err := upgradeStore(steps, dir, 1, 3)
	if err == nil || err.Error() != "state store: no upgrade leads from format 2 toward 3" {
		t.Errorf("err = %v, want the gap at 2 named", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, buildstore.StoreFormatFile)); string(b) != "2\n" {
		t.Errorf("stamp after the step that ran = %q, want \"2\\n\"", b)
	}
}

// TestPruneBuildCapturesKeepsTheNewestSix: of eight captured builds, the
// newest keptBuilds+1 pairs stand whatever the progress record holds — here
// one run event, as a rotation leaves it — and the stage reports stay.
func TestPruneBuildCapturesKeepsTheNewestSix(t *testing.T) {
	dir := t.TempDir()
	reports := filepath.Join(dir, buildstore.ReportsDir)
	p, err := openProgress(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.emit(buildstore.Event{Event: buildstore.EventRun}); err != nil {
		t.Fatal(err)
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	want := []string{"start.yaml"}
	storetest.WriteFile(t, filepath.Join(reports, "start.yaml"), "x")
	for i := range 8 {
		stem := fmt.Sprintf("%s202601%02dT000000Z-x", buildstore.CapturePrefix, i+1)
		for _, suffix := range []string{buildstore.CaptureStdoutSuffix, buildstore.CaptureStderrSuffix} {
			storetest.WriteFile(t, filepath.Join(reports, stem+suffix), "x")
			if i >= 8-(keptBuilds+1) {
				want = append(want, stem+suffix)
			}
		}
	}

	if err := pruneBuildCaptures(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(reports)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("reports kept %q, want %q", got, want)
	}
}

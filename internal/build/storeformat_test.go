package build

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kbase/internal/asks"
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

// TestPruneCapturesKeepsTheLatestBuilds: of keptBuilds+2 builds that walked,
// the captures last written before the oldest kept one's start are removed;
// later captures, the stage reports and the answer cache stay, and refused
// builds do not count.
func TestPruneCapturesKeepsTheLatestBuilds(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(i int) time.Time { return base.Add(time.Duration(i) * time.Hour) }
	clock := base
	p, err := openProgress(dir, func() time.Time { return clock })
	if err != nil {
		t.Fatal(err)
	}
	builds := keptBuilds + 2
	for i := range builds {
		for _, outcome := range []string{result.Done, result.Refused} {
			clock = at(i)
			if outcome == result.Refused {
				clock = clock.Add(30 * time.Minute)
			}
			for _, e := range []buildstore.Event{{Event: buildstore.EventRun}, {Event: buildstore.EventEnd, Outcome: outcome}} {
				if err := p.emit(e); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	captures := filepath.Join(dir, buildstore.ScratchDir, asks.CapturesDir)
	reports := filepath.Join(dir, buildstore.ReportsDir)
	answers := filepath.Join(dir, buildstore.ScratchDir, asks.AnswersDir)
	written := func(path string, when time.Time) {
		storetest.WriteFile(t, path, "x")
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	for i := range builds {
		when := at(i).Add(10 * time.Minute)
		written(filepath.Join(captures, "ask-"+string(rune('a'+i))+".capture.jsonl"), when)
		written(filepath.Join(reports, buildstore.CapturePrefix+string(rune('a'+i))+".stdout.yaml"), when)
		written(filepath.Join(answers, string(rune('a'+i))+".txt"), when)
	}
	written(filepath.Join(reports, "start.yaml"), at(0))

	if err := pruneCaptures(dir); err != nil {
		t.Fatal(err)
	}
	names := func(d string) []string {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	if got, want := names(captures), []string{"ask-c.capture.jsonl", "ask-d.capture.jsonl", "ask-e.capture.jsonl", "ask-f.capture.jsonl", "ask-g.capture.jsonl"}; !slices.Equal(got, want) {
		t.Errorf("captures kept %q, want %q", got, want)
	}
	if got, want := names(reports), []string{"build-c.stdout.yaml", "build-d.stdout.yaml", "build-e.stdout.yaml", "build-f.stdout.yaml", "build-g.stdout.yaml", "start.yaml"}; !slices.Equal(got, want) {
		t.Errorf("reports kept %q, want %q", got, want)
	}
	if got := names(answers); len(got) != builds {
		t.Errorf("answer cache holds %q, want all %d answers", got, builds)
	}
}

// TestPruneCapturesBelowTheLimitKeepsAll: fewer builds than keptBuilds remove
// nothing, and a store with no progress record is no error.
func TestPruneCapturesBelowTheLimitKeepsAll(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, buildstore.ScratchDir, asks.CapturesDir, "old.capture.jsonl")
	storetest.WriteFile(t, capture, "x")
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(capture, old, old); err != nil {
		t.Fatal(err)
	}
	if err := pruneCaptures(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(capture); errors.Is(err, os.ErrNotExist) {
		t.Error("a capture was removed with no build recorded")
	}
}

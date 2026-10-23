//go:build !kbasedist

package score

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/query"
	"kbase/internal/result"
)

// replayable is a run in dir over the first item of runSet: asked once with a
// reply the parse refuses, then again, as a resume asks a failed item, with
// "none". The item list stands beside it as the verb writes one.
func replayable(t *testing.T, dir string) *Ledger {
	t.Helper()
	src, ix, set := runSet(t)
	l := kept(t, dir)
	if err := l.WriteItems(set[:1]); err != nil {
		t.Fatal(err)
	}
	write, _ := writing()
	for _, reply := range []string{"about half", "none"} {
		ask, _ := asking(t, []asked{{title: "1| An Analysis", reply: reply}})
		if _, err := (&Runner{Source: src, Index: ix, Items: set[:1], Pass: query.PassRigor, Ask: ask, Write: write,
			Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	return l
}

// replayed is a replay of run-1 in dir, every item answered "none", and its
// result.
func replayed(t *testing.T, dir string) (*Replay, ReplayResult) {
	t.Helper()
	rp, err := OpenReplay(dir, header().Run, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	ask, _ := asking(t, []asked{{reply: "none"}})
	res, err := rp.Walk(context.Background(), ask, ReplayHeader{BackEnd: "provider", Provider: "local", Model: "small",
		Started: minute(1000)})
	if err != nil {
		t.Fatal(err)
	}
	return rp, res
}

// TestReplayAsksAnItemsLastCapture: an item asked twice is replayed from its
// last capture — the earlier one's digest is made to differ, and the replay's
// prompt matches the one it re-asked.
func TestReplayAsksAnItemsLastCapture(t *testing.T) {
	dir := t.TempDir()
	l := replayable(t, dir)
	path := filepath.Join(l.Dir(), capturesFile)
	captures := rawLines(t, path)
	if len(captures) != 2 {
		t.Fatalf("the run left %d captures, want two calls of its one item", len(captures))
	}
	var first map[string]any
	if err := json.Unmarshal(captures[0], &first); err != nil {
		t.Fatal(err)
	}
	first["prompt_sha256"] = "not the prompt's digest"
	b, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	captures[0] = b
	if err := os.WriteFile(path, append(bytes.Join(captures, []byte{'\n'}), '\n'), 0o666); err != nil {
		t.Fatal(err)
	}

	rp, res := replayed(t, dir)
	if res.Outcome != result.Done || res.Counts.Total != 1 || res.Counts.Done != 1 || res.Changed != 0 {
		t.Errorf("the replay: %+v; want its one item done as the run's last line records it", res)
	}
	compared := lines(t, filepath.Join(rp.Dir(), comparisonFile))
	if len(compared) != 1 || compared[0]["prompt_changed"] != false {
		t.Errorf("the comparison %v; want one item whose prompt matches its last capture's", compared)
	}
}

// TestReplayKeepsTheNewestReplays: a replay over sixteen standing replays of its
// run keeps itself and the newest fifteen, removing the oldest.
func TestReplayKeepsTheNewestReplays(t *testing.T) {
	dir := t.TempDir()
	l := replayable(t, dir)
	stood := series(replayKind, keptRuns, 0)
	for _, s := range stood {
		at := filepath.Join(l.Dir(), replaysDir, s.name)
		if err := os.MkdirAll(at, 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(at, replayFile), []byte("started: "+s.started+"\n"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	rp, _ := replayed(t, dir)
	if _, err := os.Stat(rp.Dir()); err != nil {
		t.Errorf("the replay removed itself: %v", err)
	}
	for i, s := range stood {
		_, err := os.Stat(filepath.Join(l.Dir(), replaysDir, s.name))
		if gone := errors.Is(err, os.ErrNotExist); gone != (i == 0) {
			t.Errorf("%s removed %v, want %v", s.name, gone, i == 0)
		}
	}
}

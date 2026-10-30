//go:build kbasedev

package score

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/buildstore"
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
		if _, err := (&Runner{Source: src, Index: ix, Items: set[:1], Pass: query.PassRigor, Launch: sequentialLaunch(ask), Write: write,
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
	rp, err := OpenReplay(dir, header().Run, "", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	ask, _ := asking(t, []asked{{reply: "none"}})
	res, err := rp.Walk(context.Background(), sequentialLaunch(ask), ReplayHeader{BackEnd: "provider", Provider: "local", Model: "small",
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
	before, _ := lines(t, filepath.Join(l.Dir(), linesFile))[0]["before"].(string)
	if before == "" || compared[0]["before"] != before {
		t.Errorf("the comparison's before = %q, want the value the run's line recorded, %q", compared[0]["before"], before)
	}
}

// TestReplaySelectsOnePass: a replay narrowed to a pass re-asks that pass's
// captures alone, a narrowing to the run's own set or to nothing takes every
// call, and a pass the run never walked is refused naming --pass and the pass
// the run did walk.
func TestReplaySelectsOnePass(t *testing.T) {
	dir := t.TempDir()
	replayable(t, dir)
	for _, c := range []struct {
		pass  string
		calls int
		refus string
	}{
		{pass: query.PassRigor, calls: 1},
		{pass: query.PassBoth, calls: 1},
		{pass: "", calls: 1},
		{pass: query.PassBearing, refus: "--pass"},
		{pass: "sideways", refus: "--pass"},
	} {
		t.Run(c.pass, func(t *testing.T) {
			rp, err := OpenReplay(dir, header().Run, c.pass, false, 0)
			if c.refus != "" {
				var ref result.Refusal
				if !errors.As(err, &ref) || ref[0].Key != c.refus {
					t.Fatalf("OpenReplay(%q) = %v, want a refusal naming %s", c.pass, err, c.refus)
				}
				if !strings.Contains(ref[0].Detail, header().Pass) {
					t.Errorf("the refusal %q does not name the pass the run walked", ref[0].Detail)
				}
				return
			}
			if err != nil {
				t.Fatalf("OpenReplay(%q): %v", c.pass, err)
			}
			if len(rp.calls) != c.calls {
				t.Errorf("the replay selected %d calls, want %d", len(rp.calls), c.calls)
			}
		})
	}
}

// TestCapturesReRenderToTheirDigest: each capture the runner wrote — of every
// class, read back as a replay reads them — renders through its class's template
// to the prompt whose digest it records, byte for byte. A torn tail is no
// capture and is not read.
func TestCapturesReRenderToTheirDigest(t *testing.T) {
	src, ix, _ := runSet(t)
	set, err := query.ScoringSet(ix, query.PassBoth)
	if err != nil {
		t.Fatal(err)
	}
	var items []query.Item
	for _, it := range set {
		if it.Node == nil || it.Node.ID != "clm-dddddd" {
			items = append(items, it)
		}
	}
	h := header()
	h.Pass = query.PassBoth
	l, err := Create(t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ask, _ := asking(t, []asked{{reply: "none"}})
	write, _ := writing()
	if _, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassBoth, Launch: sequentialLaunch(ask), Write: write,
		Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	appendTorn(t, filepath.Join(l.Dir(), capturesFile))

	captured, err := readLastCaptures(l.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) != len(items) {
		t.Fatalf("read %d captures, want one per item asked, %d", len(captured), len(items))
	}
	classes := map[string]bool{}
	for key, c := range captured {
		prompt, err := c.class.render(c.closed)
		if err != nil {
			t.Fatal(err)
		}
		if digestOf(prompt) != c.PromptSHA256 || prompt != c.Prompt {
			t.Errorf("%s: the re-rendered prompt digests %s, the capture records %s", key, digestOf(prompt), c.PromptSHA256)
		}
		classes[c.class.name] = true
	}
	for _, cl := range []class{statementClass, methodClass, bearingClass} {
		if !classes[cl.name] {
			t.Errorf("no capture of class %s was read", cl.name)
		}
	}
}

// appendTorn appends to the JSONL file at path the start of a line a killed
// process left unfinished.
func appendTorn(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"run":"run-1","pass":`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestReadLastCapturesRefusesWhatItCannotParseAgainst: a capture naming no class,
// or numbering bytes its text does not hold, is refused rather than read.
func TestReadLastCapturesRefusesWhatItCannotParseAgainst(t *testing.T) {
	for _, tc := range []struct{ name, line, want string }{
		{"a line naming no class", `{"kind":"claim","id":"clm-a","class":"verdict","text":"ab","lines":[{"no":1,"end":"title","from":0,"to":2}]}`, `no class is named "verdict"`},
		{"a line ranging past its text", `{"kind":"claim","id":"clm-a","class":"statement","text":"ab","lines":[{"no":1,"end":"title","from":1,"to":5}]}`, "names bytes 1-5 of a 2-byte text"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, capturesFile), []byte(tc.line+"\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			if _, err := readLastCaptures(dir); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("readLastCaptures = %v, want a refusal naming %q", err, tc.want)
			}
		})
	}
}

// TestResumeDropsATornCapture: a resume drops what a killed process left after
// the capture file's last newline, so the capture it appends is a whole line and
// the file reads back as captures. The first call's reply is no answer, so the
// resume asks the item again.
func TestResumeDropsATornCapture(t *testing.T) {
	dir := t.TempDir()
	src, ix, set := runSet(t)
	write, _ := writing()
	runOnce := func(l *Ledger, reply string) {
		t.Helper()
		ask, _ := asking(t, []asked{{title: "1| An Analysis", reply: reply}})
		if _, err := (&Runner{Source: src, Index: ix, Items: set[:1], Pass: query.PassRigor, Launch: sequentialLaunch(ask), Write: write,
			Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	first := kept(t, dir)
	runOnce(first, "about half")
	first.Close()
	path := filepath.Join(first.Dir(), capturesFile)
	appendTorn(t, path)

	runOnce(reopened(t, dir), "none")
	captures := rawLines(t, path)
	if len(captures) != 2 {
		t.Fatalf("captures.jsonl holds %d lines, want the run's two calls", len(captures))
	}
	for i, c := range captures {
		if !json.Valid(c) {
			t.Errorf("capture %d is no JSON line: %s", i+1, c)
		}
	}
	if _, err := readLastCaptures(first.Dir()); err != nil {
		t.Errorf("the resumed run's captures do not read back: %v", err)
	}
}

// TestReplayOfARemovedRunRecreatesNothing: a replay whose run directory was
// removed after it was opened fails before asking, and the run directory does
// not stand again.
func TestReplayOfARemovedRunRecreatesNothing(t *testing.T) {
	dir := t.TempDir()
	l := replayable(t, dir)
	rp, err := OpenReplay(dir, header().Run, "", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(l.Dir()); err != nil {
		t.Fatal(err)
	}
	ask := func(context.Context, string) (Reply, error) {
		t.Fatal("a replay of a removed run asked")
		return Reply{}, nil
	}
	if _, err := rp.Walk(context.Background(), sequentialLaunch(ask), ReplayHeader{Started: minute(1000)}); err == nil {
		t.Error("the replay of a removed run succeeded")
	}
	if _, err := os.Stat(l.Dir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the removed run's directory stands again: %v", err)
	}
}

// TestReplayFailsWhereItsDirectoryIsRemovedUnderIt: a replay directory removed
// while the walk's files are open takes the rest of the walk's lines with it,
// so the replay fails naming the directory.
func TestReplayFailsWhereItsDirectoryIsRemovedUnderIt(t *testing.T) {
	dir := t.TempDir()
	src, ix, set := runSet(t)
	l := kept(t, dir)
	if err := l.WriteItems(set[:2]); err != nil {
		t.Fatal(err)
	}
	ask, _ := asking(t, []asked{{reply: "none"}})
	write, _ := writing()
	if _, err := (&Runner{Source: src, Index: ix, Items: set[:2], Pass: query.PassRigor, Launch: sequentialLaunch(ask), Write: write,
		Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	rp, err := OpenReplay(dir, header().Run, "", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	removing := func(context.Context, string) (Reply, error) {
		if calls++; calls == 2 {
			if err := os.RemoveAll(rp.Dir()); err != nil {
				t.Fatal(err)
			}
		}
		return Reply{Text: "none"}, nil
	}
	if _, err := rp.Walk(context.Background(), sequentialLaunch(removing), ReplayHeader{Started: minute(1000)}); err == nil ||
		!strings.Contains(err.Error(), rp.Dir()) {
		t.Errorf("Walk = %v, want a failure naming %s", err, rp.Dir())
	}
}

// twoItemRun is a finished run in dir over the first two items of runSet, and a
// replay of it.
func twoItemRun(t *testing.T, dir string) *Replay {
	t.Helper()
	src, ix, set := runSet(t)
	l := kept(t, dir)
	if err := l.WriteItems(set[:2]); err != nil {
		t.Fatal(err)
	}
	ask, _ := asking(t, []asked{{reply: "none"}})
	write, _ := writing()
	if _, err := (&Runner{Source: src, Index: ix, Items: set[:2], Pass: query.PassRigor, Launch: sequentialLaunch(ask), Write: write,
		Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	rp, err := OpenReplay(dir, header().Run, "", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	return rp
}

// TestReplayAppendsEachFileOnceAfterItsAsks: while the replay asks, its ledger,
// captures and comparison files are absent; after the walk each holds one line
// per asked item, in the run's order.
func TestReplayAppendsEachFileOnceAfterItsAsks(t *testing.T) {
	rp := twoItemRun(t, t.TempDir())
	files := []string{linesFile, capturesFile, comparisonFile}
	ask := func(context.Context, string) (Reply, error) {
		for _, f := range files {
			if _, err := os.Stat(filepath.Join(rp.Dir(), f)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s stands while the replay asks (stat: %v), want it written after the walk", f, err)
			}
		}
		return Reply{Text: "none"}, nil
	}
	if _, err := rp.Walk(context.Background(), sequentialLaunch(ask), ReplayHeader{Started: minute(1000)}); err != nil {
		t.Fatal(err)
	}
	var items []string
	for _, c := range lines(t, filepath.Join(rp.Dir(), comparisonFile)) {
		items = append(items, c["item"].(string))
	}
	if len(items) != 2 || items[0] == items[1] {
		t.Fatalf("the comparison names %v, want the two items", items)
	}
	for _, f := range []string{linesFile, capturesFile} {
		if got := len(lines(t, filepath.Join(rp.Dir(), f))); got != 2 {
			t.Errorf("%s holds %d lines, want one per asked item", f, got)
		}
	}
}

// TestReplayCancelledMidWalkKeepsWhatItAsked: a cancel during the second ask
// leaves one line in each file, for the item asked before the stop.
func TestReplayCancelledMidWalkKeepsWhatItAsked(t *testing.T) {
	rp := twoItemRun(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	ask := func(context.Context, string) (Reply, error) {
		if calls++; calls == 2 {
			cancel()
			return Reply{}, ctx.Err()
		}
		return Reply{Text: "none"}, nil
	}
	res, err := rp.Walk(ctx, sequentialLaunch(ask), ReplayHeader{Started: minute(1000)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Cancelled {
		t.Errorf("outcome = %q, want cancelled", res.Outcome)
	}
	for _, f := range []string{linesFile, capturesFile, comparisonFile} {
		if got := len(lines(t, filepath.Join(rp.Dir(), f))); got != 1 {
			t.Errorf("%s holds %d lines, want one for the item asked before the stop", f, got)
		}
	}
}

// TestReplayCountsNoChangeWithoutALedgerLine: a capture whose run recorded no
// ledger line for its item — a crash between the two — compares against no
// outcome, and is not counted changed.
func TestReplayCountsNoChangeWithoutALedgerLine(t *testing.T) {
	dir := t.TempDir()
	l := replayable(t, dir)
	if err := os.Remove(filepath.Join(l.Dir(), linesFile)); err != nil {
		t.Fatal(err)
	}
	rp, res := replayed(t, dir)
	if res.Outcome != result.Done || res.Counts.Done != 1 || res.Changed != 0 {
		t.Errorf("the replay: %+v; want its one item done and none changed", res)
	}
	compared := lines(t, filepath.Join(rp.Dir(), comparisonFile))
	if len(compared) != 1 {
		t.Fatalf("the comparison holds %d lines, want one", len(compared))
	}
	if original, _ := compared[0]["original"].(map[string]any); original["status"] != "" {
		t.Errorf("the original's status = %v, want \"\": the run recorded no line", original["status"])
	}
}

// TestReplayKeepsTheNewestReplays: a replay over sixteen standing replays of its
// run keeps itself and the newest fifteen, removing the oldest.
func TestReplayKeepsTheNewestReplays(t *testing.T) {
	dir := t.TempDir()
	l := replayable(t, dir)
	stood := series(replayKind, buildstore.KeptRuns, 0)
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

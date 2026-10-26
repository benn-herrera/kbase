package score

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"kbase/internal/asks"
	"kbase/internal/kb"
	"kbase/internal/query"
	"kbase/internal/result"
)

// landed is one value a write seam was handed: the item's id and the value as
// the KB spells it.
type landed struct{ id, value string }

// asked is one scripted reply: the title its item's prompt carries, and the
// reply — cut off at the completion cap where truncated — or the error the call
// returns.
type asked struct {
	title     string
	reply     string
	truncated bool
	err       error
}

// asking is a scripted Ask: the first reply whose title the prompt carries
// answers it, and every prompt the run was handed is recorded.
func asking(t *testing.T, answers []asked) (Ask, *[]string) {
	t.Helper()
	var prompts []string
	return func(_ context.Context, prompt string) (Reply, error) {
		prompts = append(prompts, prompt)
		for _, a := range answers {
			if strings.Contains(prompt, a.title) {
				return Reply{Text: a.reply, Truncated: a.truncated}, a.err
			}
		}
		t.Fatalf("the run asked an item no reply was scripted for:\n%s", prompt)
		return Reply{}, nil
	}, &prompts
}

// writing is a Write seam recording every value it lands.
func writing() (Write, *[]landed) {
	var got []landed
	return func(it query.Item, v kb.Bearing) error {
		got = append(got, landed{itemID(it), kb.BearingText(v)})
		return nil
	}, &got
}

// itemID is an item's identity as a test names it: a node's id, or an edge's
// relation and ends.
func itemID(it query.Item) string {
	switch {
	case it.Node != nil:
		return it.Node.ID
	case it.Edge != nil:
		return it.Edge.Relation + " " + it.Edge.Source + " → " + it.Edge.Target
	}
	return ""
}

// runSet is testKB's rigor set, the index it came from, and the source both
// renderers read: seven items, so the six- and three-item sets below are
// prefixes of a real set.
func runSet(t *testing.T) (*kb.Source, *query.Index, []query.Item) {
	t.Helper()
	src := testKB(t)
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	set, err := query.ScoringSet(ix, query.PassRigor)
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 7 {
		t.Fatalf("testKB's rigor set holds %d items, want the fixture's seven", len(set))
	}
	return src, ix, set
}

// TestTheLineRecordsTheValueItReplaced: a run's line carries the value the KB
// held before its attempt — a node's rigor, as a result spells it — so the run's
// answers pair with what they replaced, and a replay can state the before.
func TestTheLineRecordsTheValueItReplaced(t *testing.T) {
	dir := t.TempDir()
	src, ix, set := runSet(t)
	l := kept(t, dir)
	ask, _ := asking(t, []asked{{reply: "none"}})
	write, _ := writing()
	if _, err := (&Runner{Source: src, Index: ix, Items: set, Pass: query.PassRigor, Ask: ask, Write: write,
		Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, rec := range lines(t, filepath.Join(l.Dir(), linesFile)) {
		id, _ := rec["id"].(string)
		before, _ := rec["before"].(string)
		got[id] = before
	}
	want := map[string]string{
		"clm-aaaaaa": "0.8", "clm-bbbbbb": "0.6", "clm-cccccc": "0.5",
		"clm-dddddd": "0.4", "clm-eeeeee": "0.3", "ana-aaaaaa": "0.7", "exp-aaaaaa": "0.9",
	}
	if len(got) != len(want) {
		t.Fatalf("the ledger holds %v, want a line for each of %v", got, want)
	}
	for id, before := range want {
		if got[id] != before {
			t.Errorf("%s recorded before %q, want the rigor it carried, %q", id, got[id], before)
		}
	}
}

// TestRunRefusesAnItemItsPassDoesNotWalk: the pass decides what an item's call
// asks and what its charges cost, so an item the pass does not walk is that
// item's failure — never rendered and priced as the other pass's subject under
// this one's name.
func TestRunRefusesAnItemItsPassDoesNotWalk(t *testing.T) {
	src, ix, _ := runSet(t)
	bearing, err := query.ScoringSet(ix, query.PassBearing)
	if err != nil {
		t.Fatal(err)
	}
	if len(bearing) == 0 {
		t.Fatal("the fixture holds no bearing item")
	}
	l := kept(t, t.TempDir())
	ask := func(context.Context, string) (Reply, error) {
		t.Fatal("a rigor run asked a model about an edge")
		return Reply{}, nil
	}
	write, landed := writing()
	res, err := (&Runner{Source: src, Index: ix, Items: bearing[:1], Pass: query.PassRigor, Ask: ask, Write: write,
		Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Failed || len(*landed) != 0 {
		t.Errorf("outcome %q, landed %v; want the item failed and nothing written", res.Outcome, *landed)
	}
	recs := lines(t, filepath.Join(l.Dir(), linesFile))
	if len(recs) != 1 || recs[0]["status"] != StatusFailed {
		t.Fatalf("the ledger holds %v, want one failed line", recs)
	}
	if reply, _ := recs[0]["reply"].(string); !strings.Contains(reply, "no rigor subject") {
		t.Errorf("the line's reply = %q, want the pass's own refusal of the item", reply)
	}
}

// TestRunReadsTheIndexBeforeTheWalkAndNotDuringIt: a run reads the index before
// its walk and composes every item from that one read. There is no seam over the
// index's derivation to count, so it is made to fail instead: the record files
// it is derived from are removed once it is loaded, and a renderer that derived
// it again would fail its item. Every item of both passes still composes and
// lands.
func TestRunReadsTheIndexBeforeTheWalkAndNotDuringIt(t *testing.T) {
	src, ix, _ := runSet(t)
	for _, rel := range []string{kb.GraphNodesFile, kb.GraphEdgesFile} {
		if err := os.Remove(src.KBPath(rel)); err != nil {
			t.Fatal(err)
		}
	}
	rigor, err := query.ScoringSet(ix, query.PassRigor)
	if err != nil {
		t.Fatal(err)
	}
	bearing, err := query.ScoringSet(ix, query.PassBearing)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's clm-dddddd is refused by the guard, which is another test's
	// case: this one needs items that reach a call.
	var items []query.Item
	for _, it := range rigor {
		if it.Node == nil || it.Node.ID != "clm-dddddd" {
			items = append(items, it)
		}
	}
	write, landed := writing()

	l := kept(t, t.TempDir())
	ask, prompts := asking(t, []asked{{title: "markdown", reply: "none"}})
	res, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: ask, Write: write,
		Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Done || res.Counts.Done != len(items) {
		t.Errorf("the rigor run: outcome %q, counts %+v; want every item done with the index files gone", res.Outcome, res.Counts)
	}
	if len(*prompts) != len(items) || len(*landed) != len(items) {
		t.Errorf("the rigor run made %d calls and landed %d, want the %d items", len(*prompts), len(*landed), len(items))
	}

	bh := header()
	bh.Run, bh.Pass = "run-bearing", query.PassBearing
	bl, err := Create(t.TempDir(), bh)
	if err != nil {
		t.Fatal(err)
	}
	defer bl.Close()
	bask, bprompts := asking(t, []asked{{title: "## Source:", reply: "none"}})
	if _, err := (&Runner{Source: src, Index: ix, Items: bearing, Pass: query.PassBearing, Ask: bask, Write: write,
		Ledger: bl, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*bprompts) != len(bearing) {
		t.Errorf("the bearing run made %d calls, want %d with the index files gone", len(*bprompts), len(bearing))
	}
}

// TestReplySummaryIsBoundedAndPlain: a failed line's reply as a document or a log
// line states it — control characters dropped, a line break read as a space, and
// a long reason cut to its bound on a character boundary.
func TestReplySummaryIsBoundedAndPlain(t *testing.T) {
	long := strings.Repeat("a", replySummaryLimit+100)
	for _, tc := range []struct {
		name, reply, want string
	}{
		{"plain", "the write was refused", "the write was refused"},
		{"an escape and a newline", "refused\n\x1b[2J done", "refused [2J done"},
		{"a terminal bell and a carriage return", "a\ab\rc", "ab c"},
		{"a bidirectional override", "a\u202eb", "ab"},
		{"nothing but control", "\x1b\x07", ""},
		{"over the bound", long, strings.Repeat("a", replySummaryLimit) + "…"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReplySummary(tc.reply); got != tc.want {
				t.Errorf("ReplySummary(%q) = %q, want %q", tc.reply, got, tc.want)
			}
		})
	}
	// A cut lands on a character boundary, so the summary is valid UTF-8 and is
	// never half a multi-byte character.
	got := ReplySummary(strings.Repeat("é", replySummaryLimit))
	if !utf8.ValidString(got) || len(got) > replySummaryLimit+len("…") {
		t.Errorf("a cut over multi-byte text = %q (%d bytes), want a valid, bounded summary", got, len(got))
	}
}

// TestRunWalksTheSet: a scripted sequence of good, malformed, failed and good
// replies is counted by outcome — a reply the parse refuses fails its attempt —
// the two good values reach the write seam, and a limit stops the walk at the
// item boundary with what it left counted as remaining.
func TestRunWalksTheSet(t *testing.T) {
	src, ix, set := runSet(t)
	for _, tc := range []struct {
		name   string
		items  []query.Item
		limit  int
		answer []asked
		want   Counts
		landed []landed
	}{
		{
			// Six of the fixture's seven items, walked four deep: the two the
			// limit leaves are the ones counted remaining.
			name:  "a charge, a malformed reply, a failed call and a charge",
			items: set[:6],
			limit: 4,
			answer: []asked{
				{title: "1| An Analysis", reply: "none"},
				{title: "1| Claim A", reply: "0.5, roughly — the paragraph is thin."},
				{title: "1| Claim B", err: errors.New("the provider closed the connection")},
				{title: "1| Claim C", reply: "flt-overreach: 2\n"},
			},
			want:   Counts{Total: 6, Done: 2, Failed: 2, Remaining: 2},
			landed: []landed{{"ana-aaaaaa", "1.0"}, {"clm-cccccc", "0.85"}},
		},
		{
			name:   "a limit of one",
			items:  set[:3],
			limit:  1,
			answer: []asked{{title: "1| An Analysis", reply: "none"}},
			want:   Counts{Total: 3, Done: 1, Remaining: 2},
			landed: []landed{{"ana-aaaaaa", "1.0"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := kept(t, t.TempDir())
			ask, prompts := asking(t, tc.answer)
			write, landed := writing()
			res, err := (&Runner{Source: src, Index: ix, Items: tc.items, Pass: query.PassRigor, Ask: ask, Write: write,
				Ledger: l, Model: "small", Limit: tc.limit}).Run(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != result.Bounded {
				t.Errorf("outcome = %q, want %q", res.Outcome, result.Bounded)
			}
			if res.Counts != tc.want {
				t.Errorf("counts = %+v, want %+v", res.Counts, tc.want)
			}
			if len(*prompts) != len(tc.answer) {
				t.Errorf("the run made %d calls, want %d: a limit stops the walk", len(*prompts), len(tc.answer))
			}
			if !slices.Equal(*landed, tc.landed) {
				t.Errorf("the write seam was handed %v, want %v", *landed, tc.landed)
			}
			var written []string
			for _, it := range res.Written {
				written = append(written, itemID(it))
			}
			if want := ids(tc.landed); !slices.Equal(written, want) {
				t.Errorf("written = %v, want %v", written, want)
			}
			// The walk this run covered is the cut, reported rather than left
			// for the caller to cut again: the closing record names items of
			// this slice.
			wantWalked := tc.items
			if tc.limit > 0 && tc.limit < len(wantWalked) {
				wantWalked = wantWalked[:tc.limit]
			}
			if got := itemIDs(res.Walked); !slices.Equal(got, itemIDs(wantWalked)) {
				t.Errorf("walked = %v, want %v", got, itemIDs(wantWalked))
			}
		})
	}
}

// itemIDs is a slice of items' identities as a test names them.
func itemIDs(items []query.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = itemID(it)
	}
	return out
}

// ids is the item ids of what landed.
func ids(landed []landed) []string {
	out := make([]string, len(landed))
	for i, l := range landed {
		out[i] = l.id
	}
	return out
}

// TestRunRecordsARefusedTextWithoutACall: a statement carrying a scoring token
// is refused by the guard, recorded failed with the document and line it came
// from, and no call is made.
func TestRunRecordsARefusedTextWithoutACall(t *testing.T) {
	src, ix, _ := runSet(t)
	l := kept(t, t.TempDir())
	ask := func(context.Context, string) (Reply, error) {
		t.Fatal("a refused text made a call")
		return Reply{}, nil
	}
	write, landed := writing()
	items := []query.Item{{Node: &query.ScoringNode{Kind: kb.NodeKindClaim, ID: "clm-dddddd"}}}
	res, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: ask, Write: write, Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Failed || res.Counts != (Counts{Total: 1, Failed: 1}) {
		t.Errorf("outcome %q, counts %+v; want %q and one failed item", res.Outcome, res.Counts, result.Failed)
	}
	if len(*landed) != 0 {
		t.Errorf("a refused text landed %v", *landed)
	}
	recs := lines(t, filepath.Join(l.Dir(), linesFile))
	if len(recs) != 1 || recs[0]["status"] != StatusFailed {
		t.Fatalf("the ledger holds %v, want one failed line", recs)
	}
	reply, _ := recs[0]["reply"].(string)
	if !strings.Contains(reply, src.KBPath("vol/a.md")+":") || !strings.Contains(reply, "- rigor:") {
		t.Errorf("the line's reply = %q, want the refused document, line and token", reply)
	}
	if _, withPrompt := recs[0]["prompt_sha256"]; withPrompt {
		t.Errorf("a refused text recorded a prompt digest: %v", recs[0])
	}
}

// TestRunRecordsAWriteRefusal: a write the KB refuses lands nothing, is
// recorded failed with the write's reason, and ends the run failed.
func TestRunRecordsAWriteRefusal(t *testing.T) {
	src, ix, set := runSet(t)
	l := kept(t, t.TempDir())
	ask, _ := asking(t, []asked{{title: "1| An Analysis", reply: "none"}})
	refuse := errors.New("the register changed under the write")
	write := func(query.Item, kb.Bearing) error { return refuse }
	res, err := (&Runner{Source: src, Index: ix, Items: set[:1], Pass: query.PassRigor, Ask: ask, Write: write, Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Failed || len(res.Written) != 0 {
		t.Errorf("outcome %q, written %v; want %q and nothing written", res.Outcome, res.Written, result.Failed)
	}
	recs := lines(t, filepath.Join(l.Dir(), linesFile))
	if len(recs) != 1 || recs[0]["status"] != StatusFailed || recs[0]["reply"] != refuse.Error() {
		t.Errorf("the ledger holds %v, want one failed line carrying the write's reason", recs)
	}
	if _, withValue := recs[0]["value"]; withValue {
		t.Errorf("a refused write recorded a value: %v", recs[0])
	}
}

// TestRunFailsATruncatedReply: a reply cut off at the completion cap fails its
// item even where the text it reached parses as a clean answer — the lines it
// did not reach are charges the list is missing. Nothing lands, the line records
// the truncation as its reason, and the capture keeps no reply.
func TestRunFailsATruncatedReply(t *testing.T) {
	src, ix, set := runSet(t)
	l := kept(t, t.TempDir())
	ask, _ := asking(t, []asked{{title: "1| An Analysis", reply: "none", truncated: true}})
	write, landed := writing()
	res, err := (&Runner{Source: src, Index: ix, Items: set[:1], Pass: query.PassRigor, Ask: ask, Write: write, Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Failed || res.Counts != (Counts{Total: 1, Failed: 1}) || len(*landed) != 0 {
		t.Errorf("outcome %q, counts %+v, landed %v; want %q, one failed item and nothing written",
			res.Outcome, res.Counts, *landed, result.Failed)
	}
	recs := lines(t, filepath.Join(l.Dir(), linesFile))
	if len(recs) != 1 || recs[0]["status"] != StatusFailed || recs[0]["reply"] != errTruncatedReply.Error() {
		t.Errorf("the ledger holds %v, want one failed line carrying the truncation", recs)
	}
	caps := lines(t, filepath.Join(l.Dir(), capturesFile))
	if len(caps) != 1 {
		t.Fatalf("the run left %d captures, want one", len(caps))
	}
	if caps[0]["reply"] != "" || caps[0]["error"] != errTruncatedReply.Error() {
		t.Errorf("the capture's reply %q, error %q; want no reply and the truncation as its error", caps[0]["reply"], caps[0]["error"])
	}
}

// TestRunStopsAtAnItemBoundaryOnCancel: a context cancelled during a call stops
// the run before the next ask, records nothing for the item in flight, and
// reports the ledger as it stands.
func TestRunStopsAtAnItemBoundaryOnCancel(t *testing.T) {
	src, ix, set := runSet(t)
	set = set[:3]
	l := kept(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	ask := func(_ context.Context, _ string) (Reply, error) {
		calls++
		if calls == 2 {
			cancel()
			return Reply{}, context.Canceled
		}
		return Reply{Text: "none"}, nil
	}
	write, landed := writing()
	res, err := (&Runner{Source: src, Index: ix, Items: set, Pass: query.PassRigor, Ask: ask, Write: write, Ledger: l, Model: "small"}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Cancelled {
		t.Errorf("outcome = %q, want %q", res.Outcome, result.Cancelled)
	}
	if want := (Counts{Total: 3, Done: 1, Remaining: 2}); res.Counts != want {
		t.Errorf("counts = %+v, want %+v", res.Counts, want)
	}
	if calls != 2 {
		t.Errorf("the run made %d calls, want 2: a cancelled context stops before the next ask", calls)
	}
	if len(*landed) != 1 {
		t.Errorf("the write seam was handed %v, want the one item that finished", *landed)
	}
	if got := l.Status(set[1]); got != "" {
		t.Errorf("the item in flight recorded %q, want nothing", got)
	}
	if recs := lines(t, filepath.Join(l.Dir(), linesFile)); len(recs) != 1 {
		t.Errorf("the ledger holds %d lines, want the one item that finished", len(recs))
	}
}

// TestRunContinuesARun: a ledger recording an item as landed is the whole run's
// record, not this invocation's: a second runner over the same ledger asks
// nothing it records done, and reports the whole run's counts.
func TestRunContinuesARun(t *testing.T) {
	src, ix, set := runSet(t)
	set = set[:3]
	l := kept(t, t.TempDir())
	first, _ := asking(t, []asked{{title: "1| An Analysis", reply: "none"}})
	w, _ := writing()
	res, err := (&Runner{Source: src, Index: ix, Items: set, Pass: query.PassRigor, Ask: first, Write: w, Ledger: l, Model: "small", Limit: 1}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts != (Counts{Total: 3, Done: 1, Remaining: 2}) {
		t.Fatalf("the first run's counts = %+v, want one done and two remaining", res.Counts)
	}

	// The second run is scripted for the two items the ledger leaves; asking the
	// one it records done would fail here.
	second, prompts := asking(t, []asked{
		{title: "1| Claim A", reply: "flt-overreach: 2"},
		{title: "1| Claim B", reply: "flt-underspecified: 2"},
	})
	w2, secondLanded := writing()
	res, err = (&Runner{Source: src, Index: ix, Items: set, Pass: query.PassRigor, Ask: second, Write: w2, Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Done || res.Counts != (Counts{Total: 3, Done: 3}) {
		t.Errorf("outcome %q, counts %+v; want %q and every item done", res.Outcome, res.Counts, result.Done)
	}
	if len(*prompts) != 2 {
		t.Errorf("the second run made %d calls, want the two the ledger leaves", len(*prompts))
	}
	if want := []landed{{"clm-aaaaaa", "0.85"}, {"clm-bbbbbb", "0.85"}}; !slices.Equal(*secondLanded, want) {
		t.Errorf("the write seam was handed %v, want %v", *secondLanded, want)
	}
}

// TestRunSkipsManuallyScoredValues: over a KB marking clm-aaaaaa and the lifts
// edge into it manually scored, a both-pass walk records each skipped — status
// skipped, reason manually-scored, no value, charges, call or capture — unless
// --rescore-manual names its pass; a skip is counted apart from done and failed
// and outcome stays done. A continued walk over the same ledger appends no
// second skipped line, and one whose rescore names the pass asks the item.
func TestRunSkipsManuallyScoredValues(t *testing.T) {
	src := editedTestKB(t, func(g *kb.Graph) {
		g.Claims[0].ManuallyScored = true
		g.Scorable[0].ManuallyScored = true
	})
	ix, err := query.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	both, err := query.ScoringSet(ix, query.PassBoth)
	if err != nil {
		t.Fatal(err)
	}
	// clm-dddddd is refused by the guard, which is another test's case.
	var items []query.Item
	for _, it := range both {
		if itemID(it) != "clm-dddddd" {
			items = append(items, it)
		}
	}
	node, lift := "clm-aaaaaa", kb.RelationLifts+" ana-aaaaaa → clm-aaaaaa"
	for _, tc := range []struct {
		rescore string
		skipped []string
	}{
		{RescoreNone, []string{node, lift}},
		{query.PassRigor, []string{lift}},
		{query.PassBearing, []string{node}},
		{query.PassBoth, nil},
	} {
		t.Run(tc.rescore, func(t *testing.T) {
			dir := t.TempDir()
			l := kept(t, dir)
			ask, prompts := asking(t, []asked{{title: "", reply: "none"}})
			write, landed := writing()
			runner := &Runner{Source: src, Index: ix, Items: items, Pass: query.PassBoth, Ask: ask, Write: write,
				Skip: ManualSkip(ix, tc.rescore), Ledger: l, Model: "small"}
			res, err := runner.Run(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			n := len(tc.skipped)
			want := Counts{Total: len(items), Done: len(items) - n, Skipped: n}
			if res.Outcome != result.Done || res.Counts != want {
				t.Errorf("outcome %q, counts %+v; want %q and %+v", res.Outcome, res.Counts, result.Done, want)
			}
			if len(*prompts) != len(items)-n || len(*landed) != len(items)-n {
				t.Errorf("the run made %d calls and landed %d, want %d of each", len(*prompts), len(*landed), len(items)-n)
			}
			for _, it := range res.Written {
				if slices.Contains(tc.skipped, itemID(it)) {
					t.Errorf("a skipped item was written: %s", itemID(it))
				}
			}
			var got []string
			for _, rec := range lines(t, filepath.Join(l.Dir(), linesFile)) {
				if rec["status"] != StatusSkipped {
					continue
				}
				got = append(got, recordedItem(rec))
				if rec["reply"] != ReasonManuallyScored {
					t.Errorf("a skipped line's reason = %v, want %s", rec["reply"], ReasonManuallyScored)
				}
				for _, key := range []string{"value", "faults", "moot", "model", "prompt_sha256"} {
					if _, ok := rec[key]; ok {
						t.Errorf("a skipped line carries %s: %v", key, rec)
					}
				}
			}
			if !slices.Equal(got, tc.skipped) {
				t.Errorf("the skipped lines name %v, want %v", got, tc.skipped)
			}
			if n == 0 {
				return
			}
			for _, rec := range lines(t, filepath.Join(l.Dir(), capturesFile)) {
				if slices.Contains(tc.skipped, recordedItem(rec)) {
					t.Errorf("a skipped item left a capture: %v", rec)
				}
			}
			if st, err := ReadRun(dir, l.run); err != nil || st.Counts.Skipped != n {
				t.Errorf("ReadRun counts %+v (%v), want %d skipped", st.Counts, err, n)
			}

			// The same walk again appends no second skipped line.
			before := len(lines(t, filepath.Join(l.Dir(), linesFile)))
			if _, err := runner.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if after := len(lines(t, filepath.Join(l.Dir(), linesFile))); after != before {
				t.Errorf("a continued walk appended %d lines over items it had skipped", after-before)
			}
			// A walk told to re-judge both asks every skipped item.
			again, prompted := asking(t, []asked{{title: "", reply: "none"}})
			runner.Ask, runner.Skip = again, ManualSkip(ix, query.PassBoth)
			res, err = runner.Run(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(*prompted) != n || res.Counts != (Counts{Total: len(items), Done: len(items)}) {
				t.Errorf("the re-judging walk made %d calls, counts %+v; want %d calls and every item done", len(*prompted), res.Counts, n)
			}
		})
	}
}

// recordedItem is a ledger or capture line's item as a test names it.
func recordedItem(rec map[string]any) string {
	if id, ok := rec["id"].(string); ok {
		return id
	}
	return fmt.Sprintf("%v %v → %v", rec["relation"], rec["source"], rec["target"])
}

// TestRunDigestsTheVariantEachItemReceived: a claim and an analysis are asked
// different instructions, so each landed line's digest is the instruction that
// item received — which is what answers "which variant judged this item" — and
// the two differ.
func TestRunDigestsTheVariantEachItemReceived(t *testing.T) {
	src, ix, set := runSet(t)
	var items []query.Item
	for _, it := range set {
		if it.Node != nil && (it.Node.ID == "clm-aaaaaa" || it.Node.ID == "ana-aaaaaa") {
			items = append(items, it)
		}
	}
	if len(items) != 2 {
		t.Fatalf("the fixture's set names %d of the claim and the analysis, want both", len(items))
	}
	l := kept(t, t.TempDir())
	ask, _ := asking(t, []asked{{title: "markdown", reply: "none"}})
	write, _ := writing()
	if _, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: ask, Write: write,
		Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, rec := range lines(t, filepath.Join(l.Dir(), linesFile)) {
		if id, ok := rec["id"].(string); ok {
			byID[id], _ = rec["prompt_sha256"].(string)
		}
	}
	g := graphOf(ix)
	for _, c := range []struct {
		id  string
		ask func(string) (string, error)
	}{
		{"clm-aaaaaa", asks.ScoreRigorStatement},
		{"ana-aaaaaa", asks.ScoreRigorMethod},
	} {
		text, err := rigorText(src, g, c.id)
		if err != nil {
			t.Fatal(err)
		}
		prompt, err := c.ask(text.numbered())
		if err != nil {
			t.Fatal(err)
		}
		if got := byID[c.id]; got != digestOf(prompt) {
			t.Errorf("%s's line digests %q, and the instruction it was sent digests %q", c.id, got, digestOf(prompt))
		}
	}
	if byID["clm-aaaaaa"] == byID["ana-aaaaaa"] {
		t.Error("the claim and the analysis record one digest: the line would not name the variant that judged it")
	}
}

// TestRunRecordsChargesTheirIDsPrice: every landed line carries the charges the
// reply recorded, and its value is its class's table applied to their ids alone —
// for both passes. A resume then re-derives each value from the ids without
// asking a model, which is what makes a landing checkable and a run re-scorable.
func TestRunRecordsChargesTheirIDsPrice(t *testing.T) {
	src, ix, _ := runSet(t)
	rigorSet, err := query.ScoringSet(ix, query.PassRigor)
	if err != nil {
		t.Fatal(err)
	}
	bearingSet, err := query.ScoringSet(ix, query.PassBearing)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's clm-dddddd quotes a register's scoring line, so the guard
	// refuses it before any call — another test's case. This one walks the items
	// a call reaches.
	var items []query.Item
	for _, it := range rigorSet {
		if it.Node == nil || it.Node.ID != "clm-dddddd" {
			items = append(items, it)
		}
	}
	write, _ := writing()

	state := t.TempDir()
	l := kept(t, state)
	rigorAsk, _ := asking(t, []asked{
		{title: "1| Claim A", reply: "flt-overreach: 2"},
		{title: "1| Claim B", reply: "flt-underspecified: 2\nflt-underspecified: 4"},
		{title: "1| Claim C", reply: "none"},
		{title: "1| Claim D", reply: "none"},
		{title: "1| Bearing 0.5 In Words", reply: "none"},
		{title: "1| An Analysis", reply: "none"},
		{title: "1| An Experiment", reply: "none"},
	})
	if _, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: rigorAsk, Write: write, Ledger: l, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertDerivable(t, l, rigorTables)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	// The bearing pass is a ledger of its own: a run records one pass, and its
	// header names that pass's ask.
	bstate := t.TempDir()
	bh := header()
	bh.Run, bh.Pass = "run-bearing", query.PassBearing
	bl, err := Create(bstate, bh)
	if err != nil {
		t.Fatal(err)
	}
	defer bl.Close()
	bearingAsk, _ := asking(t, []asked{
		{title: "1| A Cited Work", reply: "flt-transfer: 1"},
		{title: "1| The design rule", reply: "none"},
		{title: "1| An Analysis", reply: "none"},
	})
	if _, err := (&Runner{Source: src, Index: ix, Items: bearingSet, Pass: query.PassBearing, Ask: bearingAsk, Write: write, Ledger: bl, Model: "small"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertDerivable(t, bl, bearingTables)

	// A resume re-derives every landed value and asks nothing: the ask seam
	// fails the test on any call, and every item is recorded done.
	back := reopened(t, state)
	defer back.Close()
	assertDerivable(t, back, rigorTables)
	inert := func(context.Context, string) (Reply, error) {
		t.Fatal("a resume asked a model")
		return Reply{}, nil
	}
	res, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: inert, Write: write, Ledger: back, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Done || res.Counts.Done != len(items) {
		t.Errorf("the resumed run: outcome %q, counts %+v; want every item done and no call", res.Outcome, res.Counts)
	}
}

// TestRunLandsALineCitationAndReAsksAnAbsentNumber: a run over two fixture items
// lands the value whose charge names a line the call shows — recording the number
// and the line the tool quotes — and fails the one whose charge names a line it
// does not, recording the case and no charge; the resume asks that item again and
// lands it.
func TestRunLandsALineCitationAndReAsksAnAbsentNumber(t *testing.T) {
	src, ix, set := runSet(t)
	var items []query.Item
	for _, it := range set {
		if it.Node != nil && (it.Node.ID == "clm-aaaaaa" || it.Node.ID == "clm-bbbbbb") {
			items = append(items, it)
		}
	}
	if len(items) != 2 {
		t.Fatalf("the fixture's set names %d of the two claims, want both", len(items))
	}
	state := t.TempDir()
	l := kept(t, state)
	ask, _ := asking(t, []asked{
		{title: "1| Claim A", reply: "flt-overreach: 2"},
		{title: "1| Claim B", reply: "flt-overreach: 9"},
	})
	write, wrote := writing()
	res, err := (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: ask, Write: write,
		Ledger: l, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Failed || res.Counts != (Counts{Total: 2, Done: 1, Failed: 1}) {
		t.Errorf("outcome %q, counts %+v; want one item landed and one failed", res.Outcome, res.Counts)
	}
	if want := []landed{{"clm-aaaaaa", "0.85"}}; !slices.Equal(*wrote, want) {
		t.Errorf("the write seam was handed %v, want %v", *wrote, want)
	}
	byID := map[string]map[string]any{}
	for _, rec := range lines(t, filepath.Join(l.Dir(), linesFile)) {
		byID[rec["id"].(string)] = rec
	}
	faults, ok := byID["clm-aaaaaa"]["faults"].([]any)
	if !ok || len(faults) != 1 {
		t.Fatalf("the landed line records %v, want one charge", byID["clm-aaaaaa"]["faults"])
	}
	charge := faults[0].(map[string]any)
	if charge["lines"] != "2" || charge["passage"] != passageStatement {
		t.Errorf("the charge is %v, want line 2 of the statement", charge)
	}
	quote, _ := charge["citation"].(string)
	if !strings.HasPrefix(quote, "The claim's own paragraph, stated mid-section.") {
		t.Errorf("the charge quotes %q, want the line's own words", quote)
	}
	if _, withQuote := byID["clm-bbbbbb"]["faults"]; withQuote {
		t.Errorf("the failed line carries charges: %v", byID["clm-bbbbbb"])
	}
	reason, _ := byID["clm-bbbbbb"]["reply"].(string)
	if !strings.Contains(reason, "line 9 is not one the call shows") {
		t.Errorf("the failed line's reason = %q, want the absent number named", reason)
	}

	// The resume asks only the item the ledger records failed, and lands it.
	back := resumed(t, state, write)
	again, prompts := asking(t, []asked{{title: "1| Claim B", reply: "flt-underspecified: 2"}})
	res, err = (&Runner{Source: src, Index: ix, Items: items, Pass: query.PassRigor, Ask: again, Write: write,
		Ledger: back, Model: "small"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != result.Done || res.Counts != (Counts{Total: 2, Done: 2}) {
		t.Errorf("the resumed run: outcome %q, counts %+v; want both items done", res.Outcome, res.Counts)
	}
	if len(*prompts) != 1 || !strings.Contains((*prompts)[0], "1| Claim B") {
		t.Errorf("the resume made %d calls, want the one item the ledger left", len(*prompts))
	}
}

// rigorTables is the class mapping as the scheme states it — a claim's statement
// table, an analysis's and an experiment's method table — and bearingTables is the
// one table a scoring edge takes. The tests state the mapping here rather than
// reading the derivation they check.
var (
	rigorTables = map[string]kb.FaultTable{
		kb.NodeKindClaim:      kb.RigorStatement,
		kb.NodeKindAnalysis:   kb.RigorMethod,
		kb.NodeKindExperiment: kb.RigorMethod,
	}
	bearingTables = map[string]kb.FaultTable{"": kb.BearingFaults}
)

// assertDerivable is what a run's record owes: every landed line's value is its
// class's table applied to the ids its line records, and its charge list is
// present — empty only where the model answered `none`.
func assertDerivable(t *testing.T, l *Ledger, tableOf map[string]kb.FaultTable) {
	t.Helper()
	recs := lines(t, filepath.Join(l.Dir(), linesFile))
	done := 0
	for _, rec := range recs {
		if rec["status"] != StatusDone {
			continue
		}
		done++
		kind, _ := rec["kind"].(string)
		table, ok := tableOf[kind]
		if !ok {
			t.Errorf("the line %v is of kind %q, which the test holds no table for", rec["id"], kind)
			continue
		}
		raw, ok := rec["faults"].([]any)
		if !ok {
			t.Errorf("a landed line records no charges: %v", rec)
			continue
		}
		ids := make([]string, 0, len(raw))
		for _, r := range raw {
			charge, ok := r.(map[string]any)
			if !ok {
				t.Fatalf("a charge is not a record: %v", r)
			}
			ids = append(ids, charge["id"].(string))
		}
		if want := kb.BearingText(table.Price(ids)); rec["value"] != want {
			t.Errorf("the line %v lands %v, and its ids %v price %s", rec["id"], rec["value"], ids, want)
		}
	}
	if done == 0 {
		t.Fatal("the run landed nothing to re-derive")
	}
}

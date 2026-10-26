package score

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/log"
	"kbase/internal/query"
	"kbase/internal/result"
	"kbase/internal/write"
)

// invokeKB is a refreshed KB an invocation walks whole: two claims, an analysis
// and an experiment for the rigor set; two supports edges, a lifts and a refutes
// for the bearing set; and a work and an axiom founding a node, which neither
// set holds. Every scoring bearing is pending. It returns the KB's root. It is
// cmd's testdata/score-kb in this package's own form: a package test reads
// only its own package's testdata.
func invokeKB(t *testing.T) string {
	t.Helper()
	pending := kb.Bearing{Pending: true}
	src := refreshedKB(t, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n- [ana](ana.md)\n- [exp](exp.md)\n",
		"vol/a.md": "[↑ Vol](index.md)\n\n# The First Result\n\n" +
			"> **Theorem 1.** Every exchange conserves stock. <!-- claim-rigor: clm-aaaaaa -->\n\n" +
			"> **Theorem 2.** Every exchange clears at the posted price. <!-- claim-rigor: clm-bbbbbb -->\n",
		"vol/ana.md": "[↑ Vol](index.md)\n\n# A Supporting Argument\n<!-- claim-rigor: ana-aaaaaa -->\n\n" +
			"An argument from clearing to conservation, carrying the trial's result.\n",
		"vol/exp.md": "[↑ Vol](index.md)\n\n# A Trial Run\n<!-- claim-rigor: exp-aaaaaa -->\n\n" +
			"A trial of the exchange against a posted price series, run once.\n",
		"axioms.md": "# Axioms\n\n### Axiom S1: Conservation Of Stock\n\nStock is conserved across every exchange, whatever the venue.\n",
	}, kb.Graph{
		Claims: []kb.ClaimNode{
			{ID: "clm-aaaaaa", Title: "The First Result", Rigor: rigor(0.6), Rationale: "Read off the leaf's own statement."},
			{ID: "clm-bbbbbb", Title: "The Second Result", Rigor: rigor(0.55), Rationale: "Clears where the first result holds."},
		},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "A Supporting Argument", Rigor: rigor(0.65), Rationale: "Carries the trial."}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "A Trial Run", Status: "run", Rigor: rigor(0.8)}},
		Works:       []kb.WorkNode{{ID: "wrk-aaaaaa", Title: "Smith, A. 2020. *Markets and Clearing*", Rigor: rigor(0.9), Rationale: "Fixture Press."}},
		Axioms:      []kb.AxiomNode{{ID: "axm-S1", Title: "Conservation Of Stock"}},
		Founds: []kb.BearingEdge{
			{Source: "axm-S1", Target: "ana-aaaaaa", Context: "a conservation assumption", Bearing: pending},
			{Source: "wrk-aaaaaa", Target: "clm-bbbbbb", Context: "Theorem 3", Bearing: pending},
		},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Bearing: pending},
			{Relation: kb.RelationSupports, Source: "exp-aaaaaa", Target: "clm-bbbbbb", Bearing: pending},
			{Relation: kb.RelationLifts, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: pending},
			{Relation: kb.RelationRefutes, Source: "ana-aaaaaa", Target: "clm-aaaaaa", Bearing: pending},
		},
	})
	return src.KBPath("")
}

// oneClaimKB is a refreshed KB whose scoring set is one claim, authored at
// rigor 0.5. It returns the KB's root.
func oneClaimKB(t *testing.T) string {
	t.Helper()
	src := refreshedKB(t, map[string]string{
		"vol/index.md": "[↑ KB](../entry-point.md)\n\n# Vol\n\n- [a](a.md)\n",
		"vol/a.md":     "[↑ Vol](index.md)\n\n# A\n\nThe claim's own paragraph. <!-- claim-rigor: clm-aaaaaa -->\n",
	}, kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A Claim", Rigor: rigor(0.5), Rationale: "Shown."}}})
	return src.KBPath("")
}

// The orders invokeKB's sets are walked in: the rigor set by id, the bearing set
// by relation, then source, then target.
var (
	invokeRigorItems   = []string{"ana-aaaaaa", "clm-aaaaaa", "clm-bbbbbb", "exp-aaaaaa"}
	invokeBearingItems = []string{"supports clm-aaaaaa → clm-bbbbbb", "supports exp-aaaaaa → clm-bbbbbb",
		"lifts ana-aaaaaa → clm-bbbbbb", "refutes ana-aaaaaa → clm-aaaaaa"}
	invokeFounds = []query.ScoringEdge{
		{Source: "axm-S1", Target: "ana-aaaaaa", Relation: kb.RelationFounds},
		{Source: "wrk-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationFounds},
	}
)

// invocation is one invocation over the KB at root with its records under
// state, as a caller hands it over: the KB refreshed and opened, a run id
// RunID minted, and the scoring model answering through newAsk.
func invocation(t *testing.T, root, state, pass string, dryRun bool, newAsk func() (Ask, error)) Invocation {
	t.Helper()
	if _, err := index.Refresh(kb.OnDisk(root), log.Discard()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	run, err := RunID(state, "", dryRun)
	if err != nil {
		t.Fatal(err)
	}
	inv := Invocation{KBRoot: root, Source: kb.OnDisk(root), StateDir: state, Run: run, Pass: pass,
		Rescore: RescoreNone, DryRun: dryRun, Log: log.Discard(), NewAsk: newAsk}
	if !dryRun {
		inv.BackEnd, inv.Provider, inv.Model = "provider", "solo", "scoring-model"
	}
	return inv
}

// resumeOf is inv continuing the run rep reported, over the KB as it now stands.
func resumeOf(t *testing.T, inv Invocation, rep Report, newAsk func() (Ask, error)) Invocation {
	t.Helper()
	next := invocation(t, inv.KBRoot, inv.StateDir, inv.Pass, false, newAsk)
	next.Run, next.Resume = rep.Run, true
	return next
}

// replying is a NewAsk whose Ask answers each call with the next of texts, in
// order; calls counts them, and a call past the last fails the test.
func replying(t *testing.T, texts ...string) (newAsk func() (Ask, error), calls *int) {
	t.Helper()
	n := 0
	return func() (Ask, error) {
		return func(context.Context, string) (Reply, error) {
			if n >= len(texts) {
				t.Fatalf("the run asked %d times, and %d replies were scripted", n+1, len(texts))
			}
			n++
			return Reply{Text: texts[n-1]}, nil
		}, nil
	}, &n
}

// askingNothing is a NewAsk whose Ask fails the test on any call.
func askingNothing(t *testing.T) func() (Ask, error) {
	t.Helper()
	return func() (Ask, error) {
		return func(context.Context, string) (Reply, error) {
			t.Error("the run asked the model")
			return Reply{}, errors.New("no call was expected")
		}, nil
	}
}

// invoked is Invoke's report, the test failing on its error.
func invoked(t *testing.T, inv Invocation) Report {
	t.Helper()
	rep, err := Invoke(context.Background(), inv)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	return rep
}

// rigorAt is the rigor the KB at root holds for the node id, as a reader derives
// it.
func rigorAt(t *testing.T, root, id string) any {
	t.Helper()
	ix, err := query.Load(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := query.ShowPayload(ix, id)
	if !ok {
		t.Fatalf("the KB holds no node %s", id)
	}
	for _, f := range rec {
		if f.Key == "rigor" {
			return f.Value
		}
	}
	return nil
}

// bearingAt is the bearing the KB at root holds for e, as the column spells it:
// the value, or the pending literal.
func bearingAt(t *testing.T, root string, e query.ScoringEdge) string {
	t.Helper()
	ix, err := query.Load(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	row, ok := ix.Edge(e)
	if !ok {
		t.Fatalf("the KB holds no %s edge %s → %s", e.Relation, e.Source, e.Target)
	}
	text, _ := row.Bearing.(string)
	return text
}

// fieldOf is one field of every line of a run's JSONL file, in file order.
func fieldOf(t *testing.T, path, key string) []string {
	t.Helper()
	var out []string
	for _, rec := range lines(t, path) {
		v, _ := rec[key].(string)
		out = append(out, v)
	}
	return out
}

// TestInvokeLandsEachPassThroughTheWriteAPI: a rigor invocation asks one item per
// claim, analysis and experiment and a bearing invocation one per supports,
// lifts and refutes edge, each landing its reply's value in the KB; a work's own
// rigor and a founds edge's bearing are no item and stay as authored. The run's
// records stand beside each other, every line names the model the invocation
// was given, and none names a founds edge.
func TestInvokeLandsEachPassThroughTheWriteAPI(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()

	newAsk, calls := replying(t, "none", "flt-overreach: 2", "flt-underspecified: 2", "none")
	rep := invoked(t, invocation(t, root, state, query.PassRigor, false, newAsk))
	if rep.Outcome != result.Done || !rep.Asked || *calls != 4 {
		t.Fatalf("the rigor run: outcome %s, asked %v, %d calls", rep.Outcome, rep.Asked, *calls)
	}
	if rep.Counts != (Counts{Total: 4, Done: 4}) {
		t.Errorf("counts %+v, want the four items done", rep.Counts)
	}
	if !slices.Equal(rep.Scored, invokeRigorItems) {
		t.Errorf("scored %q, want the four nodes in id order", rep.Scored)
	}
	for _, tc := range []struct {
		id   string
		want float64
	}{{"ana-aaaaaa", 1.00}, {"clm-aaaaaa", 0.85}, {"clm-bbbbbb", 0.85}, {"exp-aaaaaa", 1.00}} {
		if got := rigorAt(t, root, tc.id); got != tc.want {
			t.Errorf("%s's rigor = %v, want the reply's %v", tc.id, got, tc.want)
		}
	}
	if got := rigorAt(t, root, "wrk-aaaaaa"); got != 0.9 {
		t.Errorf("the work's rigor = %v, want the authored 0.9", got)
	}

	newAsk, calls = replying(t, "flt-transfer: 1", "none", "none", "none")
	rep = invoked(t, invocation(t, root, state, query.PassBearing, false, newAsk))
	if rep.Outcome != result.Done || *calls != 4 || rep.Counts != (Counts{Total: 4, Done: 4}) {
		t.Fatalf("the bearing run: outcome %s, %d calls, counts %+v; want the four edges done", rep.Outcome, *calls, rep.Counts)
	}
	for _, tc := range []struct {
		edge query.ScoringEdge
		want string
	}{
		{query.ScoringEdge{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationSupports}, "0.9"},
		{query.ScoringEdge{Source: "exp-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationSupports}, "1.0"},
		{query.ScoringEdge{Source: "ana-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationLifts}, "1.0"},
		{query.ScoringEdge{Source: "ana-aaaaaa", Target: "clm-aaaaaa", Relation: kb.RelationRefutes}, "1.0"},
	} {
		if got := bearingAt(t, root, tc.edge); got != tc.want {
			t.Errorf("the %s edge %s → %s bears %q, want the reply's %q", tc.edge.Relation, tc.edge.Source, tc.edge.Target, got, tc.want)
		}
	}
	for _, e := range invokeFounds {
		if got := bearingAt(t, root, e); got != kb.PendingLiteral {
			t.Errorf("the %s edge %s → %s bears %q, want the authored %s: no pass asks it", e.Relation, e.Source, e.Target, got, kb.PendingLiteral)
		}
	}
	for _, name := range []string{runFile, itemsFile, linesFile} {
		if _, err := os.Stat(filepath.Join(rep.Ledger, name)); err != nil {
			t.Errorf("the run left no %s: %v", name, err)
		}
	}
	for _, model := range fieldOf(t, filepath.Join(rep.Ledger, linesFile), "model") {
		if model != "scoring-model" {
			t.Errorf("a line names the model %q, want the invocation's scoring-model", model)
		}
	}
	if ledger, err := os.ReadFile(filepath.Join(rep.Ledger, linesFile)); err != nil || strings.Contains(string(ledger), kb.RelationFounds) {
		t.Errorf("the ledger names a founds edge, which no pass asks (%v):\n%s", err, ledger)
	}
}

// TestInvokeResumeOfAFinishedRunAsksNothing: continuing a run whose every item is
// done asks nothing and lands nothing, reports the whole run done over its own
// ledger, and leaves nothing unfinished.
func TestInvokeResumeOfAFinishedRunAsksNothing(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	newAsk, _ := replying(t, "flt-transfer: 1", "none", "none", "none")
	inv := invocation(t, root, state, query.PassBearing, false, newAsk)
	first := invoked(t, inv)
	if first.Outcome != result.Done {
		t.Fatalf("the run: outcome %s", first.Outcome)
	}

	again := invoked(t, resumeOf(t, inv, first, askingNothing(t)))
	if again.Outcome != result.Done || again.Run != first.Run || again.Ledger != first.Ledger {
		t.Fatalf("the resumed run: outcome %s, run %s at %s; want done over %s at %s",
			again.Outcome, again.Run, again.Ledger, first.Run, first.Ledger)
	}
	if again.Counts.Done != 4 || again.Counts.Failed != 0 {
		t.Errorf("the resumed run: counts %+v, want the four items already done", again.Counts)
	}
	if len(again.Scored) != 0 || again.Unfinished {
		t.Errorf("the resumed run landed %q, unfinished %v; want nothing landed and nothing to continue", again.Scored, again.Unfinished)
	}
}

// TestInvokeFailsAMalformedReplyAndGoesOn: a reply the parse refuses lands
// nothing for its item and fails it, and the walk goes on: the other three land,
// and the report names the failed item with the reason its line recorded.
func TestInvokeFailsAMalformedReplyAndGoesOn(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	newAsk, _ := replying(t, "none", "about half", "none", "none")
	rep := invoked(t, invocation(t, root, state, query.PassRigor, false, newAsk))
	if rep.Outcome != result.Failed {
		t.Fatalf("the run with a malformed reply: outcome %s", rep.Outcome)
	}
	if rep.Counts != (Counts{Total: 4, Done: 3, Failed: 1}) {
		t.Errorf("counts %+v, want three done and one failed", rep.Counts)
	}
	if slices.Contains(rep.Scored, "clm-aaaaaa") {
		t.Errorf("the refused item's value landed anyway: %q", rep.Scored)
	}
	if got := rigorAt(t, root, "clm-aaaaaa"); got != 0.6 {
		t.Errorf("the refused item's rigor = %v, want the authored 0.6", got)
	}
	if len(rep.Failures) != 1 || rep.Failures[0].Check != checkError {
		t.Fatalf("failures %+v, want the one refused item", rep.Failures)
	}
	if detail := rep.Failures[0].Detail; !strings.HasPrefix(detail, "clm-aaaaaa: ") || !strings.Contains(detail, "not a charge line") {
		t.Errorf("the failure %q, want the item and the reason its line recorded", detail)
	}
}

// TestInvokeWalksBothSetsInOneRun: an invocation over both sets walks the node
// set and then the edge set in one run — one run.yaml, one items.jsonl and one
// ledger.jsonl — landing a node's value and an edge's, the header naming the
// mixed set and each line naming its own item's pass.
func TestInvokeWalksBothSetsInOneRun(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	newAsk, _ := replying(t, "none", "flt-overreach: 2", "flt-underspecified: 2", "none",
		"flt-transfer: 1", "none", "none", "none")
	rep := invoked(t, invocation(t, root, state, query.PassBoth, false, newAsk))
	if rep.Outcome != result.Done || rep.Counts != (Counts{Total: 8, Done: 8}) || rep.Unfinished {
		t.Fatalf("the run: outcome %s, counts %+v, unfinished %v; want the eight items of both sets done",
			rep.Outcome, rep.Counts, rep.Unfinished)
	}
	if want := slices.Concat(invokeRigorItems, invokeBearingItems); !slices.Equal(rep.Scored, want) {
		t.Errorf("scored %q, want the node set and then the edge set %q", rep.Scored, want)
	}
	for _, e := range invokeFounds {
		if got := bearingAt(t, root, e); got != kb.PendingLiteral {
			t.Errorf("the %s edge %s → %s bears %q, want the authored %s: neither pass asks it", e.Relation, e.Source, e.Target, got, kb.PendingLiteral)
		}
	}
	if v := rigorAt(t, root, "clm-aaaaaa"); v != 0.85 {
		t.Errorf("the claim's rigor = %v, want the reply's 0.85", v)
	}
	if v := bearingAt(t, root, query.ScoringEdge{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationSupports}); v != "0.9" {
		t.Errorf("the supports edge clm-aaaaaa → clm-bbbbbb bears %q, want the reply's 0.9", v)
	}
	header, err := os.ReadFile(filepath.Join(rep.Ledger, runFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pass: " + query.PassBoth, "ask_sha256: "} {
		if !strings.Contains(string(header), key) {
			t.Errorf("run.yaml does not carry %q:\n%s", key, header)
		}
	}
	passes := slices.Concat(slices.Repeat([]string{query.PassRigor}, 4), slices.Repeat([]string{query.PassBearing}, 4))
	for _, name := range []string{linesFile, itemsFile} {
		if got := fieldOf(t, filepath.Join(rep.Ledger, name), "pass"); !slices.Equal(got, passes) {
			t.Errorf("%s passes %q, want each item's own %q", name, got, passes)
		}
	}
}

// TestInvokeDryRunAsksNothing: a dry run over both sets opens no Ask, lands
// nothing and leaves nothing to continue; it lists both sets in its item list
// with every item remaining, records the mixed set and no back end in its
// header, and writes no ledger line.
func TestInvokeDryRunAsksNothing(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	inv := invocation(t, root, state, query.PassBoth, true, func() (Ask, error) {
		t.Error("a dry run opened its Ask")
		return nil, errors.New("no Ask was expected")
	})
	rep := invoked(t, inv)
	if rep.Outcome != result.Done || rep.Asked || rep.Unfinished || rep.Wrote {
		t.Fatalf("the dry run: outcome %s, asked %v, unfinished %v, wrote %v; want done with nothing asked, left or written",
			rep.Outcome, rep.Asked, rep.Unfinished, rep.Wrote)
	}
	if rep.Counts != (Counts{Total: 8, Remaining: 8}) || len(rep.Scored) != 0 {
		t.Errorf("counts %+v, scored %q; want both sets listed and none asked", rep.Counts, rep.Scored)
	}
	if v := rigorAt(t, root, "clm-aaaaaa"); v != 0.6 {
		t.Errorf("a dry run changed the claim's rigor to %v", v)
	}
	passes := slices.Concat(slices.Repeat([]string{query.PassRigor}, 4), slices.Repeat([]string{query.PassBearing}, 4))
	if got := fieldOf(t, filepath.Join(rep.Ledger, itemsFile), "pass"); !slices.Equal(got, passes) {
		t.Errorf("the dry run's item list %q, want both sets %q", got, passes)
	}
	header, err := os.ReadFile(filepath.Join(rep.Ledger, runFile))
	if err != nil {
		t.Fatalf("read the dry run's header: %v", err)
	}
	if !strings.Contains(string(header), "pass: "+query.PassBoth) || strings.Contains(string(header), "backend") {
		t.Errorf("the dry run's header, want the mixed set and no back end:\n%s", header)
	}
	if _, err := os.Stat(filepath.Join(rep.Ledger, linesFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a dry run wrote a ledger line (%v): it asked nothing", err)
	}
}

// TestInvokeResumesAMixedRun: a run bounded inside the node set leaves the rest
// unfinished, and continuing it walks the mixed set its header names to the end;
// a resume naming one pass against that header is refused naming --resume before
// the walk asks anything.
func TestInvokeResumesAMixedRun(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	newAsk, _ := replying(t, "none", "none", "none", "none", "none")
	inv := invocation(t, root, state, query.PassBoth, false, newAsk)
	inv.Limit = 5
	bounded := invoked(t, inv)
	if bounded.Outcome != result.Bounded || bounded.Counts != (Counts{Total: 8, Done: 5, Remaining: 3}) || !bounded.Unfinished {
		t.Fatalf("the bounded run: outcome %s, counts %+v, unfinished %v; want five of the eight done and the rest left",
			bounded.Outcome, bounded.Counts, bounded.Unfinished)
	}

	newAsk, _ = replying(t, "none", "none", "none")
	again := invoked(t, resumeOf(t, inv, bounded, newAsk))
	if again.Outcome != result.Done || again.Counts != (Counts{Total: 8, Done: 8}) {
		t.Fatalf("the resumed run: outcome %s, counts %+v; want the whole mixed set done", again.Outcome, again.Counts)
	}

	rigorOnly := resumeOf(t, inv, bounded, askingNothing(t))
	rigorOnly.Pass = query.PassRigor
	_, err := Invoke(context.Background(), rigorOnly)
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Key != "--resume" {
		t.Errorf("a rigor resume over a mixed run = %v, want one refusal naming --resume", err)
	}
}

// TestInvokeResumeRepricesUnderAMovedTable: a resume after the software changed
// continues rather than refusing. The value the running table moves is
// re-derived from the ledger's ids and lands through the write a landing uses,
// so the KB's value moves and the report says the invocation wrote; the run's
// directory records from what to what; the header keeps the ask the run's own
// judgements answered; and nothing asks the model.
func TestInvokeResumeRepricesUnderAMovedTable(t *testing.T) {
	root, state := oneClaimKB(t), t.TempDir()
	newAsk, _ := replying(t, "flt-overreach: 2")
	inv := invocation(t, root, state, query.PassRigor, false, newAsk)
	first := invoked(t, inv)
	if first.Outcome != result.Done || !slices.Equal(first.Scored, []string{"clm-aaaaaa"}) {
		t.Fatalf("the run: outcome %s, scored %q; want the one claim landed", first.Outcome, first.Scored)
	}
	if got := rigorAt(t, root, "clm-aaaaaa"); got != 0.85 {
		t.Fatalf("the claim's rigor = %v, want the reply's 0.85", got)
	}

	// What a run written under the earlier table and ask left behind: the KB
	// holds the value that table landed, the line records it, and the header
	// names the instruction the run's judgements answered. The value is the
	// machine's, as a landing writes it, so the resume re-prices it.
	if res := write.Run("set-rigor", write.Options{KBRoot: root, Values: []byte(`{"entry": [{"id": "clm-aaaaaa", "rigor": 0.3}]}`),
		NoRefresh: true, Provenance: write.ProvenanceMachine, Logger: log.Discard()}); res.Outcome != result.Done {
		t.Fatalf("set-rigor: outcome %s, refusals %v, failures %v", res.Outcome, res.Refusals, res.Failures)
	}
	if got := rigorAt(t, root, "clm-aaaaaa"); got != 0.3 {
		t.Fatalf("the claim's rigor = %v, want the value the earlier table landed", got)
	}
	ledger := filepath.Join(first.Ledger, linesFile)
	rewriteTheLine(t, ledger, func(rec map[string]any) { rec["value"] = "0.30" })
	header := moveTheAsk(t, filepath.Join(first.Ledger, runFile))

	again := invoked(t, resumeOf(t, inv, first, askingNothing(t)))
	if again.Outcome != result.Done || again.Run != first.Run || !again.Wrote {
		t.Fatalf("the resumed run: outcome %s, run %s, wrote %v; want done over %s, having written", again.Outcome, again.Run, again.Wrote, first.Run)
	}
	if got := rigorAt(t, root, "clm-aaaaaa"); got != 0.85 {
		t.Errorf("the claim's rigor = %v, want the running table's 0.85: the moved value landed", got)
	}
	if recs := lines(t, ledger); len(recs) != 1 || recs[0]["value"] != "0.85" {
		t.Errorf("the ledger holds %v, want its one line valued as the table prices its ids, 0.85", recs)
	}
	record, err := os.ReadFile(filepath.Join(first.Ledger, repricedFile))
	if err != nil {
		t.Fatalf("the re-price left no record: %v", err)
	}
	// The record names the item as the report does — one spelling for both, so a
	// repriced.yaml line joins to the run's scored without a translation.
	for _, want := range []string{"item: " + first.Scored[0], `from: "0.30"`, `to: "0.85"`} {
		if !strings.Contains(string(record), want) {
			t.Errorf("the record does not carry %q:\n%s", want, record)
		}
	}
	if got, err := os.ReadFile(filepath.Join(first.Ledger, runFile)); err != nil || string(got) != header {
		t.Errorf("the resume rewrote run.yaml:\n%s\n---\n%s", header, got)
	}
}

// rewriteTheLine rewrites a one-line ledger's line through f, which is how a test
// states what a run under an earlier table recorded.
func rewriteTheLine(t *testing.T, path string, f func(map[string]any)) {
	t.Helper()
	recs := lines(t, path)
	if len(recs) != 1 {
		t.Fatalf("%s holds %d lines, want the one this rewrite states", path, len(recs))
	}
	f(recs[0])
	line, err := json.Marshal(recs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o666); err != nil {
		t.Fatal(err)
	}
}

// moveTheAsk replaces the digest run.yaml records with another, which is the
// header an update that moved the ask leaves behind, and returns the file's
// bytes as it wrote them.
func moveTheAsk(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if strings.HasPrefix(line, "ask_sha256: ") {
			line = "ask_sha256: " + strings.Repeat("0", 64)
		}
		out = append(out, line)
	}
	written := strings.Join(out, "\n") + "\n"
	if err := os.WriteFile(path, []byte(written), 0o666); err != nil {
		t.Fatal(err)
	}
	return written
}

// TestInvokeNamesTheItemACancelStoppedBefore: a run whose context is cancelled
// during its second call stops at that item boundary, and its report names the
// walk's first item with no attempt — the one it stopped before — with the one
// item answered landed and the rest remaining.
func TestInvokeNamesTheItemACancelStoppedBefore(t *testing.T) {
	root, state := invokeKB(t), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	inv := invocation(t, root, state, query.PassRigor, false, func() (Ask, error) {
		return func(context.Context, string) (Reply, error) {
			calls++
			if calls == 2 {
				cancel()
				return Reply{}, context.Canceled
			}
			return Reply{Text: "none"}, nil
		}, nil
	})
	rep, err := Invoke(ctx, inv)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if rep.Outcome != result.Cancelled || !rep.Unfinished {
		t.Fatalf("the cancelled run: outcome %s, unfinished %v", rep.Outcome, rep.Unfinished)
	}
	if want := (result.Record{{Key: "item", Value: "clm-aaaaaa"}}); !slices.Equal(rep.Stopped, want) {
		t.Errorf("stopped %v, want the claim the run stopped before", rep.Stopped)
	}
	if rep.Counts != (Counts{Total: 4, Done: 1, Remaining: 3}) {
		t.Errorf("counts %+v, want the one item answered and the three remaining", rep.Counts)
	}
	if !slices.Equal(rep.Scored, []string{"ana-aaaaaa"}) {
		t.Errorf("scored %q, want the one item that landed", rep.Scored)
	}
}

// TestInvokeFailsAnItemItCannotLand: an item whose value the KB's records cannot
// take fails, and the report names it with the reason its line recorded; the
// KB keeps its value.
func TestInvokeFailsAnItemItCannotLand(t *testing.T) {
	root, state := oneClaimKB(t), t.TempDir()
	newAsk, _ := replying(t, "none")
	inv := invocation(t, root, state, query.PassRigor, false, newAsk)
	// The records' directory refuses every new file, which is where a landing
	// writes.
	dir := filepath.Join(root, kb.IndexDir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	rep := invoked(t, inv)
	if rep.Outcome != result.Failed || len(rep.Failures) != 1 {
		t.Fatalf("the unwritable KB: outcome %s, failures %+v; want one failure naming the claim", rep.Outcome, rep.Failures)
	}
	if f := rep.Failures[0]; f.Check != checkError || !strings.HasPrefix(f.Detail, "clm-aaaaaa: ") || len(f.Detail) == len("clm-aaaaaa: ") {
		t.Errorf("failure %+v, want check %s, the item's name and the reason its line recorded", f, checkError)
	}
	if v := rigorAt(t, root, "clm-aaaaaa"); v != 0.5 {
		t.Errorf("the claim's rigor = %v, want the authored 0.5: nothing landed", v)
	}
}

// TestInvokeRefusesAnUnsavedConversion: a source read in an older metadata
// format, converted only in memory, is refused before its set is read, naming
// the index check and kbase refresh, which saves the conversion. The KB is a
// reduced copy of cmd's testdata/score-kb-1.0.0, for the reason invokeKB is.
func TestInvokeRefusesAnUnsavedConversion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo", kb.KBDir)
	for rel, text := range map[string]string{
		kb.EntryPointFile: "---\nkind: entry-point\nsubtree-claims: [clm-aaaaaa]\nkb-format: \"1.0.0\"\n---\n\n# KB\n\n- [a](a.md)\n",
		"a.md": "---\nkind: leaf\nclaims: [clm-aaaaaa]\n---\n[↑ KB](entry-point.md)\n\n\n# A Flow Lemma\n\n" +
			"> **Lemma 1.** Every flow conserves stock. <!-- claim-quality: clm-aaaaaa -->\n",
		"claim-quality.md": "# Claim Quality Register\n\n---\n\n## A Flow Lemma\n<!-- id: clm-aaaaaa -->\n\n" +
			"> **Leaf references:** *(none)*\n\n### Quality\n- confidence: 0.80\n- rationale: Read off the leaf's own statement.\n\n" +
			"### Solidity\n- solidity: 0.80\n",
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src, err := kbload.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if !src.Migrated() {
		t.Fatalf("the KB at %s read as current; the case needs one converted in memory", src.Version())
	}
	state := t.TempDir()
	run, err := RunID(state, "", true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Invoke(context.Background(), Invocation{KBRoot: root, Source: src, StateDir: state, Run: run,
		Pass: query.PassRigor, Rescore: RescoreNone, DryRun: true, Log: log.Discard()})
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) == 0 || refusal[0].Check != index.CheckIndex || refusal[0].Remedy != index.RefreshRemedy {
		t.Errorf("Invoke over an unsaved conversion = %v, want the index refusal naming %s", err, index.RefreshRemedy)
	}
}

// standRuns stands one run per entry of runs in the store at state, each
// header naming its start, and returns their ids.
func standRuns(t *testing.T, state string, runs []standing) []string {
	t.Helper()
	ids := make([]string, len(runs))
	for i, r := range runs {
		h := header()
		h.Run, h.Started = r.name, r.started
		l, err := Create(state, h)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		ids[i] = r.name
	}
	return ids
}

// standingRuns is the run ids standing in the store at state, sorted.
func standingRuns(t *testing.T, state string) []string {
	t.Helper()
	entries, err := os.ReadDir(RunsDir(state))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	slices.Sort(out)
	return out
}

// TestInvokeDryRunPrunesDryRunsAlone: a dry run runs under a dry- id and keeps
// the newest runs of its own kind, so over a full store of dry runs and another
// of older real ones it removes the oldest dry run and no real run.
func TestInvokeDryRunPrunesDryRunsAlone(t *testing.T) {
	root, state := oneClaimKB(t), t.TempDir()
	reals := standRuns(t, state, series(RunKindReal, KeptRuns, 0))
	dry := standRuns(t, state, series(RunKindDry, KeptRuns, 100))
	inv := invocation(t, root, state, query.PassRigor, true, nil)
	if !strings.HasPrefix(inv.Run, RunKindDry+"-") {
		t.Fatalf("RunID minted %q for a dry run, want a %s- id", inv.Run, RunKindDry)
	}
	if rep := invoked(t, inv); rep.Outcome != result.Done {
		t.Fatalf("the dry run: outcome %s", rep.Outcome)
	}
	want := slices.Concat(dry[1:], []string{inv.Run}, reals)
	slices.Sort(want)
	if have := standingRuns(t, state); !slices.Equal(have, want) {
		t.Errorf("the store holds %q, want every real run, the dry run and the newest dry runs before it: %q", have, want)
	}
}

// TestInvokeRefusesAResumeOfAPrunedRun: a real run over a full store removes the
// oldest; continuing that run is refused as a run with no run.yaml is, naming
// --resume, and prunes nothing.
func TestInvokeRefusesAResumeOfAPrunedRun(t *testing.T) {
	root, state := oneClaimKB(t), t.TempDir()
	stood := standRuns(t, state, series(RunKindReal, KeptRuns, 0))
	newAsk, _ := replying(t, "none")
	inv := invocation(t, root, state, query.PassRigor, false, newAsk)
	if rep := invoked(t, inv); rep.Outcome != result.Done {
		t.Fatalf("the run: outcome %s", rep.Outcome)
	}
	want := append(slices.Clone(stood[1:]), inv.Run)
	slices.Sort(want)
	if have := standingRuns(t, state); !slices.Equal(have, want) {
		t.Fatalf("the store holds %q, want the run and the newest before it: %q", have, want)
	}

	pruned := resumeOf(t, inv, Report{Run: stood[0]}, askingNothing(t))
	_, err := Invoke(context.Background(), pruned)
	var refusal result.Refusal
	if !errors.As(err, &refusal) || len(refusal) != 1 || refusal[0].Check != checkUsage || refusal[0].Key != "--resume" ||
		refusal[0].Detail != "no run stands here: run.yaml is missing" {
		t.Errorf("a resume of the pruned %s = %v, want the missing run.yaml's refusal", stood[0], err)
	}
	if have := standingRuns(t, state); !slices.Equal(have, want) {
		t.Errorf("the refused resume left %q standing, want the runs it found: %q", have, want)
	}
}

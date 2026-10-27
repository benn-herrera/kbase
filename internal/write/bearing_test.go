package write

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/query"
	"kbase/internal/result"
)

// TestSetBearingOnEveryRelation lands a bearing on a claim premise, a founds
// edge, a lift and a refutation, on a claim's own refutation and on an
// experiment's support, each edge written by the op that authors it, and reads
// each back. The founds case is the user's only path to that value: no score
// pass asks it.
func TestSetBearingOnEveryRelation(t *testing.T) {
	root := seedKB(t)
	unrelated := insertClaim(t, root, "Unrelated Result", "")
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n    bearing: 0.5\n")
	if res := run(t, root, "add-pairs", "entry:\n- id: "+claim+"\n  refutes:\n  - id: "+unrelated+"\n"); res.Outcome != result.Done {
		t.Fatalf("refute the unrelated claim: %+v", res)
	}
	insert := func(op, values string) string {
		t.Helper()
		res := run(t, root, op, values)
		if res.Outcome != result.Done || len(res.IDs) != 1 {
			t.Fatalf("%s: %+v", op, res)
		}
		return res.IDs[0]
	}
	work := insert("insert-work-entry", "entry:\n- key: Smith2020\n  title: Smith (2020)\n  rigor: 0.8\n  rationale: Cited.\n")
	if res := run(t, root, "add-rests-on", "entry:\n- id: "+claim+"\n  rests-on:\n  - id: "+work+"\n    bearing: 0.8\n"); res.Outcome != result.Done {
		t.Fatalf("rest on the work: %+v", res)
	}
	analysis := insert("insert-analysis-entry", "entry:\n- title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n  lifts:\n  - id: "+claim+
		"\n    bearing: 0.5\n  refutes:\n  - id: "+seedClaim+"\n    bearing: 0.4\n")
	experiment := insert("insert-experiment-entry", "entry:\n- document: leaf.md\n  title: A Trial\n  status: run\n  rigor: 0.9\n  supports:\n  - id: "+
		claim+"\n    bearing: 0.6\n")

	for _, w := range []scoreWrite{
		{"claim premise", "set-bearing", seedClaim, claim, kb.RelationSupports, "0.25", true},
		{"founds", "set-bearing", work, claim, kb.RelationFounds, "0.3", false},
		{"lift", "set-bearing", analysis, claim, kb.RelationLifts, kb.PendingLiteral, true},
		{"refute", "set-bearing", analysis, seedClaim, kb.RelationRefutes, "0.0", true},
		{"claim refutation", "set-bearing", claim, unrelated, kb.RelationRefutes, "0.6", true},
		{"experiment support", "set-bearing", experiment, claim, kb.RelationSupports, "0.75", true},
	} {
		t.Run(w.name, func(t *testing.T) {
			if res := run(t, root, w.op, w.values()); res.Outcome != result.Done {
				t.Fatalf("set-bearing: %+v", res)
			}
			if got, manual := scored(t, records(t, root), w.op, w.id, w.target, w.relation); got != w.value || manual != w.flagged {
				t.Errorf("record carries %s, manually-scored %v; want %s, %v", got, manual, w.value, w.flagged)
			}
			before := snapshot(t, root)
			if again := run(t, root, w.op, w.values()); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("again: %+v, want unchanged byte-identical", again)
			}
		})
	}
}

// TestAPairNamesAnAnalysis: a pair list takes a claim or an analysis as its
// target, as BearingLegal does, so an analysis's lift of another analysis and
// a claim's refutation of one land as edges into it; a work id is no pair
// target.
func TestAPairNamesAnAnalysis(t *testing.T) {
	root := seedKB(t)
	insertAnalysis := func(title, lifts string) string {
		t.Helper()
		res := run(t, root, "insert-analysis-entry", "entry:\n- title: "+title+"\n  rigor: 0.9\n  rationale: Lifts.\n"+lifts)
		if res.Outcome != result.Done || len(res.IDs) != 1 {
			t.Fatalf("insert %s: %+v", title, res)
		}
		return res.IDs[0]
	}
	first := insertAnalysis("First Analysis", "  lifts:\n  - id: "+seedClaim+"\n    bearing: 0.5\n")
	second := insertAnalysis("Second Analysis", "  lifts:\n  - id: "+first+"\n    bearing: 0.25\n")
	refuter := insertClaim(t, root, "A Refuting Result", "  refutes:\n  - id: "+first+"\n    bearing: 0.4\n")
	g := records(t, root)
	for _, want := range []kb.ScorableEdge{
		{Relation: kb.RelationLifts, Source: second, Target: first, Bearing: kb.Bearing{Value: 0.25}, ManuallyScored: true},
		{Relation: kb.RelationRefutes, Source: refuter, Target: first, Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true},
	} {
		if !slices.Contains(g.Scorable, want) {
			t.Errorf("no record %+v among %+v", want, g.Scorable)
		}
	}

	before := snapshot(t, root)
	res := run(t, root, "insert-analysis-entry", "entry:\n- title: A Third Analysis\n  rigor: 0.9\n"+
		"  rationale: Lifts a work.\n  lifts:\n  - id: wrk-smith2020\n    bearing: 0.5\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "lifts[1].id" || res.Refusals[0].Entry != 1 {
		t.Errorf("a work id as a pair target: %+v, want one refusal at entry 1 keyed on the pair's id", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused insert-analysis-entry wrote")
	}
}

// TestSetBearingRefusals: an illegal pairing, a relation outside the four, an
// edge no record carries and a value outside the domain are each refused with
// nothing written.
func TestSetBearingRefusals(t *testing.T) {
	root := seedKB(t)
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n    bearing: 0.5\n")
	res := run(t, root, "insert-analysis-entry", "entry:\n- title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n")
	if res.Outcome != result.Done {
		t.Fatalf("insert analysis: %+v", res)
	}
	analysis := res.IDs[0]
	before := snapshot(t, root)
	for _, tc := range []struct {
		name, values, detail string
	}{
		{"invariant token", "entry:\n- id: INVARIANT-S1\n  target: " + claim + "\n  relation: supports\n  bearing: 0.5\n", "not an id"},
		{"illegal pairing", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: founds\n  bearing: 0.5\n", "no bearing judges that pairing"},
		{"unknown relation", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: depends\n  bearing: 0.5\n", "outside the closed relation vocabulary"},
		{"out of domain", "entry:\n- id: " + seedClaim + "\n  target: " + claim + "\n  relation: supports\n  bearing: 1.5\n", "outside [0, 1]"},
		{"no such edge", "entry:\n- id: " + analysis + "\n  target: " + claim + "\n  relation: lifts\n  bearing: 0.5\n", "never creates one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, root, "set-bearing", tc.values)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || !strings.Contains(res.Refusals[0].Detail, tc.detail) {
				t.Errorf("got %+v, want one refusal naming %q", res, tc.detail)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused set-bearing wrote")
			}
		})
	}
}

// TestASetBearingRefusalNamesEveryOffendingEntry: a batch whose entries 1 and
// 3 name no edge record is refused with one item for each, naming its entry,
// and the legal entry between them is not written.
func TestASetBearingRefusalNamesEveryOffendingEntry(t *testing.T) {
	root := seedKB(t)
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n    bearing: 0.5\n")
	res := run(t, root, "insert-analysis-entry", "entry:\n- title: An Analysis\n  rigor: 0.9\n  rationale: Lifts.\n")
	if res.Outcome != result.Done {
		t.Fatalf("insert analysis: %+v", res)
	}
	analysis := res.IDs[0]
	before := snapshot(t, root)
	res = run(t, root, "set-bearing", "entry:\n"+
		"- id: "+analysis+"\n  target: "+claim+"\n  relation: lifts\n  bearing: 0.5\n"+
		"- id: "+seedClaim+"\n  target: "+claim+"\n  relation: supports\n  bearing: 0.6\n"+
		"- id: "+analysis+"\n  target: "+seedClaim+"\n  relation: lifts\n  bearing: 0.5\n")
	if !refusedAt(res, result.Item{Key: "id=" + analysis, Entry: 1}, result.Item{Key: "id=" + analysis, Entry: 3}) {
		t.Errorf("got %+v, want two refusals, entries 1 and 3", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused set-bearing wrote")
	}
}

// TestARigorWriteTakesTheTablesMaximum: every op that takes a node's own rigor
// refuses a value above the rigor tables' maximum, naming the key and the
// range, and the maximum itself — the whole of the scale — is written and read
// back.
func TestARigorWriteTakesTheTablesMaximum(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	rangeText := "[0, " + domainTop(kb.RigorMaximum) + "]"
	for _, tc := range []struct{ op, values string }{
		{"set-rigor", "entry:\n- id: " + seedClaim + "\n  rigor: 1.2\n"},
		{"insert-claim-entry", "entry:\n- title: A New Result\n  rigor: 1.2\n  rationale: Shown.\n"},
		{"insert-analysis-entry", "entry:\n- title: An Analysis\n  rigor: 1.2\n  rationale: Lifts.\n"},
		{"insert-work-entry", "entry:\n- key: Smith2020\n  title: Smith (2020)\n  rigor: 1.2\n  rationale: Cited.\n"},
		{"insert-experiment-entry", "entry:\n- document: leaf.md\n  title: A Trial\n  status: run\n  rigor: 1.2\n"},
	} {
		t.Run(tc.op, func(t *testing.T) {
			res := run(t, root, tc.op, tc.values)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("%s: %+v, want one refusal", tc.op, res)
			}
			if it := res.Refusals[0]; it.Key != "rigor" || !strings.Contains(it.Detail, "outside "+rangeText) {
				t.Errorf("%s refusal = %+v, want key rigor and the range %s", tc.op, it, rangeText)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("a refused %s wrote", tc.op)
			}
		})
	}

	atMaximum := f64(kb.RigorMaximum)
	if res := run(t, root, "set-rigor", "entry:\n- id: "+seedClaim+"\n  rigor: "+formatScore(atMaximum)+"\n"); res.Outcome != result.Done {
		t.Fatalf("set-rigor at the maximum: %+v", res)
	}
	if got := records(t, root).Claims[0].Rigor; !scoreEqual(got, atMaximum) {
		t.Errorf("rigor = %v, want the maximum %v", got, *atMaximum)
	}
}

// nodeRows is each node row of the KB at root, by id, as Derive yields it.
func nodeRows(t *testing.T, root string) map[string]kb.NodeRow {
	t.Helper()
	rows, err := index.Derive(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]kb.NodeRow{}
	for _, r := range rows.Nodes {
		out[r.ID] = r
	}
	return out
}

// TestAnAuthoredOneIsRead: 1.00 is the rigor tables' maximum, so an authored
// `rigor: 1.0` — a converted experiment — is in domain and reads as itself, and
// a hand-edited value above it reads at the maximum: show's record and the
// index's row both carry the read value, and verify reports nothing, neither a
// value at the maximum nor a clamped one being a finding. An axiom's 1.00 is its
// rigor by rule, and every surface still carries it.
func TestAnAuthoredOneIsRead(t *testing.T) {
	root := seedKB(t)
	g := records(t, root)
	g.Experiments = []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Converted", Status: "run", Rigor: f64(1.0)},
		{ID: "exp-bbbbbb", Title: "Hand-edited", Status: "run", Rigor: f64(1.2)}}
	g.Axioms = []kb.AxiomNode{{ID: "axm-S1", Title: "The rule"}}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(kb.GraphNodesFile)), kb.EncodeNodes(g), 0o644); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(root, "leaf.md")
	text, err := os.ReadFile(leaf)
	if err != nil {
		t.Fatal(err)
	}
	trials := string(text) + "\n## Trials\n" + kb.StatementMarker("exp-aaaaaa") + "\n" + kb.StatementMarker("exp-bbbbbb") + "\n\nBoth trials ran.\n"
	if err := os.WriteFile(leaf, []byte(trials), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte("# Axioms\n\n### Axiom S1: The rule\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := kb.OnDisk(root)
	if _, err := index.Refresh(src, log.Discard()); err != nil {
		t.Fatalf("refresh over an authored 1.0 and a hand-edited 1.20: %v", err)
	}
	if findings, err := index.Verify(src); err != nil || len(findings) > 0 {
		t.Errorf("verify over an authored 1.0 and a hand-edited 1.20: %v, %v; want no finding", findings, err)
	}

	ix, err := query.Load(src)
	if err != nil {
		t.Fatalf("query.Load: %v", err)
	}
	shown := func(id string) any {
		t.Helper()
		record, ok := query.ShowPayload(ix, id)
		if !ok {
			t.Fatalf("show %s: no record", id)
		}
		for _, f := range record {
			if f.Key == "rigor" {
				return f.Value
			}
		}
		t.Fatalf("show %s carries no rigor field: %+v", id, record)
		return nil
	}
	rows := nodeRows(t, root)
	for _, tc := range []struct {
		id   string
		want float64
	}{
		{"exp-aaaaaa", kb.RigorMaximum}, {"exp-bbbbbb", kb.RigorMaximum}, {"axm-S1", kb.AxiomRigor},
	} {
		if got := shown(tc.id); got != tc.want {
			t.Errorf("show %s carries rigor %v, want %v", tc.id, got, tc.want)
		}
		if got := rows[tc.id].Rigor; got == nil || !scoreEqual(got, f64(tc.want)) {
			t.Errorf("the index row for %s carries rigor %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestANewPremiseIsPendingAndMovesNoClaim: an inserted premise lands with a
// pending bearing, so it contributes nothing and the claim reads 0 until the
// bearing is judged; judging the edge afterwards is what moves it.
func TestANewPremiseIsPendingAndMovesNoClaim(t *testing.T) {
	root := seedKB(t)
	// The seed rests on an axiom, so it holds a value of its own for the
	// inserted claim's edge to multiply once that edge is judged.
	if res := run(t, root, "insert-axiom-entry", "entry:\n- label: S1\n  title: The rule\n"); res.Outcome != result.Done {
		t.Fatalf("insert-axiom-entry: %+v", res)
	}
	if res := run(t, root, "add-rests-on", "entry:\n- id: "+seedClaim+"\n  rests-on:\n  - id: axm-S1\n    bearing: 1.0\n"); res.Outcome != result.Done {
		t.Fatalf("add-rests-on: %+v", res)
	}
	claim := insertClaim(t, root, "Bound Result", "  rests-on:\n  - id: "+seedClaim+"\n")
	if got := nodeRows(t, root)[claim].Strength; got == nil || *got != 0 {
		t.Fatalf("a pending premise moved the claim: strength %v, want 0", got)
	}
	if res := run(t, root, "set-bearing", "entry:\n- id: "+seedClaim+"\n  target: "+claim+"\n  relation: supports\n  bearing: 0.2\n"); res.Outcome != result.Done {
		t.Fatalf("set-bearing: %+v", res)
	}
	// 0.2 × the seed's 0.90 is 0.18, under the claim's 0.50.
	bearing, seedRigor := 0.2, 0.9
	got := nodeRows(t, root)[claim].Strength
	if got == nil {
		t.Fatal("a judged premise left the claim with no strength")
	}
	if want := bearing * seedRigor; *got != want {
		t.Errorf("a judged premise left the claim at %v, want %v", *got, want)
	}
}

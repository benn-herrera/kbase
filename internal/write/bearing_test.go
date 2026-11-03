package write

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/query"
	"kbase/internal/result"
)

// seedPlace is an add's placement of its marker on the seed leaf's last line.
const seedPlace = "  document: leaf.md\n  line: 11\n  snippet: bound is tight\n"

// TestARigorWriteTakesTheTablesMaximum: every op that takes a node's own rigor
// refuses a value above the rigor tables' maximum, naming the key and the
// range, and the maximum itself — the whole of the scale — is written and read
// back.
func TestARigorWriteTakesTheTablesMaximum(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	rangeText := "[0, " + domainTop(kb.RigorMaximum) + "]"
	for _, tc := range []struct{ op, values string }{
		{EditClaim, "entry:\n- id: " + seedClaim + "\n  rigor: 1.2\n"},
		{AddClaim, "entry:\n- title: A New Result\n  rigor: 1.2\n  rationale: Shown.\n" + seedPlace},
		{AddAnalysis, "entry:\n- title: An Analysis\n  rigor: 1.2\n  rationale: Lifts.\n" + seedPlace},
		{AddWork, "entry:\n- label: Smith2020\n  title: Smith (2020)\n  rigor: 1.2\n  rationale: Cited.\n"},
		{AddExperiment, "entry:\n- title: A Trial\n  status: run\n  rigor: 1.2\n" + seedPlace},
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

	if res := run(t, root, EditClaim, "entry:\n- id: "+seedClaim+"\n  rigor: "+kb.PyFloatRepr(kb.RigorMaximum)+"\n"); res.Outcome != result.Done {
		t.Fatalf("edit-claim at the maximum: %+v", res)
	}
	if got := records(t, root).Claims[0].Rigor; got == nil || *got != kb.RigorMaximum {
		t.Errorf("rigor = %v, want the maximum %v", got, kb.RigorMaximum)
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
// `rigor: 1.0` — a converted experiment — is in domain and reads as itself:
// show's record and the index's row both carry it, and verify reports nothing.
// An axiom's 1.00 is its rigor by rule, and every surface still carries it.
func TestAnAuthoredOneIsRead(t *testing.T) {
	root := seedKB(t)
	g := records(t, root)
	g.Experiments = []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Converted", Status: "run", Rigor: f64(1.0)}}
	g.Axioms = []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "The rule"}}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(kb.GraphNodesFile)), kb.EncodeNodes(g), 0o644); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(root, "leaf.md")
	text, err := os.ReadFile(leaf)
	if err != nil {
		t.Fatal(err)
	}
	trials := string(text) + "\n## Trials\n" + kb.StatementMarker("exp-aaaaaa") + "\n\nThe trial ran.\n"
	if err := os.WriteFile(leaf, []byte(trials), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte("# Axioms\n\n"+kb.AxiomHeadingLine("S1", "The rule", "axm-s10000")+"\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := kb.OnDisk(root)
	if _, err := index.Refresh(src, log.Discard()); err != nil {
		t.Fatalf("refresh over an authored 1.0: %v", err)
	}
	if findings, err := index.Verify(src); err != nil || len(findings) > 0 {
		t.Errorf("verify over an authored 1.0: %v, %v; want no finding", findings, err)
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
		{"exp-aaaaaa", kb.RigorMaximum}, {"axm-s10000", kb.AxiomRigor},
	} {
		if got := shown(tc.id); got != tc.want {
			t.Errorf("show %s carries rigor %v, want %v", tc.id, got, tc.want)
		}
		if got := rows[tc.id].Rigor; got == nil || *got != tc.want {
			t.Errorf("the index row for %s carries rigor %v, want %v", tc.id, got, tc.want)
		}
	}
}

// TestANewPremiseIsPendingAndMovesNoClaim: an added premise lands with a
// pending bearing, so it contributes nothing and the claim reads 0 until the
// bearing is judged; judging the edge afterwards is what moves it.
func TestANewPremiseIsPendingAndMovesNoClaim(t *testing.T) {
	root := seedKB(t)
	// The seed rests on an axiom, so it holds a value of its own for the
	// added claim's edge to multiply once that edge is judged.
	axiom := onlyEntry(t, run(t, root, AddAxiom, "entry:\n- label: S1\n  title: The rule\n")).ID
	if res := runEdges(t, root, edgeEntry(AddEdge, axiom, seedClaim, kb.RelationFounds, "bearing: 1.0")); res.Outcome != result.Done {
		t.Fatalf("the seed's founds edge: %+v", res)
	}
	claim := onlyEntry(t, run(t, root, AddClaim, "entry:\n- title: Bound Result\n  rigor: 0.5\n  rationale: Shown.\n"+seedPlace)).ID
	if res := runEdges(t, root, edgeEntry(AddEdge, seedClaim, claim, kb.RelationSupports)); res.Outcome != result.Done {
		t.Fatalf("the claim's premise: %+v", res)
	}
	if got := nodeRows(t, root)[claim].Strength; got == nil || *got != 0 {
		t.Fatalf("a pending premise moved the claim: strength %v, want 0", got)
	}
	if res := runEdges(t, root, edgeEntry(EditEdge, seedClaim, claim, kb.RelationSupports, "bearing: 0.2")); res.Outcome != result.Done {
		t.Fatalf("edit-edge: %+v", res)
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

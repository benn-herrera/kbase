package write

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// splitKB is recordKB with an entry point listing one leaf holding text.
func splitKB(t *testing.T, g kb.Graph, text string) string {
	t.Helper()
	root := recordKB(t, g)
	for rel, body := range map[string]string{kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n", "leaf.md": text} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// splitTwice issues values, then again, and requires the second call unchanged
// and kb-root byte-identical.
func splitTwice(t *testing.T, root, values string) {
	t.Helper()
	if res := runRecordOp(t, root, SplitClaim, values, ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("split-claim: %+v", res)
	}
	before := snapshot(t, root)
	if again := runRecordOp(t, root, SplitClaim, values, ProvenanceMachine); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("split-claim again: %+v, want unchanged, byte-identical", again)
	}
}

func splitEntry(id, line, extra string) string {
	return "- id: " + id + "\n  title: The Split Analysis\n  rationale: Split off.\n  document: leaf.md\n  line: " + line + "\n" + extra
}

func TestSplitClaim(t *testing.T) {
	const c, p, q, r, s, w = "clm-aaaaaa", "clm-bbbbbb", "clm-dddddd", "clm-eeeeee", "clm-ffffff", "wrk-k"
	via := kb.Bearing{Value: 0.4}
	root := splitKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: c, Title: "C"}, {ID: p, Title: "P"}, {ID: q, Title: "Q"}, {ID: r, Title: "R"}, {ID: s, Title: "S"}},
		Works:  []kb.WorkNode{{ID: w, Title: "W"}},
		Founds: []kb.BearingEdge{{Source: w, Target: c, Bearing: kb.Bearing{Pending: true}}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: p, Target: c, Context: "via P", Bearing: via, ManuallyScored: true},
			{Relation: kb.RelationSupports, Source: c, Target: s, Bearing: kb.Bearing{Pending: true}},
		},
		References: []kb.UnscoredEdge{{Source: r, Target: c}},
		Demoted:    []kb.DemotedEdge{{Source: q, Target: c, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Value: 0.2}, ManuallyScored: true}},
	}, "[↑ Entry](entry-point.md)\n\n# Leaf\n\n> **Theorem.**\n>\n> The statement holds. <!-- claim-rigor: "+c+" -->\n>\n> More.\n")
	splitTwice(t, root, "entry:\n"+splitEntry(c, "6", ""))

	g := records(t, root)
	if len(g.Analyses) != 1 {
		t.Fatalf("analyses %+v, want one", g.Analyses)
	}
	a := g.Analyses[0]
	if a.Title != "The Split Analysis" || a.Rationale != "Split off." || a.Rigor != nil || a.ManuallyScored || a.NoEdge != "" {
		t.Errorf("analysis %+v", a)
	}
	leaf := snapshot(t, root)["leaf.md"]
	if !strings.Contains(leaf, "\n> "+kb.StatementMarker(a.ID)+"\n") || !strings.Contains(leaf, "holds. "+kb.StatementMarker(c)+"\n") {
		t.Errorf("markers misplaced:\n%s", leaf)
	}
	holds := func(relation, source, target string, bearing kb.Bearing, manual bool, context string) bool {
		return slices.ContainsFunc(g.Scorable, func(e kb.ScorableEdge) bool {
			return e.Relation == relation && e.Source == source && e.Target == target && e.Bearing == bearing && e.ManuallyScored == manual && e.Context == context
		})
	}
	if !holds(kb.RelationSupports, p, a.ID, via, true, "via P") {
		t.Errorf("supports P → A missing: %+v", g.Scorable)
	}
	if !slices.Equal(g.Founds, []kb.BearingEdge{{Source: w, Target: a.ID, Bearing: kb.Bearing{Pending: true}}}) {
		t.Errorf("founds %+v", g.Founds)
	}
	wantDemoted := kb.DemotedEdge{Source: q, Target: a.ID, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Value: 0.2}, ManuallyScored: true}
	if !slices.Equal(g.Demoted, []kb.DemotedEdge{wantDemoted}) {
		t.Errorf("demoted %+v, want %+v", g.Demoted, wantDemoted)
	}
	if !slices.Equal(g.References, []kb.UnscoredEdge{{Source: r, Target: c}}) {
		t.Errorf("references %+v", g.References)
	}
	if !holds(kb.RelationSupports, c, s, kb.Bearing{Pending: true}, false, "") {
		t.Errorf("supports C → S missing: %+v", g.Scorable)
	}
	if !holds(kb.RelationLifts, a.ID, c, kb.Bearing{Pending: true}, false, "") {
		t.Errorf("lifts A → C missing: %+v", g.Scorable)
	}
	for _, e := range g.Scorable {
		if e.Target == c && e.Relation != kb.RelationLifts {
			t.Errorf("%+v still targets C", e)
		}
	}
}

func TestSplitClaimMovesAProseMarker(t *testing.T) {
	const c, d = "clm-aaaaaa", "clm-bbbbbb"
	const statement = "Lead. The first opening sentence holds here."
	const opener = "The first opening sentence holds here."
	root := splitKB(t, kb.Graph{Claims: []kb.ClaimNode{{ID: c, Title: "C"}, {ID: d, Title: "D"}}},
		"[↑ Entry](entry-point.md)\n\n# Leaf\n\n"+statement+" More follows. "+kb.StatementMarker(c)+"\n")
	second := "[↑ Entry](entry-point.md)\n\n# Leaf Two\n\nLead in. " + kb.StatementMarker(d) + "\n" + opener + " More follows.\n"
	if err := os.WriteFile(filepath.Join(root, "leaf2.md"), []byte(second), 0o644); err != nil {
		t.Fatal(err)
	}
	splitTwice(t, root, "entry:\n"+splitEntry(c, "5", "  statement-end: "+strconv.Itoa(len(statement))+"\n"))
	a := records(t, root).Analyses[0].ID
	want := statement + " " + kb.StatementMarker(c) + " More follows. " + kb.StatementMarker(a) + "\n"
	if got := snapshot(t, root)["leaf.md"]; !strings.HasSuffix(got, "\n\n"+want) {
		t.Errorf("leaf %q, want it to end in %q", got, want)
	}

	across := strings.Replace(splitEntry(d, "6", "  statement-line: 6\n  statement-end: "+strconv.Itoa(len(opener))+"\n"), "leaf.md", "leaf2.md", 1)
	splitTwice(t, root, "entry:\n"+across)
	var b string
	for _, n := range records(t, root).Analyses {
		if n.ID != a {
			b = n.ID
		}
	}
	want = "Lead in.\n" + opener + " " + kb.StatementMarker(d) + " More follows. " + kb.StatementMarker(b) + "\n"
	if got := snapshot(t, root)["leaf2.md"]; !strings.HasSuffix(got, "\n\n"+want) {
		t.Errorf("leaf2 %q, want it to end in %q", got, want)
	}
}

// TestSplitClaimKeepsEveryPath: splitting two claims of the chain X → A → B → Y
// leaves a path between every pair that had one, and no cycle.
func TestSplitClaimKeepsEveryPath(t *testing.T) {
	const x, a, b, y = "clm-aaaaaa", "clm-bbbbbb", "clm-dddddd", "clm-eeeeee"
	nodes := []string{x, a, b, y}
	root := splitKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: x, Title: "X"}, {ID: a, Title: "A"}, {ID: b, Title: "B"}, {ID: y, Title: "Y"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: x, Target: a, Bearing: kb.Bearing{Pending: true}},
			{Relation: kb.RelationSupports, Source: a, Target: b, Bearing: kb.Bearing{Pending: true}},
			{Relation: kb.RelationSupports, Source: b, Target: y, Bearing: kb.Bearing{Pending: true}},
		},
	}, "[↑ Entry](entry-point.md)\n\n# Leaf\n\nA holds. "+kb.StatementMarker(a)+"\n\nB holds. "+kb.StatementMarker(b)+"\n")
	before := index.GraphDependsGraph(records(t, root))
	had := map[[2]string]bool{}
	for _, u := range nodes {
		for _, v := range nodes {
			had[[2]string{u, v}] = before.Path(u, v) != nil
		}
	}
	values := "entry:\n" + splitEntry(a, "5", "") + splitEntry(b, "7", "")
	if res := runRecordOp(t, root, SplitClaim, values, ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("split-claim: %+v", res)
	}
	g := records(t, root)
	after := index.GraphDependsGraph(g)
	for pair, ok := range had {
		if ok && after.Path(pair[0], pair[1]) == nil {
			t.Errorf("path %s → %s lost", pair[0], pair[1])
		}
	}
	through := after.Path(b, x)
	for _, n := range g.Analyses {
		if !slices.Contains(through, n.ID) {
			t.Errorf("Path(B, X) = %v does not pass through %s", through, n.ID)
		}
	}
	if len(g.Analyses) != 2 {
		t.Fatalf("analyses %+v, want two", g.Analyses)
	}
	for _, hosted := range [][]string{nil, {a, b, x, y}, {y, x, b, a}} {
		if _, cut, _ := index.BreakCycles(g, hosted); len(cut) != 0 {
			t.Errorf("hosted %v: BreakCycles cut %+v", hosted, cut)
		}
	}
}

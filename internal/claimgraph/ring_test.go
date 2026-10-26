package claimgraph

import (
	"slices"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/write"
)

// seedRingKB writes a minimal KB whose one leaf hosts three claims, each
// marked, whose depends edges close the cycle a → b → c → a.
func seedRingKB(t *testing.T) (string, map[string]string) {
	const (
		aID = "clm-raaaaa"
		bID = "clm-rbbbbb"
		cID = "clm-rccccc"
	)
	t.Helper()
	root := seedKB(t, "# Entry\n\n- [V](v/index.md)\n", map[string]string{
		"v/index.md": "[↑ Entry](../entry-point.md)\n\n# V\n\n- [Leaf](leaf.md)\n",
		"v/leaf.md": "[↑ V](index.md)\n\n# Leaf\n\nA holds here. <!-- claim-rigor: " + aID + " -->\n\nB holds here. <!-- claim-rigor: " + bID +
			" -->\n\nC holds here. <!-- claim-rigor: " + cID + " -->\n",
	}, kb.Graph{Claims: claimRecords(map[string]string{aID: "Result A", bID: "Result B", cID: "Result C"})})
	w := writer{kbRoot: root, lg: log.Discard()}
	wires := values{
		{"id": aID, kb.ListRestsOn: []map[string]any{{"id": bID, "bearing": kb.PendingLiteral}}},
		{"id": bID, kb.ListRestsOn: []map[string]any{{"id": cID, "bearing": kb.PendingLiteral}}},
		{"id": cID, kb.ListRestsOn: []map[string]any{{"id": aID, "bearing": kb.PendingLiteral}}},
	}
	if _, err := w.run("seed", write.AddBuildEdges, wires); err != nil {
		t.Fatal(err)
	}
	return root, map[string]string{"a": aID, "b": bID, "c": cID}
}

// TestRingRowCutsTheWalksEdge: over the planted cycle a → b → c → a the ring
// row cuts the one edge index.BreakCycles' walk closes it on, a's premise b,
// through demote-claim-edge, with a warning for it; the other two edges stand,
// the gate over the tree as the row left it holds, and a second row cuts
// nothing.
func TestRingRowCutsTheWalksEdge(t *testing.T) {
	root, ids := seedRingKB(t)
	w := writer{kbRoot: root, lg: log.Discard()}
	cut, warnings, err := ringRow(w)
	if err != nil {
		t.Fatalf("ringRow: %v", err)
	}
	if want := []pair{{ids["a"], ids["b"]}}; !slices.Equal(cut, want) || len(warnings) != 1 {
		t.Errorf("cut = %v with warnings %q, want %v with one warning", cut, warnings, want)
	}
	for _, f := range gate(root, log.Discard()) {
		if f.Status == StatusFail {
			t.Errorf("the gate failed %s: %+v", f.Check, f.Fields)
		}
	}
	g := loadRecords(t, root)
	want := []kb.DemotedEdge{{Source: ids["b"], Target: ids["a"], ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}}}
	if !slices.Equal(g.Demoted, want) || len(g.Scorable) != 2 {
		t.Errorf("records after the ring row: demoted %+v, %d scorable; want %+v and the other two edges standing", g.Demoted, len(g.Scorable), want)
	}
	if again, _, err := ringRow(w); err != nil || len(again) != 0 {
		t.Errorf("a second ring row cut %v (%v), want nothing", again, err)
	}
}

package query

import (
	"slices"
	"testing"

	"kbase/internal/kb"
)

// scoringFixtureNodes is one node of every kind, in no kind order.
func scoringFixtureNodes() []map[string]any {
	return []map[string]any{
		{"node_type": kb.NodeKindWork, "id": "wrk-aaaaaa", "title": "A Work"},
		{"node_type": kb.NodeKindClaim, "id": "clm-aaaaaa", "title": "A Claim"},
		{"node_type": kb.NodeKindAxiom, "id": "axm-aaaaaa", "title": "An Axiom"},
		{"node_type": kb.NodeKindExperiment, "id": "exp-aaaaaa", "title": "An Experiment"},
		{"node_type": kb.NodeKindAnalysis, "id": "ana-aaaaaa", "title": "An Analysis"},
	}
}

// scoringFixtureEdges is one edge of every relation — the scoring ones from
// each source kind that carries them — in no order the set keeps. Its two
// founds edges are what the bearing set must leave out.
func scoringFixtureEdges() []map[string]any {
	supports := edge("clm-aaaaaa", "ana-aaaaaa", kb.RelationSupports, kb.NodeKindAnalysis, "1.0")
	supports["context"] = "the sentence that marks the dependency"
	demoted := edge("clm-aaaaaa", "ana-aaaaaa", kb.RelationDemoted, kb.NodeKindAnalysis, nil)
	demoted["origin"] = kb.OriginCited
	return []map[string]any{
		edge("exp-aaaaaa", "clm-aaaaaa", kb.RelationRefutes, kb.NodeKindClaim, "1.0"),
		edge("wrk-aaaaaa", "clm-aaaaaa", kb.RelationFounds, kb.NodeKindClaim, "1.0"),
		supports,
		edge("ana-aaaaaa", "clm-aaaaaa", kb.RelationLifts, kb.NodeKindClaim, "1.0"),
		edge("axm-aaaaaa", "ana-aaaaaa", kb.RelationFounds, kb.NodeKindAnalysis, "1.0"),
		edge("exp-aaaaaa", "clm-aaaaaa", kb.RelationSupports, kb.NodeKindClaim, "1.0"),
		edge("clm-aaaaaa", "ana-aaaaaa", kb.RelationReferences, kb.NodeKindAnalysis, nil),
		demoted,
	}
}

// scoringItems is the set of pass, failing on an error.
func scoringItems(t *testing.T, ix *Index, pass string) []Item {
	t.Helper()
	items, err := ScoringSet(ix, pass)
	if err != nil {
		t.Fatalf("ScoringSet(%q): %v", pass, err)
	}
	return items
}

// splitItems is the set's nodes and its edges, failing on an item that carries
// neither or both.
func splitItems(t *testing.T, items []Item) ([]ScoringNode, []ScoringEdge) {
	t.Helper()
	var nodes []ScoringNode
	var edges []ScoringEdge
	for _, it := range items {
		switch {
		case it.Node != nil && it.Edge != nil:
			t.Fatalf("item %+v carries a node and an edge", it)
		case it.Node != nil:
			nodes = append(nodes, *it.Node)
		case it.Edge != nil:
			edges = append(edges, *it.Edge)
		default:
			t.Fatalf("item %+v carries neither", it)
		}
	}
	return nodes, edges
}

func TestScoringSet(t *testing.T) {
	reload := func() *Index {
		t.Helper()
		ix, err := fromRows(kb.OnDisk(t.TempDir()), indexRows(t, map[string][]map[string]any{"claims": scoringFixtureNodes(), "edges": scoringFixtureEdges()}))
		if err != nil {
			t.Fatal(err)
		}
		return ix
	}
	ix := reload()

	t.Run("rigor is the claim, the analysis and the experiment", func(t *testing.T) {
		nodes, edges := splitItems(t, scoringItems(t, ix, PassRigor))
		want := []ScoringNode{
			{Kind: kb.NodeKindAnalysis, ID: "ana-aaaaaa"},
			{Kind: kb.NodeKindClaim, ID: "clm-aaaaaa"},
			{Kind: kb.NodeKindExperiment, ID: "exp-aaaaaa"},
		}
		if !slices.Equal(nodes, want) {
			t.Errorf("rigor = %v, want %v", nodes, want)
		}
		if len(edges) != 0 {
			t.Errorf("rigor carries edges: %v", edges)
		}
	})

	t.Run("bearing is every edge the walk asks, ordered", func(t *testing.T) {
		nodes, edges := splitItems(t, scoringItems(t, ix, PassBearing))
		want := []ScoringEdge{
			{Source: "clm-aaaaaa", Target: "ana-aaaaaa", Relation: kb.RelationSupports},
			{Source: "exp-aaaaaa", Target: "clm-aaaaaa", Relation: kb.RelationSupports},
			{Source: "ana-aaaaaa", Target: "clm-aaaaaa", Relation: kb.RelationLifts},
			{Source: "exp-aaaaaa", Target: "clm-aaaaaa", Relation: kb.RelationRefutes},
		}
		if !slices.Equal(edges, want) {
			t.Errorf("bearing = %v, want %v", edges, want)
		}
		if len(nodes) != 0 {
			t.Errorf("bearing carries nodes: %v", nodes)
		}
	})

	t.Run("order is stable across reloads", func(t *testing.T) {
		for _, pass := range []string{PassRigor, PassBearing} {
			fn, fe := splitItems(t, scoringItems(t, ix, pass))
			sn, se := splitItems(t, scoringItems(t, reload(), pass))
			if !slices.Equal(fn, sn) || !slices.Equal(fe, se) {
				t.Errorf("%s: a reload changed the set: %v/%v, then %v/%v", pass, fn, fe, sn, se)
			}
		}
	})
}

// TestScoringSetExcludesFounds: a founds edge carries a bearing — kb.Scoring
// says so, and set-bearing writes it — but no pass walks it. Its source is a
// work or an axiom outside the corpus, so the model has only a title to judge
// the link by, and the value is the user's. The bearing set leaves it out, and
// so does the both set, from the edge side and from the node side alike: a work
// and an axiom are no rigor subject either.
func TestScoringSetExcludesFounds(t *testing.T) {
	ix := load(t, map[string][]map[string]any{"claims": scoringFixtureNodes(), "edges": scoringFixtureEdges()})
	for _, pass := range []string{PassBearing, PassBoth} {
		for _, it := range scoringItems(t, ix, pass) {
			switch {
			case it.Edge != nil && it.Edge.Relation == kb.RelationFounds:
				t.Errorf("%s walks the founds edge %s → %s", pass, it.Edge.Source, it.Edge.Target)
			case it.Node != nil && (it.Node.Kind == kb.NodeKindWork || it.Node.Kind == kb.NodeKindAxiom):
				t.Errorf("%s walks the %s %s, which is no rigor subject", pass, it.Node.Kind, it.Node.ID)
			}
		}
	}
}

// TestScoringSetRigorByID: the rigor set is ordered by id, not by the kind-major
// order the claims file is written in. An index mints kind-prefixed ids, whose
// id order coincides with kind order, so these ids are the sort keys that tell
// the two orders apart.
func TestScoringSetRigorByID(t *testing.T) {
	ix := &Index{nodes: []kb.NodeRow{
		{NodeType: kb.NodeKindAnalysis, ID: "n-c"},
		{NodeType: kb.NodeKindAxiom, ID: "n-9"},
		{NodeType: kb.NodeKindClaim, ID: "n-b"},
		{NodeType: kb.NodeKindExperiment, ID: "n-a"},
		{NodeType: kb.NodeKindWork, ID: "n-0"},
	}}
	nodes, edges := splitItems(t, scoringItems(t, ix, PassRigor))
	want := []ScoringNode{
		{Kind: kb.NodeKindExperiment, ID: "n-a"},
		{Kind: kb.NodeKindClaim, ID: "n-b"},
		{Kind: kb.NodeKindAnalysis, ID: "n-c"},
	}
	if !slices.Equal(nodes, want) {
		t.Errorf("rigor = %v, want %v", nodes, want)
	}
	if len(edges) != 0 {
		t.Errorf("rigor carries edges: %v", edges)
	}
}

func TestIndexEdge(t *testing.T) {
	ix := load(t, map[string][]map[string]any{"claims": scoringFixtureNodes(), "edges": scoringFixtureEdges()})
	supports := ScoringEdge{Source: "clm-aaaaaa", Target: "ana-aaaaaa", Relation: kb.RelationSupports}
	row, ok := ix.Edge(supports)
	if !ok {
		t.Fatalf("the index holds no %+v", supports)
	}
	if row.Context == nil || *row.Context != "the sentence that marks the dependency" {
		t.Errorf("Edge(%+v).Context = %v, want the authored context", supports, row.Context)
	}
	if _, ok := ix.Edge(ScoringEdge{Source: "clm-aaaaaa", Target: "wrk-aaaaaa", Relation: kb.RelationSupports}); ok {
		t.Error("an identity the index does not hold answers a row")
	}
}

func TestScoringSetUnknownPass(t *testing.T) {
	ix := load(t, nil)
	for _, tc := range []struct{ name, pass string }{
		{"empty", ""},
		{"another quantity", "strength"},
		{"not the pass's spelling", "Rigor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if items, err := ScoringSet(ix, tc.pass); err == nil {
				t.Errorf("ScoringSet(%q) = %v, want an unknown pass", tc.pass, items)
			}
		})
	}
}

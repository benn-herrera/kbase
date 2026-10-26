package query

import (
	"slices"
	"testing"

	"kbase/internal/kb"
)

// scoringFixture is one node of every kind and one edge record of every
// relation, in no order the set keeps. Its two founds edges are what the
// bearing set must leave out.
func scoringFixture() kb.Graph {
	supports := scorable(kb.RelationSupports, "clm-aaaaaa", "ana-aaaaaa", 1)
	supports.Context = "the sentence that marks the dependency"
	return kb.Graph{
		Works:       []kb.WorkNode{{ID: "wrk-aaaaaa", Title: "A Work"}},
		Claims:      []kb.ClaimNode{claim("clm-aaaaaa", "A Claim")},
		Axioms:      []kb.AxiomNode{{ID: "axm-aaaaaa", Title: "An Axiom"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "An Experiment", Status: kb.ExperimentRun}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "An Analysis"}},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationRefutes, "exp-aaaaaa", "clm-aaaaaa", 1),
			supports,
			scorable(kb.RelationLifts, "exp-aaaaaa", "ana-aaaaaa", 1),
			scorable(kb.RelationSupports, "exp-aaaaaa", "clm-aaaaaa", 1),
		},
		Founds: []kb.BearingEdge{{Source: "wrk-aaaaaa", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 1}},
			{Source: "axm-aaaaaa", Target: "ana-aaaaaa", Bearing: kb.Bearing{Value: 1}}},
		References: []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "ana-aaaaaa"}},
		Demoted: []kb.DemotedEdge{{Source: "clm-aaaaaa", Target: "ana-aaaaaa", ReplacedRelation: kb.RelationSupports,
			Bearing: kb.Bearing{Pending: true}}},
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
		return load(t, scoringFixture())
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
			{Source: "exp-aaaaaa", Target: "ana-aaaaaa", Relation: kb.RelationLifts},
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
	ix := load(t, scoringFixture())
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
	ix := load(t, scoringFixture())
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
	ix := load(t, kb.Graph{})
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

package index

import (
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

func TestDependsGraphPath(t *testing.T) {
	g := DependsGraph{"a": {"c", "b"}, "b": {"d"}, "c": {"d"}, "d": {"e"}}
	for _, tc := range []struct {
		name, from, to string
		want           []string
	}{
		{"shortest, successors in sorted order", "a", "e", []string{"a", "b", "d", "e"}},
		{"one edge", "b", "d", []string{"b", "d"}},
		{"a node to itself", "c", "c", []string{"c"}},
		{"against the edges", "e", "a", nil},
		{"a node with no edges", "x", "a", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.Path(tc.from, tc.to); !slices.Equal(got, tc.want) {
				t.Errorf("Path(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// TestBreakCyclesCutsAsTheWalkFindsThem: three cycles over unhosted nodes,
// walked by id from ana-aaaaaa.
// The first, ana-aaaaaa → clm-aaaaaa → ana-bbbbbb → clm-bbbbbb → ana-aaaaaa,
// holds two supports edges into an analysis and loses the first by identity,
// clm-aaaaaa → ana-bbbbbb, not the edge that closed it; the restarted walk
// then closes clm-bbbbbb → clm-cccccc → clm-bbbbbb and clm-cccccc →
// clm-dddddd → clm-cccccc, each losing its closing edge, a refutes edge among
// them. The demoted clm-aaaaaa → clm-dddddd would close a fourth and is not
// walked. Each cut is its record whole as a demoted one, the cuts do not
// depend on the records' order, and the result has no cycle left to cut.
func TestBreakCyclesCutsAsTheWalkFindsThem(t *testing.T) {
	one, pending := kb.Bearing{Value: 1}, kb.Bearing{Pending: true}
	edge := func(relation, source, target string) kb.ScorableEdge {
		return kb.ScorableEdge{Relation: relation, Source: source, Target: target, Bearing: one}
	}
	firstCut := kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "ana-bbbbbb",
		Context: "the cut", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true}
	secondCut := kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-cccccc", Target: "clm-bbbbbb", Bearing: pending}
	thirdCut := kb.ScorableEdge{Relation: kb.RelationRefutes, Source: "clm-dddddd", Target: "clm-cccccc", Bearing: kb.Bearing{Value: 0.3}}
	kept := []kb.ScorableEdge{
		edge(kb.RelationLifts, "ana-aaaaaa", "clm-aaaaaa"),
		edge(kb.RelationLifts, "ana-bbbbbb", "clm-bbbbbb"),
		edge(kb.RelationSupports, "clm-bbbbbb", "ana-aaaaaa"),
		edge(kb.RelationSupports, "clm-bbbbbb", "clm-cccccc"),
		edge(kb.RelationSupports, "clm-cccccc", "clm-dddddd"),
		edge(kb.RelationSupports, "exp-aaaaaa", "clm-aaaaaa"),
	}
	g := kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa"}, {ID: "clm-bbbbbb"}, {ID: "clm-cccccc"}, {ID: "clm-dddddd"}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa"}, {ID: "ana-bbbbbb"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Status: "run"}},
		Works:       []kb.WorkNode{{ID: "wrk-key2020"}},
		Demoted:     []kb.DemotedEdge{{Source: "clm-aaaaaa", Target: "clm-dddddd", ReplacedRelation: kb.RelationSupports, Bearing: one}},
		Founds:      []kb.BearingEdge{{Source: "wrk-key2020", Target: "clm-dddddd", Bearing: one}},
		Scorable:    slices.Concat(kept[:2], []kb.ScorableEdge{thirdCut, firstCut}, kept[2:], []kb.ScorableEdge{secondCut}),
	}
	demoted := func(e kb.ScorableEdge) kb.DemotedEdge {
		return kb.DemotedEdge{Source: e.Source, Target: e.Target, Context: e.Context, ReplacedRelation: e.Relation,
			Bearing: e.Bearing, ManuallyScored: e.ManuallyScored}
	}
	wantDemoted := []kb.DemotedEdge{g.Demoted[0], demoted(firstCut), demoted(secondCut), demoted(thirdCut)}
	wantWarnings := [][2]string{
		{"supports clm-aaaaaa → ana-bbbbbb", "ana-aaaaaa → clm-aaaaaa → ana-bbbbbb → clm-bbbbbb → ana-aaaaaa"},
		{"supports clm-cccccc → clm-bbbbbb", "clm-bbbbbb → clm-cccccc → clm-bbbbbb"},
		{"refutes clm-dddddd → clm-cccccc", "clm-cccccc → clm-dddddd → clm-cccccc"},
	}
	byIdentity := func(a, b kb.ScorableEdge) int {
		return slices.Compare(kb.EdgeKey(a.Relation, "", a.Source, a.Target), kb.EdgeKey(b.Relation, "", b.Source, b.Target))
	}

	got, cuts, warnings := BreakCycles(g, nil)
	if !slices.Equal(got.Scorable, kept) {
		t.Errorf("scorable records %+v, want %+v", got.Scorable, kept)
	}
	if want := []kb.ScorableEdge{firstCut, secondCut, thirdCut}; !slices.Equal(cuts, want) {
		t.Errorf("cuts %+v, want %+v in cut order", cuts, want)
	}
	if !slices.Equal(got.Demoted, wantDemoted) || !slices.Equal(got.Founds, g.Founds) {
		t.Errorf("demoted %+v and founds %+v, want %+v and %+v", got.Demoted, got.Founds, wantDemoted, g.Founds)
	}
	if len(warnings) != len(wantWarnings) {
		t.Fatalf("warnings %+v, want one per cut: %v", warnings, wantWarnings)
	}
	for i, w := range warnings {
		if w.Path != kb.GraphEdgesFile || !strings.Contains(w.Detail, wantWarnings[i][0]) || !strings.Contains(w.Detail, wantWarnings[i][1]) {
			t.Errorf("warning %d is %+v, want %s naming the edge %q and the cycle %q", i, w, kb.GraphEdgesFile, wantWarnings[i][0], wantWarnings[i][1])
		}
	}

	if again, more, _ := BreakCycles(got, nil); len(more) != 0 || !slices.Equal(again.Scorable, got.Scorable) {
		t.Errorf("a second walk cut %+v", more)
	}

	reordered := g
	reordered.Scorable = slices.SortedFunc(slices.Values(g.Scorable), func(a, b kb.ScorableEdge) int { return -byIdentity(a, b) })
	other, _, _ := BreakCycles(reordered, nil)
	if !slices.Equal(other.Demoted, wantDemoted) {
		t.Errorf("over the records in another order the cuts are %+v, want %+v", other.Demoted, wantDemoted)
	}
}

// TestBreakCyclesWalksTheHostedOrder: the cycle p0 → p1 → p2 → p3 → p0 holds
// two supports edges into an analysis, p0 → p1 and p2 → p3. Under either
// minting of the four ids the walk in hosted order p0, p1, p2, p3 cuts p0 → p1,
// the first of them in that order, though the second minting's ids put p2's
// edge first.
func TestBreakCyclesWalksTheHostedOrder(t *testing.T) {
	for _, minted := range [][4]string{
		{"clm-aaaaaa", "ana-aaaaaa", "clm-bbbbbb", "ana-bbbbbb"},
		{"clm-zzzzzz", "ana-zzzzzz", "clm-yyyyyy", "ana-yyyyyy"},
	} {
		t.Run(minted[0], func(t *testing.T) {
			edge := func(relation string, source, target int) kb.ScorableEdge {
				return kb.ScorableEdge{Relation: relation, Source: minted[source], Target: minted[target], Bearing: kb.Bearing{Pending: true}}
			}
			g := kb.Graph{
				Claims:   []kb.ClaimNode{{ID: minted[0]}, {ID: minted[2]}},
				Analyses: []kb.AnalysisNode{{ID: minted[1]}, {ID: minted[3]}},
				Scorable: []kb.ScorableEdge{edge(kb.RelationSupports, 2, 3), edge(kb.RelationLifts, 3, 0),
					edge(kb.RelationSupports, 0, 1), edge(kb.RelationLifts, 1, 2)},
			}
			got, _, warnings := BreakCycles(g, minted[:])
			cut := g.Scorable[2]
			want := []kb.DemotedEdge{{Source: cut.Source, Target: cut.Target, ReplacedRelation: cut.Relation, Bearing: cut.Bearing}}
			if !slices.Equal(got.Demoted, want) || len(warnings) != 1 {
				t.Errorf("demoted %+v with warnings %+v, want %+v alone", got.Demoted, warnings, want)
			}
		})
	}
}

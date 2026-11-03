package query

import (
	"fmt"
	"slices"
	"testing"

	"kbase/internal/argmark"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// diamondGraph is a diamond of supports edges: clm-dddddd under clm-bbbbbb
// and clm-cccccc, both under clm-aaaaaa. clm-dddddd rests on scoringAxiom at
// rigor 0.9, so it reads 0.9; clm-bbbbbb reads 0.5 × 0.9 = 0.45 and
// clm-cccccc 0.8 × 0.9 = 0.72, each under a rigor of 1.
func diamondGraph() kb.Graph {
	rigid := func(id string) kb.ClaimNode { c := claim(id, id); c.Rigor = f(1); return c }
	g := kb.Graph{Claims: []kb.ClaimNode{rigid("clm-aaaaaa"), rigid("clm-bbbbbb"), rigid("clm-cccccc"), claim("clm-dddddd", "D")},
		Scorable: []kb.ScorableEdge{
			scorable(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa", 1),
			scorable(kb.RelationSupports, "clm-cccccc", "clm-aaaaaa", 0.5),
			scorable(kb.RelationSupports, "clm-dddddd", "clm-bbbbbb", 0.5),
			scorable(kb.RelationSupports, "clm-dddddd", "clm-cccccc", 0.8),
		}}
	return scored(g, "clm-dddddd", f(0.9))
}

// traceRows is a trace answer as "source relation target depth strength
// contribution" lines, checking each row is an edges row then its three keys.
func traceRows(t *testing.T, records []result.Record) []string {
	t.Helper()
	want := []string{"source", "target", "relation", "bearing", "context", manuallyScoredKey, "replaced_relation", "depth", "strength", "contribution"}
	var out []string
	for _, r := range records {
		var keys []string
		for _, f := range r {
			keys = append(keys, f.Key)
		}
		if !slices.Equal(keys, want) {
			t.Fatalf("trace record keys %q, want %q", keys, want)
		}
		out = append(out, fmt.Sprintf("%v %v %v %v %v %v", r[0].Value, r[2].Value, r[1].Value, r[7].Value, r[8].Value, r[9].Value))
	}
	return out
}

func TestTraceWalksADiamond(t *testing.T) {
	ix := load(t, diamondGraph())
	for _, tc := range []struct {
		name, id, direction string
		want                []string
	}{
		{"in from the top", "clm-aaaaaa", DirectionIn, []string{
			"clm-bbbbbb supports clm-aaaaaa 1 0.45 0.45",
			"clm-cccccc supports clm-aaaaaa 1 0.72 0.36",
			"clm-dddddd supports clm-bbbbbb 2 0.9 0.45",
			"clm-dddddd supports clm-cccccc 2 0.9 0.72",
			"axm-s00000 founds clm-dddddd 3 1 1",
		}},
		{"out from the bottom", "clm-dddddd", DirectionOut, []string{
			"clm-dddddd supports clm-bbbbbb 1 0.9 0.45",
			"clm-dddddd supports clm-cccccc 1 0.9 0.72",
			"clm-bbbbbb supports clm-aaaaaa 2 0.45 0.45",
			"clm-cccccc supports clm-aaaaaa 2 0.72 0.36",
		}},
		{"out from the top reaches nothing", "clm-aaaaaa", DirectionOut, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, usage := TracePayload(ix, TraceSelection{ID: tc.id, Direction: tc.direction, Order: OrderAscending})
			if usage != nil {
				t.Fatalf("refused: %v", usage)
			}
			if got := traceRows(t, rows); !slices.Equal(got, tc.want) {
				t.Errorf("trace %s %s = %q, want %q", tc.id, tc.direction, got, tc.want)
			}
		})
	}
}

// TestTraceContributionRanksInputs: contribution ascending gives the weakest
// input first, a pending bearing's below every number (its source, a claim
// nothing carries, reads 0) and a planned experiment's, which takes no part,
// null and last in either order.
func TestTraceContributionRanksInputs(t *testing.T) {
	g := diamondGraph()
	g.Claims = append(g.Claims, claim("clm-eeeeee", "E"))
	g.Experiments = []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "X", Status: kb.ExperimentPlanned, Rigor: f(0.9)}}
	pending := scorable(kb.RelationSupports, "clm-eeeeee", "clm-aaaaaa", 0)
	pending.Bearing = kb.Bearing{Pending: true}
	g.Scorable = append(g.Scorable, pending, scorable(kb.RelationSupports, "exp-aaaaaa", "clm-aaaaaa", 0.5))
	// A refuter at strength 0.2 contributes its cap, 1 - 0.2: bearing x
	// strength would put the row below every supports row instead.
	g.Claims = append(g.Claims, claim("clm-ffffff", "F"))
	g = scored(g, "clm-ffffff", f(0.2))
	g.Scorable = append(g.Scorable, scorable(kb.RelationRefutes, "clm-ffffff", "clm-aaaaaa", 1))
	ix := load(t, g)
	sort := SortContribution
	trace := func(order string) []string {
		t.Helper()
		rows, usage := TracePayload(ix, TraceSelection{ID: "clm-aaaaaa", Direction: DirectionIn, Sort: &sort, Order: order})
		if usage != nil {
			t.Fatalf("refused: %v", usage)
		}
		return traceRows(t, rows)
	}
	ascending := []string{
		"clm-eeeeee supports clm-aaaaaa 1 0 *pending*",
		"clm-cccccc supports clm-aaaaaa 1 0.72 0.36",
		"clm-bbbbbb supports clm-aaaaaa 1 0.45 0.45",
		"clm-dddddd supports clm-bbbbbb 2 0.9 0.45",
		"clm-dddddd supports clm-cccccc 2 0.9 0.72",
		"clm-ffffff refutes clm-aaaaaa 1 0.2 0.8",
		"axm-s00000 founds clm-ffffff 2 1 1",
		"axm-s00000 founds clm-dddddd 3 1 1",
		"exp-aaaaaa supports clm-aaaaaa 1 0.9 <nil>",
	}
	if got := trace(OrderAscending); !slices.Equal(got, ascending) {
		t.Errorf("contribution ascending = %q, want %q", got, ascending)
	}
	descending := []string{ascending[6], ascending[7], ascending[5], ascending[4], ascending[2], ascending[3], ascending[1], ascending[0], ascending[8]}
	if got := trace(OrderDescending); !slices.Equal(got, descending) {
		t.Errorf("contribution descending = %q, want %q", got, descending)
	}
}

func TestTraceRefusesItsUsage(t *testing.T) {
	ix := load(t, diamondGraph())
	depth := SortDepth
	zero := 0
	for _, tc := range []struct {
		name string
		sel  TraceSelection
		want []string
	}{
		{"no direction", TraceSelection{ID: "clm-aaaaaa", Order: OrderAscending}, []string{"usage --direction"}},
		{"both is no trace direction", TraceSelection{ID: "clm-aaaaaa", Direction: DirectionBoth, Order: OrderAscending}, []string{"usage --direction"}},
		{"limit without sort", TraceSelection{ID: "clm-aaaaaa", Direction: DirectionIn, Order: OrderAscending, Limit: &zero}, []string{"usage --limit"}},
		{"limit 0", TraceSelection{ID: "clm-aaaaaa", Direction: DirectionIn, Order: OrderAscending, Sort: &depth, Limit: &zero}, []string{"usage --limit"}},
		{"no node", TraceSelection{ID: "clm-ffffff", Direction: DirectionIn, Order: OrderAscending}, []string{kb.CheckUnknownID + " <id>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, usage := TracePayload(ix, tc.sel)
			var got []string
			for _, item := range usage {
				got = append(got, item.Check+" "+argmark.Spelling{}.Spell(item.Key))
			}
			if rows != nil || !slices.Equal(got, tc.want) {
				t.Errorf("refusals %q (rows %v), want %q", got, rows, tc.want)
			}
		})
	}
}

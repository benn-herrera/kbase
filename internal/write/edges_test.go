package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// cycleGraph holds a claim B resting on A, an analysis and an experiment.
func cycleGraph() kb.Graph {
	return kb.Graph{
		Claims:      []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "An"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "E", Status: "run"}},
		Scorable:    []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: demotedA, Target: demotedB, Bearing: kb.Bearing{Value: 0.5}}},
	}
}

// TestAnAddingOpRefusesAnEdgeClosingACycle: add-rests-on and add-pairs refuse
// an edge that would close a cycle, check dependency-cycle, naming the path
// and writing nothing; add-build-edges writes it, the build's walk cutting
// what it closes.
func TestAnAddingOpRefusesAnEdgeClosingACycle(t *testing.T) {
	for _, c := range []struct{ op, values, key string }{
		{"add-rests-on", "entry:\n- id: " + demotedA + "\n  rests-on:\n  - id: " + demotedB + "\n", kb.ListRestsOn},
		{"add-pairs", "entry:\n- id: " + demotedB + "\n  refutes:\n  - id: " + demotedA + "\n", kb.RefutesKey},
	} {
		t.Run(c.op, func(t *testing.T) {
			root := recordKB(t, cycleGraph())
			before := snapshot(t, root)
			res := runRecordOp(t, root, c.op, c.values, ProvenanceCaller)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 {
				t.Fatalf("%s: %+v, want one refusal", c.op, res)
			}
			if r := res.Refusals[0]; r.Check != checkDependencyCycle || r.Key != c.key || r.Entry != 1 ||
				!strings.Contains(r.Detail, demotedB+" → "+demotedA+" → "+demotedB) {
				t.Errorf("refusal = %+v, want check %s on %s naming the path", r, checkDependencyCycle, c.key)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("a refused %s wrote the KB", c.op)
			}
		})
	}
	t.Run(AddBuildEdges, func(t *testing.T) {
		root := recordKB(t, cycleGraph())
		res := runRecordOp(t, root, AddBuildEdges, "entry:\n- id: "+demotedA+"\n  rests-on:\n  - id: "+demotedB+"\n    bearing: 0.5\n", ProvenanceCaller)
		if res.Outcome != result.Done {
			t.Fatalf("%s: %+v", AddBuildEdges, res)
		}
		want := kb.ScorableEdge{Relation: kb.RelationSupports, Source: demotedB, Target: demotedA, Bearing: kb.Bearing{Value: 0.5}}
		if g := records(t, root); !slices.Contains(g.Scorable, want) {
			t.Errorf("edges = %+v, want %+v written as a run's, the cycle standing", g.Scorable, want)
		}
	})
}

// TestTheBuildCutsTheCycleItWrote: add-build-edges closing a cycle lands, and
// over the degraded load it leaves a public op is refused, check degraded,
// writing nothing, while demote-claim-edge on the closing edge lands and leaves
// the load clean.
func TestTheBuildCutsTheCycleItWrote(t *testing.T) {
	root := recordKB(t, cycleGraph())
	if res := runRecordOp(t, root, AddBuildEdges, "entry:\n- id: "+demotedA+"\n  rests-on:\n  - id: "+demotedB+"\n", ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("%s: %+v", AddBuildEdges, res)
	}
	before := snapshot(t, root)
	res := runRecordOp(t, root, "set-rigor", "entry:\n- id: "+demotedA+"\n  rigor: 0.3\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) == 0 || res.Refusals[0].Check != kb.CheckDegraded {
		t.Errorf("set-rigor over the cycle: %+v, want refused, check %s", res, kb.CheckDegraded)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused set-rigor wrote the KB")
	}
	if res := runRecordOp(t, root, DemoteClaimEdge, "entry:\n- id: "+demotedA+"\n  target: "+demotedB+"\n", ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("%s over the cycle: %+v", DemoteClaimEdge, res)
	}
	if res := runRecordOp(t, root, "set-rigor", "entry:\n- id: "+demotedA+"\n  rigor: 0.3\n", ProvenanceCaller); res.Outcome != result.Done {
		t.Errorf("set-rigor once the cycle is cut: %+v, want done", res)
	}
}

// TestAddedEdgesCarryTheirValues: add-rests-on's and add-pairs' edges land with
// their context, a supplied bearing manually scored and an omitted one pending
// and not; a re-issue is unchanged and byte-identical.
func TestAddedEdgesCarryTheirValues(t *testing.T) {
	pending := kb.Bearing{Pending: true}
	for _, c := range []struct {
		op, values string
		want       []kb.ScorableEdge
	}{
		{"add-rests-on", "entry:\n- id: ana-aaaaaa\n  rests-on:\n  - id: " + demotedA + "\n    bearing: 0.4\n    context: its bound\n  - id: exp-aaaaaa\n",
			[]kb.ScorableEdge{
				{Relation: kb.RelationSupports, Source: demotedA, Target: "ana-aaaaaa", Context: "its bound", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true},
				{Relation: kb.RelationSupports, Source: "exp-aaaaaa", Target: "ana-aaaaaa", Bearing: pending},
			}},
		{"add-pairs", "entry:\n- id: exp-aaaaaa\n  supports:\n  - id: " + demotedA + "\n    bearing: 0.4\n  lifts:\n  - id: " + demotedB +
			"\n    context: the trial\n  refutes:\n  - id: ana-aaaaaa\n",
			[]kb.ScorableEdge{
				{Relation: kb.RelationSupports, Source: "exp-aaaaaa", Target: demotedA, Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true},
				{Relation: kb.RelationLifts, Source: "exp-aaaaaa", Target: demotedB, Context: "the trial", Bearing: pending},
				{Relation: kb.RelationRefutes, Source: "exp-aaaaaa", Target: "ana-aaaaaa", Bearing: pending},
			}},
	} {
		t.Run(c.op, func(t *testing.T) {
			root := recordKB(t, cycleGraph())
			if res := runRecordOp(t, root, c.op, c.values, ProvenanceCaller); res.Outcome != result.Done {
				t.Fatalf("%s: %+v", c.op, res)
			}
			g := records(t, root)
			for _, w := range c.want {
				if !slices.Contains(g.Scorable, w) {
					t.Errorf("no record %+v among %+v", w, g.Scorable)
				}
			}
			before := snapshot(t, root)
			if again := runRecordOp(t, root, c.op, c.values, ProvenanceCaller); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
				t.Errorf("again: %+v, want unchanged, byte-identical", again)
			}
		})
	}
}

// TestAddPairsTakesItsSourcesLists: add-pairs refuses a list its source's kind
// does not author, naming the key and the lists it does, and an entry naming
// no list; nothing is written.
func TestAddPairsTakesItsSourcesLists(t *testing.T) {
	root := recordKB(t, cycleGraph())
	before := snapshot(t, root)
	for _, c := range []struct {
		name, values, key string
		allowed           []string
	}{
		{"a claim's lift", "entry:\n- id: " + demotedA + "\n  lifts:\n  - id: " + demotedB + "\n", kb.LiftsKey, []string{kb.RefutesKey}},
		{"an analysis's support", "entry:\n- id: ana-aaaaaa\n  supports:\n  - id: " + demotedB + "\n", kb.SupportsKey, []string{kb.LiftsKey, kb.RefutesKey}},
		{"no list", "entry:\n- id: ana-aaaaaa\n", "id=ana-aaaaaa", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := runRecordOp(t, root, "add-pairs", c.values, ProvenanceCaller)
			if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key || !slices.Equal(res.Refusals[0].Allowed, c.allowed) {
				t.Errorf("add-pairs: %+v, want one refusal on %s allowing %v", res, c.key, c.allowed)
			}
			if !maps.Equal(before, snapshot(t, root)) {
				t.Error("a refused add-pairs wrote the KB")
			}
		})
	}
}

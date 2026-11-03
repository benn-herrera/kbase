package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestACycleIsNamedInEdgeDirection: the cycle an add refuses reads as its
// edges run, each X → Y the edge from X to Y.
func TestACycleIsNamedInEdgeDirection(t *testing.T) {
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}, {ID: demotedC, Title: "C"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: demotedA, Target: demotedB, Bearing: kb.Bearing{Pending: true}},
			{Relation: kb.RelationSupports, Source: demotedB, Target: demotedC, Bearing: kb.Bearing{Pending: true}},
		},
	})
	before := snapshot(t, root)
	res := runEdges(t, root, edgeEntry(AddEdge, demotedC, demotedA, kb.RelationSupports))
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("add-edge: %+v, want one refusal", res)
	}
	if r := res.Refusals[0]; r.Check != checkDependencyCycle ||
		!strings.Contains(r.Detail, demotedC+" → "+demotedA+" → "+demotedB+" → "+demotedC) {
		t.Errorf("refusal = %+v, want check %s naming the cycle in edge direction", r, checkDependencyCycle)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("a refused add-edge wrote the KB")
	}
}

// cycleGraph holds a claim B resting on A, an analysis and an experiment.
func cycleGraph() kb.Graph {
	return kb.Graph{
		Claims:      []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "An"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "E", Status: "run"}},
		Scorable:    []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: demotedA, Target: demotedB, Bearing: kb.Bearing{Value: 0.5}}},
	}
}

// TestAddBuildEdgesWritesAnEdgeClosingACycle: add-build-edges writes an edge
// closing a cycle as a run's, the build's walk cutting what it closes.
func TestAddBuildEdgesWritesAnEdgeClosingACycle(t *testing.T) {
	root := recordKB(t, cycleGraph())
	res := runRecordOp(t, root, AddBuildEdges, "entry:\n- id: "+demotedA+"\n  rests-on:\n  - id: "+demotedB+"\n    bearing: 0.5\n", ProvenanceCaller)
	if res.Outcome != result.Done {
		t.Fatalf("%s: %+v", AddBuildEdges, res)
	}
	want := kb.ScorableEdge{Relation: kb.RelationSupports, Source: demotedB, Target: demotedA, Bearing: kb.Bearing{Value: 0.5},
		JudgedHash: edgeJudgedNow(t, root, demotedB, demotedA)}
	if g := records(t, root); !slices.Contains(g.Scorable, want) {
		t.Errorf("edges = %+v, want %+v written as a run's, the cycle standing", g.Scorable, want)
	}
}

// TestTheBuildCutsTheCycleItWrote: add-build-edges closing a cycle lands, and
// over the cycle it leaves a public op is refused, a load error, check
// metadata, writing nothing, while demote-claim-edge on the closing edge lands
// and leaves the load clean.
func TestTheBuildCutsTheCycleItWrote(t *testing.T) {
	root := recordKB(t, cycleGraph())
	if res := runRecordOp(t, root, AddBuildEdges, "entry:\n- id: "+demotedA+"\n  rests-on:\n  - id: "+demotedB+"\n", ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("%s: %+v", AddBuildEdges, res)
	}
	before := snapshot(t, root)
	res := runRecordOp(t, root, EditClaim, "entry:\n- id: "+demotedA+"\n  rigor: 0.3\n", ProvenanceCaller)
	if res.Outcome != result.Refused || len(res.Refusals) == 0 || res.Refusals[0].Check != kb.CheckMetadata {
		t.Errorf("edit-claim over the cycle: %+v, want refused, check %s", res, kb.CheckMetadata)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused edit-claim wrote the KB")
	}
	if res := runRecordOp(t, root, DemoteClaimEdge, "entry:\n- source: "+demotedB+"\n  target: "+demotedA+"\n  relation: "+kb.RelationSupports+"\n", ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("%s over the cycle: %+v", DemoteClaimEdge, res)
	}
	if res := runRecordOp(t, root, EditClaim, "entry:\n- id: "+demotedA+"\n  rigor: 0.3\n", ProvenanceCaller); res.Outcome != result.Done {
		t.Errorf("edit-claim once the cycle is cut: %+v, want done", res)
	}
}

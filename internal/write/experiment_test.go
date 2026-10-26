package write

import (
	"maps"
	"slices"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestAnExperimentPremiseResolvesAtWrite: a rests-on entry naming an
// experiment the KB records lands, and reads back as a supports edge from that
// experiment to the entry, manually scored at the supplied bearing.
func TestAnExperimentPremiseResolvesAtWrite(t *testing.T) {
	root := seedKB(t)
	res := run(t, root, "insert-experiment-entry", "entry:\n- document: leaf.md\n  title: A Trial\n  status: run\n  rigor: 0.9\n")
	if res.Outcome != result.Done || len(res.IDs) != 1 {
		t.Fatalf("insert experiment: %+v", res)
	}
	experiment := res.IDs[0]
	values := "entry:\n- id: " + seedClaim + "\n  rests-on:\n  - id: " + experiment + "\n    bearing: 0.5\n"
	if r := run(t, root, "add-rests-on", values); r.Outcome != result.Done {
		t.Fatalf("rest on %s: %+v", experiment, r)
	}
	want := kb.ScorableEdge{Relation: kb.RelationSupports, Source: experiment, Target: seedClaim, Bearing: kb.Bearing{Value: 0.5}, ManuallyScored: true}
	if got := records(t, root).Scorable; !slices.Equal(got, []kb.ScorableEdge{want}) {
		t.Errorf("edges = %+v, want %+v", got, want)
	}
}

// TestAnUndeclaredExperimentPremiseRefuses: an exp- id no record carries is
// refused naming the premise and the id, with nothing written.
func TestAnUndeclaredExperimentPremiseRefuses(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	res := run(t, root, "add-rests-on", "entry:\n- id: "+seedClaim+"\n  rests-on:\n  - id: exp-zzzzzz\n    bearing: 0.5\n")
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("an undeclared experiment: %+v", res)
	}
	if it := res.Refusals[0]; it.Key != kb.ListRestsOn+".id=exp-zzzzzz" {
		t.Errorf("refusal = %+v, want the premise and the id named", it)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-rests-on wrote")
	}
}

package write

import (
	"maps"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/result"
)

// TestAnExperimentPremiseResolvesAtWrite: a rests-on bullet naming an
// experiment the KB declares lands, and a reload reads it back as a supports
// edge from that experiment to the entry.
func TestAnExperimentPremiseResolvesAtWrite(t *testing.T) {
	root := seedKB(t)
	res := run(t, root, "insert-experiment-entry", "entry:\n- document: leaf.md\n  status: run\n  rigor: 1.0\n", false)
	if res.Outcome != result.Done || len(res.IDs) != 1 {
		t.Fatalf("insert experiment: %+v", res)
	}
	experiment := res.IDs[0]
	values := "entry:\n- id: " + seedClaim + "\n  rests-on:\n  - id: " + experiment + "\n    bearing: 0.5\n"
	if r := run(t, root, "add-rests-on", values, false); r.Outcome != result.Done {
		t.Fatalf("rest on %s: %+v", experiment, r)
	}

	st, err := kb.Discover(kb.OnDisk(root), log.Discard())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range st.ClaimEntries {
		if e.ID != seedClaim {
			continue
		}
		for _, p := range e.Premises {
			if p.Source == experiment && p.Target == seedClaim && p.Relation == kb.RelationSupports &&
				p.Bearing == (kb.Bearing{Value: 0.5}) {
				return
			}
		}
		t.Fatalf("premises of %s after the reload: %+v, want %s → %s %s at bearing 0.5",
			seedClaim, e.Premises, experiment, seedClaim, kb.RelationSupports)
	}
	t.Fatalf("%s is in no register entry after the reload", seedClaim)
}

// TestAnUndeclaredExperimentPremiseRefuses: an exp- id no document declares is
// refused naming where an experiment is declared, with nothing written.
func TestAnUndeclaredExperimentPremiseRefuses(t *testing.T) {
	root := seedKB(t)
	before := snapshot(t, root)
	res := run(t, root, "add-rests-on", "entry:\n- id: "+seedClaim+"\n  rests-on:\n  - id: exp-zzzzzz\n    bearing: 0.5\n", false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("an undeclared experiment: %+v", res)
	}
	if it := res.Refusals[0]; it.Key != kb.ListRestsOn+".id" || !strings.Contains(it.Detail, "exp-zzzzzz") ||
		!strings.Contains(it.Detail, "no document declares") || !strings.Contains(it.Remedy, "insert-experiment-entry") {
		t.Errorf("refusal = %+v, want the experiment source named in its detail and its remedy", it)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-rests-on wrote")
	}
}

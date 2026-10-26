package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestAnAxiomPremiseResolvesAtWrite: a premise naming an axiom by its id lands
// only where the axiom's record stands; before it does, the premise is refused
// naming the axiom, with nothing written, and once it does the premise founds
// the claim at its bearing.
func TestAnAxiomPremiseResolvesAtWrite(t *testing.T) {
	root := seedKB(t)
	values := "entry:\n- id: " + seedClaim + "\n  rests-on:\n  - id: axm-S9\n    bearing: 0.5\n"
	before := snapshot(t, root)
	res := run(t, root, "add-rests-on", values)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || !strings.Contains(res.Refusals[0].Key, "axm-S9") {
		t.Fatalf("an undeclared axiom: %+v, want one refusal naming axm-S9", res)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-rests-on wrote")
	}

	if r := run(t, root, "insert-axiom-entry", "entry:\n- label: S9\n  title: A rule\n"); r.Outcome != result.Done {
		t.Fatalf("insert-axiom-entry: %+v", r)
	}
	if r := run(t, root, "add-rests-on", values); r.Outcome != result.Done {
		t.Fatalf("the same call against a declared axiom: %+v", r)
	}
	want := kb.BearingEdge{Source: "axm-S9", Target: seedClaim, Bearing: kb.Bearing{Value: 0.5}}
	if got := records(t, root).Founds; !slices.Equal(got, []kb.BearingEdge{want}) {
		t.Errorf("founds = %+v, want %+v", got, want)
	}
}

// TestASupersededAxiomSpellingNamesTheID: a premise spelling an axiom as the
// 2.0.x "Axiom <label>" or the 1.x INVARIANT-<label> is refused at its key,
// the refusal naming the axm- id that names the axiom.
func TestASupersededAxiomSpellingNamesTheID(t *testing.T) {
	for _, token := range []string{"Axiom S9", "INVARIANT-S9"} {
		t.Run(token, func(t *testing.T) {
			_, refusals := parseValues([]byte("entry:\n- id: "+seedClaim+"\n  rests-on:\n  - id: "+token+"\n"), "add-rests-on")
			if len(refusals) != 1 || refusals[0].Key != kb.ListRestsOn+"[1].id" || !strings.Contains(refusals[0].Detail, "axm-S9") {
				t.Errorf("refusals = %+v, want one at %s[1].id naming axm-S9", refusals, kb.ListRestsOn)
			}
		})
	}
}

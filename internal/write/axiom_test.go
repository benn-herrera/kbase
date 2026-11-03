package write

import (
	"strings"
	"testing"

	"kbase/internal/kb"
)

// TestASupersededAxiomSpellingNamesTheID: a premise spelling an axiom as the
// 2.0.x "Axiom <label>" or the 1.x INVARIANT-<label> is refused at its key,
// the refusal naming the query that lists the minted ids an axiom is named by.
func TestASupersededAxiomSpellingNamesTheID(t *testing.T) {
	for _, token := range []string{"Axiom S9", "INVARIANT-S9"} {
		t.Run(token, func(t *testing.T) {
			_, refusals := parseValues([]byte("entry:\n- id: "+seedClaim+"\n  rests-on:\n  - id: "+token+"\n"), AddBuildEdges)
			if len(refusals) != 1 || refusals[0].Key != kb.ListRestsOn+"[1].id" || !strings.Contains(refusals[0].Detail, kb.NodeLister) {
				t.Errorf("refusals = %+v, want one at %s[1].id naming %s", refusals, kb.ListRestsOn, kb.NodeLister)
			}
		})
	}
}

package write

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestAnAxiomPremiseResolvesAtWrite: a bullet citing "Axiom <label>" lands only
// where the axiom source declares it; a typo is refused naming that source, with
// nothing written.
func TestAnAxiomPremiseResolvesAtWrite(t *testing.T) {
	root := seedKB(t)
	values := "entry:\n- id: " + seedClaim + "\n  rests-on:\n  - id: Axiom S9\n    bearing: 0.5\n"
	before := snapshot(t, root)
	res := run(t, root, "add-rests-on", values, false)
	if res.Outcome != result.Refused || len(res.Refusals) != 1 {
		t.Fatalf("an undeclared axiom: %+v", res)
	}
	if it := res.Refusals[0]; !strings.Contains(it.Detail, kb.AxiomsFile) || !strings.Contains(it.Remedy, kb.AxiomsFile) {
		t.Errorf("refusal = %+v, want the axiom source named in its detail and its remedy", it)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-rests-on wrote")
	}

	if err := os.WriteFile(filepath.Join(root, kb.AxiomsFile), []byte("# Axioms\n\n### Axiom S9: A rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := run(t, root, "add-rests-on", values, false); r.Outcome != result.Done {
		t.Fatalf("the same call against a declared axiom: %+v", r)
	}
	if e, ok := premiseOf(t, snapshot(t, root)[kb.RegisterFile], seedClaim, "axm-S9"); !ok || e.Bearing != (kb.Bearing{Value: 0.5}) {
		t.Errorf("premise = %+v (set %v), want axm-S9 at bearing 0.5", e, ok)
	}
}

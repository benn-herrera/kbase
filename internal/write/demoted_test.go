package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// insertClaim inserts a claim titled title into the seed KB's register and
// returns its id.
func insertClaim(t *testing.T, root, title, extra string) string {
	t.Helper()
	res := run(t, root, "insert-claim-entry", "entry:\n- register: claim-quality.md\n  title: "+title+"\n  rigor: 0.5\n  rationale: Shown.\n"+extra, false)
	if res.Outcome != result.Done || len(res.IDs) != 1 {
		t.Fatalf("insert %s: %+v", title, res)
	}
	return res.IDs[0]
}

// TestDemotedEdges lands demoted edges as the build does and resolves them:
// each call issued twice, the second unchanged and byte-identical; a restore
// that would close a cycle, alone or with the batch's earlier restores, is
// refused naming the cycle.
func TestDemotedEdges(t *testing.T) {
	root := seedKB(t)
	b := insertClaim(t, root, "Result B", "  depends-on:\n  - id: "+seedClaim+"\n")
	c := insertClaim(t, root, "Result C", "")
	d := insertClaim(t, root, "Result D", "")
	register := func() string { return snapshot(t, root)[kb.RegisterFile] }
	twice := func(op, values string, want []Resolution) {
		t.Helper()
		first := run(t, root, op, values, false)
		if first.Outcome != result.Done || !slices.Equal(first.Resolved, want) {
			t.Fatalf("%s %q: %+v, want done resolving %v", op, values, first, want)
		}
		before := snapshot(t, root)
		second := run(t, root, op, values, false)
		if second.Outcome != result.Unchanged || len(second.Resolved) != 0 {
			t.Errorf("%s %q again: %+v, want unchanged resolving nothing", op, values, second)
		}
		if !maps.Equal(before, snapshot(t, root)) {
			t.Errorf("%s %q again changed kb-root", op, values)
		}
	}

	twice(AddBuildEdges, "entry:\n- id: "+seedClaim+"\n  demoted:\n  - id: "+b+"\n    origin: cited\n  - id: "+c+"\n    origin: inferred\n"+
		"- id: "+c+"\n  demoted:\n  - id: "+d+"\n    origin: cited\n- id: "+d+"\n  demoted:\n  - id: "+c+"\n    origin: cited\n", nil)
	if text := register(); !strings.Contains(text, "- demoted:\n  - "+b+" — Result B (origin cited)\n  - "+c+" — Result C (origin inferred)\n") {
		t.Fatalf("the demoted list is not spelled as wanted:\n%s", text)
	}
	index := snapshot(t, root)[kb.IndexDir+"/"+kb.IndexFileName("depends-on")]
	if !strings.Contains(index, `"source": "`+seedClaim+`", "target": "`+b+`", "relation": "demoted", "target_kind": "claim", "target_solidity_recorded": null, `+
		`"strength": null, "context": null, "fraction": null, "origin": "cited"}`) {
		t.Errorf("no demoted index row with its origin:\n%s", index)
	}

	cycle := run(t, root, ResolveDemoted, "entry:\n- id: "+seedClaim+"\n  target: "+b+"\n  action: restore\n", false)
	if len(cycle.Refusals) != 1 || cycle.Refusals[0].Check != checkDependencyCycle || !strings.Contains(cycle.Refusals[0].Detail, b+" → "+seedClaim+" → "+b) {
		t.Errorf("a restore closing a cycle: %+v", cycle)
	}
	batch := run(t, root, ResolveDemoted, "entry:\n- id: "+c+"\n  target: "+d+"\n  action: restore\n- id: "+d+"\n  target: "+c+"\n  action: restore\n", false)
	if len(batch.Refusals) != 1 || batch.Refusals[0].Entry != 2 || !strings.Contains(batch.Refusals[0].Detail, c+" → "+d+" → "+c) {
		t.Errorf("a batch whose second restore closes a cycle with its first: %+v", batch)
	}
	before := register()
	twiceNamed := run(t, root, ResolveDemoted, "entry:\n- id: "+c+"\n  target: "+d+"\n  action: remove\n- id: "+c+"\n  target: "+d+"\n  action: restore\n", false)
	if twiceNamed.Outcome != result.Refused || len(twiceNamed.Refusals) != 1 || twiceNamed.Refusals[0].Entry != 2 || twiceNamed.Refusals[0].Key != "target" {
		t.Errorf("a batch naming one pair twice: %+v, want refused at entry 2", twiceNamed)
	}
	if register() != before {
		t.Error("a refused batch naming one pair twice changed the register")
	}

	twice(ResolveDemoted, "entry:\n- id: "+seedClaim+"\n  target: "+c+"\n  action: restore\n", []Resolution{{seedClaim, c, resolvedRestore}})
	text := register()
	held := readRegister(text).claims[seedClaim]
	if !strings.Contains(text, "- depends-on:\n  - "+c+" — Result C (solidity ") ||
		slices.ContainsFunc(held.Demoted, func(e kb.Edge) bool { return e.Target == c }) {
		t.Errorf("the restore did not rewrite the demoted bullet as a depends-on bullet:\n%s", text)
	}
	twice(ResolveDemoted, "entry:\n- id: "+seedClaim+"\n  target: "+b+"\n  action: remove\n", []Resolution{{seedClaim, b, resolvedRemove}})
	if text := register(); strings.Contains(text, b+" — Result B (origin") {
		t.Errorf("the removed demoted bullet stands:\n%s", text)
	}
	seed := readRegister(register()).claims[seedClaim]
	if len(seed.Demoted) != 0 || strings.Count(register(), "- demoted:") != 2 {
		t.Errorf("the emptied demoted list kept its header: %+v\n%s", seed.Demoted, register())
	}
	nothing := run(t, root, ResolveDemoted, "entry:\n- id: "+b+"\n  target: "+seedClaim+"\n  action: restore\n", false)
	if nothing.Outcome != result.Unchanged {
		t.Errorf("restoring a pair already a depends edge: %+v, want unchanged", nothing)
	}
}

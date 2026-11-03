package asks

import (
	"testing"

	"kbase/internal/kb"
)

// TestJudgedHashTellsItsInputsApart: the hash moves with the template's
// digest, with each statement and with their order, never reads two lists
// whose concatenations agree as one, and is never the all-zeros hash a value
// never judged carries.
func TestJudgedHashTellsItsInputsApart(t *testing.T) {
	base := JudgedHash("template", "source", "target")
	for _, c := range []struct {
		name string
		got  kb.JudgedHash
	}{
		{"another template", JudgedHash("template2", "source", "target")},
		{"another statement", JudgedHash("template", "source", "target!")},
		{"the statements swapped", JudgedHash("template", "target", "source")},
		{"the same bytes split elsewhere", JudgedHash("template", "sourcet", "arget")},
		{"one statement fewer", JudgedHash("template", "source")},
	} {
		if c.got == base {
			t.Errorf("%s hashes as the base does", c.name)
		}
	}
	if base == (kb.JudgedHash{}) || JudgedHash("") == (kb.JudgedHash{}) {
		t.Error("a computed hash is all zeros, which marks a value never judged")
	}
	if JudgedHash("template", "source", "target") != base {
		t.Error("the same inputs hash apart")
	}
}

// TestNodeJudgedHashTakesItsKindsAsk: a claim is judged under the statement
// ask and an analysis and an experiment under the method ask, so a claim's
// hash and an analysis's over one statement differ while an analysis's and an
// experiment's agree; a work and an axiom carry none.
func TestNodeJudgedHashTakesItsKindsAsk(t *testing.T) {
	hash := func(kind string) kb.JudgedHash {
		t.Helper()
		h, err := NodeJudgedHash(kind, "It holds.")
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		return h
	}
	if hash(kb.NodeKindClaim) == hash(kb.NodeKindAnalysis) || hash(kb.NodeKindAnalysis) != hash(kb.NodeKindExperiment) {
		t.Error("a claim and an analysis share an ask, or an analysis and an experiment do not")
	}
	for _, kind := range []string{kb.NodeKindWork, kb.NodeKindAxiom} {
		if _, err := NodeJudgedHash(kind, "It holds."); err == nil {
			t.Errorf("a %s has a judged hash", kind)
		}
	}
	edge, err := EdgeJudgedHash("It holds.", "")
	if err != nil {
		t.Fatal(err)
	}
	if edge == hash(kb.NodeKindClaim) || edge == hash(kb.NodeKindAnalysis) {
		t.Error("an edge is judged under a rigor ask")
	}
}

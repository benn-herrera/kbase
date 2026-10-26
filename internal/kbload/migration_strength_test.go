package kbload

import (
	"fmt"
	"testing"

	"kbase/internal/formats/format200"
	"kbase/internal/index"
	"kbase/internal/kb"
)

// strengthText is a strength as a test message names it: two decimals, or "none"
// for an absent — pending — one.
func strengthText(v *float64) string {
	if v == nil {
		return "none"
	}
	return fmt.Sprintf("%.2f", *v)
}

// TestAMigratedClaimsStrengthIsIts100Solidity: a 1.0.0 claim fused its statement
// with the argument for it, and its register wrote the minimum of the two as
// `- solidity:`. The migration splits the argument into an analysis, and the KB
// derives the same number — the claim lifting off that analysis at full bearing,
// the analysis resting on the premises the claim's entry listed. A claim whose
// authored confidence is pending reads 0: its analysis is unjudged, and an
// unjudged analysis lifts nothing, as an unjudged bearing supports nothing.
func TestAMigratedClaimsStrengthIsIts100Solidity(t *testing.T) {
	root := writeEntryPoint(t, "---\nkind: entry-point\nsubtree-claims: [clm-base01, clm-two001, clm-pend01, clm-axm001]\n"+
		"kb-format: \"1.0.0\"\n---\n\n# A KB\n\n- [A Leaf](leaf.md)\n")
	writeFiles(t, root, map[string]string{
		"leaf.md": "---\nkind: leaf\nclaims: [clm-base01, clm-two001, clm-pend01, clm-axm001]\n---\n[↑ KB](entry-point.md)\n\n\n" +
			"# A Leaf\n\nThe base result holds. <!-- claim-quality: clm-base01 -->\n\n" +
			"The second result holds. <!-- claim-quality: clm-two001 -->\n\n" +
			"The pending result holds. <!-- claim-quality: clm-pend01 -->\n\n" +
			"The axiom result holds. <!-- claim-quality: clm-axm001 -->\n",
		"claim-quality.md": "# Claim Quality Register\n\n---\n\n" +
			"## A Base Result\n<!-- id: clm-base01 -->\n\n### Quality\n- confidence: 0.90\n- rationale: Asserted outright.\n\n" +
			"### Solidity\n- solidity: 0.90\n\n---\n\n" +
			"## A Second Result\n<!-- id: clm-two001 -->\n\n### Quality\n- confidence: 0.80\n- depends-on:\n" +
			"  - clm-base01 — A Base Result (solidity 0.90)\n" +
			"  - work-smith2020 — Smith, A. 2020 (applicability 1.0)\n" +
			"- rationale: Rests on the base result and on Smith.\n\n### Solidity\n- solidity: 0.60\n\n---\n\n" +
			"## A Pending Result\n<!-- id: clm-pend01 -->\n\n### Quality\n- confidence: *pending*\n- depends-on:\n" +
			"  - clm-base01 — A Base Result (solidity 0.90)\n" +
			"- rationale: Unjudged.\n\n### Solidity\n- solidity: *pending*\n\n---\n\n" +
			"## An Axiom Result\n<!-- id: clm-axm001 -->\n\n### Quality\n- confidence: 0.50\n- depends-on:\n" +
			"  - INVARIANT-S1 (a labelling convention)\n" +
			"- rationale: Rests on the labelling convention.\n\n### Solidity\n- solidity: 0.50\n\n---\n\n" +
			"## Smith, A. 2020\n<!-- id: work-smith2020 -->\n\n### Quality\n- strength: 0.60\n- rationale: Fixture Press.\n",
		"invariants.md": "# Invariants\n\n### INVARIANT-S1: A labelling convention\n\nLabels are written as authored.\n",
	})
	src, err := Open(root)
	if err != nil {
		t.Fatalf("opening the 1.0.0 KB: %v", err)
	}
	if !src.Migrated() || src.Version() != "1.0.0" {
		t.Fatalf("the KB reads as migrated %t at %s, want a 1.0.0 KB converted in memory", src.Migrated(), src.Version())
	}

	// The 1.0.0 register line the conversion carries: its `- solidity:` under the
	// name the 2.0.0 form reads it by.
	register, err := src.ReadText(src.Path(kb.KBDir + "/" + kb.RegisterFile))
	if err != nil {
		t.Fatalf("reading the converted register: %v", err)
	}
	claims := format200.ParseClaimEntries(register, kb.RegisterFile, nil, func(string, int, string) {})
	if len(claims) != 4 {
		t.Fatalf("the converted register holds %d claim entries, want 4:\n%s", len(claims), register)
	}

	rows, err := index.Derive(src)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	derived := map[string]*float64{}
	for _, n := range rows.Nodes {
		if n.NodeType == kb.NodeKindClaim {
			derived[n.ID] = n.Strength
		}
	}
	zero := 0.0
	for _, c := range claims {
		want := c.Strength // the 1.0.0 register's "- solidity:", as the 2.0.0 register carries it
		if want == nil {
			want = &zero
		}
		got, ok := derived[c.ID]
		if !ok || got == nil || *got != *want {
			t.Errorf("%s: the 1.0.0 register wrote %s, the migrated KB derives %s", c.ID, strengthText(c.Strength), strengthText(got))
		}
	}
}

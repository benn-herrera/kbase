package write

import (
	"strings"
	"testing"

	"kbase/internal/kb"
)

// dropListFixture is a claim register holding the list shapes the edge ops
// splice: a demoted list of two bullets and a bulletless references header,
// a depends-on list of one bullet and a references list of two, and a
// references list whose bullet follows non-bullet text.
const dropListFixture = `# Register

## Seed Result
<!-- id: clm-aaaaaa -->

> **Leaf references:** [leaf](./leaf.md).

### Quality
- confidence: 0.9
- demoted:
  - clm-bbbbbb — Result B (origin cited)
  - clm-cccccc — Result C (origin inferred)
- references:
- solidity: 0.90 (ok to build on)
- rationale: Seeded.

---

## Result B
<!-- id: clm-bbbbbb -->

> **Leaf references:** *(none — entry has no citing leaf; bidirectional-coverage check will fail)*

### Quality
- confidence: 0.5
- depends-on:
  - clm-aaaaaa — Seed Result (solidity 0.90) [the bound it rests on]
- references:
  - clm-aaaaaa — Seed Result
  - clm-cccccc — Result C
- solidity: 0.50 (use as input only, don't build deeper) [= min(0.50, 0.90)]
- rationale: Shown.

---

## Result C
<!-- id: clm-cccccc -->

> **Leaf references:** *(none — entry has no citing leaf; bidirectional-coverage check will fail)*

### Quality
- confidence: 0.5
- references:
  Cites the seed informally.
  - clm-aaaaaa — Seed Result
- solidity: 0.50 (use as input only, don't build deeper)
- rationale: Shown.
`

// TestDropListBullets pins dropListBullets over each list shape, each with
// the matcher of the op that splices it: drop-claim-edges takes every bullet,
// drop-inbound-edges and the demoted removals the bullets naming a target. A
// re-issue over each result changes nothing.
func TestDropListBullets(t *testing.T) {
	const a, b, c = "clm-aaaaaa", "clm-bbbbbb", "clm-cccccc"
	every := func([]string) bool { return true }
	replaced := func(old, new string) string {
		t.Helper()
		if strings.Count(dropListFixture, old) != 1 {
			t.Fatalf("the fixture does not hold %q exactly once", old)
		}
		return strings.Replace(dropListFixture, old, new, 1)
	}
	without := func(cut string) string { t.Helper(); return replaced(cut, "") }
	dependsOn, references, demoted := kb.IndexDependsOn, kb.RelationReferences, kb.RelationDemoted
	for _, tc := range []struct {
		name, nodeID, list string
		match              func([]string) bool
		want               string
	}{
		{"every bullet, the list's only one", b, dependsOn, every,
			without("- depends-on:\n  - clm-aaaaaa — Seed Result (solidity 0.90) [the bound it rests on]\n")},
		{"every bullet of two", b, references, every,
			without("- references:\n  - clm-aaaaaa — Seed Result\n  - clm-cccccc — Result C\n")},
		{"every bullet, a bulletless header", a, references, every,
			replaced("- references:\n- solidity: 0.90", "- solidity: 0.90")},
		{"every bullet, an absent list", c, dependsOn, every, dropListFixture},
		{"every bullet, text before it kept with the header", c, references, every,
			replaced("informally.\n  - clm-aaaaaa — Seed Result\n", "informally.\n")},
		{"one bullet of two naming the target", b, references, bulletNames(c), without("  - clm-cccccc — Result C\n")},
		{"the only bullet naming the target", b, dependsOn, bulletNames(a),
			without("- depends-on:\n  - clm-aaaaaa — Seed Result (solidity 0.90) [the bound it rests on]\n")},
		{"no bullet naming the target", b, references, bulletNames("clm-dddddd"), dropListFixture},
		{"one demoted bullet of two", a, demoted, bulletNames(b), without("  - clm-bbbbbb — Result B (origin cited)\n")},
		{"every demoted bullet", a, demoted, bulletNames(b, c),
			without("- demoted:\n  - clm-bbbbbb — Result B (origin cited)\n  - clm-cccccc — Result C (origin inferred)\n")},
		{"an absent demoted list", b, demoted, bulletNames(a), dropListFixture},
	} {
		t.Run(tc.name, func(t *testing.T) {
			splice := dropListBullets(tc.nodeID, tc.list, tc.match)
			got, err := splice(dropListFixture)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
			}
			again, err := splice(got)
			if err != nil {
				t.Fatal(err)
			}
			if again != got {
				t.Errorf("a re-issue changed the document:\n%s", again)
			}
		})
	}
}

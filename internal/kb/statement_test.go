package kb

import (
	"strings"
	"testing"
)

// TestStatementReadsTheLayout: a hosted node's statement is read from where
// its marker stands — a claim's paragraph, or a heading's section where its
// marker ends the heading; an analysis's nearest section; and the two layouts
// the build's analysis split writes, which read so only where the analysis
// lifts the claim.
func TestStatementReadsTheLayout(t *testing.T) {
	quotedSplit := "# H\n\n> The claim.<!-- kbase-id: clm-aaaaaa -->\n> Its argument.\n>\n> <!-- kbase-id: ana-cccccc -->\n"
	proseSplit := "# H\n\nThe claim.<!-- kbase-id: clm-aaaaaa --> Its argument.<!-- kbase-id: ana-cccccc -->\n"
	for _, tc := range []struct {
		name, text, id, want string
		lifted               bool
	}{
		{"a claim's paragraph", "# H\n\nBefore.\n\nFirst line\nsecond line.<!-- kbase-id: clm-aaaaaa -->\n\nAfter.\n", "clm-aaaaaa",
			"First line\nsecond line.", false},
		{"a paragraph opens one list item", "# H\n\n- one\n- two<!-- kbase-id: clm-aaaaaa -->\n  more\n", "clm-aaaaaa",
			"- two\n  more", false},
		{"a claim marking a heading", "# H\n\n## Bound<!-- kbase-id: clm-aaaaaa -->\n\nIt holds.\n\n### Deeper\n\nStill.\n\n## Next\n\nNo.\n", "clm-aaaaaa",
			"It holds.\n\n### Deeper\n\nStill.", false},
		{"an analysis's section", "# H\n\nIntro.\n\n## Proof\n\nStep one.\n\nStep two.<!-- kbase-id: ana-cccccc -->\n\n## Next\n\nNo.\n", "ana-cccccc",
			"Step one.\n\nStep two.", false},
		{"a claim's paragraph holding a display-maths fence",
			"# H\n\nThe bound holds:\n```math\na = b\n\nc = d\n```\nfor every n.<!-- kbase-id: clm-aaaaaa -->\n\nAfter.\n", "clm-aaaaaa",
			"The bound holds:\n```math\na = b\n\nc = d\n```\nfor every n.", false},
		{"a fenced line opens no list item",
			"# H\n\nIt holds:\n```math\n- x\n```\nhere.<!-- kbase-id: clm-aaaaaa -->\n", "clm-aaaaaa",
			"It holds:\n```math\n- x\n```\nhere.", false},
		{"no marker", "# H\n\nText.\n", "clm-aaaaaa", "", false},
		{"a quoted claim lifted from a blank quoted line", quotedSplit, "clm-aaaaaa", "The claim.", true},
		{"a quoted claim no analysis lifts", quotedSplit, "clm-aaaaaa", "The claim.\nIts argument.", false},
		{"the analysis on the blank quoted line", quotedSplit, "ana-cccccc", "The claim.\nIts argument.", true},
		{"a claim split in prose", proseSplit, "clm-aaaaaa", "The claim.", true},
		{"a claim in prose no analysis lifts", proseSplit, "clm-aaaaaa", "The claim. Its argument.", false},
		{"the analysis split in prose", proseSplit, "ana-cccccc", "The claim. Its argument.", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lifts := func(analysis, claim string) bool {
				return tc.lifted && analysis == "ana-cccccc" && claim == "clm-aaaaaa"
			}
			if got := Statement(tc.text, tc.id, lifts); got != strings.TrimSpace(tc.want) {
				t.Errorf("Statement = %q, want %q", got, strings.TrimSpace(tc.want))
			}
		})
	}
}

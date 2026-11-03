package format400

import (
	"slices"
	"testing"
)

// TestSlugify: a heading's anchor under GitHub's slugger, before a repeat's
// suffix.
func TestSlugify(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"A — B", "a--b"},
		{"Solidity — the dep gate", "solidity--the-dep-gate"},
		{"Rule : applied", "rule--applied"},
		{"INVARIANT-S2: Axiom numbering", "invariant-s2-axiom-numbering"},
		{"  Ünïcode Wörds_ok ", "ünïcode-wörds_ok"},
		{"(Parenthesised) Title!", "parenthesised-title"},
		{"Equation (`eq:flow`) — Flow", "equation-eqflow--flow"},
		{"A <!-- a note --> heading", "a--heading"},
		{"Cure Is Hard<!-- kbase-id: clm-8350hb --><!-- kbase-id: ana-8350hb -->", "cure-is-hard"},
		{"Cure Is Hard <!-- kbase-id: clm-8350hb -->", "cure-is-hard"},
		{"Tab\there", "tabhere"},
		{"Title ##", "title"},
		{"The [Bound](b.md)", "the-bound"},
		{"An _emph_ word", "an-emph-word"},
		{"An __x__ word", "an-x-word"},
		{"snake_case and _real_ emphasis", "snake_case-and-real-emphasis"},
		{"Code `_x_` here", "code-_x_-here"},
		{"See ![Alt](i.png) end", "see-alt-end"},
		{"Title ## <!-- c -->", "title-"},
		{"_lone", "_lone"},
	} {
		if got := Slugify(tc.in); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestSluggerSuffixesARepeat: a heading repeated in one document takes -1,
// then -2, and a suffix an authored heading already holds is skipped.
func TestSluggerSuffixesARepeat(t *testing.T) {
	var s Slugger
	var got []string
	for _, h := range []string{"Rigor", "Strength", "Rigor", "Rigor-2", "Rigor", "rigor<!-- kbase-id: clm-aaaaaa -->"} {
		got = append(got, s.Slug(h))
	}
	if want := []string{"rigor", "strength", "rigor-1", "rigor-2", "rigor-3", "rigor-4"}; !slices.Equal(got, want) {
		t.Errorf("slugs %q, want %q", got, want)
	}
}

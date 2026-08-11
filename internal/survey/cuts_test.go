package survey

import (
	"strings"
	"testing"

	"kbase/internal/ingest"
	"kbase/internal/log"
)

// oneDocument is a corpus of a single 20-byte document whose bytes are known
// by eye: five words separated by single spaces, so offset 5 is
// whitespace-adjacent and offset 6 is inside a word.
//
//	"aaaa bbbb cccc dddd\n"
//	 0123456789...
func oneDocument(t *testing.T) (ingest.Corpus, []byte) {
	t.Helper()
	src := []byte("aaaa bbbb cccc dddd\n")
	c, err := ingest.New([]ingest.Unit{{Path: "a.md", Bytes: src}})
	if err != nil {
		t.Fatalf("ingest.New: %v", err)
	}
	return c, src
}

// fileWith is a well-formed single-file inventory carrying the given
// candidates: one preamble covering the whole document, which is the shape a
// headingless file has.
func fileWith(cuts []CutCandidate, size int) []File {
	return []File{{
		Path:     "a.md",
		Bytes:    size,
		Tokens:   5,
		Preamble: &Section{Start: 0, End: size},
		Cuts:     cuts,
	}}
}

// TestAssembleAcceptsLegalCandidates: the checks are a floor, not a filter —
// a legal list passes through unchanged and lands in the artifact.
func TestAssembleAcceptsLegalCandidates(t *testing.T) {
	corpus, src := oneDocument(t)
	cuts := []CutCandidate{{Offset: 5, Kind: CutParagraph}, {Offset: 15, Kind: CutHeading}}

	art, err := Assemble(corpus, fileWith(cuts, len(src)), log.Discard())
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if got := art.Files[0].Cuts; len(got) != 2 || got[0] != cuts[0] || got[1] != cuts[1] {
		t.Errorf("cuts = %+v, want %+v", got, cuts)
	}
}

// TestAssembleRefusesIllegalCandidates is the neutral half of the cut
// contract (ARCHITECTURE.md §5). Every case here is an adapter defect that
// would otherwise surface as a leaf cut in the middle of a word, several
// stages away from its cause — so it fails the run, and the message names the
// offset.
func TestAssembleRefusesIllegalCandidates(t *testing.T) {
	corpus, src := oneDocument(t)

	for _, tc := range []struct {
		name string
		cuts []CutCandidate
		want string
	}{{
		name: "out of order",
		cuts: []CutCandidate{{Offset: 15, Kind: CutHeading}, {Offset: 5, Kind: CutParagraph}},
		want: "does not follow the previous one",
	}, {
		name: "duplicated",
		cuts: []CutCandidate{{Offset: 5, Kind: CutHeading}, {Offset: 5, Kind: CutParagraph}},
		want: "does not follow the previous one",
	}, {
		name: "past the end of the file",
		cuts: []CutCandidate{{Offset: 200, Kind: CutHeading}},
		want: "is not interior to the 20-byte file",
	}, {
		name: "the file's own start",
		cuts: []CutCandidate{{Offset: 0, Kind: CutHeading}},
		want: "is not interior",
	}, {
		name: "the file's own end",
		cuts: []CutCandidate{{Offset: len(src), Kind: CutHeading}},
		want: "is not interior",
	}, {
		name: "mid-word: the tripwire",
		cuts: []CutCandidate{{Offset: 6, Kind: CutParagraph}},
		want: "has non-whitespace on both sides",
	}, {
		name: "a kind the artifact does not know",
		cuts: []CutCandidate{{Offset: 5, Kind: CutKind("stanza")}},
		want: `unknown kind "stanza"`,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			art, err := Assemble(corpus, fileWith(tc.cuts, len(src)), log.Discard())
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "a.md") {
				t.Errorf("error = %v, want the offending file named", err)
			}
			if len(art.Files) != 0 {
				t.Errorf("a refused inventory must not yield a partial artifact; got %+v", art)
			}
		})
	}
}

// TestWhitespaceAdjacent covers the rule both sides of the seam read: the
// survey when it accepts a candidate, stage 4 when it re-checks a cut the
// model chose.
func TestWhitespaceAdjacent(t *testing.T) {
	src := []byte("ab cd")
	for _, tc := range []struct {
		off  int
		want bool
	}{
		{0, true},  // the document's own start is a boundary
		{1, false}, // inside "ab"
		{2, true},  // before the space
		{3, true},  // after the space
		{4, false}, // inside "cd"
		{5, true},  // the document's own end
	} {
		if got := WhitespaceAdjacent(src, tc.off); got != tc.want {
			t.Errorf("WhitespaceAdjacent(%q, %d) = %v, want %v", src, tc.off, got, tc.want)
		}
	}
}

// TestCutKindRankOrdersByStructuralStrength: Split reads this order and the
// adapter's duplicate collapse reads it too, so it is asserted once rather
// than assumed twice.
func TestCutKindRankOrdersByStructuralStrength(t *testing.T) {
	if !(CutHeading.Rank() < CutFence.Rank() && CutFence.Rank() < CutParagraph.Rank()) {
		t.Errorf("ranks = %d/%d/%d, want heading < fence < paragraph",
			CutHeading.Rank(), CutFence.Rank(), CutParagraph.Rank())
	}
	if CutKind("stanza").Known() {
		t.Error("an unenumerated kind must not be Known")
	}
	if !CutHeading.Known() {
		t.Error("an enumerated kind must be Known")
	}
}

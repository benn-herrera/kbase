package dissect

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"kbase/internal/survey"
)

// TestSplitAlwaysPassesVerify is the invariant the whole seam stands on: the
// mechanical list is the fallback every model failure lands on, so it has to
// be valid without anyone checking (§3 monotone safety).
//
// It is a property rather than a set of expected outputs. What Split chooses
// is a heuristic and will be tuned; what it may never do is emit a list its
// own verifier refuses — so the cases below vary the document shape and the
// budget across the space where the heuristic has decisions to make, and
// assert the property rather than the choice. A budget that starves the span
// is a legal outcome too, and the assertion there is that it REFUSES rather
// than returning something short.
func TestSplitAlwaysPassesVerify(t *testing.T) {
	shapes := map[string][]blk{
		"headings and prose": {
			{survey.CutHeading, "# One\n\n" + words("a", 200)},
			{survey.CutParagraph, words("b", 200)},
			{survey.CutHeading, "# Two\n\n" + words("c", 200)},
			{survey.CutParagraph, words("d", 200)},
			{survey.CutHeading, "# Three\n\n" + words("e", 200)},
		},
		"prose only": {
			{survey.CutParagraph, words("a", 150)},
			{survey.CutParagraph, words("b", 150)},
			{survey.CutParagraph, words("c", 150)},
			{survey.CutParagraph, words("d", 150)},
		},
		"one huge fenced block among small ones": {
			{survey.CutParagraph, words("a", 80)},
			{survey.CutFence, words("code", 2000)},
			{survey.CutParagraph, words("c", 80)},
		},
		"fragments below the minimum": {
			{survey.CutHeading, "# One\n\n" + words("a", 200)},
			{survey.CutParagraph, "tiny"},
			{survey.CutParagraph, "also tiny"},
			{survey.CutHeading, "# Two\n\n" + words("c", 200)},
			{survey.CutParagraph, "trailing fragment"},
		},
		"candidates bunched at the start": {
			{survey.CutParagraph, words("a", 70)},
			{survey.CutParagraph, words("b", 70)},
			{survey.CutParagraph, words("c", 70)},
			{survey.CutFence, words("tail", 3000)},
		},
		"a single block with no candidates at all": {
			{survey.CutParagraph, words("a", 1000)},
		},
	}
	budgets := []int{minTokens, 100, 250, 500, 1000, 100_000}

	for name, blocks := range shapes {
		for _, budget := range budgets {
			t.Run(fmt.Sprintf("%s/budget=%d", name, budget), func(t *testing.T) {
				src, cands := buildDoc(blocks...)
				span := wholeSpan(src)

				cuts, err := Split(src, span, cands, budget)
				var starved StarvedError
				if errors.As(err, &starved) {
					// A refusal must name the span it could not cut, and
					// must not have produced anything.
					if cuts != nil {
						t.Fatalf("a refusal returned %+v; refuse-and-split never returns a partial list", cuts)
					}
					if !strings.Contains(err.Error(), fmt.Sprint(starved.Span.End)) {
						t.Errorf("refusal %v does not name the span", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("Split: %v", err)
				}
				assertVerifies(t, src, span, cands, cuts)
			})
		}
	}
}

// TestSplitPrefersStrongerStructure: among the candidates that fit, a heading
// beats a paragraph even when the paragraph would fill the budget better. The
// budget is a size limit; the heading is what the section is ABOUT.
func TestSplitPrefersStrongerStructure(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutParagraph, words("a", 100)},
		blk{survey.CutHeading, "# The heading\n\n" + words("b", 100)},
		blk{survey.CutParagraph, words("c", 100)},
		blk{survey.CutParagraph, words("d", 100)},
	)
	span := wholeSpan(src)
	heading := cands[0].Offset

	// A budget two paragraphs PAST the heading: the furthest candidate that
	// fits is a paragraph, so a splitter that only filled would cut there.
	budget := tokensOf(string(src[:cands[2].Offset])) + 10
	cuts, err := Split(src, span, cands, budget)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(cuts) < 2 || cuts[0].End != heading {
		t.Fatalf("first cut at %d, want the heading at %d (cuts %+v)", cuts[0].End, heading, cuts)
	}
	assertVerifies(t, src, span, cands, cuts)
}

// TestSplitFillsTowardTheBudget: among equals, the furthest candidate that
// fits wins. A splitter that took the first one would emit twice the leaves at
// half the size, which is a cost every later stage pays per call.
func TestSplitFillsTowardTheBudget(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutParagraph, words("a", 100)},
		blk{survey.CutParagraph, words("b", 100)},
		blk{survey.CutParagraph, words("c", 100)},
		blk{survey.CutParagraph, words("d", 100)},
	)
	span := wholeSpan(src)
	// Room for the first three blocks but not the fourth.
	budget := tokensOf(string(src[:cands[2].Offset]))

	cuts, err := Split(src, span, cands, budget)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if cuts[0].End != cands[2].Offset {
		t.Errorf("first cut at %d, want the last candidate within budget at %d", cuts[0].End, cands[2].Offset)
	}
	assertVerifies(t, src, span, cands, cuts)
}

// TestSplitPreMergesFragments: §5 step 1 — below-minimum fragments are gone
// before the model sees anything, so refinement only adjudicates boundaries
// somebody would defend.
func TestSplitPreMergesFragments(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 200)},
		blk{survey.CutHeading, "# Two\n\n" + words("b", 200)},
		blk{survey.CutParagraph, "a trailing fragment"},
	)
	span := wholeSpan(src)
	// One block per section exactly, which leaves the trailing fragment as a
	// section of its own for the merge to deal with.
	budget := tokensOf(string(src[:cands[0].Offset]))

	cuts, err := Split(src, span, cands, budget)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(cuts) != 2 {
		t.Fatalf("cuts = %+v, want two sections: the fragment merged into the one before it", cuts)
	}
	for i, c := range cuts {
		if n := estimate(src[c.Start:c.End]); n < minTokens {
			t.Errorf("section %d is %d tokens, under the %d-token minimum", i, n, minTokens)
		}
	}
	if last := cuts[len(cuts)-1]; last.End != span.End {
		t.Errorf("the merged tail ends at %d, want %d", last.End, span.End)
	}
	assertVerifies(t, src, span, cands, cuts)
}

// TestSplitRefusesAStarvedSpan: no legal cut fits, so the span goes back to
// the stage that sized it. Naming it is the whole content of the refusal —
// "something was too big" is not actionable, and truncating is nobody's job.
func TestSplitRefusesAStarvedSpan(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutParagraph, words("a", 50)},
		blk{survey.CutFence, words("code", 5000)},
	)
	span := wholeSpan(src)

	cuts, err := Split(src, span, cands, 200)
	var starved StarvedError
	if !errors.As(err, &starved) {
		t.Fatalf("err = %v (%T), want a StarvedError", err, err)
	}
	if cuts != nil {
		t.Errorf("cuts = %+v, want nothing at all", cuts)
	}
	if starved.Budget != 200 || starved.Tokens <= starved.Budget {
		t.Errorf("refusal = %+v, want the span's size against the budget", starved)
	}
	if !strings.Contains(err.Error(), "re-split") {
		t.Errorf("error = %v, want the remedy named", err)
	}
}

// TestSplitLeavesASmallSpanWhole: a span already under the budget is one
// section, whether or not it holds candidates. Cutting it would be work for
// its own sake, and the minimum would refuse the result anyway.
func TestSplitLeavesASmallSpanWhole(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutParagraph, words("a", 20)},
		blk{survey.CutParagraph, words("b", 20)},
	)
	span := wholeSpan(src)

	cuts, err := Split(src, span, cands, 100_000)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(cuts) != 1 || cuts[0] != span {
		t.Errorf("cuts = %+v, want the whole span as one section", cuts)
	}
}

// TestSplitWorksOnASubSpan: the splitter is contract-parameterized (span and
// budget in, cut list out), so it needs no taxonomy stage to exist and no
// knowledge of what the span is for.
func TestSplitWorksOnASubSpan(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 200)},
		blk{survey.CutHeading, "# Two\n\n" + words("b", 200)},
		blk{survey.CutHeading, "# Three\n\n" + words("c", 200)},
		blk{survey.CutHeading, "# Four\n\n" + words("d", 200)},
	)
	span := survey.Range{Start: cands[0].Offset, End: cands[2].Offset}

	cuts, err := Split(src, span, cands, 250)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if cuts[0].Start != span.Start || cuts[len(cuts)-1].End != span.End {
		t.Errorf("cuts %+v do not tile the sub-span %+v", cuts, span)
	}
	assertVerifies(t, src, span, cands, cuts)
}

// TestWindowsReachIntoBothNeighbours: the window is §9's boundary overlap made
// concrete — a fraction of each neighbour, capped per side — and it is
// asymmetric because the neighbours are.
func TestWindowsReachIntoBothNeighbours(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutParagraph, words("a", 1000)},
		blk{survey.CutParagraph, words("b", 100)},
	)
	span := wholeSpan(src)
	cuts := []survey.Range{{Start: 0, End: cands[0].Offset}, {Start: cands[0].Offset, End: span.End}}

	got := Windows(src, cuts)
	if len(got) != 1 {
		t.Fatalf("%d windows, want 1", len(got))
	}
	w := got[0]
	if !w.Contains(cuts[1].Start) {
		t.Errorf("window %+v does not contain the cut it is centred on", w)
	}
	back, forward := cuts[1].Start-w.Lo, w.Hi-cuts[1].Start
	if back <= forward {
		t.Errorf("window reaches %d back and %d forward; the long neighbour is the one behind", back, forward)
	}
	if want := int(float64(cuts[0].End-cuts[0].Start) * overlapFraction); back != want {
		t.Errorf("reach back = %d, want %d (%.0f%% of the neighbour)", back, want, overlapFraction*100)
	}

	// The cap binds before the fraction does on a large enough neighbour.
	big, bigCands := buildDoc(
		blk{survey.CutParagraph, words("a", 20_000)},
		blk{survey.CutParagraph, words("b", 20_000)},
	)
	bigCuts := []survey.Range{{Start: 0, End: bigCands[0].Offset}, {Start: bigCands[0].Offset, End: len(big)}}
	bw := Windows(big, bigCuts)[0]
	if bw.Hi-bw.Lo != 2*overlapCapBytes {
		t.Errorf("capped window spans %d bytes, want %d", bw.Hi-bw.Lo, 2*overlapCapBytes)
	}
	if estimate(big[bw.Lo:bigCuts[1].Start]) > overlapCapTokens {
		t.Errorf("one side of the window is over the %d-token cap", overlapCapTokens)
	}
}

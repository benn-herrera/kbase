package dissect

import (
	"errors"
	"strings"
	"testing"

	"kbase/internal/survey"
)

// verifyDoc is a four-block document whose blocks are all comfortably over
// the minimum, so a case can move one cut without tripping a size rule it did
// not mean to test.
func verifyDoc() ([]byte, []survey.CutCandidate) {
	return buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 120)},
		blk{survey.CutHeading, "# Two\n\n" + words("b", 120)},
		blk{survey.CutParagraph, words("c", 120)},
		blk{survey.CutHeading, "# Four\n\n" + words("d", 120)},
	)
}

// cutsAt turns a list of interior offsets into the section list they describe.
func cutsAt(span survey.Range, at ...int) []survey.Range {
	out := make([]survey.Range, 0, len(at)+1)
	cursor := span.Start
	for _, off := range at {
		out = append(out, survey.Range{Start: cursor, End: off})
		cursor = off
	}
	return append(out, survey.Range{Start: cursor, End: span.End})
}

// TestVerifyAcceptsALegalList: every rule is a refusal, so the first thing
// worth proving is that a legal list is not refused by any of them.
func TestVerifyAcceptsALegalList(t *testing.T) {
	src, cands := verifyDoc()
	span := wholeSpan(src)
	cuts := cutsAt(span, cands[0].Offset, cands[2].Offset)
	p := params(0)
	assertVerifies(t, src, span, cands, cuts, p)

	// A single section is a legal list too, and the minimum does not apply to
	// it: nobody chose the size of a span.
	if err := Verify(src, span, cands, []survey.Range{span}, nil, p); err != nil {
		t.Errorf("an uncut span must verify: %v", err)
	}
	tiny := []byte("hello\n")
	small := wholeSpan(tiny)
	if err := Verify(tiny, small, nil, []survey.Range{small}, nil, p); err != nil {
		t.Errorf("a span under the minimum must still verify as one section: %v", err)
	}
}

// TestVerifyRejections walks every check a CHOICE can fail. All of them are
// RejectionError, because all of them are answers the seam retries once and
// then discards for the mechanical baseline.
func TestVerifyRejections(t *testing.T) {
	src, cands := verifyDoc()
	span := wholeSpan(src)
	at := cands[1].Offset

	for _, tc := range []struct {
		name string
		cuts []survey.Range
		want string
	}{{
		name: "empty list",
		cuts: nil,
		want: "at least one section",
	}, {
		name: "a gap between two sections",
		cuts: []survey.Range{{Start: 0, End: at - 20}, {Start: at, End: len(src)}},
		want: "does not start where the one before it ended",
	}, {
		name: "two sections overlapping",
		cuts: []survey.Range{{Start: 0, End: at + 20}, {Start: at, End: len(src)}},
		want: "does not start where the one before it ended",
	}, {
		name: "the list starts after the span",
		cuts: []survey.Range{{Start: 4, End: len(src)}},
		want: "section 0 does not start where the one before it ended",
	}, {
		name: "the list stops short of the span",
		cuts: []survey.Range{{Start: 0, End: len(src) - 10}},
		want: "do not reach the end of the span",
	}, {
		name: "an empty section",
		cuts: []survey.Range{{Start: 0, End: 0}, {Start: 0, End: len(src)}},
		want: "empty or inverted",
	}, {
		name: "an offset nobody enumerated",
		cuts: cutsAt(span, at+1),
		want: "not an enumerated cut candidate",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := Verify(src, span, cands, tc.cuts, nil, params(0))
			var rej RejectionError
			if !errors.As(err, &rej) {
				t.Fatalf("err = %v (%T), want a RejectionError", err, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
			// The reason is what a corrective note is assembled from, so it
			// carries no byte offset; the operator-facing Error adds the one
			// byte the complaint is about.
			assertNoOffsets(t, rej.Note(), offsetsOf(cands)...)
		})
	}
}

// TestVerifyRejectsAnUndersizedSection: the minimum is a rule about choices,
// so it needs a legal candidate that produces a fragment. The fixture puts
// two candidates close together for exactly that.
func TestVerifyRejectsAnUndersizedSection(t *testing.T) {
	src, cands := buildDoc(
		blk{survey.CutHeading, "# One\n\n" + words("a", 120)},
		blk{survey.CutParagraph, "tiny"},
		blk{survey.CutHeading, "# Three\n\n" + words("c", 120)},
	)
	span := wholeSpan(src)
	cuts := cutsAt(span, cands[0].Offset, cands[1].Offset)

	err := Verify(src, span, cands, cuts, nil, params(0))
	var rej RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v (%T), want a RejectionError", err, err)
	}
	if !strings.Contains(err.Error(), "under the") {
		t.Errorf("error = %v, want the minimum named", err)
	}
}

// TestVerifyClampsToTheWindow: the model's authority is clamped twice (§5) —
// to the enumerated set and to the overlap window. This is the second clamp,
// and a candidate outside the window is a rejection even though it is a
// perfectly legal position in the document.
func TestVerifyClampsToTheWindow(t *testing.T) {
	src, cands := verifyDoc()
	span := wholeSpan(src)
	mechanical := cutsAt(span, cands[1].Offset)
	p := params(0)
	windows := Windows(src, mechanical, p)

	if err := Verify(src, span, cands, mechanical, windows, p); err != nil {
		t.Fatalf("the mechanical cut sits at the centre of its own window: %v", err)
	}
	// Move it to a candidate far outside the window.
	moved := cutsAt(span, cands[0].Offset)
	err := Verify(src, span, cands, moved, windows, p)
	var rej RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v (%T), want a RejectionError", err, err)
	}
	if !strings.Contains(err.Error(), "the range this boundary may move in") {
		t.Errorf("error = %v, want the clamp named", err)
	}
	assertNoOffsets(t, rej.Note(), windows[0].Lo, windows[0].Hi)
	if n := len(windows); n != len(mechanical)-1 {
		t.Errorf("%d windows for %d sections, want one per interior boundary", n, len(mechanical))
	}
	// A window count that does not match the boundaries is nobody's answer: it
	// is a plain error, which the seam classifies as OUR defect rather than
	// retrying it at a model.
	wrong := Verify(src, span, cands, mechanical, windows[:0], p)
	if wrong == nil {
		t.Fatal("a window count that does not match the boundaries must be refused")
	}
	if errors.As(wrong, &rej) {
		t.Errorf("err = %v, want a plain error rather than a rejected choice", wrong)
	}
}

// TestVerifyTripwireIsADefectNotARejection is the §5 argument made executable.
// The candidate set here claims an offset in the middle of a word — which is
// what a rebasing bug looks like from the inside — and the cut is a legal
// member of it. Membership passes; the bytes say otherwise; the verdict is
// OUR defect, and it must not be confused with a model's bad answer.
func TestVerifyTripwireIsADefectNotARejection(t *testing.T) {
	src := []byte(words("a", 120) + "\n\n" + words("b", 120) + "\n")
	span := wholeSpan(src)
	// Offset 2 is inside the first word: no adapter would enumerate it, and
	// no model could choose it if one had not.
	cands := []survey.CutCandidate{{Offset: 2, Kind: survey.CutParagraph}}

	err := Verify(src, span, cands, cutsAt(span, 2), nil, params(0))
	var defect OffsetDefectError
	if !errors.As(err, &defect) {
		t.Fatalf("err = %v (%T), want an OffsetDefectError", err, err)
	}
	var rej RejectionError
	if errors.As(err, &rej) {
		t.Error("the tripwire must not verdict as a rejection; a retry would be spent on a defect")
	}
	if defect.Offset != 2 {
		t.Errorf("the defect names offset %d, want 2", defect.Offset)
	}
	if !strings.Contains(err.Error(), "non-whitespace on both sides") {
		t.Errorf("error = %v, want the tripwire's own diagnosis", err)
	}
}

// TestVerifyRefusesAnImpossibleSpan: a span that is not a range of the source
// is neither class — nobody chose it and no cut list is in question — so it
// is a plain error rather than a rejection the seam would retry.
func TestVerifyRefusesAnImpossibleSpan(t *testing.T) {
	src := []byte("short\n")
	err := Verify(src, survey.Range{Start: 0, End: 99}, nil, []survey.Range{{Start: 0, End: 99}}, nil, params(0))
	if err == nil {
		t.Fatal("a span past the end of the source must be refused")
	}
	var rej RejectionError
	var defect OffsetDefectError
	if errors.As(err, &rej) || errors.As(err, &defect) {
		t.Errorf("err = %v, want a plain caller error", err)
	}
}

// TestDissectIsAViewNotACopy: the cut list is an overlay on immutable source
// (§5 step 4). The leaves must be the source's own bytes, so nothing
// downstream can be handed a re-typed copy of a document nobody kept.
func TestDissectIsAViewNotACopy(t *testing.T) {
	src, cands := verifyDoc()
	span := wholeSpan(src)
	leaves := Dissect(src, cutsAt(span, cands[1].Offset))
	if len(leaves) != 2 {
		t.Fatalf("%d leaves, want 2", len(leaves))
	}
	if &leaves[0][0] != &src[0] {
		t.Error("the first leaf is a copy; the dissector must return views into the source")
	}
}

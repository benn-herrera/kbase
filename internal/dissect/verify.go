package dissect

import (
	"fmt"

	"kbase/internal/survey"
)

// MoveWindow is the inclusive interval of byte offsets one boundary's cut may
// land in: the model's clamp, and the same bytes it was shown around the
// mechanical cut (§5 step 2).
//
// It is inclusive at both ends, which is why it is not a survey.Span: a
// Range is half-open because it describes bytes, and this describes
// POSITIONS. Both of a window's ends are positions a cut may legally take,
// and the mechanical cut sits at the centre of its own window — which a
// half-open interval would exclude whenever the overlap rounded to zero.
type MoveWindow struct {
	Lo, Hi int
}

// Contains reports whether off is a legal position in the window.
func (w MoveWindow) Contains(off int) bool { return off >= w.Lo && off <= w.Hi }

// Verify checks a cut list against every rule ARCHITECTURE.md §5 names, in
// the order that makes a failure diagnostic rather than merely true.
//
//	exact tiling → candidate membership → clamp → tripwire → minimum size
//
// The arguments are the whole of what a cut list is judged against: src and
// span are the bytes it claims to tile, cands is the enumerated candidate set
// from the survey artifact, and windows carries one clamp per INTERIOR
// boundary (len(cuts)-1 of them) or is nil for a list nobody clamped — the
// mechanical fallback, which is measured against nothing because it is what
// everything else is measured against.
//
// Every failure is one of two types and the distinction is the point:
//
//   - RejectionError is a choice that did not check out. The runner retries
//     once with the reason attached and then keeps the mechanical fallback
//     (§3 monotone safety) — model failure costs quality, never correctness.
//   - OffsetDefectError is the whitespace tripwire, and it is not
//     recoverable. See the package comment for why a cut can only fail it if
//     our own offsets are wrong.
//
// Membership is checked BEFORE the tripwire on purpose. An invented offset
// fails membership and is the model's doing; only an offset the adapter
// really did enumerate can reach the tripwire, and one that reaches it and
// fails is the evidence that these are not the bytes those candidates were
// enumerated against.
func Verify(src []byte, span survey.Span, cands []survey.CutCandidate, cuts []survey.Span, windows []MoveWindow, p Params) error {
	if err := checkSpan(src, span); err != nil {
		return err
	}
	if len(cuts) == 0 {
		return RejectionError{Offset: span.Start, Reason: "the cut list is empty; a span tiles as at least one section"}
	}
	if windows != nil && len(windows) != len(cuts)-1 {
		return fmt.Errorf("dissect: %d clamp windows for %d interior boundaries", len(windows), len(cuts)-1)
	}

	// Exact tiling: monotonic, no gaps, no overlaps, endpoints exact. Every
	// section is checked against where the previous one ended, so a gap and an
	// overlap are the same comparison read in two directions.
	cursor := span.Start
	for _, c := range cuts {
		if c.Start != cursor {
			return RejectionError{Offset: c.Start,
				Reason: "a section does not start where the one before it ended"}
		}
		if c.End <= c.Start {
			return RejectionError{Offset: c.Start, Reason: "a section is empty or inverted"}
		}
		cursor = c.End
	}
	if cursor != span.End {
		return RejectionError{Offset: cursor, Reason: "the sections do not reach the end of the span"}
	}

	for i := 1; i < len(cuts); i++ {
		at := cuts[i].Start
		if !isCandidate(cands, at) {
			return RejectionError{Offset: at, Reason: "not an enumerated cut candidate"}
		}
		if windows != nil && !windows[i-1].Contains(at) {
			return RejectionError{Offset: at, Reason: "outside the range this boundary may move in"}
		}
		if !survey.WhitespaceAdjacent(src, at) {
			return OffsetDefectError{Offset: at}
		}
	}

	// The minimum applies to CHOICES. A one-section list chose nothing — the
	// span is the size it is, and refusing it would refuse a small document
	// rather than a bad cut.
	if len(cuts) > 1 {
		for _, c := range cuts {
			if p.UnderMinimum(src, c) {
				return RejectionError{Offset: c.Start, Reason: fmt.Sprintf(
					"a section is under the %d-token minimum", MinTokens)}
			}
		}
	}
	return nil
}

// isCandidate reports whether off is in the enumerated set. The set is sorted
// (survey.Assemble refuses one that is not), but a linear scan over a few
// hundred entries per file is not worth a binary search anyone has to read.
func isCandidate(cands []survey.CutCandidate, off int) bool {
	for _, c := range cands {
		if c.Offset == off {
			return true
		}
		if c.Offset > off {
			return false
		}
	}
	return false
}

// RejectionError is a cut list that failed a structural check a choice could
// plausibly fail: a list that does not tile, an offset nobody enumerated, a
// cut outside its clamp, a section under the minimum.
//
// It is the retryable class. Its message is one short mechanical fact,
// because the runner caps the retry note it becomes at a dozen words
// (pipeline.retryNoteWords) — a sentence of prose there is a sentence
// the model never sees the end of.
//
// It has TWO renderings and the difference is the point (§5): Error is
// operator-facing and names the byte, Note is what may be shown to a model and
// names no byte at all. Reason is the shared half and carries NO NUMBER the
// model could act on — not a byte offset, and not a small integer either. That
// is a contract this type states and every construction of it must keep: the
// note is assembled from Reason, the model's whole vocabulary is a menu number
// in 1..menuCap, and a section index or a menu size formatted into a reason is
// a legal answer sitting in the next call's prompt. What the operator loses is
// covered by Error's offset, which locates the section exactly.
type RejectionError struct {
	// Offset is the byte the complaint is about.
	Offset int
	// Reason is the mechanical fact, carrying neither a byte offset (Error
	// adds the one offset this rejection is about) nor any number in the
	// menu's own range.
	Reason string
}

func (e RejectionError) Error() string {
	return fmt.Sprintf("dissect: cut at %d: %s", e.Offset, e.Reason)
}

// Note is the model-facing rendering: the mechanical fact alone.
//
// The runner turns whatever error a verifier returns into the retry note
// the retry carries, capped at twelve words. Error's operator prefix would
// spend four of them on "dissect: cut at 8392:" — before any content, and
// truncating the actionable half — and it would put a raw byte offset in the
// prompt of the very next call, in the same numeric shape the model is being
// asked to answer in. The model emits no raw offsets, so it is shown none.
func (e RejectionError) Note() string { return e.Reason }

// OffsetDefectError is the whitespace-adjacency tripwire firing: a cut that
// the candidate set admits and that still lands between two non-whitespace
// bytes.
//
// It is NOT retryable and NOT a fallback case (§5). Under the candidate
// clamp, no choice a model can make produces it — it means the offsets in
// play and the bytes they index came from different states of the world. A
// retry would re-ask a question that was never the problem, and falling back
// to the mechanical list would trust offsets from the same broken pipeline.
// So it aborts, loudly, naming the offset.
type OffsetDefectError struct {
	Offset int
}

func (e OffsetDefectError) Error() string {
	return fmt.Sprintf("dissect: cut at %d has non-whitespace on both sides; "+
		"a candidate cannot be there, so these offsets were not taken over these bytes", e.Offset)
}

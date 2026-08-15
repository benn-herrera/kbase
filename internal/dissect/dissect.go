// Package dissect is pipeline stage 4 (ARCHITECTURE.md §4, §5): the
// mechanical splitter, the verifier primitives every cut list passes through,
// the model-facing fallback-backed seam, and the dissector that finally slices the
// source.
//
// It is format-neutral. It reads the survey artifact's neutral types —
// section ranges, cut candidates — and the raw bytes under custody, and it
// never imports a format adapter or a parser: what counts as a legal cut
// position was decided behind the format seam and arrived as data
// (survey.CutCandidate). The import-policy test in internal/survey enforces
// that.
//
// # Two nets, deliberately orthogonal
//
// Verification runs two checks over every cut a model chose, and their value
// is that they look at different things:
//
//   - Candidate-set membership inspects STRUCTURE. The model chooses among
//     positions the format adapter enumerated, so a cut that bisects a
//     heading or lands inside a code fence is unrepresentable rather than
//     merely detectable. A cut that is not in the set is the model having
//     invented one: a rejection, retried once and then discarded for the
//     mechanical fallback.
//   - The whitespace-adjacency tripwire inspects RAW BYTES. At every cut at
//     least one neighbouring byte must be whitespace (survey.WhitespaceAdjacent
//     — the same function that admitted the candidate in the first place).
//
// A cut that passes membership and fails the tripwire cannot come from a
// model misjudging anything: it means the offsets we are checking were taken
// against different bytes than the ones they were enumerated against —
// rebasing drift, the wrong buffer, a source mutated under us. That is OUR
// defect, so it is a loud abort (OffsetDefectError) and never a retry. A bug
// that reaches production has to thread both nets at once, and the two are
// looking at unrelated evidence.
package dissect

import (
	"fmt"

	"kbase/internal/survey"
	"kbase/internal/tokens"
)

// Constants from ARCHITECTURE.md §9 (ruled 2026-08-10 as starting values,
// revisited at calibration like everything else in that table).
const (
	// overlapFraction is how much of each neighbouring section a boundary's
	// refinement window reaches into: the model sees the tail of the section
	// before the cut and the head of the one after it, and its authority is
	// clamped to that window.
	overlapFraction = 0.20

	// overlapCapTokens bounds ONE SIDE of that window. A percentage alone
	// would make the window grow with the section, and the point of the
	// window is that a boundary call stays small however large its
	// neighbours are.
	overlapCapTokens = 1000

	// minTokens is the smallest section refinement may produce or keep.
	// Below-minimum fragments are pre-merged before the model sees anything,
	// so it only ever adjudicates real boundaries (§5 step 1).
	//
	// It holds "min > overlap into a minimum-sized section" by construction:
	// the overlap is a FRACTION under 100%, so a minimum section's own
	// contribution to a window is always smaller than the section.
	minTokens = 64
)

// Params is what a cut list for one span was derived under: the estimator
// every size question in this package goes through, and the budget the
// mechanical splitter filled toward.
//
// It is a plain value threaded through Split, MoveWindows, Verify and NewRefiner —
// the shape internal/survey already uses for the same estimator, not an
// abstraction. The point is that calibration has ONE place to land (§8): a
// package that read tokens.Estimator{} at each call site would be the second
// estimation path the estimator's own doc comment exists to prevent.
type Params struct {
	// Est is the appliance's single token estimator (§8). The zero value
	// estimates at the calibrated default, so a caller with nothing to
	// calibrate leaves it alone.
	Est tokens.Estimator

	// BudgetTokens is the per-section budget Split fills toward.
	//
	// It is NOT a verification rule — premerge may legitimately push a
	// section past it (see premerge) — so Verify never reads it. It travels
	// with the estimator because it is half of what identifies a cut list:
	// the fold's parameter digest (Refiner.digest) covers it, so a re-plan at
	// a different budget invalidates the cut list adjudicated under the old
	// one.
	BudgetTokens int
}

// estimate is the appliance's single token estimator applied to bytes this
// package already holds.
func (p Params) estimate(b []byte) int { return p.Est.EstimateBytes(b) }

// underMinimum reports whether a section is below the minimum size.
//
// One spelling, because this is the predicate the proposer and the verifier
// must never disagree about: premerge folds a section away when it is true and
// Verify rejects a list when it is true, and two copies of it would drift on
// exactly the day the difference decided whether a cut list was legal.
func (p Params) underMinimum(src []byte, r survey.Span) bool {
	return p.estimate(src[r.Start:r.End]) < minTokens
}

// SliceLeaves returns one byte slice per section of a VERIFIED cut list, in
// order.
//
// The slices are views into src, not copies: the cut list is a derived
// overlay on immutable source (§5 step 4), and the source is never
// sliced-and-retyped. A caller that lands them as store artifacts holds them
// only as long as it holds src, and writes them out unchanged — anything that
// would mutate one is copying it first, by the same discipline that makes the
// custody bytes authoritative.
//
// It is a pure function of its inputs and does no checking: Verify is the
// place a cut list earns the name, and duplicating its checks here would be
// two implementations of one rule with nothing keeping them in agreement.
// Passing an unverified list is a caller defect, and the doc comment is the
// contract.
func SliceLeaves(src []byte, cuts []survey.Span) [][]byte {
	out := make([][]byte, 0, len(cuts))
	for _, c := range cuts {
		out = append(out, src[c.Start:c.End])
	}
	return out
}

// checkSpan refuses a span that is not a range of src, or that is a range of
// no bytes. It is a caller defect rather than either error class this package
// classifies — no model chose it, and no cut list is in question yet — so it
// is a plain error.
//
// The empty case is refused HERE rather than at each entry point because that
// is what makes one refusal cover Split, Verify and everything added later. An
// empty span is reachable (an empty document, a whole-file span over zero
// bytes) and Split's answer to it was a one-section list that its own Verify
// then rejected as "empty or inverted" — the right refusal blaming the wrong
// party, since the caller handed over a span with nothing in it.
func checkSpan(src []byte, span survey.Span) error {
	if span.Start < 0 || span.End < span.Start || span.End > len(src) {
		return fmt.Errorf("dissect: span [%d,%d) is not a range of the %d-byte source",
			span.Start, span.End, len(src))
	}
	if span.End == span.Start {
		return fmt.Errorf("dissect: span [%d,%d) holds no bytes; a span with no bytes is not a span to cut",
			span.Start, span.End)
	}
	return nil
}

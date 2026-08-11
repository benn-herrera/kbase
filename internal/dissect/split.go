package dissect

import (
	"fmt"

	"kbase/internal/survey"
	"kbase/internal/tokens"
)

// overlapCapBytes is overlapCapTokens in the unit windows are actually cut
// in. The estimator is a chars-per-token ratio (§8), so its inverse is that
// same constant read the other way — there is no second calibration here,
// just the one arithmetic.
const overlapCapBytes = int(overlapCapTokens * tokens.DefaultCharsPerToken)

// Split proposes the mechanical cut list for one span: the always-valid
// baseline the whole seam stands on (§3 monotone safety). Its output passes
// Verify by construction, and that is a property test rather than a comment.
//
// The heuristic is one sentence: fill toward the budget, and cut at the
// strongest structure available before it. Among the candidates that fit,
// the strongest KIND wins (heading over fence over paragraph —
// survey.CutKind.Rank, the same order the adapter's duplicate collapse
// reads), and among equals the LAST one wins, which is what "fill toward the
// budget" means. Below-minimum fragments are merged away before the list is
// returned, so refinement only ever adjudicates real boundaries.
//
// It is contract-parameterized — span and budget in, cut list out — so it
// needs no taxonomy stage to exist and no knowledge of what the span is for.
//
// A span it cannot cut under the budget is refused, loudly, naming the span
// (StarvedError). Truncation is nobody's job, and refuse-and-split is stage
// 3's: an oversized unit goes back to the skeleton that sized it.
func Split(src []byte, span survey.Range, cands []survey.CutCandidate, budgetTokens int) ([]survey.Range, error) {
	if err := checkSpan(src, span); err != nil {
		return nil, err
	}

	var cuts []survey.Range
	cursor := span.Start
	for estimate(src[cursor:span.End]) > budgetTokens {
		at, ok := pick(src, cands, cursor, span.End, budgetTokens)
		if !ok {
			return nil, StarvedError{
				Span:   survey.Range{Start: cursor, End: span.End},
				Tokens: estimate(src[cursor:span.End]),
				Budget: budgetTokens,
			}
		}
		cuts = append(cuts, survey.Range{Start: cursor, End: at})
		cursor = at
	}
	cuts = append(cuts, survey.Range{Start: cursor, End: span.End})
	return premerge(src, cuts), nil
}

// pick chooses the next cut after from: the strongest kind among the
// candidates that leave a section between the minimum and the budget, and
// the furthest one of that kind.
//
// The minimum is applied HERE rather than only in the merge afterwards, so
// the merge has at most a tail to deal with: a below-minimum section is never
// proposed in the first place, and the greedy walk cannot paint itself into a
// corner of fragments.
func pick(src []byte, cands []survey.CutCandidate, from, to, budget int) (int, bool) {
	best, bestRank := 0, 0
	found := false
	for _, c := range cands {
		if c.Offset <= from {
			continue
		}
		if c.Offset >= to {
			break
		}
		n := estimate(src[from:c.Offset])
		if n > budget {
			// Candidates are sorted, so everything after this one is larger.
			break
		}
		if n < minTokens {
			continue
		}
		rank := c.Kind.Rank()
		if !found || rank < bestRank || (rank == bestRank && c.Offset > best) {
			best, bestRank, found = c.Offset, rank, true
		}
	}
	return best, found
}

// premerge folds every below-minimum section into a neighbour: backwards into
// the section before it, or forwards for a leading fragment that has none.
//
// This is §5 step 1's "pre-merges below-minimum fragments so the model only
// adjudicates real boundaries". A merged section can exceed the budget — a
// tail of a dozen tokens joins a section that was already full — and that is
// the right trade: the budget is a target the taxonomy stage owns and can
// re-split against, while a 12-token leaf is a boundary decision nobody would
// have made on purpose.
func premerge(src []byte, cuts []survey.Range) []survey.Range {
	for len(cuts) > 1 {
		i := -1
		for j, c := range cuts {
			if estimate(src[c.Start:c.End]) < minTokens {
				i = j
				break
			}
		}
		if i < 0 {
			return cuts
		}
		if i == 0 {
			cuts[1].Start = cuts[0].Start
			cuts = cuts[1:]
			continue
		}
		cuts[i-1].End = cuts[i].End
		cuts = append(cuts[:i], cuts[i+1:]...)
	}
	return cuts
}

// Windows returns one clamp window per INTERIOR boundary of a cut list, in
// order — what the refinement pass shows the model and what Verify measures
// its answer against. A list with no interior boundary produces none.
//
// The window reaches overlapFraction into each neighbour, capped per side
// (overlapCapTokens). It is deliberately asymmetric: a boundary between a
// long section and a short one reaches further back than forward, because
// what the model needs is context proportional to what it is deciding
// between, and the short side simply has less of it.
func Windows(src []byte, cuts []survey.Range) []Window {
	if len(cuts) < 2 {
		return nil
	}
	out := make([]Window, 0, len(cuts)-1)
	for i := 1; i < len(cuts); i++ {
		at := cuts[i].Start
		out = append(out, Window{
			Lo: at - overlap(src, cuts[i-1]),
			Hi: at + overlap(src, cuts[i]),
		})
	}
	return out
}

// overlap is one side's reach into one section: a fraction of it, capped.
func overlap(src []byte, sec survey.Range) int {
	n := int(float64(sec.End-sec.Start) * overlapFraction)
	if n > overlapCapBytes {
		return overlapCapBytes
	}
	return n
}

// StarvedError refuses a span the enumerated candidates cannot cut under the
// budget: every legal position leaves a section too large, or the span has no
// legal position at all (a single enormous code fence, a table nobody broke
// up).
//
// It names the span rather than trimming it. Refuse-and-split is the §3
// invariant: the remedy is at the stage that decides how big a unit is, and a
// silently truncated leaf is how a verbatim-leaf discipline dies unnoticed.
type StarvedError struct {
	Span   survey.Range
	Tokens int
	Budget int
}

func (e StarvedError) Error() string {
	return fmt.Sprintf("dissect: span [%d,%d) is %d tokens against a %d-token budget and holds no legal cut "+
		"that fits; it must be re-split at the stage that sized it", e.Span.Start, e.Span.End, e.Tokens, e.Budget)
}

package skeleton

import (
	"fmt"

	"kbase/internal/tokens"
)

// Constants from ARCHITECTURE.md §9 (provisional starting values per
// TEMP_DESIGN_V020_RESHAPE; calibrated later like everything else in that
// table). They are the defaults DefaultBudgets hands out; the numbers a given
// skeleton was verified against travel in the artifact, because a consumer
// re-reading a constant would be the second source I-1 forbids.
const (
	// defaultLeafTokens is G-1: the per-leaf source budget stage 4 cuts
	// toward and stage 5 slices under. It is what makes stage 4's and stage
	// 5's per-call input bounded by construction.
	defaultLeafTokens = 4000

	// defaultSummaryInputTokens is G-2: the whole input one stage-6 summary
	// call may read — leaf children as bodies, index children at
	// defaultSummaryTokens each. It sits an order of magnitude under §9's
	// per-call target, which is the headroom the prompt scaffolding and the
	// gist coverage hints are paid out of.
	defaultSummaryInputTokens = 60000

	// defaultSummaryTokens is what one index child costs its parent: the cap
	// stage 6's own verifier holds a summary to, and therefore the honest
	// digest cost of that child at the level above.
	defaultSummaryTokens = 400

	// defaultEntryPointTokens is the entry-point's hard ceiling (§4.3), from
	// the exemplar's own taxonomy definition. It is carried here and enforced
	// at stage 9, where the rendered bytes exist to measure.
	defaultEntryPointTokens = 3000

	// defaultDepthCap is I-9: four index levels (entry-point, domain,
	// subtopic, sub-subtopic), leaves at level five. It is mechanical rather
	// than stylistic because stage 6 declares one static stage per level, and
	// an uncapped depth would make the stage list depend on an artifact that
	// does not exist at job setup.
	defaultDepthCap = 4

	// defaultFanOutCap bounds one index's children. A group of one is legal —
	// the exemplar's single-child subtopic directories are a budget artifact,
	// not a defect (§0.4).
	defaultFanOutCap = 12

	// defaultCandidateCap bounds one taxonomy call's candidate list. A
	// container with more direct children is mechanically pre-batched before
	// any call is described, so a breach reaching the verifier is OUR
	// arithmetic being wrong and not a runtime condition.
	defaultCandidateCap = 120
)

// Budgets is what a tree was verified against: the three token budgets the
// design's guarantees are stated in, plus the three structural caps.
//
// It is plain data on the marshal path — the estimator that turns bytes into
// those numbers lives on Params, not here, because an estimator is a runtime
// parameter and an artifact that carried one would be claiming a calibration
// it cannot prove.
type Budgets struct {
	LeafTokens         int `json:"leafTokens"`
	SummaryInputTokens int `json:"summaryInputTokens"`
	SummaryTokens      int `json:"summaryTokens"`
	EntryPointTokens   int `json:"entryPointTokens"`
	DepthCap           int `json:"depthCap"`
	FanOutCap          int `json:"fanOutCap"`
	CandidateCap       int `json:"candidateCap"`
}

// DefaultBudgets is the provisional operating point (§9).
func DefaultBudgets() Budgets {
	return Budgets{
		LeafTokens:         defaultLeafTokens,
		SummaryInputTokens: defaultSummaryInputTokens,
		SummaryTokens:      defaultSummaryTokens,
		EntryPointTokens:   defaultEntryPointTokens,
		DepthCap:           defaultDepthCap,
		FanOutCap:          defaultFanOutCap,
		CandidateCap:       defaultCandidateCap,
	}
}

// Validate refuses a budget set whose arithmetic cannot be discharged.
//
// Two of the checks are load-bearing rather than hygienic, and both are about
// whether §2.7's index interposition can always make progress:
//
//   - A single leaf child must fit one summary call (LeafTokens ≤
//     SummaryInputTokens). If it did not, an index with one leaf child would
//     violate G-2 with no repair available — interposition partitions
//     children, and a partition of one child changes nothing.
//   - A single index child must likewise fit (SummaryTokens ≤
//     SummaryInputTokens), for the same reason one level up.
//
// Together they guarantee every greedy batch takes at least one child, which
// is what makes the interposition operator terminate.
func (b Budgets) Validate() error {
	for _, f := range []struct {
		name string
		v    int
	}{
		{"leafTokens", b.LeafTokens},
		{"summaryInputTokens", b.SummaryInputTokens},
		{"summaryTokens", b.SummaryTokens},
		{"entryPointTokens", b.EntryPointTokens},
		{"depthCap", b.DepthCap},
		{"fanOutCap", b.FanOutCap},
		{"candidateCap", b.CandidateCap},
	} {
		if f.v <= 0 {
			return fmt.Errorf("skeleton: budget %s is %d; every budget and cap is positive", f.name, f.v)
		}
	}
	if b.LeafTokens > b.SummaryInputTokens {
		return fmt.Errorf("skeleton: leafTokens (%d) exceeds summaryInputTokens (%d); "+
			"one leaf child could then never be digested and no operator could repair it",
			b.LeafTokens, b.SummaryInputTokens)
	}
	if b.SummaryTokens > b.SummaryInputTokens {
		return fmt.Errorf("skeleton: summaryTokens (%d) exceeds summaryInputTokens (%d); "+
			"one index child could then never be digested and no operator could repair it",
			b.SummaryTokens, b.SummaryInputTokens)
	}
	return nil
}

// Params is what a skeleton is verified under: the appliance's single token
// estimator and the budgets it is measured against.
//
// It is the shape internal/dissect already uses (dissect.Params), not an
// abstraction, and for the same reason: calibration has ONE place to land
// (§8). A package that read tokens.Estimator{} at each call site would be the
// second estimation path the estimator's own doc comment exists to prevent.
type Params struct {
	// Est is the appliance's single token estimator (§8). The zero value
	// estimates at the calibrated default, so a caller with nothing to
	// calibrate leaves it alone.
	Est tokens.Estimator

	// Budgets is the operating point. The zero value is not usable —
	// NewVerifier refuses it — because a zero cap would silently pass every
	// check it appears in.
	Budgets Budgets
}

// DefaultParams is the provisional operating point with the default estimator.
func DefaultParams() Params {
	return Params{Budgets: DefaultBudgets()}
}

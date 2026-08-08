// Package tokens is the appliance's single token estimator.
//
// ARCHITECTURE.md §8: the operative counter is a calibrated chars-per-token
// heuristic — no cgo SentencePiece binding, no dependency on a `/tokenize`
// endpoint most providers do not expose. Every consumer (survey section
// sizes, taxonomy leaf budgets, per-call context budgets) is a threshold
// check with 2–3× headroom between the per-call target and the ceiling (§9),
// and refuse-and-split makes correctness independent of estimator error: an
// undercount surfaces as one loud re-split, an efficiency blip, never a wrong
// answer.
//
// It is its own package so there is exactly one estimator, and so calibration
// has exactly one place to land.
package tokens

import "math"

// DefaultCharsPerToken is the provisional gemma-4 bytes-per-token ratio
// (ARCHITECTURE.md §9, "Chars-per-token"). The constants table lists the
// value as TBD at calibration; 4.0 is the placeholder every English-ish
// subword tokenizer lands near. It is a constant rather than a config knob
// until the calibration path exists to move it (§8: tokenize-endpoint probe
// at configure time, then refinement from the `usage` counts providers return
// on every real call).
const DefaultCharsPerToken = 4.0

// Estimator converts text to an estimated token count.
//
// The ratio is a field rather than a package constant read directly at the
// call site so calibration later swaps it without a second estimation code
// path. The zero value estimates at DefaultCharsPerToken, so `var e Estimator`
// is the correct estimator until calibration exists.
type Estimator struct {
	// CharsPerToken is bytes per token. Zero or negative selects
	// DefaultCharsPerToken — a caller cannot accidentally configure a
	// divide-by-zero or a negative-token estimate.
	CharsPerToken float64
}

// Estimate returns the estimated token count of s, rounded up.
//
// Length is measured in BYTES, not runes. A multi-byte rune costs more than
// one token far more often than it costs exactly one, so bytes is both the
// closer approximation and the cheaper measurement — and the ratio is
// calibrated against real byte lengths, which folds any residual error into
// the constant rather than into a second rule here.
func (e Estimator) Estimate(s string) int {
	if s == "" {
		return 0
	}
	return int(math.Ceil(float64(len(s)) / e.ratio()))
}

func (e Estimator) ratio() float64 {
	if e.CharsPerToken > 0 {
		return e.CharsPerToken
	}
	return DefaultCharsPerToken
}

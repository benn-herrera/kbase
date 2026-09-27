package prompt

import "testing"

// Test-only exports for the EXTERNAL prompt_test package.
//
// The prefix and stability tests live in `package prompt_test` so they can
// import internal/pipeline and derive their expected frontiers from the phase
// phaseOpTable (ARCHITECTURE.md §12) — pipeline imports prompt, so an in-package test
// could not. What those tests still need from in here is the render scheme's
// own vocabulary (a helper predicting a slot's content has to join it the way
// the builder does) and the shared fixture the in-package tests already build
// against.
//
// This file is a _test.go file, so none of it is in the shipped package.

// StatusLineDelimiter and TrailerCriteriaDelimiter are the builder's join
// bytes, exported so a test predicting a slot's content builds it from the
// same constant the builder renders from.
const (
	StatusLineDelimiter      = statusLineDelimiter
	TrailerCriteriaDelimiter = trailerCriteriaDelimiter
)

// The fixture forwarders. The external tests used to carry a byte-for-byte
// copy of the in-package fixture, defended as "these tests are about which
// bytes move, not which bytes are rendered" — but the copies never diverged,
// so it was duplication with a live drift risk: change the golden fixture and
// the prefix tests keep validating the shape it used to have. Four forwarders
// cost less than the comment defending the copy did.
func BaseSpec(t *testing.T) StageSpec                              { return baseSpec(t) }
func BaseInput() CallInput                                         { return baseInput() }
func NewContext(t *testing.T, spec StageSpec) *StageContext        { return newContext(t, spec) }
func Build(t *testing.T, sc *StageContext, in CallInput) BuiltCall { return build(t, sc, in) }

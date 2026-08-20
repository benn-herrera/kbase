package dissect

import (
	"kbase/internal/model"
	"kbase/internal/pipeline"
)

// stubDefinition and stubTaskDef are PLACEHOLDER prompt text.
//
// TODO(embedded-definitions): the real gemma-4-tuned definition, its
// `## CRITICAL` section and its eval belong to the embedded-definitions burst
// (ROADMAP). What is being exercised here is the seam — window, menu,
// verifier, fallback, retry, fallback — and none of that depends on the
// wording. Keeping a stub visible and marked is honest; inventing tuned text
// nobody evaluated would look like the real thing.
const (
	stubDefinition = "# Boundary refinement\n\n" +
		"You are given the end of one section, the start of the next, and a numbered list of the\n" +
		"positions the boundary between them may be moved to.\n\n" +
		"## CRITICAL\n\n" +
		"Answer with one number from the list and nothing else.\n"

	stubTaskDef = "Task: choose the position in the numbered list that best separates the two sections."
)

// Effort is the boundary-refinement definition's DECLARED effort — the
// registration site's statement of how hard this exact ask is worth asking,
// threaded from the composing verb through NewRefiner and StagePlan to the
// wire (ARCHITECTURE.md §9, §12).
//
// Thinking is OFF. The menu choice is the narrowest ask in the pipeline — pick
// one of at most seven numbered positions, in a window the model is already
// shown — and the live A/B of 2026-08-12 measured what reasoning bought on it:
// a byte-identical cut list for 20,924 completion tokens instead of 4, two
// minutes instead of three seconds, and one MORE compliance rejection (a
// reasoning run answered boundary 2 with something other than a bare number).
// It bought nothing and cost everything. The off value is stated rather than
// defaulted so that a later ask which IS worth reasoning over has to say so on
// its own line.
var Effort = model.DeclareEffort(model.RequestEffort{Thinking: false})

// Retry is the boundary-refinement definition's DECLARED retry policy — the
// shipped attempt count and note budget, and an informed retry that asks with
// thinking OFF, exactly as the first attempt does (ARCHITECTURE.md §9, §12).
//
// This is the one definition that does not escalate, and the reason is that it
// is the one definition whose value was MEASURED rather than judged. The
// 2026-08-12 A/B put reasoning on this exact ask and it produced a
// byte-identical cut list for 20,924 completion tokens against 4 — and one MORE
// compliance rejection, a reasoning run answering a menu question with something
// other than a bare number. A retry escalation here would buy the measured
// failure mode on precisely the calls that already failed once. The other two
// definitions escalate because their asks are unmeasured and their seams have no
// fallback; this one has a valid mechanical fallback sitting under it and
// nothing to prove.
var Retry = pipeline.DeclareRetry(pipeline.RetryPolicy{Effort: Effort})

package taxonomy

import (
	"kbase/internal/model"
	"kbase/internal/pipeline"
)

// stubDefinition and stubTaskDef are PLACEHOLDER prompt text.
//
// TODO(embedded-definitions): the real gemma-4-tuned definition, its
// `## CRITICAL` section and its eval belong to the embedded-definitions burst
// (ROADMAP). What is being exercised here is the seam — the candidate list,
// the answer schema, the verifier, the retry, the composed artifact — and none
// of that depends on the wording. Keeping a stub visible and marked is honest;
// inventing tuned text nobody evaluated would look like the real thing.
//
// The one part of it that is NOT provisional is the schema: it is the contract
// parseAnswer reads, so the two move together or neither does.
const (
	stubDefinition = "# Taxonomy design\n\n" +
		"You are shown one container of a documentation corpus — the corpus itself, a folder, or a\n" +
		"document — and a numbered list of what it holds directly. Group those entries into the\n" +
		"sections and pages of a knowledge base.\n\n" +
		"A group is a `page` when its entries belong together as one page of text a reader reads.\n" +
		"A group is a `section` when its entries need a heading of their own with more below it.\n\n" +
		"Answer with one JSON object:\n\n" +
		"    {\"groups\": [{\"title\": \"…\", \"scope\": \"…\", \"kind\": \"page\", \"members\": [1, 2]}]}\n\n" +
		"- `title` — what the group is called, in the corpus's own words.\n" +
		"- `scope` — one line saying what a reader finds there.\n" +
		"- `kind` — `page` or `section`.\n" +
		"- `members` — the numbers of the entries in this group.\n\n" +
		"## CRITICAL\n\n" +
		"Every numbered entry belongs to exactly one group: no entry twice, none left out.\n" +
		"Answer with the JSON object and nothing else.\n"

	stubTaskDef = "Task: group the numbered entries of one container into the sections and pages of a knowledge base."
)

// Effort is the taxonomy definition's DECLARED effort — this stage's statement
// of how hard its exact ask is worth asking (ARCHITECTURE.md §9, §12).
//
// Thinking is OFF, and that is a judgment call rather than a measurement. The
// one A/B this appliance has run (boundary refinement, 2026-08-12) bought a
// byte-identical artifact for 20,924 completion tokens against 4, and the ask
// here is the same SHAPE — a partition of a numbered list under a stated
// post-condition, where the verifier rejects and re-asks anything that does
// not partition. It is stated on its own line precisely so the day someone
// measures it, the value moves here and nowhere else; the dev verb's A/B
// harness is where that measurement belongs.
//
// It lives beside the definition rather than at the registration site because
// the effort is a property of the ask, and the ask is this package's. A verb
// that wants the other value overrides it explicitly, and records that it did.
var Effort = model.DeclareEffort(model.RequestEffort{Thinking: false})

// Retry is the taxonomy definition's DECLARED retry policy (ARCHITECTURE.md
// §9, §12): the attempt count and note budget are the shipped defaults, and the
// informed retry asks with thinking ON.
//
// The escalation is the point, and the asymmetry with Effort above is the
// argument for it. The first attempt asks cold, where the A/B says reasoning
// buys nothing on a partition; the retry does not ask cold — it carries the
// verifier's mechanical statement of what was wrong with the last answer ("a
// group holds no entries", "entry 4 belongs to two groups"), and a rule the
// answer already broke once is exactly the kind of thing worth reasoning over
// before answering again. It is also the cheap side of the trade: an escalated
// retry costs reasoning tokens only on the calls that already failed, and what
// it buys is a container that would otherwise fail the unit, poison the lane
// and take the whole job's tree plan with it (this is a no-fallback seam).
var Retry = pipeline.DeclareRetry(pipeline.RetryPolicy{
	Effort: model.DeclareEffort(model.RequestEffort{Thinking: true}),
})

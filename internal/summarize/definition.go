package summarize

import (
	"kbase/internal/model"
	"kbase/internal/pipeline"
)

// stubDefinition and stubTaskDef are PLACEHOLDER prompt text.
//
// TODO(embedded-definitions): the real gemma-4-tuned definition, its
// `## CRITICAL` section and its eval belong to the embedded-definitions burst
// (ROADMAP). What is being exercised here is the seam — the leaf-group call and
// its card, the caps, the verifier, the level chain — and none of that depends on
// the wording.
//
// ONE definition serves both of a node's calls (summarize.go): the group of
// pages and the section above it are the same question asked of different
// material, so the body below says what it is given without naming which call
// gave it. A card-specific instruction would be a second definition to keep true.
//
// What is NOT provisional is the discipline the body states, which is §4.5
// read as constraints rather than as advice: conclusions first, no imported
// framing, hedging preserved, no audience shift, and verbatim quotation as a
// TACTIC rather than a requirement. That list is source-general on purpose —
// the exemplar's "Key Results" sections are a property of a results-dense
// maths paper, not of knowledge bases, which is why the heading is the model's
// to choose and why an empty conclusions block is a legal answer.
//
// Nor is the BOUND on the conclusions block provisional, and it is a shape
// rather than a number by ruling (2026-08-21). check() caps the framing and the
// conclusions TOGETHER at Budgets.SummaryTokens, and until this line existed
// the ask bounded the framing alone while opening with a completeness demand —
// so an entry-point invited a comprehensive conclusions block over the whole
// knowledge base and was then rejected for length, which is a rejection this
// definition engineered. The remedy states a shape a compliant answer naturally
// fits inside, in the vocabulary FRAMING already uses ("one to three
// sentences"), and states no token count anywhere: the verifier owns the
// number, the ask owns the form, and check()'s messages stay number-free for
// the reason recorded there. The calibration, which lives here and in the test
// that proves it rather than in the prompt: the worst answer the shape permits
// is three framing sentences plus four conclusions of two, eleven sentences in
// all, which at an ordinary sentence length lands inside Budgets.SummaryTokens
// with room for the estimator's own error (§8). conclusionsCap and its
// neighbours in the test state the same shape this text does, so a reworded
// bound moves both.
const (
	stubDefinition = "# Section summary\n\n" +
		"You are given what sits under one section of a knowledge base — its pages in full, or\n" +
		"summaries of what it holds. Write that section's own summary.\n\n" +
		"- Conclusions first: what this material establishes, decides or instructs.\n" +
		"- Every concept and every framing comes from the material. If you are reaching for a word\n" +
		"  the source does not use, stop.\n" +
		"- Keep the source's hedging. An open question stated as settled is a defect.\n" +
		"- No analogies the source does not make, and no shift of audience.\n" +
		"- Where the material has quotable conclusions — a rule, a signature, a configuration key —\n" +
		"  quote them. Where it does not, an accurate paraphrase is the right answer and no quote\n" +
		"  is required.\n" +
		"- If the material only points elsewhere, say so and leave the conclusions empty. An\n" +
		"  invented conclusion is worse than a missing one.\n\n" +
		"Everything around your words is already settled — the section's own title, the list of\n" +
		"what sits under it, and where all of it goes. What is open is three pieces of text.\n" +
		"Write them under these labels, in this order, each one plain text on the lines below\n" +
		"its label:\n\n" +
		"    " + labelFraming + "\n" +
		"    one to three sentences saying what this section holds\n\n" +
		"    " + labelHeading + "\n" +
		"    your own plain title for the block below, no markup\n\n" +
		"    " + labelConclusions + "\n" +
		"    the conclusions themselves, at most four and each a sentence or two,\n" +
		"    or nothing at all\n\n" +
		"There is nothing to close and nothing to escape: a block runs to the next label, and\n" +
		"the last one runs to the end of what you write.\n\n" +
		"## CRITICAL\n\n" +
		"No links and no file names: the navigation is added mechanically after you.\n" +
		"Write the three labels — " + labelFraming + " " + labelHeading + " " + labelConclusions +
		" — with your text under each.\n"

	stubTaskDef = "Task: write the framing and the conclusions of one section of a knowledge base."
)

// Effort is the summary definition's DECLARED effort (ARCHITECTURE.md §9, §12).
//
// Thinking is OFF, and it is a judgment call rather than a measurement — the
// same one the taxonomy definition makes, for a different reason. This ask is
// generative rather than combinatorial, which is the case where reasoning
// might pay; what argues against it here is consistency with the one A/B this
// appliance has measured (boundary refinement, 2026-08-12: a byte-identical
// artifact for 20,924 completion tokens against 4) and the fact that the
// definition is a stub, so a measurement taken now would be a measurement of
// the stub. The value is stated on its own line so the tuning burst's A/B
// moves it here and nowhere else.
var Effort = model.DeclareEffort(model.RequestEffort{Thinking: false})

// Retry is the summary definition's DECLARED retry policy (ARCHITECTURE.md §9,
// §12): the shipped attempt count and note budget, and an informed retry that
// asks with thinking ON.
//
// The same escalation the taxonomy definition declares, for the same reason and
// with one of its own. A rejected summary was rejected on a mechanical fact —
// over the summary cap, a link where §6.4 permits none, an empty framing — and
// the retry carries that fact, which makes it the one attempt where reasoning
// has something to reason about. What is particular here is the ask: this is
// the generative seam, where a second cold pass is most likely to reproduce the
// first, so the retry needs a reason to be different, and the escalation is it.
// A summary that fails twice fails the unit and refuses the delivery, so the
// retry is the last thing standing between one bad answer and no knowledge base.
//
// The escalated attempt states its own completion window, because thinking is
// spent out of the answer's (model.ThinkingMaxTokens). Inheriting the first
// attempt's window made the attempt asked to work hardest the attempt with
// least room to answer, and on 2026-08-18 it returned one byte — the whole of
// TEMP_INVESTIGATION_EMPTYRETRY.
var Retry = pipeline.DeclareRetry(pipeline.RetryPolicy{
	Effort: model.DeclareEffort(model.RequestEffort{
		Thinking:  true,
		MaxTokens: model.ThinkingMaxTokens,
	}),
})

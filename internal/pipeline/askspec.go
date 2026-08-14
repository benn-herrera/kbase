package pipeline

import (
	"fmt"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/prompt"
)

// StageConstantFrontier reports the last slot that holds its bytes for the
// whole life of a stage: slots 1–3, the job frame, the agent definition
// and the task definition (ARCHITECTURE.md §7 slot layout).
//
// It is DERIVED from the phaseOpTable's section-transition row rather than spelled
// again beside it, because it is that row seen from the other side: a section
// transition flushes the stage reference buffer, which is slot 4, so what
// survives one is exactly what is constant for the stage. Two spellings
// reconciled by a test is strictly worse than one spelling — the phaseOpTable is
// THE table (§12), and this reads it.
func StageConstantFrontier() (prompt.Slot, error) {
	return StabilityFrontier(PhaseSectionTransition)
}

// StageConstantSlots returns the stage-constant slots in render order — the
// R-1 cross-worker assert's subject: every worker of a stage must hash these
// identically to the values captured when the stage's context was built.
//
// It is derived by numeric filter rather than enumerated. A slot's number IS
// its position in the stack — prompt states that as public contract, and
// CheckStability already relies on it — so "at or below the frontier" is the
// whole definition, and a slot added to the stack joins or does not join by
// construction rather than by someone remembering a second list.
func StageConstantSlots() ([]prompt.Slot, error) {
	frontier, err := StageConstantFrontier()
	if err != nil {
		return nil, err
	}
	all := prompt.AllSlots()
	out := make([]prompt.Slot, 0, len(all))
	for _, s := range all {
		if s <= frontier {
			out = append(out, s)
		}
	}
	return out, nil
}

// jobFrame is slot 1 rendered once, at job setup, with the hash every call of
// every stage is measured against.
//
// §7 calls slot 1 job-constant, and until now nothing checked it: a worker's
// previous-call hashes are reset at the start of every lane, so the first
// call of every stage claims nothing and slot 1 was never compared across a
// stage boundary. One Plan.JobFrame makes divergence unrepresentable by
// construction; this hash is what catches the case construction cannot — a
// builder change that lets a per-call field bleed into slot 1, which would
// re-render it identically at stage setup and differently on the wire.
type jobFrame struct {
	text string
	hash [32]byte
}

// newJobFrame renders slot 1 through the real builder and captures its hash.
//
// Through the builder, not by hashing the string: the canonical hash has to
// be what the builder actually renders, normalization and all, or every
// comparison against it would be comparing two different things and firing on
// the difference.
func newJobFrame(text string) (jobFrame, error) {
	sc, err := prompt.NewStageContext(prompt.StageSpec{JobFrame: text})
	if err != nil {
		return jobFrame{}, fmt.Errorf("pipeline: rendering the job frame: %w", err)
	}
	built, err := sc.Build(prompt.CallInput{})
	if err != nil {
		return jobFrame{}, fmt.Errorf("pipeline: rendering the job frame: %w", err)
	}
	return jobFrame{text: text, hash: built.Hashes[prompt.SlotJobFrame]}, nil
}

// Verifier is the propose-and-verify seam made concrete (ARCHITECTURE.md §12):
// the model returns text claiming to be data, and this parses it and checks the
// stage's mechanical post-condition. It returns a typed artifact or the reason
// the response is not one. Raw model text never escapes the runner.
//
// unit is the store path of the unit being verified — the same string the
// stage's LaneTask named. A stage whose post-condition is the same for every unit
// ignores it; a stage whose post-condition is per-unit (stage 4 checks a cut
// against THIS boundary's candidate menu and clamp window) looks the unit up
// in a table its ask spec built when the stage's work was described. That keeps
// the AskSpec stage-constant, which is what lets every worker share one: the
// table is fixed for the stage, and the unit is what selects a row.
//
// # What a verifier may and may not do
//
// Its VERDICT must be a function of (artifactPath, response, the stage's state as of
// the call) — nothing else. Same three, same answer, on the retry and in a
// test.
//
// It MAY fold its result into the stage's own state; a verifier is allowed to
// be an accumulator. Stage 4's fold is the case: each boundary's accepted
// choice becomes the working cut list the next boundary's window, menu and
// verification derive from, and the last one composes the artifact
// (ARCHITECTURE.md §5). What it may never write is the STORE — the artifact
// reaches disk through the runner and the encoder, once, on the attempt that
// won.
//
// A stage that accumulates depends on guarantees the runner makes, so they
// are stated here rather than discovered:
//
//   - Verify runs ONCE per model attempt, and never over a stored
//     response: an attempt is a fresh call.
//   - Verify never runs concurrently for one stage's lane. A lane is one
//     worker's serial assignment, so the fold has one writer by construction
//     (dissect.Refiner asserts it as well, at the seam that depends on it).
//   - A rejected attempt must leave the stage state as it found it, which is
//     what makes the single informed retry re-ask the SAME question.
//
// A verifier that concludes the failure is OURS rather than the model's wraps
// ErrVerifierDefect; see it for what the runner then does.
type Verifier func(artifactPath, response string) (artifact any, err error)

// MechanicalFallback returns the artifact a fallback-backed seam falls back to
// — the mechanically produced result the model was asked to improve on, which
// was already valid (§3 monotone safety). A AskSpec that has one is a
// fallback-backed seam; an AskSpec that does not is a no-fallback seam.
//
// It takes the unit for the same reason Verifier does: the mechanical result
// a boundary falls back to is that boundary's own cut, not the stage's.
//
// Like Verify it may touch the stage's own state and not the store, and the
// runner's guarantee is narrower: a MechanicalFallback is called AT MOST ONCE
// per unit, and only after that unit's model attempts are exhausted. Stage 4's
// fold relies on both — its fallback counts the fallback and, for the last
// boundary, composes the artifact the stage still owes.
type MechanicalFallback func(artifactPath string) (artifact any)

// Encoder renders a verified artifact as the bytes the store keeps. It is
// separate from the verifier because the two run at different times: the
// verifier runs on every attempt, the encoder once, on the artifact that won.
type Encoder func(artifact any) ([]byte, error)

// InferenceSeam classifies what happens when the model cannot produce a
// verifiable answer. It is derived from the AskSpec, never set: the presence of a
// mechanical fallback IS the classification, so the two cannot disagree.
type InferenceSeam string

const (
	// FallbackBackedSeam is a seam where the model improves an already-valid
	// mechanical result. Exhausting the retries keeps the fallback and marks
	// the unit as fallen back: model failure costs quality, never correctness.
	FallbackBackedSeam InferenceSeam = "refinement"
	// NoFallbackSeam is a seam where inference was chosen because there is no
	// mechanical answer — taxonomy, summaries, format translation. There is
	// no fallback by definition, so exhausting the retries fails the unit and
	// the job refuses emission at assembly.
	NoFallbackSeam InferenceSeam = "essential"
)

// AskSpec is one stage's model-facing construct: what the model is told, which
// tier answers, how the answer is checked, and what happens when it does not
// check out. It carries no per-call state — one AskSpec serves every unit of its
// stage and every worker running them.
type AskSpec struct {
	// Def is the parsed agent definition: slot 2 and the trailer's source
	// text. The AskSpec owns it, so newBoundAsk overwrites whatever the StageSpec
	// carried — one definition per ask, not two places to set it.
	Def prompt.Definition

	// Tier is config.TierHeavy or config.TierLight. The pipeline names a
	// tier and never a model id; which gemma-4 variant serves it is config's
	// business (§10).
	Tier string

	// Effort is what this definition asks of the model — thinking today,
	// temperature next (model.RequestEffort). It sits on the AskSpec because the AskSpec
	// IS the definition here: one exact ask, one declared effort, and every
	// call of the stage inherits it because every call asks that question.
	//
	// It is required. A stage registers its ask by literal, so this is the
	// one hop the positional parameters on NewRefiner and DefaultRequest
	// cannot make mandatory — validate refuses an undeclared value rather
	// than letting a forgotten field read as a deliberate "no thinking".
	Effort model.RequestEffort

	// Verify is the mechanical post-condition. Required.
	Verify Verifier

	// Encode renders the winning artifact for the store. Required.
	Encode Encoder

	// Fallback is the mechanical answer kept when the model's does not
	// verify. Non-nil makes this a fallback-backed seam; nil makes it a
	// no-fallback seam (R-4).
	Fallback MechanicalFallback
}

// Seam reports which failure policy this ask gets.
func (r AskSpec) Seam() InferenceSeam {
	if r.Fallback != nil {
		return FallbackBackedSeam
	}
	return NoFallbackSeam
}

// validate refuses an ask that cannot run before a stage is built around it. A
// missing verifier is the one that matters: without it, raw model text would
// reach the store, which is the propose-and-verify invariant inverted.
func (r AskSpec) validate() error {
	switch {
	case r.Verify == nil:
		return fmt.Errorf("pipeline: this ask has no verifier; the model's output would reach the store unchecked")
	case r.Encode == nil:
		return fmt.Errorf("pipeline: this ask has no encoder; its artifact could not be written")
	case r.Tier != config.TierHeavy && r.Tier != config.TierLight:
		return fmt.Errorf("pipeline: this ask's tier %q is neither %s nor %s", r.Tier, config.TierHeavy, config.TierLight)
	case !r.Effort.Declared():
		return fmt.Errorf("pipeline: this ask declares no effort; every definition states one for its exact ask (model.DeclareEffort)")
	}
	return nil
}

// boundAsk is the §3 Go-side construct: one ask bound to one immutable
// StageContext, shared by every worker of the stage. It is never an LLM-driven
// loop — it makes one-shot calls and nothing else.
//
// Sharing one context across the stage's workers is what makes the
// stage-constant slots identical across workers BY CONSTRUCTION (§12, workers
// and frontiers), and one Plan.JobFrame does the same for slot 1 across
// stages. The canonical hashes below are what catches the failures
// construction cannot see; see checkCanonicalHashes for what those are.
type boundAsk struct {
	ask             AskSpec
	ctx             *prompt.StageContext
	slots           []prompt.Slot
	canonicalHashes map[prompt.Slot][32]byte
}

// newBoundAsk freezes a stage's context around an ask and the job's job frame,
// and captures the canonical stage-constant slot hashes.
//
// Slot 1 comes from the job, not from the spec — the spec's own JobFrame is
// overwritten the same way its AgentDef is, because §7 calls slot 1
// job-constant and one place to set it is the only way to say that which
// cannot be contradicted. Its canonical hash is the JOB's, so every stage's
// calls are measured against the same bytes rather than against their own
// stage's re-render.
//
// The stage-constant hashes come from a probe Build rather than from hashing
// the spec's strings, for the same reason newJobFrame renders rather than
// hashes. The probe doubles as a dev-time gate — a definition whose CRITICAL
// section is over cap fails here, at stage setup, instead of on every call the
// stage makes — and it is where a stage that somehow renders slot 1 differently
// from the job is refused, one loud failure per stage instead of one per call.
func newBoundAsk(ask AskSpec, spec prompt.StageSpec, frame jobFrame) (*boundAsk, error) {
	if err := ask.validate(); err != nil {
		return nil, err
	}
	slots, err := StageConstantSlots()
	if err != nil {
		return nil, err
	}
	spec.AgentDef = ask.Def
	spec.JobFrame = frame.text
	sc, err := prompt.NewStageContext(spec)
	if err != nil {
		return nil, err
	}
	probe, err := sc.Build(prompt.CallInput{})
	if err != nil {
		return nil, fmt.Errorf("pipeline: stage context does not build its own constant slots: %w", err)
	}
	canonicalHashes := make(map[prompt.Slot][32]byte, len(slots))
	for _, s := range slots {
		canonicalHashes[s] = probe.Hashes[s]
	}
	if canonicalHashes[prompt.SlotJobFrame] != frame.hash {
		return nil, StageContextMismatchError{Slot: prompt.SlotJobFrame}
	}
	// The job's value, not the probe's — identical here, and the point is
	// that every later comparison is against the JOB.
	canonicalHashes[prompt.SlotJobFrame] = frame.hash
	return &boundAsk{ask: ask, ctx: sc, slots: slots, canonicalHashes: canonicalHashes}, nil
}

// Seam reports the bound ask's failure policy.
func (a *boundAsk) Seam() InferenceSeam { return a.ask.Seam() }

// checkCanonicalHashes asserts that a built call's stage-constant slots are the
// values captured at setup — slot 1 against the JOB's canonical hash, slots 2–3
// against this stage's. It costs three array comparisons per call, which is
// why it runs on every call rather than behind a flag.
//
// What it catches is narrower than "a worker holding the wrong context", which
// one shared StageContext per stage already makes unrepresentable. It catches
// two things that construction cannot:
//
//   - A build that renders a stage-constant slot differently from the probe —
//     a prompt.Build regression bleeding a per-call field into slots 1–3. The
//     probe builds with an empty CallInput and every real call does not, so
//     this comparison is the one place that difference would show.
//   - Slot 1 drifting across a stage boundary. The per-worker churn tripwire
//     cannot see that: a worker's previous-call hashes are reset at the start
//     of every lane, so its first call of every stage claims nothing.
func (a *boundAsk) checkCanonicalHashes(got map[prompt.Slot][32]byte) error {
	for _, s := range a.slots {
		h, ok := got[s]
		if !ok {
			return StageContextMismatchError{Slot: s}
		}
		if h != a.canonicalHashes[s] {
			return StageContextMismatchError{Slot: s}
		}
	}
	return nil
}

// StageContextMismatchError reports a call whose stage-constant slots are not
// the canonical hashes — see boundAsk.checkCanonicalHashes for what that
// means in practice. It is distinct from prompt.ErrSlotChurn because the diagnosis
// differs: churn is one worker's own bytes moving between its calls, while
// this is a call disagreeing with the values the job and the stage froze,
// which a worker perfectly consistent with itself would sail straight past.
type StageContextMismatchError struct {
	Slot prompt.Slot
}

func (e StageContextMismatchError) Error() string {
	return fmt.Sprintf("pipeline: %s does not match the canonical bytes captured for it; "+
		"this call was not built from the job's frame and this stage's context", e.Slot)
}

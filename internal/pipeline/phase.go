// Package pipeline is the orchestration layer: it runs the fixed stage
// pipeline (ARCHITECTURE.md §4) and owns the machinery that makes a run safe
// to interrupt (§12). "Agent" here is the §3 Go-side construct — a role with
// an embedded definition, slot-built context and one-shot calls — never an
// LLM-driven loop.
//
// Three separable pieces live here:
//
//   - The phase matrix (this file): ONE data table mapping each execution
//     phase to the operations it permits and the prompt-stability frontier it
//     declares. It is the single source §7 promises: runtime op-gating, the
//     per-call churn tripwire, and the prefix-stability tests all read this
//     table rather than each restating the policy.
//   - Artifact custody (store.go): a job-dir-rooted store whose every write
//     is atomic and chain-stamped, so an artifact's validity is decidable by
//     inspection with no knowledge of how the previous run died.
//   - Resume (resume.go, lock.go): a scan that finds the deepest provably
//     valid prefix of the stage chain, under a single-writer lock. Resume is
//     an optimization, never load-bearing — ModeFresh ignores every prior
//     output and is always sufficient.
package pipeline

import (
	"errors"
	"fmt"

	"kbase/internal/log"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/prompt"
)

// Phase is one position in a worker's execution cycle:
//
//	JobSetup → StageSetup → CallLoop ⇄ SectionTransition → StageTeardown
//
// The phase is what the matrix is keyed on, so it is the one piece of
// execution state every gated operation has to name.
type Phase int

const (
	// PhaseJobSetup runs once per job: the lock is taken, the store is
	// scanned, and the job-constant system frame (slot 1) is rendered.
	PhaseJobSetup Phase = iota + 1
	// PhaseStageSetup builds the stage's one shared immutable
	// StageContext — the construction that makes the stage-constant slots
	// identical across every worker of the stage (§12, workers and
	// frontiers).
	PhaseStageSetup
	// PhaseCallLoop is the per-unit steady state: build a call, run it,
	// write what it produced, advance.
	PhaseCallLoop
	// PhaseSectionTransition is the only phase in which reference buffer A
	// may be flushed. That restriction is what makes the §7 slot ordering
	// safe rather than a loss: slot 4 outranks the per-call status block
	// only while it churns at section boundaries and not per call.
	PhaseSectionTransition
	// PhaseStageTeardown closes the stage: the stage-level artifact is
	// written and the stage context is dropped.
	PhaseStageTeardown
)

// Op is a gated operation. The set is deliberately small: an operation earns
// a place here by being able to invalidate the prefix cache or to change what
// is on disk, which is the same as saying the matrix is the list of things
// that can go quietly wrong.
type Op int

const (
	// OpBuildSystemFrame renders slot 1. It is job-constant by
	// construction (§7 layer 1) and job-constant by gating here: rendering
	// it a second time mid-job would move the frontier to the floor for
	// every call that followed.
	OpBuildSystemFrame Op = iota + 1
	// OpResumeScan reads the store to derive the worklist. Gating it to
	// job setup is what keeps the worklist stateless-but-stable: it is
	// derived once from the outputs on disk, not re-derived mid-run
	// against a store the run itself is mutating.
	OpResumeScan
	// OpRebuildStageContext constructs a fresh StageContext. Types cannot
	// catch this one — building a new immutable context with different
	// bytes is perfectly legal Go and a churn bug everywhere but stage
	// setup (§7 layer 2: the risk is identity across instances, not
	// mutation of one).
	OpRebuildStageContext
	// OpBuildCall renders a call from the slot stack.
	OpBuildCall
	// OpAdvanceUnit moves the worker to the next unit of work, which is
	// what churns the per-call slots.
	OpAdvanceUnit
	// OpFlushRefA replaces the multi-call reference buffer. Legal in
	// exactly one phase; see PhaseSectionTransition.
	OpFlushRefA
	// OpWriteArtifact puts a unit or stage artifact into the store.
	OpWriteArtifact
)

// opNames is the ops' one enumeration: String reads it, and the matrix
// completeness test iterates it. The const block above is the other half of
// the same declaration — a new op added there and forgotten here has no name,
// which is loud — but a switch would have let a new op be forgotten by the
// very test that exists to notice, since the test's own list was a third
// spelling. The spellings match the identifiers so a message greps back to
// its call site.
var opNames = map[Op]string{
	OpBuildSystemFrame:    "build-system-frame",
	OpResumeScan:          "resume-scan",
	OpRebuildStageContext: "rebuild-stage-context",
	OpBuildCall:           "build-call",
	OpAdvanceUnit:         "advance-unit",
	OpFlushRefA:           "flush-ref-a",
	OpWriteArtifact:       "write-artifact",
}

// phaseCrashpointPrefix names the phase-entry kill points, one per phase, so
// a resume harness can kill a run at every phase boundary by enumerating
// crashpoint.RegisteredNames rather than by knowing this package.
const phaseCrashpointPrefix = "pipeline.phase.enter."

// phaseRule is one row of the matrix. The frontier and the op set live in the
// same row because they are the same policy seen from two sides: the ops a
// phase permits are exactly the ones that cannot break the stability it
// declares.
type phaseRule struct {
	phase    Phase
	name     string
	frontier prompt.Slot
	ops      []Op
	crash    string
}

// matrix is THE table (ARCHITECTURE.md §12, "Phase matrix"). It is a slice in
// phase order rather than a map so that every enumeration over it — tests,
// coverage gates, documentation — is deterministic without sorting.
//
// Reading the frontier column: a frontier is the claim "slots 1..frontier are
// byte-identical to the previous call". Only PhaseCallLoop builds calls, so
// the other rows' frontiers are the claim a call built AFTER passing through
// that phase may still make. That is why they descend where the phase
// invalidates something: a section transition flushed slot 4, so the first
// call after it may only claim through slot 3; a stage boundary rebuilt slots
// 2 and 3, so only slot 1 survives; and job setup, having no previous call at
// all, claims nothing. LowestFrontier composes them for a traversal.
var matrix = [...]phaseRule{
	{
		phase:    PhaseJobSetup,
		name:     "job-setup",
		frontier: prompt.SlotTotal,
		ops:      []Op{OpBuildSystemFrame, OpResumeScan},
		crash:    crashpoint.Register(phaseCrashpointPrefix + "job-setup"),
	},
	{
		phase:    PhaseStageSetup,
		name:     "stage-setup",
		frontier: prompt.SlotSystemFrame,
		ops:      []Op{OpRebuildStageContext},
		crash:    crashpoint.Register(phaseCrashpointPrefix + "stage-setup"),
	},
	{
		phase:    PhaseCallLoop,
		name:     "call-loop",
		frontier: prompt.SlotRefA,
		ops:      []Op{OpBuildCall, OpAdvanceUnit, OpWriteArtifact},
		crash:    crashpoint.Register(phaseCrashpointPrefix + "call-loop"),
	},
	{
		phase:    PhaseSectionTransition,
		name:     "section-transition",
		frontier: prompt.SlotTaskDef,
		ops:      []Op{OpFlushRefA, OpAdvanceUnit},
		crash:    crashpoint.Register(phaseCrashpointPrefix + "section-transition"),
	},
	{
		phase: PhaseStageTeardown,
		name:  "stage-teardown",
		// The stage context is being dropped, so nothing stage-scoped
		// survives. Slot 1 is job-constant and does survive, and this row
		// says so — but what ENFORCES that across the boundary is not this
		// frontier: a worker's previous-call hashes are reset at every
		// stream start, so no call ever compares slot 1 against the last
		// stage's call. The job-constant canonical does that (see
		// jobFrame and Agent.checkCanonical); this row is the composable
		// claim for LowestFrontier, not the tripwire.
		frontier: prompt.SlotSystemFrame,
		// RESERVED, 2026-08-09: no code path writes a stage-level artifact
		// yet. It is here for the stage-level roll-up a stage with a
		// whole-stage output will need (§4's roll-up stages), and it MUST
		// be deleted if nothing has wired it after about a week of
		// development — a matrix cell nothing exercises reads like enforced
		// policy, and dropping it leaves this phase with zero ops, which
		// TestMatrixCompleteness rejects, so the deletion is a decision
		// rather than a quiet edit.
		ops:   []Op{OpWriteArtifact},
		crash: crashpoint.Register(phaseCrashpointPrefix + "stage-teardown"),
	},
}

// rule returns the matrix row for p.
func rule(p Phase) (phaseRule, bool) {
	for _, r := range matrix {
		if r.phase == p {
			return r, true
		}
	}
	return phaseRule{}, false
}

// String names the phase as it appears in log records and error messages.
func (p Phase) String() string {
	if r, ok := rule(p); ok {
		return r.name
	}
	return fmt.Sprintf("phase(%d)", int(p))
}

// String names the operation as it appears in error messages.
func (o Op) String() string {
	if name, ok := opNames[o]; ok {
		return name
	}
	return fmt.Sprintf("op(%d)", int(o))
}

// guard reports whether op is permitted in phase p, and is the runtime half
// of the matrix: every gated operation calls it and treats a non-nil result
// as fatal.
//
// A refusal is a defect class, not a condition to recover from. The
// operations gated here do not fail — they succeed at doing the wrong thing,
// and the damage (a halved cache hit rate, a stale artifact written under a
// fresh stamp) shows up far from the cause. Returning an error rather than
// panicking keeps the failure on the caller's own error path, where a worker
// can abort itself and let its siblings drain.
func guard(p Phase, o Op) error {
	r, ok := rule(p)
	if !ok {
		return UnknownPhaseError{Phase: p}
	}
	for _, allowed := range r.ops {
		if allowed == o {
			return nil
		}
	}
	return OpNotAllowedError{Phase: p, Op: o}
}

// Frontier reports the prompt-stability frontier p declares — the slot
// through which a call built in (or after) this phase must hash identically
// to the previous call. It is what the caller hands prompt.CheckStability.
//
// An unknown phase is an error rather than a permissive zero: a frontier that
// quietly degraded to "nothing is stable" would disarm the tripwire at
// exactly the moment the orchestrator had lost track of its own state.
func Frontier(p Phase) (prompt.Slot, error) {
	r, ok := rule(p)
	if !ok {
		return prompt.SlotTotal, UnknownPhaseError{Phase: p}
	}
	return r.frontier, nil
}

// errNoPhases reports a frontier asked for over no phases at all.
var errNoPhases = errors.New("pipeline: frontier over an empty phase traversal")

// lowestFrontier reports the strongest stability claim that survives a
// traversal — the minimum frontier over every phase passed through since the
// previous call was built, including the phase the call is being built in.
//
// It exists because the honest frontier for a given call is not a property of
// where the worker is now but of where it has been: a worker that ran
// [CallLoop, SectionTransition, CallLoop] flushed slot 4 in the middle, so
// its next call may claim only through slot 3 even though it is back in the
// call loop. Taking the minimum is safe in the one direction that matters —
// it can only under-claim stability, never assert a prefix the phases already
// invalidated.
func lowestFrontier(phases ...Phase) (prompt.Slot, error) {
	if len(phases) == 0 {
		return prompt.SlotTotal, errNoPhases
	}
	lowest := prompt.Slot(-1)
	for _, p := range phases {
		f, err := Frontier(p)
		if err != nil {
			return prompt.SlotTotal, err
		}
		if lowest < 0 || f < lowest {
			lowest = f
		}
	}
	return lowest, nil
}

// enterPhase records a worker's arrival in a phase: one debug record for
// forensics, and the phase's registered crashpoint.
//
// It is the single declaration site for "I am now in phase p", which is what
// gives every phase boundary a kill point without each caller knowing the
// crashpoint vocabulary. It holds no state — the caller owns its own current
// phase, because in a domain-stream worker model there is no single current
// phase to hold.
func enterPhase(p Phase, lg log.Logger) error {
	r, ok := rule(p)
	if !ok {
		return UnknownPhaseError{Phase: p}
	}
	lg.Debug("pipeline phase entered", "phase", r.name, "frontier", r.frontier.String())
	crashpoint.At(r.crash)
	return nil
}

// OpNotAllowedError reports an operation the current phase does not permit
// (ARCHITECTURE.md §12). It is a kbase defect: the orchestrator asked for
// something the phase matrix forbids, which means its state and its actions
// have diverged. Errors here are value types — match with errors.As over a
// value target, as in internal/prompt.
type OpNotAllowedError struct {
	Phase Phase
	Op    Op
}

func (e OpNotAllowedError) Error() string {
	return fmt.Sprintf("pipeline: operation %s is not allowed in phase %s", e.Op, e.Phase)
}

// UnknownPhaseError reports a phase value the matrix has no row for. The
// matrix is the single source, so a phase outside it is not an unsupported
// case — it is a value that was never a phase.
type UnknownPhaseError struct {
	Phase Phase
}

func (e UnknownPhaseError) Error() string {
	return fmt.Sprintf("pipeline: no phase matrix row for %s", e.Phase)
}

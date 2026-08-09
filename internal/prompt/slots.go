package prompt

import "fmt"

// Slot identifies one position in the per-call slot stack (ARCHITECTURE.md
// §7, "Per-call slot layout"). Slots are ordered most-stable first, which is
// simultaneously cache-optimal (prefix caching pays for byte-identical
// leading tokens, so the lowest-numbered slot a call mutates is where reuse
// ends) and attention-safe (the stable slots sit in the primacy zone, the
// trailer in the recency zone, and only the middle buffers hold at-risk
// positions).
type Slot int

const (
	// SlotSystemFrame is job-constant: process framing plus job-stable
	// specifics, rendered once at job start. The builder receives it as
	// final bytes; ordering within it (universal static text first,
	// job-interpolated lines last) is the orchestrator's template concern.
	SlotSystemFrame Slot = iota + 1
	// SlotAgentDef is stage-constant: the stage's definition, rendered
	// exactly as authored — including its `## CRITICAL` section, which is
	// the primacy half of the dual render.
	SlotAgentDef
	// SlotTaskDef is task-constant: what this agent is working on.
	SlotTaskDef
	// SlotRefA is the multi-call reference buffer: orchestrator-curated
	// material stable across several calls, flushed on stage/section
	// transitions. It sits ABOVE the per-call status block because it is the
	// bulkier of the two and the more stable: a status line churning every
	// call must not re-prefill a cross-file listing. That ordering is only
	// worth its position while the flush-at-transition policy holds — RefA
	// churning per call would move the frontier up instead.
	SlotRefA
	// SlotTaskStatus is per-call: checklist/status lines the caller has
	// already ordered most→least stable, so the fastest-churning line is
	// last and the cache holds through everything above it.
	SlotTaskStatus
	// SlotContent is the per-call content buffer: the source span, child
	// summaries, or prior output under work.
	SlotContent
	// SlotRefB is the per-call transient reference buffer: curated material
	// specific to the current content.
	SlotRefB
	// SlotReminder is the trailer. It is DERIVED — no CallInput field sets
	// it — because §7 layer 1 puts slot 8 beyond direct reach of the call
	// loop by construction.
	SlotReminder
)

// SlotTotal is not a slot. It is the aggregate key in BuiltCall.EstTokens and
// the Slot an ErrOverBudget carries when the whole call, rather than one
// slot, is over budget.
const SlotTotal Slot = 0

// allSlots is the render order, which is also ascending slot order — the two
// are the same list because a slot's number IS its position, and
// CheckStability walks it to a frontier on that basis. Every map BuiltCall
// returns is keyed over exactly this set, so an empty slot has an entry (a
// hash, an offset) rather than a hole a caller could misread as "unchanged".
var allSlots = [...]Slot{
	SlotSystemFrame, SlotAgentDef, SlotTaskDef, SlotRefA,
	SlotTaskStatus, SlotContent, SlotRefB, SlotReminder,
}

// String renders a slot for error messages: number and name, no content.
func (s Slot) String() string {
	switch s {
	case SlotTotal:
		return "call total"
	case SlotSystemFrame:
		return "slot 1 (system frame)"
	case SlotAgentDef:
		return "slot 2 (agent definition)"
	case SlotTaskDef:
		return "slot 3 (task definition)"
	case SlotRefA:
		return "slot 4 (reference buffer A)"
	case SlotTaskStatus:
		return "slot 5 (task status)"
	case SlotContent:
		return "slot 6 (content buffer)"
	case SlotRefB:
		return "slot 7 (reference buffer B)"
	case SlotReminder:
		return "slot 8 (reminder trailer)"
	default:
		return fmt.Sprintf("slot %d (unknown)", int(s))
	}
}

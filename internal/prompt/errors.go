package prompt

import (
	"fmt"
	"strings"
)

// Errors in this package never carry slot content. A slot may hold tens of
// thousands of bytes of corpus text, and an error that dumps it is unreadable
// in a log and useless to grep. They carry identity (which slot, which
// placeholder) and arithmetic (estimate versus budget) instead; the caller
// already holds the bytes.
//
// All are value types: match with errors.As over a value target.

// ErrOverBudget is the refuse-and-split signal (ARCHITECTURE.md §3, §7): a
// slot, or the whole call, exceeds the budget the orchestrator set. Slot is
// SlotTotal when the ceiling was the limit breached. The orchestrator maps
// this back to the tree plan and splits the unit; nothing is ever truncated.
type ErrOverBudget struct {
	Slot     Slot
	Estimate int
	Budget   int
}

func (e ErrOverBudget) Error() string {
	if e.Slot == SlotTotal {
		return fmt.Sprintf("prompt: call estimate %d tokens over ceiling %d; split the unit",
			e.Estimate, e.Budget)
	}
	return fmt.Sprintf("prompt: %s estimate %d tokens over budget %d; split the unit",
		e.Slot, e.Estimate, e.Budget)
}

// ErrCriticalOverCap reports an authored `## CRITICAL` section over its
// reserved share of the §9 trailer cap. It comes from ValidateDefinition (the
// dev-time gate) and from Build (the same check at runtime, which covers
// user-adapted definition copies that never passed through the gate). The
// definition is at fault: nothing the orchestrator does at call time can fix
// it, which is exactly why it is a different type from ErrCriteriaOverCap.
type ErrCriticalOverCap struct {
	Words int
	Cap   int
}

func (e ErrCriticalOverCap) Error() string {
	return fmt.Sprintf("prompt: authored CRITICAL section is %d words, cap is %d", e.Words, e.Cap)
}

// ErrCriteriaOverCap reports injected acceptance criteria over their reserved
// share of the §9 trailer cap. This one IS the caller's to fix: the criteria
// are per-call data the orchestrator assembles, so it can shorten them or
// split the unit that needed so many.
type ErrCriteriaOverCap struct {
	Words int
	Cap   int
}

func (e ErrCriteriaOverCap) Error() string {
	return fmt.Sprintf("prompt: injected acceptance criteria are %d words, cap is %d", e.Words, e.Cap)
}

// ErrReservedBudgetKey reports a per-slot budget keyed by something that is
// not a slot — today only SlotTotal, whose budget is CallCeiling. Refused at
// construction rather than ignored at build time: a budget the caller set and
// the builder never enforces is worse than no budget at all.
type ErrReservedBudgetKey struct {
	Slot Slot
}

func (e ErrReservedBudgetKey) Error() string {
	return fmt.Sprintf("prompt: %s is not a per-slot budget key; the whole-call budget is CallCeiling", e.Slot)
}

// ErrMultipleCritical reports a definition carrying more than one
// `## CRITICAL` section. Exactly one is allowed: two sections mean the author
// does not know which constraints are the stage's characteristic failure, and
// the trailer cannot silently pick.
type ErrMultipleCritical struct {
	Count int
}

func (e ErrMultipleCritical) Error() string {
	return fmt.Sprintf("prompt: definition has %d `%s` sections, exactly one is allowed",
		e.Count, criticalHeading)
}

// ErrBadPlaceholder reports a `{{` sequence that is not a well-formed
// placeholder. Definitions are markdown full of literal braces, so single
// braces pass through untouched and only the doubled form is reserved — but a
// malformed doubled form is a collision (a Go-template example, a stray
// brace), and a collision must fail loudly at dev time rather than reach a
// model as literal `{{`.
//
// Snippet is a short bounded excerpt starting at the offending sequence —
// enough to locate it, never enough to dump content.
type ErrBadPlaceholder struct {
	Snippet string
}

func (e ErrBadPlaceholder) Error() string {
	return fmt.Sprintf("prompt: malformed %s...%s placeholder near %q",
		placeholderOpen, placeholderClose, e.Snippet)
}

// ErrInterpolate reports placeholder/variable mismatch in both directions at
// once: placeholders the caller supplied no value for, and values the
// template never used. Both are loud because a silent partial fill is how a
// prompt drifts from what its author believes it says, and reporting both
// directions together means one fix pass rather than two.
type ErrInterpolate struct {
	Unknown []string // placeholders in the template with no supplied value
	Unused  []string // supplied values no placeholder consumed
}

func (e ErrInterpolate) Error() string {
	var parts []string
	if len(e.Unknown) > 0 {
		parts = append(parts, "unfilled placeholders: "+strings.Join(e.Unknown, ", "))
	}
	if len(e.Unused) > 0 {
		parts = append(parts, "unused variables: "+strings.Join(e.Unused, ", "))
	}
	return "prompt: interpolate: " + strings.Join(parts, "; ")
}

// ErrSlotChurn is the churn tripwire firing (§7 layer 3): a slot the current
// phase declares stable changed bytes between calls. It invalidates the
// prefix cache from that slot onward and is a defect in the orchestrator, not
// a condition to recover from.
type ErrSlotChurn struct {
	Slot Slot
}

func (e ErrSlotChurn) Error() string {
	return fmt.Sprintf("prompt: %s changed between calls but is declared stable for this phase", e.Slot)
}

// ErrMissingHash reports a stability check against a hash map that does not
// carry the slot. BuiltCall.Hashes always carries every slot, so this can
// only come from a hand-assembled map — reported rather than skipped, because
// a missing entry compared as a zero value would pass silently and disarm the
// tripwire.
type ErrMissingHash struct {
	Slot Slot
}

func (e ErrMissingHash) Error() string {
	return fmt.Sprintf("prompt: stability check: no hash recorded for %s", e.Slot)
}

// ErrNoPreviousHashes reports a stability check with a frontier to enforce
// and no previous call to compare against. It is loud rather than a silent
// pass because the case that produces it is a refused Build: that call
// returns a zero BuiltCall with no hashes, and treating "no hashes" as
// "nothing to check" would disarm the tripwire for the call after the
// failure — the worst possible moment. The first call in a phase has genuinely
// nothing to compare and says so by passing the zero frontier.
type ErrNoPreviousHashes struct {
	Frontier Slot
}

func (e ErrNoPreviousHashes) Error() string {
	return fmt.Sprintf("prompt: stability check through %s has no previous call to compare against",
		e.Frontier)
}

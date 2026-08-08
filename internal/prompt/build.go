// Package prompt assembles the single user turn every pipeline call sends.
//
// ARCHITECTURE.md §7 is the contract. A call is an ordered stack of eight
// slots, most stable first, rendered through one deterministic template into
// one user turn — gemma's chat template has no true system role, so slot
// boundaries are a builder concept, not wire messages. Descending stability
// maximizes the prefix a provider's cache can reuse across a stage's call
// fan-out, and puts the load-bearing text at the two well-attended ends.
//
// Build is a pure function of its inputs: no clock, no randomness, no I/O.
// That is what makes prompt bytes reproducible from the provenance tuple, and
// what lets the prefix-stability property be a test rather than a hope.
//
// Nothing here truncates. A slot over budget or a call over the ceiling is
// refused (ErrOverBudget) and pushed back to the stage that can split it.
package prompt

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"kbase/internal/tokens"
)

// Render scaffolding. These bytes are pinned by a golden test: every one of
// them is a prefix-cache boundary, so changing one invalidates every cached
// prefix in flight and must be a deliberate act.
const (
	// slotDelimiter separates two rendered slots — exactly one blank line.
	// The builder invents no headers: a slot's content is what it is, and a
	// label the author did not write is a label the author cannot tune.
	slotDelimiter = "\n\n"

	// statusLineDelimiter joins slot-4 status lines, one per line, in the
	// order the caller supplied (most→least stable).
	statusLineDelimiter = "\n"

	// reminderLabel opens the trailer. The `## CRITICAL` section is named
	// for its content, so this label is the only thing the trailer adds.
	reminderLabel = "REMINDER:"

	// trailerFence wraps the trailer body. FOUR backticks, not three, so a
	// three-backtick fence inside a CRITICAL section nests instead of
	// terminating the trailer.
	trailerFence = "````"

	// trailerPartDelimiter separates the CRITICAL text from the injected
	// acceptance criteria inside the fence.
	trailerPartDelimiter = "\n\n"
)

// Per-call token budgets from ARCHITECTURE.md §9. The target is the
// quality/cost operating point and the ceiling is the reliable zone of the
// model's window; the gap between them is the headroom that makes the
// chars-per-token heuristic good enough (§8).
const (
	// DefaultCallTarget is the top of the §9 per-call target band (60–80K).
	DefaultCallTarget = 80_000
	// DefaultCallCeiling is the §9 context ceiling.
	DefaultCallCeiling = 180_000
)

// Budgets are the limits a stage sets on its calls. A zero value budgets
// nothing, which is right for tests and for stages whose sizes are already
// bounded upstream.
type Budgets struct {
	// PerSlot maps a slot to its token budget. An absent slot is
	// unbudgeted; a present slot set to 0 requires that slot to render
	// empty, which is a usable way to assert a stage never fills a buffer.
	PerSlot map[Slot]int

	// CallTarget is advisory: over it, Build returns the call with a
	// Warning. Nobody should be running near the ceiling, so the target is
	// where the operating point is questioned, not where work stops.
	CallTarget int

	// CallCeiling is hard: over it, Build refuses. Zero disables the check.
	CallCeiling int
}

// StageSpec is the constructor input for a stage context.
type StageSpec struct {
	// SystemFrame is slot 1 as final bytes — rendered once at job start by
	// the orchestrator, which owns the template and the interpolation.
	SystemFrame string

	// AgentDef is slot 2 plus the trailer's source text.
	AgentDef Definition

	// TaskDef is slot 3.
	TaskDef string

	// Budgets are the per-call limits.
	Budgets Budgets

	// Est is the token estimator; the zero value is the default ratio.
	Est tokens.Estimator

	// DisableDualRender turns OFF the §7 dual render, leaving the CRITICAL
	// text in slot 2 only. It is named negatively so the zero value is the
	// §7 default (render twice), and it exists for the eval harness to A/B
	// the trailer per stage and tier — not as a routine knob. Acceptance
	// criteria still render in slot 8: they are data injection, not part of
	// the experiment.
	DisableDualRender bool
}

// StageContext is the immutable per-stage half of a call: the slots that must
// not change while a stage's call loop runs.
//
// Its fields are unexported and set once at construction. This is §7's first
// enforcement layer: mutating a stable slot mid-loop is not a runtime error
// to be caught, it does not compile. The layer above it — a fresh context
// built with different bytes mid-task, which is type-legal and still a churn
// bug — is the orchestrator's phase matrix, and the layer below it is
// CheckStability.
type StageContext struct {
	systemFrame string
	agentDef    Definition
	taskDef     string
	budgets     Budgets
	dualRender  bool
	est         tokens.Estimator
}

// NewStageContext freezes a stage's constant slots. Construct once per stage;
// constructing a second one mid-stage with different bytes is the churn bug
// the orchestrator's phase matrix exists to catch.
func NewStageContext(spec StageSpec) *StageContext {
	return &StageContext{
		systemFrame: spec.SystemFrame,
		agentDef:    spec.AgentDef,
		taskDef:     spec.TaskDef,
		budgets:     spec.Budgets,
		dualRender:  !spec.DisableDualRender,
		est:         spec.Est,
	}
}

// CallInput is the per-call half. There is no reminder field: slot 8 is
// derived from the stage's CRITICAL text and these acceptance criteria, and
// is not directly settable by design.
type CallInput struct {
	// StatusLines is slot 4, pre-ordered most→least stable by the caller.
	// The builder does not sort: which line churns fastest is stage
	// knowledge, and a sort here would silently reorder a caller who knew
	// better.
	StatusLines []string

	// RefA, Content and RefB are slots 5, 6 and 7. Any may be empty, and an
	// empty one renders as nothing at all.
	RefA    string
	Content string
	RefB    string

	// AcceptanceCriteria are the per-call criteria the taxonomy skeleton
	// already emits (tiling ranges, budgets), injected into the trailer as
	// data. They share the CriticalWordCap budget.
	AcceptanceCriteria []string
}

// SlotSpan is a slot's byte range in the rendered turn. An empty slot has
// Start == End at the position it would occupy, so a caller comparing two
// calls can still name the byte where a newly-filled slot begins.
type SlotSpan struct {
	Start int
	End   int
}

// BuiltCall is one assembled call.
type BuiltCall struct {
	// UserTurn is the single wire turn.
	UserTurn string

	// EstTokens holds a per-slot estimate plus the whole-call estimate
	// under SlotTotal. The total exceeds the sum of the parts by the
	// delimiters, which are part of what the model reads.
	EstTokens map[Slot]int

	// Offsets locates every slot in UserTurn — the raw material for the
	// prefix-stability property and, later, for pipeline-level receipts.
	Offsets map[Slot]SlotSpan

	// Hashes is the churn tripwire's input (§7 layer 3): sha256 over each
	// slot's rendered bytes, for every slot including the empty ones.
	Hashes map[Slot][32]byte

	// Warning is advisory, non-empty only when the call is over the target
	// but under the ceiling. It is not an error: the call is usable, the
	// operating point is not what the stage intended.
	Warning string
}

// Build assembles one call. It returns a zero BuiltCall with a typed error
// when the trailer is over cap or any budget is exceeded — refuse-and-split,
// never truncate.
func (sc *StageContext) Build(in CallInput) (BuiltCall, error) {
	trailer, trailerWords := sc.renderTrailer(in.AcceptanceCriteria)
	if trailerWords > CriticalWordCap {
		return BuiltCall{}, ErrCriticalOverCap{Words: trailerWords, Cap: CriticalWordCap}
	}

	content := map[Slot]string{
		SlotSystemFrame: sc.systemFrame,
		SlotAgentDef:    sc.agentDef.Body,
		SlotTaskDef:     sc.taskDef,
		SlotTaskStatus:  strings.Join(in.StatusLines, statusLineDelimiter),
		SlotRefA:        in.RefA,
		SlotContent:     in.Content,
		SlotRefB:        in.RefB,
		SlotReminder:    trailer,
	}

	var b strings.Builder
	out := BuiltCall{
		EstTokens: make(map[Slot]int, len(allSlots)+1),
		Offsets:   make(map[Slot]SlotSpan, len(allSlots)),
		Hashes:    make(map[Slot][32]byte, len(allSlots)),
	}
	for _, s := range allSlots {
		text := normalizeSlot(content[s])
		out.Hashes[s] = sha256.Sum256([]byte(text))
		out.EstTokens[s] = sc.est.Estimate(text)
		if text == "" {
			// An empty slot is nothing on the wire, not empty
			// scaffolding — but it still gets a span, at the point it
			// would occupy, so a later call that fills it can be shown
			// to have left everything before that byte untouched.
			out.Offsets[s] = SlotSpan{Start: b.Len(), End: b.Len()}
			continue
		}
		if b.Len() > 0 {
			b.WriteString(slotDelimiter)
		}
		start := b.Len()
		b.WriteString(text)
		out.Offsets[s] = SlotSpan{Start: start, End: b.Len()}
	}
	out.UserTurn = b.String()
	out.EstTokens[SlotTotal] = sc.est.Estimate(out.UserTurn)

	if err := sc.checkBudgets(out.EstTokens); err != nil {
		return BuiltCall{}, err
	}
	if t := sc.budgets.CallTarget; t > 0 && out.EstTokens[SlotTotal] > t {
		out.Warning = fmt.Sprintf("prompt: call estimate %d tokens over per-call target %d",
			out.EstTokens[SlotTotal], t)
	}
	return out, nil
}

// checkBudgets reports the first violation in slot order, then the ceiling.
// Slot order rather than map order so the same over-budget call always names
// the same slot — an error that moves between runs is an error nobody trusts.
func (sc *StageContext) checkBudgets(est map[Slot]int) error {
	for _, s := range allSlots {
		budget, ok := sc.budgets.PerSlot[s]
		if !ok {
			continue
		}
		if est[s] > budget {
			return ErrOverBudget{Slot: s, Estimate: est[s], Budget: budget}
		}
	}
	if c := sc.budgets.CallCeiling; c > 0 && est[SlotTotal] > c {
		return ErrOverBudget{Slot: SlotTotal, Estimate: est[SlotTotal], Budget: c}
	}
	return nil
}

// renderTrailer builds slot 8 and returns it with the word count the cap
// applies to. The count covers the trailer's content — the CRITICAL text and
// the criteria — and not the label or the fence, which are scaffolding the
// author cannot shorten.
//
// An empty trailer renders as nothing: a stage with no CRITICAL section and
// no criteria for this call has nothing to restate.
func (sc *StageContext) renderTrailer(criteria []string) (string, int) {
	var parts []string
	if sc.dualRender {
		if crit := trimBlock(sc.agentDef.Critical); crit != "" {
			parts = append(parts, crit)
		}
	}
	var lines []string
	for _, c := range criteria {
		if strings.TrimSpace(c) != "" {
			lines = append(lines, c)
		}
	}
	if len(lines) > 0 {
		parts = append(parts, strings.Join(lines, statusLineDelimiter))
	}
	if len(parts) == 0 {
		return "", 0
	}
	body := strings.Join(parts, trailerPartDelimiter)
	return reminderLabel + "\n" + trailerFence + "\n" + body + "\n" + trailerFence, wordCount(body)
}

// normalizeSlot canonicalizes a slot's bytes so the delimiter scheme means
// what it says: exactly one blank line between slots, regardless of how the
// caller's source happened to end.
//
// Trailing whitespace and leading blank lines go — both are invisible
// artifacts of how a file was read, and leaving them in would let a
// whitespace-only difference between two sources of the same text break a
// prefix cache. Leading spaces and tabs on the first content line STAY: that
// is indentation, which is structure in the verbatim material slot 6 carries.
func normalizeSlot(s string) string {
	return strings.TrimRight(strings.TrimLeft(s, "\n\r"), " \t\n\r")
}

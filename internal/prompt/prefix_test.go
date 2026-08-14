package prompt_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"kbase/internal/pipeline"
	"kbase/internal/prompt"
)

// The prefix-stability property is the whole reason for the slot ordering
// (ARCHITECTURE.md §7): prefix caching pays for byte-identical leading
// tokens, so the wire bytes of two calls must be identical up to the first
// slot whose input changed. These tests make that falsifiable, and in doing so
// pin the delimiter scheme against regressions.
//
// They are an EXTERNAL test package so they can import internal/pipeline. The
// frontiers they assert are the ones the phase-op table declares — §12 names
// these tests as one of the phaseOpTable's consumers, and deriving the expectations
// there is what makes "single source" true rather than aspirational. A
// hand-written frontier here would be a second source that agrees until the
// day it does not.

// frontier reads a phase's declared stability frontier out of the phaseOpTable. Any
// failure here is fatal: a test that silently substituted a default frontier
// would assert something the orchestrator does not enforce.
func frontier(t *testing.T, p pipeline.JobPhase) prompt.Slot {
	t.Helper()
	f, err := pipeline.StabilityFrontier(p)
	if err != nil {
		t.Fatalf("pipeline.StabilityFrontier(%v): %v", p, err)
	}
	return f
}

// statusBlock renders the status slot the way the builder does, from the
// input alone.
func statusBlock(in prompt.CallInput) string {
	return strings.Join(in.StatusLines, prompt.StatusLineDelimiter)
}

// stageConstantSlots is the orchestrator's derivation, with its error
// checked. A test that quietly substituted a default here would assert
// something the orchestrator does not enforce.
func stageConstantSlots(t *testing.T) []prompt.Slot {
	t.Helper()
	slots, err := pipeline.StageConstantSlots()
	if err != nil {
		t.Fatalf("pipeline.StageConstantSlots(): %v", err)
	}
	return slots
}

// perCallSlots is everything the stage-constant set does not cover, in render
// order, derived from AllSlots rather than listed again. A slot added to the
// stack therefore joins the frontier scan by construction — a parallel list
// would let it slip in unscanned, and the tests would keep passing while
// checking less.
func perCallSlots(t *testing.T) []prompt.Slot {
	t.Helper()
	stageConstant := stageConstantSlots(t)
	all := prompt.AllSlots()
	out := make([]prompt.Slot, 0, len(all)-len(stageConstant))
	for _, s := range all {
		if !slices.Contains(stageConstant, s) {
			out = append(out, s)
		}
	}
	return out
}

// perCallInput returns the input text behind a per-call slot, derived from
// CallInput rather than from the built call — the helper must be able to
// predict the frontier without consulting the output it is checking.
//
// An unmapped slot is fatal, not empty: that is the other half of deriving
// perCallSlots. A new slot reaches here with no case of its own, and
// returning "" would report it as never changing.
func perCallInput(t *testing.T, in prompt.CallInput, s prompt.Slot) string {
	t.Helper()
	switch s {
	case prompt.SlotTaskStatus:
		return statusBlock(in)
	case prompt.SlotStageRef:
		return in.StageRef
	case prompt.SlotContent:
		return in.Content
	case prompt.SlotCallRef:
		return in.CallRef
	case prompt.SlotCriticalEcho:
		return strings.Join(in.AcceptanceCriteria, prompt.TrailerCriteriaDelimiter)
	default:
		t.Fatalf("no CallInput mapping for %s", s)
		return ""
	}
}

// commonLinePrefix returns the length of the longest common prefix of a and b
// that ends on a line boundary. Within the status slot the guarantee is per
// line, not per byte: when only the last status line changes, the bytes above
// it are untouched and the cache holds through them.
func commonLinePrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	if i := strings.LastIndexByte(a[:n], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}

// expectedFrontier computes the byte offset through which two built calls
// must agree: the start of the first per-call slot whose input differs,
// extended through the unchanged leading lines of the status block.
func expectedFrontier(t *testing.T, prevIn, curIn prompt.CallInput, prev, cur prompt.BuiltCall) int {
	t.Helper()
	for _, s := range perCallSlots(t) {
		a, b := perCallInput(t, prevIn, s), perCallInput(t, curIn, s)
		if a == b {
			continue
		}
		// A slot that empties out takes its leading delimiter with it, so
		// the two calls disagree about where it starts; the earlier of the
		// two positions is the honest frontier.
		start := min(prev.Offsets[s].Start, cur.Offsets[s].Start)
		if s == prompt.SlotTaskStatus {
			return start + commonLinePrefix(a, b)
		}
		return start
	}
	return len(prev.UserTurn) // nothing changed: the whole turn is the prefix
}

// assertStablePrefix checks the general invariant for one transition.
func assertStablePrefix(t *testing.T, prevIn, curIn prompt.CallInput, prev, cur prompt.BuiltCall) {
	t.Helper()
	f := expectedFrontier(t, prevIn, curIn, prev, cur)
	if f > len(prev.UserTurn) || f > len(cur.UserTurn) {
		t.Fatalf("frontier %d is past the end of a turn (%d, %d)",
			f, len(prev.UserTurn), len(cur.UserTurn))
	}
	if prev.UserTurn[:f] != cur.UserTurn[:f] {
		t.Errorf("bytes diverge before the frontier at %d:\n prev %q\n cur  %q",
			f, prev.UserTurn[:f], cur.UserTurn[:f])
	}
	// Slots 1–3 are stage-constant: they can never churn across a call loop.
	for _, s := range stageConstantSlots(t) {
		if prev.Hashes[s] != cur.Hashes[s] {
			t.Errorf("%s churned across a call", s)
		}
	}
}

func TestPrefixStabilityPairwise(t *testing.T) {
	sc := prompt.NewContext(t, prompt.BaseSpec(t))

	// The two frontiers the phaseOpTable declares for a running stage. Every case
	// below whose claim IS one of them states it this way, so a table edit
	// moves the test with it.
	var (
		callLoop          = frontier(t, pipeline.PhaseCallLoop)
		sectionTransition = frontier(t, pipeline.PhaseSectionTransition)
	)

	for _, tc := range []struct {
		name string
		// mutate produces the second call's input.
		mutate func(prompt.CallInput) prompt.CallInput
		// stableThrough is the last slot whose bytes must be identical in
		// both calls — the §7 claim stated per case, alongside the
		// general property assertStablePrefix derives.
		stableThrough prompt.Slot
	}{
		{
			name: "content only",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.Content = "A different span entirely."
				return in
			},
			stableThrough: prompt.SlotTaskStatus,
		},
		{
			name: "transient reference only",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.CallRef = "Prior output ends: elsewhere."
				return in
			},
			stableThrough: prompt.SlotContent,
		},
		{
			name: "acceptance criteria only",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.AcceptanceCriteria = []string{"- cover lines 43-77", "- budget 800 tokens"}
				return in
			},
			stableThrough: prompt.SlotCallRef,
		},
		{
			// The flush is the one per-call event that costs the whole
			// stage-constant prefix and everything the buffer holds — which
			// is exactly what the section-transition row of the phaseOpTable
			// declares, and why the flush is legal only in that phase.
			name: "multi-call reference flush",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.StageRef = "Cross-file listing:\n- c.md\n- d.md"
				return in
			},
			stableThrough: sectionTransition,
		},
		{
			// Status sits BELOW the multi-call buffer, so churning it costs
			// the status block and what follows — never the buffer. That is
			// the call-loop frontier.
			name: "last status line only",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.StatusLines = []string{"Sections done: 2/9", "Current file: guide/api.md"}
				return in
			},
			stableThrough: callLoop,
		},
		{
			name: "first status line invalidates the rest of the status block",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.StatusLines = []string{"Sections done: 3/9", "Current file: guide/intro.md"}
				return in
			},
			stableThrough: callLoop,
		},
		{
			name: "buffer emptied",
			mutate: func(in prompt.CallInput) prompt.CallInput {
				in.CallRef = ""
				return in
			},
			stableThrough: prompt.SlotContent,
		},
		{
			name:          "identical inputs",
			mutate:        func(in prompt.CallInput) prompt.CallInput { return in },
			stableThrough: prompt.SlotCriticalEcho,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prevIn := prompt.BaseInput()
			curIn := tc.mutate(prompt.BaseInput())
			prev := prompt.Build(t, sc, prevIn)
			cur := prompt.Build(t, sc, curIn)

			end := prev.Offsets[tc.stableThrough].End
			if end != cur.Offsets[tc.stableThrough].End {
				t.Fatalf("%s ends at %d in one call and %d in the other",
					tc.stableThrough, end, cur.Offsets[tc.stableThrough].End)
			}
			if prev.UserTurn[:end] != cur.UserTurn[:end] {
				t.Errorf("bytes are not stable through %s", tc.stableThrough)
			}
			assertStablePrefix(t, prevIn, curIn, prev, cur)
		})
	}
}

// TestPrefixStabilitySequence walks a distillation-shaped call loop: status
// and content churn every call, the transient reference with them, and the
// multi-call reference buffer flushes every stageRefFlushEvery calls. The
// invariant must hold at every transition, not just in isolated pairs.
func TestPrefixStabilitySequence(t *testing.T) {
	const (
		calls              = 12
		stageRefFlushEvery = 4
	)
	sc := prompt.NewContext(t, prompt.BaseSpec(t))

	// The subject is the stage reference buffer, which is what this case is
	// ABOUT — "the buffer holds its bytes between flushes". The call-loop
	// frontier happens to be the same slot today, and that agreement is worth
	// asserting, but it is a separate claim: a frontier is "the last slot
	// that must be stable", not "the slot this test is about", and reading
	// the subject out of the phaseOpTable would silently start checking a
	// different slot the day the phaseOpTable lowered it.
	if callLoop := frontier(t, pipeline.PhaseCallLoop); callLoop != prompt.SlotStageRef {
		t.Fatalf("call-loop frontier = %v, want %v", callLoop, prompt.SlotStageRef)
	}

	callInput := func(i int) prompt.CallInput {
		return prompt.CallInput{
			StatusLines: []string{
				"Stage: distillation",
				"Sections done: 3/9",
				fmt.Sprintf("Leaves done: %d/%d", i, calls),
				fmt.Sprintf("Current file: guide/leaf-%02d.md", i),
			},
			StageRef:           fmt.Sprintf("Cross-file listing (batch %d):\n- a.md\n- b.md", i/stageRefFlushEvery),
			Content:            fmt.Sprintf("Source span %d: the quick brown fox jumps over the lazy dog.", i),
			CallRef:            fmt.Sprintf("Prior leaf ended at line %d.", 40*i),
			AcceptanceCriteria: []string{fmt.Sprintf("- cover lines %d-%d", 40*i, 40*i+39)},
		}
	}

	prevIn := callInput(0)
	prev := prompt.Build(t, sc, prevIn)
	for i := 1; i < calls; i++ {
		curIn := callInput(i)
		cur := prompt.Build(t, sc, curIn)

		t.Run(fmt.Sprintf("call %02d", i), func(t *testing.T) {
			assertStablePrefix(t, prevIn, curIn, prev, cur)

			// Between flushes the multi-call buffer must hold its bytes —
			// and now that it sits above the status block, holding them
			// buys cache: the whole prefix through the buffer survives a
			// per-call status change. That is the claim the pre-swap
			// ordering could not make, because a status line churning
			// every call moved the frontier above the buffer either way.
			if i%stageRefFlushEvery != 0 {
				if prev.Hashes[prompt.SlotStageRef] != cur.Hashes[prompt.SlotStageRef] {
					t.Errorf("%s churned between flushes", prompt.SlotStageRef)
				}
				end := prev.Offsets[prompt.SlotStageRef].End
				if end != cur.Offsets[prompt.SlotStageRef].End || prev.UserTurn[:end] != cur.UserTurn[:end] {
					t.Errorf("status churn cost the prefix through %s", prompt.SlotStageRef)
				}
			}
			// Only the last status line churns per call, so the cache must
			// hold through the lines above it.
			held := commonLinePrefix(statusBlock(prevIn), statusBlock(curIn))
			if held == 0 {
				t.Error("no status line survived a per-call transition")
			}
		})

		prevIn, prev = curIn, cur
	}
}

package prompt

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The prefix-stability property is the whole reason for the slot ordering
// (ARCHITECTURE.md §7): prefix caching pays for byte-identical leading
// tokens, so the wire bytes of two calls must be identical up to the first
// slot whose input changed. These tests make that falsifiable, and in doing so
// pin the delimiter scheme against regressions.

// statusBlock renders the status slot the way the builder does, from the
// input alone.
func statusBlock(in CallInput) string { return strings.Join(in.StatusLines, statusLineDelimiter) }

// stageConstantSlots are the slots no CallInput field can move: they are
// fixed for the life of a stage, so a call loop can never churn them.
var stageConstantSlots = []Slot{SlotSystemFrame, SlotAgentDef, SlotTaskDef}

// perCallSlots is everything else, in render order, derived from allSlots
// rather than listed again. A slot added to the stack therefore joins the
// frontier scan by construction — a parallel list would let it slip in
// unscanned, and the tests would keep passing while checking less.
func perCallSlots() []Slot {
	out := make([]Slot, 0, len(allSlots)-len(stageConstantSlots))
	for _, s := range allSlots {
		if !slices.Contains(stageConstantSlots, s) {
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
func perCallInput(t *testing.T, in CallInput, s Slot) string {
	t.Helper()
	switch s {
	case SlotTaskStatus:
		return statusBlock(in)
	case SlotRefA:
		return in.RefA
	case SlotContent:
		return in.Content
	case SlotRefB:
		return in.RefB
	case SlotReminder:
		return strings.Join(in.AcceptanceCriteria, trailerCriteriaDelimiter)
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
func expectedFrontier(t *testing.T, prevIn, curIn CallInput, prev, cur BuiltCall) int {
	t.Helper()
	for _, s := range perCallSlots() {
		a, b := perCallInput(t, prevIn, s), perCallInput(t, curIn, s)
		if a == b {
			continue
		}
		// A slot that empties out takes its leading delimiter with it, so
		// the two calls disagree about where it starts; the earlier of the
		// two positions is the honest frontier.
		start := min(prev.Offsets[s].Start, cur.Offsets[s].Start)
		if s == SlotTaskStatus {
			return start + commonLinePrefix(a, b)
		}
		return start
	}
	return len(prev.UserTurn) // nothing changed: the whole turn is the prefix
}

// assertStablePrefix checks the general invariant for one transition.
func assertStablePrefix(t *testing.T, prevIn, curIn CallInput, prev, cur BuiltCall) {
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
	for _, s := range stageConstantSlots {
		if prev.Hashes[s] != cur.Hashes[s] {
			t.Errorf("%s churned across a call", s)
		}
	}
}

func TestPrefixStabilityPairwise(t *testing.T) {
	sc := newContext(t, baseSpec(t))

	for _, tc := range []struct {
		name string
		// mutate produces the second call's input.
		mutate func(CallInput) CallInput
		// stableThrough is the last slot whose bytes must be identical in
		// both calls — the §7 claim stated per case, alongside the
		// general property assertStablePrefix derives.
		stableThrough Slot
	}{
		{
			name:          "content only",
			mutate:        func(in CallInput) CallInput { in.Content = "A different span entirely."; return in },
			stableThrough: SlotTaskStatus,
		},
		{
			name:          "transient reference only",
			mutate:        func(in CallInput) CallInput { in.RefB = "Prior output ends: elsewhere."; return in },
			stableThrough: SlotContent,
		},
		{
			name: "acceptance criteria only",
			mutate: func(in CallInput) CallInput {
				in.AcceptanceCriteria = []string{"- cover lines 43-77", "- budget 800 tokens"}
				return in
			},
			stableThrough: SlotRefB,
		},
		{
			// The flush is the one per-call event that costs the whole
			// stage-constant prefix and everything the buffer holds.
			name: "multi-call reference flush",
			mutate: func(in CallInput) CallInput {
				in.RefA = "Cross-file listing:\n- c.md\n- d.md"
				return in
			},
			stableThrough: SlotTaskDef,
		},
		{
			// Status sits BELOW the multi-call buffer, so churning it costs
			// the status block and what follows — never the buffer.
			name: "last status line only",
			mutate: func(in CallInput) CallInput {
				in.StatusLines = []string{"Sections done: 2/9", "Current file: guide/api.md"}
				return in
			},
			stableThrough: SlotRefA,
		},
		{
			name: "first status line invalidates the rest of the status block",
			mutate: func(in CallInput) CallInput {
				in.StatusLines = []string{"Sections done: 3/9", "Current file: guide/intro.md"}
				return in
			},
			stableThrough: SlotRefA,
		},
		{
			name:          "buffer emptied",
			mutate:        func(in CallInput) CallInput { in.RefB = ""; return in },
			stableThrough: SlotContent,
		},
		{
			name:          "identical inputs",
			mutate:        func(in CallInput) CallInput { return in },
			stableThrough: SlotReminder,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prevIn := baseInput()
			curIn := tc.mutate(baseInput())
			prev := build(t, sc, prevIn)
			cur := build(t, sc, curIn)

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
// multi-call reference buffer flushes every refAFlushEvery calls. The
// invariant must hold at every transition, not just in isolated pairs.
func TestPrefixStabilitySequence(t *testing.T) {
	const (
		calls          = 12
		refAFlushEvery = 4
	)
	sc := newContext(t, baseSpec(t))

	callInput := func(i int) CallInput {
		return CallInput{
			StatusLines: []string{
				"Stage: distillation",
				"Sections done: 3/9",
				fmt.Sprintf("Leaves done: %d/%d", i, calls),
				fmt.Sprintf("Current file: guide/leaf-%02d.md", i),
			},
			RefA:               fmt.Sprintf("Cross-file listing (batch %d):\n- a.md\n- b.md", i/refAFlushEvery),
			Content:            fmt.Sprintf("Source span %d: the quick brown fox jumps over the lazy dog.", i),
			RefB:               fmt.Sprintf("Prior leaf ended at line %d.", 40*i),
			AcceptanceCriteria: []string{fmt.Sprintf("- cover lines %d-%d", 40*i, 40*i+39)},
		}
	}

	prevIn := callInput(0)
	prev := build(t, sc, prevIn)
	for i := 1; i < calls; i++ {
		curIn := callInput(i)
		cur := build(t, sc, curIn)

		t.Run(fmt.Sprintf("call %02d", i), func(t *testing.T) {
			assertStablePrefix(t, prevIn, curIn, prev, cur)

			// Between flushes the multi-call buffer must hold its bytes —
			// and now that it sits above the status block, holding them
			// buys cache: the whole prefix through the buffer survives a
			// per-call status change. That is the claim the pre-swap
			// ordering could not make, because a status line churning
			// every call moved the frontier above the buffer either way.
			if i%refAFlushEvery != 0 {
				if prev.Hashes[SlotRefA] != cur.Hashes[SlotRefA] {
					t.Error("reference buffer A churned between flushes")
				}
				end := prev.Offsets[SlotRefA].End
				if end != cur.Offsets[SlotRefA].End || prev.UserTurn[:end] != cur.UserTurn[:end] {
					t.Error("status churn cost the prefix through the multi-call buffer")
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

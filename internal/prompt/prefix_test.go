package prompt

import (
	"fmt"
	"strings"
	"testing"
)

// The prefix-stability property is the whole reason for the slot ordering
// (ARCHITECTURE.md §7): prefix caching pays for byte-identical leading
// tokens, so the wire bytes of two calls must be identical up to the first
// slot whose input changed. These tests make that falsifiable, and in doing so
// pin the delimiter scheme against regressions.

// statusBlock renders slot 4 the way the builder does, from the input alone.
func statusBlock(in CallInput) string { return strings.Join(in.StatusLines, statusLineDelimiter) }

// perCallInput returns the input text behind a per-call slot, derived from
// CallInput rather than from the built call — the helper must be able to
// predict the frontier without consulting the output it is checking.
func perCallInput(in CallInput, s Slot) string {
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
		return strings.Join(in.AcceptanceCriteria, statusLineDelimiter)
	default:
		return ""
	}
}

// commonLinePrefix returns the length of the longest common prefix of a and b
// that ends on a line boundary. Within slot 4 the guarantee is per line, not
// per byte: when only the last status line changes, the bytes above it are
// untouched and the cache holds through them.
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
// extended through the unchanged leading lines of slot 4.
func expectedFrontier(t *testing.T, prevIn, curIn CallInput, prev, cur BuiltCall) int {
	t.Helper()
	for _, s := range []Slot{SlotTaskStatus, SlotRefA, SlotContent, SlotRefB, SlotReminder} {
		a, b := perCallInput(prevIn, s), perCallInput(curIn, s)
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
	for _, s := range []Slot{SlotSystemFrame, SlotAgentDef, SlotTaskDef} {
		if prev.Hashes[s] != cur.Hashes[s] {
			t.Errorf("%s churned across a call", s)
		}
	}
}

func TestPrefixStabilityPairwise(t *testing.T) {
	sc := NewStageContext(baseSpec(t))

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
			stableThrough: SlotRefA,
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
			name: "multi-call reference flush",
			mutate: func(in CallInput) CallInput {
				in.RefA = "Cross-file listing:\n- c.md\n- d.md"
				return in
			},
			stableThrough: SlotTaskStatus,
		},
		{
			name: "last status line only",
			mutate: func(in CallInput) CallInput {
				in.StatusLines = []string{"Sections done: 2/9", "Current file: guide/api.md"}
				return in
			},
			stableThrough: SlotTaskDef,
		},
		{
			name: "first status line invalidates the rest of slot 4",
			mutate: func(in CallInput) CallInput {
				in.StatusLines = []string{"Sections done: 3/9", "Current file: guide/intro.md"}
				return in
			},
			stableThrough: SlotTaskDef,
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
	sc := NewStageContext(baseSpec(t))

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

			// Between flushes the multi-call buffer must hold its bytes.
			// Note what this does NOT claim: slot 5 sits after slot 4, so
			// a status line churning every call moves the frontier above
			// it either way. Slot 5's stability is worth prefix cache only
			// across calls whose status block is identical.
			if i%refAFlushEvery != 0 && prev.Hashes[SlotRefA] != cur.Hashes[SlotRefA] {
				t.Error("reference buffer A churned between flushes")
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

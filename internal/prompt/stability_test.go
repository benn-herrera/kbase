package prompt

import (
	"errors"
	"testing"
)

// stableFrontier is what a call-loop phase would declare: everything down to
// the multi-call reference buffer holds between calls, and the status block
// below it is where per-call churn starts.
const stableFrontier = SlotRefA

func TestCheckStabilityPasses(t *testing.T) {
	sc := newContext(t, baseSpec(t))
	prev := build(t, sc, baseInput())

	in := baseInput()
	in.Content = "A different span."
	in.StatusLines = []string{"Sections done: 3/9", "Current file: guide/api.md"}
	cur := build(t, sc, in)

	if err := CheckStability(stableFrontier, prev.Hashes, cur.Hashes); err != nil {
		t.Errorf("per-call slots changed but stable slots did not: %v", err)
	}
}

// TestCheckStabilityCatchesChurn: a stage context rebuilt mid-loop with
// different bytes is type-legal and still a churn bug — this is the layer
// that catches it.
func TestCheckStabilityCatchesChurn(t *testing.T) {
	prev := build(t, newContext(t, baseSpec(t)), baseInput())

	spec := baseSpec(t)
	spec.TaskDef = "Task: distill section 4."
	cur := build(t, newContext(t, spec), baseInput())

	err := CheckStability(stableFrontier, prev.Hashes, cur.Hashes)
	var target ErrSlotChurn
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrSlotChurn", err)
	}
	if target.Slot != SlotTaskDef {
		t.Errorf("Slot = %v, want %v", target.Slot, SlotTaskDef)
	}
}

// TestCheckStabilityEmptySlots: an empty slot hashes consistently, so a
// buffer that is empty on both calls is stable rather than unevaluable.
func TestCheckStabilityEmptySlots(t *testing.T) {
	sc := newContext(t, baseSpec(t))
	in := baseInput()
	in.RefA = ""

	first := build(t, sc, in)
	in.Content = "A different span."
	second := build(t, sc, in)

	if err := CheckStability(stableFrontier, first.Hashes, second.Hashes); err != nil {
		t.Errorf("an empty stable buffer should compare equal: %v", err)
	}
}

func TestCheckStabilityEdges(t *testing.T) {
	cur := build(t, newContext(t, baseSpec(t)), baseInput()).Hashes

	// The first call in a phase says so with the zero frontier. Claiming a
	// real frontier with nothing to compare against is the shape a refused
	// Build produces (zero BuiltCall, nil Hashes), and it must not pass
	// quietly — that would disarm the tripwire for the call after a failure.
	t.Run("first call passes the zero frontier", func(t *testing.T) {
		if err := CheckStability(SlotTotal, nil, cur); err != nil {
			t.Errorf("the zero frontier claims nothing: %v", err)
		}
	})

	t.Run("a real frontier with no previous call is reported", func(t *testing.T) {
		err := CheckStability(stableFrontier, nil, cur)
		var target ErrNoPreviousHashes
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrNoPreviousHashes", err)
		}
		if target.Frontier != stableFrontier {
			t.Errorf("Frontier = %v, want %v", target.Frontier, stableFrontier)
		}
	})

	// A frontier reaches every slot at or above it, so a churn below the
	// frontier cannot be excluded from the check the way an arbitrary
	// stable set could exclude it.
	t.Run("the frontier covers everything above it", func(t *testing.T) {
		churned := map[Slot][32]byte{}
		for s, h := range cur {
			churned[s] = h
		}
		churned[SlotAgentDef] = [32]byte{1}

		err := CheckStability(stableFrontier, cur, churned)
		var target ErrSlotChurn
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrSlotChurn", err)
		}
		if target.Slot != SlotAgentDef {
			t.Errorf("Slot = %v, want %v", target.Slot, SlotAgentDef)
		}
	})

	t.Run("missing hash is reported, not skipped", func(t *testing.T) {
		partial := map[Slot][32]byte{SlotSystemFrame: cur[SlotSystemFrame]}
		err := CheckStability(stableFrontier, partial, cur)
		var target ErrMissingHash
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrMissingHash", err)
		}
		if target.Slot != SlotAgentDef {
			t.Errorf("Slot = %v, want %v", target.Slot, SlotAgentDef)
		}
	})
}

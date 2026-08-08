package prompt

import (
	"errors"
	"testing"
)

// stableSlots is the set a call-loop phase would declare stable: everything
// the loop is not supposed to touch between calls.
var stableSlots = []Slot{SlotSystemFrame, SlotAgentDef, SlotTaskDef, SlotRefA}

func TestCheckStabilityPasses(t *testing.T) {
	sc := NewStageContext(baseSpec(t))
	prev := build(t, sc, baseInput())

	in := baseInput()
	in.Content = "A different span."
	in.StatusLines = []string{"Sections done: 3/9", "Current file: guide/api.md"}
	cur := build(t, sc, in)

	if err := CheckStability(stableSlots, prev.Hashes, cur.Hashes); err != nil {
		t.Errorf("per-call slots changed but stable slots did not: %v", err)
	}
}

// TestCheckStabilityCatchesChurn: a stage context rebuilt mid-loop with
// different bytes is type-legal and still a churn bug — this is the layer
// that catches it.
func TestCheckStabilityCatchesChurn(t *testing.T) {
	prev := build(t, NewStageContext(baseSpec(t)), baseInput())

	spec := baseSpec(t)
	spec.TaskDef = "Task: distill section 4."
	cur := build(t, NewStageContext(spec), baseInput())

	err := CheckStability(stableSlots, prev.Hashes, cur.Hashes)
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
	sc := NewStageContext(baseSpec(t))
	in := baseInput()
	in.RefA = ""

	first := build(t, sc, in)
	in.Content = "A different span."
	second := build(t, sc, in)

	if err := CheckStability(stableSlots, first.Hashes, second.Hashes); err != nil {
		t.Errorf("an empty stable buffer should compare equal: %v", err)
	}
}

func TestCheckStabilityEdges(t *testing.T) {
	cur := build(t, NewStageContext(baseSpec(t)), baseInput()).Hashes

	t.Run("no previous call", func(t *testing.T) {
		if err := CheckStability(stableSlots, nil, cur); err != nil {
			t.Errorf("first call in a phase has nothing to compare: %v", err)
		}
	})

	t.Run("nothing declared stable", func(t *testing.T) {
		if err := CheckStability(nil, cur, cur); err != nil {
			t.Errorf("an empty stable set always passes: %v", err)
		}
	})

	t.Run("missing hash is reported, not skipped", func(t *testing.T) {
		partial := map[Slot][32]byte{SlotSystemFrame: cur[SlotSystemFrame]}
		err := CheckStability(stableSlots, partial, cur)
		var target ErrMissingHash
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrMissingHash", err)
		}
		if target.Slot != SlotAgentDef {
			t.Errorf("Slot = %v, want %v", target.Slot, SlotAgentDef)
		}
	})
}

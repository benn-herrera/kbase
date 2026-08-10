package prompt_test

import (
	"errors"
	"testing"

	"kbase/internal/pipeline"
	"kbase/internal/prompt"
)

// The tripwire's frontier is not a fact about this package — it is whatever
// the orchestrator's phase matrix declares for the phase the call is built in
// (ARCHITECTURE.md §12). These tests read it from there, which is why they are
// an external test package: pipeline imports prompt, so an in-package test
// could not ask.

// callLoopFrontier is what a call-loop phase declares: everything down to the
// multi-call reference buffer holds between calls, and the status block below
// it is where per-call churn starts.
func callLoopFrontier(t *testing.T) prompt.Slot {
	t.Helper()
	return frontier(t, pipeline.PhaseCallLoop)
}

func TestCheckStabilityPasses(t *testing.T) {
	sc := prompt.NewContext(t, prompt.BaseSpec(t))
	prev := prompt.Build(t, sc, prompt.BaseInput())

	in := prompt.BaseInput()
	in.Content = "A different span."
	in.StatusLines = []string{"Sections done: 3/9", "Current file: guide/api.md"}
	cur := prompt.Build(t, sc, in)

	if err := prompt.CheckStability(callLoopFrontier(t), prev.Hashes, cur.Hashes); err != nil {
		t.Errorf("per-call slots changed but stable slots did not: %v", err)
	}
}

// TestCheckStabilityCatchesChurn: a stage context rebuilt mid-loop with
// different bytes is type-legal and still a churn bug — this is the layer
// that catches it.
func TestCheckStabilityCatchesChurn(t *testing.T) {
	prev := prompt.Build(t, prompt.NewContext(t, prompt.BaseSpec(t)), prompt.BaseInput())

	spec := prompt.BaseSpec(t)
	spec.TaskDef = "Task: distill section 4."
	cur := prompt.Build(t, prompt.NewContext(t, spec), prompt.BaseInput())

	err := prompt.CheckStability(callLoopFrontier(t), prev.Hashes, cur.Hashes)
	var target prompt.ErrSlotChurn
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrSlotChurn", err)
	}
	if target.Slot != prompt.SlotTaskDef {
		t.Errorf("Slot = %v, want %v", target.Slot, prompt.SlotTaskDef)
	}
}

// TestCheckStabilityEmptySlots: an empty slot hashes consistently, so a
// buffer that is empty on both calls is stable rather than unevaluable.
func TestCheckStabilityEmptySlots(t *testing.T) {
	sc := prompt.NewContext(t, prompt.BaseSpec(t))
	in := prompt.BaseInput()
	in.RefA = ""

	first := prompt.Build(t, sc, in)
	in.Content = "A different span."
	second := prompt.Build(t, sc, in)

	if err := prompt.CheckStability(callLoopFrontier(t), first.Hashes, second.Hashes); err != nil {
		t.Errorf("an empty stable buffer should compare equal: %v", err)
	}
}

func TestCheckStabilityEdges(t *testing.T) {
	cur := prompt.Build(t, prompt.NewContext(t, prompt.BaseSpec(t)), prompt.BaseInput()).Hashes
	stableFrontier := callLoopFrontier(t)

	// The first call in a phase says so with the zero frontier. Claiming a
	// real frontier with nothing to compare against is the shape a refused
	// Build produces (zero BuiltCall, nil Hashes), and it must not pass
	// quietly — that would disarm the tripwire for the call after a failure.
	t.Run("first call passes the zero frontier", func(t *testing.T) {
		if err := prompt.CheckStability(prompt.SlotTotal, nil, cur); err != nil {
			t.Errorf("the zero frontier claims nothing: %v", err)
		}
	})

	// Job setup declares exactly that zero frontier, because it has no
	// previous call at all — the matrix and the tripwire agree on what a
	// worker may claim before it has built anything.
	t.Run("job setup declares the zero frontier", func(t *testing.T) {
		if f := frontier(t, pipeline.PhaseJobSetup); f != prompt.SlotTotal {
			t.Errorf("job setup frontier = %v, want %v", f, prompt.SlotTotal)
		}
	})

	t.Run("a real frontier with no previous call is reported", func(t *testing.T) {
		err := prompt.CheckStability(stableFrontier, nil, cur)
		var target prompt.ErrNoPreviousHashes
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
		churned := map[prompt.Slot][32]byte{}
		for s, h := range cur {
			churned[s] = h
		}
		churned[prompt.SlotAgentDef] = [32]byte{1}

		err := prompt.CheckStability(stableFrontier, cur, churned)
		var target prompt.ErrSlotChurn
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrSlotChurn", err)
		}
		if target.Slot != prompt.SlotAgentDef {
			t.Errorf("Slot = %v, want %v", target.Slot, prompt.SlotAgentDef)
		}
	})

	t.Run("missing hash is reported, not skipped", func(t *testing.T) {
		partial := map[prompt.Slot][32]byte{prompt.SlotSystemFrame: cur[prompt.SlotSystemFrame]}
		err := prompt.CheckStability(stableFrontier, partial, cur)
		var target prompt.ErrMissingHash
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want ErrMissingHash", err)
		}
		if target.Slot != prompt.SlotAgentDef {
			t.Errorf("Slot = %v, want %v", target.Slot, prompt.SlotAgentDef)
		}
	})
}

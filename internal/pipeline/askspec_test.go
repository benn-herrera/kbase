package pipeline

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/model"
	"kbase/internal/prompt"
)

// TestRoleSeam: the seam is DERIVED from the presence of a fallback, so the
// classification and the thing it classifies cannot disagree (R-4).
func TestRoleSeam(t *testing.T) {
	if got := noFallbackAsk(t).Seam(); got != NoFallbackSeam {
		t.Errorf("an ask with no fallback is %v, want %v", got, NoFallbackSeam)
	}
	if got := fallbackBackedAsk(t).Seam(); got != FallbackBackedSeam {
		t.Errorf("an ask with a fallback is %v, want %v", got, FallbackBackedSeam)
	}
}

func TestRoleValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(*AskSpec)
		want  string
	}{
		{"no verifier", func(r *AskSpec) { r.Verify = nil }, "verifier"},
		{"no encoder", func(r *AskSpec) { r.Encode = nil }, "encoder"},
		{"unmapped tier", func(r *AskSpec) { r.Tier = "medium" }, "tier"},
		// A zero Effort is the shape of a forgotten field, and thinking-off
		// is a real declaration some definition will legitimately make — so
		// the two must not be the same value. Refusing here is what keeps
		// "every ask states its effort" true of a struct field, which no
		// positional parameter upstream can enforce.
		{"undeclared effort", func(r *AskSpec) { r.Effort = model.RequestEffort{} }, "effort"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ask := noFallbackAsk(t)
			tc.spoil(&ask)
			_, err := newBoundAsk(ask, synthSpec("task"), synthJobFrame(t))
			if err == nil {
				t.Fatal("an ask that cannot run must be refused at stage setup")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestNewAgentCapturesCanonicalHashes: the canonical hashes must be what the
// BUILDER renders, not what the spec's strings hash to — otherwise the assert
// would compare two different normalizations and fire on the difference.
func TestNewAgentCapturesCanonicalHashes(t *testing.T) {
	ask := noFallbackAsk(t)
	spec := synthSpec("Task: inventory.")
	bound, err := newBoundAsk(ask, spec, synthJobFrame(t))
	if err != nil {
		t.Fatalf("newBoundAsk: %v", err)
	}

	built, err := bound.ctx.Build(synthTask("survey/a.json", "all").Input())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, s := range bound.slots {
		if bound.canonicalHashes[s] != built.Hashes[s] {
			t.Errorf("%s canonical hash does not match a real call's", s)
		}
	}
	if err := bound.checkCanonicalHashes(built.Hashes); err != nil {
		t.Errorf("a call from this bound ask must match its own canonical hashes: %v", err)
	}
}

// TestNewAgentOwnsTheDefinition: the AskSpec carries the definition, so a spec
// that also carries one does not get a vote. Two sources for slot 2 would be
// two things to keep in agreement for no gain.
func TestNewAgentOwnsTheDefinition(t *testing.T) {
	ask := noFallbackAsk(t)
	spec := synthSpec("Task: inventory.")
	spec.AgentDef = synthDef(t, "# Impostor\n")
	spec.JobFrame = "Job: an impostor frame."

	bound, err := newBoundAsk(ask, spec, synthJobFrame(t))
	if err != nil {
		t.Fatalf("newBoundAsk: %v", err)
	}
	built, err := bound.ctx.Build(prompt.CallInput{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(built.UserTurn, "Impostor") {
		t.Error("the spec's definition reached slot 2 over the ask's")
	}
	// Slot 1 is the JOB's, for the same reason and with more at stake: a
	// per-stage frame that reached the wire would cost the whole cross-stage
	// prefix while every check the stage makes against itself still passed.
	if !strings.HasPrefix(built.UserTurn, synthFrame) {
		t.Errorf("the spec's job frame reached slot 1 over the job's; turn starts %q",
			built.UserTurn[:min(len(built.UserTurn), 40)])
	}
}

// TestCheckCanonicalCatchesAnotherStagesContext is the R-1 cross-worker
// assert. Today one immutable StageContext per stage makes this failure
// unrepresentable by construction — which is exactly why the check is here:
// it keeps a guarantee from quietly becoming an assumption if a later change
// ever gives a worker its own context. The tampered map stands in for a worker
// that built against the wrong one.
func TestCheckCanonicalCatchesAnotherStagesContext(t *testing.T) {
	bound, err := newBoundAsk(noFallbackAsk(t), synthSpec("Task: inventory."), synthJobFrame(t))
	if err != nil {
		t.Fatalf("newBoundAsk: %v", err)
	}
	built, err := bound.ctx.Build(prompt.CallInput{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	t.Run("a differing stage-constant slot is reported", func(t *testing.T) {
		bound.canonicalHashes[prompt.SlotAgentDef] = [32]byte{0xAA}
		defer func() { bound.canonicalHashes[prompt.SlotAgentDef] = built.Hashes[prompt.SlotAgentDef] }()

		err := bound.checkCanonicalHashes(built.Hashes)
		var target StageContextMismatchError
		if !errors.As(err, &target) {
			t.Fatalf("err = %v, want StageContextMismatchError", err)
		}
		if target.Slot != prompt.SlotAgentDef {
			t.Errorf("Slot = %v, want %v", target.Slot, prompt.SlotAgentDef)
		}
	})

	// A hash map missing the slot must not compare equal to a zero value and
	// pass; that would disarm the assert for exactly the hand-assembled case
	// it is guarding against.
	t.Run("a missing hash is reported, not skipped", func(t *testing.T) {
		var target StageContextMismatchError
		if err := bound.checkCanonicalHashes(map[prompt.Slot][32]byte{}); !errors.As(err, &target) {
			t.Fatalf("err = %v, want StageContextMismatchError", err)
		}
	})
}

// TestNewAgentRefusesAnUnbuildableStage: an over-cap CRITICAL section fails at
// stage setup rather than on every call the stage would have made.
func TestNewAgentRefusesAnUnbuildableStage(t *testing.T) {
	ask := noFallbackAsk(t)
	ask.Def = prompt.Definition{Body: "# Wordy\n", Critical: strings.TrimSpace(strings.Repeat("word ", 200))}

	_, err := newBoundAsk(ask, synthSpec("task"), synthJobFrame(t))
	var target prompt.ErrCriticalOverCap
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrCriticalOverCap", err)
	}
}

// TestStageConstantSlotsAreDerivedFromThePhaseOpTable: the set the cross-worker
// assert walks is not a list beside the phaseOpTable, it is a filter over the slot
// stack at the frontier the phaseOpTable declares. Lower the section-transition
// frontier and this set follows, with nothing to keep in agreement.
func TestStageConstantSlotsAreDerivedFromThePhaseOpTable(t *testing.T) {
	frontier, err := StageConstantFrontier()
	if err != nil {
		t.Fatal(err)
	}
	if want, err := StabilityFrontier(PhaseSectionTransition); err != nil || frontier != want {
		t.Fatalf("StageConstantFrontier = %v (err %v), section transition declares %v", frontier, err, want)
	}

	got, err := StageConstantSlots()
	if err != nil {
		t.Fatal(err)
	}
	var want []prompt.Slot
	for _, s := range prompt.AllSlots() {
		if s <= frontier {
			want = append(want, s)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("StageConstantSlots() = %v, want %v", got, want)
	}

	// Data callers walk, so it is a fresh slice: a caller that reordered it
	// would otherwise silently move the assert for everyone.
	slices.Reverse(got)
	if again, _ := StageConstantSlots(); slices.Equal(got, again) {
		t.Error("StageConstantSlots handed out shared backing storage")
	}
}

// TestSlotOneIsMeasuredAgainstTheJob is the §7 job-constancy claim armed. A
// stage whose context renders a different slot 1 than the job's is the exact
// failure the old teardown-frontier comment promised to catch and did not:
// self-consistent within the stage, silently costing the whole cross-stage
// prefix. It fails at stage setup, and — since a per-call render could differ
// from the setup probe — every call is measured against the job's value too.
func TestSlotOneIsMeasuredAgainstTheJob(t *testing.T) {
	job := synthJobFrame(t)

	t.Run("a stage rendering a different frame is refused at setup", func(t *testing.T) {
		divergent, err := newJobFrame("Job: a different job entirely.")
		if err != nil {
			t.Fatal(err)
		}
		// The bound ask is built against the divergent frame while the job
		// the canonical hash is the original: what a plan carrying a per-stage frame
		// would produce at its second stage.
		bound, err := newBoundAsk(noFallbackAsk(t), synthSpec("Task: inventory."), divergent)
		if err != nil {
			t.Fatalf("newBoundAsk: %v", err)
		}
		bound.canonicalHashes[prompt.SlotJobFrame] = job.hash

		built, err := bound.ctx.Build(prompt.CallInput{})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var target StageContextMismatchError
		if err := bound.checkCanonicalHashes(built.Hashes); !errors.As(err, &target) {
			t.Fatalf("err = %v, want StageContextMismatchError", err)
		}
		if target.Slot != prompt.SlotJobFrame {
			t.Errorf("Slot = %v, want %v", target.Slot, prompt.SlotJobFrame)
		}
	})

	t.Run("the canonical hash for slot 1 is the job's own render", func(t *testing.T) {
		bound, err := newBoundAsk(noFallbackAsk(t), synthSpec("Task: inventory."), job)
		if err != nil {
			t.Fatalf("newBoundAsk: %v", err)
		}
		if bound.canonicalHashes[prompt.SlotJobFrame] != job.hash {
			t.Error("slot 1's canonical hash is not the job's rendered frame")
		}
	})
}

// TestRoleTiersAreTheConfiguredOnes pins the two tier spellings to config's
// constants rather than to string literals here.
func TestRoleTiersAreTheConfiguredOnes(t *testing.T) {
	cfg := synthConfig()
	for _, ask := range []AskSpec{noFallbackAsk(t), fallbackBackedAsk(t)} {
		if _, ok := cfg.ModelFor(ask.Tier); !ok {
			t.Errorf("tier %q resolves to no model; want one of %s/%s",
				ask.Tier, config.TierHeavy, config.TierLight)
		}
	}
}

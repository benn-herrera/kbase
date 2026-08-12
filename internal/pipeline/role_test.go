package pipeline

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"kbase/internal/config"
	"kbase/internal/prompt"
)

// TestRoleSeam: the seam is DERIVED from the presence of a baseline, so the
// classification and the thing it classifies cannot disagree (R-4).
func TestRoleSeam(t *testing.T) {
	if got := essentialRole(t).Seam(); got != SeamEssential {
		t.Errorf("a role with no baseline is %v, want %v", got, SeamEssential)
	}
	if got := refinementRole(t).Seam(); got != SeamRefinement {
		t.Errorf("a role with a baseline is %v, want %v", got, SeamRefinement)
	}
}

func TestRoleValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(*Role)
		want  string
	}{
		{"no verifier", func(r *Role) { r.Verify = nil }, "verifier"},
		{"no encoder", func(r *Role) { r.Encode = nil }, "encoder"},
		{"unmapped tier", func(r *Role) { r.Tier = "medium" }, "tier"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			role := essentialRole(t)
			tc.spoil(&role)
			_, err := newAgent(role, synthSpec("task"), synthJobFrame(t))
			if err == nil {
				t.Fatal("a role that cannot run must be refused at stage setup")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestNewAgentCapturesCanonicalHashes: the canonical values must be what the
// BUILDER renders, not what the spec's strings hash to — otherwise the assert
// would compare two different normalizations and fire on the difference.
func TestNewAgentCapturesCanonicalHashes(t *testing.T) {
	role := essentialRole(t)
	spec := synthSpec("Task: inventory.")
	agent, err := newAgent(role, spec, synthJobFrame(t))
	if err != nil {
		t.Fatalf("newAgent: %v", err)
	}

	built, err := agent.ctx.Build(synthTask("survey/a.json", "all").Input())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, s := range agent.slots {
		if agent.canonical[s] != built.Hashes[s] {
			t.Errorf("%s canonical hash does not match a real call's", s)
		}
	}
	if err := agent.checkCanonical(built.Hashes); err != nil {
		t.Errorf("a call from this agent must match its own canonical: %v", err)
	}
}

// TestNewAgentOwnsTheDefinition: the Role carries the definition, so a spec
// that also carries one does not get a vote. Two sources for slot 2 would be
// two things to keep in agreement for no gain.
func TestNewAgentOwnsTheDefinition(t *testing.T) {
	role := essentialRole(t)
	spec := synthSpec("Task: inventory.")
	spec.AgentDef = synthDef(t, "# Impostor\n")
	spec.SystemFrame = "Job: an impostor frame."

	agent, err := newAgent(role, spec, synthJobFrame(t))
	if err != nil {
		t.Fatalf("newAgent: %v", err)
	}
	built, err := agent.ctx.Build(prompt.CallInput{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(built.UserTurn, "Impostor") {
		t.Error("the spec's definition reached slot 2 over the role's")
	}
	// Slot 1 is the JOB's, for the same reason and with more at stake: a
	// per-stage frame that reached the wire would cost the whole cross-stage
	// prefix while every check the stage makes against itself still passed.
	if !strings.HasPrefix(built.UserTurn, synthFrame) {
		t.Errorf("the spec's system frame reached slot 1 over the job's; turn starts %q",
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
	agent, err := newAgent(essentialRole(t), synthSpec("Task: inventory."), synthJobFrame(t))
	if err != nil {
		t.Fatalf("newAgent: %v", err)
	}
	built, err := agent.ctx.Build(prompt.CallInput{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	t.Run("a differing stage-constant slot is reported", func(t *testing.T) {
		agent.canonical[prompt.SlotAgentDef] = [32]byte{0xAA}
		defer func() { agent.canonical[prompt.SlotAgentDef] = built.Hashes[prompt.SlotAgentDef] }()

		err := agent.checkCanonical(built.Hashes)
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
		if err := agent.checkCanonical(map[prompt.Slot][32]byte{}); !errors.As(err, &target) {
			t.Fatalf("err = %v, want StageContextMismatchError", err)
		}
	})
}

// TestNewAgentRefusesAnUnbuildableStage: an over-cap CRITICAL section fails at
// stage setup rather than on every call the stage would have made.
func TestNewAgentRefusesAnUnbuildableStage(t *testing.T) {
	role := essentialRole(t)
	role.Def = prompt.Definition{Body: "# Wordy\n", Critical: strings.TrimSpace(strings.Repeat("word ", 200))}

	_, err := newAgent(role, synthSpec("task"), synthJobFrame(t))
	var target prompt.ErrCriticalOverCap
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want ErrCriticalOverCap", err)
	}
}

// TestStageConstantSlotsAreDerivedFromTheMatrix: the set the cross-worker
// assert walks is not a list beside the matrix, it is a filter over the slot
// stack at the frontier the matrix declares. Lower the section-transition
// frontier and this set follows, with nothing to keep in agreement.
func TestStageConstantSlotsAreDerivedFromTheMatrix(t *testing.T) {
	frontier, err := StageConstantFrontier()
	if err != nil {
		t.Fatal(err)
	}
	if want, err := Frontier(PhaseSectionTransition); err != nil || frontier != want {
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
		// The agent is built against the divergent frame while the job
		// canonical is the original: what a plan carrying a per-stage frame
		// would produce at its second stage.
		agent, err := newAgent(essentialRole(t), synthSpec("Task: inventory."), divergent)
		if err != nil {
			t.Fatalf("newAgent: %v", err)
		}
		agent.canonical[prompt.SlotSystemFrame] = job.hash

		built, err := agent.ctx.Build(prompt.CallInput{})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var target StageContextMismatchError
		if err := agent.checkCanonical(built.Hashes); !errors.As(err, &target) {
			t.Fatalf("err = %v, want StageContextMismatchError", err)
		}
		if target.Slot != prompt.SlotSystemFrame {
			t.Errorf("Slot = %v, want %v", target.Slot, prompt.SlotSystemFrame)
		}
	})

	t.Run("the canonical slot 1 is the job's own render", func(t *testing.T) {
		agent, err := newAgent(essentialRole(t), synthSpec("Task: inventory."), job)
		if err != nil {
			t.Fatalf("newAgent: %v", err)
		}
		if agent.canonical[prompt.SlotSystemFrame] != job.hash {
			t.Error("slot 1's canonical is not the job's rendered frame")
		}
	})
}

// TestRoleTiersAreTheConfiguredOnes pins the two tier spellings to config's
// constants rather than to string literals here.
func TestRoleTiersAreTheConfiguredOnes(t *testing.T) {
	cfg := synthConfig()
	for _, role := range []Role{essentialRole(t), refinementRole(t)} {
		if _, ok := cfg.ModelFor(role.Tier); !ok {
			t.Errorf("tier %q resolves to no model; want one of %s/%s",
				role.Tier, config.TierHeavy, config.TierLight)
		}
	}
}

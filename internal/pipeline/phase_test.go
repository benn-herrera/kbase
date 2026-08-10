package pipeline

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log/logtest"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/prompt"
)

func TestGuard(t *testing.T) {
	cases := []struct {
		name  string
		phase Phase
		op    Op
		allow bool
	}{
		{"job setup renders the system frame", PhaseJobSetup, OpBuildSystemFrame, true},
		{"job setup scans the store", PhaseJobSetup, OpResumeScan, true},
		{"job setup does not build calls", PhaseJobSetup, OpBuildCall, false},
		{"stage setup rebuilds the stage context", PhaseStageSetup, OpRebuildStageContext, true},
		{"stage setup does not write artifacts", PhaseStageSetup, OpWriteArtifact, false},
		{"call loop builds calls", PhaseCallLoop, OpBuildCall, true},
		{"call loop advances units", PhaseCallLoop, OpAdvanceUnit, true},
		{"call loop writes artifacts", PhaseCallLoop, OpWriteArtifact, true},
		{"call loop may not flush ref A", PhaseCallLoop, OpFlushRefA, false},
		{"call loop may not rebuild the stage context", PhaseCallLoop, OpRebuildStageContext, false},
		{"call loop may not re-render the system frame", PhaseCallLoop, OpBuildSystemFrame, false},
		{"call loop may not rescan the store", PhaseCallLoop, OpResumeScan, false},
		{"section transition flushes ref A", PhaseSectionTransition, OpFlushRefA, true},
		{"section transition advances units", PhaseSectionTransition, OpAdvanceUnit, true},
		{"section transition does not build calls", PhaseSectionTransition, OpBuildCall, false},
		{"stage teardown writes the stage artifact", PhaseStageTeardown, OpWriteArtifact, true},
		{"stage teardown may not flush ref A", PhaseStageTeardown, OpFlushRefA, false},
		{"a phase outside the matrix allows nothing", Phase(99), OpBuildCall, false},
		{"an op outside the matrix is allowed nowhere", PhaseCallLoop, Op(99), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := guard(tc.phase, tc.op)
			if tc.allow {
				if err != nil {
					t.Fatalf("guard(%s, %s) = %v, want allowed", tc.phase, tc.op, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("guard(%s, %s) allowed the operation", tc.phase, tc.op)
			}
			var notAllowed OpNotAllowedError
			var unknown UnknownPhaseError
			if !errors.As(err, &notAllowed) && !errors.As(err, &unknown) {
				t.Fatalf("guard(%s, %s) = %v, want a typed defect error", tc.phase, tc.op, err)
			}
		})
	}
}

// TestFlushRefAIsSectionTransitionOnly states the enforcement R-3 names by
// itself, because it is the one matrix cell the §7 slot ordering rests on: a
// reference buffer flushed anywhere else churns slot 4 per call, which makes
// its position above the status block a loss instead of a win.
func TestFlushRefAIsSectionTransitionOnly(t *testing.T) {
	for _, r := range matrix {
		err := guard(r.phase, OpFlushRefA)
		if want := r.phase == PhaseSectionTransition; (err == nil) != want {
			t.Errorf("guard(%s, %s) = %v, allowed=%v want allowed=%v", r.phase, OpFlushRefA, err, err == nil, want)
		}
	}
}

// TestMatrixCompleteness is the table's own gate: every phase carries a
// frontier, at least one operation and a registered crashpoint, and every
// operation is legal somewhere. An op no phase permits is dead code that
// reads like policy; a phase with no frontier is a hole the tripwire falls
// through.
func TestMatrixCompleteness(t *testing.T) {
	// The ops come from opNames, which String also reads — one enumeration,
	// so an op added to the const block and forgotten cannot be silently
	// uncovered by the very test that exists to notice it.
	allOps := make([]Op, 0, len(opNames))
	for op := range opNames {
		allOps = append(allOps, op)
	}
	allPhases := []Phase{
		PhaseJobSetup, PhaseStageSetup, PhaseCallLoop,
		PhaseSectionTransition, PhaseStageTeardown,
	}

	if len(matrix) != len(allPhases) {
		t.Fatalf("matrix has %d rows, %d phases are declared", len(matrix), len(allPhases))
	}
	registered := crashpoint.RegisteredNames()
	for _, p := range allPhases {
		r, ok := rule(p)
		if !ok {
			t.Errorf("phase %s has no matrix row", p)
			continue
		}
		if len(r.ops) == 0 {
			t.Errorf("phase %s permits no operations", p)
		}
		if r.name == "" {
			t.Errorf("phase %d has no name", int(p))
		}
		if _, err := Frontier(p); err != nil {
			t.Errorf("phase %s has no frontier: %v", p, err)
		}
		// The zero frontier is a legitimate declaration (job setup claims
		// nothing), but it must be a stated one, so the row is checked for
		// a value in range rather than for a non-zero value.
		if r.frontier < prompt.SlotTotal || r.frontier > prompt.SlotReminder {
			t.Errorf("phase %s declares frontier %v, outside the slot stack", p, r.frontier)
		}
		if r.crash == "" {
			t.Errorf("phase %s has no crashpoint", p)
		}
		if !slices.Contains(registered, r.crash) {
			t.Errorf("phase %s crashpoint %q is not registered", p, r.crash)
		}
	}

	for _, op := range allOps {
		legal := false
		for _, r := range matrix {
			if guard(r.phase, op) == nil {
				legal = true
			}
		}
		if !legal {
			t.Errorf("operation %s is not permitted in any phase", op)
		}
		if op.String() == "" || strings.HasPrefix(op.String(), "op(") {
			t.Errorf("operation %d has no name of its own", int(op))
		}
	}
}

// TestFrontierDropsAtSectionTransition makes the §7 slot-swap safety
// falsifiable rather than asserted in prose: the call loop claims stability
// through reference buffer A, the section transition — which is where that
// buffer is legally replaced — claims strictly less, and the churn tripwire
// agrees with both. Raise the section-transition frontier to SlotRefA and the
// last check here fails, which is exactly the bug (a flushed buffer reported
// as stable) that would otherwise surface as a silently destroyed cache.
func TestFrontierDropsAtSectionTransition(t *testing.T) {
	loop, err := Frontier(PhaseCallLoop)
	if err != nil {
		t.Fatal(err)
	}
	transition, err := Frontier(PhaseSectionTransition)
	if err != nil {
		t.Fatal(err)
	}
	if loop != prompt.SlotRefA {
		t.Errorf("call-loop frontier is %v, want %v", loop, prompt.SlotRefA)
	}
	if transition >= loop {
		t.Fatalf("section-transition frontier %v does not drop below the call-loop frontier %v", transition, loop)
	}

	// Two calls that differ only in reference buffer A: the flush that the
	// section transition exists to perform.
	prev := hashes(map[prompt.Slot]byte{prompt.SlotRefA: 1})
	cur := hashes(map[prompt.Slot]byte{prompt.SlotRefA: 2})

	var churn prompt.ErrSlotChurn
	if err := prompt.CheckStability(loop, prev, cur); !errors.As(err, &churn) || churn.Slot != prompt.SlotRefA {
		t.Errorf("call-loop frontier accepted a flushed reference buffer: %v", err)
	}
	if err := prompt.CheckStability(transition, prev, cur); err != nil {
		t.Errorf("section-transition frontier rejected its own legal flush: %v", err)
	}
}

func TestLowestFrontier(t *testing.T) {
	cases := []struct {
		name   string
		phases []Phase
		want   prompt.Slot
	}{
		{"steady call loop", []Phase{PhaseCallLoop}, prompt.SlotRefA},
		{
			"a section transition since the last call",
			[]Phase{PhaseCallLoop, PhaseSectionTransition, PhaseCallLoop},
			prompt.SlotTaskDef,
		},
		{
			"a stage boundary since the last call",
			[]Phase{PhaseStageTeardown, PhaseStageSetup, PhaseCallLoop},
			prompt.SlotSystemFrame,
		},
		{
			"the first call of the job claims nothing",
			[]Phase{PhaseJobSetup, PhaseStageSetup, PhaseCallLoop},
			prompt.SlotTotal,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lowestFrontier(tc.phases...)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("lowestFrontier(%v) = %v, want %v", tc.phases, got, tc.want)
			}
		})
	}

	if _, err := lowestFrontier(); err == nil {
		t.Error("lowestFrontier() over no phases returned a frontier")
	}
	if _, err := lowestFrontier(PhaseCallLoop, Phase(99)); err == nil {
		t.Error("lowestFrontier() accepted a phase outside the matrix")
	}
}

func TestEnterPhase(t *testing.T) {
	lg := &logtest.Capture{}
	for _, r := range matrix {
		if err := enterPhase(r.phase, lg); err != nil {
			t.Fatalf("enterPhase(%s): %v", r.phase, err)
		}
		if !lg.Has(t, "debug", "phase", r.name) {
			t.Errorf("entering %s recorded no debug record", r.phase)
		}
	}
	if err := enterPhase(Phase(99), lg); err == nil {
		t.Error("enterPhase accepted a phase outside the matrix")
	}
}

// TestEnterPhaseCrashpoint proves the phase seam is armable: the crash
// interrupts inside EnterPhase, so a harness killing "at the call loop"
// stops before the phase's first operation rather than somewhere near it.
func TestEnterPhaseCrashpoint(t *testing.T) {
	r, ok := rule(PhaseCallLoop)
	if !ok {
		t.Fatal("no call-loop row")
	}
	defer crashpoint.Arm(r.crash)()

	if !crashedAt(t, r.crash, func() { _ = enterPhase(PhaseCallLoop, &logtest.Capture{}) }) {
		t.Error("enterPhase returned instead of crashing at its armed point")
	}
}

// hashes builds a slot-hash map covering every slot, with the named slots
// distinguished by a single byte — enough for CheckStability, which compares
// values and never interprets them.
func hashes(distinct map[prompt.Slot]byte) map[prompt.Slot][32]byte {
	out := map[prompt.Slot][32]byte{}
	for s := prompt.SlotSystemFrame; s <= prompt.SlotReminder; s++ {
		var h [32]byte
		h[0] = distinct[s]
		out[s] = h
	}
	return out
}

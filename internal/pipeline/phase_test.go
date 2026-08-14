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
		phase JobPhase
		op    Op
		allow bool
	}{
		{"job setup renders the job frame", PhaseJobSetup, OpBuildJobFrame, true},
		{"job setup scans the store", PhaseJobSetup, OpResumeScan, true},
		{"job setup does not build calls", PhaseJobSetup, OpBuildCall, false},
		{"stage setup rebuilds the stage context", PhaseStageSetup, OpRebuildStageContext, true},
		{"stage setup does not write artifacts", PhaseStageSetup, OpWriteArtifact, false},
		{"call loop builds calls", PhaseCallLoop, OpBuildCall, true},
		{"call loop advances units", PhaseCallLoop, OpAdvanceUnit, true},
		{"call loop writes artifacts", PhaseCallLoop, OpWriteArtifact, true},
		{"call loop may not flush stage ref", PhaseCallLoop, OpFlushStageRef, false},
		{"call loop may not rebuild the stage context", PhaseCallLoop, OpRebuildStageContext, false},
		{"call loop may not re-render the job frame", PhaseCallLoop, OpBuildJobFrame, false},
		{"call loop may not rescan the store", PhaseCallLoop, OpResumeScan, false},
		{"section transition flushes stage ref", PhaseSectionTransition, OpFlushStageRef, true},
		{"section transition advances units", PhaseSectionTransition, OpAdvanceUnit, true},
		{"section transition does not build calls", PhaseSectionTransition, OpBuildCall, false},
		{"stage teardown writes the stage artifact", PhaseStageTeardown, OpWriteArtifact, true},
		{"stage teardown may not flush stage ref", PhaseStageTeardown, OpFlushStageRef, false},
		{"a phase outside the phaseOpTable allows nothing", JobPhase(99), OpBuildCall, false},
		{"an op outside the phaseOpTable is allowed nowhere", PhaseCallLoop, Op(99), false},
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

// TestFlushStageRefIsSectionTransitionOnly states the enforcement R-3 names by
// itself, because it is the one table cell the §7 slot ordering rests on: a
// reference buffer flushed anywhere else churns slot 4 per call, which makes
// its position above the status block a loss instead of a win.
func TestFlushStageRefIsSectionTransitionOnly(t *testing.T) {
	for _, r := range phaseOpTable {
		err := guard(r.phase, OpFlushStageRef)
		if want := r.phase == PhaseSectionTransition; (err == nil) != want {
			t.Errorf("guard(%s, %s) = %v, allowed=%v want allowed=%v", r.phase, OpFlushStageRef, err, err == nil, want)
		}
	}
}

// TestPhaseOpTableCompleteness is the table's own gate: every phase carries a
// frontier, at least one operation and a registered crashpoint, and every
// operation is legal somewhere. An op no phase permits is dead code that
// reads like policy; a phase with no frontier is a hole the tripwire falls
// through.
func TestPhaseOpTableCompleteness(t *testing.T) {
	// The ops come from opNames, which String also reads — one enumeration,
	// so an op added to the const block and forgotten cannot be silently
	// uncovered by the very test that exists to notice it.
	allOps := make([]Op, 0, len(opNames))
	for op := range opNames {
		allOps = append(allOps, op)
	}
	allPhases := []JobPhase{
		PhaseJobSetup, PhaseStageSetup, PhaseCallLoop,
		PhaseSectionTransition, PhaseStageTeardown,
	}

	if len(phaseOpTable) != len(allPhases) {
		t.Fatalf("phaseOpTable has %d rows, %d phases are declared", len(phaseOpTable), len(allPhases))
	}
	registered := crashpoint.RegisteredNames()
	for _, p := range allPhases {
		r, ok := rule(p)
		if !ok {
			t.Errorf("phase %s has no table row", p)
			continue
		}
		if len(r.ops) == 0 {
			t.Errorf("phase %s permits no operations", p)
		}
		if r.name == "" {
			t.Errorf("phase %d has no name", int(p))
		}
		if _, err := StabilityFrontier(p); err != nil {
			t.Errorf("phase %s has no frontier: %v", p, err)
		}
		// The zero frontier is a legitimate declaration (job setup claims
		// nothing), but it must be a stated one, so the row is checked for
		// a value in range rather than for a non-zero value.
		if r.frontier < prompt.SlotTotal || r.frontier > prompt.SlotCriticalEcho {
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
		for _, r := range phaseOpTable {
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
// through the stage reference buffer, the section transition — which is where
// that buffer is legally replaced — claims strictly less, and the churn
// tripwire agrees with both. Raise the section-transition frontier to
// SlotStageRef and the last check here fails, which is exactly the bug (a
// flushed buffer reported as stable) that would otherwise surface as a
// silently destroyed cache.
func TestFrontierDropsAtSectionTransition(t *testing.T) {
	loop, err := StabilityFrontier(PhaseCallLoop)
	if err != nil {
		t.Fatal(err)
	}
	transition, err := StabilityFrontier(PhaseSectionTransition)
	if err != nil {
		t.Fatal(err)
	}
	if loop != prompt.SlotStageRef {
		t.Errorf("call-loop frontier is %v, want %v", loop, prompt.SlotStageRef)
	}
	if transition >= loop {
		t.Fatalf("section-transition frontier %v does not drop below the call-loop frontier %v", transition, loop)
	}

	// Two calls that differ only in the stage reference buffer: the flush that the
	// section transition exists to perform.
	prev := hashes(map[prompt.Slot]byte{prompt.SlotStageRef: 1})
	cur := hashes(map[prompt.Slot]byte{prompt.SlotStageRef: 2})

	var churn prompt.ErrSlotChurn
	if err := prompt.CheckStability(loop, prev, cur); !errors.As(err, &churn) || churn.Slot != prompt.SlotStageRef {
		t.Errorf("call-loop frontier accepted a flushed reference buffer: %v", err)
	}
	if err := prompt.CheckStability(transition, prev, cur); err != nil {
		t.Errorf("section-transition frontier rejected its own legal flush: %v", err)
	}
}

func TestLowestFrontier(t *testing.T) {
	cases := []struct {
		name   string
		phases []JobPhase
		want   prompt.Slot
	}{
		{"steady call loop", []JobPhase{PhaseCallLoop}, prompt.SlotStageRef},
		{
			"a section transition since the last call",
			[]JobPhase{PhaseCallLoop, PhaseSectionTransition, PhaseCallLoop},
			prompt.SlotTaskDef,
		},
		{
			"a stage boundary since the last call",
			[]JobPhase{PhaseStageTeardown, PhaseStageSetup, PhaseCallLoop},
			prompt.SlotJobFrame,
		},
		{
			"the first call of the job claims nothing",
			[]JobPhase{PhaseJobSetup, PhaseStageSetup, PhaseCallLoop},
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
	if _, err := lowestFrontier(PhaseCallLoop, JobPhase(99)); err == nil {
		t.Error("lowestFrontier() accepted a phase outside the phaseOpTable")
	}
}

func TestEnterPhase(t *testing.T) {
	lg := &logtest.Capture{}
	for _, r := range phaseOpTable {
		if err := enterPhase(r.phase, lg); err != nil {
			t.Fatalf("enterPhase(%s): %v", r.phase, err)
		}
		if !lg.Has(t, "debug", "phase", r.name) {
			t.Errorf("entering %s recorded no debug record", r.phase)
		}
	}
	if err := enterPhase(JobPhase(99), lg); err == nil {
		t.Error("enterPhase accepted a phase outside the phaseOpTable")
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
	for s := prompt.SlotJobFrame; s <= prompt.SlotCriticalEcho; s++ {
		var h [32]byte
		h[0] = distinct[s]
		out[s] = h
	}
	return out
}

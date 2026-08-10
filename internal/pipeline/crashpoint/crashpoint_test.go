package crashpoint

import (
	"slices"
	"testing"
)

// These tests share the process-global armed set, so they never run in
// parallel with each other; each disarms what it armed before returning.

func TestUnarmedIsANoOp(t *testing.T) {
	name := Register("test.unarmed")
	At(name) // must not panic
	staged := false
	AtStaging(name, func() { staged = true })
	if staged {
		t.Error("an unarmed crossing staged residue")
	}
}

func TestArmedPointCrashes(t *testing.T) {
	name := Register("test.armed")
	disarm := Arm(name)
	defer disarm()

	crash := recoverCrash(t, func() { At(name) })
	if crash == nil {
		t.Fatal("At returned instead of crashing")
	}
	if crash.Point != name {
		t.Errorf("crashed at %q, want %q", crash.Point, name)
	}
	// A neighbouring point stays cold: arming is per name, so a harness
	// killing at one seam is not killing at every seam that runs first.
	At(Register("test.armed.neighbour"))
}

func TestArmOnHit(t *testing.T) {
	name := Register("test.hit")
	defer Arm(name, ArmOnHit(3))()

	for i := 1; i < 3; i++ {
		if c := recoverCrash(t, func() { At(name) }); c != nil {
			t.Fatalf("crashed on hit %d, want hit 3", i)
		}
	}
	if c := recoverCrash(t, func() { At(name) }); c == nil {
		t.Fatal("hit 3 did not crash")
	}
	// Once a point fires it keeps firing until disarmed — a countdown that
	// silently rearmed itself would make a harness's later steps run under
	// a live kill point.
	if c := recoverCrash(t, func() { At(name) }); c == nil {
		t.Error("hit 4 did not crash")
	}
}

func TestDisarm(t *testing.T) {
	name := Register("test.disarm")
	Arm(name)()
	if c := recoverCrash(t, func() { At(name) }); c != nil {
		t.Error("a disarmed point still crashed")
	}
	Disarm("test.never-armed") // must not panic
}

func TestRegisteredNamesAreSortedAndUnique(t *testing.T) {
	Register("test.zzz")
	Register("test.aaa")
	Register("test.aaa")

	names := RegisteredNames()
	if !slices.IsSorted(names) {
		t.Errorf("names are not sorted: %v", names)
	}
	if n := slices.Index(names, "test.aaa"); n < 0 {
		t.Fatalf("a registered name is missing from %v", names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("%q appears twice", n)
		}
		seen[n] = true
	}
}

// TestAtStagingStagesOnlyOnTheKill is the residue contract the store's write
// window depends on: the staging runs on the crossing that kills and on no
// other, and it runs BEFORE the panic — so a call site can arrange the
// on-disk state a real kill would have left and still die at the same
// instruction. Deciding and staging in one call is also what makes this
// race-free against a sibling goroutine consuming the deciding hit.
func TestAtStagingStagesOnlyOnTheKill(t *testing.T) {
	name := Register("test.staging")
	defer Arm(name, ArmOnHit(2))()

	staged := 0
	if c := recoverCrash(t, func() { AtStaging(name, func() { staged++ }) }); c != nil {
		t.Fatal("crashed on hit 1, want hit 2")
	}
	if staged != 0 {
		t.Errorf("staging ran %d times on a crossing that did not kill", staged)
	}
	if c := recoverCrash(t, func() { AtStaging(name, func() { staged++ }) }); c == nil {
		t.Fatal("hit 2 did not crash")
	}
	if staged != 1 {
		t.Errorf("staging ran %d times on the killing crossing, want once", staged)
	}
}

// recoverCrash runs fn and returns the *Crash it panicked with, or nil if it
// returned normally. Any other panic value fails the test rather than being
// swallowed.
func recoverCrash(t *testing.T, fn func()) (crash *Crash) {
	t.Helper()
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}
		c, ok := rec.(*Crash)
		if !ok {
			t.Fatalf("recovered %v, want a *Crash", rec)
		}
		crash = c
	}()
	fn()
	return nil
}

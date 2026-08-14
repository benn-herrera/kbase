package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/pipeline/crashpoint"
)

// The resume claim (ARCHITECTURE.md §12) is that a run killed at any moment
// leaves a state a later run can either prove valid or redo, and that what
// comes out the far side is what an uninterrupted run would have produced.
// This file is that claim made falsifiable: for EVERY registered crashpoint,
// kill there, resume, and compare byte for byte.
//
// Two things make it a real proof rather than a shape.
//
// First, coverage is enumerated, not listed: the table comes from
// crashpoint.RegisteredNames, so a point added anywhere in the package joins
// this test without anyone remembering to add it, and a point this pipeline
// never reaches fails rather than passing vacuously.
//
// Second, each unit's artifact is a digest of the prompt that produced it, so
// byte-identical output means the resumed run rebuilt the same prompts — not
// merely that it wrote the same constant everywhere.
//
// Crashpoints are process-global, so nothing here runs in parallel.

// crashWorkers is the pool size the harness runs at. More than one, because a
// kill with siblings in flight is the case a single-worker harness would never
// produce; small, because the assertions are about the store and not about
// throughput.
const crashWorkers = 2

// runToCompletion runs the synthetic plan in dir and fails the test on
// anything but a clean, emit-ready result.
func runToCompletion(t *testing.T, dir string, mode Mode) (*stubClient, JobResult) {
	t.Helper()
	client := echoStub()
	synthProduceRuns.Store(0)
	res, err := newSynthCoordinator(t, dir, client, crashWorkers, log.Discard()).
		Run(context.Background(), synthPlan(t), mode)
	if err != nil {
		t.Fatalf("run in %s mode: %v", mode, err)
	}
	if !res.DeliveryReady() {
		t.Fatalf("run in %s mode is not emit-ready: %+v", mode, res)
	}
	return client, res
}

// clearLock applies the after-a-kill remedy before a resume.
//
// It does not assert the lockfile was there, and cannot: an in-process crash
// unwinds through the deferred Release, so the file a REAL kill leaves is
// already gone. That difference is the one piece of real-kill state this
// harness cannot reproduce, and it is not papered over — see
// TestAHardKillsLockfileRefusesEveryModeUntilRemoved, which models the
// leftover lock directly and pins the remedy the documentation promises.
func clearLock(t *testing.T, dir string) {
	t.Helper()
	if err := os.Remove(filepath.Join(storeRoot(dir), LockFileName)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove the stale lockfile: %v", err)
	}
}

// runUntilCrash runs with point armed on its nth hit and requires that the run
// actually died there. A point that never fires is a hole in the coverage this
// test exists to close, so it is a failure and not a skip.
func runUntilCrash(t *testing.T, dir, point string, hit int) {
	t.Helper()
	disarm := crashpoint.Arm(point, crashpoint.ArmOnHit(hit))
	defer disarm()

	_, err := newSynthCoordinator(t, dir, echoStub(), crashWorkers, log.Discard()).
		Run(context.Background(), synthPlan(t), ModeResume)

	var crash *crashpoint.Crash
	if !errors.As(err, &crash) {
		t.Fatalf("arming %s (hit %d) did not kill the run; err = %v", point, hit, err)
	}
	if crash.Point != point {
		t.Fatalf("run died at %s, armed %s", crash.Point, point)
	}
}

// TestResumeAfterEveryCrashpoint is the key-windows proof.
func TestResumeAfterEveryCrashpoint(t *testing.T) {
	golden := t.TempDir()
	uninterrupted, _ := runToCompletion(t, golden, ModeResume)
	want := storeState(t, golden)
	if len(want) != 2*synthUnits {
		t.Fatalf("uninterrupted run left %d files, want an artifact and a stamp per unit", len(want))
	}
	if uninterrupted.callCount() != synthCalls {
		t.Fatalf("uninterrupted run made %d calls, want %d", uninterrupted.callCount(), synthCalls)
	}

	points := crashpoint.RegisteredNames()
	if len(points) == 0 {
		t.Fatal("no crashpoints are registered; the coverage gate has nothing to enumerate")
	}
	for _, point := range points {
		// Hit 1 is the first crossing; hit 3 is a kill with the stage part
		// done, which is the case per-unit resume exists for and which the
		// once-per-job points (the job-setup phase) cannot reach.
		for _, hit := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/hit%d", point, hit), func(t *testing.T) {
				dir := t.TempDir()
				if hit > 1 && !crossedAtLeast(t, point, hit) {
					t.Skipf("%s is crossed fewer than %d times in one run", point, hit)
				}
				runUntilCrash(t, dir, point, hit)

				clearLock(t, dir)

				resumed, res := runToCompletion(t, dir, ModeResume)
				assertSameState(t, want, storeState(t, dir), "after "+point)

				// The fold's own claim, which no per-unit comparison can
				// make: its lane is redone WHOLE or not at all. A
				// half-redone fold would compose a list from a prefix
				// nothing recorded the dependencies of (§12).
				fold := foldCalls(resumed)
				if fold != 0 && fold != synthFoldCalls {
					t.Errorf("the fold made %d of its %d calls; an interrupted fold is redone whole",
						fold, synthFoldCalls)
				}
				// Reuse means the model was not consulted. Every unit the
				// resume did not redo is a call it did not make — plus the
				// fold's extra calls, if the fold was one of the redone, and
				// minus the mechanical units, which are produced in process
				// and consult nothing at all.
				extra := 0
				if fold > 0 {
					extra = fold - 1
				}
				produced := int(synthProduceRuns.Load())
				if got := resumed.callCount(); got != res.Produced-produced+extra {
					t.Errorf("%d calls to produce %d units (%d of them mechanical): a reused unit was re-executed",
						got, res.Produced, produced)
				}
				if res.Reused+res.Produced != synthUnits {
					t.Errorf("reused %d + produced %d != %d units", res.Reused, res.Produced, synthUnits)
				}
			})
		}
	}
}

// foldCalls counts the calls a run made on the fold stage's lane, read off
// the prompts the client recorded. It is how a test asks whether the fold ran
// at all, without the coordinator having to report per-lane call counts
// nobody else needs.
func foldCalls(c *stubClient) int {
	n := 0
	for _, p := range c.recorded() {
		if strings.Contains(p, "Unit: "+synthFoldStage+"/") {
			n++
		}
	}
	return n
}

// crossedAtLeast reports whether point is crossed at least n times in one
// complete run. It arms the point at hit n in a throwaway job dir and sees
// whether it fires — the only honest way to ask, since how often a point is
// crossed is a property of the run and not of the registry.
//
// The probe runs with ONE worker, deliberately. How MANY times a point is
// crossed is worker-count independent — every point here is crossed per unit,
// per lane or per stage, and the pool changes only the order — but with two
// workers racing, a probe that lands mid-flight can answer differently between
// runs, and a skip that varies is a coverage gate that sometimes is not one.
func crossedAtLeast(t *testing.T, point string, n int) bool {
	t.Helper()
	dir := t.TempDir()
	disarm := crashpoint.Arm(point, crashpoint.ArmOnHit(n))
	defer disarm()

	_, err := newSynthCoordinator(t, dir, echoStub(), 1, log.Discard()).
		Run(context.Background(), synthPlan(t), ModeResume)
	var crash *crashpoint.Crash
	return errors.As(err, &crash)
}

// TestFreshAfterCrashEqualsUninterrupted: resume is an optimization, and
// --fresh is the button that skips the question entirely. It must be
// sufficient over any wreckage a kill leaves — including the artifact-without-
// stamp state the store's write window produces.
func TestFreshAfterCrashEqualsUninterrupted(t *testing.T) {
	golden := t.TempDir()
	runToCompletion(t, golden, ModeResume)
	want := storeState(t, golden)

	dir := t.TempDir()
	runUntilCrash(t, dir, cpPutPreStamp, 2)
	clearLock(t, dir)

	client, res := runToCompletion(t, dir, ModeFresh)
	assertSameState(t, want, storeState(t, dir), "after --fresh")
	if res.Reused != 0 || res.Produced != synthUnits {
		t.Errorf("a fresh run reused %d units; it must ignore the store entirely", res.Reused)
	}
	if client.callCount() != synthCalls {
		t.Errorf("%d calls, want %d — a fresh run rebuilds everything", client.callCount(), synthCalls)
	}
}

// TestPoisonedStampRedoesOnlyItsUnit: reuse requires affirmative proof, so a
// stamp that no longer proves anything costs exactly one unit. The rest of the
// stage stands, and the scan's forensics say which was which.
func TestPoisonedStampRedoesOnlyItsUnit(t *testing.T) {
	dir := t.TempDir()
	runToCompletion(t, dir, ModeResume)
	want := storeState(t, dir)

	const poisoned = "leaves/d1/two.md"
	stamp := filepath.Join(storeRoot(dir), filepath.FromSlash(poisoned)+StampSuffix)
	if err := os.WriteFile(stamp, []byte("{ this is not a stamp"), CreateFileMode); err != nil {
		t.Fatalf("poison the stamp: %v", err)
	}

	client, res := runToCompletion(t, dir, ModeResume)

	// The forensics: the scan stopped at the stage holding the poisoned
	// unit, verdicted that unit and only that unit non-valid, and said why.
	if res.ResumeScan.StageName != "leaves" {
		t.Errorf("resume stage = %q, want leaves", res.ResumeScan.StageName)
	}
	if res.ResumeScan.Redo != 1 {
		t.Errorf("Redo = %d, want 1", res.ResumeScan.Redo)
	}
	var redone []OwedArtifactVerdict
	for _, v := range res.ResumeScan.Verdicts {
		if v.Verdict != VerdictValid {
			redone = append(redone, v)
		}
	}
	if len(redone) != 1 || redone[0].Path != poisoned {
		t.Fatalf("redone = %+v, want just %s", redone, poisoned)
	}
	if redone[0].Verdict != VerdictInvalid {
		t.Errorf("verdict = %s, want %s: an unparseable stamp is doubt, not absence",
			redone[0].Verdict, VerdictInvalid)
	}
	if redone[0].Reason == "" {
		t.Error("a non-valid verdict carried no reason for the inventory")
	}

	// Exactly one unit was re-executed, and the siblings were not.
	if client.callCount() != 1 {
		t.Errorf("%d calls, want 1: siblings were re-executed", client.callCount())
	}
	if res.Produced != 1 || res.Reused != synthUnits-1 {
		t.Errorf("produced %d / reused %d, want 1 / %d", res.Produced, res.Reused, synthUnits-1)
	}
	assertSameState(t, want, storeState(t, dir), "after a poisoned stamp")
}

// TestResumeOfACompleteJobDoesNothing: the deepest-valid-prefix walk over a
// finished job finds no work, and finding no work must cost no tokens.
func TestResumeOfACompleteJobDoesNothing(t *testing.T) {
	dir := t.TempDir()
	runToCompletion(t, dir, ModeResume)
	want := storeState(t, dir)

	client, res := runToCompletion(t, dir, ModeResume)
	if !res.ResumeScan.Complete() {
		t.Errorf("scan = %+v, want a complete chain", res.ResumeScan)
	}
	if client.callCount() != 0 || res.Produced != 0 || res.Reused != synthUnits {
		t.Errorf("%d calls, produced %d, reused %d; want 0/0/%d",
			client.callCount(), res.Produced, res.Reused, synthUnits)
	}
	if !res.DeliveryReady() {
		t.Error("a job with everything already proven is emit-ready")
	}
	assertSameState(t, want, storeState(t, dir), "after a no-op resume")
}

// TestCrashLeavesNoUnprovenArtifactTrusted: the pre-stamp window is the one
// state the design must never mistake for done — the bytes are in place and
// nothing proves them. A resume must verdict that Invalid and pay to redo it.
func TestCrashLeavesNoUnprovenArtifactTrusted(t *testing.T) {
	dir := t.TempDir()
	runUntilCrash(t, dir, cpPutPreStamp, 1)
	clearLock(t, dir)

	// Exactly the shape the window produces: an artifact with no stamp.
	var orphan string
	err := filepath.WalkDir(storeRoot(dir), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(d.Name(), tempSuffix) || d.Name() == LockFileName {
			return err
		}
		if strings.HasSuffix(path, StampSuffix) {
			return nil
		}
		if _, statErr := os.Stat(path + StampSuffix); statErr != nil {
			orphan = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if orphan == "" {
		t.Fatal("the pre-stamp crash left no unproven artifact; the window under test was not reached")
	}

	rel, err := filepath.Rel(storeRoot(dir), orphan)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	store := synthStore(t, dir, log.Discard())
	v, reason, err := store.verify(filepath.ToSlash(rel), []Input{corpusInput})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if v != VerdictInvalid {
		t.Errorf("verdict = %s, want %s: bytes with no proof are not reuse", v, VerdictInvalid)
	}
	if reason == "" {
		t.Error("the verdict carried no reason")
	}
}

// TestResumeAcrossADerivedStageBoundary is the dynamic chain's own crash case:
// stage 2's UNIT PATHS are derived from the bytes stage 1 wrote, so the resumed
// run cannot even know what it owes until it has re-proven stage 1.
//
// Killing at the stage boundary is what makes it a proof rather than a shape.
// The first run dies with stage 1 on disk and stage 2 undescribed; the second
// reuses stage 1 from its stamp, describes stage 2 from the artifact it just
// proved, and must land byte-for-byte where an uninterrupted run does. A
// resolver that had run at job setup could not have produced those paths at
// all, and one that derived them from anything but the proven upstream would
// produce different ones here.
func TestResumeAcrossADerivedStageBoundary(t *testing.T) {
	golden := t.TempDir()
	goldenClient := echoStub()
	goldenRes, err := newSynthCoordinator(t, golden, goldenClient, crashWorkers, log.Discard()).
		Run(context.Background(), synthDerivedPlan(t, golden), ModeResume)
	if err != nil || !goldenRes.DeliveryReady() {
		t.Fatalf("uninterrupted derived run: err = %v, result = %+v", err, goldenRes)
	}
	if goldenRes.Units != synthDerivedUnits {
		t.Fatalf("the derived chain described %d units, want %d", goldenRes.Units, synthDerivedUnits)
	}
	want := storeState(t, golden)

	dir := t.TempDir()
	disarm := crashpoint.Arm(cpStageComplete)
	_, err = newSynthCoordinator(t, dir, echoStub(), crashWorkers, log.Discard()).
		Run(context.Background(), synthDerivedPlan(t, dir), ModeResume)
	disarm()
	var crash *crashpoint.Crash
	if !errors.As(err, &crash) {
		t.Fatalf("the run did not die at the stage boundary; err = %v", err)
	}
	clearLock(t, dir)

	client := echoStub()
	res, err := newSynthCoordinator(t, dir, client, crashWorkers, log.Discard()).
		Run(context.Background(), synthDerivedPlan(t, dir), ModeResume)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !res.DeliveryReady() {
		t.Fatalf("the resumed derived run is not emit-ready: %+v", res)
	}
	assertSameState(t, want, storeState(t, dir), "after a kill at the derived stage boundary")

	// Stage 1 was proven and skipped; only the derived stage was executed.
	if res.Reused != 1 || res.Produced != synthDerivedUnits-1 {
		t.Errorf("reused %d / produced %d, want 1 / %d", res.Reused, res.Produced, synthDerivedUnits-1)
	}
	if got := client.callCount(); got != res.Produced {
		t.Errorf("%d calls to produce %d units", got, res.Produced)
	}
}

// TestSweepClearsWhatTheChainDoesNotAccountFor: the resume scan only inspects
// paths the chain names, so a killed write's temp file and a previous plan's
// orphaned artifacts are invisible to it and accumulate in a directory that is
// (or becomes) the emitted tree. A completed run removes them.
//
// The orphan carries a perfectly valid stamp of its own, which is the point:
// it is not incoherent and no verdict would ever catch it. Only "the chain
// does not account for this" does.
func TestSweepClearsWhatTheChainDoesNotAccountFor(t *testing.T) {
	dir := t.TempDir()

	// A kill before a rename leaves the temp-file residue behind.
	runUntilCrash(t, dir, cpPutPreRename, 1)
	clearLock(t, dir)

	// And a previous plan's artifact, stamped and internally consistent.
	const orphan = "leaves/from-an-older-taxonomy.md"
	store := synthStore(t, dir, log.Discard())
	if err := store.Put(orphan, []byte("a previous plan's leaf\n"), []Input{corpusInput}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, res := runToCompletion(t, dir, ModeResume); !res.DeliveryReady() {
		t.Fatalf("the completing run is not emit-ready: %+v", res)
	}

	for path := range storeState(t, dir) {
		if path == orphan || path == orphan+StampSuffix {
			t.Errorf("%s survived the sweep", path)
		}
	}
	entries, err := os.ReadDir(filepath.Join(storeRoot(dir), "survey"))
	if err != nil {
		t.Fatalf("read the job dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), tempSuffix) {
			t.Errorf("temp-file residue %s survived the sweep", e.Name())
		}
	}

	// And what the chain does account for is untouched.
	uninterrupted := t.TempDir()
	runToCompletion(t, uninterrupted, ModeResume)
	assertSameState(t, storeState(t, uninterrupted), storeState(t, dir), "after a sweep")
}

// TestAHardKillsLockfileRefusesEveryModeUntilRemoved pins the one piece of
// real-kill state the in-process harness cannot produce, and the qualification
// it puts on "--fresh is always sufficient".
//
// A SIGKILL or a power loss leaves job.lock behind, and the lock is
// deliberately never broken automatically (see AcquireLock). So after a hard
// kill BOTH modes refuse — resume and --fresh alike — until a human deletes
// one file, and then both work. That is the whole remedy, and it is stated in
// ARCHITECTURE §12 because it is the only manual step this design has.
func TestAHardKillsLockfileRefusesEveryModeUntilRemoved(t *testing.T) {
	dir := t.TempDir()
	runToCompletion(t, dir, ModeResume)
	want := storeState(t, dir)

	// What the kill left: a lockfile whose holder is gone.
	held, err := AcquireLock(storeRoot(dir), log.Discard())
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	_ = held // never released; the "process" it belonged to died here

	for _, mode := range []Mode{ModeResume, ModeFresh} {
		_, err := newSynthCoordinator(t, dir, echoStub(), crashWorkers, log.Discard()).
			Run(context.Background(), synthPlan(t), mode)
		var locked LockedError
		if !errors.As(err, &locked) {
			t.Fatalf("%s mode ran over a locked job dir; err = %v", mode, err)
		}
		if !strings.Contains(locked.Error(), LockFileName) {
			t.Errorf("the refusal does not name the file to delete: %v", locked)
		}
	}

	clearLock(t, dir)

	for _, mode := range []Mode{ModeResume, ModeFresh} {
		runToCompletion(t, dir, mode)
		assertSameState(t, want, storeState(t, dir), "after deleting a stale lockfile, "+string(mode))
	}
}

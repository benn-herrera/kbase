package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/log"
	"kbase/internal/log/logtest"
	"kbase/internal/model"
	"kbase/internal/prompt"
)

// runSynth runs the synthetic plan to completion in dir. Failures inside the
// job are returned in the result, not as an error.
func runSynth(t *testing.T, dir string, client model.Client, workers int, lg log.Logger, mode Mode) (JobResult, error) {
	t.Helper()
	return newSynthCoordinator(t, dir, client, workers, lg).Run(context.Background(), synthPlan(t), mode)
}

// TestPlanChainDerivesTheWorklist: the chain resume verdicts and the streams
// workers run are ONE description. Two would be two things to keep in
// agreement, and the resume filter reads the verdicts of the first to decide
// the second.
func TestPlanChainDerivesTheWorklist(t *testing.T) {
	chain := synthPlan(t).Chain()
	if len(chain) != 4 {
		t.Fatalf("chain has %d stages, want 4", len(chain))
	}
	if chain[0].Name != "survey" || chain[1].Name != synthFoldStage ||
		chain[2].Name != synthProduceStage || chain[3].Name != "leaves" {
		t.Errorf("stage names = %q/%q/%q/%q", chain[0].Name, chain[1].Name, chain[2].Name, chain[3].Name)
	}
	store, _ := newStore(t)
	n := 0
	for i := range chain {
		// resolveStage is both the description and its validation: unique
		// paths, upstreams produced strictly earlier or already proven.
		units, err := store.resolveStage(chain, i)
		if err != nil {
			t.Fatalf("the derived chain is not a valid one: %v", err)
		}
		n += len(units)
	}
	if n != synthUnits {
		t.Errorf("chain describes %d units, plan has %d", n, synthUnits)
	}
}

func TestCoordinatorRunsEveryUnit(t *testing.T) {
	dir := t.TempDir()
	client := echoStub()
	lg := &logtest.Capture{}

	res, err := runSynth(t, dir, client, 2, lg, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != synthUnits || res.Reused != 0 || len(res.Failures) != 0 {
		t.Errorf("result = %+v, want %d produced and nothing else", res, synthUnits)
	}
	if !res.EmitReady() {
		t.Error("a complete job with no failures must be emit-ready")
	}
	// More calls than units, because the fold spends synthFoldCalls of them
	// on one artifact.
	if client.callCount() != synthCalls {
		t.Errorf("%d calls for %d units", client.callCount(), synthUnits)
	}
	// Usage is aggregated per call into the job total.
	if want := synthCalls * 100; res.Usage.PromptTokens != want {
		t.Errorf("PromptTokens = %d, want %d", res.Usage.PromptTokens, want)
	}

	// Every unit is on disk with a stamp beside it.
	store := synthStore(t, dir, lg)
	chain := synthPlan(t).Chain()
	for i := range chain {
		units, err := store.resolveStage(chain, i)
		if err != nil {
			t.Fatalf("resolveStage(%d): %v", i, err)
		}
		for _, u := range units {
			inputs, err := store.resolveInputs(u)
			if err != nil {
				t.Fatalf("ResolveInputs(%s): %v", u.Path, err)
			}
			v, reason, err := store.verify(u.Path, inputs)
			if err != nil {
				t.Fatalf("verify(%s): %v", u.Path, err)
			}
			if v != VerdictValid {
				t.Errorf("%s is %s: %s", u.Path, v, reason)
			}
		}
	}
	// The lock is released on the way out; a job dir left locked after a
	// clean run would refuse every later run.
	if _, err := AcquireLock(storeRoot(dir), lg); err != nil {
		t.Errorf("the job dir is still locked after a clean run: %v", err)
	}
}

// TestCoordinatorPhaseTraversal: the workers walk the matrix, and reference
// buffer A is flushed only where the matrix allows it. The section count is
// the plan's: three streams, one of which changes section once.
func TestCoordinatorPhaseTraversal(t *testing.T) {
	lg := &logtest.Capture{}
	if _, err := runSynth(t, t.TempDir(), echoStub(), 1, lg, ModeResume); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, phase := range []Phase{PhaseJobSetup, PhaseStageSetup, PhaseCallLoop, PhaseSectionTransition, PhaseStageTeardown} {
		if !lg.Has(t, "debug", "phase", phase.String()) {
			t.Errorf("%s was never entered", phase)
		}
	}
	// One transition per stream (five of them, the mechanical stage's
	// included) plus one mid-stream section change.
	if got, want := lg.Count("debug", "phase", PhaseSectionTransition.String()), 6; got != want {
		t.Errorf("%d section transitions, want %d", got, want)
	}
	if got, want := lg.Count("debug", "phase", PhaseStageSetup.String()), 4; got != want {
		t.Errorf("%d stage setups for %d stages", got, want)
	}
}

// TestCoordinatorRefusesTaskOwnedRefA: slot 4 is the worker's, flushed at
// section transitions. A task that set it directly would be a per-call buffer
// wearing the multi-call buffer's position, which is what the §7 ordering
// cannot survive — so it is refused rather than overwritten.
func TestCoordinatorRefusesTaskOwnedRefA(t *testing.T) {
	plan := synthPlan(t)
	streams, err := plan.Stages[0].resolve()
	if err != nil {
		t.Fatal(err)
	}
	// The refusal is against the BUILT input, so the sneak goes in the
	// builder — which is the only place a call-time input could carry one.
	sneaky := streams[0].Tasks[0].Input()
	sneaky.RefA = "sneaked in"
	streams[0].Tasks[0].Input = ConstInput(sneaky)

	_, err = newSynthCoordinator(t, t.TempDir(), echoStub(), 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	var refA TaskOwnsRefAError
	if !errors.As(err, &refA) {
		t.Fatalf("err = %v, want a TaskOwnsRefAError", err)
	}
}

// TestCoordinatorGracefulDegradation: one essential unit fails, its siblings
// still finish, the inventory names it, and emission is refused.
func TestCoordinatorGracefulDegradation(t *testing.T) {
	dir := t.TempDir()
	// The survey stage is the essential one; reject the response for its
	// second unit both times, whichever call that turns out to be.
	client := &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		if strings.Contains(req.Messages[0].Content, "survey/b.json") {
			return model.Response{Content: "not data at all", FinishReason: "stop"}, nil
		}
		return model.Response{Content: synthAccept + " fine", FinishReason: "stop"}, nil
	}}

	res, err := runSynth(t, dir, client, 2, log.Discard(), ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	byPath := map[string]UnitFailure{}
	for _, f := range res.Failures {
		byPath[f.Path] = f
	}
	if f := byPath["survey/b.json"]; f.Kind != FailureVerification || f.Stage != "survey" {
		t.Errorf("survey/b.json failed as %+v, want %s in survey", f, FailureVerification)
	}
	// Its two dependents cascade — and cost nothing, because a unit that
	// failed this run is known before the stage that consumes it starts. Each
	// names the failure that caused it, which is the remedy for all three.
	for _, path := range []string{"leaves/d2/one.md", "leaves/d2/two.md"} {
		f := byPath[path]
		if f.Kind != FailureCascade {
			t.Errorf("%s failed as %q, want %s", path, f.Kind, FailureCascade)
		}
		var cascade CascadeFailureError
		if !errors.As(f.Err, &cascade) {
			t.Errorf("%s carries %v, want a CascadeFailureError", path, f.Err)
		} else if cascade.Upstream != "survey/b.json" || cascade.Root != "survey/b.json" {
			t.Errorf("%s cascade = %+v, want it to name survey/b.json as cause and root", path, cascade)
		}
	}
	if len(res.Failures) != 3 {
		t.Errorf("failures = %+v, want the failed unit and its two dependents", res.Failures)
	}
	if res.EmitReady() {
		t.Error("a job with an essential failure must refuse emission")
	}
	// The sibling in the same stream finished, and so did the downstream
	// units that did not depend on the failed one.
	if res.Produced != synthUnits-3 {
		t.Errorf("Produced = %d, want %d — a unit failure stopped its siblings", res.Produced, synthUnits-3)
	}
	// The produced units, one call each except the fold's and the mechanical
	// stage's, plus the rejected unit's two attempts.
	if want := (synthUnits - 3 - synthProduceUnits) + (synthFoldCalls - 1) + semanticAttempts; client.callCount() != want {
		t.Errorf("%d calls, want %d; the cascaded units must not have consulted the model",
			client.callCount(), want)
	}
	if _, err := synthStore(t, dir, log.Discard()).readStamp("survey/b.json"); err == nil {
		t.Error("a failed essential unit left an artifact behind")
	}
}

// TestCoordinatorSlotOneIsJobConstant is the §7 claim armed at the level it
// is made: slot 1 is job-constant, and a plan whose second stage carries a
// different system frame cannot spend the cross-stage prefix on it.
//
// The frame is a job-level field, so the stage's own spec has no vote — the
// run below carries a deliberately divergent one and lands byte-for-byte where
// a plan without it does. The other half of the guarantee, that a stage which
// somehow DID render a different slot 1 is refused rather than shipped, is
// TestSlotOneIsMeasuredAgainstTheJob.
func TestCoordinatorSlotOneIsJobConstant(t *testing.T) {
	golden := t.TempDir()
	if _, err := runSynth(t, golden, echoStub(), 1, log.Discard(), ModeResume); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := storeState(t, golden)

	plan := synthPlan(t)
	plan.Stages[1].Spec.SystemFrame = "Job: a second frame nobody asked for."

	dir := t.TempDir()
	client := echoStub()
	res, err := newSynthCoordinator(t, dir, client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.EmitReady() {
		t.Fatalf("result = %+v, want a clean run", res)
	}
	// Every artifact is a digest of the prompt that produced it, so identical
	// output means stage 2's calls carried the job's frame and not its own.
	assertSameState(t, want, storeState(t, dir), "with a divergent stage frame in the plan")
	for _, p := range client.recorded() {
		if strings.Contains(p, "a second frame nobody asked for") {
			t.Fatal("a stage's own system frame reached the wire")
		}
	}
}

// TestCoordinatorCountsAFailedUnitsTokens: the job total includes what the
// units that produced nothing spent. §12 wants the accounting for cost, and an
// aggregate that dropped the failures would understate exactly the units worth
// looking at — two full semantic attempts each, reported as free.
func TestCoordinatorCountsAFailedUnitsTokens(t *testing.T) {
	const perCall = 100
	client := &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		usage := model.Usage{PromptTokens: perCall, CompletionTokens: 10, TotalTokens: perCall + 10}
		if strings.Contains(req.Messages[0].Content, "survey/b.json") {
			return model.Response{Content: "not data at all", FinishReason: "stop", Usage: usage}, nil
		}
		return model.Response{Content: synthAccept + " fine", FinishReason: "stop", Usage: usage}, nil
	}}

	res, err := runSynth(t, t.TempDir(), client, 2, log.Discard(), ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The produced units cost one call each except the fold's and the
	// mechanical stage's, and the failed one made both its semantic attempts.
	// Its two dependents cascaded and cost nothing.
	if want := (synthUnits - 3 - synthProduceUnits + synthFoldCalls - 1 + semanticAttempts) * perCall; res.Usage.PromptTokens != want {
		t.Errorf("Usage.PromptTokens = %d, want %d — a failed unit's tokens went unaccounted",
			res.Usage.PromptTokens, want)
	}
	var failed UnitFailure
	for _, f := range res.Failures {
		if f.Path == "survey/b.json" {
			failed = f
		}
	}
	if failed.Usage.PromptTokens != semanticAttempts*perCall {
		t.Errorf("the failure itself carries %d prompt tokens, want %d",
			failed.Usage.PromptTokens, semanticAttempts*perCall)
	}
	if failed.SemanticAttempts != semanticAttempts {
		t.Errorf("SemanticAttempts = %d, want %d", failed.SemanticAttempts, semanticAttempts)
	}
}

// TestCoordinatorStoppedJobIsNotEmitReady: a run that broke before it had
// described every stage cannot be emit-ready on a count of what it did
// describe. Under the dynamic chain that count is a partial one by
// construction, so the stage tally is what refuses.
func TestCoordinatorStoppedJobIsNotEmitReady(t *testing.T) {
	plan := synthPlan(t)
	plan.Stages[1].Streams = func() ([]DomainStream, error) {
		return nil, errors.New("the skeleton this stage derives from is unreadable")
	}

	res, err := newSynthCoordinator(t, t.TempDir(), echoStub(), 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err == nil {
		t.Fatal("a stage that cannot be described must stop the job")
	}
	if res.EmitReady() {
		t.Errorf("result = %+v; a job whose chain was never fully described is not emit-ready", res)
	}
	if res.Stages != 1 {
		t.Errorf("Stages = %d, want the one stage that was described", res.Stages)
	}
}

// TestCoordinatorDegradedUnitsStillEmit: a refinement seam that fell back
// produced a correct artifact. The job is emit-ready and says it is degraded —
// model failure costs quality, not correctness.
func TestCoordinatorDegradedUnitsStillEmit(t *testing.T) {
	dir := t.TempDir()
	client := &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		if strings.Contains(req.Messages[0].Content, "leaves/") {
			return model.Response{Content: "not data at all", FinishReason: "stop"}, nil
		}
		return model.Response{Content: synthAccept + " fine", FinishReason: "stop"}, nil
	}}

	res, err := runSynth(t, dir, client, 2, log.Discard(), ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Degraded != 4 || res.Produced != synthUnits || len(res.Failures) != 0 {
		t.Errorf("result = %+v, want 4 degraded and all %d produced", res, synthUnits)
	}
	if !res.EmitReady() {
		t.Error("a degraded but complete job is still emit-ready")
	}
	data, err := readArtifact(dir, "leaves/d1/one.md")
	if err != nil {
		t.Fatalf("read leaf: %v", err)
	}
	if !strings.Contains(data, synthBaselineText) {
		t.Errorf("leaf = %q, want the mechanical baseline", data)
	}
}

// TestCoordinatorBudgetRefusalIsInventoried: refuse-and-split reaches the
// report as its own failure kind, because the remedy is different — the unit
// goes back to the stage that sizes units.
func TestCoordinatorBudgetRefusalIsInventoried(t *testing.T) {
	plan := synthPlan(t)
	// The survey stage and the fold stage, so the inventory covers both
	// shapes: a refusal at a unit's own call, and a refusal at one of the
	// CALLS an artifact is composed from.
	for _, i := range []int{0, 1} {
		plan.Stages[i].Spec.Budgets = prompt.Budgets{PerSlot: map[prompt.Slot]int{prompt.SlotContent: 1}}
	}
	client := echoStub()

	res, err := newSynthCoordinator(t, t.TempDir(), client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Two survey units and the fold's first boundary refused; four leaves and
	// the two mechanical units cascaded off the survey, and the fold's
	// artifact off its own poisoned call.
	if len(res.Failures) != 10 {
		t.Fatalf("failures = %+v, want the three refusals and their seven cascades", res.Failures)
	}
	kinds := map[FailureKind]int{}
	for _, f := range res.Failures {
		kinds[f.Kind]++
	}
	// Three kinds, three stories: the refusals themselves, the composed
	// artifact one of them poisoned inside its own stream, and the six units
	// of later stages that consume what the refusals did not produce.
	if kinds[FailureBudget] != 3 || kinds[FailureUpstream] != 1 || kinds[FailureCascade] != 6 {
		t.Errorf("kinds = %v, want 3 %s, 1 %s and 6 %s",
			kinds, FailureBudget, FailureUpstream, FailureCascade)
	}
	if client.callCount() != 0 {
		t.Errorf("%d calls went on the wire for prompts that never built", client.callCount())
	}
	// Sorted by path, so a broken job reports the same inventory twice.
	if !slices.IsSortedFunc(res.Failures, func(a, b UnitFailure) int { return strings.Compare(a.Path, b.Path) }) {
		t.Error("the failure inventory is not sorted by path")
	}
}

// TestCoordinatorNonUnitErrorStopsTheJob: some failures are not a unit's.
// A misconfigured tier, like a frozen-prompt violation, would fail every
// remaining call in the same way, so the worker stops rather than inventorying
// the same defect once per unit. (The abort classes themselves are exercised
// against the runner directly — with one shared StageContext per stage, a
// frozen-prompt violation is not reachable through the coordinator.)
func TestCoordinatorNonUnitErrorStopsTheJob(t *testing.T) {
	plan := synthPlan(t)
	cfg := synthConfigWithout(t, plan.Stages[0].Role.Tier)
	store := NewStore(t.TempDir(), log.Discard())
	coord := NewCoordinator(store, NewCallRunner(echoStub(), cfg, log.Discard()), 2, log.Discard())

	res, err := coord.Run(context.Background(), plan, ModeResume)
	if err == nil {
		t.Fatal("a misconfigured tier must stop the job, not be inventoried per unit")
	}
	if res.EmitReady() {
		t.Error("a job that stopped early must not be emit-ready")
	}
}

// TestCoordinatorLockRefusesASecondWriter: two runs over one job dir would
// interleave artifacts into something a later resume would happily trust.
func TestCoordinatorLockRefusesASecondWriter(t *testing.T) {
	dir := t.TempDir()
	held, err := AcquireLock(storeRoot(dir), log.Discard())
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer held.Release()

	_, err = runSynth(t, dir, echoStub(), 1, log.Discard(), ModeResume)
	var target LockedError
	if !errors.As(err, &target) {
		t.Fatalf("err = %v, want LockedError", err)
	}
}

// TestCoordinatorConcurrency: the pool is bounded, every goroutine exits, and
// the run is race-clean (the gate runs this under -race). Streams outnumber
// workers so the feed channel is exercised rather than drained in one go.
func TestCoordinatorConcurrency(t *testing.T) {
	plan := synthPlan(t)
	// Fan the refinement stage out across more domains than there are
	// workers.
	var streams []DomainStream
	for _, d := range []string{"d1", "d2", "d3", "d4", "d5", "d6"} {
		streams = append(streams, DomainStream{Domain: d, Tasks: []Task{
			synthTask(filepath.ToSlash("leaves/"+d+"/one.md"), "s1", "survey/a.json"),
			synthTask(filepath.ToSlash("leaves/"+d+"/two.md"), "s2", "survey/b.json"),
		}})
	}
	plan.Stages[3].Streams = staticStreams(streams...)
	want := 2 + 1 + synthProduceUnits + 12

	res, err := newSynthCoordinator(t, t.TempDir(), echoStub(), 3, &logtest.Capture{}).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != want || len(res.Failures) != 0 {
		t.Errorf("result = %+v, want %d produced", res, want)
	}
	if res.Units != want {
		t.Errorf("Units = %d, want %d", res.Units, want)
	}
}

// TestCoordinatorCancellation: a cancelled context stops the job rather than
// degrading every remaining unit through a seam it was never offered to.
func TestCoordinatorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &stubClient{respond: func(n int, req model.Request) (model.Response, error) {
		if n >= 1 {
			cancel()
		}
		return model.Response{Content: synthAccept + " fine", FinishReason: "stop"}, nil
	}}

	res, err := newSynthCoordinator(t, t.TempDir(), client, 2, log.Discard()).
		Run(ctx, synthPlan(t), ModeResume)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if res.EmitReady() {
		t.Error("a cancelled job must not be emit-ready")
	}
}

// readArtifact reads one artifact out of a job dir.
func readArtifact(dir, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(storeRoot(dir), filepath.FromSlash(rel)))
	return string(data), err
}

// TestTaskInputIsBuiltAtTheUnit: a task's prompt input is assembled when the
// worker reaches it, not when the stage was described.
//
// It is what stage 4's fold needs — each boundary's question comes off the cut
// list the previous boundary produced — so the test states the same shape: a
// stream whose second call must carry a value that did not exist when the
// stream was described.
func TestTaskInputIsBuiltAtTheUnit(t *testing.T) {
	answered := 0
	role := essentialRole(t)
	inner := role.Verify
	role.Verify = func(unit, response string) (any, error) {
		art, err := inner(unit, response)
		if err == nil {
			answered++
		}
		return art, err
	}

	tasks := make([]Task, 0, 2)
	for _, path := range []string{"leaves/one.md", "leaves/two.md"} {
		base := synthTask(path, "s1")
		base.Input = func() prompt.CallInput {
			in := synthTask(path, "s1").Input()
			in.Content = fmt.Sprintf("%s, with %d answered before it", in.Content, answered)
			return in
		}
		tasks = append(tasks, base)
	}

	client := echoStub()
	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{{
		Name: "leaves", Role: role, Spec: synthSpec("Task: synthetic."),
		Streams: staticStreams(DomainStream{Domain: "d", Tasks: tasks}),
	}}}
	res, err := newSynthCoordinator(t, t.TempDir(), client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 2 || len(res.Failures) != 0 {
		t.Fatalf("result = %+v, want both units", res)
	}
	prompts := client.recorded()
	if len(prompts) != 2 {
		t.Fatalf("%d prompts, want one per unit", len(prompts))
	}
	if !strings.Contains(prompts[0], "with 0 answered before it") {
		t.Errorf("the first call was not built against the state it ran under:\n%s", prompts[0])
	}
	// The claim: a value that changed AFTER the stage was described reached
	// the second prompt. A stored input could not carry it.
	if !strings.Contains(prompts[1], "with 1 answered before it") {
		t.Errorf("the second call was built before its predecessor ran:\n%s", prompts[1])
	}
}

// TestCallOnlyTasksProduceNoArtifact: a CallOnly task is a call whose result
// is the stage's own state. It writes nothing, describes no unit, and is
// therefore invisible to the resume scan — which is what makes a multi-call
// artifact (stage 4's fold) redone whole rather than resumed against a prefix
// no stamp describes.
func TestCallOnlyTasksProduceNoArtifact(t *testing.T) {
	dir := t.TempDir()
	tasks := []Task{synthTask("leaves/one.md", "s1"), synthTask("leaves/two.md", "s1")}
	tasks[0].CallOnly = true

	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{{
		Name: "leaves", Role: essentialRole(t), Spec: synthSpec("Task: synthetic."),
		Streams: staticStreams(DomainStream{Domain: "d", Tasks: tasks}),
	}}}
	client := echoStub()
	res, err := newSynthCoordinator(t, dir, client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Units != 1 || res.Produced != 1 || len(res.Failures) != 0 {
		t.Fatalf("result = %+v, want two calls behind one unit", res)
	}
	if !res.EmitReady() {
		t.Error("a stage whose calls outnumber its units is still complete")
	}
	if n := client.callCount(); n != 2 {
		t.Errorf("%d calls, want one per task", n)
	}
	if _, err := os.Stat(filepath.Join(storeRoot(dir), filepath.FromSlash("leaves/one.md"))); !os.IsNotExist(err) {
		t.Errorf("the CallOnly task left an artifact behind (err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(storeRoot(dir), filepath.FromSlash("leaves/two.md"))); err != nil {
		t.Errorf("the producing task wrote nothing: %v", err)
	}

	// And the stage is complete on a rescan: the calls it made are not units
	// anyone is waiting for.
	scan, err := synthStore(t, dir, log.Discard()).Scan(plan.Chain(), ModeResume)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !scan.Complete() || scan.Reused != 1 {
		t.Errorf("scan = %+v, want the one unit proven and the stage complete", scan)
	}
}

// TestCallOnlyDegradationIsCounted: a CallOnly call that kept its mechanical
// baseline degraded the artifact it fed, even though it wrote nothing itself.
// Counting only produced units would report a composed artifact with a
// fallback in it as clean.
func TestCallOnlyDegradationIsCounted(t *testing.T) {
	tasks := []Task{synthTask("leaves/one.md", "s1"), synthTask("leaves/two.md", "s1")}
	tasks[0].CallOnly = true

	// Refuse the first call twice, so it exhausts its attempts and falls back.
	client := &stubClient{respond: func(n int, req model.Request) (model.Response, error) {
		if strings.Contains(req.Messages[0].Content, "leaves/one.md") {
			return model.Response{Content: "not the token", FinishReason: "stop"}, nil
		}
		return model.Response{Content: synthAccept + " fine", FinishReason: "stop"}, nil
	}}
	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{{
		Name: "leaves", Role: refinementRole(t), Spec: synthSpec("Task: synthetic."),
		Streams: staticStreams(DomainStream{Domain: "d", Tasks: tasks}),
	}}}
	res, err := newSynthCoordinator(t, t.TempDir(), client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.Degraded != 1 || len(res.Failures) != 0 {
		t.Fatalf("result = %+v, want the fallback counted against one produced unit", res)
	}
}

// TestAFailedCallPoisonsTheArtifactItFeeds is the R-Q-A ruling made
// falsifiable: the artifact exists exactly when EVERY call of its stream
// succeeded.
//
// The failure this closes is quiet. A CallOnly failure refuses the run it
// happens in, but if the producing task still wrote, the artifact would carry
// a perfectly valid stamp — nothing in a stamp knows a call failed — so the
// next run would verdict it Valid, drop the whole stream, and the failure
// would have existed in one JobResult and nowhere else. Which is why the third
// assertion is the load-bearing one: the next run redoes the fold WHOLE.
func TestAFailedCallPoisonsTheArtifactItFeeds(t *testing.T) {
	dir := t.TempDir()
	// An essential role, so a response that does not verify is a FAILURE and
	// not a fallback: the fold's own seam has a baseline, and what is under
	// test here is what a failure does.
	stage := func() Plan {
		tasks := []Task{
			synthTask("fold/0001.boundary", "span"),
			synthTask("fold/0002.boundary", "span"),
			synthTask(synthFoldUnit, "span"),
		}
		tasks[0].CallOnly, tasks[1].CallOnly = true, true
		return Plan{SystemFrame: synthFrame, Stages: []*StagePlan{{
			Name: synthFoldStage, Role: essentialRole(t), Spec: synthSpec("Task: synthetic."),
			Streams: staticStreams(DomainStream{Domain: synthFoldStage, Tasks: tasks}),
		}}}
	}

	// The first boundary's call never verifies; every other call would.
	poison := &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		if strings.Contains(req.Messages[0].Content, "fold/0001.boundary") {
			return model.Response{Content: "not the token", FinishReason: "stop"}, nil
		}
		return model.Response{Content: synthAccept + " fine", FinishReason: "stop"}, nil
	}}
	res, err := newSynthCoordinator(t, dir, poison, 1, log.Discard()).
		Run(context.Background(), stage(), ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// (a) The job is refused, and it names both the call that failed and the
	// artifact that could not be written because of it.
	if res.EmitReady() {
		t.Errorf("result = %+v; a stream with a failed call owes an artifact it did not write", res)
	}
	kinds := map[string]FailureKind{}
	for _, f := range res.Failures {
		kinds[f.Path] = f.Kind
	}
	if kinds["fold/0001.boundary"] != FailureVerification {
		t.Errorf("the failed call is %+v, want %s", res.Failures, FailureVerification)
	}
	if kinds[synthFoldUnit] != FailureUpstream {
		t.Errorf("the artifact is %+v, want %s — the cascade of the call beside it", res.Failures, FailureUpstream)
	}
	// The calls after the poison are not spent: they would feed state nobody
	// is going to record. Two semantic attempts at the first boundary, nothing
	// else.
	if got := poison.callCount(); got != semanticAttempts {
		t.Errorf("%d calls, want %d: a poisoned stream stops spending", got, semanticAttempts)
	}

	// (b) Nothing landed on disk.
	if state := storeState(t, dir); len(state) != 0 {
		t.Errorf("the store holds %v; a stream with a failed call writes nothing at all", state)
	}

	// (c) And the next run redoes the fold whole — every call remade, because
	// there is nothing for the scan to verdict Valid.
	good := echoStub()
	res, err = newSynthCoordinator(t, dir, good, 1, log.Discard()).
		Run(context.Background(), stage(), ModeResume)
	if err != nil {
		t.Fatalf("the second run: %v", err)
	}
	if !res.EmitReady() || res.Produced != 1 || res.Reused != 0 {
		t.Fatalf("result = %+v, want the fold redone whole and the artifact written", res)
	}
	if got := good.callCount(); got != synthFoldCalls {
		t.Errorf("%d calls on the redo, want all %d of the stream's", got, synthFoldCalls)
	}
	if _, err := os.Stat(filepath.Join(storeRoot(dir), filepath.FromSlash(synthFoldUnit))); err != nil {
		t.Errorf("the redone fold wrote nothing: %v", err)
	}
}

// TestCascadeCrossesStagesWithoutSpending is the F-8 extension made
// falsifiable: a unit whose upstream failed THIS RUN is not attempted at all,
// and neither is what depends on it, all the way down the chain.
//
// The chain here is three stages deep and the middle one is mechanical, so the
// claim covers both task modes: the model stage below the failure makes no
// call, and the producer between them never runs. Each cascade names the
// failure at the head of the chain rather than its immediate predecessor,
// which is the difference between an inventory that states one remedy and one
// that makes a reader walk the chain a hop at a time.
func TestCascadeCrossesStagesWithoutSpending(t *testing.T) {
	dir := t.TempDir()
	const root = "survey/a.json"
	synthProduceRuns.Store(0)

	// The one call this run has any business making is the one that fails.
	client := &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		if !strings.Contains(req.Messages[0].Content, root) {
			t.Errorf("a call was made below a failed unit:\n%s", req.Messages[0].Content)
		}
		return model.Response{Content: "not data at all", FinishReason: "stop"}, nil
	}}

	const middle = "assemble/one.md"
	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{
		{
			Name: "survey", Role: essentialRole(t), Spec: synthSpec("Task: inventory."),
			Streams: staticStreams(DomainStream{Domain: "corpus", Tasks: []Task{synthTask(root, "all")}}),
		},
		{
			Name: synthProduceStage, Encode: synthEncodeText,
			Streams: staticStreams(DomainStream{Domain: "d", Tasks: []Task{{
				Unit:    Unit{Path: middle, Inputs: []Input{corpusInput}, Upstreams: []string{root}},
				Produce: func() (any, error) { synthProduceRuns.Add(1); return synthProduceText(middle), nil },
			}}}),
		},
		{
			Name: "leaves", Role: essentialRole(t), Spec: synthSpec("Task: distil."),
			Streams: staticStreams(DomainStream{Domain: "d", Tasks: []Task{
				synthTask("leaves/one.md", "s1", middle),
			}}),
		},
	}}

	res, err := newSynthCoordinator(t, dir, client, 2, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	byPath := map[string]UnitFailure{}
	for _, f := range res.Failures {
		byPath[f.Path] = f
	}
	if len(res.Failures) != 3 {
		t.Fatalf("failures = %+v, want the failed unit and its two descendants", res.Failures)
	}
	if f := byPath[root]; f.Kind != FailureVerification {
		t.Errorf("the root failure is %+v, want %s", f, FailureVerification)
	}
	for _, tc := range []struct{ path, upstream string }{{middle, root}, {"leaves/one.md", middle}} {
		f := byPath[tc.path]
		if f.Kind != FailureCascade {
			t.Errorf("%s failed as %q, want %s", tc.path, f.Kind, FailureCascade)
		}
		var cascade CascadeFailureError
		if !errors.As(f.Err, &cascade) {
			t.Fatalf("%s carries %v, want a CascadeFailureError", tc.path, f.Err)
		}
		if cascade.Upstream != tc.upstream || cascade.Root != root {
			t.Errorf("%s cascade = %+v, want upstream %s and root %s", tc.path, cascade, tc.upstream, root)
		}
		if f.Usage != (model.Usage{}) || f.SemanticAttempts != 0 {
			t.Errorf("%s cost %+v over %d attempts; a cascade spends nothing",
				tc.path, f.Usage, f.SemanticAttempts)
		}
	}

	// Nothing was spent and nothing was written: one failed call, no producer
	// run, an empty store, and a job that refuses emission.
	if client.callCount() != semanticAttempts {
		t.Errorf("%d calls, want the failing unit's %d and nothing else", client.callCount(), semanticAttempts)
	}
	if n := synthProduceRuns.Load(); n != 0 {
		t.Errorf("the producer ran %d times under a failed upstream", n)
	}
	if state := storeState(t, dir); len(state) != 0 {
		t.Errorf("the store holds %v; nothing in this run had a recordable input set", state)
	}
	if res.EmitReady() {
		t.Error("a job whose chain failed at the top must refuse emission")
	}
	// And it is run-scoped: no stamp, no state, so the next run simply redoes
	// all three (here, the first of them, which fails the same way).
	scan, err := synthStore(t, dir, log.Discard()).Scan(plan.Chain(), ModeResume)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if scan.ResumeStage != 0 || scan.Reused != 0 {
		t.Errorf("scan = %+v, want the next run to start at the failure", scan)
	}
	if v := scan.Verdicts[0]; v.Verdict != VerdictAbsent {
		t.Errorf("verdict = %s, want %s: a cascade leaves nothing behind to verdict",
			v.Verdict, VerdictAbsent)
	}
}

// TestCascadeDropsTheCallsThatFedIt: a fold's calls come BEFORE the task that
// writes what they compose, so a cascade recognised at the unit would already
// have spent all of them on an artifact nobody was going to write.
func TestCascadeDropsTheCallsThatFedIt(t *testing.T) {
	const root = "survey/a.json"
	client := &stubClient{respond: func(_ int, req model.Request) (model.Response, error) {
		return model.Response{Content: "not data at all", FinishReason: "stop"}, nil
	}}

	fold := []Task{
		synthTask("fold/0001.boundary", "span", root),
		synthTask("fold/0002.boundary", "span", root),
		synthTask(synthFoldUnit, "span", root),
	}
	fold[0].CallOnly, fold[1].CallOnly = true, true
	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{
		{
			Name: "survey", Role: essentialRole(t), Spec: synthSpec("Task: inventory."),
			Streams: staticStreams(DomainStream{Domain: "corpus", Tasks: []Task{synthTask(root, "all")}}),
		},
		{
			Name: synthFoldStage, Role: essentialRole(t), Spec: synthSpec("Task: adjudicate."),
			Streams: staticStreams(DomainStream{Domain: synthFoldStage, Tasks: fold}),
		},
	}}

	res, err := newSynthCoordinator(t, t.TempDir(), client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Failures) != 2 {
		t.Fatalf("failures = %+v, want the failed unit and the artifact that cascaded off it", res.Failures)
	}
	if client.callCount() != semanticAttempts {
		t.Errorf("%d calls, want the failing unit's %d: a dropped artifact drops its calls with it",
			client.callCount(), semanticAttempts)
	}
}

// TestStageModeIsValidatedAtDescription: a task asks a model or produces its
// artifact itself, a stage is all of one kind, and the encoder lives wherever
// the stage's artifacts actually come from. Every violation is a plan defect,
// so it is refused when the stage is DESCRIBED — before a worker starts, and
// therefore before any of it could be half-executed.
func TestStageModeIsValidatedAtDescription(t *testing.T) {
	produce := func() (any, error) { return "produced\n", nil }
	model := synthTask("stage/model.md", "s1")
	mech := Task{Unit: Unit{Path: "stage/mech.md", Inputs: []Input{corpusInput}}, Produce: produce}

	both := model
	both.Produce = produce
	neither := model
	neither.Input = nil
	callOnly := mech
	callOnly.CallOnly = true

	tests := []struct {
		name   string
		tasks  []Task
		encode Encoder
		want   string
	}{
		{"both modes", []Task{both}, nil, "exactly one"},
		{"neither mode", []Task{neither}, nil, "exactly one"},
		{"a produce task that is CallOnly", []Task{callOnly}, synthEncodeText, "cannot be CallOnly"},
		{"a mixed stage", []Task{model, mech}, nil, "one or the other"},
		{"a mechanical stage with no encoder", []Task{mech}, nil, "needs StagePlan.Encode"},
		{"a model stage with one", []Task{model}, synthEncodeText, "encoded by its Role"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sp := &StagePlan{
				Name: "stage", Role: essentialRole(t), Spec: synthSpec("Task: synthetic."),
				Encode:  tc.encode,
				Streams: staticStreams(DomainStream{Domain: "d", Tasks: tc.tasks}),
			}
			_, err := sp.resolve()
			var target StageModeError
			if !errors.As(err, &target) {
				t.Fatalf("err = %v (%T), want a StageModeError", err, err)
			}
			if !strings.Contains(target.Reason, tc.want) {
				t.Errorf("reason = %q, want it to name %q", target.Reason, tc.want)
			}
			// The same description is what the scan reads, so the refusal
			// reaches a run through the chain as well.
			if _, err := sp.units(); !errors.As(err, &target) {
				t.Errorf("the chain resolved a stage the plan refuses: %v", err)
			}
		})
	}
}

// TestProduceRunsInPlaceOfTheCall: a mechanical stage's units are described,
// stamped, swept and resumed exactly like a model stage's — the only
// difference is that nothing was asked.
//
// The stage declares NO Role, which is the load-bearing half: a run that built
// an agent for it would be refused by Role.validate for having no verifier, no
// tier and no declared effort. Reaching a produced artifact at all proves the
// produce path never went near the prompt machinery.
func TestProduceRunsInPlaceOfTheCall(t *testing.T) {
	dir := t.TempDir()
	client := echoStub()
	synthProduceRuns.Store(0)

	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{synthProducePlan()}}
	plan.Stages[0].Streams = staticStreams(DomainStream{Domain: "d", Tasks: []Task{
		{Unit: Unit{Path: "assemble/one.md", Inputs: []Input{corpusInput}},
			Produce: func() (any, error) { synthProduceRuns.Add(1); return synthProduceText("assemble/one.md"), nil }},
	}})

	res, err := newSynthCoordinator(t, dir, client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Produced != 1 || res.Units != 1 || len(res.Failures) != 0 || !res.EmitReady() {
		t.Fatalf("result = %+v, want one produced unit and a clean job", res)
	}
	if client.callCount() != 0 {
		t.Errorf("%d calls; a mechanical stage consults nothing", client.callCount())
	}
	data, err := readArtifact(dir, "assemble/one.md")
	if err != nil {
		t.Fatalf("read the produced artifact: %v", err)
	}
	if data != synthProduceText("assemble/one.md") {
		t.Errorf("artifact = %q, want the producer's own bytes", data)
	}

	// And the proof beside it is the ordinary one, so the next run reuses it
	// and the producer does not run again.
	synthProduceRuns.Store(0)
	res, err = newSynthCoordinator(t, dir, client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("the second run: %v", err)
	}
	if res.Reused != 1 || res.Produced != 0 {
		t.Errorf("result = %+v, want the produced unit reused on its stamp", res)
	}
	if n := synthProduceRuns.Load(); n != 0 {
		t.Errorf("the producer ran %d times over a proven artifact", n)
	}
}

// TestProduceFailureIsInventoried: a producer's failure is a unit failure like
// any other — the siblings finish, the inventory names it under its own kind,
// nothing is written for it, and the job refuses emission.
func TestProduceFailureIsInventoried(t *testing.T) {
	dir := t.TempDir()
	broken := errors.New("the cut list does not tile the span")
	tasks := []Task{
		{Unit: Unit{Path: "assemble/bad.md", Inputs: []Input{corpusInput}},
			Produce: func() (any, error) { return nil, broken }},
		{Unit: Unit{Path: "assemble/good.md", Inputs: []Input{corpusInput}},
			Produce: func() (any, error) { return synthProduceText("assemble/good.md"), nil }},
	}
	plan := Plan{SystemFrame: synthFrame, Stages: []*StagePlan{{
		Name: synthProduceStage, Encode: synthEncodeText,
		Streams: staticStreams(DomainStream{Domain: "d", Tasks: tasks}),
	}}}

	client := echoStub()
	res, err := newSynthCoordinator(t, dir, client, 1, log.Discard()).
		Run(context.Background(), plan, ModeResume)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Path != "assemble/bad.md" {
		t.Fatalf("failures = %+v, want just the unit whose producer failed", res.Failures)
	}
	if f := res.Failures[0]; f.Kind != FailureProduce || !errors.Is(f.Err, broken) {
		t.Errorf("failure = %+v, want %s carrying the producer's own error", f, FailureProduce)
	}
	if res.Failures[0].Usage != (model.Usage{}) || res.Failures[0].SemanticAttempts != 0 {
		t.Errorf("failure = %+v, want no attempts and no tokens: nothing was asked", res.Failures[0])
	}
	if res.EmitReady() {
		t.Error("a job with a failed mechanical unit must refuse emission")
	}
	if res.Produced != 1 {
		t.Errorf("Produced = %d, want the sibling finished", res.Produced)
	}
	if _, err := readArtifact(dir, "assemble/bad.md"); !os.IsNotExist(err) {
		t.Errorf("the failed unit left something behind (err = %v)", err)
	}
	if client.callCount() != 0 {
		t.Errorf("%d calls; a mechanical stage consults nothing, failing or not", client.callCount())
	}
}

// TestAStreamMayCarryCallsAndOneArtifact: filterStreams keeps or drops a
// stream whole, so a stream carrying two artifacts and the calls that feed
// them cannot be filtered honestly — keeping it for the second artifact's sake
// re-spends the first's calls to feed state nobody records.
//
// Nothing builds that shape. The assertion is what makes the day someone does
// a refusal rather than a quiet bill.
func TestAStreamMayCarryCallsAndOneArtifact(t *testing.T) {
	calls := []Task{synthTask("fold/0001.boundary", "span")}
	calls[0].CallOnly = true
	one := synthTask("fold/a.txt", "span")
	two := synthTask("fold/b.txt", "span")

	// Two artifacts with no calls between them is the ordinary leaf stream and
	// is fine; adding a call is what makes the stream unfilterable.
	if _, err := filterStreams([]DomainStream{{Domain: "d", Tasks: []Task{one, two}}}, nil); err != nil {
		t.Errorf("a stream of independent artifacts is legal: %v", err)
	}
	_, err := filterStreams([]DomainStream{{
		Domain: "d", Tasks: []Task{calls[0], one, two},
	}}, nil)
	var target MultiArtifactStreamError
	if !errors.As(err, &target) {
		t.Fatalf("err = %v (%T), want a MultiArtifactStreamError", err, err)
	}
	if target.Artifacts != 2 || target.Domain != "d" {
		t.Errorf("err = %+v, want it to name the stream and how many artifacts it carries", target)
	}
}

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline/crashpoint"
	"kbase/internal/prompt"
)

// DefaultWorkers bounds the worker pool.
//
// The bound is not about cores: a worker spends its life waiting on a model
// call, so the pool is sized against the provider and against how much work is
// in flight and therefore lost to a kill, not against CPU. Four keeps a local
// single-GPU server (the reference deployment) busy without queueing requests
// behind each other, and keeps the resume cost of a crash to a handful of
// units. A hosted provider with real concurrency wants more; that is a
// configuration question, which is why this is a default and not a constant
// the coordinator reads directly.
const DefaultWorkers = 4

// cpStageComplete kills with a whole stage's artifacts on disk, every worker
// joined, and the chain's next link holding nothing at all. It is the case the
// deepest-valid-prefix walk exists to answer.
var cpStageComplete = crashpoint.Register("pipeline.coordinator.stage.complete")

// cpProduced kills a mechanical unit with its artifact derived and nothing
// written — the produce path's counterpart of pipeline.runner.verified, and the
// same claim: work that is done and unrecorded is redone, never guessed at.
var cpProduced = crashpoint.Register("pipeline.coordinator.produce.done")

// InputBuilder produces a task's per-call prompt input at the moment the
// worker reaches the task, rather than when the stage was described.
//
// Most stages know their inputs up front and pass ConstInput. A stage whose
// question depends on what the stage itself has decided so far cannot: stage
// 4's boundary fold judges each boundary against the cut list its earlier
// boundaries produced, so the window and the menu for boundary i do not exist
// until boundary i-1 has been adjudicated (ARCHITECTURE.md §5). Deferring the
// build is the whole of what that needs — a serial lane already guarantees
// the ordering, and everything downstream of the build is unchanged: the
// frozen-prompt assertion, the budget refusal and the churn tripwire all run
// against the BUILT input exactly as they did against a stored one.
//
// It returns no error on purpose. A builder assembles material the stage
// already holds and already verified; the failure that can arise from a call's
// size — a slot over its budget — is the prompt builder's to raise, and it
// raises it at Build with the refuse-and-split classification the runner
// already routes.
type InputBuilder func() prompt.CallInput

// ConstInput is the builder for a task whose per-call input is fully known
// when the stage is described — every stage but the fold.
func ConstInput(in prompt.CallInput) InputBuilder {
	return func() prompt.CallInput { return in }
}

// Producer derives a unit's artifact in process, with no model involved. It is
// the deterministic half of the pipeline (ARCHITECTURE.md §4: ingest, survey,
// distillation, assembly, verify) expressed as a task, so those stages get the
// resume, the stamps, the sweep accounting and the one worklist that the model
// stages already have.
//
// It runs where the runner would have run, and everything around it is the
// same: the unit is described identically, its input set is resolved and hashed
// BEFORE it runs, its artifact goes through the stage's encoder and ArtifactStore.Put,
// and a failure is an inventoried unit failure rather than an abort. A producer
// is not a call: it makes no prompt, touches no bound ask, and spends no tokens.
//
// It returns an error where InputBuilder does not, because the two failures are
// different. A builder assembles material the stage already verified; a
// producer does the stage's actual work, and a leaf whose cut list does not
// tile is a unit that failed.
type Producer func() (artifact any, err error)

// LaneTask is one unit of work: the artifact it must produce and the per-call
// context that produces it.
type LaneTask struct {
	// Owed describes the output artifact and what it is derived from. It is
	// the SAME description the resume scan verdicts, which is what keeps the
	// worklist stateless — there is no second place that says what a unit is.
	//
	// For a contributing task it is not an output description at all: only Path
	// is read, as the call's name in the log and the key the stage's verifier
	// correlates a response with.
	Owed OwedArtifact

	// Contributes marks a task whose result is STAGE STATE rather than an
	// artifact: the call is made, its response is verified, and nothing is
	// written.
	//
	// It exists for stage 4's boundary fold, where n model calls produce one
	// artifact. Each boundary's answer updates the stage's working cut list;
	// the composed list is the stage's only output, carried by the last task
	// of the lane. That is what makes the fold's resume stage-granular by
	// construction (ARCHITECTURE.md §12): there is no per-boundary artifact
	// for a scan to verdict Valid, so an interrupted fold is redone whole
	// rather than resumed against a prefix whose dependencies nothing
	// recorded.
	Contributes bool

	// Section names the section this unit belongs to. A change between
	// consecutive tasks is what drives the worker through
	// PhaseSectionTransition, which is the only phase that may flush
	// the stage reference buffer.
	Section string

	// SectionRef is the stage reference buffer's content for that section: the
	// orchestrator-curated material stable across the section's calls.
	SectionRef string

	// Input builds the per-call half of the prompt, with StageRef left EMPTY —
	// the worker owns slot 4 and fills it from SectionRef at the transition.
	// A non-empty StageRef is refused rather than overwritten, because silently
	// discarding a caller's buffer is how slot 4 would start churning per
	// call without anyone noticing.
	//
	// Exactly one of Input and Produce is set; see StagePlan.validate.
	Input InputBuilder

	// Produce derives the artifact in process instead of asking a model for
	// it. A task that has one asks nothing, so it has no Input, no Section
	// material that reaches a prompt, and no AskSpec behind it.
	Produce Producer
}

// SerialLane is one worker's whole assignment: a domain and its units in
// order (R-2). A worker owns a domain and processes it serially, so the
// stability state a lane depends on — previous-call hashes, the phases
// traversed, the current section buffer — is never shared.
type SerialLane struct {
	Domain string
	Tasks  []LaneTask
}

// LaneResolver produces a stage's serial lanes. It is called ONCE per
// job, when the chain walk reaches the stage — the plan half of the dynamic
// chain (see OwedArtifactResolver, which this backs).
//
// A stage whose work is known up front returns a fixed slice and ignores the
// laziness. A stage whose work is not — stage 4's leaves come out of the
// tree plan stage 3 emitted, stage 5's out of the cut list stage 4 verified —
// reads what its upstream left in the store and builds its lanes from that.
type LaneResolver func() ([]SerialLane, error)

// StagePlan is one stage: the ask every worker of the stage runs, the stage
// context spec they all share, and the serial lanes that produce its units.
//
// A stage is EITHER a model stage or a mechanical one, and the difference is
// which of the two task modes its tasks use (see StagePlan.validate for why
// mixing them is refused). A model stage carries AskSpec and Spec, and its
// artifacts are encoded by AskSpec.Encode. A mechanical stage — every task a
// Producer — carries neither, because there is nothing to tell a model and no
// seam to classify; its encoder is the field below, which is that stage's only
// piece of AskSpec-shaped state.
type StagePlan struct {
	Name string
	Ask  AskSpec
	Spec prompt.StageSpec

	// Encode renders a MECHANICAL stage's artifacts for the store — the
	// counterpart of AskSpec.Encode, which a stage with no AskSpec cannot have.
	// Required on a mechanical stage and refused on a model one: two
	// encoders where one is used is one silently ignored.
	Encode Encoder

	// Lanes resolves the stage's work when the stage is reached.
	Lanes LaneResolver

	// The resolution memo. It is what makes "called once" true: the scan
	// asks for this stage's units and the run then asks for its lanes,
	// and both must be looking at the same description — a resolver that
	// ran twice could read a store the run itself had changed in between.
	resolved   []SerialLane
	resolveErr error
	done       bool
}

// resolve produces the stage's lanes, once, and refuses a description whose
// task modes do not describe one coherent stage.
func (sp *StagePlan) resolve() ([]SerialLane, error) {
	if !sp.done {
		sp.resolved, sp.resolveErr = sp.Lanes()
		if sp.resolveErr == nil {
			sp.resolveErr = sp.validate()
		}
		sp.done = true
	}
	return sp.resolved, sp.resolveErr
}

// validate refuses a stage description that cannot be executed as one stage.
// It runs where the stage is DESCRIBED — which under the lazy chain is the
// first moment the tasks exist — so a plan defect is a loud refusal before any
// worker starts rather than a nil dereference in the middle of a run.
//
// Two rules, and they are the same rule seen at two scopes:
//
//   - A task asks a model or produces its artifact itself, never both and
//     never neither. Both would mean the plan does not know which one made the
//     artifact it is about to stamp; neither is a unit nothing could produce.
//     A Producer cannot contribute either: Contributes means "the result is the
//     stage's state, not an artifact", and a producer that writes nothing and
//     calls nothing is a task with no effect at all.
//   - A STAGE is uniform. Everything stage-scoped here is declared per stage
//     and not per task — one AskSpec, one shared StageContext, one seam, one
//     tier, one encoder — so a half-mechanical stage has no honest answer for
//     what its seam is or which encoder its artifacts went through. The
//     pipeline's stages are whole-stage mechanical or whole-stage inference
//     (ARCHITECTURE.md §4), so the uniformity costs nothing and buys the
//     absence of a state nothing could describe.
func (sp *StagePlan) validate() error {
	produce, ask := 0, 0
	for _, ds := range sp.resolved {
		for _, t := range ds.Tasks {
			switch {
			case (t.Produce == nil) == (t.Input == nil):
				return StageModeError{Stage: sp.Name, Path: t.Owed.Path,
					Reason: "a task sets exactly one of Produce and Input"}
			case t.Produce != nil && t.Contributes:
				return StageModeError{Stage: sp.Name, Path: t.Owed.Path,
					Reason: "a Produce task makes no call, so it cannot Contribute"}
			case t.Produce != nil:
				produce++
			default:
				ask++
			}
		}
	}
	switch {
	case produce > 0 && ask > 0:
		return StageModeError{Stage: sp.Name, Reason: fmt.Sprintf(
			"%d of its tasks produce their artifacts and %d ask a model; a stage is one or the other",
			produce, ask)}
	case produce > 0 && sp.Encode == nil:
		return StageModeError{Stage: sp.Name,
			Reason: "a mechanical stage needs StagePlan.Encode; it has no AskSpec to carry one"}
	case ask > 0 && sp.Encode != nil:
		return StageModeError{Stage: sp.Name,
			Reason: "a model stage's artifacts are encoded by its AskSpec, so StagePlan.Encode would never run"}
	}
	return nil
}

// units derives the stage's output units from its resolved lanes. It is the
// OwedArtifactResolver the chain walks, which is what keeps the worklist and the thing
// resume verdicts ONE description: a unit that is planned is a unit that is
// scanned.
//
// contributing tasks describe no unit, because they produce no artifact. That is
// the same statement read from the resume side: what a stage owes is what it
// writes, so a fold's boundary calls are invisible to the scan and its
// composed list is the whole of what the stage must have on disk to count as
// complete.
func (sp *StagePlan) units() ([]OwedArtifact, error) {
	lanes, err := sp.resolve()
	if err != nil {
		return nil, err
	}
	var units []OwedArtifact
	for _, ds := range lanes {
		for _, t := range ds.Tasks {
			if t.Contributes {
				continue
			}
			units = append(units, t.Owed)
		}
	}
	return units, nil
}

// Plan is the whole job: the job-constant job frame and the stages in
// order — the same order the resume chain walks, because StageChain derives from
// it.
type Plan struct {
	// JobFrame is slot 1 as final bytes: process framing plus job-stable
	// specifics (ARCHITECTURE.md §7). It lives on the JOB rather than on
	// each stage because §7 calls slot 1 job-constant, and one field is the
	// only way to say that which cannot be contradicted — a per-stage frame
	// would let a plan re-render slot 1 at a stage boundary and stay
	// internally consistent while silently costing the whole cross-stage
	// prefix.
	JobFrame string

	// Stages are the pipeline's stages in execution order.
	Stages []*StagePlan
}

// StageChain derives the resume chain from the plan.
func (p Plan) StageChain() StageChain {
	chain := make(StageChain, 0, len(p.Stages))
	for _, sp := range p.Stages {
		chain = append(chain, &Stage{Name: sp.Name, Units: sp.units})
	}
	return chain
}

// JobResult is what a run produced and what it could not.
type JobResult struct {
	// Mode is the resume mode the job ran in.
	Mode Mode
	// ResumeScan is the resume scan's verdicts — the forensics for why anything
	// was reused or redone.
	ResumeScan ResumeScanResult
	// Stages is how many stages this run described. Under the dynamic
	// chain that is not known until the run gets there, so a job that
	// stopped early describes fewer stages than its chain has — which is
	// the fact DeliveryReady needs and the unit counts alone cannot carry.
	Stages int
	// Units is how many units the stages described so far hold.
	Units int
	// Reused is how many were proven valid and skipped.
	Reused int
	// Produced is how many this run wrote, fallen-back ones included.
	Produced int
	// FallbackCount counts fallback-backed CALLS that kept a mechanical fallback
	// because the model's answer did not verify.
	//
	// Per call rather than per unit, because stage 4's fold spends n calls on
	// one artifact (see LaneTask.Contributes). Counting units there would report a
	// composed cut list holding three fallbacks as one degradation, which is
	// the number that hides the fact worth knowing; for every stage whose
	// units are one call each, the two readings coincide.
	FallbackCount int
	// Failures is the inventory, sorted by path so two runs of the same
	// broken job report it identically.
	Failures []OwedArtifactFailure
	// Usage is the token accounting summed over every call the job made.
	Usage model.Usage
}

// DeliveryReady reports whether the job may go on to assemble and emit.
//
// It is derived rather than stored: the three conditions ARE the definition,
// and a stored flag is a fourth place for them to disagree. Any failure at all
// refuses — a no-fallback seam produced nothing, a build refusal means a unit
// was never sized right. So does any shortfall in the unit count, which is how
// an aborted worker's untouched units are caught even though nothing reported
// them individually. And so does a chain not described to its end: under the
// dynamic chain a run that stopped before a stage resolved never learned what
// that stage owed, so counting only what it did learn would let a job that
// died at stage 2 of 5 report a tidy, complete-looking two stages.
func (r JobResult) DeliveryReady() bool {
	return len(r.Failures) == 0 && r.Stages == r.ResumeScan.Stages && r.Reused+r.Produced == r.Units
}

// Coordinator runs a plan: lock, scan, then stage by stage, a bounded pool of
// serial-lane workers. Workers talk to it and to nothing else (R-2) — one
// channel of lanes down, one channel of results up — so there is no shared
// mutable state between them to synchronize.
type Coordinator struct {
	store   *ArtifactStore
	runner  *CallRunner
	workers int
	lg      log.Logger
}

// NewCoordinator returns a coordinator over store and runner. workers ≤ 0
// selects DefaultWorkers.
func NewCoordinator(store *ArtifactStore, runner *CallRunner, workers int, lg log.Logger) *Coordinator {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	return &Coordinator{store: store, runner: runner, workers: workers, lg: lg}
}

// Run executes the plan under the job directory's single-writer lock.
//
// It returns a JobResult for every outcome that leaves the job's own state
// coherent — including one full of failures — and an error only for the
// classes that make the result meaningless: lock contention, an incoherent
// store, a worker abort (a kbase defect), context cancellation, or a simulated
// crash. A caller checks the error first and DeliveryReady second.
func (c *Coordinator) Run(ctx context.Context, plan Plan, mode Mode) (res JobResult, err error) {
	// Outermost defer, so it runs last and catches a simulated crash raised
	// anywhere below — including out of the lock's own release path.
	defer crashGuard(&err)

	lock, err := AcquireLock(c.store.root, c.lg)
	if err != nil {
		return JobResult{}, err
	}
	defer func() {
		if rerr := lock.Release(); rerr != nil && err == nil {
			err = rerr
		}
	}()

	if err = enterPhase(PhaseJobSetup, c.lg); err != nil {
		return JobResult{}, err
	}
	if err = guard(PhaseJobSetup, OpBuildJobFrame); err != nil {
		return JobResult{}, err
	}
	frame, err := newJobFrame(plan.JobFrame)
	if err != nil {
		return JobResult{}, err
	}
	if err = guard(PhaseJobSetup, OpResumeScan); err != nil {
		return JobResult{}, err
	}
	chain := plan.StageChain()
	scan, err := c.store.ResumeScan(chain, mode)
	if err != nil {
		return JobResult{}, err
	}

	res = JobResult{Mode: mode, ResumeScan: scan, Reused: scan.Reused}
	valid := validPaths(scan)
	// What has already failed in this run, and the root cause behind each —
	// the whole of cascade-failure's state, and it lives here because it is
	// run-scoped: nothing about it is written down, and the next run sees both
	// the cause and its cascade as simply Absent.
	roots := map[string]string{}
	inventoried := 0

	for i, sp := range plan.Stages {
		// The stage is described HERE, not at job setup — for a stage past
		// the resume boundary this is the first time anyone has asked what
		// it owes, and it can only answer because the stages it derives
		// from have already run. Below the boundary the scan asked first
		// and this is the memo.
		units, uerr := c.store.resolveStage(chain, i)
		if uerr != nil {
			err = uerr
			break
		}
		res.Units += len(units)

		// Stages before the resume boundary stand on proof: their units are
		// already counted in scan.Reused, and not building their bound ask is
		// what makes "reused" mean the model was never consulted.
		if i < scan.ResumeStage {
			res.Stages++
			c.lg.Info("stage reused whole", "stage", sp.Name)
			continue
		}
		lanes, serr := sp.resolve()
		if serr != nil {
			err = serr
			break
		}
		runnable, ferr := filterLanes(lanes, valid)
		if ferr != nil {
			err = fmt.Errorf("pipeline: stage %s: %w", sp.Name, ferr)
			break
		}
		runnable, cascaded := markCascades(sp.Name, runnable, roots)
		res.Failures = append(res.Failures, cascaded...)
		// A stage that stops takes the job with it, but the summary below
		// still runs: what a job managed before it died is the first thing
		// anyone will want, and reporting it only on the happy path is
		// reporting it when it is least needed.
		if err = c.runStage(ctx, sp, frame, runnable, &res); err != nil {
			break
		}
		// Every worker of the stage has exited, so the stage's failures are
		// complete and the NEXT stage's dependents can be told apart from
		// their causes. A stage boundary is the only place this can be read:
		// within a stage the units run concurrently, and a unit may not name
		// a sibling anyway (see ArtifactStore.validateStage).
		noteFailures(roots, res.Failures[inventoried:])
		inventoried = len(res.Failures)
		res.Stages++
	}

	// Only a run that described its whole chain knows what belongs in the
	// job directory; see ArtifactStore.sweep for why that is the one safe moment.
	if err == nil && res.Stages == len(chain) {
		if serr := c.store.sweep(chain); serr != nil {
			err = serr
		}
	}

	slices.SortFunc(res.Failures, func(a, b OwedArtifactFailure) int { return strings.Compare(a.Path, b.Path) })
	c.lg.Info("job finished",
		"mode", string(mode), "stages", res.Stages, "units", res.Units,
		"reused", res.Reused, "produced", res.Produced,
		"fallbacks", res.FallbackCount, "failed", len(res.Failures), "emit_ready", res.DeliveryReady(),
		"prompt_tokens", res.Usage.PromptTokens, "cached_tokens", res.Usage.CachedPromptTokens,
		"completion_tokens", res.Usage.CompletionTokens)
	for _, f := range res.Failures {
		c.lg.Error("unit failed", "stage", f.Stage, "path", f.Path,
			"kind", string(f.Kind), "attempts", f.ModelAttempts, "error", f.Err)
	}
	return res, err
}

// runStage builds the stage's one shared bound ask, if it has one, and runs its
// lanes through the pool.
//
// A mechanical stage has none: its tasks produce their artifacts in process, so
// there is no context to freeze, no definition to render and no tier to
// resolve. Skipping the construction is what makes "no AskSpec needed" true rather
// than a null AskSpec passing a validation it was never going to satisfy.
func (c *Coordinator) runStage(ctx context.Context, stage *StagePlan, frame jobFrame, lanes []SerialLane, res *JobResult) error {
	if len(lanes) == 0 {
		return nil
	}
	if err := enterPhase(PhaseStageSetup, c.lg); err != nil {
		return err
	}
	var (
		ag     *boundAsk
		encode = stage.Encode
		mode   = "mechanical"
	)
	if !mechanicalStage(lanes) {
		if err := guard(PhaseStageSetup, OpRebuildStageContext); err != nil {
			return err
		}
		built, err := newBoundAsk(stage.Ask, stage.Spec, frame)
		if err != nil {
			return fmt.Errorf("pipeline: stage %s: %w", stage.Name, err)
		}
		ag, encode, mode = built, stage.Ask.Encode, string(built.Seam())
	}
	c.lg.Info("stage started", "stage", stage.Name, "mode", mode,
		"tier", stage.Ask.Tier, "lanes", len(lanes))

	// A worker that aborts stops the whole stage: the abort classes are
	// defects and cancellation, and neither is something the remaining
	// workers should keep spending tokens through.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		pending  = make(chan SerialLane)
		results  = make(chan unitResult)
		wg       sync.WaitGroup
		abortMu  sync.Mutex
		abortErr error
	)
	abort := func(err error) {
		abortMu.Lock()
		if abortErr == nil {
			abortErr = err
		}
		abortMu.Unlock()
		cancel()
	}

	for range min(c.workers, len(lanes)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &worker{stage: stage.Name, boundAsk: ag, encode: encode, runner: c.runner, store: c.store, lg: c.lg}
			// Ranging to completion rather than breaking on the first
			// abort: the feeder is only unblocked by this loop or by the
			// cancellation it already received, and leaving it blocked
			// would leak the goroutine.
			for s := range pending {
				if err := w.run(ctx, s, results); err != nil {
					abort(err)
				}
			}
		}()
	}
	go func() {
		defer close(pending)
		for _, s := range lanes {
			select {
			case pending <- s:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	// Draining to close is what makes Run's return a synchronization point:
	// every worker goroutine has exited by the time this loop ends, so no
	// store write is still in flight when a harness inspects the job dir.
	for r := range results {
		res.Usage = addUsage(res.Usage, r.Usage)
		switch {
		case r.Failure != nil:
			res.Failures = append(res.Failures, *r.Failure)
		case r.Produced:
			res.Produced++
			if r.FellBack {
				res.FallbackCount++
			}
		case r.FellBack:
			// A contributing call that fell back: it wrote nothing, and the
			// artifact it fed is degraded all the same.
			res.FallbackCount++
		}
	}
	abortMu.Lock()
	stopped := abortErr
	abortMu.Unlock()
	if stopped != nil {
		return stopped
	}

	// Every worker goroutine has exited by here and the stage is not yet
	// closed — the one moment in a run with the stage's units on disk and
	// nothing in flight. No phase entry produces that state, so it gets its
	// own point.
	crashpoint.At(cpStageComplete)
	return enterPhase(PhaseStageTeardown, c.lg)
}

// unitResult is one unit's outcome on its way back to the coordinator. Exactly
// one of Failure and Produced is set.
type unitResult struct {
	Domain   string
	Path     string
	Produced bool
	FellBack bool
	Usage    model.Usage
	Failure  *OwedArtifactFailure
}

// worker runs one serial lane at a time. Everything on it is owned by one
// goroutine: the phase traversal, the previous call's hashes, and the section
// buffer are per-worker state precisely so the churn tripwire is per-worker
// (R-1), and nothing here is shared with a sibling.
type worker struct {
	stage string
	// boundAsk is the stage's shared bound ask, nil on a mechanical stage — the one
	// piece of a worker that a Produce task never reaches.
	boundAsk *boundAsk
	encode   Encoder
	runner   *CallRunner
	store    *ArtifactStore
	lg       log.Logger

	phase JobPhase
	// traversed is every phase entered since the last call was built. The
	// honest frontier for the next call is the minimum over these, not the
	// frontier of wherever the worker happens to be standing.
	traversed []JobPhase
	// prev is the last built call's hashes, nil when the worker has nothing
	// to compare against — its first call, and its first call after a
	// failure, which claim nothing rather than claim against a call that
	// may never have been sent.
	prev map[prompt.Slot][32]byte
	// stageRef is slot 4's current content, replaced only at a section
	// transition.
	stageRef string
}

// run processes one lane's units in order. A unit failure is reported and
// the lane continues (graceful degradation: siblings are worth finishing);
// an abort class stops the lane and is returned.
//
// # A failed call POISONS the artifact its calls were feeding
//
// The contributing tasks before a producing task are that artifact's calls: their
// answers are the stage state it is composed from (see LaneTask.Contributes). So the
// invariant this loop keeps is "the artifact exists exactly when every call of
// its lane succeeded" — one failed boundary and the composed list is not
// written at all, the same state a kill mid-fold leaves (§12).
//
// Suppressing the write rather than inventorying the failure alone is what
// makes the next run redo the fold WHOLE. A written artifact would carry a
// perfectly valid stamp — the source hash and parameter digest do not know a
// call failed — so the scan would verdict it Valid, filterLanes would drop
// the whole lane, and the failure would exist in exactly one run's
// JobResult and nowhere on disk. The remaining calls of a poisoned artifact
// are skipped for the same reason: they would spend tokens feeding state
// nobody is going to record.
//
// The poison is cleared at the producing task, so a lane carrying two
// artifacts (a shape filterLanes refuses today, and asserts it refuses)
// would not have the first's failure suppress the second.
func (w *worker) run(ctx context.Context, lane SerialLane, out chan<- unitResult) (err error) {
	// A panic cannot cross a goroutine boundary to the coordinator, so the
	// simulated-crash sentinel is caught here and returned as an error.
	defer crashGuard(&err)

	w.traversed = []JobPhase{PhaseStageSetup}
	w.prev = nil
	section := ""
	poisoned := false
	for i, task := range lane.Tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if poisoned {
			if task.Contributes {
				w.lg.Warn("skipping a call whose artifact a failed call already poisoned",
					"stage", w.stage, "unit", task.Owed.Path)
				continue
			}
			out <- unitResult{Domain: lane.Domain, Path: task.Owed.Path, Failure: &OwedArtifactFailure{
				Stage: w.stage, Path: task.Owed.Path, Kind: FailureUpstream,
				Err: PoisonedLaneError{Stage: w.stage, Path: task.Owed.Path},
			}}
			poisoned = false
			continue
		}
		if i == 0 || task.Section != section {
			// The first task's buffer fill goes through the transition too:
			// one path, so the flush is gated by the phaseOpTable every time
			// rather than every time but once.
			if err := w.enter(PhaseSectionTransition); err != nil {
				return err
			}
			if err := guard(w.phase, OpFlushStageRef); err != nil {
				return err
			}
			w.stageRef, section = task.SectionRef, task.Section
			if err := w.enter(PhaseCallLoop); err != nil {
				return err
			}
		}
		res, err := w.unit(ctx, lane.Domain, task)
		if err != nil {
			return err
		}
		poisoned = task.Contributes && res.Failure != nil
		out <- res
	}
	return nil
}

// unit runs one task: derive the artifact — by call or by producer — and write
// it. The phase guards live here rather than inside the runner because the
// phase is the worker's state; the runner is stateless and shared, and a
// producer knows nothing about phases at all.
func (w *worker) unit(ctx context.Context, domain string, task LaneTask) (unitResult, error) {
	// The input set is resolved BEFORE the artifact is derived, not at write
	// time. It reads the upstream stamps, so a unit whose upstream never got
	// produced is knowable in advance — and spending a model call, or a
	// producer's work, on something that cannot be recorded is spending it to
	// learn nothing. It is also the SAME function the scan uses, so a stamp
	// can never be written under one derivation and checked under another. A
	// contributing task records nothing, so there is nothing to resolve and
	// nothing that could be unrecordable.
	var inputs []Input
	if !task.Contributes {
		var err error
		if inputs, err = w.store.resolveInputs(task.Owed); err != nil {
			return unitResult{Domain: domain, Path: task.Owed.Path, Failure: &OwedArtifactFailure{
				Stage: w.stage, Path: task.Owed.Path, Kind: FailureUpstream, Err: err,
			}}, nil
		}
	}
	if task.Produce != nil {
		return w.produce(domain, task, inputs)
	}

	if err := guard(w.phase, OpBuildCall); err != nil {
		return unitResult{}, err
	}
	// Built HERE, at the unit, which is what lets a stage's later questions
	// depend on its own earlier answers (see InputBuilder). Everything that
	// judges a call — the StageRef refusal below, the budget refusal and the
	// frozen-prompt assertion inside the runner — judges what came out of it.
	in := task.Input()
	if in.StageRef != "" {
		return unitResult{}, TaskOwnsStageRefError{Stage: w.stage, Path: task.Owed.Path}
	}

	in.StageRef = w.stageRef
	// A worker with no previous call claims nothing, which is what the
	// phaseOpTable's job-setup row already says — read from there rather than
	// spelled a second time here.
	frontier, err := StabilityFrontier(PhaseJobSetup)
	if err != nil {
		return unitResult{}, err
	}
	if w.prev != nil {
		f, err := lowestFrontier(w.traversed...)
		if err != nil {
			return unitResult{}, err
		}
		frontier = f
	}

	res, err := w.runner.Run(ctx, call{
		Stage: w.stage, ArtifactPath: task.Owed.Path, BoundAsk: w.boundAsk,
		Input: in, Frontier: frontier, Prev: w.prev,
	})
	// A call was attempted, so the traversal restarts from whatever phase
	// the worker is standing in — w.phase rather than the literal, so a
	// phase inserted between the build and here cannot make the traversal
	// lie about where the next call was built.
	w.traversed = []JobPhase{w.phase}
	if err != nil {
		w.prev = nil
		if f, ok := w.classify(task, err); ok {
			return unitResult{Domain: domain, Path: task.Owed.Path, Usage: f.Usage, Failure: f}, nil
		}
		return unitResult{}, err
	}
	w.prev = res.Hashes

	if task.Contributes {
		// The answer is in the stage's own state — the verifier folded it in
		// — so the unit ends here: nothing to encode, nothing to write, and
		// nothing for the scan to find. a call that fell back is still reported,
		// because a fold that fell back on a boundary produced a composed
		// artifact that is correct and less good than it was meant to be.
		if err := guard(w.phase, OpAdvanceUnit); err != nil {
			return unitResult{}, err
		}
		return unitResult{Domain: domain, Path: task.Owed.Path, FellBack: res.FellBack, Usage: res.Usage}, nil
	}

	return w.write(domain, task, res.Artifact, inputs, res)
}

// produce runs a mechanical task: the unit's own derivation, in process, where
// the call would have been. Everything on both sides of it is the model path's
// — the inputs were resolved and hashed before it ran, and what it returns goes
// through the same encoder, the same Put and the same stamp.
//
// A producer's failure is an inventoried unit failure, not an abort. It is the
// deterministic counterpart of a no-fallback seam's verification failure: there
// is no fallback to fall back to and no retry worth making (the same inputs
// would derive the same failure), so the unit fails, its siblings finish, and
// the job refuses emission.
func (w *worker) produce(domain string, task LaneTask, inputs []Input) (unitResult, error) {
	artifact, err := task.Produce()
	if err != nil {
		return unitResult{Domain: domain, Path: task.Owed.Path, Failure: &OwedArtifactFailure{
			Stage: w.stage, Path: task.Owed.Path, Kind: FailureProduce, Err: err,
		}}, nil
	}
	crashpoint.At(cpProduced)
	return w.write(domain, task, artifact, inputs, CallResult{})
}

// write encodes a derived artifact and puts it in the store under the input set
// resolved for it. It is shared by both task modes on purpose: a produced
// artifact and a verified one reach disk the same way, or "everything else is
// unchanged" would be a claim rather than a fact.
//
// res carries what the derivation cost, and a produced unit passes the zero
// value: no call was made, so there are no attempts, no tokens and no seam to
// have degraded.
func (w *worker) write(domain string, task LaneTask, artifact any, inputs []Input, res CallResult) (unitResult, error) {
	if err := guard(w.phase, OpWriteArtifact); err != nil {
		return unitResult{}, err
	}
	data, err := w.encode(artifact)
	if err != nil {
		return w.writeFailure(domain, task, res, fmt.Errorf("encode: %w", err)), nil
	}
	if err := w.store.Put(task.Owed.Path, data, inputs); err != nil {
		return w.writeFailure(domain, task, res, err), nil
	}
	if err := guard(w.phase, OpAdvanceUnit); err != nil {
		return unitResult{}, err
	}
	return unitResult{
		Domain: domain, Path: task.Owed.Path, Produced: true,
		FellBack: res.FellBack, Usage: res.Usage,
	}, nil
}

// classify sorts a runner error into "inventory it and keep going" or "stop".
// The dividing line is whether the rest of the lane is still worth running:
// a unit that failed verification says nothing about its siblings, while a
// frozen-prompt violation or a cancelled context says everything.
func (w *worker) classify(task LaneTask, err error) (*OwedArtifactFailure, bool) {
	var unit OwedArtifactFailure
	if errors.As(err, &unit) {
		return &unit, true
	}
	var budget prompt.ErrOverBudget
	if errors.As(err, &budget) {
		// Refuse-and-split: the unit is too big for one call, which is the
		// tree plan's problem and not this run's. Inventoried so the report
		// names what to re-split.
		return &OwedArtifactFailure{Stage: w.stage, Path: task.Owed.Path, Kind: FailureBudget, Err: err}, true
	}
	return nil, false
}

// writeFailure records a verified artifact that could not be stored. It is a
// unit failure rather than an abort: one unwritable path (a name collision, a
// full disk on one volume) does not mean the next one fails too, and the
// inventory is more useful than the first error.
func (w *worker) writeFailure(domain string, task LaneTask, res CallResult, err error) unitResult {
	return unitResult{
		Domain: domain, Path: task.Owed.Path, Usage: res.Usage,
		Failure: &OwedArtifactFailure{
			Stage: w.stage, Path: task.Owed.Path, Kind: FailureWrite,
			ModelAttempts: res.Attempts, Usage: res.Usage, Err: err,
		},
	}
}

// enter moves the worker into a phase and records the traversal.
func (w *worker) enter(p JobPhase) error {
	if err := enterPhase(p, w.lg); err != nil {
		return err
	}
	w.phase = p
	w.traversed = append(w.traversed, p)
	return nil
}

// TaskOwnsStageRefError reports a task that filled the stage reference buffer
// itself. Slot 4 is the worker's, replaced only at a section transition, and a
// task that set it would be a per-call buffer wearing the multi-call buffer's
// position — the one thing the §7 slot ordering cannot survive. It is a
// typed value like every other refusal here, so a caller can tell it from an
// I/O failure.
type TaskOwnsStageRefError struct {
	Stage string
	Path  string
}

func (e TaskOwnsStageRefError) Error() string {
	return fmt.Sprintf("pipeline: %s: task %s set the stage reference buffer directly; the worker fills it from SectionRef",
		e.Stage, e.Path)
}

// crashGuard converts a simulated-crash panic into an error on *errp.
//
// Only the crashpoint sentinel is caught. Any other panic is re-raised, because
// a real bug must still take the process down — a general recover here would
// turn every nil dereference in a worker into a quiet unit failure, which is
// the opposite of what this codebase does with defects.
//
// It exists because a panic in a worker goroutine cannot be recovered by the
// parent, and the resume harness needs a kill to be observable as a returned
// error rather than as a dead test binary.
func crashGuard(errp *error) {
	r := recover()
	if r == nil {
		return
	}
	crash, ok := r.(*crashpoint.Crash)
	if !ok {
		panic(r)
	}
	*errp = crash
}

// validPaths collects the resume stage's proven units. Units in later stages
// are absent from it because the scan does not verdict them — their inputs are
// about to change, so nothing there is reusable by definition.
func validPaths(scan ResumeScanResult) map[string]bool {
	valid := make(map[string]bool)
	for _, v := range scan.Verdicts {
		if v.Verdict == VerdictValid {
			valid[v.Path] = true
		}
	}
	return valid
}

// filterLanes drops the tasks the scan proved and the lanes that empty out.
// This is the resume made operational: the scan's verdicts ARE the worklist
// filter, so there is no second decision about what to redo.
//
// A contributing task is never proved — it produces nothing to verdict — so it
// survives the filter only as long as some task in its lane still has an
// artifact to write. A lane whose every producing task is already valid is
// dropped entire, calls and all: its remaining calls would spend tokens
// feeding state nobody is going to record.
//
// Which is exactly why a lane that carries calls may carry only ONE
// artifact, and why that is asserted here rather than assumed. Given
// [fold-A calls…, artifact-A, fold-B calls…, artifact-B] with A already
// valid, this filter would keep the lane for B's sake and re-spend every
// one of fold A's calls to feed state nobody records. Nothing builds that
// shape today; the assertion is what makes the day someone does a loud
// refusal rather than a quiet bill.
func filterLanes(lanes []SerialLane, valid map[string]bool) ([]SerialLane, error) {
	for _, s := range lanes {
		calls, produces := 0, 0
		for _, t := range s.Tasks {
			if t.Contributes {
				calls++
				continue
			}
			produces++
		}
		if calls > 0 && produces > 1 {
			return nil, MultiArtifactLaneError{Domain: s.Domain, Artifacts: produces}
		}
	}
	if len(valid) == 0 {
		return lanes, nil
	}
	out := make([]SerialLane, 0, len(lanes))
	for _, s := range lanes {
		kept := make([]LaneTask, 0, len(s.Tasks))
		for _, t := range s.Tasks {
			if !t.Contributes && valid[t.Owed.Path] {
				continue
			}
			kept = append(kept, t)
		}
		if producing(kept) {
			out = append(out, SerialLane{Domain: s.Domain, Tasks: kept})
		}
	}
	return out, nil
}

// mechanicalStage reports whether the stage's tasks derive their artifacts in
// process. LaneTask modes are uniform per stage (StagePlan.validate), so the first
// task answers for the stage; the loop is over lanes only because which
// lane holds it is not fixed.
func mechanicalStage(lanes []SerialLane) bool {
	for _, s := range lanes {
		for _, t := range s.Tasks {
			return t.Produce != nil
		}
	}
	return false
}

// StageModeError refuses a stage description whose task modes do not describe
// one coherent stage — see StagePlan.validate for the two rules. It is a plan
// defect: no unit here could be produced by any remedy the run has, so it stops
// the job at the moment the stage is described rather than per unit.
type StageModeError struct {
	Stage  string
	Path   string
	Reason string
}

func (e StageModeError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("pipeline: stage %s: task %s: %s", e.Stage, e.Path, e.Reason)
	}
	return fmt.Sprintf("pipeline: stage %s: %s", e.Stage, e.Reason)
}

// markCascades holds back the units whose upstream failed EARLIER IN THIS RUN,
// and with them the calls that were going to feed them. It returns the lanes
// still worth running and the inventory of what it abandoned.
//
// ARCHITECTURE.md §12 has two rules either side of this case and neither
// reaches it. "A failed call poisons the lane it is in" is intra-lane, and
// this is a chain edge. "An unstamped upstream is structural incoherence" is
// scoped by its own justification — nothing this chain runs would ever produce
// it — which is exactly false for an artifact whose producing unit failed a
// moment ago; applying it anyway would refuse the whole resume with `--fresh`
// guidance and discard every proven artifact in the job over one failed unit.
// So the poisoning discipline is applied ACROSS the chain edge instead: the
// dependent is marked cascade-failed — not attempted, no call spent, nothing
// written, inventoried beside its cause and naming the root of the chain — and
// it propagates transitively, because a cascade-failed unit is itself a failure
// the next stage reads.
//
// It is RUN-SCOPED bookkeeping: no new stamp, no persisted state, no change to
// the resume scan. Next run both the cause and its cascade are simply Absent
// and both are redone, which is why this needs no artifact machinery at all.
//
// Marking HERE rather than at the unit is what makes "spends nothing" true, in
// two ways a worker-side check could not. A fold lane's calls come before the
// task that writes its artifact, so a worker would have spent every one of them
// before reaching the unit that could not be recorded. And a unit that failed
// this run may still have a PREVIOUS run's artifact and stamp sitting on disk —
// resolveInputs would resolve it happily and the dependent would spend a call
// deriving from bytes this run has already superseded. resolveInputs remains
// the second net, for what this map cannot know (an artifact deleted under a
// running job); this is the first.
func markCascades(stage string, lanes []SerialLane, roots map[string]string) ([]SerialLane, []OwedArtifactFailure) {
	if len(roots) == 0 {
		return lanes, nil
	}
	var (
		out      []SerialLane
		cascaded []OwedArtifactFailure
	)
	for _, s := range lanes {
		kept := make([]LaneTask, 0, len(s.Tasks))
		for _, t := range s.Tasks {
			up, ok := failedUpstream(t, roots)
			if !ok {
				kept = append(kept, t)
				continue
			}
			cascaded = append(cascaded, OwedArtifactFailure{
				Stage: stage, Path: t.Owed.Path, Kind: FailureCascade,
				Err: CascadeFailureError{
					Stage: stage, Path: t.Owed.Path, Upstream: up, Root: roots[up],
				},
			})
		}
		// A lane with nothing left to write is dropped whole, calls and
		// all — the same rule filterLanes keeps, for the same reason: its
		// remaining calls would feed state nobody is going to record.
		if producing(kept) {
			out = append(out, SerialLane{Domain: s.Domain, Tasks: kept})
		}
	}
	return out, cascaded
}

// failedUpstream returns the first upstream of t that failed this run. It reads
// the upstreams in their declared order, so a unit with two failed dependencies
// names the same one in every run of the same broken job.
//
// A contributing task consumes nothing of its own — its unit description is a name,
// not an artifact (see LaneTask.Owed) — so it never cascades by itself; it is
// dropped with the lane whose artifact did.
func failedUpstream(t LaneTask, roots map[string]string) (string, bool) {
	if t.Contributes {
		return "", false
	}
	for _, up := range t.Owed.Upstreams {
		if _, ok := roots[up]; ok {
			return up, true
		}
	}
	return "", false
}

// noteFailures records a stage's failures as causes the next stage's units may
// cascade off, carrying the ROOT of each chain forward: a unit that cascaded
// off a cascade names the failure whose remedy fixes the whole line, not its
// immediate predecessor.
func noteFailures(roots map[string]string, failures []OwedArtifactFailure) {
	for _, f := range failures {
		root := f.Path
		var cascade CascadeFailureError
		if errors.As(f.Err, &cascade) {
			root = cascade.Root
		}
		roots[f.Path] = root
	}
}

// producing reports whether the tasks still hold an artifact to write. A lane
// that does not is dropped: whatever calls it has left would be spent feeding
// state nobody records.
func producing(tasks []LaneTask) bool {
	for _, t := range tasks {
		if !t.Contributes {
			return true
		}
	}
	return false
}

// CascadeFailureError reports a unit that was never attempted because a unit it
// consumes failed earlier in the same run.
//
// Upstream is the dependency that failed; Root is the failure at the head of
// the chain. They are the same path for a one-hop cascade, and differ down a
// chain of stages — the level-sliced summary stages are the case this exists
// for — where the remedy is Root's and reporting the immediate predecessor
// would send a reader one hop at a time.
type CascadeFailureError struct {
	Stage    string
	Path     string
	Upstream string
	Root     string
}

func (e CascadeFailureError) Error() string {
	if e.Root != "" && e.Root != e.Upstream {
		return fmt.Sprintf("pipeline: %s: %s was not attempted: its upstream %s failed this run, "+
			"in the cascade of %s", e.Stage, e.Path, e.Upstream, e.Root)
	}
	return fmt.Sprintf("pipeline: %s: %s was not attempted: its upstream %s failed this run",
		e.Stage, e.Path, e.Upstream)
}

// PoisonedLaneError reports an artifact that was not written because one of
// the calls feeding it failed. It is the cascade of the failure beside it in
// the inventory, and the remedy is that one — which is why the unit is
// classified FailureUpstream and costs no tokens.
type PoisonedLaneError struct {
	Stage string
	Path  string
}

func (e PoisonedLaneError) Error() string {
	return fmt.Sprintf("pipeline: %s: %s was not written; a call it is composed from failed, "+
		"so the whole lane is redone rather than half-recorded", e.Stage, e.Path)
}

// MultiArtifactLaneError reports a lane that mixes contributing tasks with
// more than one artifact — see filterLanes for why that shape cannot be
// filtered honestly.
type MultiArtifactLaneError struct {
	Domain    string
	Artifacts int
}

func (e MultiArtifactLaneError) Error() string {
	return fmt.Sprintf("pipeline: serial lane %q carries calls and %d artifacts; "+
		"a lane whose tasks feed an artifact may carry exactly one, "+
		"or a resume would re-spend the calls of an artifact it already proved",
		e.Domain, e.Artifacts)
}

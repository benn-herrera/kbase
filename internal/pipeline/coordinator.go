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

// Task is one unit of work: the artifact it must produce and the per-call
// context that produces it.
type Task struct {
	// Unit describes the output artifact and what it is derived from. It is
	// the SAME description the resume scan verdicts, which is what keeps the
	// worklist stateless — there is no second place that says what a unit is.
	Unit Unit

	// Section names the section this unit belongs to. A change between
	// consecutive tasks is what drives the worker through
	// PhaseSectionTransition, which is the only phase that may flush
	// reference buffer A.
	Section string

	// SectionRef is reference buffer A's content for that section: the
	// orchestrator-curated material stable across the section's calls.
	SectionRef string

	// Input is the per-call half of the prompt, with RefA left EMPTY — the
	// worker owns slot 4 and fills it from SectionRef at the transition. A
	// non-empty RefA here is refused rather than overwritten, because
	// silently discarding a caller's buffer is how slot 4 would start
	// churning per call without anyone noticing.
	Input prompt.CallInput
}

// DomainStream is one worker's whole assignment: a domain and its units in
// order (R-2). A worker owns a domain and processes it serially, so the
// stability state a stream depends on — previous-call hashes, the phases
// traversed, the current section buffer — is never shared.
type DomainStream struct {
	Domain string
	Tasks  []Task
}

// StreamResolver produces a stage's domain streams. It is called ONCE per
// job, when the chain walk reaches the stage — the plan half of the dynamic
// chain (see UnitResolver, which this backs).
//
// A stage whose work is known up front returns a fixed slice and ignores the
// laziness. A stage whose work is not — stage 4's leaves come out of the
// skeleton stage 3 emitted, stage 5's out of the cut list stage 4 verified —
// reads what its upstream left in the store and builds its streams from that.
type StreamResolver func() ([]DomainStream, error)

// StagePlan is one stage: the role every worker of the stage runs, the stage
// context spec they all share, and the domain streams that produce its units.
type StagePlan struct {
	Name string
	Role Role
	Spec prompt.StageSpec

	// Streams resolves the stage's work when the stage is reached.
	Streams StreamResolver

	// The resolution memo. It is what makes "called once" true: the scan
	// asks for this stage's units and the run then asks for its streams,
	// and both must be looking at the same description — a resolver that
	// ran twice could read a store the run itself had changed in between.
	resolved   []DomainStream
	resolveErr error
	done       bool
}

// resolve produces the stage's streams, once.
func (sp *StagePlan) resolve() ([]DomainStream, error) {
	if !sp.done {
		sp.resolved, sp.resolveErr = sp.Streams()
		sp.done = true
	}
	return sp.resolved, sp.resolveErr
}

// units derives the stage's output units from its resolved streams. It is the
// UnitResolver the chain walks, which is what keeps the worklist and the thing
// resume verdicts ONE description: a unit that is planned is a unit that is
// scanned.
func (sp *StagePlan) units() ([]Unit, error) {
	streams, err := sp.resolve()
	if err != nil {
		return nil, err
	}
	var units []Unit
	for _, ds := range streams {
		for _, t := range ds.Tasks {
			units = append(units, t.Unit)
		}
	}
	return units, nil
}

// Plan is the whole job: the job-constant system frame and the stages in
// order — the same order the resume chain walks, because Chain derives from
// it.
type Plan struct {
	// SystemFrame is slot 1 as final bytes: process framing plus job-stable
	// specifics (ARCHITECTURE.md §7). It lives on the JOB rather than on
	// each stage because §7 calls slot 1 job-constant, and one field is the
	// only way to say that which cannot be contradicted — a per-stage frame
	// would let a plan re-render slot 1 at a stage boundary and stay
	// internally consistent while silently costing the whole cross-stage
	// prefix.
	SystemFrame string

	// Stages are the pipeline's stages in execution order.
	Stages []*StagePlan
}

// Chain derives the resume chain from the plan.
func (p Plan) Chain() Chain {
	chain := make(Chain, 0, len(p.Stages))
	for _, sp := range p.Stages {
		chain = append(chain, &Stage{Name: sp.Name, Units: sp.units})
	}
	return chain
}

// JobResult is what a run produced and what it could not.
type JobResult struct {
	// Mode is the resume mode the job ran in.
	Mode Mode
	// Scan is the resume scan's verdicts — the forensics for why anything
	// was reused or redone.
	Scan ScanResult
	// Stages is how many stages this run described. Under the dynamic
	// chain that is not known until the run gets there, so a job that
	// stopped early describes fewer stages than its chain has — which is
	// the fact EmitReady needs and the unit counts alone cannot carry.
	Stages int
	// Units is how many units the stages described so far hold.
	Units int
	// Reused is how many were proven valid and skipped.
	Reused int
	// Produced is how many this run wrote, degraded ones included.
	Produced int
	// Degraded counts produced units that kept a mechanical baseline
	// because the model's refinement did not verify.
	Degraded int
	// Failures is the inventory, sorted by path so two runs of the same
	// broken job report it identically.
	Failures []UnitFailure
	// Usage is the token accounting summed over every call the job made.
	Usage model.Usage
}

// EmitReady reports whether the job may go on to assemble and emit.
//
// It is derived rather than stored: the three conditions ARE the definition,
// and a stored flag is a fourth place for them to disagree. Any failure at all
// refuses — an essential seam produced nothing, a build refusal means a unit
// was never sized right. So does any shortfall in the unit count, which is how
// an aborted worker's untouched units are caught even though nothing reported
// them individually. And so does a chain not described to its end: under the
// dynamic chain a run that stopped before a stage resolved never learned what
// that stage owed, so counting only what it did learn would let a job that
// died at stage 2 of 5 report a tidy, complete-looking two stages.
func (r JobResult) EmitReady() bool {
	return len(r.Failures) == 0 && r.Stages == r.Scan.Stages && r.Reused+r.Produced == r.Units
}

// Coordinator runs a plan: lock, scan, then stage by stage, a bounded pool of
// domain-stream workers. Workers talk to it and to nothing else (R-2) — one
// channel of streams down, one channel of results up — so there is no shared
// mutable state between them to synchronize.
type Coordinator struct {
	store   *Store
	runner  *CallRunner
	workers int
	lg      log.Logger
}

// NewCoordinator returns a coordinator over store and runner. workers ≤ 0
// selects DefaultWorkers.
func NewCoordinator(store *Store, runner *CallRunner, workers int, lg log.Logger) *Coordinator {
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
// crash. A caller checks the error first and EmitReady second.
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
	if err = guard(PhaseJobSetup, OpBuildSystemFrame); err != nil {
		return JobResult{}, err
	}
	frame, err := newJobFrame(plan.SystemFrame)
	if err != nil {
		return JobResult{}, err
	}
	if err = guard(PhaseJobSetup, OpResumeScan); err != nil {
		return JobResult{}, err
	}
	chain := plan.Chain()
	scan, err := c.store.Scan(chain, mode)
	if err != nil {
		return JobResult{}, err
	}

	res = JobResult{Mode: mode, Scan: scan, Reused: scan.Reused}
	valid := validPaths(scan)

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
		// already counted in scan.Reused, and not building their agent is
		// what makes "reused" mean the model was never consulted.
		if i < scan.ResumeStage {
			res.Stages++
			c.lg.Info("stage reused whole", "stage", sp.Name)
			continue
		}
		streams, serr := sp.resolve()
		if serr != nil {
			err = serr
			break
		}
		// A stage that stops takes the job with it, but the summary below
		// still runs: what a job managed before it died is the first thing
		// anyone will want, and reporting it only on the happy path is
		// reporting it when it is least needed.
		if err = c.runStage(ctx, sp, frame, filterStreams(streams, valid), &res); err != nil {
			break
		}
		res.Stages++
	}

	// Only a run that described its whole chain knows what belongs in the
	// job directory; see Store.sweep for why that is the one safe moment.
	if err == nil && res.Stages == len(chain) {
		if serr := c.store.sweep(chain); serr != nil {
			err = serr
		}
	}

	slices.SortFunc(res.Failures, func(a, b UnitFailure) int { return strings.Compare(a.Path, b.Path) })
	c.lg.Info("job finished",
		"mode", string(mode), "stages", res.Stages, "units", res.Units,
		"reused", res.Reused, "produced", res.Produced,
		"degraded", res.Degraded, "failed", len(res.Failures), "emit_ready", res.EmitReady(),
		"prompt_tokens", res.Usage.PromptTokens, "cached_tokens", res.Usage.CachedPromptTokens,
		"completion_tokens", res.Usage.CompletionTokens)
	for _, f := range res.Failures {
		c.lg.Error("unit failed", "stage", f.Stage, "path", f.Path,
			"kind", string(f.Kind), "attempts", f.SemanticAttempts, "error", f.Err)
	}
	return res, err
}

// runStage builds the stage's one shared agent and runs its streams through
// the pool.
func (c *Coordinator) runStage(ctx context.Context, stage *StagePlan, frame jobFrame, streams []DomainStream, res *JobResult) error {
	if len(streams) == 0 {
		return nil
	}
	if err := enterPhase(PhaseStageSetup, c.lg); err != nil {
		return err
	}
	if err := guard(PhaseStageSetup, OpRebuildStageContext); err != nil {
		return err
	}
	agent, err := newAgent(stage.Role, stage.Spec, frame)
	if err != nil {
		return fmt.Errorf("pipeline: stage %s: %w", stage.Name, err)
	}
	c.lg.Info("stage started", "stage", stage.Name, "seam", string(agent.Seam()),
		"tier", stage.Role.Tier, "streams", len(streams))

	// A worker that aborts stops the whole stage: the abort classes are
	// defects and cancellation, and neither is something the remaining
	// workers should keep spending tokens through.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		pending  = make(chan DomainStream)
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

	for range min(c.workers, len(streams)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &worker{stage: stage.Name, agent: agent, runner: c.runner, store: c.store, lg: c.lg}
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
		for _, s := range streams {
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
			if r.Degraded {
				res.Degraded++
			}
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
	Degraded bool
	Usage    model.Usage
	Failure  *UnitFailure
}

// worker runs one domain stream at a time. Everything on it is owned by one
// goroutine: the phase traversal, the previous call's hashes, and the section
// buffer are per-worker state precisely so the churn tripwire is per-worker
// (R-1), and nothing here is shared with a sibling.
type worker struct {
	stage  string
	agent  *Agent
	runner *CallRunner
	store  *Store
	lg     log.Logger

	phase Phase
	// traversed is every phase entered since the last call was built. The
	// honest frontier for the next call is the minimum over these, not the
	// frontier of wherever the worker happens to be standing.
	traversed []Phase
	// prev is the last built call's hashes, nil when the worker has nothing
	// to compare against — its first call, and its first call after a
	// failure, which claim nothing rather than claim against a call that
	// may never have been sent.
	prev map[prompt.Slot][32]byte
	// refA is slot 4's current content, replaced only at a section
	// transition.
	refA string
}

// run processes one stream's units in order. A unit failure is reported and
// the stream continues (graceful degradation: siblings are worth finishing);
// an abort class stops the stream and is returned.
func (w *worker) run(ctx context.Context, stream DomainStream, out chan<- unitResult) (err error) {
	// A panic cannot cross a goroutine boundary to the coordinator, so the
	// simulated-crash sentinel is caught here and returned as an error.
	defer crashGuard(&err)

	w.traversed = []Phase{PhaseStageSetup}
	w.prev = nil
	section := ""
	for i, task := range stream.Tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if i == 0 || task.Section != section {
			// The first task's buffer fill goes through the transition too:
			// one path, so the flush is gated by the matrix every time
			// rather than every time but once.
			if err := w.enter(PhaseSectionTransition); err != nil {
				return err
			}
			if err := guard(w.phase, OpFlushRefA); err != nil {
				return err
			}
			w.refA, section = task.SectionRef, task.Section
			if err := w.enter(PhaseCallLoop); err != nil {
				return err
			}
		}
		res, err := w.unit(ctx, stream.Domain, task)
		if err != nil {
			return err
		}
		out <- res
	}
	return nil
}

// unit runs one task: build and call through the runner, then write what came
// back. The phase guards live here rather than inside the runner because the
// phase is the worker's state — the runner is stateless and shared.
func (w *worker) unit(ctx context.Context, domain string, task Task) (unitResult, error) {
	if task.Input.RefA != "" {
		return unitResult{}, TaskOwnsRefAError{Stage: w.stage, Path: task.Unit.Path}
	}
	if err := guard(w.phase, OpBuildCall); err != nil {
		return unitResult{}, err
	}

	// The input set is resolved BEFORE the model is consulted, not at write
	// time. It reads the upstream stamps, so a unit whose upstream never got
	// produced is knowable in advance — and spending a model call on work
	// that cannot be recorded is spending tokens to learn nothing. It is also
	// the SAME function the scan uses, so a stamp can never be written under
	// one derivation and checked under another.
	inputs, err := w.store.resolveInputs(task.Unit)
	if err != nil {
		return unitResult{Domain: domain, Path: task.Unit.Path, Failure: &UnitFailure{
			Stage: w.stage, Path: task.Unit.Path, Kind: FailureUpstream, Err: err,
		}}, nil
	}

	in := task.Input
	in.RefA = w.refA
	// A worker with no previous call claims nothing, which is what the
	// matrix's job-setup row already says — read from there rather than
	// spelled a second time here.
	frontier, err := Frontier(PhaseJobSetup)
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

	res, err := w.runner.Run(ctx, Call{
		Stage: w.stage, Unit: task.Unit.Path, Agent: w.agent,
		Input: in, Frontier: frontier, Prev: w.prev,
	})
	// A call was attempted, so the traversal restarts from whatever phase
	// the worker is standing in — w.phase rather than the literal, so a
	// phase inserted between the build and here cannot make the traversal
	// lie about where the next call was built.
	w.traversed = []Phase{w.phase}
	if err != nil {
		w.prev = nil
		if f, ok := w.classify(task, err); ok {
			return unitResult{Domain: domain, Path: task.Unit.Path, Usage: f.Usage, Failure: f}, nil
		}
		return unitResult{}, err
	}
	w.prev = res.Hashes

	if err := guard(w.phase, OpWriteArtifact); err != nil {
		return unitResult{}, err
	}
	data, err := w.agent.role.Encode(res.Artifact)
	if err != nil {
		return w.writeFailure(domain, task, res, fmt.Errorf("encode: %w", err)), nil
	}
	if err := w.store.Put(task.Unit.Path, data, inputs); err != nil {
		return w.writeFailure(domain, task, res, err), nil
	}
	if err := guard(w.phase, OpAdvanceUnit); err != nil {
		return unitResult{}, err
	}
	return unitResult{
		Domain: domain, Path: task.Unit.Path, Produced: true,
		Degraded: res.Degraded, Usage: res.Usage,
	}, nil
}

// classify sorts a runner error into "inventory it and keep going" or "stop".
// The dividing line is whether the rest of the stream is still worth running:
// a unit that failed verification says nothing about its siblings, while a
// frozen-prompt violation or a cancelled context says everything.
func (w *worker) classify(task Task, err error) (*UnitFailure, bool) {
	var unit UnitFailure
	if errors.As(err, &unit) {
		return &unit, true
	}
	var budget prompt.ErrOverBudget
	if errors.As(err, &budget) {
		// Refuse-and-split: the unit is too big for one call, which is the
		// skeleton's problem and not this run's. Inventoried so the report
		// names what to re-split.
		return &UnitFailure{Stage: w.stage, Path: task.Unit.Path, Kind: FailureBudget, Err: err}, true
	}
	return nil, false
}

// writeFailure records a verified artifact that could not be stored. It is a
// unit failure rather than an abort: one unwritable path (a name collision, a
// full disk on one volume) does not mean the next one fails too, and the
// inventory is more useful than the first error.
func (w *worker) writeFailure(domain string, task Task, res CallResult, err error) unitResult {
	return unitResult{
		Domain: domain, Path: task.Unit.Path, Usage: res.Usage,
		Failure: &UnitFailure{
			Stage: w.stage, Path: task.Unit.Path, Kind: FailureWrite,
			SemanticAttempts: res.Attempts, Usage: res.Usage, Err: err,
		},
	}
}

// enter moves the worker into a phase and records the traversal.
func (w *worker) enter(p Phase) error {
	if err := enterPhase(p, w.lg); err != nil {
		return err
	}
	w.phase = p
	w.traversed = append(w.traversed, p)
	return nil
}

// TaskOwnsRefAError reports a task that filled reference buffer A itself.
// Slot 4 is the worker's, replaced only at a section transition, and a task
// that set it would be a per-call buffer wearing the multi-call buffer's
// position — the one thing the §7 slot ordering cannot survive. It is a
// typed value like every other refusal here, so a caller can tell it from an
// I/O failure.
type TaskOwnsRefAError struct {
	Stage string
	Path  string
}

func (e TaskOwnsRefAError) Error() string {
	return fmt.Sprintf("pipeline: %s: task %s set reference buffer A directly; the worker fills it from SectionRef",
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
func validPaths(scan ScanResult) map[string]bool {
	valid := make(map[string]bool)
	for _, v := range scan.Verdicts {
		if v.Verdict == VerdictValid {
			valid[v.Path] = true
		}
	}
	return valid
}

// filterStreams drops the tasks the scan proved and the streams that empty out.
// This is the resume made operational: the scan's verdicts ARE the worklist
// filter, so there is no second decision about what to redo.
func filterStreams(streams []DomainStream, valid map[string]bool) []DomainStream {
	if len(valid) == 0 {
		return streams
	}
	out := make([]DomainStream, 0, len(streams))
	for _, s := range streams {
		kept := make([]Task, 0, len(s.Tasks))
		for _, t := range s.Tasks {
			if !valid[t.Unit.Path] {
				kept = append(kept, t)
			}
		}
		if len(kept) > 0 {
			out = append(out, DomainStream{Domain: s.Domain, Tasks: kept})
		}
	}
	return out
}

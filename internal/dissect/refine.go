package dissect

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/prompt"
	"kbase/internal/survey"
	"kbase/internal/text"
)

// This file is §5 step 2: the model-facing half of boundary refinement, wired
// through the real orchestrator (internal/pipeline) rather than around it.
//
// The shape §5 specifies is a serial scan — one call per boundary, each
// window loaded into the end of the context and overwriting the previous, so
// the context stays O(1) while the calls are O(n). That is why StagePlan
// emits ONE serial lane: a worker owns a domain and runs it serially, so a
// single lane IS the serial scan, and the per-call Content slot holding
// this boundary's window IS the overwrite.
//
// # The scan is a FOLD
//
// The serial scan judges each boundary against the cut list as it stands NOW,
// not against the frozen mechanical fallback. An accepted choice updates the
// working list, and every later boundary's window, menu and verification come
// off that updated list.
//
// It has to be that way, and the counterexample is one line: a 70-token
// section whose left boundary moves 6 tokens in and whose right boundary then
// moves 6 tokens back composes to 58, under the 64-token floor, with each move
// having verified against the ORIGINAL neighbours and neither being wrong on
// its own. Judged against current state the second move is simply refused —
// the slack is real, it is finite, and it goes to whoever is adjudicated
// first. No pre-rationing, no halved freedom, and no separate folding step
// left for a later stage to forget (ARCHITECTURE.md §5).
//
// Two consequences run through the rest of this file. The stage's output is
// ONE artifact — the composed cut list, whole-list-Verified before it is
// written — so the per-boundary calls produce no artifacts and are
// pipeline.LaneTask.Contributes. And the Refiner is MUTABLE while the stage runs,
// which is sound only because the stage is one serial lane and therefore one
// worker: see Refiner.enter, which asserts it rather than assuming it.
//
// It is a fallback-backed seam in the §12 sense: the mechanical cut list is
// already valid, so a model that answers badly, or not at all, costs quality
// and never correctness. The one failure that is not treated that way is the
// tripwire — see ErrVerifierDefect below and the package comment above it.

const (
	// menuLabelWords bounds the text a menu entry quotes from the document.
	// The entry exists so the model can tell one candidate from another, not
	// so it can read the section twice — the section is already in the
	// window above it.
	menuLabelWords = 12

	// menuCap bounds the menu at seven entries: three candidates before the
	// mechanical cut, the mechanical cut itself, three after (§9).
	//
	// Two things at once. Seven is the honest ceiling for a choice a small
	// tier reasons over — a window of dense prose can hold a hundred
	// candidates, and a hundred-way question is not a better question. And it
	// is what makes this seam's prompt bounded BY CONSTRUCTION (§12): with the
	// window capped per side and the menu capped in entries, every input to
	// this call is a named constant, so an over-budget build here can only be
	// our own sizing arithmetic being wrong.
	menuCap = 7

	// menuSide is how many candidates flank the incumbent. Derived, so the
	// menu is symmetric by construction and moving menuCap moves both sides.
	menuSide = (menuCap - 1) / 2

	// incumbentMark labels the mechanical cut in the rendered menu. Confirming
	// the boundary is the most common correct answer at a fallback-backed seam, and
	// a model that cannot see which entry it is cannot deliberately give it.
	incumbentMark = " (current)"

	// boundarySuffix names a boundary's CALL. It is not an artifact path:
	// nothing is ever written there. The name exists so a boundary is one
	// thing in the log, in a failure, and in the table this stage's verifier
	// correlates a response with.
	boundarySuffix = ".boundary"

	// cutListUnit names the stage's ONE output artifact: the composed cut
	// list, in the stage's unit directory.
	//
	// One artifact rather than one per boundary is the fold's own shape read
	// from the store side. A per-boundary artifact would be individually
	// reusable, and boundary i+1 was adjudicated against boundary i's
	// ACCEPTED position — a dependency no stamp records — so a resume that
	// reused i+1 while redoing i would compose an answer to a question nobody
	// asked. The honest alternatives were a chained per-boundary digest or
	// stage-granular resume; the ruling took the second (ARCHITECTURE.md §12).
	cutListUnit = "cutlist.txt"

	// paramsInput names the parameter digest the composed cut list is stamped
	// with. The colon namespaces it the way pipeline's own upstream inputs are
	// namespaced, so it can never collide with a caller's name.
	paramsInput = "dissect:parameters"
)

// stubDefinition and stubTaskDef are PLACEHOLDER prompt text.
//
// TODO(embedded-definitions): the real gemma-4-tuned definition, its
// `## CRITICAL` section and its eval belong to the embedded-definitions burst
// (ROADMAP). What is being exercised here is the seam — window, menu,
// verifier, fallback, retry, fallback — and none of that depends on the
// wording. Keeping a stub visible and marked is honest; inventing tuned text
// nobody evaluated would look like the real thing.
const (
	stubDefinition = "# Boundary refinement\n\n" +
		"You are given the end of one section, the start of the next, and a numbered list of the\n" +
		"positions the boundary between them may be moved to.\n\n" +
		"## CRITICAL\n\n" +
		"GroupingAnswer with one number from the list and nothing else.\n"

	stubTaskDef = "Task: choose the position in the numbered list that best separates the two sections."
)

// Boundary is one adjudication as the fold currently has it: the cut between
// two sections, the window the model may move it within, and the candidates
// inside that window as a numbered menu.
//
// It is a SNAPSHOT, derived from the working cut list when the question is
// asked (Refiner.at). A boundary the fold has not reached yet still sits at
// its mechanical cut; its window and menu, though, are already the ones its
// neighbours' accepted positions imply.
//
// The menu is the model's whole vocabulary. It answers with a number, so a
// cut through the middle of a heading or a code fence is not a wrong answer
// it could give — it is not an answer it can express (§5).
type Boundary struct {
	// Unit names this boundary's call. Only the last boundary's name is an
	// artifact path — the composed cut list, which its call writes.
	Unit string
	// Index is the position in the cut list of the section that STARTS at
	// Cut, so cuts[Index-1] and cuts[Index] are the two neighbours.
	Index int
	// Cut is the incumbent: where this boundary stands now, which is its
	// mechanical cut until it is adjudicated. It is the fallback, the window's
	// centre, and the entry marked (current) in the menu.
	Cut int
	// Window is the clamp (MoveWindows).
	Window MoveWindow
	// Menu is the candidates inside the window, in document order. Menu[0]
	// is choice 1.
	Menu []survey.CutCandidate
}

// Refiner is the boundary-refinement stage over one span's mechanical cut
// list: the questions it asks, the fold that answers them, and the composed
// cut list that comes out.
//
// It is built when the stage's work is described and MUTATES while the stage
// runs — the working list is the fold. That is safe for exactly one reason,
// stated here because everything below depends on it: the stage is one domain
// lane, so one worker walks its boundaries in order and no second goroutine
// is ever inside the fold. enter asserts it.
type Refiner struct {
	src    []byte
	span   survey.Span
	cands  []survey.CutCandidate
	params Params
	def    prompt.Definition
	effort model.RequestEffort
	lg     log.Logger

	// mech is the mechanical cut list, frozen. It is what the stage's
	// parameter digest is taken over, and the list NewRefiner verified before
	// any of this was allowed to run.
	mech []survey.Span

	// calls names each interior boundary's call, in document order;
	// calls[i-1] belongs to boundary i. The LAST one is the composed
	// artifact's path, because the call that adjudicates the last boundary is
	// the one that writes it — see StagePlan. byUnit is the reverse table, so
	// a response can be routed back to the boundary it answers.
	calls  []string
	byUnit map[string]int

	// stage is the stage's name, for the log. Set by StagePlan, which is
	// where a stage learns what it is called.
	stage string

	// --- fold state, owned by the single worker running the stage's lane.
	//
	// busy and next are that ownership asserted rather than assumed — the
	// exclusivity and the ordinal halves of it. work is the cut list as it
	// stands; windows are its clamps, recomputed whenever it moves; the
	// counters are the stage's metrics. started and composed are the fold's
	// own lifecycle: logged once, run once.
	busy     atomic.Bool
	next     int
	work     []survey.Span
	windows  []MoveWindow
	moved    int
	falls    int
	rejects  int
	tokens   int
	started  bool
	composed bool

	// latched is the first defect the INPUT BUILDER found, which it has no
	// way to return (pipeline.InputBuilder yields no error) and which verify
	// raises on its behalf. It carries its own mutex because the case it
	// exists for is exactly the one where the fold's exclusivity has already
	// failed, and a racy latch would be a race in the code that reports races.
	latchMu sync.Mutex
	latched error
}

// NewRefiner builds the stage over a mechanical cut list, with unitDir as the
// store directory its composed cut list lands in. The mechanical list is also
// the fold's seed: the working list starts as a copy of it and every
// adjudication moves it from there.
//
// effort is the refinement definition's declared ask (model.RequestEffort). It is
// positional and unavoidable on purpose: the value belongs to whoever
// registers this stage, and a package-level default here would be this
// package quietly answering a question only the registration site can
// (ARCHITECTURE.md §9, §12).
//
// It verifies the mechanical list first, and that is not a formality. The
// fallback is what every failure falls back to, so a fallback that does not
// check out is a fallback that would emit unverified material; and the
// tripwire firing here — before a single call — is the offset pipeline being
// broken in a way no model interaction could have caused. Both are worth
// learning at stage setup rather than n calls later.
func NewRefiner(src []byte, span survey.Span, cands []survey.CutCandidate, cuts []survey.Span, unitDir string, p Params, effort model.RequestEffort, lg log.Logger) (*Refiner, error) {
	if unitDir == "" {
		// Every path is built from it, so an empty one yields "/cutlist.txt"
		// — a path the store refuses much later, naming the artifact rather
		// than the caller who never said where the stage's work goes.
		return nil, fmt.Errorf("dissect: the refinement stage has no unit directory to write its cut list into")
	}
	if err := Verify(src, span, cands, cuts, nil, p); err != nil {
		return nil, fmt.Errorf("dissect: the mechanical cut list this stage would fall back to does not verify: %w", err)
	}
	def, err := prompt.ParseDefinition(stubDefinition)
	if err != nil {
		return nil, fmt.Errorf("dissect: parsing the refinement definition: %w", err)
	}

	r := &Refiner{
		src:    src,
		span:   span,
		cands:  cands,
		params: p,
		def:    def,
		effort: effort,
		lg:     lg,
		// Cloned, both of them: the caller keeps its own slice (Split hands
		// back one it built), and the fold writes into work.
		mech:   slices.Clone(cuts),
		work:   slices.Clone(cuts),
		byUnit: make(map[string]int, len(cuts)),
		// The fold starts at the first interior boundary and walks forward.
		next: 1,
	}
	r.windows = MoveWindows(src, r.work, p)
	for i := 1; i < len(cuts); i++ {
		// The last boundary's call carries the composed artifact, so its name
		// IS the artifact's path — one name for the call and the thing the
		// call finally writes.
		name := fmt.Sprintf("%s/%04d%s", unitDir, i, boundarySuffix)
		if i == len(cuts)-1 {
			name = unitDir + "/" + cutListUnit
		}
		r.calls = append(r.calls, name)
		r.byUnit[name] = i
	}
	return r, nil
}

// Boundaries returns the adjudications in document order, as they stand NOW.
//
// It is a view of live fold state, so it answers a different question before
// the stage runs (the mechanical questions) than after it (the ones the fold
// arrived at), and it REFUSES while the fold is running. The signature said
// none of that before: a caller could read a working list mid-scan and get a
// list half of one run and half of another. Its callers read it between runs —
// a plan describing itself, a verb reporting what moved, a test setting a
// scene — and that is now what it permits.
func (r *Refiner) Boundaries() ([]Boundary, error) {
	if err := r.claim(); err != nil {
		return nil, err
	}
	defer r.leave()
	out := make([]Boundary, 0, len(r.calls))
	for i := 1; i <= len(r.calls); i++ {
		out = append(out, r.snapshot(i))
	}
	return out, nil
}

// at is boundary i for the FOLD: the snapshot, plus the ordinal half of the
// single-owner assertion.
//
// The fold is a serial scan, so the only boundary that can legitimately be
// asked is the one it expects next. Checking that catches interleaving on the
// BUILD path as well as the verify path — two lanes over one Refiner race on
// the working list inside callInput, before the exclusivity CAS in verify
// could fire, and the menu they raced to build would already be derived from a
// torn read. next advances only on a terminal outcome (accept, fallback), so
// the single informed retry re-asks the same boundary without tripping it.
func (r *Refiner) at(i int) (Boundary, error) {
	if i != r.next {
		return Boundary{}, fmt.Errorf(
			"dissect: boundary %d was entered while the fold expects boundary %d; "+
				"the fold is one serial scan over one working list", i, r.next)
	}
	return r.snapshot(i), nil
}

// snapshot is boundary i as the working list has it: the window it may move in
// and the menu it may choose from, both derived from the CURRENT neighbours.
// It asks nothing about whose turn it is — see at, and Boundaries.
func (r *Refiner) snapshot(i int) Boundary {
	w := r.windows[i-1]
	cut := r.work[i].Start
	return Boundary{
		Unit:   r.calls[i-1],
		Index:  i,
		Cut:    cut,
		Window: w,
		Menu:   menu(r.cands, w, cut),
	}
}

// last reports whether boundary i is the one whose call writes the composed
// cut list.
func (r *Refiner) last(i int) bool { return i == len(r.calls) }

// claim takes the fold's state for the calling goroutine, and refuses if
// anyone else already holds it.
//
// The fold is single-writer by construction — one serial lane, one worker —
// and this is that construction asserted at every seam that depends on it,
// the read path included. A plan that fanned these boundaries across two
// lanes would otherwise interleave two scans over one working list and
// produce a cut list neither of them adjudicated.
func (r *Refiner) claim() error {
	if !r.busy.CompareAndSwap(false, true) {
		return errors.New("dissect: this span's fold is already in use; " +
			"it is one serial scan over one working list and the stage plans exactly one serial lane")
	}
	return nil
}

func (r *Refiner) leave() { r.busy.Store(false) }

// enter claims the fold for one adjudication step: claim, plus the two things
// that make a step legitimate at all.
//
// A Refiner is ONE RUN's. The working list, the windows and the counters are
// seeded at NewRefiner and never reset, so a second fold over the same
// instance would restart from a half-folded list and re-adjudicate boundary 1
// against the positions boundaries 1..k had already moved to. The composed
// list would still verify, so nothing would be loud about it — it would simply
// be the answer to a question nobody asked, which is the failure §12's
// stage-granularity ruling exists to make unrepresentable. So composing the
// list spends the Refiner, and entering a spent one is a defect that names the
// remedy.
func (r *Refiner) enter() error {
	if err := r.claim(); err != nil {
		return err
	}
	if r.composed {
		r.leave()
		return errors.New("dissect: this span's cut list has already been composed; " +
			"construct a new Refiner per run")
	}
	if !r.started {
		// One record per fold — and a Refiner is one run's, so a second
		// "started" for the same span with no composed list between them is an
		// interrupted fold seen from the log alone.
		r.started = true
		r.lg.Info("boundary fold started", "stage", r.stage,
			"boundaries", len(r.calls), "sections", len(r.work))
	}
	return nil
}

// latch records a defect the input builder found, and refuse hands the builder
// back the only thing it has left to say.
//
// pipeline.InputBuilder returns no error by design — a builder assembles
// material the stage already holds and already verified — so a refusal
// discovered there has nowhere to go at the moment it is found. It is latched
// instead, and verify raises it as a defect on the same boundary: the next
// thing this unit does, and a seam that can classify it. The empty input is
// deliberate; a fold that has refused the question has no legitimate prompt to
// build for it.
func (r *Refiner) latch(err error) {
	r.latchMu.Lock()
	defer r.latchMu.Unlock()
	if r.latched == nil {
		r.latched = err
	}
}

func (r *Refiner) latchedErr() error {
	r.latchMu.Lock()
	defer r.latchMu.Unlock()
	return r.latched
}

func (r *Refiner) refuse(err error) prompt.CallInput {
	r.latch(err)
	r.lg.Error("the boundary fold refused to build a call", "stage", r.stage, "error", err)
	return prompt.CallInput{}
}

// StagePlan describes the stage for the coordinator: the ask every worker
// runs, the stage-constant context, and one serial lane of boundary calls
// ending in the one unit the stage owes.
//
// inputs are the named hashes the composed cut list is stamped with — the
// source identity and the upstream artifacts this cut list was derived from.
// They are the caller's because the caller is the one that knows what job
// this span came out of.
//
// The stage's OWN parameters are appended here rather than trusted to the
// caller, because the artifact means nothing without them: this run's
// `cuts/cutlist.txt` and a re-plan's are the same path holding a list
// adjudicated for two different sets of questions. A stamp that proves less
// than it appears to is the one failure §12 does not permit, so the stage
// derives the input it alone can derive.
//
// Every task but the last contributes: its answer lands in the working list
// and nothing else. The last one carries the artifact, so the stage's unit
// exists exactly when the whole fold has run — which is what makes an
// interrupted fold Absent rather than half-proven.
//
// Each task's input is BUILT when the worker reaches it
// (pipeline.InputBuilder). It has to be: boundary i's window and menu come off
// the list boundary i-1 left behind, so they do not exist when this plan is
// described.
func (r *Refiner) StagePlan(name string, inputs []pipeline.Input) *pipeline.StagePlan {
	r.stage = name
	stamped := slices.Concat(inputs, []pipeline.Input{{Name: paramsInput, Hash: r.digest()}})
	tasks := make([]pipeline.LaneTask, 0, len(r.calls))
	for i := 1; i <= len(r.calls); i++ {
		task := pipeline.LaneTask{
			Owed:        pipeline.OwedArtifact{Path: r.calls[i-1]},
			Contributes: !r.last(i),
			Section:     name,
			SectionRef:  r.sectionRef(),
			Input:       func() prompt.CallInput { return r.callInput(i) },
		}
		if r.last(i) {
			task.Owed.Inputs = stamped
		}
		tasks = append(tasks, task)
	}
	return &pipeline.StagePlan{
		Name: name,
		Ask: pipeline.AskSpec{
			Def: r.def,
			// The light tier: refinement is the parallel, checklist-shaped
			// work §10 maps to 26B-A4B.
			Tier: config.TierLight,
			// The caller's declaration, passed through untouched. This stage
			// has an opinion about it and no standing to hold one: the ask
			// is a property of the definition, and the definition is
			// registered outside this package.
			Effort:   r.effort,
			Verify:   r.verify,
			Encode:   EncodeCutList,
			Fallback: r.fallback,
		},
		Spec: prompt.StageSpec{TaskDef: stubTaskDef},
		Lanes: func() ([]pipeline.SerialLane, error) {
			// One lane, so the boundaries are walked in order by one
			// worker: §5's serial scan, and the reason each window can
			// overwrite the last.
			return []pipeline.SerialLane{{Domain: name, Tasks: tasks}}, nil
		},
	}
}

// digest is the stage's parameter digest: a canonical rendering of everything
// outside the store that determined this stage's questions — the span, the
// budget it was cut to, the effort they were asked at, and the mechanical cut
// list itself.
//
// Canonical means one line per fact in a fixed order, so the same parameters
// hash the same on every platform and a different span, budget, effort or
// mechanical list all produce a different digest. The cut list is in it
// because it is computed in-process rather than read from an upstream
// artifact: without it the store has no way to see that the boundary changed.
// It is taken over the MECHANICAL list, which the fold never touches: the
// digest identifies the questions the stage asked, and the working list is the
// answers.
//
// The effort is in it because §3's claim is that the provenance tuple
// REPRODUCES the artifact, and a cut list adjudicated with thinking on and one
// adjudicated with it off are two answers to two askings. Today nothing can
// observe the difference (`dev-refine` runs ModeFresh), which is the reason to
// close it now rather than the reason not to: the day a resume meets an
// artifact from an operator's experiment is not the day to discover that the
// stamp never recorded which one it was.
func (r *Refiner) digest() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "span %d %d\nbudget %d\nthinking %t\n",
		r.span.Start, r.span.End, r.params.BudgetTokens, r.effort.Thinking)
	for _, c := range r.mech {
		fmt.Fprintf(&sb, "cut %d %d\n", c.Start, c.End)
	}
	return pipeline.HashBytes([]byte(sb.String()))
}

// sectionRef is the stage reference buffer for the stage: what is stable
// across every call of this span's scan.
//
// The counts it renders are the section count and the boundary count, and the
// fold moves neither — a cut list is re-tiled, never re-sized — so slot 4 is
// stage-constant under the fold as it was before it. That is not decoration:
// the churn tripwire aborts the worker if this text changes between calls of
// one lane.
//
// No byte offsets. The model answers with a menu number and has no use for a
// raw offset, so showing it one is an invitation to answer with one — and
// `parseChoice` reads a number out of the response, which makes any stray
// number in the prompt a number the response might echo (§5).
func (r *Refiner) sectionRef() string {
	return fmt.Sprintf("This span holds %d sections and %d boundaries to adjudicate.",
		len(r.work), len(r.calls))
}

// callInput is one boundary's per-call half of the prompt, built when the
// worker reaches the boundary — so the window and the menu are the ones the
// fold's current cut list implies, not the ones the mechanical list did.
//
// The window goes in the Content slot — it is the material under work — and
// the menu in the call reference buffer, which is §7's slot for transient
// material specific to the current content. The status lines are ordered
// most→least stable, so the line that changes every call is last. Like
// sectionRef, none of it renders a byte offset.
func (r *Refiner) callInput(i int) prompt.CallInput {
	// The build reads the working list, so it takes the fold's own guard: the
	// hazard W1 named is a torn read HERE, in the worker goroutine, before any
	// exclusivity check in verify could fire.
	if err := r.enter(); err != nil {
		return r.refuse(err)
	}
	defer r.leave()
	b, err := r.at(i)
	if err != nil {
		return r.refuse(err)
	}
	return prompt.CallInput{
		StatusLines: []string{
			fmt.Sprintf("Boundary %d of %d", b.Index, len(r.calls)),
		},
		Content:            string(r.src[b.Window.Lo:b.Window.Hi]),
		CallRef:            renderMenu(r.src, b),
		AcceptanceCriteria: []string{"- answer with one number from the list, nothing else"},
	}
}

// verify maps a response back to a byte offset and folds it into the working
// cut list.
//
// The check is the WHOLE list with this boundary moved, run through the same
// Verify every other caller uses, rather than a per-boundary re-implementation
// of membership, clamp, minimum size and tripwire. One implementation is the
// point: a second one would be a second thing to keep true, and the two would
// disagree on exactly the day it mattered.
//
// The list it checks is the CURRENT one — every move accepted so far, plus
// this one. That is what makes the checks compose: the minimum does not
// survive n independent moves judged against frozen neighbours (see the fold
// note at the top of this file), and judged against current state it needs no
// rationing to survive at all. A boundary whose neighbour has already spent
// the slack is simply told so, retried once with the reason, and left where it
// stands.
//
// A rejection changes nothing. Only an accepted choice writes, so the informed
// retry re-asks the same question against the same list, and a boundary that
// falls back leaves the list exactly as the previous boundary did.
//
// The error classification is exhaustive by construction rather than by
// enumeration: RejectionError is the only class a model's answer can be
// responsible for, so everything else — the tripwire, a window-count
// mismatch, a span this package should never have been handed, any plain error
// a future check adds — is ours and is wrapped as a defect. The taxonomy's
// default must be "blame ourselves", because the cost of the two mistakes is
// not symmetric: a defect routed to the model burns a retry and then emits a
// degraded unit from a broken derivation, while a model failure routed to the
// defect path stops the job loudly.
func (r *Refiner) verify(artifactPath, response string) (any, error) {
	i, ok := r.byUnit[artifactPath]
	if !ok {
		return nil, r.defect(fmt.Errorf("dissect: %s is not a boundary of this span", artifactPath))
	}
	if err := r.enter(); err != nil {
		return nil, r.defect(err)
	}
	defer r.leave()
	// A refusal the input builder had no way to return (see latch): the call
	// that reached here was built from nothing, so no response could be right.
	if err := r.latchedErr(); err != nil {
		return nil, r.defect(err)
	}

	b, err := r.at(i)
	if err != nil {
		return nil, r.defect(err)
	}
	n, err := parseChoice(response, len(b.Menu))
	if err != nil {
		return nil, r.reject(b, RejectionError{Offset: b.Cut, Reason: err.Error()})
	}
	at := b.Menu[n-1].Offset

	moved := slices.Clone(r.work)
	moved[i-1].End, moved[i].Start = at, at

	if err := Verify(r.src, r.span, r.cands, moved, r.windows, r.params); err != nil {
		var rej RejectionError
		if errors.As(err, &rej) {
			return nil, r.reject(b, rej)
		}
		return nil, r.defect(err)
	}
	r.accept(b, moved, at)

	// Every boundary but the last hands its answer to the working list and
	// nothing else; the last one composes what they all built. A nil artifact
	// rather than a copy of the list, because the task contributes and the
	// worker discards what it hands back — n−1 clones of the cut list per fold
	// with no reader is a cost the doc comment above already explains away.
	if !r.last(i) {
		return nil, nil
	}
	list, err := r.compose(artifactPath)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// accept installs a boundary's choice: the working list becomes the moved one,
// the clamps are recomputed off it, and the metrics record what it cost.
//
// The windows are recomputed rather than kept, because a window is a
// neighbourhood: moving this boundary changes how far the next one may reach
// back, and a stale clamp would let the fold hand a later boundary a window
// its own sections no longer justify. Each recomputed window is centred on its
// own current cut, so an already-accepted position is never re-judged against
// a window drawn after it.
// It is one of the two TERMINAL outcomes, so it is where the fold's ordinal
// advances: a rejection changes nothing and the informed retry re-asks this
// same boundary.
func (r *Refiner) accept(b Boundary, moved []survey.Span, at int) {
	r.work = moved
	r.windows = MoveWindows(r.src, r.work, r.params)
	r.next++

	if at != b.Cut {
		r.moved++
	}
	spent := r.windowTokens(b)
	r.tokens += spent
	r.lg.Info("boundary adjudicated", "stage", r.stage, "unit", b.Unit,
		"boundary", b.Index, "of", len(r.calls), "outcome", "accepted",
		"move_tokens", r.params.estimate(r.src[min(b.Cut, at):max(b.Cut, at)]),
		"window_tokens", spent)
}

// reject records a rejected answer and returns it on its way to the model.
//
// The runner logs the rejection too, as one call's outcome; this record is the
// same event in the fold's own series, where it carries the boundary number
// and lines up with the accept and fallback records around it.
func (r *Refiner) reject(b Boundary, rej RejectionError) error {
	r.rejects++
	r.lg.Info("boundary adjudicated", "stage", r.stage, "unit", b.Unit,
		"boundary", b.Index, "of", len(r.calls), "outcome", "rejected",
		"reason", rej.Note())
	return rejection{rej}
}

// windowTokens estimates the material a boundary's call carries — the window
// in the content slot, which is all of it that scales. The stage's accounting
// adds it once per boundary, not once per attempt: it is a measure of the work
// a redo would have to do again, and a redo asks each boundary afresh.
func (r *Refiner) windowTokens(b Boundary) int {
	return r.params.estimate(r.src[b.Window.Lo:b.Window.Hi])
}

// compose is the stage's output: the working cut list, checked WHOLE before
// anyone writes it.
//
// The check is belt-and-braces and is meant to stay that way. Every accepted
// move verified the whole list at the moment it was made, so a composed list
// that does not verify means the fold's own bookkeeping is wrong — not a bad
// answer — which is why it is a defect and not a rejection. It is the last
// place the §5 counterexample could still bite, and the cost of running it is
// one linear pass per stage.
//
// The windows argument is nil deliberately. The clamp was checked when each
// move was made, against the window the model was actually shown; re-checking
// it against windows recomputed from the final list would be checking a
// different claim and calling it the same one.
func (r *Refiner) compose(artifactPath string) (CutList, error) {
	if err := Verify(r.src, r.span, r.cands, r.work, nil, r.params); err != nil {
		r.lg.Info("cut list composed", "stage", r.stage, "unit", artifactPath,
			"boundaries", len(r.calls), "verify", "failed")
		return CutList{}, r.defect(fmt.Errorf("dissect: the composed cut list does not verify: %w", err))
	}
	// The stage's cost, and — because resume is stage-granular — exactly what
	// an interruption anywhere in this stage re-spends. A second one of these
	// records for the same span is a fold that was redone; what it would have
	// taken to salvage part of it is the difference between the two.
	r.lg.Info("cut list composed", "stage", r.stage, "unit", artifactPath,
		"sections", len(r.work), "boundaries", len(r.calls),
		"moved", r.moved, "fallbacks", r.falls, "rejections", r.rejects,
		"adjudicated_tokens", r.tokens, "verify", "ok")
	// The fold is finished, and finishing it spends this Refiner: see enter
	// for what a second run over a half-folded working list would produce.
	r.composed = true
	return CutList{Unit: artifactPath, Cuts: slices.Clone(r.work)}, nil
}

// defect hands the runner the classification this package is the only one that
// can make: no answer to the question we asked could land here, so it is not
// the model that is wrong. Neither remedy the runner has applies — a retry
// re-asks a question that was never the problem, and the fallback comes from
// the same derivation — so the worker aborts.
func (r *Refiner) defect(err error) error {
	return fmt.Errorf("%w: %w", pipeline.ErrVerifierDefect, err)
}

// rejection is a RejectionError on its way to the MODEL.
//
// The runner renders whatever error a verifier returns into the corrective
// note the retry carries, so the note IS Error(). RejectionError.Note is the
// rendering that belongs in a prompt; this type is what routes it there, and
// errors.As still recovers the operator-facing value underneath.
type rejection struct{ err RejectionError }

func (r rejection) Error() string { return r.err.Note() }
func (r rejection) Unwrap() error { return r.err }

// fallback is the §3 fallback: the boundary stays where the fold has it,
// which for a boundary nobody has adjudicated yet is its mechanical cut.
//
// Standing still is the whole of it — the working list already holds the
// position being kept, so there is nothing to install and nothing a fallback
// can invalidate. What it does have to do is compose, when the boundary that
// failed is the last one: the artifact is still owed, and it is owed whether
// the final answer was the model's or ours.
func (r *Refiner) fallback(artifactPath string) any {
	// An unknown unit cannot arrive through the coordinator (the calls and
	// this table are built from one loop), and a MechanicalFallback has no way to
	// refuse. The empty list is one EncodeCutList rejects, so the case
	// surfaces as a loud write failure rather than as a plausible artifact.
	i, ok := r.byUnit[artifactPath]
	if !ok {
		return CutList{Unit: artifactPath}
	}
	if err := r.enter(); err != nil {
		r.lg.Error("the boundary fold refused a fallback", "stage", r.stage, "unit", artifactPath, "error", err)
		return CutList{Unit: artifactPath}
	}
	defer r.leave()

	b, err := r.at(i)
	if err != nil {
		r.lg.Error("the boundary fold refused a fallback", "stage", r.stage, "unit", artifactPath, "error", err)
		return CutList{Unit: artifactPath}
	}
	r.falls++
	// The other terminal outcome: the boundary stands where it is and the fold
	// moves on, so the ordinal advances here as it does on an acceptance.
	r.next++
	spent := r.windowTokens(b)
	r.tokens += spent
	r.lg.Info("boundary adjudicated", "stage", r.stage, "unit", b.Unit,
		"boundary", b.Index, "of", len(r.calls), "outcome", "fallback",
		"move_tokens", 0, "window_tokens", spent)

	// Nothing to hand back: the task contributes and its artifact is discarded
	// (see verify).
	if !r.last(i) {
		return nil
	}
	list, err := r.compose(artifactPath)
	if err != nil {
		// MechanicalFallback cannot refuse, so the refusal is the artifact: an empty
		// list, which the encoder turns into a loud write failure. The defect
		// is logged here because this is the only place it is diagnosed.
		r.lg.Error("the composed cut list does not verify", "stage", r.stage, "unit", artifactPath, "error", err)
		return CutList{Unit: artifactPath}
	}
	return list
}

// CutList is the stage's artifact: the composed cut list for one span, after
// every boundary has been adjudicated.
type CutList struct {
	Unit string
	Cuts []survey.Span
}

// EncodeCutList renders the composed cut list: one section per line, start and
// end, in document order.
//
// Exported for the same reason DecodeCutList is: there is more than one writer
// of this format. The refinement stage composes a cut list from a model fold,
// and the mechanical cuts stage writes dissect.Split's output directly — one
// format, one encoder, whichever stage produced the list.
//
// Two decimals a line rather than the interior cuts alone, because a list that
// carries its own endpoints is checkable without knowing the span it came from
// — and the reader that will consume it (stage 5) is a different stage in a
// different run. Which boundaries the model moved is not in the bytes: it is
// not a property of the cut list, and the fold's log records it where a
// question about model performance is answered.
func EncodeCutList(artifact any) ([]byte, error) {
	l, ok := artifact.(CutList)
	if !ok {
		return nil, fmt.Errorf("dissect: artifact is %T, not a CutList", artifact)
	}
	if len(l.Cuts) == 0 {
		return nil, fmt.Errorf("dissect: %s has no cut list; the fold produced nothing to write", l.Unit)
	}
	var out []byte
	for _, c := range l.Cuts {
		out = fmt.Appendf(out, "%d %d\n", c.Start, c.End)
	}
	return out, nil
}

// DecodeCutList reads an encoded cut list back into the sections it records —
// EncodeCutList's counterpart, and the ONE decoder of this format.
//
// One, because there were nearly three: the dev verb re-derived the section
// list from the fold's own boundaries rather than from the bytes, the tests
// hand-rolled a Sscanf, and stage 5 will need a real one when it arrives.
// Three projections of one shape is three things to keep in step, and none of
// them makes the round trip testable.
//
// It is strict about the line shape for the same reason the encoder writes
// both endpoints: this artifact crosses a run boundary, so the reader that
// consumes it cannot ask the writer what it meant.
func DecodeCutList(data []byte) ([]survey.Span, error) {
	var cuts []survey.Span
	for n, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("dissect: cut list line %d is %q; a section is a start and an end", n+1, line)
		}
		var (
			c   survey.Span
			err error
		)
		if c.Start, err = strconv.Atoi(fields[0]); err != nil {
			return nil, fmt.Errorf("dissect: cut list line %d: %w", n+1, err)
		}
		if c.End, err = strconv.Atoi(fields[1]); err != nil {
			return nil, fmt.Errorf("dissect: cut list line %d: %w", n+1, err)
		}
		cuts = append(cuts, c)
	}
	if len(cuts) == 0 {
		return nil, errors.New("dissect: the cut list holds no sections")
	}
	return cuts, nil
}

// menu is the candidates the model may choose between: the ones inside the
// window, narrowed to at most menuCap around the incumbent.
//
// The cap is applied around the INCUMBENT rather than to the head of the
// window, so the entries offered are the ones nearest the boundary actually
// under discussion, and the incumbent is always among them — confirming where
// the boundary already stands has to be an answer the model can give. A side with fewer
// than menuSide candidates contributes what it has and does not lend its
// unused places to the other side: the menu is a neighbourhood of the
// boundary, and a "nearest seven" rule would reach seven candidates deep into
// one section whenever the other side was short.
//
// cut is where the boundary currently stands, which is always a candidate
// inside its own window: the mechanical list Verify passed at NewRefiner for a
// boundary the fold has not reached, and a candidate the fold itself accepted
// for one it has — and MoveWindows centres a window on the cut it belongs to. A
// cut that is somehow neither yields the window's first entries and no
// incumbent to mark — which cannot happen, and is not worth a second spelling
// of a check the fold already made.
func menu(cands []survey.CutCandidate, w MoveWindow, cut int) []survey.CutCandidate {
	var in []survey.CutCandidate
	incumbent := 0
	for _, c := range cands {
		if c.Offset > w.Hi {
			break
		}
		if !w.Contains(c.Offset) {
			continue
		}
		if c.Offset == cut {
			incumbent = len(in)
		}
		in = append(in, c)
	}
	lo := max(incumbent-menuSide, 0)
	hi := min(incumbent+menuSide+1, len(in))
	return in[lo:hi]
}

// renderMenu numbers the menu from 1 and labels each entry with its kind and
// the line it starts, capped — with the incumbent marked, so "keep the
// boundary where it is" is a choice rather than a coincidence.
//
// No byte offsets: the model emits no raw offsets (§5), so showing it any
// would be an invitation to answer with one. The number is the whole of what
// it may say back.
func renderMenu(src []byte, b Boundary) string {
	var sb strings.Builder
	sb.WriteString("Candidate positions:\n")
	for i, c := range b.Menu {
		mark := ""
		if c.Offset == b.Cut {
			mark = incumbentMark
		}
		fmt.Fprintf(&sb, "%d. %s: %s%s\n", i+1, c.Kind, label(src, c.Offset, b.Window), mark)
	}
	return sb.String()
}

// label quotes the line a candidate starts, capped to a few words and to the
// window.
//
// The window clamp is what keeps "the window the model saw" one thing: the
// content slot is src[Window.Lo:Window.Hi], so a label running past Hi to its
// end of line would put bytes in the prompt that the window says are not in
// it, and a boundary read back out of a log would show two different extents.
func label(src []byte, off int, w MoveWindow) string {
	end := off
	for end < len(src) && end < w.Hi && src[end] != '\n' {
		end++
	}
	if end == off {
		// A candidate at the window's own end: everything it introduces is
		// outside the window, so there is nothing shown to quote. Saying so
		// beats an entry with no text beside it, which the model could not
		// tell from any other.
		return "after the text above"
	}
	return text.CapWords(string(src[off:end]), menuLabelWords)
}

// parseChoice reads the menu number out of a response.
//
// The response must BE a number: whitespace and trailing punctuation are
// trimmed, and anything else is a rejection. Reading the first run of digits
// out of a sentence looks more forgiving and is not — a small tier that opens
// with "Boundary 2 of 3, and I would move it to 1" yields choice 2, which is a
// legal menu entry, so it passes membership and the clamp, verifies, and is
// written. That is the one failure the two nets cannot see, because both look
// at the offset and the mis-mapping happened before the offset existed. A
// rejection here costs one visible retry; a mis-map costs a wrong boundary and
// says nothing at all.
//
// Neither message names a number. They become the retry's retry note
// (RejectionError.Note), and the menu's own size is a legal answer — telling a
// model that answered "99" that there are five positions hands it "5" in the
// one prompt where it is most likely to be pattern-matching.
func parseChoice(response string, size int) (int, error) {
	s := strings.TrimRight(strings.TrimSpace(response), ".,;:!?)]}\"'")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.New("the answer must be one number from the list and nothing else")
	}
	if n < 1 || n > size {
		return 0, errors.New("that is not one of the listed positions")
	}
	return n, nil
}

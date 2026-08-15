// Package taxonomy is pipeline stage 3 (ARCHITECTURE.md §4 row 3;
// TEMP_DESIGN_V020_RESHAPE §3): the container descent that designs the KB
// tree, composed as a fold over one serial lane.
//
// # The shape
//
// The question set is mechanical and fixed before the first call: one question
// per container of the SOURCE's own tree — the corpus root, each folder, each
// document — presenting that container's direct entries as a numbered
// candidate list (see descent.go, which also states the two places this
// narrows the design). The answer is data: a partition of the numbered entries
// into groups, each with a title, a one-line scope and a `page`/`section`
// flag. No path, no filename, no link — those are the namer's, downstream of
// this package (I-2).
//
// The set is fixed; the CALLS are not. A container the descent reaches to find
// already absorbed — its parent's answer put its material on a page — is a
// question with no answer left to give, and its call is not made at all
// (Designer.callInput). That is what keeps a discarded answer from being able
// to fail anything: the cheapest guarantee that a call cannot fail is that it
// does not exist.
//
// Every call but the last CONTRIBUTES: its answer lands in the working
// proposal and nothing is written. The last call composes the proposal through
// treeplan.Verifier.Compose — where §2.7's level operators, §2.4's split
// expansion, the namer and every §3.3 post-condition run — and the composed
// tree plan is the stage's one artifact. That is stage 4's fold shape on the
// same built machinery, and it makes resume stage-granular by construction: an
// interrupted descent leaves nothing to salvage, which is the correct trade at
// one artifact and tens of calls (O-13).
//
// # A no-fallback seam
//
// There is no mechanical fallback. A mechanically grouped tree with generated
// titles is not "already acceptable" (O-1), so `AskSpec.Fallback` is nil, the
// seam is `pipeline.NoFallbackSeam`, and a container whose answer does not
// verify twice fails the unit — which poisons the composed artifact and makes
// the job refuse emission. treeplan.SourceStructureProposal exists for the
// no-model dev build and is never reachable from here.
package taxonomy

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/prompt"
	"kbase/internal/survey"
	"kbase/internal/treeplan"
)

const (
	// containerSuffix names a container's CALL. It is not an artifact path:
	// nothing is ever written there. The name exists so a container is one
	// thing in the log, in a failure, and in the table the verifier correlates
	// a response with.
	containerSuffix = ".container"

	// paramsInput names the parameter digest the composed tree plan is stamped
	// with. The colon namespaces it the way pipeline's own upstream inputs are
	// namespaced, so it can never collide with a caller's name.
	paramsInput = "taxonomy:parameters"
)

// Design is what one descent is over: the corpus as the survey describes it,
// the verifier that will compose and check the artifact, and the identities
// only the registration site can supply.
//
// It is a struct rather than nine positional parameters because every field is
// a different kind of thing and none of them has a defensible default here —
// the effort is the definition's declaration, the title and scope are the
// corpus's, and the unit is the composing verb's store layout.
type Design struct {
	// Survey is the taxonomy stage's only view of the corpus (§4 row 3).
	Survey survey.Artifact

	// Verifier composes and checks the artifact. It holds the same survey and
	// the corpus under custody, because §2.4's split expansion runs
	// dissect.Split for real.
	Verifier *treeplan.Verifier

	// Params is the operating point the descent's caps are read from — the
	// same value the Verifier was built with.
	Params treeplan.Params

	// Title and Scope name the entry-point. They are the corpus's own identity
	// and not a model's: no call in this descent is about the corpus as a
	// whole, and inventing a considered-sounding title here would hide that.
	Title string
	Scope string

	// Annexes are the declared territories (§2.8). Material under one is not
	// enumerated at all, and the prefixes travel into the composed artifact.
	Annexes []treeplan.Annex

	// Unit is the store path of the composed tree plan. The container calls
	// are named beside it, in its own directory.
	Unit string

	// Effort is the definition's declared ask (model.RequestEffort), threaded
	// from the registration site to the wire.
	Effort model.RequestEffort

	// Retry is the definition's declared retry policy (pipeline.RetryPolicy),
	// threaded from the same place for the same reason.
	Retry pipeline.RetryPolicy
}

// Designer is the descent over one corpus: the questions it asks, the fold
// that answers them, and the composed tree plan that comes out.
//
// It MUTATES while the stage runs — the working proposal is the fold — which
// is safe for exactly one reason, stated here because everything below depends
// on it: the stage is one serial lane, so one worker walks the containers in
// order and no second goroutine is ever inside the fold. enter asserts it
// rather than assuming it.
type Designer struct {
	in  Design
	def prompt.Definition
	lg  log.Logger

	// stage is the stage's name, for the log. Set by StagePlan, which is where
	// a stage learns what it is called.
	stage string

	// asks are the container calls in depth-first order, parents first;
	// byUnit is the reverse table, so a response can be routed back to the
	// container it answers.
	asks   []*ask
	byUnit map[string]int

	// --- fold state, owned by the single worker running the stage's lane.
	busy     atomic.Bool
	next     int
	root     *pnode
	attach   map[*cand]*pnode
	groups   int
	pages    int
	dropped  int
	rejects  int
	started  bool
	composed bool

	// latched is the first defect the input builder found, which it has no way
	// to return (pipeline.InputBuilder yields no error) and which verify raises
	// on its behalf. It carries its own mutex because the case it exists for is
	// the one where the fold's exclusivity has already failed.
	latchMu sync.Mutex
	latched error
}

// pnode is the working proposal: the tree as the descent has it so far.
//
// It is a pointer tree because the fold appends to a node an earlier call
// created, which treeplan.TreeProposal's value children cannot express. It is
// converted to the proposal at compose time and never before — between the two
// sit the operators, the expansion and the namer, and a caller holding this
// would be holding a tree no post-condition has run over.
type pnode struct {
	title   string
	scope   string
	kind    treeplan.Kind
	sources []treeplan.Span
	kids    []*pnode

	// level is the node's KB level, the entry-point at 1. It is what the
	// per-call depth check is asked about (treeplan.Verifier.CheckAnswer).
	level int
}

// New builds the descent over one corpus.
//
// It enumerates the question set immediately, because a corpus that yields no
// question is a corpus this stage cannot design a tree for, and learning that
// at stage setup beats learning it from an empty lane.
func New(d Design, lg log.Logger) (*Designer, error) {
	if d.Verifier == nil {
		return nil, errors.New("taxonomy: the descent has no tree-plan verifier to compose through")
	}
	if strings.TrimSpace(d.Unit) == "" {
		return nil, errors.New("taxonomy: the descent has no unit path to write its tree plan to")
	}
	if strings.TrimSpace(d.Title) == "" {
		return nil, errors.New("taxonomy: the descent has no corpus title for the entry-point")
	}
	if err := d.Params.Budgets.Validate(); err != nil {
		return nil, err
	}
	if d.Survey.Schema != survey.SchemaVersion {
		return nil, fmt.Errorf("taxonomy: survey declares schema %q, this build reads %q",
			d.Survey.Schema, survey.SchemaVersion)
	}
	def, err := prompt.ParseDefinition(stubDefinition)
	if err != nil {
		return nil, fmt.Errorf("taxonomy: parsing the taxonomy definition: %w", err)
	}

	root := enumerate(d.Survey, d.Title, d.Annexes)
	list := asks(root, d.Params.Budgets.CandidateCap)
	if len(list) == 0 {
		// One document with one section, or a corpus that is entirely
		// annexed: there is nothing to group, so there is no design to make.
		// Refusing here names the corpus; an empty lane would name nothing.
		return nil, errors.New("taxonomy: this corpus holds no container with more than one entry; " +
			"there is no grouping decision to make and therefore no tree to design")
	}

	t := &Designer{
		in:     d,
		def:    def,
		lg:     lg,
		asks:   list,
		byUnit: make(map[string]int, len(list)),
		root:   &pnode{title: d.Title, scope: d.Scope, kind: treeplan.KindEntryPoint, level: 1},
		attach: make(map[*cand]*pnode, len(list)),
	}
	t.attach[root] = t.root
	dir := unitDir(d.Unit)
	for i, a := range list {
		// The last container's call carries the composed artifact, so its name
		// IS the artifact's path — one name for the call and the thing the
		// call finally writes.
		a.unit = fmt.Sprintf("%s%04d%s", dir, i+1, containerSuffix)
		if i == len(list)-1 {
			a.unit = d.Unit
		}
		t.byUnit[a.unit] = i
	}
	return t, nil
}

// unitDir is the directory the artifact sits in, with its trailing slash — the
// place the container calls are named.
func unitDir(unit string) string {
	if i := strings.LastIndex(unit, "/"); i >= 0 {
		return unit[:i+1]
	}
	return ""
}

// Calls is how many container calls this descent will make. It is what a verb
// reports before it dials, and what a test asserts the enumeration against.
func (t *Designer) Calls() int { return len(t.asks) }

// StagePlan describes the stage for the coordinator: the ask every worker
// runs, the stage-constant context, and one serial lane of container calls
// ending in the one unit the stage owes.
//
// inputs are the named hashes the composed tree plan is stamped with — the
// corpus identity the caller knows — and upstreams the store artifacts it was
// derived from. The stage's OWN parameters are appended here rather than
// trusted to the caller, because the artifact means nothing without them: this
// run's tree plan and a re-plan's are the same path holding a tree designed
// under two different sets of caps.
func (t *Designer) StagePlan(name string, inputs []pipeline.Input, upstreams ...string) *pipeline.StagePlan {
	t.stage = name
	stamped := append(append([]pipeline.Input(nil), inputs...),
		pipeline.Input{Name: paramsInput, Hash: t.digest()})
	tasks := make([]pipeline.LaneTask, 0, len(t.asks))
	for i := range t.asks {
		task := pipeline.LaneTask{
			Owed:        pipeline.OwedArtifact{Path: t.asks[i].unit},
			Contributes: !t.last(i),
			Section:     name,
			SectionRef:  t.sectionRef(),
			Input:       func() (prompt.CallInput, bool) { return t.callInput(i) },
		}
		if t.last(i) {
			task.Owed.Inputs, task.Owed.Upstreams = stamped, upstreams
		}
		tasks = append(tasks, task)
	}
	return &pipeline.StagePlan{
		Name: name,
		Ask: pipeline.AskSpec{
			Def: t.def,
			// The heavy tier: taxonomy design is the serial, judgment-heavy
			// work §10 maps to the 31B, and the navigation surface it decides
			// is what quality binds hardest on.
			Tier:   config.TierHeavy,
			Effort: t.in.Effort,
			Retry:  t.in.Retry,
			Verify: t.verify,
			Encode: Encode,
			// No Fallback: this is a no-fallback seam (O-1). The absence IS
			// the classification (pipeline.AskSpec.Seam).
		},
		Spec: prompt.StageSpec{TaskDef: stubTaskDef},
		Lanes: func() ([]pipeline.SerialLane, error) {
			// One lane, so the containers are walked in order by one worker:
			// a container's groups attach to the node its parent's answer
			// created, and that ordering is the fold.
			return []pipeline.SerialLane{{Domain: name, Tasks: tasks}}, nil
		},
	}
}

// last reports whether call i is the one whose call writes the composed tree
// plan.
func (t *Designer) last(i int) bool { return i == len(t.asks)-1 }

// digest is the stage's parameter digest: a canonical rendering of everything
// outside the store that determined this stage's questions — the corpus
// identity, the budgets and caps the descent is bounded by, the effort and
// retry policy they were asked under, the entry-point's own labels, and the
// declared annex prefixes (§3.1). Changing any of them changes the tree, so any
// of them must invalidate the artifact.
//
// The retry declaration is in it because the declaration IDENTIFIES the asking:
// a tree designed where a rejected container was re-asked with reasoning is not
// the same artifact as one where it was re-asked without, and a stamp that did
// not say so would prove a run that never happened.
func (t *Designer) digest() string {
	b := t.in.Params.Budgets
	r := t.in.Retry
	var sb strings.Builder
	fmt.Fprintf(&sb, "corpus %s\ntitle %s\nscope %s\nthinking %t\n",
		t.in.Survey.Corpus.ContentHash, t.in.Title, t.in.Scope, t.in.Effort.Thinking)
	fmt.Fprintf(&sb, "retryThinking %t\nattempts %d\nretryNoteWords %d\n",
		r.Effort.Thinking, r.Attempts, r.NoteWords)
	fmt.Fprintf(&sb, "leafTokens %d\nsummaryInputTokens %d\nsummaryTokens %d\nentryPointTokens %d\n",
		b.LeafTokens, b.SummaryInputTokens, b.SummaryTokens, b.EntryPointTokens)
	fmt.Fprintf(&sb, "depthCap %d\nfanOutCap %d\ncandidateCap %d\ncalls %d\n",
		b.DepthCap, b.FanOutCap, b.CandidateCap, len(t.asks))
	for _, a := range t.in.Annexes {
		fmt.Fprintf(&sb, "annex %s\n", a.Prefix)
	}
	return pipeline.HashBytes([]byte(sb.String()))
}

// sectionRef is the stage reference buffer: what is stable across every call
// of this descent.
//
// The counts it renders are the corpus's and the descent's own, and the fold
// moves neither — the question set is fixed before the first call — so slot 4
// is stage-constant under the fold. That is not decoration: the churn tripwire
// aborts the worker if this text changes between calls of one lane.
func (t *Designer) sectionRef() string {
	return fmt.Sprintf("This corpus holds %d documents and %d sections. "+
		"The knowledge base has at most %d section levels below its entry point, "+
		"and at most %d entries under any one heading.",
		t.in.Survey.Corpus.Files, t.in.Survey.Corpus.Sections,
		t.in.Params.Budgets.DepthCap-1, t.in.Params.Budgets.FanOutCap)
}

// callInput is one container's per-call half of the prompt, built when the
// worker reaches it (pipeline.InputBuilder) — or the statement that this
// container needs no call at all.
//
// It has to be built here rather than when the stage was described: a
// container's candidate list is fixed, but the SCOPE its parent gave it is
// not — it is whatever the group its parent's answer put it in says, and that
// answer had not been given when this stage's work was described (§3.2).
//
// # An absorbed container spends nothing
//
// The same fact that supplies the scope also settles whether there is a
// question left. The question set is enumerated from the SOURCE tree before the
// first call, and a container whose parent's answer placed its material on a
// page has no node for its groups to attach to: the descent would throw the
// answer away. Since this is the moment that is known, the call is not made —
// zero tokens, and no way for a discarded container to fail the lane it sits
// in, which is what a call whose answer nobody reads must never be able to do.
//
// The one container that is asked anyway is the LAST, because its call is not
// only its container's: the composed tree plan is written by it (see
// StagePlan), and a task that owes an artifact may not skip. Its own answer is
// still discarded — verify drops it before parsing — so what survives here is
// one call per run at most, and it is the call the stage's whole output hangs
// off rather than a question about an absorbed container.
func (t *Designer) callInput(i int) (prompt.CallInput, bool) {
	if err := t.enter(); err != nil {
		return t.refuse(err)
	}
	defer t.leave()
	if i != t.next {
		return t.refuse(fmt.Errorf(
			"taxonomy: container %d was entered while the descent expects container %d; "+
				"the descent is one depth-first walk over one working proposal", i+1, t.next+1))
	}
	a := t.asks[i]
	if t.attach[a.c] == nil && !t.last(i) {
		t.discard(a, "skipped")
		t.next++
		return prompt.CallInput{}, false
	}

	status := []string{
		fmt.Sprintf("Container %d of %d", i+1, len(t.asks)),
		fmt.Sprintf("This %s is called %q.", a.c.kind, a.c.title),
	}
	if parent := t.attach[a.c]; parent != nil && parent.scope != "" {
		status = append(status, fmt.Sprintf("Its place in the knowledge base: %s", parent.scope))
	}
	if a.batches > 1 {
		status = append(status, fmt.Sprintf(
			"You are shown part %d of %d of what it holds; group only what is listed below.",
			a.batch, a.batches))
	}

	var content strings.Builder
	content.WriteString("Entries to group:\n")
	for n, c := range a.cands {
		content.WriteString(c.entry(n + 1))
		content.WriteString("\n")
	}
	return prompt.CallInput{
		StatusLines: status,
		Content:     content.String(),
		AcceptanceCriteria: []string{
			"- every numbered entry belongs to exactly one group",
			"- answer with the JSON object and nothing else",
		},
	}, true
}

// discard records a container the descent has no use for: the answer above it
// placed this material on a page, so there is no node its groups could attach
// to. spent says what the call cost — "skipped" where the question was dropped
// before it was asked, "spent" for the one container that is asked anyway
// because it carries the artifact (see callInput).
func (t *Designer) discard(a *ask, spent string) {
	t.dropped++
	t.lg.Info("container answer not used", "stage", t.stage, "unit", a.unit,
		"container", a.c.title, "call", spent,
		"reason", "a group above it placed this material on a page")
}

// verify parses one container's answer, holds it to §3.3's per-call
// post-conditions, and folds it into the working proposal.
//
// The classification is exhaustive by construction rather than by enumeration:
// a treeplan.Rejection is the only class the model's answer can be responsible
// for, so everything else — a defect, an unknown unit, a fold that was entered
// out of order — is ours and is wrapped as pipeline.ErrVerifierDefect. The
// default must be "blame ourselves": a defect routed to the model burns the
// retry and then fails the unit for the wrong reason, while a model failure
// routed to the defect path stops the job loudly.
//
// A rejection changes nothing. Only an accepted answer touches the proposal,
// so the single informed retry re-asks the same container against the same
// tree.
func (t *Designer) verify(artifactPath, response string) (any, error) {
	i, ok := t.byUnit[artifactPath]
	if !ok {
		return nil, t.defect(fmt.Errorf("taxonomy: %s is not a container of this descent", artifactPath))
	}
	if err := t.enter(); err != nil {
		return nil, t.defect(err)
	}
	defer t.leave()
	if err := t.latchedErr(); err != nil {
		return nil, t.defect(err)
	}
	if i != t.next {
		return nil, t.defect(fmt.Errorf(
			"taxonomy: container %d answered while the descent expects container %d", i+1, t.next+1))
	}
	a := t.asks[i]

	parent := t.attach[a.c]
	if parent == nil {
		// The answer above this container put its material on a page, so there
		// is no node for its groups to attach to. callInput normally drops the
		// question before it is asked; the last container is the exception —
		// its call writes the composed plan — so this is where that one answer
		// is discarded, before anything parses it. A discarded answer therefore
		// cannot be rejected, whatever the model said.
		t.discard(a, "spent")
		return t.advance(i, artifactPath)
	}

	ans, err := parseAnswer(response)
	if err != nil {
		return nil, t.reject(a, err)
	}
	if err := t.in.Verifier.CheckAnswer(ans, len(a.cands), parent.level+1); err != nil {
		if _, isRejection := treeplan.AsRejection(err); isRejection {
			return nil, t.reject(a, err)
		}
		return nil, t.defect(err)
	}
	t.apply(a, parent, ans)
	t.lg.Info("container designed", "stage", t.stage, "unit", artifactPath,
		"container", a.c.title, "entries", len(a.cands), "groups", len(ans.Groups))
	return t.advance(i, artifactPath)
}

// advance moves the fold on and, for the last container, composes the
// artifact. Every terminal outcome goes through it — an accepted answer and a
// discarded one alike — so the ordinal moves in exactly one place.
func (t *Designer) advance(i int, artifactPath string) (any, error) {
	t.next++
	if !t.last(i) {
		// The answer is in the fold's own state, so the task contributes and
		// the worker discards what it hands back.
		return nil, nil
	}
	return t.compose(artifactPath)
}

// apply folds one answer into the working proposal.
//
// A group flagged `section` becomes an index node, and its members' own
// material hangs below it: a member the descent asks about attaches its groups
// there when its turn comes, and a member it does not becomes a page named by
// the source's own heading.
//
// A group flagged `page` becomes a page. A group of one is the model's own
// title and scope; a group of several cannot be ONE page — a page is a single
// span of a single document (treeplan.SplitGroup) — so it becomes one page per
// member, each keeping the source's title and the group's scope. That is
// §2.7's dissolution applied where the members' own titles are still in hand,
// rather than after they have been reduced to a file name.
func (t *Designer) apply(a *ask, parent *pnode, ans treeplan.GroupingAnswer) {
	for _, g := range ans.Groups {
		members := make([]*cand, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, a.cands[m])
		}
		if g.Kind == treeplan.KindLeaf {
			if len(members) == 1 {
				parent.kids = append(parent.kids, page(g.Title, g.Scope, members[0], parent.level+1))
				t.pages++
				continue
			}
			for _, m := range members {
				// The model wrote ONE scope for what it thought was one page,
				// and these are several: rendered as-is, every one of a
				// dissolved group's down-links would carry the same line, which
				// is a routing surface that does not route. The source's own
				// gist is the per-page statement that exists, and the group's
				// scope stands where there is none.
				parent.kids = append(parent.kids, page(m.title, firstNonEmpty(m.gist, g.Scope), m, parent.level+1))
				t.pages++
			}
			continue
		}
		index := &pnode{title: g.Title, scope: g.Scope, kind: treeplan.KindIndex, level: parent.level + 1}
		parent.kids = append(parent.kids, index)
		t.groups++
		for _, m := range members {
			if len(m.kids) > 0 {
				t.attach[m] = index
				continue
			}
			index.kids = append(index.kids, page(m.title, m.scope(), m, index.level+1))
			t.pages++
		}
	}
}

// page is one leaf of the working proposal over a candidate's material.
func page(title, scope string, c *cand, level int) *pnode {
	return &pnode{title: title, scope: scope, kind: treeplan.KindLeaf, sources: c.spans, level: level}
}

// compose is the stage's output: the working proposal, run through the
// verifier that owns what a valid tree plan is.
//
// Nothing about the composition is this package's: §2.7's operators, §2.4's
// split expansion, the namer and every §3.3 post-condition live in
// internal/treeplan, and Compose runs Check over its own output. What this
// package does here is classify what comes back — a rejection the model could
// be responsible for, or a defect nobody's answer could have caused.
func (t *Designer) compose(artifactPath string) (any, error) {
	proposal := treeplan.TreeProposal{
		Title:    t.in.Title,
		Scope:    t.in.Scope,
		Children: proposalsOf(t.root.kids),
	}
	plan, err := t.in.Verifier.Compose(proposal, t.in.Annexes)
	if err != nil {
		if rej, ok := treeplan.AsRejection(err); ok {
			t.rejects++
			t.lg.Info("tree plan composed", "stage", t.stage, "unit", artifactPath,
				"verify", "failed", "reason", rej.Note())
			return nil, rejection{err: err, note: rej.Note()}
		}
		return nil, t.defect(err)
	}
	// The stage's cost, and — because resume is stage-granular — exactly what
	// an interruption anywhere in this stage re-spends.
	t.lg.Info("tree plan composed", "stage", t.stage, "unit", artifactPath,
		"containers", len(t.asks), "answers_unused", t.dropped, "rejections", t.rejects,
		"nodes", len(plan.Nodes), "groups", len(plan.Groups), "verify", "ok")
	t.composed = true
	return plan, nil
}

// proposalsOf converts the working tree into the proposal the verifier takes.
func proposalsOf(kids []*pnode) []treeplan.ProposalNode {
	out := make([]treeplan.ProposalNode, 0, len(kids))
	for _, k := range kids {
		out = append(out, treeplan.ProposalNode{
			Title:    k.title,
			Scope:    k.scope,
			Kind:     k.kind,
			Sources:  k.sources,
			Children: proposalsOf(k.kids),
		})
	}
	return out
}

// enter claims the fold for one step: the exclusivity the single serial lane
// promises, plus the two things that make a step legitimate at all.
//
// A Designer is ONE RUN's. The working proposal and the attach table are
// seeded at New and never reset, so a second descent over the same instance
// would append a second copy of every domain to a tree that already holds
// them. Composing the plan spends the Designer, and entering a spent one is a
// defect that names the remedy.
func (t *Designer) enter() error {
	if !t.busy.CompareAndSwap(false, true) {
		return errors.New("taxonomy: this descent is already in use; " +
			"it is one depth-first walk over one working proposal and the stage plans exactly one serial lane")
	}
	if t.composed {
		t.busy.Store(false)
		return errors.New("taxonomy: this descent's tree plan has already been composed; " +
			"construct a new Designer per run")
	}
	if !t.started {
		t.started = true
		t.lg.Info("taxonomy descent started", "stage", t.stage,
			"containers", len(t.asks), "documents", t.in.Survey.Corpus.Files)
	}
	return nil
}

func (t *Designer) leave() { t.busy.Store(false) }

// latch records a defect the input builder found; refuse hands the builder
// back the only thing it has left to say.
//
// pipeline.InputBuilder returns no error by design, so a refusal discovered
// there has nowhere to go at the moment it is found. It is latched instead,
// and verify raises it as a defect on the same container: the next thing this
// unit does, and a seam that can classify it.
func (t *Designer) latch(err error) {
	t.latchMu.Lock()
	defer t.latchMu.Unlock()
	if t.latched == nil {
		t.latched = err
	}
}

func (t *Designer) latchedErr() error {
	t.latchMu.Lock()
	defer t.latchMu.Unlock()
	return t.latched
}

// refuse hands the builder back the only thing it has left to say: an empty
// input, and NEEDED — the call still happens, so verify runs and raises the
// latched defect on this unit. Skipping instead would swallow the defect, since
// a skipped task never reaches a seam that could classify it.
func (t *Designer) refuse(err error) (prompt.CallInput, bool) {
	t.latch(err)
	t.lg.Error("the taxonomy descent refused to build a call", "stage", t.stage, "error", err)
	return prompt.CallInput{}, true
}

// reject records a rejected answer and returns it on its way to the model.
func (t *Designer) reject(a *ask, err error) error {
	t.rejects++
	note := err.Error()
	if rej, ok := treeplan.AsRejection(err); ok {
		note = rej.Note()
	}
	t.lg.Info("container answer rejected", "stage", t.stage, "unit", a.unit,
		"container", a.c.title, "reason", note)
	return rejection{err: err, note: note}
}

// defect hands the runner the classification this package is the only one that
// can make: no answer to the question we asked could land here, so it is not
// the model that is wrong. Neither remedy the runner has applies — a retry
// re-asks a question that was never the problem, and there is no fallback —
// so the worker aborts.
func (t *Designer) defect(err error) error {
	return fmt.Errorf("%w: %w", pipeline.ErrVerifierDefect, err)
}

// rejection is a treeplan.Rejection on its way to the MODEL.
//
// The runner renders whatever error a verifier returns into the corrective
// note the retry carries, so the note IS Error(). Rejection.Note is the
// rendering that belongs in a prompt — the mechanical fact, no path and no
// offset — and this type is what routes it there; errors.As still recovers the
// operator-facing value underneath.
type rejection struct {
	err  error
	note string
}

func (r rejection) Error() string { return r.note }
func (r rejection) Unwrap() error { return r.err }

// Encode renders the composed tree plan for the store.
//
// It is exported because there is more than one writer of this artifact: this
// stage composes one from a model descent, and the no-model dev build composes
// one from the source's own structure — one artifact, one encoder, whichever
// stage produced it.
func Encode(artifact any) ([]byte, error) {
	p, ok := artifact.(treeplan.TreePlan)
	if !ok {
		return nil, fmt.Errorf("taxonomy: artifact is %T, not a tree plan", artifact)
	}
	var buf bytes.Buffer
	if err := p.WriteJSON(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

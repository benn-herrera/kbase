// Package summarize is pipeline stage 6 (ARCHITECTURE.md §4 row 6;
// TEMP_DESIGN_V020_RESHAPE §6): the hierarchical summaries, bottom-up, as
// level-sliced stages — two per level (see below).
//
// # Why level-sliced stages and not one
//
// ARCHITECTURE §12 forbids a unit naming a sibling from its own stage — a
// stage's units run concurrently, so "earlier in the same stage" is not an
// ordering — and bottom-up summarisation is exactly that ordering. The
// resolution is to slice by tree level, deepest first: each level's units
// declare their upstreams in the PREVIOUS stage, which is a legal chain edge
// with real stamps, so a regenerated child invalidates its parent for free and
// no invalidation propagation code exists. The levels are statically declared
// because the stage list is fixed at job setup and the tree they are over is
// written by stage 3 of the same job: the chain runs to a generous ceiling
// (levelCeiling) rather than to the tree's own depth, which nothing caps
// (R-3). A level with no nodes resolves to zero lanes and the coordinator
// skips it, so the ceiling costs empty stages and nothing else — and a tree
// past it refuses rather than losing its deepest summaries (resolve).
//
// # What a call reads: summary-class material, and the leaf-group card
//
// Sibling leaves are ALWAYS summarised as a group, in isolation: one call over
// the direct leaves of one node, which is exactly the call an index whose
// children are all leaves already makes. Where the node holds nothing else that
// call's answer IS the node's summary and is delivered as one. Where the node
// ALSO holds index children, the group's answer is written as an ephemeral CARD
// — a store artifact, never a delivered page — and the node's own call then
// reads only summary-class material: the card, plus each index child's framing
// and conclusions, which its own verifier already held to the summary cap. So
// summary(node) = blend(group(direct leaves), summaries(index children)), and no
// leaf body is ever weighed against a capped summary inside one call. The
// alternative was what shipped until now: a heavy root-level page outweighing
// three whole domains ~2:1 in the entry-point's own call, which is a function of
// corpus layout rather than of anything a definition could say [MAD2: B-5].
//
// Three consequences are ruled and lived with rather than repaired. INSIDE the
// group call a copious leaf outweighs a terse sibling — size is signal there,
// and the pages are the same kind of thing. A node with one direct leaf makes a
// group of ONE. And parity at the parent is per SHELF rather than per page: one
// card speaks for N pages beside one summary per index child.
//
// A mixed node's card is its own second call, so it cannot be its sibling: §12
// forbids a unit naming an upstream from its own stage, whose units run
// concurrently. Each level therefore declares TWO stages — the level's cards,
// then the level's summaries — and a level with no mixed node resolves the first
// to zero lanes exactly as an empty level resolves both.
//
// The whole input of either call is bounded by G-2, which the tree plan proved
// before this stage ran (treeplan's digest check, which bounds the two calls
// separately and the node by the larger) — so an over-budget call here is our
// arithmetic being wrong, not a runtime condition (I-6).
//
// # A no-fallback seam
//
// A mechanically renderable index — up-link, title, down-links with the tree
// plan's scope lines — is a valid node, so a fallback is sitting right there.
// O-1 ruled it out anyway: a tree whose every index lost its conclusions block
// is a materially different artifact delivered under a quiet "degraded" count.
// `AskSpec.Fallback` is nil, a unit that fails twice fails, and the job refuses
// emission.
package summarize

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"kbase/internal/config"
	"kbase/internal/log"
	"kbase/internal/model"
	"kbase/internal/pipeline"
	"kbase/internal/prompt"
	"kbase/internal/treeplan"
)

const (
	// paramsInput names the parameter digest every summary is stamped with.
	// The colon namespaces it the way pipeline's own upstream inputs are.
	paramsInput = "summarize:parameters"

	// unitSuffix is what a node's summary artifact is called: the node's own
	// path plus this. One name derivation, so `summaries/main/index.md.json`
	// is the summary of `main/index.md` by construction and not by convention.
	unitSuffix = ".json"

	// levelStageSuffix names one level's stage: `summaries.L4`. The number is
	// the KB level the stage summarises, so the stage list reads in the order
	// it runs — deepest first.
	levelStageSuffix = ".L"

	// leavesName is the one word the leaf-group card and the stage that writes
	// it are both named after: `summaries.L4.leaves` owes
	// `summaries/main/index.md.leaves.json`. Named once, so a card and the stage
	// that owes it are one thing in the store, in the log and in a failure.
	//
	// The card sits BESIDE the node's summary rather than at it, which is what
	// keeps it out of the delivered tree by construction: Read and All look for
	// `<node path>.json` and can never find `<node path>.leaves.json`, so no
	// render path has one to reach for.
	leavesName = ".leaves"
	cardSuffix = leavesName + unitSuffix

	// levelCeiling is how many KB levels this stage declares stages for
	// (ARCHITECTURE.md §9). It is NOT a cap on the tree: depth follows the
	// nesting the source graph requires (R-3, ruled 2026-08-17), and nothing
	// refuses a deep corpus.
	//
	// It exists because the stage LIST is fixed at job setup and the tree plan
	// is written by stage 3, in the same job: at the moment the chain is
	// described there is no artifact to count levels off. A level with no nodes
	// resolves to zero lanes, so declaring more levels than a corpus has costs
	// two empty stages each and nothing else — which is what makes a generous
	// ceiling the cheap side of the trade.
	//
	// Sixteen is generous against the deepest shape anything can present: the
	// entry-point, six levels of heading nesting (Markdown's own limit, and
	// headings are the descent's witness of the source graph), and the folder
	// and grouping levels a model may interpose above them. A tree that
	// somehow exceeds it does not lose its deepest summaries quietly —
	// resolve refuses the job and names the number.
	levelCeiling = 16

	// cardTitle is the heading the card is presented under in a mixed node's own
	// call. It names a SHELF and not a page, because that is what the card is:
	// one voice for every page directly under this node (the per-shelf parity
	// this scheme owns). The pages' own titles and scopes are in the routing
	// context beside it, where they always were.
	cardTitle = "The pages of this section"
)

// ArtifactReader is the one thing this stage needs from the store: the bytes
// of an artifact an earlier stage proved. It is declared here, at the point of
// use, so the package depends on a method and not on the store.
type ArtifactReader interface {
	Get(rel string) ([]byte, error)
}

// Job is what one run's summaries are over.
type Job struct {
	// TreePlan reads the composed tree plan. It is a function rather than a
	// value because stage 3 produces it in the same job: at the moment the
	// stages are DESCRIBED it does not exist yet, and by the moment a level's
	// lanes are resolved it does (the lazy chain).
	TreePlan func() (treeplan.TreePlan, error)

	// Store reads the leaf artifacts and the level below's summaries.
	Store ArtifactReader

	// LeavesDir and SummariesDir are the store prefixes those two artifact
	// families live at. They are the composing verb's plan table (§10), not
	// this package's business.
	LeavesDir    string
	SummariesDir string

	// Params is the operating point: the estimator the caps are measured with
	// and the budgets they are (SummaryTokens, SummaryInputTokens).
	Params treeplan.Params

	// Effort is the definition's declared ask, threaded to the wire.
	Effort model.RequestEffort

	// Retry is the definition's declared retry policy
	// (pipeline.RetryPolicy), threaded from the same place for the same
	// reason.
	Retry pipeline.RetryPolicy
}

// Summarizer describes and verifies the level stages of one job.
//
// One is built per job and shared by every worker of every level: the ask is
// stage-constant (pipeline.AskSpec) and so is everything here. The only mutable
// state is the memoised tree plan and the per-unit latch, both guarded — unlike
// the stage-3 and stage-4 folds, this stage's units are independent, so its
// lanes fan out across workers and nothing accumulates.
type Summarizer struct {
	job Job
	def prompt.Definition
	lg  log.Logger

	mu   sync.Mutex
	plan *treeplan.TreePlan
	// units is every node's summary call, by artifact path; cards is the
	// leaf-group call of every MIXED node, by the path of the card it writes.
	// Two tables rather than one, because the path is what says which of a
	// node's two calls a response is answering.
	units map[string]*unit
	cards map[string]*unit
	paths []string
	// latched holds a per-unit refusal the input builder had no way to return
	// (pipeline.InputBuilder yields no error), raised by verify as a defect on
	// the same unit. Per unit and not per stage: this stage's units run
	// concurrently, so one shared slot would attribute one lane's refusal to
	// another lane's call.
	latched map[string]error
}

// unit is one summary's whole description: the node it is about, its children
// in tree-plan order, and the level it sits at.
type unit struct {
	node   treeplan.Node
	kids   []treeplan.Node
	level  int
	domain string
}

// leaves and sections are the node's direct children of one kind, in tree-plan
// order: the pages the group call is over, and the subsections whose summaries
// the node's own call reads.
func (u *unit) leaves() []treeplan.Node { return u.ofKind(true) }

func (u *unit) sections() []treeplan.Node { return u.ofKind(false) }

func (u *unit) ofKind(leaf bool) []treeplan.Node {
	var out []treeplan.Node
	for _, c := range u.kids {
		if (c.Kind == treeplan.KindLeaf) == leaf {
			out = append(out, c)
		}
	}
	return out
}

// mixed reports whether this node holds children of BOTH kinds — the one case
// that costs two calls, because it is the only one where a group of leaves would
// otherwise share a call with a subsection's summary.
func (u *unit) mixed() bool { return len(u.leaves()) > 0 && len(u.sections()) > 0 }

// call is one model call of this stage: the node it is about, and whether it is
// the node's leaf-group call rather than its summary.
//
// A node has one call or two, and the difference between them is entirely in
// what they read — same definition, same tier, same declared effort and retry.
type call struct {
	u    *unit
	card bool
}

// kids are the children this call is ABOUT: its routing context and its status
// counts. A group call is about the node's direct pages ALONE — in isolation,
// which is what makes it the same call a leaves-only container already makes —
// and a summary call is about all of them.
func (c call) kids() []treeplan.Node {
	if c.card {
		return c.u.leaves()
	}
	return c.u.kids
}

// entry is one thing a call reads: the heading it is presented under, the store
// artifact it comes from, and whether that artifact is a page.
//
// The entry list is also the UPSTREAM list — what a call reads and the chain
// edge it stands on are the same set, derived once (see entries), so a call
// cannot come to read something it did not declare.
type entry struct {
	title string
	path  string
	// page says how the artifact is read: a leaf page contributes its body,
	// anything else its framing and conclusions.
	page bool
}

// New returns the summarizer for one job.
func New(j Job, lg log.Logger) (*Summarizer, error) {
	if j.TreePlan == nil {
		return nil, errors.New("summarize: no tree plan reader; the summaries are over a tree nobody supplied")
	}
	if j.Store == nil {
		return nil, errors.New("summarize: no artifact reader; a summary reads its children's artifacts")
	}
	if j.LeavesDir == "" || j.SummariesDir == "" {
		return nil, errors.New("summarize: the store prefixes for pages and summaries are the caller's to name")
	}
	if err := j.Params.Budgets.Validate(); err != nil {
		return nil, err
	}
	def, err := prompt.ParseDefinition(stubDefinition)
	if err != nil {
		return nil, fmt.Errorf("summarize: parsing the summary definition: %w", err)
	}
	return &Summarizer{job: j, def: def, lg: lg, latched: map[string]error{}}, nil
}

// Unit is the store path of one node's summary artifact.
func (s *Summarizer) Unit(nodePath string) string {
	return s.job.SummariesDir + "/" + nodePath + unitSuffix
}

// Card is the store path of one node's leaf-group card. Only a MIXED node has
// one: everywhere else the group call's answer is the node's own summary and is
// written at Unit.
func (s *Summarizer) Card(nodePath string) string {
	return s.job.SummariesDir + "/" + nodePath + cardSuffix
}

// Cards is how many leaf-group cards this tree costs: one per mixed node, and
// the whole price of the B-5 scheme (ARCHITECTURE §4 row 6). It is the run
// record's `leafGroupCards`, on the same argument that put `taxonomyCalls`
// there — a scheme whose cost is up to one heavy call per mixed node should
// not leave a live run unable to say how many it paid [ARCH F8].
//
// Zero before the plan resolves and the enumerated count after, exactly like
// taxonomy's: the caller reads it once the run is over.
func (s *Summarizer) Cards() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cards)
}

// StagePlans describes the level stages, deepest first.
//
// There are TWO stages per level, always: the level's leaf-group cards, then
// the level's summaries. A level with no nodes resolves both to zero lanes and
// a level with no mixed node resolves the first one to zero, which is what lets
// the stage list be static while the tree is not.
//
// The chain runs to levelCeiling and no longer reads a depth budget: there is
// none (R-3, ruled 2026-08-17), and a chain sized by the old cap of four
// stopped at level 4 — so a corpus whose descent reached level 5 would have had
// its deepest summaries silently never written, and every index above them
// blended over children that had none. The ceiling is generous and the tree's
// own depth is checked against it where the plan is read (resolve), so a corpus
// past it refuses loudly instead.
//
// The order is the whole reason the card is a stage of its own: a mixed node's
// summary call reads the card, and a unit may not name an upstream from its own
// stage (§12) because a stage's units run concurrently.
//
// owed builds each unit's description from its path and the artifacts it read;
// the stage appends its own parameter digest, because the artifact means
// nothing without it and the caller cannot derive it.
func (s *Summarizer) StagePlans(prefix string, owed func(unit string, upstreams []string) pipeline.OwedArtifact) []*pipeline.StagePlan {
	var out []*pipeline.StagePlan
	for level := levelCeiling; level >= 1; level-- {
		name := fmt.Sprintf("%s%s%d", prefix, levelStageSuffix, level)
		out = append(out,
			s.levelStage(name+leavesName, level, true, owed),
			s.levelStage(name, level, false, owed))
	}
	return out
}

// levelStage is one stage of one level: the same ask either way, over the calls
// cards selects (see calls).
func (s *Summarizer) levelStage(name string, level int, cards bool, owed func(string, []string) pipeline.OwedArtifact) *pipeline.StagePlan {
	return &pipeline.StagePlan{
		Name: name,
		Ask: pipeline.AskSpec{
			Def: s.def,
			// The heavy tier: summaries are the navigation surface, and §10
			// puts quality-binding judgement work on the 31B.
			Tier:   config.TierHeavy,
			Effort: s.job.Effort,
			Retry:  s.job.Retry,
			Verify: s.verify,
			Encode: Encode,
			// No Fallback: no-fallback seam (O-1).
		},
		Spec: prompt.StageSpec{
			TaskDef: stubTaskDef,
			Est:     s.job.Params.Est,
			// G-2, enforced on the call rather than only proved on the tree
			// plan. The children's material is the content slot and nothing
			// else is, so this is the guarantee stage 3 made, asserted where
			// the bytes exist. A breach is refuse-and-split (§3), inventoried
			// against the node whose fan-out is wrong.
			Budgets: prompt.Budgets{
				PerSlot: map[prompt.Slot]int{prompt.SlotContent: s.job.Params.Budgets.SummaryInputTokens},
			},
		},
		Lanes: func() ([]pipeline.SerialLane, error) { return s.lanes(name, level, cards, owed) },
	}
}

// lanes describes one stage's work: one task per call of the level, in
// per-domain serial lanes so the level fans out across workers.
func (s *Summarizer) lanes(stage string, level int, cards bool, owed func(string, []string) pipeline.OwedArtifact) ([]pipeline.SerialLane, error) {
	if _, err := s.resolve(); err != nil {
		return nil, err
	}
	var (
		order []string
		tasks = map[string][]pipeline.LaneTask{}
	)
	for _, c := range s.calls(level, cards) {
		if _, seen := tasks[c.u.domain]; !seen {
			order = append(order, c.u.domain)
		}
		unitPath := s.path(c)
		o := owed(unitPath, s.upstreams(c))
		o.Inputs = slices.Concat(o.Inputs, []pipeline.Input{{Name: paramsInput, Hash: s.digest()}})
		tasks[c.u.domain] = append(tasks[c.u.domain], pipeline.LaneTask{
			Owed:       o,
			Section:    c.u.domain,
			SectionRef: s.sectionRef(level),
			Input:      func() (prompt.CallInput, bool) { return s.callInput(unitPath) },
		})
	}
	if len(order) == 0 {
		s.lg.Info("no sections at this level", "stage", stage, "level", level)
		return nil, nil
	}
	lanes := make([]pipeline.SerialLane, 0, len(order))
	for _, dom := range order {
		lanes = append(lanes, pipeline.SerialLane{Domain: stage + "/" + dom, Tasks: tasks[dom]})
	}
	return lanes, nil
}

// calls is one stage's calls, in tree-plan order: every node at the level for a
// summary stage, and only the MIXED ones for a leaf-group stage — everywhere
// else the group call and the summary call are one call, written at Unit.
func (s *Summarizer) calls(level int, cards bool) []call {
	var out []call
	for _, u := range s.level(level) {
		if cards && !u.mixed() {
			continue
		}
		out = append(out, call{u: u, card: cards})
	}
	return out
}

// path is the artifact one call owes.
func (s *Summarizer) path(c call) string {
	if c.card {
		return s.Card(c.u.node.Path)
	}
	return s.Unit(c.u.node.Path)
}

// entries is what one call reads, in the order it is presented.
//
// A group call reads the pages, and so does a leaves-only node's summary call:
// they are the same call, which is the point (§4 row 6). A mixed node's summary
// call reads the card in the pages' place, first, because the pages directly
// under a node are its own material and its subsections are what sits below
// them. An index child is read as its own capped framing and conclusions
// wherever it appears.
func (s *Summarizer) entries(c call) []entry {
	var out []entry
	if c.card || !c.u.mixed() {
		for _, k := range c.u.leaves() {
			out = append(out, entry{title: k.Title, path: s.job.LeavesDir + "/" + k.Path, page: true})
		}
	}
	if c.card {
		return out
	}
	if c.u.mixed() {
		out = append(out, entry{title: cardTitle, path: s.Card(c.u.node.Path)})
	}
	for _, k := range c.u.sections() {
		out = append(out, entry{title: k.Title, path: s.Unit(k.Path)})
	}
	return out
}

// upstreams is the chain edge one call stands on: the artifact of everything it
// reads, which is exactly its entry list. A leaf page for a group call, and for
// a mixed node's summary call the card plus each index child's summary — never a
// page, which is what makes the input summary-class throughout.
func (s *Summarizer) upstreams(c call) []string {
	es := s.entries(c)
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.path)
	}
	return out
}

// resolve reads the tree plan once and derives every unit from it.
//
// It is memoised because every level stage asks for it and every one of them
// must see the same tree: the plan is an upstream artifact, and a second read
// after something changed it would describe a level of a tree the level below
// was not summarising.
func (s *Summarizer) resolve() (treeplan.TreePlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.plan != nil {
		return *s.plan, nil
	}
	plan, err := s.job.TreePlan()
	if err != nil {
		return treeplan.TreePlan{}, err
	}
	if plan.Schema != treeplan.SchemaVersion {
		return treeplan.TreePlan{}, fmt.Errorf("summarize: tree plan declares schema %q, this build reads %q",
			plan.Schema, treeplan.SchemaVersion)
	}
	// The static stage chain against the tree it turned out to be over. Nothing
	// caps a tree's depth, but this stage can only summarise the levels it
	// declared stages for, and a level with no stage is a level whose summaries
	// are never written — which the tree above it would then blend over as an
	// absence rather than a failure. Refusing here is what keeps a ceiling from
	// becoming a silent truncation.
	if depth := plan.MaxDepth(); depth > levelCeiling {
		return treeplan.TreePlan{}, fmt.Errorf(
			"summarize: the tree plan is %d levels deep and this build declares summary stages for %d; "+
				"raise the level ceiling (ARCHITECTURE.md §9) rather than delivering a tree whose deepest sections have no summary",
			depth, levelCeiling)
	}

	levels := map[string]int{}
	domains := map[string]string{}
	units := map[string]*unit{}
	cards := map[string]*unit{}
	paths := make([]string, 0, len(plan.Nodes))
	byPath := map[string]*unit{}
	for _, n := range plan.Nodes {
		paths = append(paths, n.Path)
		switch {
		case n.Parent == "":
			levels[n.Path], domains[n.Path] = 1, n.Path
		default:
			levels[n.Path] = levels[n.Parent] + 1
			domains[n.Path] = domains[n.Parent]
			if levels[n.Parent] == 1 {
				// A child of the entry-point IS a domain: the top-level
				// subtree one worker owns (§6.2).
				domains[n.Path] = n.Path
			}
			if parent, ok := byPath[n.Parent]; ok {
				parent.kids = append(parent.kids, n)
			}
		}
		if n.Kind == treeplan.KindLeaf {
			continue
		}
		u := &unit{node: n, level: levels[n.Path], domain: domains[n.Path]}
		byPath[n.Path] = u
		units[s.Unit(n.Path)] = u
	}
	// The card table is filled after the walk, not inside it: whether a node is
	// mixed is a fact about its children, and a node's children are not all known
	// until every node has been seen (the plan lists a parent before its
	// children, so the last of them can be the last node).
	for _, u := range units {
		if u.mixed() {
			cards[s.Card(u.node.Path)] = u
		}
	}
	s.plan, s.units, s.cards, s.paths = &plan, units, cards, paths
	return plan, nil
}

// level is the units at one KB level, in tree-plan order.
func (s *Summarizer) level(level int) []*unit {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*unit
	for _, n := range s.plan.Nodes {
		if u, ok := s.units[s.Unit(n.Path)]; ok && u.level == level {
			out = append(out, u)
		}
	}
	return out
}

// callOf is which of the stage's calls an artifact path names. The path is the
// whole of the routing: a node's summary sits at Unit and its card at Card, so
// one lookup says both which node a response is about and which of its two calls
// answered.
func (s *Summarizer) callOf(path string) (call, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.units[path]; ok {
		return call{u: u}, true
	}
	u, ok := s.cards[path]
	return call{u: u, card: true}, ok
}

// digest is the stage's parameter digest: what outside the store determined
// this summary — the caps its verifier holds it to, and the effort and retry
// policy it was asked under. The tree plan and the children's artifacts are
// upstreams and carry their own stamps, so they are not restated here.
//
// The retry declaration is in it because the declaration identifies the asking:
// a summary a rejected call re-wrote with reasoning is not the artifact a
// re-ask without it would have produced.
func (s *Summarizer) digest() string {
	b := s.job.Params.Budgets
	r := s.job.Retry
	return pipeline.HashBytes([]byte(fmt.Sprintf(
		"summaryTokens %d\nsummaryInputTokens %d\nthinking %t\n"+
			"retryThinking %t\nattempts %d\nretryNoteWords %d\n",
		b.SummaryTokens, b.SummaryInputTokens, s.job.Effort.Thinking,
		r.Effort.Thinking, r.Attempts, r.NoteWords)))
}

// sectionRef is the stage reference buffer: stable across every call of a
// lane, which the churn tripwire asserts on every call after the first.
func (s *Summarizer) sectionRef(level int) string {
	return fmt.Sprintf(
		"You are writing at level %d of the knowledge base. Everything below this section has "+
			"already been written: what you are given is what a reader finds there.", level)
}

// callInput assembles one call: the material in the content buffer, and the
// titles and scope lines of what the call is about beside it as the routing
// context (§6.3).
//
// One builder for both of a node's calls, because they are one call shape: the
// entry list says what is read and the child list says what the call is about,
// and for every node but a mixed one those are the same children.
//
// Every call of this stage is needed (pipeline.InputBuilder). A summary's
// question is settled by the tree plan before the level's lanes are resolved,
// so there is nothing this stage can learn between describing a unit and
// reaching it that would make its call pointless.
func (s *Summarizer) callInput(unitPath string) (prompt.CallInput, bool) {
	c, ok := s.callOf(unitPath)
	if !ok {
		return s.refuse(unitPath, fmt.Errorf("summarize: %s is not a summary of this job", unitPath))
	}
	u := c.u
	var content strings.Builder
	for _, e := range s.entries(c) {
		material, err := s.material(e)
		if err != nil {
			return s.refuse(unitPath, err)
		}
		fmt.Fprintf(&content, "## %s\n\n%s\n\n", e.title, material)
	}
	var routing strings.Builder
	pages, sections := 0, 0
	for _, k := range c.kids() {
		fmt.Fprintf(&routing, "- %s — %s\n", k.Title, k.Scope)
		if k.Kind == treeplan.KindLeaf {
			pages++
			continue
		}
		sections++
	}

	subject := fmt.Sprintf("This section is called %q.", u.node.Title)
	if u.node.Kind == treeplan.KindEntryPoint {
		subject = fmt.Sprintf("This is the whole knowledge base, called %q.", u.node.Title)
	}
	status := []string{subject}
	if u.node.Scope != "" {
		status = append(status, fmt.Sprintf("Its place in the knowledge base: %s", u.node.Scope))
	}
	status = append(status, fmt.Sprintf("It holds %d pages and %d sections, below.", pages, sections))

	return prompt.CallInput{
		StatusLines: status,
		Content:     strings.TrimSpace(content.String()),
		CallRef:     "What a reader finds under this section:\n" + routing.String(),
		AcceptanceCriteria: []string{
			// The labels are named LITERALLY here, not described. This line is
			// slot 8 — the recency position, and the one part of the prompt an
			// escalated retry is still reading after reasoning: a criterion
			// that says "the three labelled blocks" leaves the model to
			// remember what they were, and a summary written as bare prose is
			// the failure that costs the unit.
			"- write " + labelFraming + " " + labelHeading + " " + labelConclusions + " blocks",
			"- no links, no file names of this knowledge base",
		},
	}, true
}

// material is what one entry contributes to the call that reads it.
//
// A page contributes its BODY — the artifact stage 5 proved, read back rather
// than re-sliced, because conclusions come from bodies and a leaf that has been
// proven once should never be derived twice. Everything else is a summary and
// contributes its framing and conclusions, which its own verifier already held
// to the summary cap — an index child's summary and a leaf-group card alike,
// rendered by one reader because the parent is meant to weigh them as one kind
// of thing.
func (s *Summarizer) material(e entry) (string, error) {
	data, err := s.job.Store.Get(e.path)
	if err != nil {
		return "", fmt.Errorf("summarize: reading %s: %w", e.path, err)
	}
	if e.page {
		return strings.TrimSpace(string(data)), nil
	}
	sum, err := ReadJSON(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(sum.Framing)
	if sum.Conclusions != "" {
		fmt.Fprintf(&b, "\n\n### %s\n\n%s", sum.ConclusionsHeading, sum.Conclusions)
	}
	return strings.TrimSpace(b.String()), nil
}

// verify parses one answer and holds it to §6.4's post-conditions.
//
// Every check is deterministic and cheap, and none of them is about quality:
// semantic judgement is stage 7's job by design, and a heavy-tier judgement is
// not re-litigated by a deterministic checker.
// Both of a node's calls are held to the same post-conditions, the summary cap
// included: the cap is what the level above pays for one shelf, so a card that
// is not a summary-sized thing would break the parent's arithmetic exactly as an
// over-long summary would.
func (s *Summarizer) verify(artifactPath, response string) (any, error) {
	c, ok := s.callOf(artifactPath)
	if !ok {
		return nil, s.defect(fmt.Errorf("summarize: %s is not a summary of this job", artifactPath))
	}
	if err := s.latchedErr(artifactPath); err != nil {
		return nil, s.defect(err)
	}
	sum, err := parseSummary(response)
	if err != nil {
		return nil, s.reject(artifactPath, err)
	}
	if err := s.check(c.u, sum); err != nil {
		return nil, s.reject(artifactPath, err)
	}
	sum.Unit, sum.Schema = artifactPath, SchemaVersion
	s.lg.Info("summary written", "stage", "summaries", "unit", artifactPath,
		"level", c.u.level, "entries", len(c.kids()), "card", c.card,
		"framing_tokens", s.job.Params.Est.Estimate(sum.Framing),
		"conclusions_tokens", s.job.Params.Est.Estimate(sum.Conclusions))
	return sum, nil
}

func (s *Summarizer) reject(unitPath string, err error) error {
	s.lg.Info("summary rejected", "stage", "summaries", "unit", unitPath, "reason", err)
	return err
}

// defect hands the runner the classification this package is the only one that
// can make: no answer could be wrong in this way, so the derivation is.
func (s *Summarizer) defect(err error) error {
	return fmt.Errorf("%w: %w", pipeline.ErrVerifierDefect, err)
}

// refuse latches a defect the builder found and hands back an empty input that
// is still NEEDED: the call happens, verify runs, and the latch is raised there
// as a defect on this unit. A skip would swallow it — a task that makes no call
// never reaches a seam that could classify anything.
func (s *Summarizer) refuse(unitPath string, err error) (prompt.CallInput, bool) {
	s.mu.Lock()
	if _, seen := s.latched[unitPath]; !seen {
		s.latched[unitPath] = err
	}
	s.mu.Unlock()
	s.lg.Error("the summary stage refused to build a call", "unit", unitPath, "error", err)
	return prompt.CallInput{}, true
}

func (s *Summarizer) latchedErr(unitPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latched[unitPath]
}

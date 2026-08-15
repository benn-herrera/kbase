// Package summarize is pipeline stage 6 (ARCHITECTURE.md §4 row 6;
// TEMP_DESIGN_V020_RESHAPE §6): the hierarchical summaries, bottom-up, as four
// level-sliced stages.
//
// # Why four stages and not one
//
// ARCHITECTURE §12 forbids a unit naming a sibling from its own stage — a
// stage's units run concurrently, so "earlier in the same stage" is not an
// ordering — and bottom-up summarisation is exactly that ordering. The
// resolution is to slice by tree level, deepest first: each level's units
// declare their upstreams in the PREVIOUS stage, which is a legal chain edge
// with real stamps, so a regenerated child invalidates its parent for free and
// no invalidation propagation code exists. The levels are statically declared
// because the depth cap is mechanical (I-9); a level with no nodes resolves to
// zero lanes and the coordinator skips it.
//
// # What a call reads: per child KIND, not per level
//
// "An index whose children are leaves" is a property of a NODE, not of a level
// (F-10). So for any index at any level: leaf children are read as BODIES (the
// leaves/* artifacts — where conclusions actually come from), index children as
// their own `framing` + `conclusions` capped at the summary cap, and a mixed
// index reads both. The whole input is bounded by G-2, which the tree plan
// proved before this stage ran (treeplan's digest check) — so an over-budget
// call here is our arithmetic being wrong, not a runtime condition (I-6).
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
	// and the budgets they are (SummaryTokens, SummaryInputTokens, DepthCap).
	Params treeplan.Params

	// Effort is the definition's declared ask, threaded to the wire.
	Effort model.RequestEffort

	// Retry is the definition's declared retry policy
	// (pipeline.RetryPolicy), threaded from the same place for the same
	// reason.
	Retry pipeline.RetryPolicy
}

// Summarizer describes and verifies the four level stages of one job.
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

	mu    sync.Mutex
	plan  *treeplan.TreePlan
	units map[string]*unit
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

// StagePlans describes the level stages, deepest first.
//
// There is one stage per index level the depth cap allows, always — a level
// with no nodes resolves to zero lanes and the coordinator skips it, which is
// what lets the stage list be static while the tree is not.
//
// owed builds each unit's description from its path and the artifacts it read;
// the stage appends its own parameter digest, because the artifact means
// nothing without it and the caller cannot derive it.
func (s *Summarizer) StagePlans(prefix string, owed func(unit string, upstreams []string) pipeline.OwedArtifact) []*pipeline.StagePlan {
	var out []*pipeline.StagePlan
	for level := s.job.Params.Budgets.DepthCap; level >= 1; level-- {
		out = append(out, s.levelStage(fmt.Sprintf("%s%s%d", prefix, levelStageSuffix, level), level, owed))
	}
	return out
}

func (s *Summarizer) levelStage(name string, level int, owed func(string, []string) pipeline.OwedArtifact) *pipeline.StagePlan {
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
		Lanes: func() ([]pipeline.SerialLane, error) { return s.lanes(name, level, owed) },
	}
}

// lanes describes one level's work: one unit per index node at that level, in
// per-domain serial lanes so the level fans out across workers.
func (s *Summarizer) lanes(stage string, level int, owed func(string, []string) pipeline.OwedArtifact) ([]pipeline.SerialLane, error) {
	if _, err := s.resolve(); err != nil {
		return nil, err
	}
	var (
		order []string
		tasks = map[string][]pipeline.LaneTask{}
	)
	for _, u := range s.level(level) {
		if _, seen := tasks[u.domain]; !seen {
			order = append(order, u.domain)
		}
		unitPath := s.Unit(u.node.Path)
		o := owed(unitPath, s.upstreams(u))
		o.Inputs = slices.Concat(o.Inputs, []pipeline.Input{{Name: paramsInput, Hash: s.digest()}})
		tasks[u.domain] = append(tasks[u.domain], pipeline.LaneTask{
			Owed:       o,
			Section:    u.domain,
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

// upstreams is the chain edge this unit stands on: the artifact of every child
// it reads. A leaf child is its page; an index child is its own summary, which
// the previous level's stage wrote.
func (s *Summarizer) upstreams(u *unit) []string {
	out := make([]string, 0, len(u.kids))
	for _, c := range u.kids {
		if c.Kind == treeplan.KindLeaf {
			out = append(out, s.job.LeavesDir+"/"+c.Path)
			continue
		}
		out = append(out, s.Unit(c.Path))
	}
	return out
}

// resolve reads the tree plan once and derives every unit from it.
//
// It is memoised because four level stages ask for it and every one of them
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

	levels := map[string]int{}
	domains := map[string]string{}
	units := map[string]*unit{}
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
	s.plan, s.units, s.paths = &plan, units, paths
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

func (s *Summarizer) unitOf(path string) (*unit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.units[path]
	return u, ok
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
		"summaryTokens %d\nsummaryInputTokens %d\ndepthCap %d\nthinking %t\n"+
			"retryThinking %t\nattempts %d\nretryNoteWords %d\n",
		b.SummaryTokens, b.SummaryInputTokens, b.DepthCap, s.job.Effort.Thinking,
		r.Effort.Thinking, r.Attempts, r.NoteWords)))
}

// sectionRef is the stage reference buffer: stable across every call of a
// lane, which the churn tripwire asserts on every call after the first.
func (s *Summarizer) sectionRef(level int) string {
	return fmt.Sprintf(
		"You are writing at level %d of the knowledge base. Everything below this section has "+
			"already been written: what you are given is what a reader finds there.", level)
}

// callInput assembles one summary's call: the children's own material in the
// content buffer, their titles and scope lines beside it as the routing
// context (§6.3).
//
// Every call of this stage is needed (pipeline.InputBuilder). A summary's
// question is settled by the tree plan before the level's lanes are resolved,
// so there is nothing this stage can learn between describing a unit and
// reaching it that would make its call pointless.
func (s *Summarizer) callInput(unitPath string) (prompt.CallInput, bool) {
	u, ok := s.unitOf(unitPath)
	if !ok {
		return s.refuse(unitPath, fmt.Errorf("summarize: %s is not a summary of this job", unitPath))
	}
	var content, routing strings.Builder
	pages, sections := 0, 0
	for _, c := range u.kids {
		fmt.Fprintf(&routing, "- %s — %s\n", c.Title, c.Scope)
		material, err := s.material(c)
		if err != nil {
			return s.refuse(unitPath, err)
		}
		fmt.Fprintf(&content, "## %s\n\n%s\n\n", c.Title, material)
		if c.Kind == treeplan.KindLeaf {
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
			"- answer with the JSON object and nothing else",
			"- no links, no file names of this knowledge base",
		},
	}, true
}

// material is what one child contributes to its parent's call (F-10).
//
// A leaf child contributes its BODY — the page artifact stage 5 proved, read
// back rather than re-sliced, because conclusions come from bodies and a leaf
// that has been proven once should never be derived twice. An index child
// contributes its own framing and conclusions, which its verifier already held
// to the summary cap. The two together are what G-2 bounds.
func (s *Summarizer) material(c treeplan.Node) (string, error) {
	if c.Kind == treeplan.KindLeaf {
		body, err := s.job.Store.Get(s.job.LeavesDir + "/" + c.Path)
		if err != nil {
			return "", fmt.Errorf("summarize: reading the page %s: %w", c.Path, err)
		}
		return strings.TrimSpace(string(body)), nil
	}
	data, err := s.job.Store.Get(s.Unit(c.Path))
	if err != nil {
		return "", fmt.Errorf("summarize: reading the summary of %s: %w", c.Path, err)
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
func (s *Summarizer) verify(artifactPath, response string) (any, error) {
	u, ok := s.unitOf(artifactPath)
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
	if err := s.check(u, sum); err != nil {
		return nil, s.reject(artifactPath, err)
	}
	sum.Unit, sum.Schema = artifactPath, SchemaVersion
	s.lg.Info("summary written", "stage", "summaries", "unit", artifactPath,
		"level", u.level, "entries", len(u.kids),
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

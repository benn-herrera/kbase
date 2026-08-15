// Package distill is pipeline stage 5 (ARCHITECTURE.md §4 row 5;
// TEMP_DESIGN_V020_RESHAPE §5): the leaf renderer.
//
// # Nothing here calls a model
//
// A leaf body is the verified byte slice (I-3). This package derives that
// slice from the two artifacts that own it — the tree plan says which BYTES a
// group holds, the cut list says where the interior boundaries of a split
// group fall [MAD1: F-2] — applies the one permitted transformation
// (link-target rebasing, §4.4), wraps the result in §4.1's grammar, and stops.
// There is no AskSpec, no seam to classify and no token to spend, which is a
// property a caller can assert rather than trust: a stage built from Lanes
// below is all Produce tasks.
//
// # Why a stage and not a function call at assembly time
//
// The leaf artifact is what an index with leaf children reads at stage 6, what
// the routing-eval unit reads at stage 7.4, what stage 9 re-derives against,
// and what a resume reuses. A leaf that has been proven once should never be
// re-sliced.
//
// # What stage 8 borrows
//
// grammar.go's Frontmatter, RelPath, UpLink and Provenance.Footer are §4's
// shared elements and internal/assemble calls them for the index and
// entry-point kinds, as stage 9 calls their readers
// [MAD1: F-9]. The import direction runs downstream, the same way the pipeline
// does.
package distill

import (
	"fmt"

	"kbase/internal/dissect"
	"kbase/internal/ingest"
	"kbase/internal/pipeline"
	"kbase/internal/survey"
	"kbase/internal/treeplan"
)

// Distiller renders every leaf of ONE tree plan.
//
// It is built once per job, over the four things §5 names as the stage's
// upstreams: the tree plan, the cut lists of its split groups, the survey (the
// link graph the rebase map is built from) and the corpus under custody. It
// holds no store and no mutable state — Render is a pure function of the
// node it is handed — so the coordinator may run its tasks across as many
// domain lanes as it likes.
type Distiller struct {
	plan   treeplan.TreePlan
	corpus ingest.Corpus
	cuts   map[string][]survey.Span
	prov   Provenance

	rebase *rebaseMap
	titles map[string]string
}

// New returns a distiller over one job's artifacts.
//
// cuts holds one entry per SPLIT group, keyed by group id, as
// dissect.DecodeCutList returned it; a one-part group needs no entry because
// its whole span is its leaf. Refusals here are structural incoherence rather
// than unit failures: every input has already been proven by the stage that
// wrote it, so a disagreement between them means the job's own artifacts
// contradict each other and no amount of redoing one unit repairs it.
func New(plan treeplan.TreePlan, art survey.Artifact, corpus ingest.Corpus,
	cuts map[string][]survey.Span, prov Provenance) (*Distiller, error) {

	if plan.Schema != treeplan.SchemaVersion {
		return nil, fmt.Errorf("distill: tree plan declares schema %q, this build reads %q",
			plan.Schema, treeplan.SchemaVersion)
	}
	if plan.CorpusHash != art.Corpus.ContentHash || plan.CorpusHash != corpus.ContentHash {
		return nil, fmt.Errorf("distill: the tree plan plans corpus %s, the survey describes %s and custody holds %s",
			plan.CorpusHash, art.Corpus.ContentHash, corpus.ContentHash)
	}
	if err := prov.Validate(); err != nil {
		return nil, err
	}
	rb, err := newRebaseMap(plan, art, cuts)
	if err != nil {
		return nil, err
	}
	titles := make(map[string]string, len(plan.Nodes))
	for _, n := range plan.Nodes {
		titles[n.Path] = n.Title
	}
	return &Distiller{plan: plan, corpus: corpus, cuts: cuts, prov: prov, rebase: rb, titles: titles}, nil
}

// Leaf is one rendered leaf file and what the rebase left alone.
type Leaf struct {
	// Node is the tree-plan path of the leaf, which is also the artifact's
	// store path under the stage's prefix and its delivered path under <out>.
	Node string
	// Data is the whole §4.1 file: frontmatter, up-link, H1, rebased body,
	// footer.
	Data []byte
	// Unresolved are the corpus-relative destinations in this leaf that the
	// rebase map does not land — a target that is `LinkUnresolved` in the
	// survey, one inside an annex, or one whose file no node was drawn from
	// (§4.4 rule 3). They stay verbatim in Data, and §9 check 1 exempts them:
	// a defect in someone else's corpus is inventoried, not repaired and not
	// fatal.
	Unresolved []string
}

// Render derives one leaf's whole delivered file.
func (d *Distiller) Render(n treeplan.Node) (Leaf, error) {
	if n.Kind != treeplan.KindLeaf {
		return Leaf{}, fmt.Errorf("distill: %s is a %s, and only a page is distilled", n.Path, n.Kind)
	}
	g, ok := d.plan.SplitGroup(n.SplitGroup)
	if !ok {
		return Leaf{}, fmt.Errorf("distill: %s names group %q, which the tree plan does not hold", n.Path, n.SplitGroup)
	}
	span, err := partSpan(g, n.Part, d.cuts[g.ID])
	if err != nil {
		return Leaf{}, err
	}
	doc, ok := d.corpus.Doc(g.Source.File)
	if !ok {
		return Leaf{}, fmt.Errorf("distill: %s draws on %s, which is not under custody", n.Path, g.Source.File)
	}
	if span.Start < 0 || span.End <= span.Start || span.End > len(doc.Bytes) {
		return Leaf{}, fmt.Errorf("distill: %s draws [%d,%d) of the %d-byte file %s",
			n.Path, span.Start, span.End, len(doc.Bytes), g.Source.File)
	}
	if _, ok := d.titles[n.Parent]; !ok {
		return Leaf{}, fmt.Errorf("distill: %s names parent %q, which the tree plan does not hold", n.Path, n.Parent)
	}

	// SliceLeaves rather than a bare re-slice: one function returns a leaf's
	// bytes out of a verified list, and stage 9 re-derives through the same
	// one.
	body := dissect.SliceLeaves(doc.Bytes, []survey.Span{span})[0]
	rebased, left := d.rebase.apply(g.Source.File, n.Path, body)
	return Leaf{Node: n.Path, Data: leaf(n, rebased, d.prov), Unresolved: left}, nil
}

// Leaves is every leaf node of the plan, in tree-plan order.
func (d *Distiller) Leaves() []treeplan.Node {
	out := make([]treeplan.Node, 0, len(d.plan.Nodes))
	for _, n := range d.plan.Nodes {
		if n.Kind == treeplan.KindLeaf {
			out = append(out, n)
		}
	}
	return out
}

// Lanes describes stage 5's work: one serial lane per domain, each holding the
// Produce tasks for the leaves under it.
//
// prefix is the store directory the stage's artifacts land in (`leaves`), and
// owed builds each unit's input and upstream description — the caller owns the
// store layout, because the layout is the composing verb's plan table (§10)
// and not this package's business.
//
// The lane key is the DOMAIN — the entry-point's child a leaf descends from —
// which is what §5 asks for and what keeps one worker's stability state to one
// subtree. Leaves parented directly by the entry-point form their own lane.
func (d *Distiller) Lanes(prefix string, owed func(treeplan.Node) pipeline.OwedArtifact) []pipeline.SerialLane {
	domains := d.domains()
	order := make([]string, 0, 8)
	tasks := map[string][]pipeline.LaneTask{}
	for _, n := range d.Leaves() {
		dom := domains[n.Path]
		if _, seen := tasks[dom]; !seen {
			order = append(order, dom)
		}
		tasks[dom] = append(tasks[dom], pipeline.LaneTask{
			Owed:    owed(n),
			Section: dom,
			Produce: d.producer(n),
		})
	}
	lanes := make([]pipeline.SerialLane, 0, len(order))
	for _, dom := range order {
		lanes = append(lanes, pipeline.SerialLane{Domain: prefix + "/" + dom, Tasks: tasks[dom]})
	}
	return lanes
}

// producer closes over one node so the task carries no index into a slice that
// could be re-ordered before it runs.
func (d *Distiller) producer(n treeplan.Node) pipeline.Producer {
	return func() (any, error) {
		l, err := d.Render(n)
		if err != nil {
			return nil, err
		}
		return l, nil
	}
}

// domains maps every node to the entry-point child it descends from.
func (d *Distiller) domains() map[string]string {
	parent := make(map[string]string, len(d.plan.Nodes))
	root := ""
	for _, n := range d.plan.Nodes {
		parent[n.Path] = n.Parent
		if n.Kind == treeplan.KindEntryPoint {
			root = n.Path
		}
	}
	out := make(map[string]string, len(d.plan.Nodes))
	for _, n := range d.plan.Nodes {
		p := n.Path
		for parent[p] != root && parent[p] != "" {
			p = parent[p]
		}
		out[n.Path] = p
	}
	return out
}

// Encode renders a leaf artifact for the store — the stage's
// pipeline.StagePlan.Encode.
//
// A leaf's bytes ARE the artifact, so this unwraps rather than serialises. It
// still refuses an empty one: a zero-byte page is a leaf whose span produced
// nothing, and writing it would put an empty file in the delivered tree with a
// stamp proving it.
func Encode(artifact any) ([]byte, error) {
	l, ok := artifact.(Leaf)
	if !ok {
		return nil, fmt.Errorf("distill: artifact is %T, not a Leaf", artifact)
	}
	if len(l.Data) == 0 {
		return nil, fmt.Errorf("distill: %s rendered no bytes", l.Node)
	}
	return l.Data, nil
}

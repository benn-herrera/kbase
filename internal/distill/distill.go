// Package distill is pipeline stage 5 (ARCHITECTURE.md §4 row 5;
// TEMP_DESIGN_V020_RESHAPE §5): the leaf renderer.
//
// # Nothing here calls a model
//
// A leaf body is the verified byte slice (I-3). This package derives that
// slice from the two artifacts that own it — the tree plan says which BYTES a
// group holds, the cut list says where the interior boundaries of a split
// group fall [MAD1: F-2] — applies the one permitted transformation
// (link-target rebasing, §4.5), wraps the result in §4.1's grammar, and stops.
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
	// parts is which leaf is part k of a split group — the continuation edges
	// read as a lookup rather than re-derived from a name, because the part
	// naming grammar is the tree plan's and nothing here may spell it twice
	// (I-1).
	parts map[partKey]string
	// headings is every source heading of every file, in document order, which
	// is what a part's post-cut descriptor is drawn from (Descriptors).
	headings map[string][]heading
}

// partKey names one part of one split group.
type partKey struct {
	group string
	part  int
}

// heading is one source heading: where it starts, and the text the survey read
// off it. The survey is the only place a heading is ever parsed (the format
// seam, ARCHITECTURE §4), so a descriptor drawn from these carries plain text
// and never a fragment of Markdown syntax.
type heading struct {
	start int
	title string
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
	parts := map[partKey]string{}
	for _, n := range plan.Nodes {
		titles[n.Path] = n.Title
		if n.Kind == treeplan.KindLeaf {
			parts[partKey{group: n.SplitGroup, part: n.Part}] = n.Path
		}
	}
	return &Distiller{plan: plan, corpus: corpus, cuts: cuts, prov: prov, rebase: rb,
		titles: titles, parts: parts, headings: sourceHeadings(art)}, nil
}

// sourceHeadings flattens the survey's heading tree per file into document
// order: a section's own heading, then the headings under it, which is the
// order their bytes appear in.
func sourceHeadings(art survey.Artifact) map[string][]heading {
	out := make(map[string][]heading, len(art.Files))
	for _, f := range art.Files {
		var hs []heading
		var walk func([]survey.Section)
		walk = func(secs []survey.Section) {
			for _, s := range secs {
				if s.Title != "" {
					hs = append(hs, heading{start: s.Start, title: s.Title})
				}
				walk(s.Children)
			}
		}
		walk(f.Sections)
		out[f.Path] = hs
	}
	return out
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
	// (§4.5 rule 3). They stay verbatim in Data, and §4.8 guarantee 1 exempts them:
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
	prev, next := d.siblings(n)
	return Leaf{Node: n.Path, Data: leaf(n, prev, next, rebased, d.prov), Unresolved: left}, nil
}

// siblings is the part before and the part after this one in its own split
// group — empty where there is none, which is the first part's previous, the
// last part's next, and both of an unsplit leaf.
func (d *Distiller) siblings(n treeplan.Node) (prev, next string) {
	return d.parts[partKey{group: n.SplitGroup, part: n.Part - 1}],
		d.parts[partKey{group: n.SplitGroup, part: n.Part + 1}]
}

// Descriptors is what each part of a split group actually turns out to hold,
// keyed by node path: the title of the first source heading the part delivers,
// and of the last where it delivers more than one.
//
// A split group's parts share ONE scope, authored at stage 3 against the whole
// span — the boundaries were not chosen yet, so no per-part description could
// have been written there [MAD2: B-4]. This is the post-cut half: mechanical,
// no model, and derived from the same (tree plan, cut list, survey) triple the
// pages themselves are, which is what lets stage 8 render it into the index and
// stage 9 re-derive the same string rather than trust the one it is checking.
//
// Only split groups appear. An unsplit leaf's bullet is already the only one
// naming its material, and a part that delivers no heading at all — a cut that
// fell in open prose — contributes nothing rather than a fabricated label.
//
// # Distinctness
//
// Two parts of one group whose first-and-last headings coincide — a source
// repeating `## Notes` / `## Examples` across a cut boundary — would render
// bullets that differ in nothing but the path, and telling two sibling bullets
// apart is the descriptor's whole job (SPEC §4.3). Where that happens, every
// part sharing the descriptor falls back to its ORDINAL instead, which is
// distinct by construction. Parts whose headings already distinguish them keep
// them: the fallback is per collision, not per group.
func (d *Distiller) Descriptors() (map[string]string, error) {
	out := map[string]string{}
	// byGroup holds each group's described parts in part order, so the
	// collision check below is over siblings and nothing else.
	byGroup := map[string][]treeplan.Node{}
	for _, n := range d.Leaves() {
		g, ok := d.plan.SplitGroup(n.SplitGroup)
		if !ok {
			return nil, fmt.Errorf("distill: %s names group %q, which the tree plan does not hold", n.Path, n.SplitGroup)
		}
		if g.Parts == 1 {
			continue
		}
		span, err := partSpan(g, n.Part, d.cuts[g.ID])
		if err != nil {
			return nil, err
		}
		if desc := describePart(d.headings[g.Source.File], span); desc != "" {
			out[n.Path] = desc
			byGroup[g.ID] = append(byGroup[g.ID], n)
		}
	}
	for id, parts := range byGroup {
		g, ok := d.plan.SplitGroup(id)
		if !ok {
			return nil, fmt.Errorf("distill: group %q left the tree plan mid-derivation", id)
		}
		seen := map[string]int{}
		for _, n := range parts {
			seen[out[n.Path]]++
		}
		for _, n := range parts {
			if seen[out[n.Path]] > 1 {
				out[n.Path] = partOrdinal(n.Part, g.Parts)
			}
		}
	}
	return out, nil
}

// partOrdinal is the descriptor a part falls back to when its headings do not
// tell it apart from a sibling's: which part of how many, in the same
// vocabulary the title's `(k/n)` already uses.
func partOrdinal(k, n int) string { return fmt.Sprintf("%d of %d", k, n) }

// describePart names one part by the headings inside its own byte range.
//
// A heading whose line STARTS inside the range is delivered on that page —
// the body is the range verbatim — so the membership test is the range test,
// with no second notion of where a section belongs.
//
// The titles are NEUTRALIZED on the way out (see neutralize). This is the
// first channel putting arbitrary corpus bytes onto a class-A index page, and
// an index page is not verbatim source: it is rendered, and a heading like
// "`](x)` form" would otherwise land a live link destination in a bullet that
// the tree does not deliver, failing guarantee 1 on a well-formed corpus with
// no repair available [GO M-7].
func describePart(hs []heading, span survey.Span) string {
	var first, last string
	for _, h := range hs {
		if h.start < span.Start || h.start >= span.End {
			continue
		}
		if first == "" {
			first = h.title
		}
		last = h.title
	}
	if first == "" || first == last {
		return neutralize(first)
	}
	return neutralize(first) + headingSpan + neutralize(last)
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

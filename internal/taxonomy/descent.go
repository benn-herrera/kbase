package taxonomy

import (
	"fmt"
	"path"
	"strings"

	"kbase/internal/survey"
	"kbase/internal/treeplan"
)

// The mechanical half of §3.2: the descent's question set, enumerated from the
// survey BEFORE the first call.
//
// The question set is derived from the SOURCE's container tree, not from the
// tree the model is building, which is what makes it enumerable up front —
// and it has to be enumerable up front, because a stage's lanes are resolved
// once, when the stage is reached (pipeline.LaneResolver).
//
// # Sections are containers too
//
// A document's entries are its SKELETON (treeplan.Skeleton), not its top-level
// sections: where the source's own headings descend into a section — the rule
// is R-C, and it is stated once, in treeplan — that section is a container like
// any other, and this stage asks the same grouping question over its body and
// its subsections that it asks over a document's sections. Nothing else about
// the descent changes: the candidate list is the same shape, the answer
// vocabulary is the same, and a section that fits on one page is still one
// candidate, one span, one page.
//
// Reading the skeleton rather than re-deriving it is the whole point (I-1).
// The mechanical baseline proposal builds its tree from the same function, so
// the two stage-3 shapes cannot disagree about which containers a corpus has —
// which is what makes the hermetic recipes evidence about the shipped path.
//
// # Single-entry containers are absorbed
//
// A container holding exactly one entry offers no grouping decision — the only
// partition of one candidate is the one group — so asking it would spend a
// heavy call to learn nothing and add a KB level nobody chose. It is absorbed
// instead, which is §2.7's chain collapse read on the source side, before any
// call is described rather than after the tree is built.

// candKind is what a candidate is on the SOURCE side. It is the vocabulary the
// candidate list uses, and it is deliberately not the KB's (`page`/`section`):
// what a candidate BECOMES is the model's answer, and a candidate list that
// pre-named the answer would be asking a leading question.
type candKind string

const (
	candFolder   candKind = "folder"
	candDocument candKind = "document"
	candSection  candKind = "section"
)

// cand is one entry of a container's candidate list — and, when it holds
// entries of its own, a container the descent asks about in turn.
type cand struct {
	kind   candKind
	title  string
	gist   string
	tokens int

	// spans is the source material this candidate covers: one range for a
	// document or a section, one per document beneath a folder. It is what a
	// group flagged `page` draws its bytes from. Everything but a folder is
	// single-span, so a folder is the only candidate whose placement on a page
	// needs §2.7's dissolution.
	spans []treeplan.Span

	// kids are the direct entries this candidate holds. Empty means it is a
	// candidate and nothing more; non-empty means the descent asks about it.
	kids []*cand
}

// enumerate builds the container tree the descent walks: the corpus root, its
// folders, its documents, and each document's skeleton.
//
// The budgets are here because the descent rule is stated in one of them
// (treeplan.Skeleton): which sections are containers is a function of the leaf
// budget, so a question set enumerated without it would be a different set of
// questions than the one the tree is verified against.
//
// Material under a declared annex prefix is not enumerated at all — an annex
// costs zero calls, which is half the point of declaring one (§3.2).
func enumerate(art survey.Artifact, title string, b treeplan.Budgets, annexes []treeplan.Annex) *cand {
	root := &cand{kind: candFolder, title: title}
	folders := map[string]*cand{"": root}
	for _, f := range art.Files {
		if _, annexed := treeplan.AnnexedBy(f.Path, annexes); annexed {
			continue
		}
		doc := documentCand(f, b)
		if doc == nil {
			// A document the survey found no material in — an empty file, or
			// one that is nothing but front matter. It contributes no
			// candidate, and coverage is unaffected because it has no section
			// to cover.
			continue
		}
		parent := folderFor(folders, path.Dir(f.Path))
		parent.kids = append(parent.kids, doc)
	}
	normalize(root)
	return root
}

// folderFor returns the folder candidate for a corpus directory, creating it
// and its ancestors on first sight. Documents arrive in the survey's path
// order, so the folders and their entries are in path order too and two runs
// over one corpus enumerate the same questions in the same order.
func folderFor(folders map[string]*cand, dir string) *cand {
	if dir == "." || dir == "/" {
		dir = ""
	}
	if c, ok := folders[dir]; ok {
		return c
	}
	parent := folderFor(folders, path.Dir(dir))
	c := &cand{kind: candFolder, title: path.Base(dir)}
	folders[dir] = c
	parent.kids = append(parent.kids, c)
	return c
}

// documentCand is one document: the candidate a folder's call sees, and the
// container whose own call groups the entries of its skeleton.
func documentCand(f survey.File, b treeplan.Budgets) *cand {
	skel := treeplan.Skeleton(f, b)
	if len(skel) == 0 {
		return nil
	}
	doc := &cand{
		kind:  candDocument,
		title: treeplan.DocTitle(f),
		gist:  firstNonEmpty(f.Description, f.Gist),
		// The document's own span runs from its first top-level entry to its
		// last, which is every byte the survey calls a section and none of the
		// out-of-band metadata block before them.
		tokens: f.Tokens,
		spans: []treeplan.Span{{File: f.Path,
			Start: skel[0].Span.Start, End: skel[len(skel)-1].Span.End}},
	}
	for _, n := range skel {
		doc.kids = append(doc.kids, skelCand(n, f.Path))
	}
	return doc
}

// skelCand is one skeleton node as a candidate, and its subtree where the
// source's headings descend into it.
//
// Titles are the skeleton's own — a section's heading, and the preamble
// convention for a container's own body (R-2) — so the descent presents a
// document's material under exactly the names the mechanical proposal gives it.
// A container's span is its WHOLE subtree, which is what an answer placing it
// on a page draws its bytes from: the model never rules on size, and an
// over-budget page is split mechanically like any other (R-5).
func skelCand(n treeplan.SkelNode, file string) *cand {
	c := &cand{
		kind:   candSection,
		title:  n.Title,
		gist:   n.Gist,
		tokens: n.Tokens,
		spans:  []treeplan.Span{{File: file, Start: n.Span.Start, End: n.Span.End}},
	}
	for _, k := range n.Children {
		c.kids = append(c.kids, skelCand(k, file))
	}
	return c
}

// normalize rolls a folder's material up from its entries and absorbs every
// single-entry container, bottom-up.
//
// Absorption keeps the OUTER title, which is §2.7's rule — except for a folder,
// whose name is a path segment rather than a title: a document names itself,
// and the directory it happens to sit alone in does not.
func normalize(c *cand) {
	for _, k := range c.kids {
		normalize(k)
	}
	if c.kind == candFolder {
		c.spans, c.tokens = nil, 0
		for _, k := range c.kids {
			c.spans = append(c.spans, k.spans...)
			c.tokens += k.tokens
		}
	}
	for len(c.kids) == 1 {
		k := c.kids[0]
		if c.kind == candFolder {
			c.kind, c.title, c.gist = k.kind, k.title, k.gist
		}
		c.spans, c.tokens = k.spans, k.tokens
		c.kids = k.kids
	}
}

// ask is one container call: the container, the slice of its entries this call
// presents, and where that slice sits when the container was pre-batched.
//
// A container with more direct entries than the candidate cap is split into
// several calls in entry order, and their groups are merged under the one
// parent — mechanically, before any call is described, so an over-cap
// candidate list reaching the verifier is our arithmetic being wrong and not a
// runtime condition (I-6, §3.2).
type ask struct {
	c       *cand
	cands   []*cand
	batch   int
	batches int
	unit    string
}

// asks enumerates the calls in depth-first order, parents before children —
// the order the fold depends on, since a container's groups attach to the node
// its parent's answer created.
func asks(root *cand, candidateCap int) []*ask {
	var out []*ask
	var walk func(*cand)
	walk = func(c *cand) {
		if len(c.kids) == 0 {
			return
		}
		batches := batch(c.kids, candidateCap)
		for i, b := range batches {
			out = append(out, &ask{c: c, cands: b, batch: i + 1, batches: len(batches)})
		}
		for _, k := range c.kids {
			walk(k)
		}
	}
	walk(root)
	return out
}

// batch splits a candidate list into calls of at most cap entries, in order.
func batch(kids []*cand, cap int) [][]*cand {
	if len(kids) <= cap {
		return [][]*cand{kids}
	}
	var out [][]*cand
	for i := 0; i < len(kids); i += cap {
		out = append(out, kids[i:min(i+cap, len(kids))])
	}
	return out
}

// entry renders one candidate for the numbered list the call presents: title,
// kind, token count, entry count and gist (§3.2).
func (c *cand) entry(n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d. %s (%s, %d tokens", n, c.title, c.kind, c.tokens)
	if len(c.kids) > 0 {
		fmt.Fprintf(&b, ", %d entries", len(c.kids))
	}
	b.WriteString(")")
	if c.gist != "" {
		b.WriteString(" — ")
		b.WriteString(c.gist)
	}
	return b.String()
}

// scope is a candidate's own one-line scope, for a page the model did not name
// individually: the survey's gist where there is one, and the title otherwise.
// The model authors the scope of every group it names (§2.1); this is what a
// mechanically created sibling gets.
func (c *cand) scope() string { return firstNonEmpty(c.gist, c.title) }

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

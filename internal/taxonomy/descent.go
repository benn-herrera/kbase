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
// # Why sections are not containers
//
// The design (§3.2) also enumerates "each survey.Section whose subtree exceeds
// the leaf budget". That question cannot be asked against the landed verifier:
// coverage is stated over WHOLE sections — treeplan's check requires every
// surveyed section, at every depth, to sit inside ONE group span — so a section
// whose children became sibling pages leaves its own body, and the section
// itself, on no page at all. An oversized section is therefore split
// mechanically instead (§2.4, one group, n parts), which is where the design
// puts that repair anyway. The consequence is a stated one: page granularity is
// a document's top-level sections, never deeper.
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
// folders, its documents, and each document's top-level sections.
//
// Material under a declared annex prefix is not enumerated at all — an annex
// costs zero calls, which is half the point of declaring one (§3.2).
func enumerate(art survey.Artifact, title string, annexes []treeplan.Annex) *cand {
	root := &cand{kind: candFolder, title: title}
	folders := map[string]*cand{"": root}
	for _, f := range art.Files {
		if _, annexed := treeplan.AnnexedBy(f.Path, annexes); annexed {
			continue
		}
		doc := documentCand(f)
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
// container whose own call groups its top-level sections.
func documentCand(f survey.File) *cand {
	tops := topSections(f)
	if len(tops) == 0 {
		return nil
	}
	title := firstNonEmpty(f.Title, baseName(f.Path))
	doc := &cand{
		kind:   candDocument,
		title:  title,
		gist:   firstNonEmpty(f.Description, f.Gist),
		tokens: f.Tokens,
		// The document's own span runs from its first top-level section to its
		// last, which is every byte the survey calls a section and none of the
		// out-of-band metadata block before them.
		spans: []treeplan.Span{{File: f.Path, Start: tops[0].Start, End: tops[len(tops)-1].End}},
	}
	for _, s := range tops {
		doc.kids = append(doc.kids, sectionCand(f, s, title))
	}
	return doc
}

// sectionCand is one top-level section — the finest granularity a page may
// have (see the package note above).
func sectionCand(f survey.File, s survey.Section, docTitle string) *cand {
	title := s.Title
	if title == "" {
		// The headingless preamble. It is named after the document it opens,
		// the same way the mechanical baseline proposal names it.
		title = "Introduction to " + docTitle
	}
	return &cand{
		kind:   candSection,
		title:  title,
		gist:   s.Gist,
		tokens: s.Tokens,
		spans:  []treeplan.Span{{File: f.Path, Start: s.Start, End: s.End}},
	}
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

// topSections is a document's preamble plus its top-level sections: the ranges
// that tile it, since a section's children nest inside it.
func topSections(f survey.File) []survey.Section {
	var out []survey.Section
	if f.Preamble != nil && f.Preamble.End > f.Preamble.Start {
		out = append(out, *f.Preamble)
	}
	out = append(out, f.Sections...)
	return out
}

func baseName(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, path.Ext(base))
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

package treeplan

import (
	"fmt"

	"kbase/internal/survey"
)

// SourceStructureProposal is the DEV/BASELINE proposal: the tree the source's
// own structure already describes — one domain per document, and under it the
// document's skeleton (Skeleton): a page per section that fits on one, and an
// index over its own body and its subsections where it does not.
//
// # What this is NOT
//
// It is not a fallback for stage 3. Taxonomy design is a NO-FALLBACK seam
// (ARCHITECTURE §12; O-1): a mechanically grouped tree with generated titles is
// not "already acceptable", and a stage that quietly produced one when the
// model failed would ship a KB whose organisation nobody chose while reporting
// success. Nothing in the shipped pipeline may call this in that position, and
// the one caller today is `kbase build` under `[dev] tree_plan = "mechanical"`,
// a switch whose entire premise is a run with no model in it.
//
// # What it is for
//
// Two things, and both want the same tree:
//
//   - The corpus property in this package holds a REAL tree plan, over real
//     material, at the shipped budgets, to every composed post-condition —
//     which needs a proposal that exercises the machinery rather than one built
//     to pass. This one covers every surveyed section, so the tiling check is
//     live; every span is real, so the splitter runs over real bytes; and the
//     fan-out cap bites on whichever containers have more entries than the cap.
//   - A hermetic end-to-end run needs a tree plan without a model in it, so
//     that stages 5, 8 and 9 can be exercised over a real corpus and produce a
//     walkable KB.
//
// Material under a declared annex is skipped, exactly as §3.2's descent does
// not enumerate it. An annex is a property of the CORPUS, so which shape of
// stage 3 is running has no bearing on it — and a proposal that covered
// annexed material would be refused by the coverage check for doing so.
//
// It takes the budgets rather than reading a package constant because the
// descent rule is stated in one of them: this proposal must produce the SAME
// containers the taxonomy stage would be handed over the same corpus, or the
// hermetic recipes that run it prove nothing about the shipped path.
//
// Scope lines are mechanical and say so: the survey's gist where there is one,
// and the title otherwise. A model writes the real ones (§2.1).
func SourceStructureProposal(art survey.Artifact, b Budgets, title, scope string, annexes []Annex) (TreeProposal, error) {
	if art.Schema != survey.SchemaVersion {
		return TreeProposal{}, fmt.Errorf("treeplan: survey declares schema %q, this build reads %q",
			art.Schema, survey.SchemaVersion)
	}
	if err := b.Validate(); err != nil {
		return TreeProposal{}, err
	}
	p := TreeProposal{Title: title, Scope: scope}
	for _, f := range art.Files {
		if _, annexed := AnnexedBy(f.Path, annexes); annexed {
			continue
		}
		doc := DocTitle(f)
		var pages []ProposalNode
		for _, n := range Skeleton(f, b) {
			pages = append(pages, skelProposal(n, f.Path))
		}
		if len(pages) == 0 {
			// A document the survey found no material in — an empty file, or
			// one that is nothing but front matter. It contributes no node,
			// and coverage is unaffected because it has no section to cover.
			continue
		}
		p.Children = append(p.Children, ProposalNode{
			Title:    doc,
			Scope:    fileScope(f, doc),
			Kind:     KindIndex,
			Children: pages,
		})
	}
	if len(p.Children) == 0 {
		return TreeProposal{}, fmt.Errorf("treeplan: the corpus holds no document with any material in it")
	}
	return p, nil
}

// skelProposal turns one skeleton node into a proposed node: a container
// becomes an index over its entries, and everything else a page over its own
// span.
//
// The scope line is the same rule at every depth — the survey's gist, or the
// title when the document offered none — because a body page and a section page
// are the same kind of thing to a reader and the skeleton already decided what
// each is called.
func skelProposal(n SkelNode, file string) ProposalNode {
	p := ProposalNode{Title: n.Title, Scope: firstNonEmpty(n.Gist, n.Title)}
	if len(n.Children) == 0 {
		p.Kind = KindLeaf
		p.Sources = []Span{{File: file, Start: n.Span.Start, End: n.Span.End}}
		return p
	}
	p.Kind = KindIndex
	p.Children = make([]ProposalNode, 0, len(n.Children))
	for _, c := range n.Children {
		p.Children = append(p.Children, skelProposal(c, file))
	}
	return p
}

// fileScope is a document's own one-line scope.
func fileScope(f survey.File, title string) string {
	return firstNonEmpty(f.Description, f.Gist, title)
}

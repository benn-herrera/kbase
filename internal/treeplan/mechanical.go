package treeplan

import (
	"fmt"

	"kbase/internal/survey"
)

// SourceStructureProposal is the DEV/BASELINE proposal: the tree the source's
// own file structure already describes — one domain per document, one page per
// top-level section.
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
//     fan-out cap bites on whichever documents have more top-level sections
//     than the cap.
//   - A hermetic end-to-end run needs a tree plan without a model in it, so
//     that stages 5, 8 and 9 can be exercised over a real corpus and produce a
//     walkable KB.
//
// Material under a declared annex is skipped, exactly as §3.2's descent does
// not enumerate it. An annex is a property of the CORPUS, so which shape of
// stage 3 is running has no bearing on it — and a proposal that covered
// annexed material would be refused by the coverage check for doing so.
//
// Scope lines are mechanical and say so: the survey's gist where there is one,
// and the title otherwise. A model writes the real ones (§2.1).
func SourceStructureProposal(art survey.Artifact, title, scope string, annexes []Annex) (TreeProposal, error) {
	if art.Schema != survey.SchemaVersion {
		return TreeProposal{}, fmt.Errorf("treeplan: survey declares schema %q, this build reads %q",
			art.Schema, survey.SchemaVersion)
	}
	p := TreeProposal{Title: title, Scope: scope}
	for _, f := range art.Files {
		if _, annexed := AnnexedBy(f.Path, annexes); annexed {
			continue
		}
		docTitle := f.Title
		if docTitle == "" {
			docTitle = f.Path
		}
		var pages []ProposalNode
		for _, span := range topLevelSpans(f) {
			t, s := spanLabels(f, span)
			pages = append(pages, ProposalNode{
				Title:   t,
				Scope:   s,
				Kind:    KindLeaf,
				Sources: []Span{{File: f.Path, Start: span.Start, End: span.End}},
			})
		}
		if len(pages) == 0 {
			// A document the survey found no material in — an empty file, or
			// one that is nothing but front matter. It contributes no node,
			// and coverage is unaffected because it has no section to cover.
			continue
		}
		p.Children = append(p.Children, ProposalNode{
			Title:    docTitle,
			Scope:    fileScope(f, docTitle),
			Kind:     KindIndex,
			Children: pages,
		})
	}
	if len(p.Children) == 0 {
		return TreeProposal{}, fmt.Errorf("treeplan: the corpus holds no document with any material in it")
	}
	return p, nil
}

// topLevelSpans is a file's preamble plus its top-level sections: the ranges
// that tile it, since a section's children nest inside it.
func topLevelSpans(f survey.File) []survey.Span {
	var out []survey.Span
	if f.Preamble != nil && f.Preamble.End > f.Preamble.Start {
		out = append(out, survey.Span{Start: f.Preamble.Start, End: f.Preamble.End})
	}
	for _, s := range f.Sections {
		out = append(out, survey.Span{Start: s.Start, End: s.End})
	}
	return out
}

// spanLabels names one span: the section's own title and gist, or the file's
// own identity when the span is the headingless preamble.
func spanLabels(f survey.File, r survey.Span) (title, scope string) {
	for _, s := range f.Sections {
		if s.Start == r.Start {
			t := s.Title
			if t == "" {
				t = f.Path
			}
			return t, firstNonEmpty(s.Gist, t)
		}
	}
	name := f.Title
	if name == "" {
		name = f.Path
	}
	return "Introduction to " + name, firstNonEmpty(f.Gist, "the opening of "+f.Path)
}

// fileScope is a document's own one-line scope.
func fileScope(f survey.File, title string) string {
	return firstNonEmpty(f.Description, f.Gist, title)
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

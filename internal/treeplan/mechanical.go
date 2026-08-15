package treeplan

import (
	"fmt"
	"path"
	"strings"

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
		doc := docTitle(f)
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
			t := firstNonEmpty(s.Title, docTitle(f))
			return t, firstNonEmpty(s.Gist, t)
		}
	}
	return "Introduction to " + docTitle(f), firstNonEmpty(f.Gist, "the opening of "+f.Path)
}

// docTitle names a document, and NEVER with a path (ruled 2026-08-15).
//
// The chain is: what the document calls itself in its metadata block, then its
// first H1 — already in the survey's heading tree, so this reaches for no new
// machinery — then the path humanised. A title is a claim about subject
// matter, and a path-shaped one ("experimental/dflash_mlx_integration.md")
// claims a file that the knowledge base does not contain: the reader is
// offered an address to somewhere that does not exist. Humanising is what
// keeps the last resort a title.
//
// It is one function because every title position in this proposal — the
// domain index, an untitled section, the preamble page — has the same three
// candidates and the same prohibition.
func docTitle(f survey.File) string {
	return firstNonEmpty(f.Title, firstH1(f), humanisePath(f.Path))
}

// firstH1 is the document's first level-1 heading. Only the top level is
// scanned: a level-1 heading cannot nest under a heading of any level, so
// every H1 in a file is one of its top-level sections.
func firstH1(f survey.File) string {
	for _, s := range f.Sections {
		if s.Level == 1 && s.Title != "" {
			return s.Title
		}
	}
	return ""
}

// humanisePath turns a corpus path into a phrase: the extension is dropped and
// the separators a path is spelled with — `/` between directories, `_` inside
// a name — become spaces. What is left reads as a subject, and no longer
// resolves as an address.
func humanisePath(p string) string {
	p = strings.TrimSuffix(p, path.Ext(p))
	p = strings.Map(func(r rune) rune {
		if r == '/' || r == '_' {
			return ' '
		}
		return r
	}, p)
	return strings.Join(strings.Fields(p), " ")
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

package treeplan

import (
	"path"
	"strings"

	"kbase/internal/survey"
)

// SkelNode is one node of a document's skeleton: a span of that document, what
// it is called, and — when it is a container — the nodes its material is made
// of.
//
// Children of a container tile its own Span exactly and in byte order: the
// container's body first, then its child sections. That is what lets a consumer
// place a container's whole subtree on one page or descend into it without
// asking anything else where the bytes are.
type SkelNode struct {
	// Span is every byte this node covers: a leaf's own material, or a
	// container's whole subtree.
	Span survey.Span
	// Title is what the node is called. A section is called what its heading
	// says; a body page is called after the container it opens (R-2).
	Title string
	// Gist is the survey's first-paragraph hint, for a scope line a caller has
	// to author mechanically.
	Gist string
	// Tokens is the material's own size. For a container it is the subtree's,
	// which is the number the descent rule is stated in.
	Tokens int
	// Children are the container's entries, empty on a leaf.
	Children []SkelNode
}

// Skeleton is THE statement of which of a document's sections are containers
// and what each container's own body page is (rule R-C, ruled 2026-08-17).
//
// # The rule
//
// A section descends — becomes a container whose entries are its own body and
// its child sections — exactly when its subtree is over the leaf budget and it
// has children. Nothing else. Descent is not "more granularity": it is the
// split that has to happen anyway, made semantic. A section that fits on one
// page IS one page; a section that does not is going to be cut regardless, and
// the author's own headings are a better cut than the splitter's heuristic
// one. So the descent count scales with corpus SIZE (siblings' subtrees are
// disjoint, so at one level there can be no more descents than
// corpusTokens/leafTokens) rather than with how many headings someone typed,
// and the rule introduces no constant of its own — it reuses the leaf budget
// the split path is already measured against.
//
// A section over the budget with no children still descends into nothing and is
// split mechanically, which is correct: there is no author structure to cut
// along. A container whose own body is over the budget is likewise split, at
// any depth — the model never rules on size (R-5, ruled 2026-08-17).
//
// # The body span carries the heading line
//
// body(S) = [S.Start, firstChild.Start), and survey.Section.Start is the first
// byte of the heading line itself (SPEC §3.2), so the body page carries `##
// Whatever` and its lead-in prose. Deliberately: starting the body after the
// heading would drop those bytes from every delivered page and open a SECOND
// carve-out in the verbatim-custody story beside `metadata`. One carve-out,
// loudly documented, is the budget.
//
// # Depth follows the source
//
// The recursion has no cap (R-3, ruled 2026-08-17). Depth is bounded by the
// nesting the source graph requires and is measured and reported, never capped
// or repaired — heading trees are one WITNESS of that graph, and directory
// layout is never a structural input to it.
//
// # Why it is one function
//
// Four places would otherwise each grow their own copy of this rule: the
// taxonomy stage's question set, the mechanical baseline proposal, the content
// floor, and the coverage gate. Two of them were already duplicates of each
// other. The rule is stated here, once (I-1); the coverage gate is stated over
// the survey's own section boundaries and holds NO rule at all, so it accepts
// any tiling the skeleton can produce without knowing what the skeleton did.
//
// Its inputs are one surveyed file and the budgets — no bytes, no corpus, no
// clock — so the stage that holds only the survey can call it and get the same
// answer this package's own composition does.
func Skeleton(f survey.File, b Budgets) []SkelNode {
	doc := DocTitle(f)
	out := make([]SkelNode, 0, len(f.Sections)+1)
	if p := f.Preamble; p != nil && p.End > p.Start {
		// The headingless opening, named after the document it opens: the
		// convention a container's body page reuses (R-2), so the tree has one
		// grammar for "this container's own material" wherever it appears.
		out = append(out, SkelNode{
			Span:   survey.Span{Start: p.Start, End: p.End},
			Title:  bodyTitle(doc),
			Gist:   firstNonEmpty(p.Gist, f.Gist),
			Tokens: p.Tokens,
		})
	}
	for _, s := range f.Sections {
		out = append(out, skelSection(s, doc, b))
	}
	return out
}

// skelSection is one surveyed section and, where the rule fires, everything
// under it.
func skelSection(s survey.Section, doc string, b Budgets) SkelNode {
	n := SkelNode{
		Span:   survey.Span{Start: s.Start, End: s.End},
		Title:  firstNonEmpty(s.Title, doc),
		Gist:   s.Gist,
		Tokens: s.Tokens,
	}
	if !descends(s, b) {
		return n
	}
	n.Children = make([]SkelNode, 0, len(s.Children)+1)
	if body := (survey.Span{Start: s.Start, End: s.Children[0].Start}); body.End > body.Start {
		n.Children = append(n.Children, SkelNode{
			Span:  body,
			Title: bodyTitle(n.Title),
			// The section's gist IS its first paragraph, which is the body's
			// own material and not the subsections'.
			Gist:   s.Gist,
			Tokens: bodyTokens(s),
		})
	}
	for _, c := range s.Children {
		n.Children = append(n.Children, skelSection(c, doc, b))
	}
	return n
}

// descends is rule R-C, spelled once: over the leaf budget, and with author
// structure to descend into.
//
// The subtree count is the survey's own (survey.Section.Tokens), which is what
// makes the rule answerable without the corpus bytes.
func descends(s survey.Section, b Budgets) bool {
	return s.Tokens > b.LeafTokens && len(s.Children) > 0
}

// bodyTokens is a container's own material: its subtree less its children's.
//
// It is arithmetic over the survey's counts rather than a fresh estimate,
// because Skeleton holds no bytes to estimate. The counts are an estimator's
// output either way (§8) and this is a routing hint, not a guarantee — G-1 is
// proved by the splitter over the real bytes, downstream.
func bodyTokens(s survey.Section) int {
	n := s.Tokens
	for _, c := range s.Children {
		n -= c.Tokens
	}
	return max(n, 0)
}

// bodyTitle names a container's own material after the container: the
// convention the file preamble has always used, now stated once for both (R-2,
// ruled 2026-08-17).
//
// The alternative — naming the body page exactly what its container is called —
// reads better in one index bullet and costs a second naming grammar, plus an
// index and its first child sharing a title.
func bodyTitle(container string) string { return "Introduction to " + container }

// DocTitle names a document, and NEVER with a path (ruled 2026-08-15).
//
// The chain is: what the document calls itself in its metadata block, then its
// first H1 — already in the survey's heading tree, so this reaches for no new
// machinery — then the path humanised. A title is a claim about subject
// matter, and a path-shaped one ("experimental/dflash_mlx_integration.md")
// claims a file that the knowledge base does not contain: the reader is
// offered an address to somewhere that does not exist. Humanising is what
// keeps the last resort a title.
//
// It is one function because every title position over a document — the domain
// index, an untitled section, the preamble page — has the same three candidates
// and the same prohibition. It is EXPORTED for the same reason Skeleton is:
// the taxonomy stage's candidate list names the same documents this package's
// own proposal does, and a document called two things in one corpus is the
// second source I-1 forbids.
func DocTitle(f survey.File) string {
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

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

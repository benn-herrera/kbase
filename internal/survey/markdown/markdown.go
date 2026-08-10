// Package markdown is the Markdown adapter for the survey stage
// (ARCHITECTURE.md §4, stage 2): it parses CommonMark and returns the neutral
// survey.Artifact.
//
// This is the only package that may import goldmark or yaml — see the import
// policy test in internal/survey. That is the whole point of the split: the
// artifact is the format-independence boundary, so every format-specific
// decision (what a heading is, how a metadata block is delimited, how a link
// is spelled) lives on this side of it, and nothing downstream ever holds a
// parser node. A LaTeX adapter is a sibling package with the same shape and
// no shared machinery until there is something real to share.
//
// The adapter owns no invariants of its own: Survey builds per-file
// inventories and hands them to survey.Assemble, which cross-checks them
// against the source under custody and verifies they tile it.
package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	gmtext "github.com/yuin/goldmark/text"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/survey"
	"kbase/internal/text"
	"kbase/internal/tokens"
)

// Ext is the only extension this adapter reads. Matching is case-insensitive
// (`.MD` off a Windows-authored tree is the same document); `.markdown` and
// friends are deliberately not accepted — a corpus that mixes extensions
// should say so, and a silent near-miss is worse than a loud absence.
//
// Link resolution needs the same constant: a corpus link written without an
// extension names a document only if this is the extension that document
// carries.
const Ext = ".md"

// Extensions is the document-extension set to walk a Markdown corpus with
// (ingest.Walk takes it as a parameter, because which extensions are
// documents is format knowledge and ingest has none).
//
// It is a function returning a fresh slice rather than a package-level var so
// that no caller can quietly redefine what this adapter considers a document.
func Extensions() []string { return []string{Ext} }

// Survey inventories every document in the corpus.
//
// lg carries the decisions the artifact records only the outcome of — which
// blocks were read as metadata, which links resolved by fold.
func Survey(corpus ingest.Corpus, est tokens.Estimator, lg log.Logger) (survey.Artifact, error) {
	// One parser for the whole corpus. goldmark's parser holds no
	// per-document state (each Parse builds its own context), so this is a
	// setup cost paid once rather than per file.
	p := goldmark.DefaultParser()

	files := make([]survey.File, 0, len(corpus.Units))
	for _, u := range corpus.Units {
		files = append(files, surveyFile(p, u, corpus, est, lg))
	}
	return survey.Assemble(corpus, files, lg)
}

// surveyFile inventories one document.
//
// The metadata block is lifted out before parsing rather than parsed as
// Markdown. Plain CommonMark reads a leading `---` as a thematic break and
// the following `key: value` lines as a paragraph — which the closing `---`
// then turns into a setext H2 whose title is the whole YAML block. That is a
// fabricated top-level section in a large share of real doc corpora. The
// block's byte range is recorded so nothing is lost, its labels are read out
// of it, and the parse runs over the remaining bytes with every offset
// rebased into the original file. The source itself is never rewritten — the
// sub-slice is a read-only view handed to the parser.
func surveyFile(p parser.Parser, u ingest.Unit, corpus ingest.Corpus, est tokens.Estimator, lg log.Logger) survey.File {
	src := u.Bytes
	f := survey.File{
		Path:   u.Path,
		SHA256: u.SHA256,
		Bytes:  len(src),
		Tokens: est.EstimateBytes(src),
	}

	bodyStart := 0
	if r, fm, ok := frontMatter(src, u.Path, lg); ok {
		f.Metadata = &r
		f.Title, f.Description, f.Tags = fm.Title, fm.Description, fm.Tags
		bodyStart = r.End
	}
	body := src[bodyStart:]

	doc := p.Parse(gmtext.NewReader(body))
	heads, paras, raws := scan(doc, body, bodyStart)
	g := gister{src: src, body: body, paras: paras, est: est}

	// Everything before the first heading is the preamble — for a file with
	// no headings, that is the whole document.
	firstHeading := len(src)
	if len(heads) > 0 {
		firstHeading = heads[0].start
	}
	if firstHeading > bodyStart {
		f.Preamble = &survey.Section{
			Start:  bodyStart,
			End:    firstHeading,
			Tokens: est.EstimateBytes(src[bodyStart:firstHeading]),
			Gist:   g.forRange(bodyStart, firstHeading),
		}
	}
	for _, n := range buildTree(heads, len(src)) {
		f.Sections = append(f.Sections, g.section(n))
	}
	f.Gist = g.forRange(bodyStart, len(src))
	f.Links = resolveLinks(raws, u.Path, corpus, lg)
	return f
}

// headingRef is a located document-level heading: what it says, what level it
// is, and where its line starts.
type headingRef struct {
	level int
	title string
	start int
}

// paraRef is a paragraph's start offset and node, kept so a gist is rendered
// only for the paragraphs that turn out to lead a section.
//
// The two fields live in DIFFERENT coordinate systems, which the names say
// out loud: srcStart is rebased into the original file (what every artifact
// offset is measured in), while bodyNode's text segments index the
// metadata-stripped body the parser actually saw. Rendering a node against
// the wrong buffer produces plausible text from the wrong place, and no
// tiling check can see it.
type paraRef struct {
	srcStart int
	bodyNode ast.Node
}

// scan walks the parsed document once and collects everything the inventory
// needs. Offsets are rebased by base, so callers index the original file
// rather than the metadata-stripped body.
//
// Only document-level headings define sections. A heading nested inside a
// blockquote or a list item is part of that block's content — promoting it to
// a section boundary would cut a leaf in the middle of a list. A heading with
// no text (`##` alone on a line) is likewise not a boundary: goldmark records
// no source line for it, and a titleless heading gives the taxonomy stage
// nothing to route on.
//
// The walk has no failure mode of its own — the visitor never refuses a node —
// so scan reports none. survey.Assemble's tiling check is where a structural
// mistake surfaces.
func scan(doc ast.Node, body []byte, base int) (heads []headingRef, paras []paraRef, links []rawLink) {
	// The error is discarded, not ignored: ast.Walk only ever returns what
	// the visitor returned, and this visitor returns nil on every path.
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Heading:
			if n.Parent() != doc || t.Lines().Len() == 0 {
				break
			}
			heads = append(heads, headingRef{
				level: t.Level,
				title: text.CapWords(plainText(t, body), survey.WordCap),
				start: base + lineStart(body, t.Lines().At(0).Start),
			})
		case *ast.Paragraph:
			if t.Lines().Len() == 0 {
				break
			}
			paras = append(paras, paraRef{srcStart: base + t.Lines().At(0).Start, bodyNode: n})
		case *ast.Link:
			links = append(links, rawLink{target: string(t.Destination)})
		case *ast.Image:
			links = append(links, rawLink{target: string(t.Destination), image: true})
		case *ast.AutoLink:
			links = append(links, rawLink{target: string(t.URL(body))})
		}
		return ast.WalkContinue, nil
	})
	return heads, paras, links
}

// sectionNode is the heading tree under construction, before token counts and
// gists turn it into the artifact's survey.Section.
type sectionNode struct {
	headingRef
	end      int
	children []*sectionNode
}

// buildTree nests located headings by level and closes each one at the next
// heading of the same or a higher level (docEnd for those still open at the
// end of the file). A skipped level (h1 straight to h3) nests as written
// rather than being corrected — the survey reports the document's structure,
// it does not repair it.
func buildTree(heads []headingRef, docEnd int) []*sectionNode {
	var roots, stack []*sectionNode
	for _, h := range heads {
		n := &sectionNode{headingRef: h}
		for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
			stack[len(stack)-1].end = h.start
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, n)
		}
		stack = append(stack, n)
	}
	for _, n := range stack {
		n.end = docEnd
	}
	return roots
}

// gister turns byte ranges into token counts and gists. It carries the
// paragraph list so a section's gist is its own leading paragraph, resolved
// by offset rather than by re-walking the tree.
type gister struct {
	src   []byte
	body  []byte
	paras []paraRef
	est   tokens.Estimator
}

// section renders one tree node into the artifact's survey.Section,
// recursively.
func (g gister) section(n *sectionNode) survey.Section {
	// A section's gist describes the section itself, so it is drawn from the
	// section's OWN body — the bytes between its heading and its first
	// child. An index-only section (heading, then immediately a subheading)
	// gets no gist rather than borrowing its first child's.
	ownEnd := n.end
	if len(n.children) > 0 {
		ownEnd = n.children[0].start
	}
	s := survey.Section{
		Level:  n.level,
		Title:  n.title,
		Start:  n.start,
		End:    n.end,
		Tokens: g.est.EstimateBytes(g.src[n.start:n.end]),
		Gist:   g.forRange(n.start, ownEnd),
	}
	for _, c := range n.children {
		s.Children = append(s.Children, g.section(c))
	}
	return s
}

// forRange renders the first paragraph starting inside [start, end) as a
// gist, or "" when the range holds no paragraph.
func (g gister) forRange(start, end int) string {
	for _, p := range g.paras {
		if p.srcStart >= end {
			break
		}
		if p.srcStart >= start {
			return text.CapWords(plainText(p.bodyNode, g.body), survey.WordCap)
		}
	}
	return ""
}

// plainText renders a node's inline content as text: link labels and code
// spans included, markup excluded. It walks children directly rather than
// through ast.Walk because it has no failure mode to report.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	appendText(&b, n, src)
	return b.String()
}

func appendText(b *strings.Builder, n ast.Node, src []byte) {
	switch t := n.(type) {
	case *ast.Text:
		b.Write(t.Value(src))
		if t.SoftLineBreak() || t.HardLineBreak() {
			b.WriteByte(' ')
		}
		return
	case *ast.String:
		b.Write(t.Value)
		return
	case *ast.AutoLink:
		b.Write(t.URL(src))
		return
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		appendText(b, c, src)
	}
}

// lineStart returns the offset of the first byte of the line containing at.
// A heading's recorded segment starts at its text, past the `#` marks; a
// section must start at the heading line so that extracting the range yields
// the heading itself.
func lineStart(body []byte, at int) int {
	if at > len(body) {
		at = len(body)
	}
	return bytes.LastIndexByte(body[:at], '\n') + 1
}

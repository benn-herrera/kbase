package survey

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"kbase/internal/ingest"
	"kbase/internal/tokens"
)

// GistWordCap bounds a gist. A gist is a routing hint — enough for the
// taxonomy stage to tell a section about installation from one about
// scripting — and every one of them is paid for in the whole-corpus artifact
// that stage reads, so the cap is deliberately short. It cuts at a word
// boundary and marks the cut: a gist truncated mid-word reads as a typo, and
// one truncated invisibly reads as a complete thought that was not.
const GistWordCap = 40

// gistEllipsis marks a gist the cap cut short.
const gistEllipsis = "…"

// frontMatterDelim opens and closes a YAML front-matter block.
const frontMatterDelim = "---"

// surveyFile inventories one document.
//
// Front matter is skipped before parsing rather than parsed. Plain
// CommonMark reads a leading `---` as a thematic break and the following
// `key: value` lines as a paragraph — which the closing `---` then turns
// into a setext H2 whose title is the whole YAML block. That is a fabricated
// top-level section in a large share of real doc corpora. Detecting the block
// mechanically costs a few lines and no dependency; its byte range is
// recorded so nothing is lost, and the parse runs over the remaining bytes
// with every offset rebased into the original file. The source itself is
// never rewritten — the sub-slice is a read-only view handed to the parser.
func surveyFile(p parser.Parser, u ingest.Unit, corpus ingest.Corpus, est tokens.Estimator) (File, error) {
	src := u.Bytes
	f := File{
		Path:   u.Path,
		SHA256: u.SHA256,
		Bytes:  len(src),
		Tokens: est.Estimate(string(src)),
	}

	bodyStart := 0
	if r, ok := frontMatter(src); ok {
		f.FrontMatter = &r
		bodyStart = r.End
	}
	body := src[bodyStart:]

	doc := p.Parse(text.NewReader(body))
	heads, paras, raws, err := scan(doc, body, bodyStart)
	if err != nil {
		return File{}, fmt.Errorf("survey: %s: %w", u.Path, err)
	}
	g := gister{src: src, body: body, paras: paras, est: est}

	// Everything before the first heading is the preamble — for a file with
	// no headings, that is the whole document.
	firstHeading := len(src)
	if len(heads) > 0 {
		firstHeading = heads[0].start
	}
	if firstHeading > bodyStart {
		f.Preamble = &Section{
			Start:  bodyStart,
			End:    firstHeading,
			Tokens: est.Estimate(string(src[bodyStart:firstHeading])),
			Gist:   g.forRange(bodyStart, firstHeading),
		}
	}
	for _, n := range buildTree(heads, len(src)) {
		f.Sections = append(f.Sections, g.section(n))
	}
	f.Gist = g.forRange(bodyStart, len(src))
	f.Links = resolveLinks(raws, u.Path, corpus)

	if err := verifyTiling(f, len(src)); err != nil {
		return File{}, fmt.Errorf("survey: %s: %w", u.Path, err)
	}
	return f, nil
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
type paraRef struct {
	start int
	node  ast.Node
}

// rawLink is a link destination exactly as written, before classification.
type rawLink struct {
	target string
	image  bool
}

// scan walks the parsed document once and collects everything the inventory
// needs. Offsets are rebased by base, so callers index the original file
// rather than the front-matter-stripped body.
//
// Only document-level headings define sections. A heading nested inside a
// blockquote or a list item is part of that block's content — promoting it to
// a section boundary would cut a leaf in the middle of a list. A heading with
// no text (`##` alone on a line) is likewise not a boundary: goldmark records
// no source line for it, and a titleless heading gives the taxonomy stage
// nothing to route on.
func scan(doc ast.Node, body []byte, base int) (heads []headingRef, paras []paraRef, links []rawLink, err error) {
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
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
				title: collapse(plainText(t, body)),
				start: base + lineStart(body, t.Lines().At(0).Start),
			})
		case *ast.Paragraph:
			if t.Lines().Len() == 0 {
				break
			}
			paras = append(paras, paraRef{start: base + t.Lines().At(0).Start, node: n})
		case *ast.Link:
			links = append(links, rawLink{target: string(t.Destination)})
		case *ast.Image:
			links = append(links, rawLink{target: string(t.Destination), image: true})
		case *ast.AutoLink:
			links = append(links, rawLink{target: string(t.URL(body))})
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("walking the document: %w", err)
	}
	return heads, paras, links, nil
}

// sectionNode is the heading tree under construction, before token counts and
// gists turn it into the artifact's Section.
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

// section renders one tree node into the artifact's Section, recursively.
func (g gister) section(n *sectionNode) Section {
	// A section's gist describes the section itself, so it is drawn from the
	// section's OWN body — the bytes between its heading and its first
	// child. An index-only section (heading, then immediately a subheading)
	// gets no gist rather than borrowing its first child's.
	ownEnd := n.end
	if len(n.children) > 0 {
		ownEnd = n.children[0].start
	}
	s := Section{
		Level:  n.level,
		Title:  n.title,
		Start:  n.start,
		End:    n.end,
		Tokens: g.est.Estimate(string(g.src[n.start:n.end])),
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
		if p.start >= end {
			break
		}
		if p.start >= start {
			return capWords(plainText(p.node, g.body))
		}
	}
	return ""
}

// resolveLinks classifies each distinct destination and sorts the result.
// Sorting by target (images after links of the same target) makes the
// artifact diffable between runs and independent of the order the walk
// happened to reach equivalent references in.
//
// An empty destination (`[text]()`) is dropped: it names nothing, so there is
// no edge to record and nothing a later stage could act on.
func resolveLinks(raws []rawLink, from string, corpus ingest.Corpus) []Link {
	seen := make(map[rawLink]bool, len(raws))
	out := make([]Link, 0, len(raws))
	for _, r := range raws {
		if r.target == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, classifyLink(r, from, corpus))
	}
	slices.SortFunc(out, func(a, b Link) int {
		if c := strings.Compare(a.Target, b.Target); c != 0 {
			return c
		}
		switch {
		case a.Image == b.Image:
			return 0
		case a.Image:
			return 1
		default:
			return -1
		}
	})
	return out
}

// classifyLink decides where one destination points.
func classifyLink(r rawLink, from string, corpus ingest.Corpus) Link {
	l := Link{Target: r.target, Image: r.image}
	u, err := url.Parse(r.target)
	if err != nil {
		// Not a URL at all. It cannot be resolved and it is certainly not a
		// reachable external reference, so it is recorded as the corpus's
		// problem rather than silently classified away.
		l.Kind = LinkUnresolved
		return l
	}
	l.Fragment = u.Fragment
	switch {
	case u.Scheme != "" || u.Host != "":
		l.Kind = LinkExternal
	case u.Path == "":
		l.Kind = LinkAnchor
	default:
		if target, ok := resolveTarget(u.Path, from, corpus); ok {
			l.Kind, l.Path = LinkInternal, target
		} else {
			l.Kind = LinkUnresolved
		}
	}
	return l
}

// resolveTarget maps a link path to a corpus document id. A rooted path is
// resolved against the corpus root, anything else against the linking file's
// directory.
//
// The extensionless retry is there because doc sites routinely link a sibling
// document by name alone (`[scope](scope)`) and leave the extension to the
// site generator. Two lookups, no guessing beyond the corpus's own file
// names: a target that matches nothing stays unresolved.
func resolveTarget(p, from string, corpus ingest.Corpus) (string, bool) {
	var target string
	if strings.HasPrefix(p, "/") {
		target = path.Clean(strings.TrimPrefix(p, "/"))
	} else {
		target = path.Join(path.Dir(from), p)
	}
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", false // escapes the corpus root
	}
	if corpus.Has(target) {
		return target, true
	}
	if path.Ext(target) == "" && corpus.Has(target+ingest.MarkdownExt) {
		return target + ingest.MarkdownExt, true
	}
	return "", false
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

// collapse reduces every run of whitespace to a single space and trims the
// ends, so a title wrapped across source lines is one line in the artifact.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// capWords collapses whitespace and enforces GistWordCap.
func capWords(s string) string {
	words := strings.Fields(s)
	if len(words) > GistWordCap {
		return strings.Join(words[:GistWordCap], " ") + gistEllipsis
	}
	return strings.Join(words, " ")
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

// frontMatter reports the byte range of a leading YAML front-matter block:
// a `---` line at the very start of the file, up to and including the next
// `---` line. Without a closing delimiter there is no block — an unterminated
// one is a thematic break followed by prose, which is what CommonMark says it
// is.
func frontMatter(src []byte) (Range, bool) {
	first, next := readLine(src, 0)
	if !isFrontMatterDelim(first) {
		return Range{}, false
	}
	for off := next; off < len(src); {
		line, after := readLine(src, off)
		if isFrontMatterDelim(line) {
			return Range{Start: 0, End: after}, true
		}
		off = after
	}
	return Range{}, false
}

// readLine returns the line at off without its terminator, and the offset of
// the next line (len(src) at the last line, so callers always advance).
func readLine(src []byte, off int) ([]byte, int) {
	i := bytes.IndexByte(src[off:], '\n')
	if i < 0 {
		return src[off:], len(src)
	}
	return src[off : off+i], off + i + 1
}

func isFrontMatterDelim(line []byte) bool {
	return string(bytes.TrimRight(line, " \t\r")) == frontMatterDelim
}

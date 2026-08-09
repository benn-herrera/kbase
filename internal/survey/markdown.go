package survey

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	"kbase/internal/ingest"
	"kbase/internal/log"
	"kbase/internal/tokens"
)

// gistWordCap bounds a gist, and every other string this package copies into
// the artifact. A gist is a routing hint — enough for the taxonomy stage to
// tell a section about installation from one about scripting — and every one
// of them is paid for in the whole-corpus artifact that stage reads, so the
// cap is deliberately short. It cuts at a word boundary and marks the cut: a
// gist truncated mid-word reads as a typo, and one truncated invisibly reads
// as a complete thought that was not.
const gistWordCap = 40

// gistEllipsis marks a gist the cap cut short.
const gistEllipsis = "…"

// frontMatterDelim opens and closes a YAML front-matter block.
const frontMatterDelim = "---"

// surveyFile inventories one document.
//
// Front matter is lifted out before parsing rather than parsed as Markdown.
// Plain CommonMark reads a leading `---` as a thematic break and the
// following `key: value` lines as a paragraph — which the closing `---` then
// turns into a setext H2 whose title is the whole YAML block. That is a
// fabricated top-level section in a large share of real doc corpora. The
// block's byte range is recorded so nothing is lost, its labels are read out
// of it, and the parse runs over the remaining bytes with every offset
// rebased into the original file. The source itself is never rewritten — the
// sub-slice is a read-only view handed to the parser.
func surveyFile(p parser.Parser, u ingest.Unit, corpus ingest.Corpus, est tokens.Estimator, lg log.Logger) (File, error) {
	src := u.Bytes
	f := File{
		Path:   u.Path,
		SHA256: u.SHA256,
		Bytes:  len(src),
		Tokens: est.EstimateBytes(src),
	}

	bodyStart := 0
	if r, fm, ok := frontMatter(src, u.Path, lg); ok {
		f.FrontMatter = &r
		f.Title, f.Description, f.Tags = fm.Title, fm.Description, fm.Tags
		bodyStart = r.End
	}
	body := src[bodyStart:]

	doc := p.Parse(text.NewReader(body))
	heads, paras, raws := scan(doc, body, bodyStart)
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
			Tokens: est.EstimateBytes(src[bodyStart:firstHeading]),
			Gist:   g.forRange(bodyStart, firstHeading),
		}
	}
	for _, n := range buildTree(heads, len(src)) {
		f.Sections = append(f.Sections, g.section(n))
	}
	f.Gist = g.forRange(bodyStart, len(src))
	f.Links = resolveLinks(raws, u.Path, corpus, lg)

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
//
// The two fields live in DIFFERENT coordinate systems, which the names say
// out loud: srcStart is rebased into the original file (what every artifact
// offset is measured in), while bodyNode's text segments index the
// front-matter-stripped body the parser actually saw. Rendering a node
// against the wrong buffer produces plausible text from the wrong place, and
// no tiling check can see it.
type paraRef struct {
	srcStart int
	bodyNode ast.Node
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
//
// The walk has no failure mode of its own — the visitor never refuses a node —
// so scan reports none. verifyTiling is where a structural mistake surfaces.
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
				title: capWords(plainText(t, body)),
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
			return capWords(plainText(p.bodyNode, g.body))
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
func resolveLinks(raws []rawLink, from string, corpus ingest.Corpus, lg log.Logger) []Link {
	seen := make(map[rawLink]bool, len(raws))
	out := make([]Link, 0, len(raws))
	for _, r := range raws {
		if r.target == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, classifyLink(r, from, corpus, lg))
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
func classifyLink(r rawLink, from string, corpus ingest.Corpus, lg log.Logger) Link {
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
		if target, ok := resolveTarget(u.Path, from, corpus, lg); ok {
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
// Two retries follow the exact lookup, in decreasing confidence:
//
//   - Extensionless. Doc sites routinely link a sibling by name alone
//     (`[scope](scope)`) and leave the extension to the site generator.
//   - Case-folded. Ingest accepts `.MD` because casing conveys nothing about
//     a document (ingest.MarkdownExt), so resolution has to agree — otherwise
//     every link into a Windows-authored file inflates the unresolved count.
//     A fold that several documents answer to is ambiguous: it is reported as
//     unresolved and warned about, never guessed at.
//
// The id returned is always the corpus's own byte-exact path. No target is
// invented: anything the corpus does not hold stays unresolved.
func resolveTarget(p, from string, corpus ingest.Corpus, lg log.Logger) (string, bool) {
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

	candidates := []string{target}
	if path.Ext(target) == "" {
		withExt := target + ingest.MarkdownExt
		if corpus.Has(withExt) {
			lg.Debug("survey resolved an extensionless link", "from", from, "target", p, "path", withExt)
			return withExt, true
		}
		candidates = append(candidates, withExt)
	}
	for _, c := range candidates {
		id, ambiguous := corpus.FoldedPath(c)
		switch {
		case ambiguous:
			lg.Warn("survey link folds to several documents; leaving it unresolved",
				"from", from, "target", p, "folded", c)
			return "", false
		case id != "":
			lg.Debug("survey resolved a link by case fold", "from", from, "target", p, "path", id)
			return id, true
		}
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

// capWords is the one normalization every copied string in the artifact goes
// through: whitespace runs collapse to a single space (a title wrapped across
// source lines is one line here), and the result is cut to gistWordCap words
// with the cut marked.
//
// Titles run through it as well as gists. A title is paid for in the same
// whole-corpus artifact and is bounded by nothing in the source — a setext
// heading's title is the entire paragraph above the underline, so one
// 300-word paragraph would otherwise buy a 300-word title. Truncating is safe
// because a title is a label; the offsets it labels are untouched.
func capWords(s string) string {
	words := strings.Fields(s)
	if len(words) > gistWordCap {
		return strings.Join(words[:gistWordCap], " ") + gistEllipsis
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

// frontMatterFields are the labels a front-matter mapping carries. They are
// in the artifact because ARCHITECTURE.md §4 stage 3 consumes the survey and
// never the raw source: in a corpus whose H1 lives in front matter and whose
// body opens with prose — exactly the convention this branch exists because
// of — a file with no title here surveys as a path and a gist, which is thin
// material for designing a hierarchy out of.
//
// Unknown keys are ignored rather than recorded. Front matter is a site
// generator's configuration; the taxonomy stage needs the labels a human
// would recognize the document by, not `sidebar_position`.
type frontMatterFields struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

// normalized puts the values through the artifact's one string rule and drops
// tags that carry nothing, so a `tags: [a, "", b]` block does not spend an
// artifact slot on an empty string.
func (f frontMatterFields) normalized() frontMatterFields {
	f.Title, f.Description = capWords(f.Title), capWords(f.Description)
	tags := make([]string, 0, len(f.Tags))
	for _, t := range f.Tags {
		if t = capWords(t); t != "" {
			tags = append(tags, t)
		}
	}
	f.Tags = nil
	if len(tags) > 0 {
		f.Tags = tags
	}
	return f
}

// bomUTF8 is the byte-order mark editors on Windows put at the head of a
// UTF-8 file.
var bomUTF8 = []byte{0xEF, 0xBB, 0xBF}

// frontMatter reports the byte range of a leading YAML front-matter block and
// the labels it carries: a `---` line at the start of the file, up to and
// including the next `---` line, whose contents parse as a YAML mapping.
//
// Three things it deliberately is not:
//
//   - Anchored at byte 0. A leading BOM is skipped before the delimiter
//     check — with it in the way, line 1 is not a thematic break and
//     CommonMark reads the whole YAML block as a setext H2 title, which is
//     precisely the misparse this function exists to prevent. The BOM is
//     folded INTO the recorded range rather than trimmed off it, so the
//     range still starts at 0 and the file's ranges still tile it exactly.
//   - Satisfied by delimiters alone. See parseFrontMatter.
//   - Willing to run past a missing close. Without a closing delimiter there
//     is no block: an unterminated one is a thematic break followed by prose,
//     which is what CommonMark says it is.
func frontMatter(src []byte, from string, lg log.Logger) (Range, frontMatterFields, bool) {
	start := 0
	if bytes.HasPrefix(src, bomUTF8) {
		start = len(bomUTF8)
	}
	first, next := readLine(src, start)
	if !isFrontMatterDelim(first) {
		return Range{}, frontMatterFields{}, false
	}
	for off := next; off < len(src); {
		line, after := readLine(src, off)
		if !isFrontMatterDelim(line) {
			off = after
			continue
		}
		fields, ok := parseFrontMatter(src[next:off])
		if !ok {
			// Not a rejection of the file — a decision that these bytes are
			// content. They go back to the Markdown parse, where the headings
			// and links inside them still count.
			lg.Debug("survey read a delimited block as content, not front matter",
				"file", from, "end", after, "reason", "not a YAML mapping")
			return Range{}, frontMatterFields{}, false
		}
		lg.Debug("survey detected front matter", "file", from, "end", after,
			"titled", fields.Title != "", "tags", len(fields.Tags))
		return Range{Start: 0, End: after}, fields, true
	}
	return Range{}, frontMatterFields{}, false
}

// parseFrontMatter decides whether a delimited block really is front matter,
// and lifts its labels out if so.
//
// The rule is exact rather than heuristic: front matter is a YAML MAPPING, or
// it is empty. A document that opens with a thematic break and prose — `---`,
// a paragraph, another `---` used as a section rule — parses as a scalar, not
// a mapping, so it is content; without this gate its headings, links and
// gists would vanish into an opaque front-matter range and no tiling check
// would notice, because an opaque range tiles perfectly well. What survives
// is the case where the opening prose genuinely reads `key: value`, which a
// human would call ambiguous too.
func parseFrontMatter(block []byte) (frontMatterFields, bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return frontMatterFields{}, false
	}
	// An empty block (`---` immediately followed by `---`) parses to no node
	// at all — the zero Kind. It is front matter carrying nothing, a shape
	// real corpora have, and nothing is lost by recording it as such.
	if doc.Kind == 0 {
		return frontMatterFields{}, true
	}
	node := &doc
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return frontMatterFields{}, true
		}
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return frontMatterFields{}, false
	}

	var f frontMatterFields
	if err := node.Decode(&f); err != nil {
		// The block is already known to be a mapping, so the only failure
		// left is a key whose value is the wrong shape (`tags: sometimes`).
		// yaml fills in what it could, and the survey keeps that: a
		// mis-shaped optional label is not grounds for throwing away a block
		// both YAML and a human read as front matter.
		var typeErr *yaml.TypeError
		if !errors.As(err, &typeErr) {
			return frontMatterFields{}, false
		}
	}
	return f.normalized(), true
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

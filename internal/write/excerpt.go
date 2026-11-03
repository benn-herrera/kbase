package write

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"kbase/internal/index"
	"kbase/internal/kb"
)

// The search a locator is resolved by. Stripping and folding are
// search-side only; every answer names a physical line of the document as
// it stands.
var (
	markupRE       = regexp.MustCompile(`<!--.*?-->|</?[A-Za-z][^<>]*>`)
	markdownLinkRE = regexp.MustCompile(`\[([^\[\]]*)\]\([^()]*\)`)
	spaceRunRE     = regexp.MustCompile(`  +`)
)

// searchBody is a document's body below its frontmatter block as one
// string: each line unquoted and collapsed, joined by single spaces, with
// where each line starts.
type searchBody struct {
	text   string
	starts []int
	first  int
}

func bodyOf(document string) searchBody {
	b := searchBody{}
	if m := kb.FindFrontmatter(document); m != nil {
		b.first = strings.Count(document[:m[1]], "\n") + 1
	}
	lines := kb.SplitLines(document)
	if b.first > len(lines) {
		return b
	}
	var parts []string
	cursor := 0
	for _, line := range lines[b.first:] {
		collapsed := collapse(kb.BlockquotePrefix.ReplaceAllString(line, ""))
		b.starts = append(b.starts, cursor)
		parts = append(parts, collapsed)
		cursor += len(collapsed) + 1
	}
	b.text = strings.Join(parts, " ")
	return b
}

// lineAt is the document line holding offset of the body text.
func (b searchBody) lineAt(offset int) int {
	return b.first + sort.Search(len(b.starts), func(i int) bool { return b.starts[i] > offset }) - 1
}

func occurrences(haystack, needle string) []int {
	var out []int
	for at := 0; at <= len(haystack); {
		i := strings.Index(haystack[at:], needle)
		if i < 0 {
			break
		}
		out = append(out, at+i)
		_, size := utf8.DecodeRuneInString(haystack[at+i:])
		at += i + max(size, 1)
	}
	return out
}

func flagged(re *regexp.Regexp, text string) []bool {
	flags := make([]bool, len(text))
	for _, m := range re.FindAllStringIndex(text, -1) {
		for i := m[0]; i < m[1]; i++ {
			flags[i] = true
		}
	}
	return flags
}

// fold is text with markup dropped, every alphanumeric lowercased and every
// other run a single space — two where the run held two spaces, the mark of
// a paragraph break — and, per byte of the result, the offset in text it
// came from.
func fold(text string) (string, []int) {
	dropped := flagged(markupRE, text)
	for _, m := range markdownLinkRE.FindAllStringSubmatchIndex(text, -1) {
		dropped[m[0]] = true
		for i := m[3]; i < m[1]; i++ {
			dropped[i] = true
		}
	}
	barrier := flagged(spaceRunRE, text)
	var out strings.Builder
	var index []int
	pending := 0
	for at, r := range text {
		if dropped[at] {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			width := 1
			if barrier[at] {
				width = 2
			}
			pending = max(pending, width)
			continue
		}
		if pending > 0 && out.Len() > 0 {
			for range pending {
				out.WriteByte(' ')
				index = append(index, at)
			}
		}
		pending = 0
		lowered := strings.ToLower(string(r))
		out.WriteString(lowered)
		for range len(lowered) {
			index = append(index, at)
		}
	}
	return out.String(), index
}

// excerptLines is every document line excerpt begins on: the strict search
// over the collapsed body, else, only where that finds nothing, the folded
// search.
func excerptLines(document, excerpt string) []int {
	b := bodyOf(document)
	if len(b.starts) == 0 {
		return nil
	}
	var lines []int
	if strict := occurrences(b.text, collapse(excerpt)); len(strict) > 0 {
		for _, at := range strict {
			lines = append(lines, b.lineAt(at))
		}
		return lines
	}
	folded, index := fold(b.text)
	wanted, _ := fold(collapse(excerpt))
	if wanted == "" {
		return nil
	}
	var seen []int
	for _, at := range occurrences(folded, wanted) {
		if offset := index[at]; !slices.Contains(seen, offset) {
			seen = append(seen, offset)
			lines = append(lines, b.lineAt(offset))
		}
	}
	return lines
}

// ExcerptLines is excerptLines, for the build's placement of the markers it
// writes.
func ExcerptLines(document, excerpt string) []int { return excerptLines(document, excerpt) }

// CanonicalForm is text as the folded search reduces it: markup-free,
// lowercased, its words separated by whitespace.
func CanonicalForm(text string) string {
	folded, _ := fold(text)
	return folded
}

// readsAsOneLink is whether citation is, whole, the one link the toolchain's
// link reader sees in it, to dest: an unbalanced [ or ] in its text, or
// whitespace or ) in dest, hides the citation or reads a shorter link inside
// it.
func readsAsOneLink(citation, dest string) bool {
	m := kb.LinkRE.FindAllStringSubmatchIndex(citation, -1)
	return len(m) == 1 && m[0][0] == 0 && m[0][1] == len(citation) && citation[m[0][2]:m[0][3]] == dest
}

// addCitation replaces the one span of the named line that its snippet names
// with a citation of the node its id names, linked to the node's target
// relative to the citing document: the quote form where the snippet is quoted
// from the node's statement, the paraphrase form where quote is, and the
// naming form, the node's title its link text, where the snippet is the id
// itself — the one form a work or an axiom takes.
func addCitation(b *recordBatch, e entry) error {
	id, snippet, line := e.str(editID.name), e.str("snippet"), e.count("line")
	kind, ok := index.GraphKinds(b.g)[id]
	if !ok {
		return noNode(e.Op, editID.name, id)
	}
	subject, l, err := b.citingDocument(e.str("document"))
	if err != nil {
		return err
	}
	at, err := citedSpan(subject, l.text, line, snippet)
	if err != nil {
		return err
	}
	naming := snippet == id
	if !naming && (kind == kb.NodeKindWork || kind == kb.NodeKindAxiom) {
		return opRefusal("snippet", fmt.Sprintf("%s is a %s, which has no statement to quote or paraphrase", id, kind),
			"restore: write "+id+" where the reference belongs and give it as snippet, for the naming form, then re-run")
	}
	to, err := b.targetOf(id)
	if err != nil {
		return err
	}
	if to.path == "" {
		return noStatementMarker(id, "a citation of it has nowhere to link")
	}
	dest := index.RelPosix(to.path, path.Dir(subject))
	if to.anchor != "" {
		dest += "#" + to.anchor
	}
	text := `"` + snippet + `"`
	switch {
	case naming:
		text = b.titleOf(kind, id)
	case e.has("quote"):
		text = snippet
		err = b.checkQuote(to.path, id, "quote", e.str("quote"))
	default:
		err = b.checkQuote(to.path, id, "snippet", snippet)
	}
	if err != nil {
		return err
	}
	link := kb.Citation(text, dest, id)
	if !readsAsOneLink(link, dest) {
		key, what, restore := "snippet", "snippet", "restore: name a span without an unbalanced bracket, then re-run"
		if naming {
			key, what, restore = editID.name, id+"'s title", "restore: balance the brackets of "+id+"'s title with "+editOps[kind]+", then re-run"
		}
		return opRefusal(key, fmt.Sprintf("composes %s, which the toolchain's link reader does not see as one link to %s — an "+
			"unbalanced '[' or ']' in the %s hides the citation from every gate that reads links", link, dest, what), restore)
	}
	before := b.leafTexts()
	raw := kb.SplitLines(l.text)[line-1]
	l.text = spliceLines(l.text, line-1, line, []string{raw[:at] + link + raw[at+len(snippet):]})
	moved := b.movedHeadings(before)
	b.report = entryReport{ID: id, Document: subject, Line: line, Link: kb.Citation(text, moved.dest(subject, dest), id)}
	if len(moved) == 0 {
		return nil
	}
	return b.repoint(func(rel string, l kb.TextLink) string { return moved.dest(rel, l.Destination) })
}

// titleOf is the title of node id, of kind.
func (b *recordBatch) titleOf(kind, id string) string {
	for _, n := range b.nodesOf(kind) {
		if n.id == id {
			return *n.title
		}
	}
	return ""
}

// citingDocument is the document of the KB document names, as the batch
// edits it: refused where it is none, a summary page among them.
func (b *recordBatch) citingDocument(document string) (string, *leafEdit, error) {
	subject, err := b.subject(document)
	if err != nil {
		return "", nil, err
	}
	documents, err := b.knownDocuments()
	if err != nil {
		return "", nil, err
	}
	if !slices.Contains(documents, subject) {
		return "", nil, opRefusal("document", fmt.Sprintf("%q is not a document of the KB, and a citation is written into "+
			"one; a summary page, which kbase renders, is none", subject),
			"restore: correct document to the kb-root-relative path of the document holding the citing text, then re-run")
	}
	l, opened := b.leaves[subject]
	if !opened {
		if l, err = b.open(subject); err != nil {
			return "", nil, err
		}
		b.edits(subject, l)
	}
	return subject, l, nil
}

// citedSpan is the byte offset of the one span of the 1-based line of text,
// the document at subject, that snippet names. It is refused where the line
// lies past the end, where no span outside code, links and statement markers
// matches, naming the nearby lines where one does, and where more than one
// does, naming each column.
func citedSpan(subject, text string, line int, snippet string) (int, error) {
	lines, views := kb.SplitLines(text), kb.SplitLines(kb.StripCodeSplitLines(text))
	if line > len(lines) {
		return 0, lineOutOfRange(subject, line, len(lines))
	}
	spans := func(i int) []int { return citableSpans(lines[i], views[i], snippet) }
	hits := spans(line - 1)
	switch {
	case len(hits) == 0:
		return 0, snippetAbsent(subject, line, len(lines), snippet, " outside code, links and statement markers",
			func(i int) bool { return len(spans(i)) > 0 })
	case len(hits) > 1:
		columns := make([]int, len(hits))
		for i, at := range hits {
			columns[i] = utf8.RuneCountInString(lines[line-1][:at]) + 1
		}
		return 0, opRefusal("snippet", fmt.Sprintf("%q occurs %d times on line %d of %s, at columns %s, and the citation "+
			"replaces one span, which kbase does not guess", snippet, len(hits), line, subject, joinInts(columns)),
			"restore: extend snippet until it names one span of the line, then re-run")
	}
	return hits[0], nil
}

// citableSpans is the byte offset of every occurrence of snippet in raw, a
// line whose code is blanked in view, that lies outside code, outside every
// link and outside every statement marker.
func citableSpans(raw, view, snippet string) []int {
	var spans [][2]int
	for _, l := range kb.Links(view) {
		spans = append(spans, [2]int{l.Start, l.End})
	}
	for _, m := range kb.LineMarkers(raw) {
		spans = append(spans, [2]int{m.Start, m.End})
	}
	var out []int
	for _, at := range occurrences(raw, snippet) {
		end := at + len(snippet)
		if view[at:end] != raw[at:end] || slices.ContainsFunc(spans, func(s [2]int) bool { return at < s[1] && s[0] < end }) {
			continue
		}
		out = append(out, at)
	}
	return out
}

// quoteRemedy is how a quote given at key that is not verbatim from the
// statement of id in host is put right: quote is re-read and given verbatim;
// a snippet given without quote, which must already stand on the line, takes
// a quote, the naming form or a rewritten line.
func quoteRemedy(key, id, host string) string {
	if key == "quote" {
		return "restore: re-read the statement of " + id + " in " + host + " and give quote verbatim from it, then re-run"
	}
	return "restore: give quote, the passage of the statement of " + id + " in " + host + " your snippet paraphrases; or write " + id +
		" in the line and give it as snippet, for the naming form; or rewrite the line so snippet quotes the statement verbatim; then re-run"
}

// checkQuote refuses, at key, a quote that does not appear verbatim, whitespace
// collapsed, in the statement of node id in its host leaf.
func (b *recordBatch) checkQuote(host, id, key, quote string) error {
	l, err := b.leafAt(host)
	if err != nil {
		return err
	}
	if strings.Contains(collapse(kb.Statement(l.text, id, b.lifts)), collapse(quote)) {
		return nil
	}
	detail := fmt.Sprintf("%q does not appear in the statement of %s in %s, and a quote is verbatim from the cited statement", quote, id, host)
	if key == "snippet" {
		detail = fmt.Sprintf("%q does not appear in the statement of %s in %s, and a snippet given without quote is quoted verbatim from the statement",
			quote, id, host)
	}
	return opRefusal(key, detail, quoteRemedy(key, id, host))
}

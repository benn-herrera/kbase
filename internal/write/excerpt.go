package write

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
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

// ExcerptLines is every document line excerpt begins on, by the matching
// mark-claim-in-leaf locates a marker with — so a caller pre-checking a
// locator asks the op's own question.
func ExcerptLines(document, excerpt string) []int { return excerptLines(document, excerpt) }

// CanonicalForm is text as the folded search reduces it: markup-free,
// lowercased, its words separated by whitespace.
func CanonicalForm(text string) string {
	folded, _ := fold(text)
	return folded
}

// locateExcerpt is the one line locator names; absent and ambiguous are two
// refusals, since a marker at the wrong one of two matches binds silently.
func locateExcerpt(document, locator string) (int, error) {
	hits := excerptLines(document, locator)
	switch {
	case len(hits) == 0:
		return 0, opRefusal("locator", fmt.Sprintf("%q does not appear in the document body. The locator is matched against "+
			"the body's whitespace-collapsed text, and where that finds nothing against a folded form of it — so wrapping, "+
			"markup, case and punctuation do not matter, but the words themselves do, and so does the paragraph they sit in", locator),
			"restore: quote the locator text verbatim from one paragraph of the document body and re-run")
	case len(hits) > 1:
		return 0, opRefusal("locator", fmt.Sprintf("%q matches %d places in the document body, so the marker's position "+
			"would be chosen arbitrarily", locator, len(hits)),
			"restore: extend the locator until it names exactly one place, then re-run")
	}
	return hits[0], nil
}

// nearestWindow is the window of haystack a failed match came closest to,
// anchored on the longest run the two share, or "" where that run is too
// short to be a quotation of anything.
func nearestWindow(needle, haystack string) string {
	a, b := []rune(needle), []rune(haystack)
	bestLen, bestA, bestB := 0, 0, 0
	prev := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		for j := 1; j <= len(b); j++ {
			if a[i-1] != b[j-1] {
				continue
			}
			cur[j] = prev[j-1] + 1
			sa, sb := i-cur[j], j-cur[j]
			if cur[j] > bestLen || (cur[j] == bestLen && (sa < bestA || (sa == bestA && sb < bestB))) {
				bestLen, bestA, bestB = cur[j], sa, sb
			}
		}
		prev = cur
	}
	if bestLen < max(12, len(a)/3) {
		return ""
	}
	start := max(0, bestB-bestA)
	return string(b[start:min(len(b), start+len(a))])
}

// renderCitations composes each entry's citation after checking what the
// citation gate will check of it, all or none.
func renderCitations(src *kb.Source, entries []entry) Result {
	res := Result{Outcome: result.Done}
	for _, e := range entries {
		citation, err := composeCitation(src, e)
		if err != nil {
			return errorResult(err)
		}
		res.Citations = append(res.Citations, citation)
	}
	return res
}

func composeCitation(src *kb.Source, e entry) (string, error) {
	root := kb.ResolvePath(src.Root())
	citingRel, citedRel := e.str("citing-document"), e.str("cited-document")
	anchor, excerpt := collapse(e.str("anchor")), e.str("excerpt")
	citing, err := containedIn(root, citingRel, "citing-document")
	if err != nil {
		return "", err
	}
	cited, err := containedIn(root, citedRel, "cited-document")
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(filepath.Dir(citing)); err != nil || !info.IsDir() {
		return "", opRefusal("citing-document", fmt.Sprintf("%q names a directory that does not exist under kb-root/. The "+
			"link is rendered relative to the citing document, so its directory decides the target's spelling — the file "+
			"itself may still be unwritten", citingRel),
			"restore: correct citing-document to the path this citation will be written into, then re-run")
	}
	if !src.IsFile(cited) {
		return "", opRefusal("cited-document", fmt.Sprintf("%q does not resolve to a file. A citation names a durable KB "+
			"path that exists at the moment it is written", citedRel),
			"restore: correct cited-document, or cite a document that has been written, then re-run")
	}
	data, err := src.ReadFile(cited)
	if err != nil {
		return "", fmt.Errorf("cited-document %q exists but could not be read: %w", citedRel, err)
	}
	if !utf8.Valid(data) {
		return "", opRefusal("cited-document", fmt.Sprintf("%q is not valid UTF-8, so the excerpt cannot be checked against it", citedRel),
			"restore: cite a document that decodes as UTF-8, or re-encode "+citedRel+", then re-run")
	}
	body, ok := kb.AnchorSection(kb.TranslateNewlines(string(data)), anchor)
	if !ok {
		return "", opRefusal("anchor", fmt.Sprintf("%q names no section of %s. An anchor is a heading's slug, and a "+
			"heading that exists only inside a fenced example is not a section", anchor, citedRel),
			"restore: correct anchor to the slug of a heading in "+citedRel+", then re-run")
	}
	needle, haystack := collapse(excerpt), collapse(body)
	if !strings.Contains(haystack, needle) {
		found := "no comparable text appears there"
		if window := nearestWindow(needle, haystack); window != "" {
			found = fmt.Sprintf("the nearest text there reads %q", window)
		}
		return "", opRefusal("excerpt", fmt.Sprintf("does not appear at %s#%s — %s. An excerpt is quoted verbatim from "+
			"the section it cites, compared with whitespace collapsed and fenced examples removed", citedRel, anchor, found),
			fmt.Sprintf("restore: quote the clause verbatim from %s's %q section, then re-run", citedRel, anchor))
	}
	linkTarget := index.RelPosix(citedRel, path.Dir(citingRel))
	citation := renderCitation(excerpt, linkTarget, anchor)
	var targets []string
	for _, m := range kb.LinkRE.FindAllStringSubmatch(citation, -1) {
		targets = append(targets, m[1])
	}
	if !slices.Equal(targets, []string{linkTarget + "#" + anchor}) {
		return "", opRefusal("excerpt", fmt.Sprintf("composes a citation the toolchain's link reader does not see as one "+
			"link to %s#%s — an unbalanced '[' or ']' in the excerpt, or whitespace or ')' in the anchor, hides the whole "+
			"citation from every gate that reads links", linkTarget, anchor),
			"restore: re-quote the excerpt without an unbalanced bracket, correct the anchor, then re-run")
	}
	return citation, nil
}

package kb

import (
	"cmp"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// bibliographyEntryRE is one rendered reference-list entry: pandoc's csl-entry
// div, its id the citation key behind "ref-".
var bibliographyEntryRE = regexp.MustCompile(`(?s)<div id="ref-([^"]+)"[^>]*>(.*?)</div>`)

// CitationSpanRE is one rendered citation span, pandoc's: its citation keys,
// space-separated, under the group keys and its rendering under rendering.
var CitationSpanRE = PyRE(`(?s)<span\s+class="citation"\s+data-cites="(?P<keys>[^"]*)"\s*>(?P<rendering>.*?)</span>`)

// BibliographyEntry is one entry of a rendered reference list: its citation
// key and its body as written between the div's tags.
type BibliographyEntry struct {
	Key, Body string
}

// BibliographyEntries is every rendered reference-list entry of text, in
// document order.
func BibliographyEntries(text string) []BibliographyEntry {
	matches := bibliographyEntryRE.FindAllStringSubmatch(text, -1)
	out := make([]BibliographyEntry, len(matches))
	for i, m := range matches {
		out[i] = BibliographyEntry{Key: m[1], Body: m[2]}
	}
	return out
}

// The uses a document makes of a citation key as a key.
const (
	// KeyInSpan is a key token of a citation span's data-cites.
	KeyInSpan = "span"
	// KeyInRendering is a key of a citation span whose text is the key-only
	// rendering, its keys parenthesised and joined by "; ".
	KeyInRendering = "rendering"
	// KeyInEntry is a reference-list entry's id, ref-<key>.
	KeyInEntry = "entry"
	// KeyInLink is a link's #ref-<key> fragment.
	KeyInLink = "link"
)

// refFragment opens the fragment of a link into a reference-list entry.
const refFragment = "#ref-"

var (
	keyTokenRE          = regexp.MustCompile(`\S+`)
	renderingKeyTokenRE = regexp.MustCompile(`[^\s;()]+`)
)

// CitationKeyRef is one use of a citation key as a key in a document: the
// key, its use (KeyInSpan, KeyInRendering, KeyInEntry or KeyInLink), its
// 0-based line and its byte offsets on the raw line.
type CitationKeyRef struct {
	Key, Use   string
	Line       int
	Start, End int
}

// CitationKeyRefs is every use of a citation key as a key in text outside
// code, in text order. A span or an entry may run over several lines; a
// blockquote's prefixes are read past, so one wrapped inside a quote is read
// whole.
func CitationKeyRefs(text string) []CitationKeyRef {
	lines := SplitLines(StripCodeSplitLines(text))
	starts := make([]int, len(lines))
	at := 0
	for i, l := range lines {
		if p := BlockquotePrefix.FindString(l); p != "" {
			lines[i] = strings.Repeat(" ", len(p)) + l[len(p):]
		}
		starts[i] = at
		at += len(lines[i]) + 1
	}
	view := strings.Join(lines, "\n")
	var out []CitationKeyRef
	add := func(key, use string, offset int) {
		line := sort.Search(len(starts), func(i int) bool { return starts[i] > offset }) - 1
		col := offset - starts[line]
		out = append(out, CitationKeyRef{Key: key, Use: use, Line: line, Start: col, End: col + len(key)})
	}
	keys, rendering := 2*CitationSpanRE.SubexpIndex("keys"), 2*CitationSpanRE.SubexpIndex("rendering")
	for _, m := range CitationSpanRE.FindAllStringSubmatchIndex(view, -1) {
		var spanKeys []string
		for _, t := range keyTokenRE.FindAllStringIndex(view[m[keys]:m[keys+1]], -1) {
			key := view[m[keys]+t[0] : m[keys]+t[1]]
			spanKeys = append(spanKeys, key)
			add(key, KeyInSpan, m[keys]+t[0])
		}
		text := view[m[rendering]:m[rendering+1]]
		if len(spanKeys) == 0 || NormalizeSpace(text) != "("+strings.Join(spanKeys, "; ")+")" {
			continue
		}
		for _, t := range renderingKeyTokenRE.FindAllStringIndex(text, -1) {
			add(text[t[0]:t[1]], KeyInRendering, m[rendering]+t[0])
		}
	}
	for _, m := range bibliographyEntryRE.FindAllStringSubmatchIndex(view, -1) {
		add(view[m[2]:m[3]], KeyInEntry, m[2])
	}
	for n, line := range lines {
		for _, l := range Links(line) {
			i := strings.Index(l.Destination, refFragment)
			if i < 0 {
				continue
			}
			key := strings.TrimSuffix(l.Destination[i+len(refFragment):], ">")
			if key != "" {
				add(key, KeyInLink, starts[n]+l.DestStart+i+len(refFragment))
			}
		}
	}
	slices.SortStableFunc(out, func(a, b CitationKeyRef) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Start, b.Start))
	})
	return out
}

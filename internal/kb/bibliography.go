package kb

import "regexp"

// bibliographyEntryRE is one rendered reference-list entry: pandoc's csl-entry
// div, its id the citation key behind "ref-".
var bibliographyEntryRE = regexp.MustCompile(`(?s)<div id="ref-([^"]+)"[^>]*>(.*?)</div>`)

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

package claimgraph

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/write"
)

var (
	// markerRE is one marker the write API appended to a line, with the space
	// separating it from the content it follows.
	markerRE = func() *regexp.Regexp {
		alts := make([]string, 0, 2)
		for _, opener := range write.MarkerOpeners() {
			alts = append(alts, `[ \t]*`+regexp.QuoteMeta(opener)+`.*?-->`)
		}
		return regexp.MustCompile(strings.Join(alts, "|"))
	}()

	// anchorRE is a rewritten cross-reference's three attributes, in the
	// contract's order: href, type, label. The type is read as whatever the
	// attribute holds.
	anchorRE = kb.PyRE(`(?s)<a\s+href="([^"]*)"\s+data-reference-type="([^"]+)"\s+data-reference="([^"]*)"`)

	// anchorRenderingRE is a cross-reference whole, with the text it shows a
	// reader inside it.
	anchorRenderingRE = kb.PyRE(`(?s)<a\s[^>]*>(.*?)</a>`)
)

// cleverefTypes are the reference types whose label attribute is a list:
// cleveref splits its argument on the comma, so a label reaching one of these
// never contains one, while a \ref's single label may.
var cleverefTypes = map[string]bool{"ref+label": true, "ref+Label": true}

// listSeparator is what a page writes between two items of one printed list,
// as a pattern fragment: the word before an anchor carries across one, and a
// claim named by hand yields one candidate per item.
const listSeparator = `[` + kb.PyWhitespace + `]*(?:,[` + kb.PyWhitespace + `]*(?:(?:and|or)[` + kb.PyWhitespace + `]+)?|[` + kb.PyWhitespace + `](?:and|or)[` + kb.PyWhitespace + `]+|(?:&|–|—|--)[` + kb.PyWhitespace + `]*)`

// anchorLabels are the labels one reference names: several only where a
// cleveref named several.
func anchorLabels(referenceType, label string) []string {
	if !cleverefTypes[referenceType] {
		return []string{label}
	}
	var out []string
	for _, part := range strings.Split(label, ",") {
		if found := kb.Strip(part); found != "" {
			out = append(out, found)
		}
	}
	if len(out) == 0 {
		return []string{label}
	}
	return out
}

// stripMarkers is text with every marker the write API appended removed.
func stripMarkers(text string) string { return markerRE.ReplaceAllString(text, "") }

// unquote is text with each line's blockquote markers removed, line count
// kept, so an offset into it names the same line of the original.
func unquote(text string) string {
	lines := kb.SplitLines(text)
	for i, l := range lines {
		lines[i] = kb.BlockquotePrefix.ReplaceAllString(l, "")
	}
	return strings.Join(lines, "\n")
}

// pageText is text as the words the page shows: citation and cross-reference
// markup off, what each renders kept.
func pageText(text string) string {
	return kb.Strip(anchorRenderingRE.ReplaceAllString(citationSpanRE.ReplaceAllString(text, "${rendering}"), "$1"))
}

// Document is one document of the tree, by its kb-root-relative slash path.
type Document struct {
	Path string
	Text string
}

// Domain is the volume the document falls under; "" for the entry point.
func (d Document) Domain() string {
	head, _, found := strings.Cut(d.Path, "/")
	if !found {
		return ""
	}
	return head
}

// Lines is the document's lines as str.splitlines() reads them.
func (d Document) Lines() []string { return kb.SplitLines(d.Text) }

// Heading is the document's first H1 as the words the page shows, or "".
func (d Document) Heading() string {
	for _, line := range kb.SplitLines(d.Text) {
		if strings.HasPrefix(line, "# ") {
			return kb.Strip(anchorRenderingRE.ReplaceAllString(stripMarkers(line[2:]), "$1"))
		}
	}
	return ""
}

// Tree is the document set and its two link relations.
type Tree struct {
	Root      string
	Documents map[string]Document
	// Paths is every document path in sorted order.
	Paths []string
	// Children are each document's down-links in the order it lists them.
	Children map[string][]string
	// Parents are the up-link each non-root document opens with.
	Parents map[string]string
}

// declaringKinds must declare their claims or their absence.
var declaringKinds = map[string]bool{kb.DocumentLeaf: true}

// documentKind is a document's kind by path shape alone.
func documentKind(p string, hasChildren bool) string {
	switch {
	case p == kb.EntryPointFile:
		return kb.DocumentEntryPoint
	case hasChildren:
		return kb.DocumentIndex
	}
	return kb.DocumentLeaf
}

func (t *Tree) kind(p string) string { return documentKind(p, len(t.Children[p]) > 0) }

// builtTree is the KB this build is writing. The walk's start loaded it and
// saved it in the current format before any claim-graph stage, so it is read
// as current, from disk.
func builtTree(kbRoot string) *kb.Source { return kb.OnDisk(kbRoot) }

// recordsAt is the built KB whose build records stand at repoRoot.
func recordsAt(repoRoot string) *kb.Source { return builtTree(filepath.Join(repoRoot, kb.KBDir)) }

// readTree reads the tree and both link relations; it asserts nothing.
func readTree(kbRoot string) (*Tree, error) {
	src := builtTree(kbRoot)
	paths, err := kb.Documents(src)
	if err != nil {
		return nil, err
	}
	t := &Tree{Root: kbRoot, Documents: map[string]Document{}, Children: map[string][]string{}, Parents: map[string]string{}}
	for _, p := range paths {
		text, err := src.ReadText(src.KBPath(p))
		if err != nil {
			return nil, err
		}
		t.Documents[p] = Document{Path: p, Text: text}
		t.Paths = append(t.Paths, p)
	}
	slices.Sort(t.Paths)
	for _, p := range t.Paths {
		parent, ok, children := kb.TreeLinks(p, t.Documents[p].Text)
		if ok {
			t.Parents[p] = parent
		}
		t.Children[p] = children
	}
	return t, nil
}

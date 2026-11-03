package kb

import (
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"kbase/internal/formats/format400"
)

// SkipDirs are never crawled, matched as one path segment at any depth.
var SkipDirs = map[string]bool{
	".venv": true, "venv": true, ".git": true, "build": true, "node_modules": true,
	".index": true, ".agents": true, "_archive": true,
}

// skipSegmentRuns exclude a file wherever their segments occur consecutively
// in its relative path.
var skipSegmentRuns = [][]string{{"tests", "fixtures"}, {".claude", "worktrees"}}

// documentTemplateSuffix marks a document template, whose links resolve only
// where a build stamps it.
const documentTemplateSuffix = ".tmpl.md"

// The link grammar is format400's: every reader of a document's links, the
// citations among them, reads through it.
var (
	// LinkRE is an inline link or image, [text](destination "title"); group 1
	// is the destination and group 2 the title with its delimiters.
	LinkRE = format400.LinkRE
	// RefDefRE is a link reference definition; group 1 is the destination.
	RefDefRE = format400.RefDefRE
	// InlineMathRE is the writer's inline maths span.
	InlineMathRE = format400.InlineMathRE
)

// Links is every inline link of line, in order: its spans, text, destination
// and title, and the node id the title cites where it is a citation.
func Links(line string) []format400.Link { return format400.Links(line) }

// TextLink is one inline link in a document's text: its 0-based line and its
// link, whose offsets hold on the raw line.
type TextLink struct {
	Line int
	format400.Link
}

// TextCitation is a TextLink whose title is a node id.
type TextCitation = TextLink

// TextLinks is every inline link in text outside code, in order. Code is
// blanked byte for byte, so each link's offsets are its offsets in the raw
// line.
func TextLinks(text string) []TextLink {
	var out []TextLink
	for n, view := range SplitLines(StripCodeSplitLines(text)) {
		for _, l := range Links(view) {
			out = append(out, TextLink{Line: n, Link: l})
		}
	}
	return out
}

// TextLinkDefinitions is every link reference definition in text outside
// code, [label]: destination "title", in order, each a TextLink spanning the
// definition, its destination and the destination's offsets on the raw line
// set; its label and title are not read.
func TextLinkDefinitions(text string) []TextLink {
	var out []TextLink
	for n, view := range SplitLines(StripCodeSplitLines(text)) {
		if m := RefDefRE.FindStringSubmatchIndex(view); m != nil {
			out = append(out, TextLink{Line: n, Link: format400.Link{Start: m[0], End: m[1], DestStart: m[2], DestEnd: m[3], Destination: view[m[2]:m[3]]}})
		}
	}
	return out
}

// TextCitations is every citation in text outside code, in order: each of
// TextLinks whose title is a node id.
func TextCitations(text string) []TextCitation {
	return slices.DeleteFunc(TextLinks(text), func(l TextLink) bool {
		_, ok := l.CitedID()
		return !ok
	})
}

// Citation is the citation of node id whose link text is text and whose
// destination is destination.
func Citation(text, destination, id string) string { return format400.Citation(text, destination, id) }

// BareIDs is every node id in line outside a statement marker, a link's
// destination and title, and a link reference definition; code is the
// caller's to blank first.
func BareIDs(line string) []format400.IDToken { return format400.BareIDs(line) }

var (
	fenceLineRE = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})[ \t]*(.*?)[ \t]*$")
	// BlockquotePrefix is one line's leading blockquote markers: indent, >
	// and an optional space, once per nesting level.
	BlockquotePrefix = regexp.MustCompile(`^(?:[ \t]{0,3}>[ \t]?)+`)
	// ListItemRE is a line, its blockquote prefix and leading space removed,
	// that opens a list item: a bullet, or a number closed by `.` or `)`.
	ListItemRE = PyRE(`^(?:[-*+]\s|\d+[.)]\s)`)
	// closerTail is what a closing fence may carry and still close: the
	// emphasis delimiter of the statement it sits in.
	closerTail = regexp.MustCompile(`^[*_]*$`)
	lineSuffix = regexp.MustCompile(`:\d+$`)
)

// Lines splits text into lines without a trailing empty one.
func Lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
}

// MarkdownFiles is every .md file of the KB, absolute and sorted, less
// document templates and anything under a skipped directory.
func MarkdownFiles(src *Source) ([]string, error) {
	root := src.Root()
	var out []string
	err := src.walk(root, func(name string) bool { return SkipDirs[name] }, func(p string) error {
		name := filepath.Base(p)
		if !strings.HasSuffix(name, ".md") || strings.HasSuffix(name, documentTemplateSuffix) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		for _, run := range skipSegmentRuns {
			if containsRun(parts, run) {
				return nil
			}
		}
		out = append(out, p)
		return nil
	})
	slices.Sort(out)
	return out, err
}

// CitingDocuments is every document citations are read in, as kb-root-relative
// slash paths, sorted: every MarkdownFiles document but those standing at a
// summary page position, the volumes taken as Volumes(Documents(src)). A file
// named like a summary page anywhere else is read.
func CitingDocuments(src *Source) ([]string, error) {
	docs, err := Documents(src)
	if err != nil {
		return nil, err
	}
	pages := SummaryPagePositions(Volumes(docs))
	files, err := MarkdownFiles(src)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, file := range files {
		rel, err := filepath.Rel(src.Root(), file)
		if err != nil {
			return nil, err
		}
		if rel = filepath.ToSlash(rel); !slices.Contains(pages, rel) {
			out = append(out, rel)
		}
	}
	return out, nil
}

func containsRun(parts, run []string) bool {
	for i := 0; i+len(run) <= len(parts); i++ {
		if slices.Equal(parts[i:i+len(run)], run) {
			return true
		}
	}
	return false
}

// BlankFencedLines is text's lines with every fenced block blanked, line
// count kept. A fence opens on three or more backticks or tildes at any
// indentation under any depth of blockquote, and closes on a run of the same
// character at least as long carrying nothing after it but emphasis markers. A
// fence opened inside a blockquote closes with the blockquote.
func BlankFencedLines(text string) []string {
	return blankFenced(Lines(text))
}

// StripCodeFences is text's str.splitlines() lines with every fenced block
// blanked, joined by \n: the metadata readers' view of a document.
func StripCodeFences(text string) string {
	return strings.Join(blankFenced(SplitLines(text)), "\n")
}

func blankFenced(lines []string) []string {
	out := slices.Clone(lines)
	for _, f := range Fences(lines) {
		for i := f.Start; i < f.End; i++ {
			out[i] = ""
		}
	}
	return out
}

// Fence is one fenced block of a document's lines: Start is its opening
// line, End one past its last, Info the opener's info string, and Closed
// whether its last line, End-1, is its closing fence.
type Fence struct {
	Start, End int
	Info       string
	Closed     bool
}

// Fences is every fenced block of lines, in order, by BlankFencedLines'
// rule; a block left open runs to the last line.
func Fences(lines []string) []Fence {
	var out []Fence
	opener := ""
	quoted := false
	for i, raw := range lines {
		prefix := BlockquotePrefix.FindString(raw)
		m := fenceLineRE.FindStringSubmatch(raw[len(prefix):])
		if opener == "" {
			if m != nil {
				opener, quoted = m[1], prefix != ""
				out = append(out, Fence{Start: i, End: len(lines), Info: m[2]})
			}
			continue
		}
		if quoted && prefix == "" {
			opener = ""
			out[len(out)-1].End = i
			continue
		}
		if m != nil && m[1][0] == opener[0] && len(m[1]) >= len(opener) && closerTail.MatchString(m[2]) {
			opener = ""
			out[len(out)-1].End, out[len(out)-1].Closed = i+1, true
		}
	}
	return out
}

// StripCode is text with fenced blocks, inline maths spans and inline code
// spans blanked, every line kept where it was.
func StripCode(text string) string {
	return blankInlineSpans(strings.Join(BlankFencedLines(text), "\n"))
}

// StripCodeSplitLines is StripCode over str.splitlines() lines, as the
// metadata and citation readers split a document.
func StripCodeSplitLines(text string) string {
	return blankInlineSpans(StripCodeFences(text))
}

func blankInlineSpans(text string) string { return format400.BlankInlineCode(text) }

var (
	// destinationRE is a link destination wherever a link text closes, so a
	// linked image's inner destination is found as well as its outer one.
	destinationRE = PyRE(`\]\(\s*(<[^>]*>|[^)` + PyWhitespace + `]+)\s*\)`)
	// unmovedTargetRE is a destination naming no file relative to the
	// document it sits in: a URL scheme, a rooted or home path, an anchor.
	unmovedTargetRE = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.\-]*:|[/~#])`)
)

// RebaseInlineLinks is inline text — a heading, a title — moved from a file in
// fromDir to a file in toDir, each relative link destination outside a code or
// maths span rewritten to name the same file. Both directories are slash paths
// relative to one root; an #anchor rides along unchanged.
func RebaseInlineLinks(text, fromDir, toDir string) string {
	if path.Clean(or(fromDir, ".")) == path.Clean(or(toDir, ".")) {
		return text
	}
	var b strings.Builder
	last := 0
	for _, m := range destinationRE.FindAllStringSubmatchIndex(blankInlineSpans(text), -1) {
		b.WriteString(text[last:m[2]])
		b.WriteString(rebaseTarget(text[m[2]:m[3]], fromDir, toDir))
		last = m[3]
	}
	b.WriteString(text[last:])
	return b.String()
}

func rebaseTarget(raw, fromDir, toDir string) string {
	angled := strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">")
	target := raw
	if angled {
		target = raw[1 : len(raw)-1]
	}
	if target == "" || unmovedTargetRE.MatchString(target) {
		return raw
	}
	p, anchor, hasAnchor := strings.Cut(target, "#")
	rebased := relativePath(path.Join(fromDir, p), or(toDir, "."))
	if hasAnchor {
		rebased += "#" + anchor
	}
	if angled {
		return "<" + rebased + ">"
	}
	return rebased
}

// relativePath is posixpath.relpath over two paths relative to one root.
func relativePath(target, start string) string {
	split := func(p string) []string {
		var out []string
		for _, s := range strings.Split(path.Join("/", p), "/") {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	t, s := split(target), split(start)
	common := 0
	for common < len(t) && common < len(s) && t[common] == s[common] {
		common++
	}
	parts := make([]string, 0, len(s)-common+len(t)-common)
	for range s[common:] {
		parts = append(parts, "..")
	}
	parts = append(parts, t[common:]...)
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// The tree's link relation: every document's up-link and child list. Three
// readers take it here — TreeLinks as the claim graph reads it, DeclaredLinks
// as the build validator reads it, and the export's child-list reading
// (childlist.go).

// ResolveLink is target, written in the document at source, as a
// kb-root-relative path.
func ResolveLink(source, target string) string { return path.Join(path.Dir(source), target) }

var urlSchemeRE = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)

// TreeLinks is a document's place in the tree as the claim graph reads it: the
// parent its up-link names — the first .md link on the up-link line, the first
// line after the frontmatter, where that line carries UplinkMarker — and its
// children, every .md link on any other line of the body in the order written,
// duplicates dropped, each resolved.
// Links inside code are not links.
func TreeLinks(source, text string) (parent string, hasParent bool, children []string) {
	text = StripFrontmatter(text)
	first := ""
	if lines := SplitLines(text); len(lines) > 0 {
		first = lines[0]
	}
	for i, line := range BlankFencedLines(StripCode(text)) {
		for _, m := range LinkRE.FindAllStringSubmatch(line, -1) {
			target := StripTarget(m[1])
			if strings.HasPrefix(target, "#") || urlSchemeRE.MatchString(target) {
				continue
			}
			p, _, _ := strings.Cut(target, "#")
			if !strings.HasSuffix(p, ".md") {
				continue
			}
			resolved := ResolveLink(source, p)
			switch {
			case i == 0 && !hasParent && strings.Contains(first, UplinkMarker):
				parent, hasParent = resolved, true
			case i == 0:
			case !slices.Contains(children, resolved):
				children = append(children, resolved)
			}
		}
	}
	return parent, hasParent, children
}

// ReadingOrder is every path children reaches from the entry point, in
// preorder over each path's children as children lists them — TreeLinks' for
// a document — each where the walk first reaches it, so a revisit ends that
// branch and a cycle ends the walk.
func ReadingOrder(children map[string][]string) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(p string)
	walk = func(p string) {
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
		for _, c := range children[p] {
			walk(c)
		}
	}
	walk(EntryPointFile)
	return out
}

// DeclaredLink is one link a document declares as the build validator reads
// it: an inline link, an up-link where its text carries UplinkMarker, or a
// reference definition.
type DeclaredLink struct {
	Target string
	Up     bool
}

// DeclaredLinks is every link text declares, outside code.
func DeclaredLinks(text string) []DeclaredLink {
	scrubbed := StripCode(text)
	var links []DeclaredLink
	for _, m := range LinkRE.FindAllStringSubmatch(scrubbed, -1) {
		links = append(links, DeclaredLink{Target: m[1], Up: strings.Contains(m[0], UplinkMarker)})
	}
	for _, m := range RefDefRE.FindAllStringSubmatch(scrubbed, -1) {
		links = append(links, DeclaredLink{Target: m[1]})
	}
	return links
}

// ResolveDeclared is a declared link's raw target, written in source, as a
// kb-root-relative path, or "" where it names no path inside the tree.
func ResolveDeclared(source, target string) string {
	cleaned := StripTarget(target)
	if cleaned == "" || strings.HasPrefix(cleaned, "/") {
		return ""
	}
	if resolved := ResolveLink(source, cleaned); !strings.HasPrefix(resolved, "..") {
		return resolved
	}
	return ""
}

// unwrapTarget is a raw link destination unwrapped from <...>.
func unwrapTarget(target string) string {
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") && len(target) >= 2 {
		return target[1 : len(target)-1]
	}
	return target
}

// StripTarget is a raw link destination as a path to resolve: unwrapped from
// <...>, its #anchor and trailing :line dropped, then percent-decoded — last,
// so an encoded %23 is not split off as an anchor.
func StripTarget(target string) string {
	target, _, _ = strings.Cut(unwrapTarget(target), "#")
	target = lineSuffix.ReplaceAllString(target, "")
	if decoded, err := url.PathUnescape(target); err == nil {
		return decoded
	}
	return target
}

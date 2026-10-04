package kb

import (
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

var (
	// LinkRE is an inline link or image, [text](target); group 1 is the
	// destination, angle-bracketed or bare. Link text may nest one level of
	// brackets.
	LinkRE = regexp.MustCompile(`\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*(<[^>]*>|[^)\s]+)\s*\)`)
	// RefDefRE is a link reference definition, [label]: destination "title";
	// group 1 is the destination. A footnote definition, [^1]: text, is not one.
	RefDefRE = regexp.MustCompile(`(?m)^[ \t]{0,3}\[[^\[\]^][^\[\]]*\]:[ \t]*(<[^>]*>|\S+)[ \t]*(?:"[^"]*"|'[^']*'|\([^)]*\))?[ \t]*$`)
	// InlineMathRE is the writer's inline maths span, which may wrap across
	// lines; the $ at both ends is what keeps an odd backtick in prose from
	// opening one.
	InlineMathRE = regexp.MustCompile("\\$`[^`]*`\\$")

	inlineCodeRE = regexp.MustCompile("`[^`\n]*`")
	fenceLineRE  = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})[ \t]*(.*?)[ \t]*$")
	// BlockquotePrefix is one line's leading blockquote markers: indent, >
	// and an optional space, once per nesting level.
	BlockquotePrefix = regexp.MustCompile(`^(?:[ \t]{0,3}>[ \t]?)+`)
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

// MarkdownFiles is every .md file under root, sorted, less document
// templates and anything under a skipped directory.
func MarkdownFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		if d.IsDir() {
			if SkipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") || strings.HasSuffix(d.Name(), documentTemplateSuffix) {
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
	var out []string
	opener := ""
	quoted := false
	for _, raw := range lines {
		prefix := BlockquotePrefix.FindString(raw)
		line := raw[len(prefix):]
		m := fenceLineRE.FindStringSubmatch(line)
		if opener == "" {
			if m != nil {
				opener, quoted = m[1], prefix != ""
				out = append(out, "")
			} else {
				out = append(out, raw)
			}
			continue
		}
		if quoted && prefix == "" {
			opener = ""
			out = append(out, raw)
			continue
		}
		out = append(out, "")
		if m != nil && m[1][0] == opener[0] && len(m[1]) >= len(opener) && closerTail.MatchString(m[2]) {
			opener = ""
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

func blankInlineSpans(text string) string {
	return inlineCodeRE.ReplaceAllStringFunc(InlineMathRE.ReplaceAllStringFunc(text, spaces), spaces)
}

// spaces is s with every character but a newline replaced by a space.
func spaces(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		return ' '
	}, s)
}

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

// The tree's link relation: every document's up-link and child list. kb_tools
// reads it two ways, and each reader here ports one of them.

// ResolveLink is target, written in the document at source, as a
// kb-root-relative path.
func ResolveLink(source, target string) string { return path.Join(path.Dir(source), target) }

var urlSchemeRE = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)

// TreeLinks is a document's place in the tree as the claim graph reads it
// (kb_claimgraph's tree.read): the parent its up-link names — the first .md
// link on line 1, where that line carries UplinkMarker — and its children,
// every .md link on any other line in the order written, duplicates dropped,
// each resolved. Links inside code are not links.
func TreeLinks(source, text string) (parent string, hasParent bool, children []string) {
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

// DeclaredLink is one link a document declares as the build validator reads
// it (kb_survey's validate): an inline link, an up-link where its text carries
// UplinkMarker, or a reference definition.
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

// StripTarget is a raw link destination as a path to resolve: unwrapped from
// <...>, its #anchor and trailing :line dropped, then percent-decoded — last,
// so an encoded %23 is not split off as an anchor.
func StripTarget(target string) string {
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") && len(target) >= 2 {
		target = target[1 : len(target)-1]
	}
	target, _, _ = strings.Cut(target, "#")
	target = lineSuffix.ReplaceAllString(target, "")
	if decoded, err := url.PathUnescape(target); err == nil {
		return decoded
	}
	return target
}

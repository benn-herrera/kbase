package kbdocs

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"kbase/internal/kb"
)

// The excerpts' caps, in characters: one document's body, and the whole,
// boundary lines included.
const (
	ExcerptDocumentChars = 12_000
	ExcerptsTotalChars   = 48_000
)

const (
	separatorFormat = "==> %s <=="
	titleJoin       = " › "
	paragraphBreak  = "\n\n"
)

var headingRE = kb.PyRE(`(?m)^#\s+(.+?)\s*$`)

// Cut is one document the caps shortened, by title, with what was kept and
// what was cut in characters.
type Cut struct {
	Title    string
	Kept     int
	Excluded int
}

// Excerpts is the text the overview passage is written from, and every cut
// the caps made to it.
type Excerpts struct {
	Text string
	Cuts []Cut
}

type excerpted struct{ title, body string }

// ComposeExcerpts reads the KB's tree into the excerpts: the entry
// point, then for each document it lists that document's index and, where it
// has one, its own-prose leaf. Each body loses its metadata block and its
// up-link, and each link is reduced to its text; a boundary line names each
// document by title. The same tree gives the same text.
func ComposeExcerpts(src *kb.Source) (Excerpts, error) {
	docs, err := excerptedDocuments(src)
	if err != nil {
		return Excerpts{}, err
	}
	return assembleExcerpts(docs), nil
}

func excerptedDocuments(src *kb.Source) ([]excerpted, error) {
	read := func(rel string) (string, error) { return src.ReadText(src.KBPath(rel)) }
	entry, err := read(kb.EntryPointFile)
	if err != nil {
		return nil, err
	}
	entry = kb.StripFrontmatter(entry)
	docs := []excerpted{{title: heading(entry), body: body(entry)}}
	seen := map[[2]string]bool{}
	for _, l := range links(entry) {
		if seen[l] {
			continue
		}
		seen[l] = true
		volumeTitle, indexPath := l[0], path.Clean(l[1])
		index, err := read(indexPath)
		if err != nil {
			return nil, err
		}
		docs = append(docs, excerpted{title: volumeTitle, body: body(index)})
		for _, child := range links(withoutUplink(kb.StripFrontmatter(index))) {
			if child[0] != kb.OwnProseTitle {
				continue
			}
			leaf, err := read(path.Clean(path.Join(path.Dir(indexPath), child[1])))
			if err != nil {
				return nil, err
			}
			docs = append(docs, excerpted{title: volumeTitle + titleJoin + kb.OwnProseTitle, body: body(leaf)})
			break
		}
	}
	return docs, nil
}

// links is every link in text as (link text, destination), in order.
func links(text string) [][2]string {
	var out [][2]string
	for _, m := range kb.LinkRE.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, [2]string{linkText(text, m), strings.Trim(text[m[2]:m[3]], "<>")})
	}
	return out
}

// linkText is the text of one kb.LinkRE match, which captures only the
// destination: what stands between the opening bracket and the last `](`
// before it.
func linkText(text string, m []int) string {
	return text[m[0]+1 : m[0]+strings.LastIndex(text[m[0]:m[2]], "](")]
}

func withoutUplink(text string) string {
	first, rest, _ := strings.Cut(strings.TrimLeft(text, "\n"), "\n")
	if strings.HasPrefix(kb.LStrip(first), "["+kb.UplinkMarker) {
		return rest
	}
	return text
}

func body(text string) string {
	b := withoutUplink(kb.StripFrontmatter(text))
	var out strings.Builder
	last := 0
	for _, m := range kb.LinkRE.FindAllStringSubmatchIndex(b, -1) {
		out.WriteString(b[last:m[0]])
		out.WriteString(linkText(b, m))
		last = m[1]
	}
	out.WriteString(b[last:])
	return strings.Trim(out.String(), "\n")
}

func heading(text string) string {
	if m := headingRE.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

func chars(s string) int { return utf8.RuneCountInString(s) }

// cutTo is the longest run of body's leading paragraphs within limit
// characters.
func cutTo(body string, limit int) string {
	if chars(body) <= limit {
		return body
	}
	var kept []string
	size := 0
	for _, p := range strings.Split(body, paragraphBreak) {
		grown := size + chars(p)
		if len(kept) > 0 {
			grown += chars(paragraphBreak)
		}
		if grown > limit {
			break
		}
		kept = append(kept, p)
		size = grown
	}
	return strings.Join(kept, paragraphBreak)
}

func assembleExcerpts(docs []excerpted) Excerpts {
	var pieces []string
	var cuts []Cut
	used := 0
	for _, d := range docs {
		head := fmt.Sprintf(separatorFormat, d.title) + "\n"
		joiner := 0
		if len(pieces) > 0 {
			joiner = chars(paragraphBreak)
		}
		room := min(ExcerptDocumentChars, ExcerptsTotalChars-used-joiner-chars(head))
		kept := ""
		if room > 0 {
			kept = cutTo(d.body, room)
		}
		if chars(kept) < chars(d.body) {
			cuts = append(cuts, Cut{Title: d.title, Kept: chars(kept), Excluded: chars(d.body) - chars(kept)})
		}
		if kept == "" {
			continue
		}
		pieces = append(pieces, head+kept)
		used += joiner + chars(head) + chars(kept)
	}
	return Excerpts{Text: strings.Join(pieces, paragraphBreak), Cuts: cuts}
}

// notProse are the constructs a passage may not hold, each matched within a
// line: a heading, a list item, a table row, a code fence, a Markdown link, a
// .md path.
var notProse = []*regexp.Regexp{
	kb.PyRE(`^\s{0,3}#{1,6}(?:\s|$)`),
	kb.PyRE(`^\s*(?:[-*+]|\d+[.)])\s`),
	kb.PyRE(`^\s*\|`),
	kb.PyRE("^\\s*(?:```|~~~)"),
	kb.LinkRE,
	kb.PyRE(`\S\.md\b`),
}

// NotProse is every line of a returned passage holding a construct that
// would corrupt the document it is substituted into, verbatim.
func NotProse(passage string) []string {
	var lines []string
	for _, line := range kb.SplitLines(passage) {
		for _, re := range notProse {
			if re.MatchString(line) {
				lines = append(lines, line)
				break
			}
		}
	}
	return lines
}

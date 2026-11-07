package format40

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The inline Markdown link and the reference definition, as every reader of
// a document's links reads them.
var (
	// LinkRE is an inline link or image, [text](destination "title"): group 1
	// is the destination, angle-bracketed or bare, and group 2 the title with
	// its delimiters, where the link carries one. Link text may nest one level
	// of brackets.
	LinkRE = regexp.MustCompile(`\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*(<[^>]*>|[^)\s]+)(?:\s+("[^"]*"|'[^']*'|\([^)]*\)))?\s*\)`)
	// RefDefRE is a link reference definition, [label]: destination "title";
	// group 1 is the destination. A footnote definition, [^1]: text, is not one.
	RefDefRE = regexp.MustCompile(`(?m)^[ \t]{0,3}\[[^\[\]^][^\[\]]*\]:[ \t]*(<[^>]*>|\S+)[ \t]*(?:"[^"]*"|'[^']*'|\([^)]*\))?[ \t]*$`)
	// InlineMathRE is the writer's inline maths span, which may wrap across
	// lines; the $ at both ends is what keeps an odd backtick in prose from
	// opening one.
	InlineMathRE = regexp.MustCompile("\\$`[^`]*`\\$")
)

// Link is one inline link of a line: the byte span of the whole link and of
// its destination, its text, its destination as written, and its title
// without its delimiters, "" where it carries none.
type Link struct {
	Start, End         int
	DestStart, DestEnd int
	Text               string
	Destination        string
	Title              string
}

// Links is every inline link of line, in order.
func Links(line string) []Link {
	var out []Link
	for _, m := range LinkRE.FindAllStringSubmatchIndex(line, -1) {
		l := Link{Start: m[0], End: m[1], DestStart: m[2], DestEnd: m[3], Destination: line[m[2]:m[3]],
			Text: line[m[0]+1 : m[0]+strings.LastIndex(line[m[0]:m[2]], "](")]}
		if m[4] >= 0 {
			l.Title = line[m[4]+1 : m[5]-1]
		}
		out = append(out, l)
	}
	return out
}

// CitedID is the node id l's title names: a citation's. ok is false for an
// ordinary link, whose title is absent or is no node id.
func (l Link) CitedID() (id string, ok bool) {
	_, ok = NodeIDKind(l.Title)
	return l.Title, ok
}

// Citation is the citation of node id whose link text is text and whose
// destination is destination.
func Citation(text, destination, id string) string {
	return "[" + text + "](" + destination + ` "` + id + `")`
}

// The name of every summary page, kb-root's and each volume's, its title, and
// the headings every entry of a summary page renders under its own title's.
const (
	SummaryFile            = "kb-summary.md"
	SummaryTitle           = "KB summary"
	SummaryRigorHeading    = "Rigor"
	SummaryStrengthHeading = "Strength"
)

// IndexFile is the name of the page a volume's directory holds.
const IndexFile = "index.md"

// Volumes is the volumes among paths, kb-root-relative documents: the
// top-level directories holding an IndexFile.
func Volumes(paths []string) map[string]bool {
	volumes := map[string]bool{}
	for _, rel := range paths {
		if dir, rest, nested := strings.Cut(rel, "/"); nested && rest == IndexFile {
			volumes[dir] = true
		}
	}
	return volumes
}

// SummaryPage is the kb-root-relative summary page a node hosted at host has
// its entry on: the page of the volume host lies in, or the root page where it
// lies in none.
func SummaryPage(host string, volumes map[string]bool) string {
	if dir, _, nested := strings.Cut(host, "/"); nested && volumes[dir] {
		return dir + "/" + SummaryFile
	}
	return SummaryFile
}

// SummaryPagePositions is every kb-root-relative path a summary page can
// stand at, sorted: SummaryFile and each volume's page.
func SummaryPagePositions(volumes map[string]bool) []string {
	out := []string{SummaryFile}
	for dir := range volumes {
		out = append(out, dir+"/"+SummaryFile)
	}
	slices.Sort(out)
	return out
}

// SummaryEntryHeadings is the text of every heading a summary page renders
// for an entry titled title, in order: the title's, its rigor section's and,
// where its node carries a derived strength, its strength section's.
func SummaryEntryHeadings(title string, derived bool) []string {
	if derived {
		return []string{title, SummaryRigorHeading, SummaryStrengthHeading}
	}
	return []string{title, SummaryRigorHeading}
}

// idTokenRE is a node id of any kind as it stands in prose.
var idTokenRE = regexp.MustCompile(IDBody())

// IDToken is one node id standing in a line: its byte span and the id.
type IDToken struct {
	Start, End int
	ID         string
}

// BareIDs is every node id of any kind in line, bounded as Python's \b bounds
// a word, that stands in prose outside every link and outside a statement
// marker and a link reference definition, and the id of a link whose whole
// text is that id, its token spanning the whole link. An id that is only part
// of a link's text is not bare: a link cannot nest, and that link already
// names a target. Code is the caller's to blank first.
func BareIDs(line string) []IDToken {
	links := Links(line)
	var out []IDToken
	for _, t := range BareTokens(line, idTokenRE) {
		i := slices.IndexFunc(links, func(l Link) bool { return l.Start < t.Start && t.End < l.DestStart })
		switch {
		case i < 0:
			out = append(out, t)
		case links[i].Text == t.ID:
			out = append(out, IDToken{Start: links[i].Start, End: links[i].End, ID: t.ID})
		}
	}
	return out
}

// BareTokens is every token idRE matches in line, bounded as BareIDs bounds
// one, outside a statement marker, a link's destination and title, and a link
// reference definition; a link's text is read. It reads an older format's ids
// for a conversion, which handles link texts itself.
func BareTokens(line string, idRE *regexp.Regexp) []IDToken {
	var outside [][]int
	for _, m := range StatementMarkerRE.FindAllStringIndex(line, -1) {
		outside = append(outside, m)
	}
	for _, l := range Links(line) {
		outside = append(outside, []int{l.DestStart, l.End})
	}
	outside = append(outside, RefDefRE.FindAllStringIndex(line, -1)...)
	var out []IDToken
	for _, m := range idRE.FindAllStringIndex(line, -1) {
		if !BoundaryAt(line, m[0]) || !BoundaryAt(line, m[1]) {
			continue
		}
		within := false
		for _, s := range outside {
			within = within || s[0] <= m[0] && m[1] <= s[1]
		}
		if !within {
			out = append(out, IDToken{Start: m[0], End: m[1], ID: line[m[0]:m[1]]})
		}
	}
	return out
}

// IsWord is whether r is a character Python's \w matches.
func IsWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// BoundaryAt is Python's \b at byte offset i of s.
func BoundaryAt(s string, i int) bool {
	before, after := false, false
	if i > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:i])
		before = IsWord(r)
	}
	if i < len(s) {
		r, _ := utf8.DecodeRuneInString(s[i:])
		after = IsWord(r)
	}
	return before != after
}

// BlankInlineCode is text with every inline maths span and every inline code
// span blanked byte for byte, newlines kept, so an offset into the result is
// the same offset into text. A code span pairs as CommonMark pairs it on one
// line: a maximal run of n backticks opens, and only the next maximal run of
// exactly n closes; a run of any other length inside is content, and an
// opener with no closer on its line is literal text. Go's regexp has no
// backreference to state the pairing as a pattern.
func BlankInlineCode(text string) string {
	text = InlineMathRE.ReplaceAllStringFunc(text, blank)
	var b strings.Builder
	last := 0
	for i := 0; i < len(text); {
		n := backtickRun(text, i)
		if n == 0 {
			i++
			continue
		}
		closer := -1
		for j := i + n; j < len(text) && text[j] != '\n'; {
			m := backtickRun(text, j)
			if m == n {
				closer = j
				break
			}
			j += max(m, 1)
		}
		if closer < 0 {
			i += n
			continue
		}
		end := closer + n
		b.WriteString(text[last:i])
		b.WriteString(blank(text[i:end]))
		last, i = end, end
	}
	b.WriteString(text[last:])
	return b.String()
}

// backtickRun is the length of the run of backticks starting at s[i].
func backtickRun(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// blank is s with every byte but a newline a space.
func blank(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c != '\n' {
			b[i] = ' '
		}
	}
	return string(b)
}

package kb

import (
	"regexp"
	"strings"
)

// The child-list reading of the tree relation: a third reading beside
// TreeLinks', and the one the LaTeX export walks. Only an index's child-list
// lines are spine edges — a body link is not, so an entry point's prose
// cross-links do not read as volumes — and the up-link is the body's first
// line carrying text, where that line carries UplinkMarker and a .md link.

// childLinkRE is one inline link in the child-list form: the link LinkRE
// reads, with the optional quoted title beside it that RefDefRE carries and
// LinkRE's destination group cannot cross the whitespace to. Group 1 is the
// destination.
var childLinkRE = regexp.MustCompile(
	`\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(\s*(<[^>]*>|[^)\s]+)\s*(?:"[^"]*"|'[^']*'|\([^)]*\))?\s*\)`)

// ChildListLines is a document's body below the frontmatter in SplitLines'
// line unit, read by the child-list reading: the index of the up-link line, -1
// where the body has none, and for each line the targets it names as a
// child-list line, resolved against source, nil for a line that is not one. A
// line inside a fenced block is never a child-list line.
func ChildListLines(source, text string) (lines []string, uplink int, children [][]string) {
	lines = SplitLines(StripFrontmatter(text))
	blanked := blankFenced(lines)
	uplink = -1
	if first := firstContentLine(lines); first >= 0 && isUplinkLine(lines[first]) {
		uplink = first
	}
	children = make([][]string, len(lines))
	for i, line := range blanked {
		targets, ok := childListLine(line)
		if !ok {
			continue
		}
		for _, target := range targets {
			children[i] = append(children[i], ResolveLink(source, target))
		}
	}
	return lines, uplink, children
}

// ChildList is every target the document at source names on a child-list
// line, in the order written, each resolved.
func ChildList(source, text string) []string {
	_, _, children := ChildListLines(source, text)
	var out []string
	for _, targets := range children {
		out = append(out, targets...)
	}
	return out
}

// firstContentLine is the index of lines' first line carrying text, -1 where
// there is none.
func firstContentLine(lines []string) int {
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			return i
		}
	}
	return -1
}

// isUplinkLine is whether line carries UplinkMarker and a .md link.
func isUplinkLine(line string) bool {
	if !strings.Contains(line, UplinkMarker) {
		return false
	}
	for _, m := range LinkRE.FindAllStringSubmatch(line, -1) {
		if strings.HasSuffix(StripTarget(m[1]), ".md") {
			return true
		}
	}
	return false
}

// childListLine reads line as a child-list line: a bulleted list item whose
// content is one or more .md links laid end to end, each with an optional
// quoted title, followed by nothing or by a description after an em dash or a
// hyphen. A description is content, so a link inside it is not a spine entry
// and does not reject the line; prose before the first link is the author's,
// and is markup rather than content only where it is a leading emphasis
// wrapper. The returned targets are the links' destinations, in order.
func childListLine(line string) (targets []string, ok bool) {
	content, ok := listItemContent(line)
	if !ok {
		return nil, false
	}
	at := skipEmphasis(content)
	targets, at, ok = linkRun(content, at)
	if !ok {
		return nil, false
	}
	tail := strings.TrimSpace(content[at:])
	if tail == "" || strings.HasPrefix(tail, "—") || strings.HasPrefix(tail, "-") {
		return targets, true
	}
	// A closing emphasis wrapper may stand between the last link and the
	// separator.
	if rest := strings.TrimSpace(strings.TrimLeft(tail, "*_")); rest != tail {
		if rest == "" || strings.HasPrefix(rest, "—") || strings.HasPrefix(rest, "-") {
			return targets, true
		}
	}
	return nil, false
}

// linkRun reads the run of .md links laid end to end from offset at, returning
// their destinations and the offset past the last. A run ends at the first gap
// carrying anything but whitespace, emphasis or another link, so the separator
// and the description after it are left to the caller's tail rule.
func linkRun(content string, at int) (targets []string, end int, ok bool) {
	for {
		next := at + skipEmphasis(content[at:])
		m := childLinkRE.FindStringSubmatchIndex(content[next:])
		if m == nil || m[0] != 0 {
			return targets, at, len(targets) > 0
		}
		target := StripTarget(content[next+m[2] : next+m[3]])
		if !strings.HasSuffix(target, ".md") {
			return nil, 0, false
		}
		targets = append(targets, target)
		at = next + m[1]
	}
}

// skipEmphasis is the length of s's leading whitespace and emphasis markers,
// under which a child item's own link starts.
func skipEmphasis(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '*' || s[i] == '_') {
		i++
	}
	return i
}

// listItemContent is a line's content after a bulleted list marker and its
// leading indentation, ok false where the line is not a bulleted list item. A
// line indented four columns opens an indented code block, which is a code
// example and not a list item.
func listItemContent(line string) (string, bool) {
	if indentedCode(line) {
		return "", false
	}
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) || (line[i] != '-' && line[i] != '*' && line[i] != '+') {
		return "", false
	}
	i++
	if i >= len(line) || (line[i] != ' ' && line[i] != '\t') {
		return "", false
	}
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[i:], true
}

// indentedCode is whether line's leading whitespace reaches four columns, a
// tab counting to the next multiple of four: CommonMark's indented code block.
func indentedCode(line string) bool {
	column := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			column++
		case '\t':
			column += 4 - column%4
		default:
			return column >= 4
		}
	}
	return column >= 4
}

package format200

import (
	"regexp"
	"slices"
	"strings"
)

var (
	fenceLineRE = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})[ \t]*(.*?)[ \t]*$")
	// BlockquotePrefix is one line's leading blockquote markers: indent, >
	// and an optional space, once per nesting level.
	BlockquotePrefix = regexp.MustCompile(`^(?:[ \t]{0,3}>[ \t]?)+`)
	// closerTail is what a closing fence may carry and still close: the
	// emphasis delimiter of the statement it sits in.
	closerTail = regexp.MustCompile(`^[*_]*$`)
)

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
// line, End one past its last, and Info the opener's info string.
type Fence struct {
	Start, End int
	Info       string
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
			out[len(out)-1].End = i + 1
		}
	}
	return out
}

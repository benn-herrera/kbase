package distill

import "strings"

// The link-destination scanner.
//
// # Why this exists here and not behind the format seam
//
// ARCHITECTURE §4's format seam says stages downstream of the survey consume
// artifact fields and byte offsets, never a parser. §4.5 then asks stage 5 to
// rewrite link TARGETS inside a leaf's bytes — and `survey.Link` carries the
// destination as written, the resolved document and the fragment, but NO byte
// offsets, and it collapses repeats of one target to a single entry. So the
// artifact says which destinations a file contains and what they mean, and
// says nothing about where they are.
//
// This scanner is the smallest thing that closes that gap without a parser: it
// finds the POSITIONS a destination can occupy and reads the literal bytes
// there. It never interprets them — every decision about what a destination
// means is the survey's, and a destination this scanner finds is only ever
// rewritten when its exact bytes match one the survey already classified as
// internal (see rebase.go). Two Markdown facts are hard-coded and they are
// listed here rather than discovered by reading the code:
//
//  1. an inline destination sits between `](` and its matching `)`, optionally
//     angle-bracketed and optionally followed by a title;
//  2. a reference definition is `[label]: destination` at the start of a line.
//
// Fenced code blocks are skipped, because a destination-shaped run of bytes
// inside a fence is source content a leaf must reproduce verbatim (I-3) and
// not a link anybody could follow. Inline code spans are NOT skipped: closing
// that hole means tracking backtick runs across the whole file for a case
// where the bytes would additionally have to equal a real internal link
// target elsewhere in the same document, and the cheap check that always runs
// beats the thorough one.
//
// The honest alternative is byte offsets on `survey.Link`, emitted by the
// adapter that already knows them (goldmark hands them over). That is a survey
// schema change and is recorded as a finding rather than taken unilaterally;
// when it lands, this file is deleted and Destinations becomes a read of the
// artifact.

// Destination is one link target found in Markdown source: the half-open byte
// range of the destination text itself, brackets and title excluded.
type Destination struct {
	Start int
	End   int
}

// Text returns the destination's own bytes out of the source it was found in.
func (d Destination) Text(src []byte) string { return string(src[d.Start:d.End]) }

// Destinations returns every link destination in src, in order.
func Destinations(src []byte) []Destination {
	var out []Destination
	unfenced(string(src), func(at int, line string) {
		if d, ok := refDefinition(line); ok {
			out = append(out, Destination{Start: at + d.Start, End: at + d.End})
			return
		}
		for _, d := range inlineDestinations(line) {
			out = append(out, Destination{Start: at + d.Start, End: at + d.End})
		}
	})
	return out
}

// unfenced calls fn with every line of s that sits OUTSIDE a fenced code
// block, and the offset that line starts at.
//
// It is the fence walk itself, factored out so that "is this line inside a
// fence" has one implementation: a destination-shaped run of bytes inside a
// fence is source content a leaf must reproduce verbatim (I-3), not a link
// anybody can follow, and a second walk would be a second answer to that
// question. Page GRAMMAR is not asked here at all — the navigation block is
// positional and stops before the first source byte (distill.NavBlock), which
// is the scoping that keeps a `[↑ ` line in a verbatim leaf from refusing a
// whole delivery [MAD2: B-11].
func unfenced(s string, fn func(at int, line string)) {
	fenced := false
	for _, ln := range lines(s) {
		line := s[ln.Start:ln.End]
		if fenceMarker(line) != "" {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		fn(ln.Start, line)
	}
}

// lines splits src into line ranges, newline excluded.
func lines(s string) []Destination {
	var out []Destination
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, Destination{Start: start, End: i})
			start = i + 1
		}
	}
	if start <= len(s) {
		out = append(out, Destination{Start: start, End: len(s)})
	}
	return out
}

// fenceMarker returns the fence a line opens or closes, or "".
//
// Three or more backticks or tildes, indented at most three spaces — the
// CommonMark rule, and the only piece of block structure this scanner needs.
func fenceMarker(line string) string {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return ""
	}
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(t, m) {
			return m
		}
	}
	return ""
}

// refDefinition matches `[label]: destination` at the head of a line.
func refDefinition(line string) (Destination, bool) {
	t := strings.TrimLeft(line, " ")
	indent := len(line) - len(t)
	if indent > 3 || !strings.HasPrefix(t, "[") {
		return Destination{}, false
	}
	close := strings.Index(t, "]:")
	if close < 0 {
		return Destination{}, false
	}
	rest := t[close+2:]
	off := indent + close + 2
	lead := len(rest) - len(strings.TrimLeft(rest, " \t"))
	d, ok := destinationRun(strings.TrimLeft(rest, " \t"))
	if !ok {
		return Destination{}, false
	}
	return Destination{Start: off + lead + d.Start, End: off + lead + d.End}, true
}

// inlineDestinations finds every `](…)` destination in one line.
func inlineDestinations(line string) []Destination {
	var out []Destination
	for i := 0; i+1 < len(line); i++ {
		if line[i] != ']' || line[i+1] != '(' {
			continue
		}
		if escaped(line, i) {
			continue
		}
		d, ok := destinationRun(line[i+2:])
		if !ok {
			continue
		}
		out = append(out, Destination{Start: i + 2 + d.Start, End: i + 2 + d.End})
		i += 1 + d.End
	}
	return out
}

// destinationRun reads a destination off the head of s: either `<…>` or a run
// ending at the first whitespace or unbalanced `)`.
func destinationRun(s string) (Destination, bool) {
	if s == "" {
		return Destination{}, false
	}
	if s[0] == '<' {
		if end := strings.IndexByte(s, '>'); end > 0 {
			return Destination{Start: 1, End: end}, true
		}
		return Destination{}, false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == ' ' || s[i] == '\t':
			return Destination{Start: 0, End: i}, i > 0
		case s[i] == '(':
			depth++
		case s[i] == ')':
			if depth == 0 {
				return Destination{Start: 0, End: i}, i > 0
			}
			depth--
		}
	}
	// A reference definition's destination legitimately runs to end of line;
	// an inline one that does is malformed and yields nothing rewritable,
	// which is the same outcome either way.
	return Destination{Start: 0, End: len(s)}, true
}

// escaped reports whether the byte at i is preceded by an odd number of
// backslashes — `\]` is a literal bracket and opens no link.
func escaped(s string, i int) bool {
	n := 0
	for i-n-1 >= 0 && s[i-n-1] == '\\' {
		n++
	}
	return n%2 == 1
}

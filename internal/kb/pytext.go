package kb

import (
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"kbase/internal/formats/format40"
)

// The metadata layer's readers follow Python's text semantics: a file read in
// text mode, str.splitlines, str.strip, and regular expressions whose \s, \w
// and \b are Unicode-aware. Go's counterparts differ at the edges, so the
// readers here go through these.

// PyWhitespace is the body of the class Python's \s and str.isspace() share,
// for use inside a bracket expression.
const PyWhitespace = format40.PyWhitespace

// PyRE compiles a pattern written in Python's spelling of \s, \S and \d,
// none of them inside a bracket expression. \d stays ASCII.
func PyRE(pattern string) *regexp.Regexp { return format40.PyRE(pattern) }

// TranslateNewlines is text as Python's text-mode read returns it: \r\n and
// \r translated to \n.
func TranslateNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

// IsFile is whether path names a regular file, following symlinks.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isLineBreak is a character str.splitlines breaks on.
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// SplitLines is str.splitlines(): no trailing empty line, \r\n one break.
func SplitLines(text string) []string {
	parts := SplitLinesKeepEnds(text)
	for i, p := range parts {
		parts[i] = p[:len(p)-len(terminator(p))]
	}
	return parts
}

// SplitLinesKeepEnds is str.splitlines(keepends=True): the parts concatenate
// back to text.
func SplitLinesKeepEnds(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		if !isLineBreak(r) {
			continue
		}
		if r == '\r' && i < len(text) && text[i] == '\n' {
			i++
		}
		out = append(out, text[start:i])
		start = i
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// terminator is the line break a keepends line ends with, or "".
func terminator(line string) string {
	if strings.HasSuffix(line, "\r\n") {
		return "\r\n"
	}
	r, size := utf8.DecodeLastRuneInString(line)
	if size > 0 && isLineBreak(r) {
		return line[len(line)-size:]
	}
	return ""
}

// IsSpace is str.isspace() for one character.
func IsSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r >= 0x1c && r <= ' ', r == 0x85, r == 0xa0, r == 0x1680,
		r >= 0x2000 && r <= 0x200a, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

// Strip is str.strip().
func Strip(s string) string { return strings.TrimFunc(s, IsSpace) }

// LStrip is str.lstrip().
func LStrip(s string) string { return strings.TrimLeftFunc(s, IsSpace) }

var spaceRunRE = PyRE(`\s+`)

// NormalizeSpace collapses every whitespace run to one space and strips.
func NormalizeSpace(s string) string {
	return Strip(spaceRunRE.ReplaceAllString(s, " "))
}

// IsWord is whether r is a character Python's \w matches.
func IsWord(r rune) bool { return format40.IsWord(r) }

// BoundaryAt is Python's \b at byte offset i of s.
func BoundaryAt(s string, i int) bool { return format40.BoundaryAt(s, i) }

// findAllBounded is the matches of re in s that Python's \b(...)\b would
// keep. Exact only where a rejected match cannot overlap an accepted one,
// which holds for every pattern here: each opens on a literal prefix its own
// body cannot contain.
func findAllBounded(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringIndex(s, -1) {
		if BoundaryAt(s, m[0]) && BoundaryAt(s, m[1]) {
			out = append(out, s[m[0]:m[1]])
		}
	}
	return out
}

// Slugify is a heading's anchor as GitHub's slugger has it, HTML comments
// stripped, before any suffix for a repeat.
func Slugify(heading string) string { return format40.Slugify(heading) }

// Slugger anchors the headings of one document in document order, a repeat
// suffixed as GitHub's slugger suffixes it.
type Slugger = format40.Slugger

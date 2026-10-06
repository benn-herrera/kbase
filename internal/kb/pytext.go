package kb

import (
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The metadata layer is kb_tools' bytes, and kb_tools reads them with
// Python's text semantics: a file read in text mode, str.splitlines,
// str.strip, and regular expressions whose \s, \w and \b are Unicode-aware.
// Go's counterparts differ at the edges, so the readers here go through these.

// PyWhitespace is the body of the class Python's \s and str.isspace() share,
// for use inside a bracket expression.
const PyWhitespace = `\t\n\v\f\r\x1c-\x1f \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var pyEscapes = strings.NewReplacer(`\s`, "["+PyWhitespace+"]", `\S`, "[^"+PyWhitespace+"]", `\d`, `[0-9]`)

// PyRE compiles a pattern written in Python's spelling of \s, \S and \d,
// none of them inside a bracket expression. \d stays ASCII.
func PyRE(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pyEscapes.Replace(pattern))
}

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

// RStrip is str.rstrip().
func RStrip(s string) string { return strings.TrimRightFunc(s, IsSpace) }

// LStrip is str.lstrip().
func LStrip(s string) string { return strings.TrimLeftFunc(s, IsSpace) }

var spaceRunRE = PyRE(`\s+`)

// NormalizeSpace collapses every whitespace run to one space and strips.
func NormalizeSpace(s string) string {
	return Strip(spaceRunRE.ReplaceAllString(s, " "))
}

// IsWord is whether r is a character Python's \w matches.
func IsWord(r rune) bool { return isWord(r) }

// isWord is a character Python's \w matches.
func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// boundaryAt is Python's \b at byte offset i of s.
func boundaryAt(s string, i int) bool {
	before, after := false, false
	if i > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:i])
		before = isWord(r)
	}
	if i < len(s) {
		r, _ := utf8.DecodeRuneInString(s[i:])
		after = isWord(r)
	}
	return before != after
}

// findAllBounded is the matches of re in s that Python's \b(...)\b would
// keep. Exact only where a rejected match cannot overlap an accepted one,
// which holds for every pattern here: each opens on a literal prefix its own
// body cannot contain.
func findAllBounded(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringIndex(s, -1) {
		if boundaryAt(s, m[0]) && boundaryAt(s, m[1]) {
			out = append(out, s[m[0]:m[1]])
		}
	}
	return out
}

// containsBounded is re.search(r"\b" + re.escape(token) + r"\b", s).
func containsBounded(s, token string) bool {
	for i := 0; i <= len(s)-len(token); {
		j := strings.Index(s[i:], token)
		if j < 0 {
			return false
		}
		at := i + j
		if boundaryAt(s, at) && boundaryAt(s, at+len(token)) {
			return true
		}
		_, size := utf8.DecodeRuneInString(s[at:])
		i = at + max(size, 1)
	}
	return false
}

// Slugify is the heading anchor kb_tools computes: lowercased, everything but
// word characters, whitespace and hyphens dropped, then each whitespace run
// one hyphen. The collapse is not GitHub's anchor: "A — B" anchors at "a-b",
// the form every KB's stored canonical_anchor carries.
func Slugify(text string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range strings.ToLower(Strip(text)) {
		switch {
		case IsSpace(r):
			if !inSpace {
				b.WriteByte('-')
			}
			inSpace = true
		case isWord(r) || r == '-':
			b.WriteRune(r)
			inSpace = false
		}
	}
	return strings.Trim(b.String(), "-")
}

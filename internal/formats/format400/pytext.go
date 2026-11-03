package format400

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PyWhitespace is the body of the class Python's \s and str.isspace() share,
// for use inside a bracket expression.
const PyWhitespace = `\t\n\v\f\r\x1c-\x1f \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var pyEscapes = strings.NewReplacer(`\s`, "["+PyWhitespace+"]", `\S`, "[^"+PyWhitespace+"]", `\d`, `[0-9]`)

// PyRE compiles a pattern written in Python's spelling of \s, \S and \d,
// none of them inside a bracket expression. \d stays ASCII.
func PyRE(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pyEscapes.Replace(pattern))
}

var (
	htmlCommentRE     = regexp.MustCompile(`(?s)<!--.*?-->`)
	closingSequenceRE = regexp.MustCompile(`[ \t]#+[ \t]*$`)
)

// Slugify is a heading's anchor as GitHub's slugger has it, before any suffix
// for a repeat. It reads the heading's text as GitHub renders it: a closing
// run of #s dropped, HTML comments stripped, an inline link or image reduced
// to its bracketed text and a _ emphasis to its content, inline code and
// maths spans kept verbatim, and the result trimmed. That text is then
// lowercased, every character but a letter, a mark, a number, a connector, a
// space or a hyphen dropped, and each space one hyphen, runs kept: "A — B"
// anchors at "a--b". A reference-style link and inline HTML other than a
// comment are not reduced.
func Slugify(heading string) string {
	text := strings.ToLower(strings.TrimSpace(renderedHeading(heading)))
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.Pc):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// renderedHeading is a heading's text, after its # run, reduced to the text
// GitHub renders, untrimmed. The closing sequence is read on the line as
// written, before comments go, as GitHub reads it.
func renderedHeading(heading string) string {
	text := htmlCommentRE.ReplaceAllString(closingSequenceRE.ReplaceAllString(heading, ""), "")
	blanked := BlankInlineCode(text)
	var b strings.Builder
	last := 0
	for _, l := range Links(blanked) {
		start := l.Start
		if start > 0 && text[start-1] == '!' {
			start--
		}
		b.WriteString(text[last:start])
		b.WriteString(text[l.Start+1 : l.Start+1+len(l.Text)])
		last = l.End
	}
	b.WriteString(text[last:])
	return dropUnderscoreEmphasis(b.String())
}

// dropUnderscoreEmphasis is text with each _ run that opens emphasis removed
// together with the next _ run that closes it. A run opens where no letter or
// number precedes it and a non-space follows, and closes where a non-space
// precedes it and no letter or number follows; an unpaired or intraword run
// stays, as does every _ inside an inline code or maths span.
func dropUnderscoreEmphasis(text string) string {
	blanked := BlankInlineCode(text)
	var runs [][2]int
	for i := 0; i < len(blanked); {
		if blanked[i] != '_' {
			i++
			continue
		}
		j := i
		for j < len(blanked) && blanked[j] == '_' {
			j++
		}
		runs = append(runs, [2]int{i, j})
		i = j
	}
	wordAt := func(r rune, ok bool) bool { return ok && (unicode.IsLetter(r) || unicode.IsNumber(r)) }
	spaceAt := func(r rune, ok bool) bool { return !ok || unicode.IsSpace(r) }
	opens := func(r [2]int) bool { return !wordAt(runeBefore(text, r[0])) && !spaceAt(runeAfter(text, r[1])) }
	closes := func(r [2]int) bool { return !spaceAt(runeBefore(text, r[0])) && !wordAt(runeAfter(text, r[1])) }
	var b strings.Builder
	last := 0
	for i := 0; i < len(runs); i++ {
		if !opens(runs[i]) {
			continue
		}
		for k := i + 1; k < len(runs); k++ {
			if closes(runs[k]) {
				b.WriteString(text[last:runs[i][0]])
				b.WriteString(text[runs[i][1]:runs[k][0]])
				last, i = runs[k][1], k
				break
			}
		}
	}
	b.WriteString(text[last:])
	return b.String()
}

// runeBefore is the rune ending at byte offset i of s; ok is false at its start.
func runeBefore(s string, i int) (r rune, ok bool) {
	if i == 0 {
		return 0, false
	}
	r, _ = utf8.DecodeLastRuneInString(s[:i])
	return r, true
}

// runeAfter is the rune starting at byte offset i of s; ok is false at its end.
func runeAfter(s string, i int) (r rune, ok bool) {
	if i == len(s) {
		return 0, false
	}
	r, _ = utf8.DecodeRuneInString(s[i:])
	return r, true
}

// Slugger anchors the headings of one document in document order: a heading
// whose slug an earlier one took gets the first free suffix -1, -2, as GitHub's
// slugger gives it.
type Slugger struct{ seen map[string]int }

// Slug is the anchor of the document's next heading.
func (s *Slugger) Slug(heading string) string {
	if s.seen == nil {
		s.seen = map[string]int{}
	}
	base := Slugify(heading)
	slug := base
	for {
		if _, taken := s.seen[slug]; !taken {
			break
		}
		s.seen[base]++
		slug = base + "-" + strconv.Itoa(s.seen[base])
	}
	s.seen[slug] = 0
	return slug
}

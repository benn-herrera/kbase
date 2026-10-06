package claimgraph

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"kbase/internal/kb"
)

// The sentence-labelled render a paragraph ask shows its leaf in, and the one
// paragraph rule the node pass and stage D share. One sentence per line,
// wraps collapsed; paragraph breaks kept as the write API's excerpt matching
// sees them, so a paragraph here is a run of lines a slice can span; the
// navigation above the body shown unlabelled, so no locator can name it.

const labelPrefix = "S"

var (
	headingRE    = kb.PyRE(`^#{1,6}\s`)
	listItemRE   = kb.PyRE(`^(?:[-*+]\s|\d+[.)]\s)`)
	tokenRE      = kb.PyRE(`\S+`)
	terminatorRE = regexp.MustCompile("[.!?][\"')\\]*_`»”’]* ")
	// opensSentenceRE is what may open the next sentence: a lowercase letter
	// after a stop is an abbreviation the list below did not carry.
	opensSentenceRE = regexp.MustCompile("^[A-Z0-9$\\\\`*_(\\[\"'#—-]$")
	initialRE       = regexp.MustCompile(`^[A-Za-z]\.$`)
)

// abbreviations are the tokens ending in a stop a capital may follow without
// opening a sentence.
var abbreviations = func() map[string]bool {
	out := map[string]bool{}
	for _, a := range strings.Fields(`al. app. approx. cf. ch. cor. def. dr. e.g. ed. eds. eq. eqn. eqns. eqs. etc.
		fig. figs. i.e. incl. lem. max. min. mr. mrs. ms. no. p. pp. prof. prop. ref.
		refs. resp. sec. sect. secs. st. thm. trans. viz. vol. vols. vs.`) {
		out[a] = true
	}
	return out
}()

// sentence is one labelled line of the render: its collapsed text, its byte
// range in the rendered source, the 0-based line it begins on, and the
// paragraph it belongs to.
type sentence struct {
	label, text string
	start, end  int
	line        int
	paragraph   int
}

// span is a run of sentences and the slice of the document it names.
type span []sentence

func (s span) locator() string {
	if len(s) == 0 {
		return ""
	}
	if len(s) == 1 {
		return s[0].label
	}
	return s[0].label + "-" + s[len(s)-1].label
}

// excerpt is the tool's own slice: the author's words, nothing a seat wrote.
func (s span) excerpt() string {
	texts := make([]string, len(s))
	for i, x := range s {
		texts[i] = x.text
	}
	return strings.Join(texts, " ")
}

// paragraph is a unit no slice may leave, named outside this file by start,
// the 0-based line it begins on.
type paragraph struct {
	index, start int
	lines        []int
}

func (p paragraph) holds(line int) bool { return slices.Contains(p.lines, line) }

// render is one document as the ask shows it.
type render struct {
	text       string
	sentences  []sentence
	paragraphs []paragraph
}

func (r *render) paragraphAt(line int) (paragraph, bool) {
	for _, p := range r.paragraphs {
		if p.holds(line) {
			return p, true
		}
	}
	return paragraph{}, false
}

func (r *render) spanOf(p paragraph) span {
	var out span
	for _, s := range r.sentences {
		if s.paragraph == p.index {
			out = append(out, s)
		}
	}
	return out
}

// bodyStart is the first line a label may name: below the frontmatter and
// the up-link line that follows it.
func bodyStart(text string) int {
	if m := kb.FindFrontmatter(text); m != nil {
		return strings.Count(text[:m[1]], "\n") + 2
	}
	return 1
}

// headingLines is every line of the labelled region that is a heading.
func headingLines(text string) map[int]bool {
	out := map[int]bool{}
	first := bodyStart(text)
	for i, line := range kb.SplitLines(text) {
		if i >= first && headingRE.MatchString(kb.Strip(kb.BlockquotePrefix.ReplaceAllString(line, ""))) {
			out[i] = true
		}
	}
	return out
}

// insideRun is whether at sits inside an unclosed run of delimiter: inline
// maths, or a code span.
func insideRun(text string, at int, delimiter byte) bool {
	opened := 0
	for i := range at {
		if text[i] == delimiter && (i == 0 || text[i-1] != '\\') {
			opened++
		}
	}
	return opened%2 == 1
}

// sentenceBounds is where each sentence of one collapsed paragraph begins and
// ends. Heuristic: a label naming half a sentence still cuts the author's own
// bytes.
func sentenceBounds(text string) [][2]int {
	var bounds [][2]int
	start := 0
	for _, m := range terminatorRE.FindAllStringIndex(text, -1) {
		end := m[1] - 1
		if end <= start {
			continue
		}
		next, _ := utf8.DecodeRuneInString(text[end+1:])
		if end+1 >= len(text) || !opensSentenceRE.MatchString(string(next)) {
			continue
		}
		if insideRun(text, m[0], '$') || insideRun(text, m[0], '`') {
			continue
		}
		token := text[:end]
		if i := strings.LastIndex(token, " "); i >= 0 {
			token = token[i+1:]
		}
		if abbreviations[strings.ToLower(token)] || initialRE.MatchString(token) {
			continue
		}
		bounds = append(bounds, [2]int{start, end})
		start = end + 1
	}
	if start < len(text) {
		bounds = append(bounds, [2]int{start, len(text)})
	}
	return bounds
}

// group is one run of source lines treated as a unit before segmenting.
type group struct {
	kind  string
	lines []int
	opens bool
}

const (
	groupProse    = "prose"
	groupExcluded = "excluded"
	groupFence    = "fence"
	groupBreak    = "break"
	groupHeading  = "heading"
)

func groups(lines []string, first int, fenced, excluded map[int]bool) []group {
	var out []group
	var prose []int
	opens := false
	flush := func() {
		if len(prose) > 0 {
			out = append(out, group{groupProse, prose, opens})
			prose = nil
		}
		opens = false
	}
	for n := first; n < len(lines); n++ {
		line := kb.Strip(kb.BlockquotePrefix.ReplaceAllString(lines[n], ""))
		switch {
		case excluded[n]:
			flush()
			out = append(out, group{groupExcluded, []int{n}, false})
		case fenced[n]:
			flush()
			out = append(out, group{groupFence, []int{n}, false})
		case line == "":
			flush()
			out = append(out, group{kind: groupBreak})
		case headingRE.MatchString(line):
			flush()
			out = append(out, group{groupHeading, []int{n}, true})
		case listItemRE.MatchString(line):
			flush()
			opens = true
			prose = append(prose, n)
		default:
			prose = append(prose, n)
		}
	}
	flush()
	return out
}

type piece struct {
	text       string
	start, end int
}

// pieces is one group's sentences: collapsed text and byte range in the
// source.
func pieces(lines []string, offsets []int, g group) []piece {
	if g.kind == groupFence || g.kind == groupHeading {
		n := g.lines[0]
		return []piece{{kb.NormalizeSpace(kb.BlockquotePrefix.ReplaceAllString(lines[n], "")), offsets[n], offsets[n] + len(lines[n])}}
	}
	type token struct {
		text string
		at   int
	}
	var tokens []token
	for _, n := range g.lines {
		cut := 0
		if m := kb.BlockquotePrefix.FindStringIndex(lines[n]); m != nil {
			cut = m[1]
		}
		for _, m := range tokenRE.FindAllStringIndex(lines[n][cut:], -1) {
			tokens = append(tokens, token{lines[n][cut+m[0] : cut+m[1]], offsets[n] + cut + m[0]})
		}
	}
	texts := make([]string, len(tokens))
	starts := make([]int, len(tokens))
	at := 0
	for i, t := range tokens {
		texts[i], starts[i] = t.text, at
		at += len(t.text) + 1
	}
	collapsed := strings.Join(texts, " ")
	var out []piece
	for _, b := range sentenceBounds(collapsed) {
		head, tail := 0, 0
		for i, s := range starts {
			if s <= b[0] {
				head = i
			}
			if s < b[1] {
				tail = i
			}
		}
		out = append(out, piece{collapsed[b[0]:b[1]], tokens[head].at, tokens[tail].at + len(tokens[tail].text)})
	}
	return out
}

// renderText is text as the ask shows it: navigation verbatim, then one
// labelled sentence per line. Excluded lines are shown as they stand, carry
// no label, and end the paragraph they interrupt.
func renderText(text string, fences []MathFence, excluded map[int]bool) render {
	lines := kb.SplitLines(text)
	offsets := make([]int, len(lines))
	cursor := 0
	for i, l := range lines {
		offsets[i] = cursor
		cursor += len(l) + 1
	}
	first := bodyStart(text)
	fenced := map[int]bool{}
	for _, f := range fences {
		for n := f.Start; n < f.End; n++ {
			fenced[n] = true
		}
	}
	shown := slices.Clone(lines[:min(first, len(lines))])
	if first < len(lines) {
		shown = append(shown, "")
	}
	var r render
	held := map[int][]int{}
	var order []int
	paragraphIndex := 0
	previous := groupBreak
	for _, g := range groups(lines, first, fenced, excluded) {
		switch g.kind {
		case groupBreak:
			if previous != groupBreak {
				shown = append(shown, "")
			}
			previous = groupBreak
			continue
		case groupExcluded:
			shown = append(shown, lines[g.lines[0]])
			previous = groupExcluded
			continue
		}
		if previous == groupBreak || previous == groupHeading || previous == groupExcluded || g.opens {
			paragraphIndex++
		}
		previous = g.kind
		if _, ok := held[paragraphIndex]; !ok {
			order = append(order, paragraphIndex)
		}
		held[paragraphIndex] = append(held[paragraphIndex], g.lines...)
		for _, p := range pieces(lines, offsets, g) {
			if p.text == "" {
				continue
			}
			label := fmt.Sprintf("%s%d", labelPrefix, len(r.sentences)+1)
			r.sentences = append(r.sentences, sentence{label, p.text, p.start, p.end, strings.Count(text[:p.start], "\n"), paragraphIndex})
			shown = append(shown, label+": "+p.text)
		}
	}
	for _, i := range order {
		r.paragraphs = append(r.paragraphs, paragraph{index: i, start: slices.Min(held[i]), lines: held[i]})
	}
	r.text = strings.Join(shown, "\n") + "\n"
	return r
}

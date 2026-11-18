// Package score is the scoring pass: the closed text one call carries, and the
// run that lands its answers.
package score

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/kb"
)

// headingRE is a line that opens an ATX heading, which ends a cited range.
var headingRE = regexp.MustCompile(`^#{1,6}\s`)

// outOfScopeError is a subject neither score pass asks about: the node kind or
// the relation named, on the id given.
type outOfScopeError struct{ Kind, ID string }

func (e outOfScopeError) Error() string {
	return fmt.Sprintf("%s %s is not a subject of the score passes", e.Kind, e.ID)
}

// passage is one piece of assembled text: end is the passage a charge's lines
// are recorded as, "" where none may lie; text is the passage as the call
// carries it. A label line — a bearing end's, a rigor title's heading mark — is
// carried on a line of its own and holds no number.
type passage struct {
	end  string
	text string
}

// closedText is the text one call carries and the lines it numbers: text is the
// carried text, each passage wrapped at word boundaries with its own breaks kept,
// and the gutter the rendering writes before a numbered line is no part of it;
// lines is one entry per line of it, in its order.
type closedText struct {
	text  string
	lines []line
}

// line is one line of the carried text: no is the number the call writes before
// it, 0 where it carries none; end is the passage its bytes lie in, "" where it
// is no site a charge may cite; from and to are its bytes in the carried text,
// the newline that follows it excluded.
type line struct {
	no       int
	end      string
	from, to int
}

// graph is the derived index as the passages read it: every node by id, and
// each hosted node's statement as kb.Statement reads it from its host leaf. A run
// builds one and every item of its walk reads that one, so no item reads an
// index file — the runner already holds the index, and a second reading path is
// how two views of the graph drift.
type graph struct {
	nodes      map[string]kb.NodeRow
	statements map[string]string
}

// rigorText is the closed text one rigor call carries: the node's title and its
// statement.
func rigorText(g graph, id string) (closedText, error) {
	row, ok := g.nodes[id]
	if !ok {
		return closedText{}, fmt.Errorf("the index holds no node %s", id)
	}
	if row.NodeType == kb.NodeKindWork || row.NodeType == kb.NodeKindAxiom {
		return closedText{}, outOfScopeError{Kind: row.NodeType, ID: id}
	}
	return assemble(endPassages(g, row, titleLabel, nodeEnds{title: passageTitle, statement: passageStatement})), nil
}

// The labels a bearing call writes: above each end's title, and the line above
// the edge's own note. They are the frame's own — carried on a line of their own,
// holding no number, and no citation's home.
const (
	sourceLabel  = "## Source:"
	targetLabel  = "## Target:"
	contextLabel = "## Context"
	// titleLabel is what a rigor call writes above the node's title.
	titleLabel = "#"
)

// bearingText is the closed text one bearing call carries: the source's
// statement, the target's, and the edge's authored context. Both ends render by
// the rule rigorText renders their node, so the target's passage is the text its
// own rigor call shows. The context is no citation's home: it states bridges and
// excuses, and a charge is about what the two texts do and do not do.
func bearingText(g graph, e kb.EdgeRow) (closedText, error) {
	if !kb.Scoring(e.Relation) {
		return closedText{}, outOfScopeError{Kind: e.Relation, ID: e.Source + " → " + e.Target}
	}
	source, ok := g.nodes[e.Source]
	if !ok {
		return closedText{}, fmt.Errorf("the index holds no node %s", e.Source)
	}
	target, ok := g.nodes[e.Target]
	if !ok {
		return closedText{}, fmt.Errorf("the index holds no node %s", e.Target)
	}
	passages := append(endPassages(g, source, sourceLabel, nodeEnds{title: passageSource, statement: passageSource}),
		endPassages(g, target, targetLabel, nodeEnds{title: passageTarget, statement: passageTarget})...)
	if e.Context != nil {
		passages = append(passages, passage{text: contextLabel}, passage{text: *e.Context})
	}
	return assemble(passages), nil
}

// nodeEnds is the end a node's title is recorded as and the end its statement is:
// a rigor call reads the two apart, and a bearing call records both as the one end
// they belong to.
type nodeEnds struct{ title, statement string }

// endPassages is one node's label line, then its title and its statement, each
// recorded as ends names. The label is the frame's own and is carried on a line
// of its own, so no numbered line begins with frame text. A node no leaf hosts
// — a work, whose standing lies outside the corpus, or a node with no marker —
// has no statement, and its text is its title alone.
func endPassages(g graph, row kb.NodeRow, label string, ends nodeEnds) []passage {
	out := []passage{{text: label}, {end: ends.title, text: row.Title}}
	if statement := g.statements[row.ID]; statement != "" {
		out = append(out, passage{end: ends.statement, text: statement})
	}
	return out
}

// wrapWidth is the column a passage's lines are wrapped to. Wrapping adds breaks
// and rewrites nothing, so a line of the carried text stays a verbatim slice of
// the passage it came from.
const wrapWidth = 100

// assemble is the passages as one text, wrapped and numbered. A passage with an
// end is a site and its lines are numbered; a label line and the edge's note are
// carried and hold no number.
func assemble(passages []passage) closedText {
	var (
		ct closedText
		no int // the last number written
	)
	for _, p := range passages {
		text := strings.TrimSpace(p.text)
		if text == "" {
			continue
		}
		if ct.text != "" {
			ct.text += "\n\n"
			ct.lines = append(ct.lines, line{from: len(ct.text), to: len(ct.text)})
		}
		for i, w := range wrapPassage(text) {
			if i > 0 {
				ct.text += "\n"
			}
			l := line{end: p.end, from: len(ct.text), to: len(ct.text) + len(w)}
			if p.end != "" && strings.TrimSpace(w) != "" {
				no++
				l.no = no
			}
			ct.text += w
			ct.lines = append(ct.lines, l)
		}
	}
	return ct
}

// wrapPassage is one passage's lines as the call carries them.
func wrapPassage(text string) (wrapped []string) {
	for _, raw := range strings.Split(text, "\n") {
		wrapped = append(wrapped, wrapLine(raw)...)
	}
	return wrapped
}

// wrapLine breaks one source line at its spaces so that no line it returns runs
// past wrapWidth, inserting breaks and removing nothing: the lines joined by
// newlines are the line's own bytes. A word wider than the width stands whole on
// a line of its own rather than being cut, and no break falls inside a `$…$` or
// `$$…$$` span, whose spaces belong to the formula.
func wrapLine(s string) []string {
	breaks := breakPoints(s)
	var out []string
	from := 0
	for len(s)-from > wrapWidth {
		// The last break this line may end after and still fit, else the first
		// break past it, so a word too wide for the line is not cut.
		cut := -1
		for _, i := range breaks {
			if i < from {
				continue
			}
			if i+1-from > wrapWidth {
				if cut < 0 {
					cut = i
				}
				break
			}
			cut = i
		}
		if cut < 0 {
			break
		}
		out = append(out, s[from:cut+1])
		from = cut + 1
	}
	return append(out, s[from:])
}

// breakPoints is the indices of the spaces a wrapped line may end after: every
// space no formula holds. A run of dollars opens a span only where a run of the
// same length closes it further on in the line — a lone `$`, as in a price,
// opens none — and a backslash escapes the character after it, `\$` among them.
func breakPoints(s string) []int {
	var (
		out  []int
		open int // the dollar run that opened the span; 0 where none stands open
	)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '$':
			run := 1
			for i+run < len(s) && s[i+run] == '$' {
				run++
			}
			switch {
			case open == run:
				open = 0
			case open == 0 && strings.Contains(s[i+run:], s[i:i+run]):
				open = run
			}
			i += run - 1
		case ' ':
			if open == 0 {
				out = append(out, i)
			}
		}
	}
	return out
}

// numbered is the text as the call carries it: each numbered line behind its
// number and the gutter, `N| `, numbered from 1 over the whole text. A line the
// gutter does not reach — a label, a blank line, the edge's note — is carried as
// it stands: a line with no number is no site.
func (ct closedText) numbered() string {
	var b strings.Builder
	for i, l := range ct.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if l.no > 0 {
			b.WriteString(strconv.Itoa(l.no))
			b.WriteString("| ")
		}
		b.WriteString(ct.text[l.from:l.to])
	}
	return b.String()
}

// at is the entry of the carried text's lines the number no is written on, ok
// false where the call shows no such number.
func (ct closedText) at(no int) (int, bool) {
	if no < 1 {
		return 0, false
	}
	for i, l := range ct.lines {
		if l.no == no {
			return i, true
		}
	}
	return 0, false
}

// linesRE is the shape of a charge's line field: one line number, or a range of
// two, joined by a hyphen, an en dash, an em dash or a minus sign, spaces around
// the dash allowed.
var linesRE = regexp.MustCompile(`^([0-9]+)\s*(?:[-–—−]\s*([0-9]+))?$`)

// cite is a charge's line field resolved: the passage its lines lie in, the
// numbers as the ledger records them, the span of the carried text they quote and
// that span's offset in it, and the numbers themselves, which is what two charges'
// line sets are compared by.
type cite struct {
	end      string
	lines    string
	citation string
	offset   int
	lo, hi   int
}

// lineFault is why a charge's line field names no lines the call shows: the
// refusal case (an asks.Refusal… constant), the cited numbers that case names as
// the reply wrote them — from alone for one number, both for a range — and the
// reason's text.
type lineFault struct {
	refusal  string
	from, to string
	why      string
}

// look resolves field as a charge's line field against the text the call carried:
// one number, or a range N-M with N at most M, every number one the call shows,
// all of them in one passage and with nothing between them — a blank line, a label
// or a heading ends a range. Where it names no such lines it returns the fault,
// which the caller's refusal names, and nil otherwise.
func (ct closedText) look(field string) (cite, *lineFault) {
	m := linesRE.FindStringSubmatch(field)
	if m == nil {
		return cite{}, &lineFault{refusal: asks.RefusalFieldNamesNoLine, why: "field names no line: a charge names one line number"}
	}
	// notShown names the number as the reply wrote it, and the reason as the
	// parse read it: a number too long for an int is read as written.
	notShown := func(written string, read any) (cite, *lineFault) {
		return cite{}, &lineFault{refusal: asks.RefusalLineNotShown, from: written, why: fmt.Sprintf("line %v is not one the call shows", read)}
	}
	lo, err := strconv.Atoi(m[1])
	if err != nil {
		return notShown(m[1], m[1])
	}
	hi := lo
	if m[2] != "" {
		if hi, err = strconv.Atoi(m[2]); err != nil {
			return notShown(m[2], m[2])
		}
	}
	ranged := func(refusal, format string) (cite, *lineFault) {
		return cite{}, &lineFault{refusal: refusal, from: m[1], to: m[2], why: fmt.Sprintf(format, lo, hi)}
	}
	if hi < lo {
		return ranged(asks.RefusalRangeBackwards, "range %d-%d runs backwards: a charge names one line number")
	}
	i, ok := ct.at(lo)
	if !ok {
		return notShown(m[1], lo)
	}
	j, ok := ct.at(hi)
	if !ok {
		return notShown(m[2], hi)
	}
	first, last := ct.lines[i], ct.lines[j]
	if first.end != last.end {
		return ranged(asks.RefusalRangeCrossesParts, "range %d-%d crosses from one passage to another")
	}
	if j-i != hi-lo || ct.crossesHeading(i, j) {
		return ranged(asks.RefusalRangeCrossesGap, "range %d-%d crosses a gap: a blank line, a label or a heading lies between its lines")
	}
	lines := strconv.Itoa(lo)
	if hi != lo {
		lines += "-" + strconv.Itoa(hi)
	}
	return cite{end: first.end, lines: lines, citation: ct.text[first.from:last.to], offset: first.from, lo: lo, hi: hi}, nil
}

// crossesHeading is whether a heading lies between the lines i and j, which ends
// a range: the lines either side of a heading are two parts of the text and no one
// site. A heading a charge names as an end of its range is not between them.
func (ct closedText) crossesHeading(i, j int) bool {
	for k := i + 1; k < j; k++ {
		l := ct.lines[k]
		if headingRE.MatchString(ct.text[l.from:l.to]) {
			return true
		}
	}
	return false
}

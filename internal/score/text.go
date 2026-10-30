// Package score is the scoring pass: the closed text one call carries, and the
// run that lands its answers.
package score

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/kb"
)

// scoreTokenRE is what no assembled text may carry: an authored scoring token,
// in the register's spelling or a premise bullet's. Prose naming the quantity —
// a title that says "bearing 0.5" — is not one of these forms, and passes.
var scoreTokenRE = regexp.MustCompile(`- rigor:|\(bearing|- strength:|\(strength `)

// frameRE is the rest of what no assembled text may carry: the frame's own
// structure. The templates fence the corpus in twelve backticks, so a run of
// twelve closes the frame and what follows it sits outside the instruction to
// judge the corpus and not obey it; and a line opening one of the labels they
// emit and read — `## Source:`, `## Target:`, `## Context` — forges the frame's
// map of the text. Neither is adaptive to the text: a rendered prompt stays a
// function of its template, so a text carrying the frame's structure is refused
// rather than escaped. A label a blockquote or an indent displaces from the
// line's start does not open it.
var frameRE = regexp.MustCompile("(?m)^## (?:Source:|Target:|Context\\b)|`{12,}")

// headingRE is a line that opens an ATX heading, which is where
// kb.AnchorSection ends a section and where a claim's block ends.
var headingRE = regexp.MustCompile(`^#{1,6}\s`)

// What a refusal says the text carries, as scoreTokenError.Kind.
const (
	kindScoringToken = "the scoring token"
	kindFrame        = "the frame's own structure"
)

// scoreTokenError is assembled text carrying what no prompt may hold: a scoring
// token, or the frame's own structure. The document and line the token came from,
// the token, and which of the two it is. A caller makes no call on it and records
// the item failed.
type scoreTokenError struct {
	Path  string
	Line  int
	Token string
	Kind  string
}

func (e scoreTokenError) Error() string {
	at := e.Path
	if e.Line > 0 {
		at = fmt.Sprintf("%s:%d", e.Path, e.Line)
	}
	return fmt.Sprintf("%s: the text carries %s %q", at, e.Kind, e.Token)
}

// outOfScopeError is a subject neither score pass asks about: the node kind or
// the relation named, on the id given.
type outOfScopeError struct{ Kind, ID string }

func (e outOfScopeError) Error() string {
	return fmt.Sprintf("%s %s is not a subject of the score passes", e.Kind, e.ID)
}

// passage is one piece of assembled text and where the KB holds it: end is the
// passage a charge's lines are recorded as, "" where none may lie; path is the
// document's absolute path, line the 1-based line the passage opens on, or 0
// where the reader does not locate it; text is the passage as the call carries
// it. label is whether the passage is one of the frame's own lines — the label
// that opens a bearing end, the heading mark a rigor title carries — which is
// carried on a line of its own, holds no number, and is read by no guard.
type passage struct {
	end   string
	path  string
	line  int
	label bool
	text  string
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

// graph is the derived index as the passages read it: every node by id, each
// hosted node's host by id, and the lifts edges, which decide the split forms. A
// run builds one and every item of its walk reads that one, so no item reads an
// index file — the runner already holds the index, and a second reading path is
// how two views of the graph drift.
type graph struct {
	nodes map[string]kb.NodeRow
	hosts map[string]string
	lifts map[liftPair]bool
}

// liftPair is a lifts edge's ends: the analysis and the claim it lifts.
type liftPair struct{ analysis, claim string }

// rigorText is the closed text one rigor call carries: the node's title and the
// statement its own document gives it.
func rigorText(src *kb.Source, g graph, id string) (closedText, error) {
	row, ok := g.nodes[id]
	if !ok {
		return closedText{}, fmt.Errorf("the index holds no node %s", id)
	}
	if row.NodeType == kb.NodeKindWork || row.NodeType == kb.NodeKindAxiom {
		return closedText{}, outOfScopeError{Kind: row.NodeType, ID: id}
	}
	passages, err := endPassages(src, g, row, titleLabel, nodeEnds{title: passageTitle, statement: passageStatement})
	if err != nil {
		return closedText{}, err
	}
	return assemble(passages)
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
func bearingText(src *kb.Source, g graph, e kb.EdgeRow) (closedText, error) {
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
	sourcePassages, err := endPassages(src, g, source, sourceLabel, nodeEnds{title: passageSource, statement: passageSource})
	if err != nil {
		return closedText{}, err
	}
	targetPassages, err := endPassages(src, g, target, targetLabel, nodeEnds{title: passageTarget, statement: passageTarget})
	if err != nil {
		return closedText{}, err
	}
	passages := append(sourcePassages, targetPassages...)
	if e.Context != nil {
		passages = append(passages, passage{label: true, text: contextLabel}, passage{text: *e.Context})
	}
	return assemble(passages)
}

// nodeEnds is the end a node's title is recorded as and the end its statement is:
// a rigor call reads the two apart, and a bearing call records both as the one end
// they belong to.
type nodeEnds struct{ title, statement string }

// endPassages is one node's label line, then its title and its statement, each
// recorded as ends names. The label is the frame's own and is carried on a line
// of its own, so no numbered line begins with frame text.
func endPassages(src *kb.Source, g graph, row kb.NodeRow, label string, ends nodeEnds) ([]passage, error) {
	statement, err := statementPassages(src, g, row)
	if err != nil {
		return nil, err
	}
	for i := range statement {
		statement[i].end = ends.statement
	}
	title := passage{end: ends.title, path: src.KBPath(row.CanonicalPath), text: row.Title}
	return append([]passage{{label: true, text: label}, title}, statement...), nil
}

// statementPassages is the statement the node's own document gives it: for a
// claim the block its statement marker sits in, in the leaf hosting it; for an
// analysis and an experiment the section its statement marker sits in, which
// the index anchors at the nearest heading above the marker in the leaf
// hosting it. Two split forms read first, for an analysis only here: where its
// marker stands on a blank quoted line, its statement is that blockquote; where
// its paragraph holds before its marker, with text between, the marker of a
// claim it lifts, its statement is that paragraph, markers and `>` prefixes
// removed.
// A work carries none: its register entry's body is its `### Rigor`
// subsection, whose `- rigor:` line the guard refuses and whose `- rationale:`
// is never sent, and everything else the KB holds of a work is its title — the
// work itself lying outside the corpus.
func statementPassages(src *kb.Source, g graph, row kb.NodeRow) ([]passage, error) {
	switch row.NodeType {
	case kb.NodeKindClaim:
		return claimPassages(src, g, row)
	case kb.NodeKindWork:
		return nil, nil
	}
	doc := src.KBPath(row.CanonicalPath)
	text, err := src.ReadText(doc)
	if err != nil {
		return nil, err
	}
	if row.NodeType == kb.NodeKindAnalysis {
		if at, ok := kb.MarkerLines(text)[row.ID]; ok {
			lines := strings.Split(text, "\n")
			if blankQuoted(lines[at]) {
				return blockPassage(doc, lines, at), nil
			}
			if first, last, _, ok := proseLift(lines, at, row.ID, g.lifts); ok {
				return []passage{{path: doc, line: first + 1, text: unquotedText(lines[first : last+1])}}, nil
			}
		}
	}
	return anchoredPassages(doc, text, row), nil
}

// blankQuoted is whether a line is a blank line of a blockquote: one holding
// only its `>` prefix and markers.
func blankQuoted(line string) bool { return quoted(line) && lineText(line) == "" }

// lineMarkerAt is a marker and the 0-based line it stands on.
type lineMarkerAt struct {
	line int
	kb.LineMarker
}

// proseLift is the prose split form in the paragraph around the marker of id on
// 0-based line at: a claim's marker followed, with text between, by the marker
// of an analysis lifting that claim, id naming one of the two. It returns the
// paragraph's lines and the claim's marker; ok is false where the paragraph
// holds no such pair naming id.
func proseLift(lines []string, at int, id string, lifts map[liftPair]bool) (first, last int, claim lineMarkerAt, ok bool) {
	first, last, ok = paragraphExtent(lines, at)
	if !ok {
		return 0, 0, lineMarkerAt{}, false
	}
	var marks []lineMarkerAt
	for i := first; i <= last; i++ {
		for _, m := range kb.LineMarkers(lines[i]) {
			marks = append(marks, lineMarkerAt{line: i, LineMarker: m})
		}
	}
	for i, c := range marks {
		for _, a := range marks[i+1:] {
			if (c.ID == id || a.ID == id) && lifts[liftPair{analysis: a.ID, claim: c.ID}] && textBetween(lines, c, a) {
				return first, last, c, true
			}
		}
	}
	return 0, 0, lineMarkerAt{}, false
}

// textBetween is whether text other than markers stands between the end of
// marker a and the start of marker b, b after a in one paragraph, every line
// between them holding text. Two ids one marker names share its span, and have
// nothing between them.
func textBetween(lines []string, a, b lineMarkerAt) bool {
	if a.line == b.line {
		return b.Start >= a.End && lineText(lines[a.line][a.End:b.Start]) != ""
	}
	return b.line > a.line+1 || lineText(lines[a.line][a.End:]) != "" || lineText(lines[b.line][:b.Start]) != ""
}

// paragraphExtent is the lines of the paragraph the 0-based line at stands in,
// cut as the build's render cuts paragraphs: the run of lines around it that
// hold text once the `>` prefix and markers are removed, quoted exactly when
// line at is, holding no heading, in which only the first line opens a list
// item. A line of a display-maths fence is in the paragraph whatever its text,
// and opens no item. ok is false where line at is itself no such line.
func paragraphExtent(lines []string, at int) (first, last int, ok bool) {
	inFence := make([]bool, len(lines))
	for _, f := range kb.Fences(lines) {
		if f.Info == kb.DisplayMathInfo {
			for i := f.Start; i < f.End; i++ {
				inFence[i] = true
			}
		}
	}
	in := func(i int) bool {
		line := lines[i]
		if quoted(line) != quoted(lines[at]) {
			return false
		}
		text := lineText(line)
		return inFence[i] || text != "" && !headingRE.MatchString(text)
	}
	opensItem := func(i int) bool { return !inFence[i] && kb.ListItemRE.MatchString(lineText(lines[i])) }
	if !in(at) {
		return 0, 0, false
	}
	first, last = at, at
	for first > 0 && !opensItem(first) && in(first-1) {
		first--
	}
	for last < len(lines)-1 && in(last+1) && !opensItem(last+1) {
		last++
	}
	return first, last, true
}

// liftedInBlockquote is whether the marker of an analysis lifting claim stands
// on a blank quoted line of the block the claim's marker on 0-based line at sits
// in.
func liftedInBlockquote(lines []string, at int, claim string, lifts map[liftPair]bool) bool {
	first, last := blockExtent(lines, at)
	for _, l := range lines[first : last+1] {
		if !blankQuoted(l) {
			continue
		}
		for _, m := range kb.LineMarkers(l) {
			if lifts[liftPair{analysis: m.ID, claim: claim}] {
				return true
			}
		}
	}
	return false
}

// unquote is the line with its statement markers, each taking the space before
// it, and every blockquote level removed.
func unquote(line string) string {
	return kb.BlockquotePrefix.ReplaceAllString(kb.StripLineMarkers(line), "")
}

// lineText is the line's text with every blockquote level and its statement
// markers removed, each marker taking the space before it, then trimmed.
func lineText(line string) string { return strings.TrimSpace(unquote(line)) }

// unquotedText is the lines, each through unquote, joined by line breaks and
// trimmed.
func unquotedText(lines []string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = unquote(l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// textBefore is the lines first through at, line at cut at byte cut, as
// unquotedText gives them.
func textBefore(lines []string, first, at, cut int) string {
	run := slices.Clone(lines[first : at+1])
	run[len(run)-1] = run[len(run)-1][:cut]
	return unquotedText(run)
}

// claimPassages is the claim's statement as the leaf hosting it holds it: the
// block its marker sits in, or, where the marker ends a heading's line, the
// section that heading opens — the heading is the frame's title line, and the
// section is the claim. Two split forms read first. Where the claim's marker is
// followed in its paragraph by text and then by the marker of an analysis
// lifting it, its statement is the paragraph up to its marker; where an analysis
// lifting it is marked on a blank quoted line of its blockquote, its statement
// is the text of its marker's line before the marker. Either is read with the
// `>` prefixes and any markers removed. A claim no leaf hosts, and one whose
// host no longer carries the marker, has none: its text is its title alone.
func claimPassages(src *kb.Source, g graph, row kb.NodeRow) ([]passage, error) {
	leaf, ok := g.hosts[row.ID]
	if !ok {
		return nil, nil
	}
	doc := src.KBPath(leaf)
	text, err := src.ReadText(doc)
	if err != nil {
		return nil, err
	}
	at, ok := kb.MarkerLines(text)[row.ID]
	if !ok {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	if first, _, m, ok := proseLift(lines, at, row.ID, g.lifts); ok {
		return []passage{{path: doc, line: first + 1, text: textBefore(lines, first, m.line, m.Start)}}, nil
	}
	if liftedInBlockquote(lines, at, row.ID, g.lifts) {
		for _, m := range kb.LineMarkers(lines[at]) {
			if m.ID == row.ID {
				return []passage{{path: doc, line: at + 1, text: textBefore(lines, at, at, m.Start)}}, nil
			}
		}
	}
	if headingLevel(lines[at]) > 0 {
		body, line := headingBody(text, at)
		if body == "" {
			return nil, nil
		}
		return []passage{{path: doc, line: line, text: body}}, nil
	}
	return blockPassage(doc, lines, at), nil
}

// blockPassage is the block the marker on 0-based line at sits in, whole.
func blockPassage(doc string, lines []string, at int) []passage {
	first, last := blockExtent(lines, at)
	return []passage{{path: doc, line: first + 1, text: strings.Join(lines[first:last+1], "\n")}}
}

// headingBody is the body of the section the heading on 0-based line at opens:
// its lines after the heading up to the next heading as deep or shallower,
// trimmed of the blank lines around them, and the 1-based line the body opens
// on. The code-stripped text decides what is a heading, so a line inside a
// fenced block opens no section, and the document supplies the bytes.
func headingBody(text string, at int) (string, int) {
	lines := strings.Split(text, "\n")
	stripped := strings.Split(kb.StripCodeSplitLines(text), "\n")
	level, end := headingLevel(stripped[at]), len(lines)
	for i := at + 1; i < len(stripped); i++ {
		if lvl := headingLevel(stripped[i]); lvl > 0 && lvl <= level {
			end = i
			break
		}
	}
	first, last := at+1, end
	for first < last && kb.Strip(lines[first]) == "" {
		first++
	}
	for last > first && kb.Strip(lines[last-1]) == "" {
		last--
	}
	if first == last {
		return "", 0
	}
	return strings.Join(lines[first:last], "\n"), first + 1
}

// headingLevel is the level of the ATX heading the line opens, 0 where it opens
// none.
func headingLevel(line string) int {
	if !headingRE.MatchString(line) {
		return 0
	}
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	return n
}

// anchoredPassages is the section at the node's canonical anchor in text, the
// document doc, which the node's canonical path names.
func anchoredPassages(doc, text string, row kb.NodeRow) []passage {
	body, ok := kb.AnchorSection(text, row.CanonicalAnchor)
	if !ok {
		return nil
	}
	if body = strings.TrimSpace(body); body == "" {
		return nil
	}
	// The body's lines are the code-stripped text's, so the body is a run of them
	// and its offset in that text is the line the section opens on.
	stripped := kb.StripCodeSplitLines(text)
	if at := strings.Index(stripped, body); at >= 0 {
		return []passage{{path: doc, line: 1 + strings.Count(stripped[:at], "\n"), text: body}}
	}
	return []passage{{path: doc, text: body}}
}

// blockExtent is the lines of the block the marker on 0-based line at sits in:
// a blockquote's quoted run whole where the marker sits in one, else the run of
// non-blank lines, either clipped at the next line that opens a heading.
func blockExtent(lines []string, at int) (first, last int) {
	in := blockMember(quoted(lines[at]))
	first, last = at, at
	for first > 0 && in(lines[first-1]) {
		first--
	}
	for last < len(lines)-1 && in(lines[last+1]) {
		last++
	}
	return first, last
}

// blockMember is whether a line continues the block the marker's line opens: in
// a blockquote, the quoted lines; elsewhere, the non-blank lines. Either run
// stops at a line that opens a heading.
func blockMember(blockquote bool) func(string) bool {
	return func(line string) bool {
		if headingRE.MatchString(line) {
			return false
		}
		if blockquote {
			return quoted(line)
		}
		return !quoted(line) && kb.Strip(line) != ""
	}
}

// quoted is whether a line is a blockquote's.
func quoted(line string) bool { return strings.HasPrefix(strings.TrimLeft(line, " \t"), ">") }

// wrapWidth is the column a passage's lines are wrapped to. Wrapping adds breaks
// and rewrites nothing, so a line of the carried text stays a verbatim slice of
// the passage it came from.
const wrapWidth = 100

// assemble is the passages as one text, wrapped and numbered, refusing the text
// where a token the frame cannot hold reaches it. A passage with an end is a site
// and its lines are numbered; a label line and the edge's note are carried and
// hold no number. Every break wrapping adds falls at a space, and neither token
// the guard reads spans one, so a wrapped line is refused as the passage would be;
// the hit names the document and the line it came from, which a break never moves.
func assemble(passages []passage) (closedText, error) {
	var (
		ct closedText
		no int // the last number written
	)
	for _, p := range passages {
		text := strings.TrimSpace(p.text)
		if text == "" {
			continue
		}
		wrapped, hit := wrapPassage(p, text)
		if hit != nil {
			return closedText{}, *hit
		}
		if ct.text != "" {
			ct.text += "\n\n"
			ct.lines = append(ct.lines, line{from: len(ct.text), to: len(ct.text)})
		}
		for i, w := range wrapped {
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
	return ct, nil
}

// wrapPassage is one passage's lines as the call carries them, and the token the
// guard found in it: a scoring token refuses the passage at once, and the frame's
// own structure is held until the passage has been read whole, so a passage
// carrying both is refused on the scoring token.
func wrapPassage(p passage, text string) (wrapped []string, hit *scoreTokenError) {
	for i, src := range strings.Split(text, "\n") {
		for _, w := range wrapLine(src) {
			if !p.label {
				kind, token, ok := refused(w)
				switch {
				case ok && kind == kindScoringToken:
					return nil, &scoreTokenError{Path: p.path, Line: p.sourceLine(i), Token: token, Kind: kind}
				case ok && hit == nil:
					hit = &scoreTokenError{Path: p.path, Line: p.sourceLine(i), Token: token, Kind: kind}
				}
			}
			wrapped = append(wrapped, w)
		}
	}
	return wrapped, hit
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

// sourceLine is the document line the passage's i'th line came from, 0 where the
// reader does not locate the passage.
func (p passage) sourceLine(i int) int {
	if p.line == 0 {
		return 0
	}
	return p.line + i
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

// look resolves field as a charge's line field against the text the call carried:
// one number, or a range N-M with N at most M, every number one the call shows,
// all of them in one passage and with nothing between them — a blank line, a label
// or a heading ends a range. Where it names no such lines it returns the case,
// which the caller's refusal names.
func (ct closedText) look(field string) (cite, string) {
	m := linesRE.FindStringSubmatch(field)
	if m == nil {
		return cite{}, fmt.Sprintf("field %q names no line: a charge names one line number or a range N-M", field)
	}
	lo, err := strconv.Atoi(m[1])
	if err != nil {
		return cite{}, fmt.Sprintf("line %s is not one the call shows", m[1])
	}
	hi := lo
	if m[2] != "" {
		if hi, err = strconv.Atoi(m[2]); err != nil {
			return cite{}, fmt.Sprintf("line %s is not one the call shows", m[2])
		}
	}
	if hi < lo {
		return cite{}, fmt.Sprintf("range %d-%d runs backwards: a range is written N-M with N at most M", lo, hi)
	}
	i, ok := ct.at(lo)
	if !ok {
		return cite{}, fmt.Sprintf("line %d is not one the call shows", lo)
	}
	j, ok := ct.at(hi)
	if !ok {
		return cite{}, fmt.Sprintf("line %d is not one the call shows", hi)
	}
	first, last := ct.lines[i], ct.lines[j]
	if first.end != last.end {
		return cite{}, fmt.Sprintf("range %d-%d crosses from one passage to another", lo, hi)
	}
	if j-i != hi-lo || ct.crossesHeading(i, j) {
		return cite{}, fmt.Sprintf("range %d-%d crosses a gap: a blank line, a label or a heading lies between its lines", lo, hi)
	}
	lines := strconv.Itoa(lo)
	if hi != lo {
		lines += "-" + strconv.Itoa(hi)
	}
	return cite{end: first.end, lines: lines, citation: ct.text[first.from:last.to], offset: first.from, lo: lo, hi: hi}, ""
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

// refused is one line's first token that may not reach a prompt — a scoring token
// or the frame's own structure — and which of the two it is.
func refused(text string) (kind, token string, ok bool) {
	if loc := scoreTokenRE.FindStringIndex(text); loc != nil {
		return kindScoringToken, text[loc[0]:loc[1]], true
	}
	if loc := frameRE.FindStringIndex(text); loc != nil {
		return kindFrame, text[loc[0]:loc[1]], true
	}
	return "", "", false
}

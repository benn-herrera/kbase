// Package score is the scoring pass: the closed text one call carries, and the
// run that lands its answers.
package score

import (
	"fmt"
	"regexp"
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
// passage a citation here is recorded as, "" where none may lie; path is the
// document's absolute path, line the 1-based line the passage opens on, or 0
// where the reader does not locate it; text is the passage as the call carries
// it, opening with label's own bytes. label is the length of the prefix the frame
// writes at the passage's head — a bearing end's label, or a rigor title's "# " —
// which is carried, is no citation's home, and is no part of the corpus the guard
// reads.
type passage struct {
	end   string
	path  string
	line  int
	label int
	text  string
}

// closedText is the text one call carries and the passages it is made of: each
// citable passage's end and its half-open range in the text, which is what a
// charge's citation is looked up in and where its offset is counted from. A
// passage with no end contributes text that is carried and never cited.
type closedText struct {
	text     string
	passages []extent
}

// extent is one passage's place in the closed text.
type extent struct {
	end      string
	from, to int
}

// graph is the derived index as the passages read it: every node by id, and the
// citations in the index's own order. A run builds one and every item of its
// walk reads that one, so no item reads an index file — the runner already holds
// the index, and a second reading path is how two views of the graph drift.
type graph struct {
	nodes map[string]kb.NodeRow
	cites []kb.CiteRow
}

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
	passages, err := endPassages(src, g, row, "# ", nodeEnds{title: passageTitle, statement: passageStatement})
	if err != nil {
		return closedText{}, err
	}
	return assemble(passages)
}

// The labels a bearing call writes: above each end's title, and the line above
// the edge's own note. They are the frame's own — carried, and no citation's home.
const (
	sourceLabel  = "## Source: "
	targetLabel  = "## Target: "
	contextLabel = "## Context"
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
		passages = append(passages, passage{label: len(contextLabel), text: contextLabel}, passage{text: *e.Context})
	}
	return assemble(passages)
}

// nodeEnds is the end a node's title is recorded as and the end its statement is:
// a rigor call reads the two apart, and a bearing call records both as the one end
// they belong to.
type nodeEnds struct{ title, statement string }

// endPassages is one node's title under label, then its statement, each recorded
// as ends names.
func endPassages(src *kb.Source, g graph, row kb.NodeRow, label string, ends nodeEnds) ([]passage, error) {
	statement, err := statementPassages(src, g, row)
	if err != nil {
		return nil, err
	}
	for i := range statement {
		statement[i].end = ends.statement
	}
	return append([]passage{titlePassage(src, row, label, ends.title)}, statement...), nil
}

// statementPassages is the statement the node's own document gives it: for a
// claim the paragraph its tier-2 marker sits in, in the leaf that declares it;
// for an analysis, an experiment and an axiom the section at its canonical
// anchor. A work carries none: its register entry's body is its `### Rigor`
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
	return anchoredPassages(src, row)
}

// claimPassages is the claim's statement as the leaf that tier-2-marks it
// holds it. A claim no leaf marks, and one whose declared leaf no longer carries
// the marker, has none: its text is its title alone.
func claimPassages(src *kb.Source, g graph, row kb.NodeRow) ([]passage, error) {
	leaf, ok := declaringLeaf(g, row.ID)
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
	first, last := blockExtent(lines, at)
	return []passage{{path: doc, line: first + 1, text: strings.Join(lines[first:last+1], "\n")}}, nil
}

// anchoredPassages is the section at the node's canonical anchor in the document
// its canonical path names.
func anchoredPassages(src *kb.Source, row kb.NodeRow) ([]passage, error) {
	doc := src.KBPath(row.CanonicalPath)
	text, err := src.ReadText(doc)
	if err != nil {
		return nil, err
	}
	body, ok := kb.AnchorSection(text, row.CanonicalAnchor)
	if !ok {
		return nil, nil
	}
	if body = strings.TrimSpace(body); body == "" {
		return nil, nil
	}
	// The body's lines are the code-stripped text's, so the body is a run of them
	// and its offset in that text is the line the section opens on.
	stripped := kb.StripCodeSplitLines(text)
	if at := strings.Index(stripped, body); at >= 0 {
		return []passage{{path: doc, line: 1 + strings.Count(stripped[:at], "\n"), text: body}}, nil
	}
	return []passage{{path: doc, text: body}}, nil
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

// declaringLeaf is the leaf that tier-2-marks the claim, the first in the
// index's cite order. A claim only leaves cite has none.
func declaringLeaf(g graph, id string) (string, bool) {
	for _, c := range g.cites {
		if c.ClaimID == id && c.Tier2Marked {
			return c.LeafPath, true
		}
	}
	return "", false
}

// titlePassage is the node's title under prefix: "# " for a rigor call, and the
// end's label for a bearing call. It is citable as end's, from after the prefix.
func titlePassage(src *kb.Source, row kb.NodeRow, prefix, end string) passage {
	return passage{end: end, path: src.KBPath(row.CanonicalPath), label: len(prefix), text: prefix + row.Title}
}

// assemble is the passages as one text and the extents of the ones a citation
// may lie in, refusing the text where a token the frame cannot hold reaches it. No
// token spans the blank line between two passages, so each passage scans as the
// whole text would, and the hit names the document and line it came from. A
// passage's label is the frame's own and is read by neither the guard nor a
// citation.
func assemble(passages []passage) (closedText, error) {
	var ct closedText
	for _, p := range passages {
		text := strings.TrimSpace(p.text)
		if text == "" {
			continue
		}
		label := min(p.label, len(text))
		if at, kind, token, ok := refused(text[label:]); ok {
			line := p.line
			if line > 0 {
				line += strings.Count(text[:label+at], "\n")
			}
			return closedText{}, scoreTokenError{Path: p.path, Line: line, Token: token, Kind: kind}
		}
		if ct.text != "" {
			ct.text += "\n\n"
		}
		from := len(ct.text) + label
		ct.text += text
		if p.end != "" {
			ct.passages = append(ct.passages, extent{end: p.end, from: from, to: len(ct.text)})
		}
	}
	return ct, nil
}

// refused is text's first token that may not reach a prompt — a scoring token or
// the frame's own structure — with its byte offset and which of the two it is. A
// passage carrying both is refused on the scoring token.
func refused(text string) (at int, kind, token string, ok bool) {
	if loc := scoreTokenRE.FindStringIndex(text); loc != nil {
		return loc[0], kindScoringToken, text[loc[0]:loc[1]], true
	}
	if loc := frameRE.FindStringIndex(text); loc != nil {
		return loc[0], kindFrame, text[loc[0]:loc[1]], true
	}
	return 0, "", "", false
}

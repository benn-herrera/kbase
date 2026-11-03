package kb

import (
	"slices"
	"strings"
)

// Statement is hosted node id's statement in text, the leaf its marker stands
// in, markers and blockquote prefixes removed, "" where text carries no marker
// of id; lifts is whether a lifts edge runs from an analysis to a claim. A
// claim's is the paragraph its marker sits in, or, where the marker ends a
// heading line, that heading's section; an analysis's or an experiment's is
// the section of the nearest heading at or above its marker. Two layouts the
// build's analysis split writes read first: an analysis marker alone on a
// blank quoted line, whose statement is the quoted run and whose lifted claim,
// marked in the same run, has the text of its own marker's line before the
// marker; and a paragraph holding a claim's marker, then text, then the marker
// of an analysis lifting it, where the claim's is the paragraph up to its
// marker and the analysis's the whole paragraph.
func Statement(text, id string, lifts func(analysis, claim string) bool) string {
	lines := SplitLines(text)
	at, ok := MarkerLines(text)[id]
	if !ok || at >= len(lines) {
		return ""
	}
	headings := HeadingAnchors(text)
	kind, _ := NodeIDKind(id)
	if quotedLine(lines[at]) {
		first, last := at, at
		for first > 0 && quotedLine(lines[first-1]) {
			first--
		}
		for last+1 < len(lines) && quotedLine(lines[last+1]) {
			last++
		}
		if kind == NodeKindAnalysis && statementText(lines[at:at+1]) == "" {
			return statementText(lines[first : last+1])
		}
		for _, l := range lines[first : last+1] {
			if kind != NodeKindClaim || statementText([]string{l}) != "" {
				continue
			}
			for _, m := range LineMarkers(l) {
				if lifts(m.ID, id) {
					return statementText([]string{lines[at][:markerStart(lines[at], id)]})
				}
			}
		}
	}
	if first, last, ok := paragraphAround(lines, headings, at); ok {
		if s, ok := liftSplit(lines[first:last+1], id, lifts); ok {
			return s
		}
		if kind == NodeKindClaim {
			return statementText(lines[first : last+1])
		}
	}
	h := at
	for ; h >= 0; h-- {
		if _, heading := headings[h]; heading {
			break
		}
	}
	if kind == NodeKindClaim && h != at {
		return ""
	}
	return statementSection(lines, headings, h)
}

// quotedLine is whether line is a blockquote's.
func quotedLine(line string) bool { return strings.HasPrefix(strings.TrimLeft(line, " \t"), ">") }

// statementText is lines with their statement markers and blockquote prefixes
// removed, joined by line breaks and trimmed.
func statementText(lines []string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = BlockquotePrefix.ReplaceAllString(StripLineMarkers(l), "")
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// markerStart is the byte offset of the marker naming id on line.
func markerStart(line, id string) int {
	for _, m := range LineMarkers(line) {
		if m.ID == id {
			return m.Start
		}
	}
	return len(line)
}

// paragraphAround is the 0-based extent of the paragraph the 0-based line at
// stands in: the run of lines around it holding text once blockquote prefixes
// and markers are removed, all quoted or none, holding no heading, in which
// only the first line opens a list item. A line of a display-maths fence is in
// the paragraph whatever its text, and opens no list item. ok is false where
// line at is no such line.
func paragraphAround(lines []string, headings map[int]string, at int) (first, last int, ok bool) {
	quoted := quotedLine(lines[at])
	inFence := make([]bool, len(lines))
	for _, f := range Fences(lines) {
		if f.Info == DisplayMathInfo {
			for i := f.Start; i < f.End; i++ {
				inFence[i] = true
			}
		}
	}
	in := func(i int) bool {
		_, heading := headings[i]
		return !heading && quotedLine(lines[i]) == quoted && (inFence[i] || statementText(lines[i:i+1]) != "")
	}
	opens := func(i int) bool {
		return !inFence[i] && ListItemRE.MatchString(strings.TrimLeft(BlockquotePrefix.ReplaceAllString(lines[i], ""), " \t"))
	}
	if !in(at) {
		return 0, 0, false
	}
	first, last = at, at
	for first > 0 && !opens(first) && in(first-1) {
		first--
	}
	for last+1 < len(lines) && in(last+1) && !opens(last+1) {
		last++
	}
	return first, last, true
}

// paragraphMarker is one statement marker of a paragraph: the 0-based line of
// the paragraph it stands on, and its id and span there.
type paragraphMarker struct {
	line int
	LineMarker
}

// liftSplit is id's statement where par, a paragraph, holds a claim's marker,
// then text, then the marker of an analysis lifting that claim, one of the two
// naming id: the paragraph up to the claim's marker for the claim, the whole
// paragraph for the analysis. ok is false where par holds no such pair.
func liftSplit(par []string, id string, lifts func(analysis, claim string) bool) (string, bool) {
	var marks []paragraphMarker
	for i, l := range par {
		for _, m := range LineMarkers(l) {
			marks = append(marks, paragraphMarker{i, m})
		}
	}
	textBetween := func(c, a paragraphMarker) bool {
		if c.line == a.line {
			return a.Start >= c.End && strings.TrimSpace(StripLineMarkers(par[c.line][c.End:a.Start])) != ""
		}
		return a.line > c.line+1 || strings.TrimSpace(StripLineMarkers(par[c.line][c.End:])) != "" ||
			statementText([]string{par[a.line][:a.Start]}) != ""
	}
	for i, c := range marks {
		for _, a := range marks[i+1:] {
			if (c.ID != id && a.ID != id) || !lifts(a.ID, c.ID) || !textBetween(c, a) {
				continue
			}
			if c.ID == id {
				return statementText(append(slices.Clone(par[:c.line]), par[c.line][:c.Start])), true
			}
			return statementText(par), true
		}
	}
	return "", false
}

// statementSection is the text of the section of the heading on 0-based line
// h, from the line after it to the next heading as deep or shallower; h of -1
// is the text above the first heading.
func statementSection(lines []string, headings map[int]string, h int) string {
	level := 6
	if h >= 0 {
		level = headingLevel(lines[h])
	}
	end := len(lines)
	for i := h + 1; i < len(lines); i++ {
		if _, ok := headings[i]; ok && headingLevel(lines[i]) <= level {
			end = i
			break
		}
	}
	return statementText(lines[h+1 : end])
}

// headingLevel is the number of #s opening line, a heading's.
func headingLevel(line string) int { return len(line) - len(strings.TrimLeft(line, "#")) }

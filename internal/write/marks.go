package write

import (
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
)

// appendMarker is document with marker appended to the end of line at, inside
// whatever block that line is in and ahead of its trailing whitespace. A line
// already carrying the marker is left as it is.
func appendMarker(document string, at int, marker string) string {
	line := kb.SplitLines(document)[at]
	marked := kb.AppendMarker(line, marker)
	if marked == line {
		return document
	}
	return spliceLines(document, at, at+1, []string{marked})
}

// unmarkLine is line with every occurrence of marker taken off, with the space before it.
func unmarkLine(line, marker string) string {
	return strings.ReplaceAll(strings.ReplaceAll(line, " "+marker, ""), marker, "")
}

// unmarked is document without id's statement marker, but on its 0-based
// line keep (-1 for none): every other occurrence is taken from its line, and
// a line the marker leaves without prose goes with it.
func unmarked(document, id string, keep int) string {
	marker := kb.StatementMarker(id)
	if !strings.Contains(document, marker) {
		return document
	}
	var kept []string
	for i, l := range kb.SplitLinesKeepEnds(document) {
		if i == keep || !strings.Contains(l, marker) {
			kept = append(kept, l)
			continue
		}
		if body := unmarkLine(l, marker); kb.Strip(body) != "" {
			kept = append(kept, body)
		}
	}
	return strings.Join(kept, "")
}

// markerLine is one line of one leaf: its kb-root-relative subject and its
// 0-based line.
type markerLine struct {
	subject string
	line    int
}

// unmarkEverywhere takes id's statement marker out of every leaf of the KB
// holding one, but the one on keep's line; a zero keep keeps none.
func (b *recordBatch) unmarkEverywhere(id string, keep markerLine) error {
	documents, err := b.knownDocuments()
	if err != nil {
		return err
	}
	for _, subject := range documents {
		l, err := b.leafAt(subject)
		if err != nil {
			return err
		}
		if l == nil {
			continue
		}
		line := -1
		if subject == keep.subject {
			line = keep.line
		} else {
			l.marked = slices.DeleteFunc(l.marked, func(m string) bool { return m == id })
		}
		l.text = unmarked(l.text, id, line)
	}
	return nil
}

// recordProse is one prose field of a record: the record file holding it, the
// record, the field and its text, and, of an edge record's context, the edge
// record.
type recordProse struct {
	file, record, field, text string
	edge                      *kb.EdgeRecord
}

// graphProse is every prose field of g's records, in record order.
func graphProse(g kb.Graph) []recordProse {
	var out []recordProse
	node := func(id string, fields ...string) {
		for i := 0; i+1 < len(fields); i += 2 {
			out = append(out, recordProse{file: kb.GraphNodesFile, record: id, field: fields[i], text: fields[i+1]})
		}
	}
	for _, n := range g.Claims {
		node(n.ID, "title", n.Title, "rationale", n.Rationale)
	}
	for _, n := range g.Analyses {
		node(n.ID, "title", n.Title, "rationale", n.Rationale)
	}
	for _, n := range g.Experiments {
		node(n.ID, "title", n.Title)
	}
	for _, n := range g.Works {
		node(n.ID, "title", n.Title, "rationale", n.Rationale)
	}
	for _, n := range g.Axioms {
		node(n.ID, "title", n.Title)
	}
	for _, r := range g.Edges() {
		out = append(out, recordProse{file: kb.GraphEdgesFile, record: index.EdgeName(r.Relation, r.Source, r.Target), field: "context",
			text: r.Context, edge: &r})
	}
	return out
}

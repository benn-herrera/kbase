package write

import (
	"fmt"
	"slices"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// removeNode removes the node its entry's id names and every record naming
// it — its record, every edge and demoted record naming it at either end —
// and its statement marker, or an axiom's headings in axioms.md, reporting
// every edge record it removed as an edge row. It is refused where no node
// carries the id, and where, the removal composed, a citation or a surviving
// record's prose field still mentions the node, or a citation span carries a
// removed work's label that no reference-list entry has (refuseMentions).
func removeNode(b *recordBatch, e entry) error {
	id := e.str(editID.name)
	kind, ok := index.GraphKinds(b.g)[id]
	if !ok {
		return noNode(e.Op, editID.name, id)
	}
	rows, err := b.derived()
	if err != nil {
		return err
	}
	var label string
	if kind == kb.NodeKindWork {
		label = record(b.g.Works, func(n kb.WorkNode) bool { return n.ID == id }).Label
	}
	names := func(source, target string) bool { return source == id || target == id }
	removed := slices.DeleteFunc(rows.Edges, func(r kb.EdgeRow) bool { return !names(r.Source, r.Target) })
	b.dropEdges(func(_, source, target string) bool { return names(source, target) })
	b.g.Demoted = slices.DeleteFunc(b.g.Demoted, func(d kb.DemotedEdge) bool { return names(d.Source, d.Target) })
	if kind == kb.NodeKindAxiom {
		b.g.Axioms = slices.DeleteFunc(b.g.Axioms, func(n kb.AxiomNode) bool { return n.ID == id })
		err = b.dropAxiomHeadings(id)
	} else {
		err = b.keepSummaryLinks(id, b.titleOf(kind, id), func() error { return b.dropNode(kind, id) })
	}
	if err != nil {
		return err
	}
	b.report = entryReport{ID: id, RemovedEdges: removed}
	return b.refuseMentions(id, label)
}

// dropNode takes the record of the claim, analysis, experiment or work id,
// of kind, out of the records, and a hosted node's statement marker out of
// every document.
func (b *recordBatch) dropNode(kind, id string) error {
	g := &b.g
	switch kind {
	case kb.NodeKindClaim:
		g.Claims = slices.DeleteFunc(g.Claims, func(n kb.ClaimNode) bool { return n.ID == id })
	case kb.NodeKindAnalysis:
		g.Analyses = slices.DeleteFunc(g.Analyses, func(n kb.AnalysisNode) bool { return n.ID == id })
	case kb.NodeKindExperiment:
		g.Experiments = slices.DeleteFunc(g.Experiments, func(n kb.ExperimentNode) bool { return n.ID == id })
	case kb.NodeKindWork:
		g.Works = slices.DeleteFunc(g.Works, func(n kb.WorkNode) bool { return n.ID == id })
		return nil
	}
	delete(b.hosts, id)
	return b.unmarkEverywhere(id, markerLine{})
}

// dropAxiomHeadings takes every heading carrying the axiom id's marker out of
// axioms.md, where it stands, and re-points every link into another heading
// of axioms.md whose anchor the removal moved, a later heading of the same
// slug taking a lower repeat suffix.
func (b *recordBatch) dropAxiomHeadings(id string) error {
	l, err := b.axiomsDocument()
	if err != nil {
		return err
	}
	was := kb.HeadingAnchors(l.text)
	var dropped []int
	headings := kb.AxiomHeadings(l.text)
	for i := len(headings) - 1; i >= 0; i-- {
		if h := headings[i]; h.ID == id {
			l.text = spliceLines(l.text, h.Line-1, h.Line, nil)
			dropped = append(dropped, h.Line-1)
		}
	}
	now := kb.HeadingAnchors(l.text)
	moved := movedAnchors{}
	for line, anchor := range was {
		above := 0
		for _, d := range dropped {
			if d < line {
				above++
			}
		}
		if a, ok := now[line-above]; ok && !slices.Contains(dropped, line) && a != anchor {
			moved[citeTarget{kb.AxiomsFile, anchor}] = a
		}
	}
	if len(moved) == 0 {
		return nil
	}
	return b.repoint(func(rel string, l kb.TextLink) string { return moved.dest(rel, l.Destination) })
}

// Mentions is whether text names id: a citation of it, by its link title, or
// the id standing bare. remove-node refuses while a surviving record's prose
// field mentions the node it removes.
func Mentions(text, id string) bool {
	for _, l := range kb.Links(text) {
		if l.Title == id {
			return true
		}
	}
	for _, t := range kb.BareIDs(text) {
		if t.ID == id {
			return true
		}
	}
	return false
}

// refuseMentions refuses a removal of id while a citation of it, outside code
// in any document but the summary pages, or a prose field of a record still
// mentions it; each is named, a citation by path and line, a field by record
// and field. A removed work's label, "" for any other kind, refuses it too
// where a citation span carries it as a key and no reference-list entry has
// that key: the work record is the span's only bibliography entry. Each such
// span is named by path and line.
func (b *recordBatch) refuseMentions(id, label string) error {
	var refusals, spans result.Refusal
	entry := false
	err := b.eachDocument(func(rel string, l *leafEdit, _ bool) error {
		last := 0
		for _, c := range kb.TextCitations(l.text) {
			if n := c.Line + 1; c.Title == id && n != last {
				last = n
				refusals = append(refusals, result.Item{Key: editID.name, Path: rel, Line: n,
					Remedy: fmt.Sprintf("edit the prose at %s:%d so it no longer cites %s, then re-run", rel, n, id),
					Detail: fmt.Sprintf("%s:%d cites %s, and a citation of a removed node points nowhere", rel, n, id)})
			}
		}
		if label == "" {
			return nil
		}
		last = 0
		for _, r := range kb.CitationKeyRefs(l.text) {
			entry = entry || (r.Key == label && r.Use == kb.KeyInEntry)
			if n := r.Line + 1; r.Key == label && r.Use == kb.KeyInSpan && n != last {
				last = n
				spans = append(spans, result.Item{Key: editID.name, Path: rel, Line: n,
					Remedy: fmt.Sprintf("edit the prose at %s:%d so it no longer cites %s, then re-run", rel, n, label),
					Detail: fmt.Sprintf("%s:%d holds a citation span carrying %s, %s's label, which no reference-list entry has as its key, "+
						"so without the work record the export prints the span as plain text", rel, n, label, id)})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !entry {
		refusals = append(refusals, spans...)
	}
	for _, p := range graphProse(b.g) {
		if !Mentions(p.text, id) {
			continue
		}
		refusals = append(refusals, result.Item{Key: editID.name, Path: p.file, Remedy: proseEditRemedy(p),
			Detail: fmt.Sprintf("%s's %s in %s mentions %s, and a removal leaves no record naming a node that does not stand",
				p.record, p.field, p.file, id)})
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

// proseEditRemedy is the entry, earlier in the same batch, that clears a
// mention in p: the edit op of its node's kind, or edit-edge; a demoted
// record's context, which no edit reaches, is cleared by restoring the edge or
// removing the record.
func proseEditRemedy(p recordProse) string {
	if p.edge == nil {
		kind, _ := kb.NodeIDKind(p.record)
		return fmt.Sprintf("earlier in the same batch, put an %s entry rewriting %s's %s without the mention, then re-run", editOps[kind], p.record, p.field)
	}
	r := *p.edge
	if r.Relation == kb.RelationDemoted {
		return fmt.Sprintf("earlier in the same batch, put a restore-demoted entry, source %s and target %s, then an edit-edge entry rewriting the restored edge's context; "+
			"or a remove-edge entry, source %s, target %s and relation %s; then re-run", r.Source, r.Target, r.Source, r.Target, r.Relation)
	}
	return fmt.Sprintf("earlier in the same batch, put an edit-edge entry, source %s, target %s and relation %s, rewriting its context without the mention, then re-run",
		r.Source, r.Target, r.Relation)
}

// nothingToMove is the remedy of a move-node entry naming a work or an axiom,
// which has no marker to move.
const nothingToMove = "restore: drop the entry, then re-run"

// moveNode moves the statement marker of the hosted node its entry's id names
// to the end of the named line, taking it from wherever else it stood, and
// re-points every citation of the node to its new host and section. A move to
// where the marker stands changes nothing. It is refused for a work or an
// axiom, which has no marker, for an id no node carries, for a node with no
// marker, and wherever an add would refuse the named line.
func moveNode(b *recordBatch, e entry) error {
	id := e.str(editID.name)
	if kind, _ := kb.NodeIDKind(id); kind == kb.NodeKindWork || kind == kb.NodeKindAxiom {
		return opRefusal(editID.name, fmt.Sprintf("%s is %s, which has no statement marker to move", id, kb.KindWithArticle(kind)),
			nothingToMove)
	}
	if _, ok := index.GraphKinds(b.g)[id]; !ok {
		return noNode(e.Op, editID.name, id)
	}
	if _, hosted := b.hosts[id]; !hosted {
		return noStatementMarker(id, "there is none to move")
	}
	if err := b.checkNamedLine(e); err != nil {
		return err
	}
	subject, l, err := b.leaf(e.str("document"))
	if err != nil {
		return err
	}
	b.report = entryReport{ID: id}
	seen := map[string]string{}
	leavesMoved(b, seen)
	at := e.count("line") - 1
	l.text = appendMarker(l.text, at, kb.StatementMarker(id))
	if err := b.unmarkEverywhere(id, markerLine{subject, at}); err != nil {
		return err
	}
	if !leavesMoved(b, seen) {
		return nil
	}
	b.marked(subject, l, id)
	to, err := b.targetOf(id)
	if err != nil {
		return err
	}
	return b.repoint(func(rel string, l kb.TextLink) string {
		if l.Title == id && pointsAt(rel, l.Destination) != to {
			return retargeted(rel, l.Destination, to)
		}
		return l.Destination
	})
}

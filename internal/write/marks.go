package write

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
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

// unmarked is document without id's statement marker: every occurrence is
// taken from its line, and a line the marker leaves without prose goes with it.
func unmarked(document, id string) string {
	marker := kb.StatementMarker(id)
	if !strings.Contains(document, marker) {
		return document
	}
	var kept []string
	for _, l := range kb.SplitLinesKeepEnds(document) {
		if !strings.Contains(l, marker) {
			kept = append(kept, l)
			continue
		}
		if body := strings.ReplaceAll(strings.ReplaceAll(l, " "+marker, ""), marker, ""); kb.Strip(body) != "" {
			kept = append(kept, body)
		}
	}
	return strings.Join(kept, "")
}

// unmarkEverywhere takes id's statement marker out of every leaf of the KB
// holding one.
func (b *recordBatch) unmarkEverywhere(id string) error {
	documents, err := b.knownDocuments()
	if err != nil {
		return err
	}
	for _, subject := range documents {
		l, err := b.leafAt(subject)
		if err != nil {
			return err
		}
		if l != nil {
			l.text = unmarked(l.text, id)
		}
	}
	return nil
}

// markNode places a node's one statement marker on the line its locator names
// in its document, taking it from wherever it stood.
func markNode(b *recordBatch, e entry) error {
	id := e.str("id")
	if _, err := b.nodeKind("id", id); err != nil {
		return err
	}
	subject, l, err := b.leaf(e.str("document"))
	if err != nil {
		return err
	}
	if err := b.unmarkEverywhere(id); err != nil {
		return err
	}
	at, err := locateExcerpt(l.text, e.str("locator"))
	if err != nil {
		return err
	}
	l.text = appendMarker(l.text, at, kb.StatementMarker(id))
	b.marked(subject, l, id)
	return nil
}

// refuseStrandedClaim refuses a claim another node's edge record names — the
// edges drop-inbound-edges removes — since dropping the claim would strand
// them. Its own edges, the supports and founds edges into it and the
// references and refutes edges out of it, go with it. No entry's drop lands
// until dropClaimNodes, so every entry is judged against the records as they
// stood.
func refuseStrandedClaim(b *recordBatch, e entry) error {
	id := e.str("id")
	var refusals result.Refusal
	for _, r := range b.g.Edges() {
		if inboundEdge(id, r.Relation, r.Source, r.Target) {
			refusals = append(refusals, result.Item{Key: "id", Entry: e.Index,
				Remedy: "run kbase drop-claim-edges and kbase drop-inbound-edges first, then re-run",
				Detail: fmt.Sprintf("%s still names %s, and dropping the node would strand the edge; "+
					"run drop-claim-edges and drop-inbound-edges first", index.EdgeName(r.Relation, r.Source, r.Target), id)})
		}
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

// dropClaimNodes removes each entry's claim record, every edge and demoted
// record naming it and its statement marker, then refuses a mention left
// standing.
func dropClaimNodes(b *recordBatch, entries []entry) error {
	for _, e := range entries {
		id := e.str("id")
		names := func(source, target string) bool { return source == id || target == id }
		b.dropEdges(func(_, source, target string) bool { return names(source, target) })
		b.g.Claims = slices.DeleteFunc(b.g.Claims, func(n kb.ClaimNode) bool { return n.ID == id })
		b.g.Demoted = slices.DeleteFunc(b.g.Demoted, func(d kb.DemotedEdge) bool { return names(d.Source, d.Target) })
		if err := b.unmarkEverywhere(id); err != nil {
			return err
		}
	}
	return refuseRemainingMentions(b, entries)
}

// recordProse is one prose field of a record: the record file holding it, the
// record, the field and its text.
type recordProse struct {
	file, record, field, text string
}

// graphProse is every prose field of g's records, in record order.
func graphProse(g kb.Graph) []recordProse {
	var out []recordProse
	node := func(id string, fields ...string) {
		for i := 0; i+1 < len(fields); i += 2 {
			out = append(out, recordProse{kb.GraphNodesFile, id, fields[i], fields[i+1]})
		}
	}
	for _, n := range g.Claims {
		node(n.ID, "title", n.Title, "rationale", n.Rationale, "no-edge", n.NoEdge)
		for i, item := range n.StrengthenBy {
			node(n.ID, fmt.Sprintf("strengthen-by[%d]", i+1), item)
		}
	}
	for _, n := range g.Analyses {
		node(n.ID, "title", n.Title, "rationale", n.Rationale, "no-edge", n.NoEdge)
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
		out = append(out, recordProse{kb.GraphEdgesFile, index.EdgeName(r.Relation, r.Source, r.Target), "context", r.Context})
	}
	return out
}

// proseRemedy is how a mention in a record's prose field is removed, "" where
// no op rewrites the field.
func proseRemedy(field string) string {
	switch field {
	case "rationale":
		return "rewrite the rationale without the mention with kbase set-rationale, then re-run"
	case "context":
		return "drop the edge with kbase drop-claim-edges or kbase drop-inbound-edges, re-add it without the mention " +
			"with kbase add-rests-on or kbase add-pairs, then re-run"
	}
	return ""
}

// refuseRemainingMentions refuses where, with the drops composed, a line
// under kb-root or a prose field of a record still names a dropped claim:
// verify's claim-id link check reads every such line, and the trailing refresh
// renders records' prose onto the register pages. The register pages are not
// read, that refresh rendering them anew.
func refuseRemainingMentions(b *recordBatch, entries []entry) error {
	pages, err := index.RegisterPagePositions(b.src)
	if err != nil {
		return err
	}
	files, err := kb.MarkdownFiles(b.src)
	if err != nil {
		return err
	}
	var refusals result.Refusal
	for _, file := range files {
		rel, err := filepath.Rel(b.src.Root(), file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if slices.Contains(pages, rel) {
			continue
		}
		var text string
		if l, composed := b.leaves[rel]; composed {
			text = l.text
		} else if text, err = b.src.ReadText(file); err != nil {
			return fmt.Errorf("%s could not be read to check it for a mention of a dropped claim: %w", rel, err)
		}
		for n, line := range kb.SplitLines(kb.StripCodeSplitLines(text)) {
			named := kb.NodeIDs(line)
			for _, e := range entries {
				id := e.str("id")
				if !slices.Contains(named, id) {
					continue
				}
				refusals = append(refusals, result.Item{Key: "id=" + id, Entry: e.Index, Path: rel, Line: n + 1,
					Remedy: "remove the mention — through the op that wrote it, or by hand where it was authored — then re-run",
					Detail: fmt.Sprintf("%s:%d names %s, a mention this op does not remove, and once the node is gone "+
						"verify's claim-id link check reports it as an unknown id", rel, n+1, id)})
			}
		}
	}
	for _, p := range graphProse(b.g) {
		named := kb.NodeIDs(p.text)
		for _, e := range entries {
			id := e.str("id")
			if !slices.Contains(named, id) {
				continue
			}
			fix := proseRemedy(p.field)
			detail := fmt.Sprintf("%s's %s in %s names %s, a mention this op does not remove, and once the node is gone "+
				"the record names an unknown id, which a register page rendering it carries to verify", p.record, p.field, p.file, id)
			if fix == "" {
				detail += "; no kbase op rewrites this field"
			}
			refusals = append(refusals, result.Item{Key: "id=" + id, Entry: e.Index, Path: p.file, Remedy: fix, Detail: detail})
		}
	}
	if len(refusals) > 0 {
		return refusals
	}
	return nil
}

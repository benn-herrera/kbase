package claimgraph

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/records"
	"kbase/internal/write"
)

// splitEntry is one claim the split gives an analysis: the 1-based line the
// analysis's marker is appended to, the 1-based line and byte offset the
// claim's own marker moves to (both 0 where it stays), and whether the host is
// a proof rather than the claim's own block or paragraph.
type splitEntry struct {
	claim                       ClaimNode
	document                    string
	line                        int
	statementLine, statementEnd int
	atProof                     bool
}

// splitPlan is every split in reading order, the bound proofs counted for the
// report, and the claims left unsplit because the block that would host their
// analysis has no blank line to mark it on.
type splitPlan struct {
	entries                          []splitEntry
	proofsBound, proofsInNoStatement int
	noBlankLine                      int
}

// Split is the analysis split: each claim whose bounds — its own block or
// paragraph and the proofs establishing it — hold text beyond its statement
// proper gets an analysis carrying the rest, all issued in one split-claim
// batch, then the gate. It asks nothing.
func Split(opts Options) (Report, error) {
	return run(opts, func(t *Tree, recs []records.Record, w writer) ([]Finding, error) {
		if err := passTwoGate(t); err != nil {
			return nil, err
		}
		inv, err := scanInventory(t, recs)
		if err != nil {
			return nil, err
		}
		g, err := readGraph(t, &inv)
		if err != nil {
			return nil, err
		}
		p := planSplits(t, g, &inv)
		ownText := 0
		vs := make(values, len(p.entries))
		for i, e := range p.entries {
			title := kb.RebaseInlineLinks(e.claim.Title, path.Dir(index.RegisterPage(e.claim.Document, t.volumes)), path.Dir(index.RegisterPage(e.document, t.volumes)))
			rationale := splitProofRationale(e.document)
			if !e.atProof {
				ownText++
				rationale = splitOwnTextRationale(e.document)
			}
			vs[i] = map[string]any{"id": e.claim.ID, "title": title, "rationale": rationale, "document": e.document, "line": e.line}
			if e.statementEnd > 0 {
				vs[i]["statement-line"], vs[i]["statement-end"] = e.statementLine, e.statementEnd
			}
		}
		if len(vs) > 0 {
			if _, err := w.run("split", write.SplitClaim, vs); err != nil {
				return nil, err
			}
		}
		return []Finding{pass("analyses-split", field("analyses", len(p.entries)), field("in-own-text", ownText),
			field("at-a-proof", len(p.entries)-ownText), field("proofs-bound", p.proofsBound),
			field("proofs-in-no-statement", p.proofsInNoStatement), field("unsplit-no-blank-line", p.noBlankLine))}, nil
	})
}

// planSplits applies the split test to every claim but an equation node, in
// reading order. A block claim splits where a line of its block other than
// its label line and its display line holds text, and its analysis is marked
// on the block's first blank line. A prose claim splits where its paragraph,
// on any of its lines, holds text after the sentence its title is drawn from;
// its analysis is marked at the end of the paragraph's last line and its own
// marker moves to just after that sentence, on the line the sentence ends on.
// Where either line closes a fenced block, the marker goes at the end of the
// line before it. Any other claim splits where a proof establishing it holds
// text below its label line, its analysis marked on the first blank line of
// the first such proof in reading order. A claim whose hosting block has no
// blank line is counted and left unsplit.
func planSplits(t *Tree, g *AuthoredGraph, inv *Inventory) splitPlan {
	docs := map[string]int{}
	for i, p := range readingOrder(t) {
		docs[p] = i
	}
	lines := map[string][]string{}
	pageOf := func(doc string) []string {
		if lines[doc] == nil {
			lines[doc] = pageLines(t, doc)
		}
		return lines[doc]
	}

	var plan splitPlan
	bound := bindProofClaims(g, inv)
	proofs := make([]boundProof, 0, len(bound))
	for _, bp := range bound {
		if len(bp.subjects) > 0 {
			proofs = append(proofs, bp)
		}
	}
	plan.proofsBound = len(proofs)
	slices.SortFunc(proofs, func(a, b boundProof) int {
		return cmp.Or(cmp.Compare(docs[a.proof.Document], docs[b.proof.Document]), strings.Compare(a.proof.Document, b.proof.Document),
			cmp.Compare(a.proof.Start, b.proof.Start))
	})
	establishing := map[string][]Proof{}
	for _, bp := range proofs {
		for _, n := range bp.subjects {
			if ps := establishing[n.ID]; len(ps) == 0 || ps[len(ps)-1].order != bp.proof.order {
				establishing[n.ID] = append(ps, bp.proof)
			}
		}
	}

	bodies := map[string]claimBody{}
	for _, b := range claimBodies(t, g, inv) {
		if _, ok := bodies[b.node.ID]; !ok {
			bodies[b.node.ID] = b
		}
	}
	leaves := map[string]*leafReading{}
	pos := claimPositions(t, g, inv)
	nodes := slices.DeleteFunc(slices.Clone(g.Nodes), func(n ClaimNode) bool { return n.Equation != "" })
	slices.SortFunc(nodes, func(a, b ClaimNode) int {
		return cmp.Or(comparePositions(pos[a.ID], pos[b.ID]), strings.Compare(a.ID, b.ID))
	})
	hosting := map[int]bool{}
	for _, n := range nodes {
		if b, ok := bodies[n.ID]; ok && b.inBlock {
			display := kb.MarkerLines(t.Documents[n.Document].Text)[n.ID]
			if blockHoldsMore(strings.Split(b.text, "\n"), b.first, display) {
				if line := firstBlankLine(pageOf(n.Document), b.first, b.first+strings.Count(b.text, "\n")+1); line > 0 {
					plan.entries = append(plan.entries, splitEntry{claim: n, document: n.Document, line: line})
				} else {
					plan.noBlankLine++
				}
				continue
			}
		} else if ok {
			if leaves[n.Document] == nil {
				leaves[n.Document] = readLeaf(t.Documents[n.Document], inv)
			}
			if line, statementLine, end, more := proseHoldsMore(leaves[n.Document], t.Documents[n.Document].Text, b.first); more {
				plan.entries = append(plan.entries, splitEntry{claim: n, document: n.Document, line: line, statementLine: statementLine, statementEnd: end})
				continue
			}
		}
		for _, p := range establishing[n.ID] {
			page := pageOf(p.Document)
			if blockHoldsMore(page[p.Start:p.End], p.Start, p.Start) {
				if line := firstBlankLine(page, p.Start, p.End); line > 0 {
					hosting[p.order] = true
					plan.entries = append(plan.entries, splitEntry{claim: n, document: p.Document, line: line, atProof: true})
				} else {
					plan.noBlankLine++
				}
				break
			}
		}
	}
	plan.proofsInNoStatement = plan.proofsBound - len(hosting)
	return plan
}

// blockHoldsMore is whether a block's lines, markers off and unquoted, the
// first being line start, hold text on a line other than start and display.
func blockHoldsMore(block []string, start, display int) bool {
	for i, l := range block {
		if k := start + i; k != start && k != display && strings.TrimSpace(l) != "" {
			return true
		}
	}
	return false
}

// firstBlankLine is the 1-based number of the first line of page, as pageLines
// gives its lines, after start and before end that is empty once trimmed; 0
// where none is.
func firstBlankLine(page []string, start, end int) int {
	for k := start + 1; k < min(end, len(page)); k++ {
		if strings.TrimSpace(page[k]) == "" {
			return k + 1
		}
	}
	return 0
}

// proseHoldsMore is, for the prose claim whose paragraph begins on 0-based
// line first of text, whether its paragraph holds text other than whitespace,
// `>` prefixes and markers between the end of the sentence its title is drawn
// from and the end of the paragraph's last line; and where it does, the
// 1-based line the analysis's marker ends, which is that last line, and the
// 1-based line and byte offset the claim's marker moves to, which is the end
// of that sentence on the line it ends on. A line that closes a fenced block
// gives way to the line before it, at that line's end. The offset is the same
// in text as in its marker-stripped form: the build places a marker on a
// paragraph's line only at its end.
func proseHoldsMore(leaf *leafReading, text string, first int) (line, statementLine, statementEnd int, more bool) {
	p, ok := leaf.render.paragraphAt(first)
	if !ok {
		return 0, 0, 0, false
	}
	s := leaf.render.spanOf(p)
	if len(s) == 0 {
		return 0, 0, 0, false
	}
	o := opener(s, leaf.fences)
	body := stripMarkers(text)
	lines := strings.Split(body, "\n")
	closes := map[int]bool{}
	for _, f := range kb.Fences(lines) {
		if f.Closed {
			closes[f.End-1] = true
		}
	}
	at := strings.Count(body[:o.end], "\n")
	end := o.end - strings.LastIndexByte(body[:o.end], '\n') - 1
	if closes[at] {
		at--
		end = len(lines[at])
	}
	last := slices.Max(p.lines)
	if closes[last] {
		last--
	}
	between := lines[at][end:]
	for _, l := range lines[at+1 : last+1] {
		between += kb.BlockquotePrefix.ReplaceAllString(l, "")
	}
	if strings.TrimSpace(between) == "" {
		return 0, 0, 0, false
	}
	return last + 1, at + 1, end, true
}

// splitOwnTextRationale is the rationale of an analysis the split hosts in its
// claim's own block or paragraph in document.
func splitOwnTextRationale(document string) string {
	return "Split by the build from the claim it lifts: the text of that claim's own block or paragraph in " + document +
		" beyond the statement it keeps. The premises and cited works the build found for that claim, in that block or " +
		"paragraph and in the proofs establishing it, support this analysis instead. Neither rigor nor bearing " +
		"assessment has run over it."
}

// splitProofRationale is the rationale of an analysis the split hosts at a
// proof in document.
func splitProofRationale(document string) string {
	return "Split by the build from the claim it lifts: the proof in " + document + " that establishes that claim, " +
		"where this analysis's statement marker stands. The premises and cited works the build found for that claim, " +
		"in its block and its proofs, support this analysis instead. Neither rigor nor bearing assessment has run over it."
}

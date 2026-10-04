package claimgraph

import (
	"slices"

	"kbase/internal/kb"
)

// blocklessReason is what a leaf with no author-marked claim block declares
// from the declared pass: a fact about the document and nothing more.
const blocklessReason = "This document carries no author-marked claim block."

// entry is one register entry to insert, in mint order. Equation is the
// label of the equation it stands for, "" for a claim-bearing block's.
type entry struct {
	register, title, rationale, document, locator, equation string
}

// documentRecord is one document's frontmatter: its kind, the positions in
// the plan's entries of the claims it hosts, or its no-claim reason.
type documentRecord struct {
	path, kind string
	claims     []int
	noClaim    string
}

// marker is one Tier-2 marker: an entry's id at its own display line.
type marker struct {
	document string
	entry    int
	locator  string
}

// plan is everything the declared pass's writes land. Claims are named by
// position in entries because ids exist only once the first pass mints them.
type plan struct {
	entries   []entry
	documents []documentRecord
	markers   []marker
	works     []CitedWork
	// restsOn is (entry position, work id), the off-graph edges.
	restsOn []restsOn
}

type restsOn struct {
	entry  int
	workID string
}

func (p *plan) registers() []string {
	var out []string
	for _, e := range p.entries {
		if !slices.Contains(out, e.register) {
			out = append(out, e.register)
		}
	}
	slices.Sort(out)
	return out
}

func blockRationale(c Claim) string {
	return "Stated by the author as a labelled " + c.Environment + " block in " + c.Document + "; the title and " +
		"the locator are that block's own display line. Neither dependency attribution nor rigor " +
		"assessment has run over it."
}

func equationRationale(e referencedEquation) string {
	return "A labelled equation in " + e.document + " that this corpus's own cross-references name and that no " +
		"claim-bearing block and no proof holds. The title carries the equation's own label, which is what " +
		"a cross-reference resolves through; the equation is a node so that those references reach " +
		"something, not because anybody read it as stating a result. Neither dependency attribution nor " +
		"rigor assessment has run over it."
}

// equationTitle is the register title of the node standing for the equation
// labelled label in a document headed heading; kb.EquationLabel reads it back.
func equationTitle(label, heading string) string {
	return "Equation (`" + label + "`) — " + heading
}

// equationEntries is one entry per referenced equation nothing else holds,
// titled with its document's heading and its own label. A document with no
// heading is refused: a node named after a file is one no reader can place.
func equationEntries(t *Tree, referenced []referencedEquation) ([]entry, error) {
	var out []entry
	for _, e := range referenced {
		heading := kb.NormalizeSpace(t.Documents[e.document].Heading())
		if heading == "" {
			return nil, stopf("equation-heading", "%s carries no H1, so the equation labelled %q in it has no title to be minted under. Every document of a conforming tree carries one", e.document, e.label)
		}
		out = append(out, entry{register: registerFor(e.document), title: equationTitle(e.label, heading),
			rationale: equationRationale(e), document: e.document, locator: e.label, equation: e.label})
	}
	return out, nil
}

// assemblePlan is kb_claimgraph stage E for the declared pass: the block
// claims arranged into registers, every document's frontmatter, the markers
// a multi-claim document carries, and the endcap's works and edges.
func assemblePlan(t *Tree, inv *Inventory, claims []Claim) plan {
	var p plan
	hosted := map[string][]int{}
	for i, c := range claims {
		p.entries = append(p.entries, entry{register: registerFor(c.Document), title: c.Title, rationale: blockRationale(c),
			document: c.Document, locator: c.Locator})
		hosted[c.Document] = append(hosted[c.Document], i)
	}
	for _, path := range t.Paths {
		kind := t.kind(path)
		rec := documentRecord{path: path, kind: kind, claims: hosted[path]}
		if declaringKinds[kind] && len(rec.claims) == 0 {
			rec.noClaim = blocklessReason
		}
		p.documents = append(p.documents, rec)
	}
	for _, path := range t.Paths {
		if positions := hosted[path]; len(positions) > 1 {
			for _, i := range positions {
				p.markers = append(p.markers, marker{path, i, p.entries[i].locator})
			}
		}
	}
	off := scanEndcap(inv)
	positionOf := map[claimSite]int{}
	for i, e := range p.entries {
		positionOf[claimSite{e.document, e.locator}] = i
	}
	cited := map[string]bool{}
	for _, s := range off.sortedSites() {
		i, ok := positionOf[s]
		if !ok {
			continue
		}
		for _, id := range off.pairings[s] {
			p.restsOn = append(p.restsOn, restsOn{i, id})
			cited[id] = true
		}
	}
	for _, w := range off.works {
		if cited[w.ID()] {
			p.works = append(p.works, w)
		}
	}
	return p
}

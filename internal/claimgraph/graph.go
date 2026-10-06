package claimgraph

import (
	"fmt"
	"path"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/write"
)

// Claim is one claim-bearing block's claim, before an id exists for it: its
// title and locator are both read off the block's display line.
type Claim struct {
	Document, Title, Locator, Environment, Identifier string
}

// blockClaims is C-mech: one claim per claim-bearing block.
func blockClaims(inv *Inventory) []Claim {
	var out []Claim
	for _, b := range inv.claimBlocks() {
		out = append(out, Claim{b.Document, b.Title, b.Display, b.Environment, b.Identifier})
	}
	return out
}

// unmarkedDocuments is every leaf hosting no claim-bearing block.
func unmarkedDocuments(t *Tree, inv *Inventory) []string {
	hosting := map[string]bool{}
	for _, b := range inv.claimBlocks() {
		hosting[b.Document] = true
	}
	var out []string
	for _, p := range t.Paths {
		if !hosting[p] && declaringKinds[t.kind(p)] {
			out = append(out, p)
		}
	}
	return out
}

// registerFor is the register a claim hosted by document belongs in: its
// volume's. The external works are registered at the KB root.
func registerFor(document string) string {
	domain, _, _ := strings.Cut(document, "/")
	return domain + "/" + kb.RegisterFile
}

// registerTitle is a title read off document as document's register carries
// it: every relative link rebased to resolve from the register's directory.
func registerTitle(title, document string) string {
	return kb.RebaseInlineLinks(title, path.Dir(document), path.Dir(registerFor(document)))
}

// ClaimNode is one minted claim as the authored graph carries it. Locator is
// where in its document the claim sits — its block's display line, or the
// line its Tier-2 marker is on — "" where neither join answers. Equation is
// the label of the equation it stands for, "" for any other claim.
type ClaimNode struct {
	ID, Document, Title, Locator, Identifier, Equation string
}

// AuthoredGraph is every claim the tree declares, in declaration order.
type AuthoredGraph struct {
	Nodes []ClaimNode
	byID  map[string]int
}

func (g *AuthoredGraph) node(id string) ClaimNode { return g.Nodes[g.byID[id]] }

// hostedBy is the claims document hosts, equation nodes not among them: they
// are reachable through the label-to-fence join alone.
func (g *AuthoredGraph) hostedBy(document string) []ClaimNode {
	var out []ClaimNode
	for _, n := range g.Nodes {
		if n.Document == document && n.Equation == "" {
			out = append(out, n)
		}
	}
	return out
}

func (g *AuthoredGraph) equationNode(document, label string) (ClaimNode, bool) {
	for _, n := range g.Nodes {
		if n.Document == document && n.Equation == label {
			return n, true
		}
	}
	return ClaimNode{}, false
}

func (g *AuthoredGraph) documents() int {
	seen := map[string]bool{}
	for _, n := range g.Nodes {
		seen[n.Document] = true
	}
	return len(seen)
}

// registerTitles is every minted claim id in the KB with its entry's title.
func registerTitles(kbRoot string) (map[string]string, error) {
	regs, err := kb.Registers(builtTree(kbRoot))
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	for _, rel := range regs {
		entries, err := claimEntries(kbRoot, rel)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			titles[e.ID] = e.Title
		}
	}
	return titles, nil
}

// markerLocator is the line carrying id's Tier-2 marker, as a locator: its
// authored content, markers and quote prefix off, whitespace collapsed.
func markerLocator(text, id string) string {
	marker := write.Tier2Marker(id)
	for _, line := range kb.SplitLines(text) {
		if strings.Contains(line, marker) {
			return kb.NormalizeSpace(kb.BlockquotePrefix.ReplaceAllString(stripMarkers(line), ""))
		}
	}
	return ""
}

// readGraph joins each document's claims: declarations to the registers'
// entries, and each claim to its site — an equation by the label its title
// carries, a block claim by title, any other by its marker.
func readGraph(t *Tree, inv *Inventory) (*AuthoredGraph, error) {
	titles, err := registerTitles(t.Root)
	if err != nil {
		return nil, err
	}
	type site struct{ display, identifier string }
	blocks := map[string]map[string]site{}
	for _, b := range inv.claimBlocks() {
		if blocks[b.Document] == nil {
			blocks[b.Document] = map[string]site{}
		}
		blocks[b.Document][registerTitle(b.Title, b.Document)] = site{b.Display, b.Identifier}
	}
	g := &AuthoredGraph{byID: map[string]int{}}
	for _, p := range t.Paths {
		fm, err := kb.ParseFrontmatter(t.Documents[p].Text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		ids, err := fm.ListOrEmpty("claims")
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			title, ok := titles[id]
			if !ok {
				return nil, stopf("orphan-claim", "%s declares %s, which no register in this KB mints. The declaration and the register disagree about what exists, and an edge authored over that disagreement would name a node with no entry", p, id)
			}
			n := ClaimNode{ID: id, Document: p, Title: title}
			if label, ok := kb.EquationLabel(title); ok {
				n.Locator, n.Equation = label, label
			} else if s, ok := blocks[p][title]; ok {
				n.Locator, n.Identifier = s.display, s.identifier
			} else {
				n.Locator = markerLocator(t.Documents[p].Text, id)
			}
			if at, seen := g.byID[id]; seen {
				g.Nodes[at] = n
				continue
			}
			g.byID[id] = len(g.Nodes)
			g.Nodes = append(g.Nodes, n)
		}
	}
	return g, nil
}

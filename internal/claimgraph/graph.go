package claimgraph

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"kbase/internal/kb"
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

// ClaimNode is one minted claim as the authored graph carries it. Locator is
// where in its document the claim sits — its block's display line, or the
// line its statement marker is on — "" where neither join answers. Equation is
// the label of the equation it stands for, "" for any other claim.
type ClaimNode struct {
	ID, Document, Title, Locator, Identifier, Equation string
}

// AuthoredGraph is every claim the tree declares, in declaration order.
type AuthoredGraph struct {
	Nodes []ClaimNode
	byID  map[string]int
}

// node is the claim id names, false where the graph holds none.
func (g *AuthoredGraph) node(id string) (ClaimNode, bool) {
	i, ok := g.byID[id]
	if !ok {
		return ClaimNode{}, false
	}
	return g.Nodes[i], true
}

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

// markerLocator is the line carrying id's statement marker, as a locator: its
// authored content, markers and quote prefix off, whitespace collapsed.
func markerLocator(text, id string) string {
	marker := kb.StatementMarker(id)
	for _, line := range kb.SplitLines(text) {
		if strings.Contains(line, marker) {
			return kb.NormalizeSpace(kb.BlockquotePrefix.ReplaceAllString(stripMarkers(line), ""))
		}
	}
	return ""
}

// readGraph joins each leaf's statement markers to the claim records — a claim
// marked in several leaves is hosted by the first in path order — and each
// claim to its site: an equation by the label its title carries, a block claim
// by title, any other by its marker. A leaf's claims are in marker order.
func readGraph(t *Tree, inv *Inventory) (*AuthoredGraph, error) {
	recs, err := kb.LoadGraph(builtTree(t.Root))
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	for _, n := range recs.Claims {
		titles[n.ID] = n.Title
	}
	type site struct{ display, identifier string }
	blocks := map[string]map[string]site{}
	for _, b := range inv.claimBlocks() {
		if blocks[b.Document] == nil {
			blocks[b.Document] = map[string]site{}
		}
		blocks[b.Document][t.registerTitle(b.Title, b.Document)] = site{b.Display, b.Identifier}
	}
	g := &AuthoredGraph{byID: map[string]int{}}
	for _, p := range t.Paths {
		if !declaringKinds[t.kind(p)] {
			continue
		}
		lines := kb.MarkerLines(t.Documents[p].Text)
		ids := slices.SortedFunc(maps.Keys(lines), func(a, b string) int {
			return cmp.Or(cmp.Compare(lines[a], lines[b]), strings.Compare(a, b))
		})
		for _, id := range ids {
			if kind, _ := kb.NodeIDKind(id); kind != kb.NodeKindClaim {
				continue
			}
			if _, seen := g.byID[id]; seen {
				continue
			}
			title, ok := titles[id]
			if !ok {
				return nil, stopf("orphan-claim", "%s marks %s, which no node record in this KB carries. The marker and the records disagree about what exists, and an edge authored over that disagreement would name a node with no record", p, id)
			}
			n := ClaimNode{ID: id, Document: p, Title: title}
			if label, ok := kb.EquationLabel(title); ok {
				n.Locator, n.Equation = label, label
			} else if s, ok := blocks[p][title]; ok {
				n.Locator, n.Identifier = s.display, s.identifier
			} else {
				n.Locator = markerLocator(t.Documents[p].Text, id)
			}
			g.byID[id] = len(g.Nodes)
			g.Nodes = append(g.Nodes, n)
		}
	}
	return g, nil
}

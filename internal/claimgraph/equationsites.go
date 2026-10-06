package claimgraph

import "strings"

// equationFences is each equation node's fence, by node id: the first fence
// in the node's document carrying its label, as the minting stage reads it.
// A node whose label no fence carries is absent.
func equationFences(g *AuthoredGraph, inv *Inventory) map[string]MathFence {
	type site struct{ document, label string }
	byLabel := map[site]MathFence{}
	for _, f := range inv.Fences {
		for _, l := range f.Labels {
			if _, ok := byLabel[site{f.Document, l}]; !ok {
				byLabel[site{f.Document, l}] = f
			}
		}
	}
	out := map[string]MathFence{}
	for _, n := range g.Nodes {
		if f, ok := byLabel[site{n.Document, n.Equation}]; ok && n.Equation != "" {
			out[n.ID] = f
		}
	}
	return out
}

// ownEquations is every (claim, equation node) pair whose equation's fence
// lies inside that claim's own body: the claim states the equation rather
// than pointing at it, so the pair is no dependency either way and no
// reference.
func ownEquations(t *Tree, g *AuthoredGraph, inv *Inventory) map[pair]bool {
	type extent struct {
		id         string
		start, end int
	}
	extents := map[string][]extent{}
	for _, b := range claimBodies(t, g, inv) {
		extents[b.node.Document] = append(extents[b.node.Document], extent{b.node.ID, b.first, b.first + len(strings.Split(b.text, "\n"))})
	}
	out := map[pair]bool{}
	for eq, f := range equationFences(g, inv) {
		for _, e := range extents[f.Document] {
			if e.start <= f.Start && f.End <= e.end {
				out[pair{e.id, eq}] = true
			}
		}
	}
	return out
}

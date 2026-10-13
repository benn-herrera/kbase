package claimgraph

import (
	"cmp"
	"math"
	"slices"

	"kbase/internal/kb"
)

// position is where a claim sits in reading order: its document's index in
// readingOrder, and its line in that document.
type position struct{ doc, line int }

// readingOrder is every document reachable from the entry point, in preorder
// over each document's children as it lists them.
func readingOrder(t *Tree) []string {
	var out []string
	var walk func(p string)
	walk = func(p string) {
		out = append(out, p)
		for _, c := range t.Children[p] {
			walk(c)
		}
	}
	walk(kb.EntryPointFile)
	return out
}

// claimPositions is every claim's position: a block or prose claim at its
// body's first line, an equation node at its fence's first line, and a claim
// neither reaches last in its document.
func claimPositions(t *Tree, g *AuthoredGraph, inv *Inventory) map[string]position {
	docs := map[string]int{}
	for i, p := range readingOrder(t) {
		docs[p] = i
	}
	lines := map[string]int{}
	for _, b := range claimBodies(t, g, inv) {
		if at, ok := lines[b.node.ID]; !ok || b.first < at {
			lines[b.node.ID] = b.first
		}
	}
	for id, f := range equationFences(g, inv) {
		lines[id] = f.Start
	}
	out := make(map[string]position, len(g.Nodes))
	for _, n := range g.Nodes {
		line, ok := lines[n.ID]
		if !ok {
			line = math.MaxInt
		}
		out[n.ID] = position{docs[n.Document], line}
	}
	return out
}

func comparePositions(a, b position) int {
	if c := cmp.Compare(a.doc, b.doc); c != 0 {
		return c
	}
	return cmp.Compare(a.line, b.line)
}

// link is an unordered pair of claims, held with a < b by id.
type link struct{ a, b string }

func linkOf(x, y string) link {
	if y < x {
		x, y = y, x
	}
	return link{x, y}
}

// proseClaims is every claim that is neither an equation node nor a
// claim-bearing block's claim.
func proseClaims(g *AuthoredGraph, inv *Inventory) map[string]bool {
	inBlocks := map[string]bool{}
	for _, s := range blockClaimSites(g, inv) {
		inBlocks[s.node.ID] = true
	}
	out := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Equation == "" && !inBlocks[n.ID] {
			out[n.ID] = true
		}
	}
	return out
}

// adjacentLinks links each document's consecutive claims, equation nodes
// left out, in (line, id) order, less the marked links; sorted.
func adjacentLinks(t *Tree, g *AuthoredGraph, _ *Inventory, pos map[string]position, marked map[link]bool) []link {
	var out []link
	for _, p := range t.Paths {
		hosted := g.hostedBy(p)
		slices.SortFunc(hosted, func(x, y ClaimNode) int {
			if c := cmp.Compare(pos[x.ID].line, pos[y.ID].line); c != 0 {
				return c
			}
			return cmp.Compare(x.ID, y.ID)
		})
		for i := 1; i < len(hosted); i++ {
			if l := linkOf(hosted[i-1].ID, hosted[i].ID); !marked[l] {
				out = append(out, l)
			}
		}
	}
	slices.SortFunc(out, func(x, y link) int {
		if c := cmp.Compare(x.a, y.a); c != 0 {
			return c
		}
		return cmp.Compare(x.b, y.b)
	})
	return out
}

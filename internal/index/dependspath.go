package index

import (
	"slices"

	"kbase/internal/kb"
)

// DependsGraph is the premise graph the acyclicity check walks: each claim or
// support to every claim it depends on, and each claim to every support
// supporting it.
type DependsGraph map[string][]string

// AuthoredDependsGraph is st's premise graph as its registers author it.
func AuthoredDependsGraph(st kb.State) DependsGraph {
	g := DependsGraph{}
	add := func(from string, edges []kb.Edge) {
		for _, e := range edges {
			if e.Relation == kb.RelationDepends && e.TargetKind == "claim" {
				g.Add(from, e.Target)
			}
		}
	}
	for _, e := range st.ClaimEntries {
		add(e.ID, e.DependsOn)
	}
	for _, s := range st.Supports {
		add(s.ID, s.DependsOn)
		for _, p := range s.Supports {
			g.Add(p.ClaimID, s.ID)
		}
	}
	return g
}

// Add is the edge from → to.
func (g DependsGraph) Add(from, to string) { g[from] = append(g[from], to) }

// Path is a shortest path from → … → to, both ends included, each node's
// successors taken in sorted order; nil where to is unreachable.
func (g DependsGraph) Path(from, to string) []string {
	prev := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		if at == to {
			path := []string{at}
			for at != from {
				at = prev[at]
				path = append(path, at)
			}
			slices.Reverse(path)
			return path
		}
		next := slices.Clone(g[at])
		slices.Sort(next)
		for _, n := range next {
			if _, seen := prev[n]; !seen {
				prev[n] = at
				queue = append(queue, n)
			}
		}
	}
	return nil
}

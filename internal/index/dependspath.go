package index

import (
	"slices"

	"kbase/internal/kb"
)

// DependsGraph is the premise graph the acyclicity check walks: from a node to
// every node its value is drawn from, one edge per scoring relation target →
// source. It is the reverse of the push direction the walk reads values in, so
// a cycle in it is a cycle in the derivation.
type DependsGraph map[string][]string

// AuthoredDependsGraph is st's premise graph as its registers and leaves
// author it.
func AuthoredDependsGraph(st kb.State) DependsGraph {
	g := DependsGraph{}
	add := func(edges []kb.Edge) {
		for _, e := range edges {
			if kb.Scoring(e.Relation) {
				g.Add(e.Target, e.Source)
			}
		}
	}
	for _, e := range st.ClaimEntries {
		add(e.Premises)
		add(e.Refutes)
	}
	for _, s := range st.Analyses {
		add(s.Premises)
		add(s.Lifts)
		add(s.Refutes)
	}
	for _, x := range st.Experiments {
		add(x.Supports)
		add(x.Lifts)
		add(x.Refutes)
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

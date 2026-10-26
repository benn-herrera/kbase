package index

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// DependsGraph is the premise graph the acyclicity check walks: from a node to
// every node its value is drawn from, one edge per scoring relation target →
// source. It is the reverse of the push direction the walk reads values in, so
// a cycle in it is a cycle in the derivation.
type DependsGraph map[string][]string

// GraphDependsGraph is g's premise graph over its founds, supports, lifts and
// refutes records.
func GraphDependsGraph(g kb.Graph) DependsGraph {
	d := DependsGraph{}
	for _, e := range g.Edges() {
		if kb.Scoring(e.Relation) {
			d.Add(e.Target, e.Source)
		}
	}
	return d
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

// cycleEdge is one scoring edge record as BreakCycles walks it.
type cycleEdge struct {
	relation, source, target string
	record                   kb.EdgeRecord
}

// EdgeName is an edge as a warning or a refusal names it.
func EdgeName(relation, source, target string) string {
	return relation + " " + source + " → " + target
}

// BreakCycles is g with no cycle among its scoring edges, the edges it cut in
// the order it cut them, and a warning per cut. The walk order is hosted's
// nodes in its order, then every other node by id; an edge's place in it is its
// relation's in kb.Relations, then its source's, then its target's. The walk is
// depth-first over the supports, founds, lifts and refutes records, starting
// from every node and following each node's edges in the walk order; demoted
// records are not walked. An edge reaching a node on the current path closes a
// cycle, of which one edge is cut: the first in the walk order of its supports
// edges into an analysis, else the closing edge. After each cut the walk starts
// over, so the cuts depend on the edge set and the walk order alone. A cut edge
// becomes its DemotedEdge, as demote-claim-edge makes it: where a demoted
// record of its identity already stands, the cut edge goes and that record
// stands alone. Nothing else is dropped.
func BreakCycles(g kb.Graph, hosted []string) (kb.Graph, []kb.ScorableEdge, []result.Item) {
	rank := map[string]int{}
	for i, id := range hosted {
		rank[id] = i
	}
	node := func(a, b string) int {
		ra, ok := rank[a]
		if !ok {
			ra = len(hosted)
		}
		rb, ok := rank[b]
		if !ok {
			rb = len(hosted)
		}
		return cmp.Or(cmp.Compare(ra, rb), strings.Compare(a, b))
	}
	kinds := GraphKinds(g)
	starts := slices.Collect(maps.Keys(kinds))
	var edges []cycleEdge
	for _, e := range g.Edges() {
		if kb.Scoring(e.Relation) {
			edges = append(edges, cycleEdge{e.Relation, e.Source, e.Target, e})
			starts = append(starts, e.Source)
		}
	}
	slices.SortStableFunc(edges, func(a, b cycleEdge) int {
		return cmp.Or(cmp.Compare(slices.Index(kb.Relations, a.relation), slices.Index(kb.Relations, b.relation)),
			node(a.source, b.source), node(a.target, b.target))
	})
	slices.SortFunc(starts, node)
	starts = slices.Compact(starts)
	cut := map[[3]string]bool{}
	demoted := slices.Clone(g.Demoted)
	var cuts []kb.ScorableEdge
	var warnings []result.Item
	for {
		at, cycle, found := closeCycle(edges, starts, kinds)
		if !found {
			break
		}
		// Only a scorable record is ever cut: a founds source, a work or an
		// axiom, takes no scoring edge in, so no cycle passes through one.
		r := edges[at].record
		e := kb.ScorableEdge{Relation: r.Relation, Source: r.Source, Target: r.Target, Context: r.Context,
			Bearing: *r.Bearing, ManuallyScored: r.ManuallyScored}
		cut[[3]string{e.Relation, e.Source, e.Target}] = true
		cuts = append(cuts, e)
		if !slices.ContainsFunc(demoted, func(d kb.DemotedEdge) bool {
			return d.ReplacedRelation == e.Relation && d.Source == e.Source && d.Target == e.Target
		}) {
			demoted = append(demoted, kb.DemotedEdge{Source: e.Source, Target: e.Target, Context: e.Context,
				ReplacedRelation: e.Relation, Bearing: e.Bearing, ManuallyScored: e.ManuallyScored})
		}
		warnings = append(warnings, result.Item{Path: kb.GraphEdgesFile,
			Detail: fmt.Sprintf("%s is demoted, breaking the cycle %s", EdgeName(e.Relation, e.Source, e.Target),
				strings.Join(append(cycle, cycle[0]), " → "))})
		edges = slices.Delete(edges, at, at+1)
	}
	if len(cut) == 0 {
		return g, nil, nil
	}
	out := g
	out.Scorable = nil
	for _, e := range g.Scorable {
		if !cut[[3]string{e.Relation, e.Source, e.Target}] {
			out.Scorable = append(out.Scorable, e)
		}
	}
	out.Demoted = demoted
	return out, cuts, warnings
}

// closeCycle walks edges, sorted in the walk order, depth-first from each of
// starts in turn until an edge reaches a node on the current path. It answers the
// index in edges of the edge to cut and the cycle's nodes in path order, from
// the node the closing edge reaches; found is false where no edge closes one.
func closeCycle(edges []cycleEdge, starts []string, kinds map[string]string) (at int, cycle []string, found bool) {
	out := map[string][]int{}
	for i, e := range edges {
		out[e.source] = append(out[e.source], i)
	}
	const (
		unseen = iota
		onPath
		done
	)
	state := map[string]int{}
	var path []string
	var via []int // via[k] is the edge that reached path[k], -1 for a start
	var visit func(node string, from int) bool
	visit = func(node string, from int) bool {
		state[node] = onPath
		path, via = append(path, node), append(via, from)
		for _, i := range out[node] {
			switch next := edges[i].target; state[next] {
			case onPath:
				k := slices.Index(path, next)
				at = i
				first := -1
				for _, j := range append(slices.Clone(via[k+1:]), i) {
					if e := edges[j]; e.relation == kb.RelationSupports && kinds[e.target] == kb.NodeKindAnalysis && (first < 0 || j < first) {
						first = j
					}
				}
				if first >= 0 {
					at = first
				}
				cycle = slices.Clone(path[k:])
				return true
			case unseen:
				if visit(next, i) {
					return true
				}
			}
		}
		state[node] = done
		path, via = path[:len(path)-1], via[:len(via)-1]
		return false
	}
	for _, s := range starts {
		if state[s] == unseen && visit(s, -1) {
			return at, cycle, true
		}
	}
	return 0, nil, false
}

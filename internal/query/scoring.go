package query

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// The score passes, the values ScoringSet takes. PassBoth is the set a run
// walks where none is named: both of them, one after the other.
const (
	PassRigor   = "rigor"
	PassBearing = "bearing"
	PassBoth    = "both"
)

// ScoringNode is one node a score pass asks about: its kind and its id.
type ScoringNode struct {
	Kind string
	ID   string
}

// ScoringEdge is one edge a score pass asks about, by its identity. The row
// itself, and the authored context it carries, is Index.Edge's answer.
type ScoringEdge struct {
	Source   string
	Target   string
	Relation string
}

// Item is one item of a scoring set: the node a rigor call carries, or the edge
// a bearing call carries, exactly one of the two.
type Item struct {
	Node *ScoringNode
	Edge *ScoringEdge
}

// ScoringSet is the items a score run walks, in the order it walks them. A
// rigor pass is every claim, analysis and experiment, by id: a work's rigor is
// judged by hand and an axiom's is 1.0 by rule, so neither is a subject. A
// bearing pass is every edge the walk asks about — supports, lifts and refutes,
// by relation in kb.BearingWalkRelations order, then source, then target. A
// founds edge is no item: its bearing is the user's, written by hand through
// set-bearing, and a reference or a demoted row carries no bearing at all.
// PassBoth is the node set and then the edge set, each in its own order, so one
// walk covers both and neither set's order depends on the other's. An unknown
// pass is an error.
func ScoringSet(ix *Index, pass string) ([]Item, error) {
	switch pass {
	case PassRigor:
		return rigorItems(ix), nil
	case PassBearing:
		return bearingItems(ix), nil
	case PassBoth:
		return append(rigorItems(ix), bearingItems(ix)...), nil
	}
	return nil, fmt.Errorf("unknown pass %q: the passes are %s, %s and %s", pass, PassRigor, PassBearing, PassBoth)
}

// rigorItems is every node the rigor pass asks about, by id: every kind but the
// work and the axiom.
func rigorItems(ix *Index) []Item {
	var nodes []ScoringNode
	for _, n := range ix.nodes {
		if n.NodeType == kb.NodeKindWork || n.NodeType == kb.NodeKindAxiom {
			continue
		}
		nodes = append(nodes, ScoringNode{Kind: n.NodeType, ID: n.ID})
	}
	slices.SortFunc(nodes, func(a, b ScoringNode) int { return strings.Compare(a.ID, b.ID) })
	out := make([]Item, len(nodes))
	for i := range nodes {
		out[i] = Item{Node: &nodes[i]}
	}
	return out
}

// bearingItems is every edge the walk asks about, by relation, then source,
// then target.
func bearingItems(ix *Index) []Item {
	var edges []ScoringEdge
	for _, e := range ix.edges {
		if kb.BearingWalk(e.Relation) {
			edges = append(edges, ScoringEdge{Source: e.Source, Target: e.Target, Relation: e.Relation})
		}
	}
	slices.SortStableFunc(edges, func(a, b ScoringEdge) int {
		return cmp.Or(slices.Index(kb.BearingWalkRelations, a.Relation)-slices.Index(kb.BearingWalkRelations, b.Relation),
			strings.Compare(a.Source, b.Source), strings.Compare(a.Target, b.Target))
	})
	out := make([]Item, len(edges))
	for i := range edges {
		out[i] = Item{Edge: &edges[i]}
	}
	return out
}

// Edge is the index row carrying e's identity, or false. Where an identity
// repeats, the last in load order answers, as Node's does.
func (ix *Index) Edge(e ScoringEdge) (kb.EdgeRow, bool) {
	var found kb.EdgeRow
	ok := false
	for _, row := range ix.edges {
		if row.Source == e.Source && row.Target == e.Target && row.Relation == e.Relation {
			found, ok = row, true
		}
	}
	return found, ok
}

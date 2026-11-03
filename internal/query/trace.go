package query

import (
	"cmp"
	"slices"
	"strconv"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// trace's sort keys: depth, its default, and contribution.
const (
	SortDepth        = "depth"
	SortContribution = "contribution"
)

// TraceSorts is the keys trace sorts by.
var TraceSorts = []string{SortDepth, SortContribution}

// TraceDirections is the directions trace walks.
var TraceDirections = []string{DirectionIn, DirectionOut}

// TraceSelection is trace's question: the scoring edges reached from the node
// ID carries, walking Direction ("" where none is given). Sort is nil where
// none is given, ordering by depth; Order is one of Orders; Limit is nil
// where none is given.
type TraceSelection struct {
	ID        string
	Direction string
	Sort      *string
	Order     string
	Limit     *int
}

// TracedEdge is one edge trace reaches: its row, its distance from the node,
// its source's strength (nil where pending), and its contribution to its
// target (index.EdgeContribution; nil where pending), or Excluded where the
// edge takes no part in the derivation.
type TracedEdge struct {
	Edge         kb.EdgeRow
	Depth        int
	Strength     *float64
	Contribution *float64
	Excluded     bool
}

// Trace is every scoring edge reached from sel.ID walking sel.Direction — in
// from each node to the edges landing on it, out to the edges leaving it —
// each once, at the depth of its first reach, in its sort's order, cut to its
// limit. Ties break by ascending depth, then edges' fixed order. It assumes
// sel's vocabulary values are valid and its node stands (TracePayload).
func (ix *Index) Trace(sel TraceSelection) []TracedEdge {
	nodes := map[string]kb.NodeRow{}
	for _, n := range ix.nodes {
		nodes[n.ID] = n
	}
	visited := map[string]bool{sel.ID: true}
	frontier := map[string]bool{sel.ID: true}
	var out []TracedEdge
	for depth := 1; len(frontier) > 0; depth++ {
		reached := map[string]bool{}
		for _, e := range ix.edges {
			if !kb.Scoring(e.Relation) {
				continue
			}
			at, next := e.Target, e.Source
			if sel.Direction == DirectionOut {
				at, next = e.Source, e.Target
			}
			if !frontier[at] {
				continue
			}
			out = append(out, tracedEdge(e, depth, nodes[e.Source]))
			if !visited[next] {
				reached[next] = true
			}
		}
		for id := range reached {
			visited[id] = true
		}
		frontier = reached
	}
	key := SortDepth
	if sel.Sort != nil {
		key = *sel.Sort
	}
	descending := sel.Order == OrderDescending
	slices.SortStableFunc(out, func(a, b TracedEdge) int {
		byDepth := cmp.Compare(a.Depth, b.Depth)
		if key == SortContribution {
			return cmp.Or(compareRanks(a.contributionRank(), b.contributionRank(), descending), byDepth, compareEdgeRows(a.Edge, b.Edge))
		}
		if descending {
			byDepth = -byDepth
		}
		return cmp.Or(byDepth, compareEdgeRows(a.Edge, b.Edge))
	})
	if sel.Limit != nil {
		out = limited(out, *sel.Limit)
	}
	return out
}

// tracedEdge is e reached at depth from source: an experiment
// index.ExperimentTakesPart excludes takes no part, and every other source's
// strength enters as the derivation reads it.
func tracedEdge(e kb.EdgeRow, depth int, source kb.NodeRow) TracedEdge {
	t := TracedEdge{Edge: e, Depth: depth, Strength: nodeStrength(source)}
	if source.NodeType == kb.NodeKindExperiment && !index.ExperimentTakesPart(source.Status) {
		t.Excluded = true
		return t
	}
	t.Contribution = index.EdgeContribution(e.Relation, rowBearing(e), t.Strength)
	return t
}

// rowBearing is a scoring edge row's bearing, which the index writes as
// kb.BearingText spells it.
func rowBearing(e kb.EdgeRow) kb.Bearing {
	text, _ := e.Bearing.(string)
	b, _ := kb.BearingFromValue(text)
	return b
}

// contributionRank is t's contribution as a sort reads it: null where the edge
// takes no part.
func (t TracedEdge) contributionRank() rank {
	if t.Excluded {
		return rank{null: true}
	}
	return strengthRank(t.Contribution)
}

// TracePayload is a row per edge sel reaches (Trace): its edges row
// (EdgeRowRecord), then its depth, its source's strength and its contribution,
// each *pending* where pending and the contribution null where the edge takes
// no part. It refuses, as usage, a direction missing or outside
// TraceDirections and what rankUsage refuses, and an id no node carries.
func TracePayload(ix *Index, sel TraceSelection) ([]result.Record, []result.Item) {
	var usage []result.Item
	direction := argmark.Flag("direction")
	switch {
	case sel.Direction == "":
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: direction, Allowed: TraceDirections,
			Detail: direction + " is required: in walks what bears on the node's strength, out what its strength reaches"})
	case !slices.Contains(TraceDirections, sel.Direction):
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: direction, Allowed: TraceDirections,
			Detail: argmark.FlagValue("direction", strconv.Quote(sel.Direction)) + " names no direction"})
	}
	if usage = append(usage, rankUsage(sel.Sort, TraceSorts, sel.Order, sel.Limit)...); usage != nil {
		return nil, usage
	}
	if _, ok := ix.Node(sel.ID); !ok {
		return nil, []result.Item{NoNode(argmark.Positional("id"), sel.ID)}
	}
	out := []result.Record{}
	for _, t := range ix.Trace(sel) {
		var contribution any
		if !t.Excluded {
			contribution = pendingOrDisplay(t.Contribution)
		}
		out = append(out, append(EdgeRowRecord(t.Edge), result.Field{Key: "depth", Value: t.Depth},
			result.Field{Key: "strength", Value: pendingOrDisplay(t.Strength)}, result.Field{Key: "contribution", Value: contribution}))
	}
	return out, nil
}

// pendingOrDisplay is a value as a query row shows it: its display spelling's
// value, or *pending* where it is pending.
func pendingOrDisplay(v *float64) any {
	if v == nil {
		return kb.PendingLiteral
	}
	return displayStrength(v)
}

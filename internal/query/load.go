// Package query answers questions over a KB's derived index: the rows
// index.Derive yields from the KB's sources, and each query's payload shaped
// for its result document.
package query

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// Index is a KB's derived index as loaded. Nodes are sorted by id, kinds in
// kb.NodeKinds order where ids tie; every other list is in file order.
type Index struct {
	src          *kb.Source
	nodes        []kb.NodeRow
	edges        []kb.EdgeRow
	strengthenBy []kb.StrengthenByRow
	cites        []kb.CiteRow
	aggregates   []kb.AggregateRow
}

// Load is the derived index of the KB src reads, the rows index.Derive yields
// from its sources: Derive's refusals, a coverage gap and a premise cycle
// among them, are Load's, and fromRows's are as it states. A KB loaded at an
// older format is refused once: its index is derived, and the refresh that
// rewrites the KB writes it.
func Load(src *kb.Source) (*Index, error) {
	if src.Migrated() {
		return nil, result.Refusal{{Check: index.CheckIndex, Path: src.KBPath(kb.EntryPointFile), Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("the KB's metadata format is %s and kbase reads %s; the index is derived, and a refresh rewrites the KB in %s",
				src.Version(), kb.FormatVersion, kb.FormatVersion)}}
	}
	rows, err := index.Derive(src)
	if err != nil {
		return nil, err
	}
	return fromRows(src, rows)
}

// fromRows is the index of rows over src. A node of no known kind, an edge of
// no known relation or with a bearing neither text nor null, and an index that
// would answer a legal edge with no bearing token are a result.Refusal naming
// each one.
func fromRows(src *kb.Source, rows index.Rows) (*Index, error) {
	ix := &Index{src: src, nodes: rows.Nodes, edges: rows.Edges, strengthenBy: rows.StrengthenBy,
		cites: rows.Cites, aggregates: rows.Aggregates}
	var bad []result.Item
	for i := range ix.nodes {
		n := &ix.nodes[i]
		if !slices.Contains(kb.NodeKinds, n.NodeType) {
			bad = append(bad, result.Item{Check: index.CheckIndex,
				Detail: fmt.Sprintf("unknown node_type %q on record id %q; known types: %v", n.NodeType, n.ID, kb.NodeKinds)})
			continue
		}
		if n.NodeType == kb.NodeKindAnalysis {
			n.BuildBand = index.BuildBand(n.Strength)
		}
	}
	if bad != nil {
		return nil, result.Refusal(bad)
	}
	if err := ix.checkEdges(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(ix.nodes, func(a, b kb.NodeRow) int {
		if a.ID != b.ID {
			return strings.Compare(a.ID, b.ID)
		}
		return slices.Index(kb.NodeKinds, a.NodeType) - slices.Index(kb.NodeKinds, b.NodeType)
	})
	return ix, nil
}

// checkEdges refuses an index with an edge of no known relation, an edge whose
// bearing is neither text nor null, a legal scoring edge that carries no
// bearing token, and a scoring edge that leaves its source kind unknown: the
// rows are derived, so any of these is an index a refresh must rewrite, never
// a state to answer from.
func (ix *Index) checkEdges() error {
	kinds := ix.kinds()
	var bad []result.Item
	for _, e := range ix.edges {
		key := e.Source + " → " + e.Target
		if !slices.Contains(kb.Relations, e.Relation) {
			bad = append(bad, result.Item{Check: index.CheckIndex, Key: key, Remedy: index.RefreshRemedy,
				Detail: fmt.Sprintf("the edge from %s to %s has relation %q; known relations: %v", e.Source, e.Target, e.Relation, kb.Relations)})
			continue
		}
		if _, text := e.Bearing.(string); e.Bearing != nil && !text {
			bad = append(bad, result.Item{Check: index.CheckIndex, Key: key, Remedy: index.RefreshRemedy,
				Detail: fmt.Sprintf("the %s edge from %s to %s has bearing %v, which is neither text nor null", e.Relation, e.Source, e.Target, e.Bearing)})
			continue
		}
		if !kb.Scoring(e.Relation) {
			continue
		}
		if kinds[e.Source] == "" {
			bad = append(bad, result.Item{Check: index.CheckIndex, Key: key, Remedy: index.RefreshRemedy,
				Detail: fmt.Sprintf("the %s edge from %s to %s has a source id the index does not hold", e.Relation, e.Source, e.Target)})
			continue
		}
		if !kb.BearingLegal(e.Relation, kinds[e.Source], e.TargetKind) || e.Bearing != nil {
			continue
		}
		bad = append(bad, result.Item{Check: index.CheckIndex, Key: key, Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("the %s edge from %s to %s carries no bearing, and its pairing has one", e.Relation, e.Source, e.Target)})
	}
	if bad != nil {
		return result.Refusal(bad)
	}
	return nil
}

// Nodes is the index's node records in the order Load leaves them: by id, kinds
// in kb.NodeKinds order where ids tie. A caller reading the graph reads them
// here rather than deriving the index a second time.
func (ix *Index) Nodes() []kb.NodeRow { return slices.Clone(ix.nodes) }

// Cites is the index's citation records in file order, which is the order a
// claim's declaring leaf is the first tier-2-marked one of.
func (ix *Index) Cites() []kb.CiteRow { return slices.Clone(ix.cites) }

// kinds is every node's kind by id.
func (ix *Index) kinds() map[string]string {
	out := map[string]string{}
	for _, n := range ix.nodes {
		out[n.ID] = n.NodeType
	}
	return out
}

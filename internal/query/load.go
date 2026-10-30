// Package query answers questions over a KB's derived index: the rows
// index.Derive yields from the KB's sources, and each query's payload shaped
// for its result document.
package query

import (
	"maps"
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
	// documentKinds is every document's kind, by kb-root-relative path.
	documentKinds map[string]string
	// hosts is the leaf hosting each node that carries a statement marker.
	hosts map[string]string
	// nodeManual is the manually-scored value of each node whose record
	// carries one, by id.
	nodeManual map[string]bool
}

// flagCounts is how many records are model-scored and how many manually
// scored.
type flagCounts struct{ model, manual int }

// Load is the derived index of the KB src reads, the rows index.Derive yields
// from its sources: Derive's refusals are Load's, and a degraded load is
// refused with a check: degraded item per fault it resolved. A KB loaded at an
// older format answers as converted.
func Load(src *kb.Source) (*Index, error) {
	rows, err := index.Derive(src)
	if err != nil {
		return nil, err
	}
	if items := rows.DegradedItems(); items != nil {
		return nil, result.Refusal(items)
	}
	return fromRows(src, rows), nil
}

// fromRows is the index of rows over src: an analysis's band is priced from
// its strength, and the nodes are sorted.
func fromRows(src *kb.Source, rows index.Rows) *Index {
	ix := &Index{src: src, nodes: rows.Nodes, edges: rows.Edges, strengthenBy: rows.StrengthenBy,
		cites: rows.Cites, aggregates: rows.Aggregates, documentKinds: rows.DocumentKinds, hosts: rows.Hosts}
	ix.nodeManual = nodeManualFlags(rows.Resolved)
	for i := range ix.nodes {
		if n := &ix.nodes[i]; n.NodeType == kb.NodeKindAnalysis {
			n.BuildBand = index.BuildBand(n.Strength)
		}
	}
	slices.SortStableFunc(ix.nodes, func(a, b kb.NodeRow) int {
		if a.ID != b.ID {
			return strings.Compare(a.ID, b.ID)
		}
		return slices.Index(kb.NodeKinds, a.NodeType) - slices.Index(kb.NodeKinds, b.NodeType)
	})
	return ix
}

// Nodes is the index's node records in the order Load leaves them: by id, kinds
// in kb.NodeKinds order where ids tie. A caller reading the graph reads them
// here rather than deriving the index a second time.
func (ix *Index) Nodes() []kb.NodeRow { return slices.Clone(ix.nodes) }

// Edges is the index's edge records in file order. A caller reading the graph
// reads them here rather than deriving the index a second time.
func (ix *Index) Edges() []kb.EdgeRow { return slices.Clone(ix.edges) }

// Hosts is the leaf hosting each claim, analysis and experiment that carries a
// statement marker, by id, as the load derived it.
func (ix *Index) Hosts() map[string]string { return maps.Clone(ix.hosts) }

// nodeManualFlags is the manually-scored value of each claim, analysis and
// experiment record of g, by id.
func nodeManualFlags(g kb.Graph) map[string]bool {
	nodes := map[string]bool{}
	for _, n := range g.Claims {
		nodes[n.ID] = n.ManuallyScored
	}
	for _, n := range g.Analyses {
		nodes[n.ID] = n.ManuallyScored
	}
	for _, n := range g.Experiments {
		nodes[n.ID] = n.ManuallyScored
	}
	return nodes
}

// add is c counting one more record, manually scored where manual.
func (c flagCounts) add(manual bool) flagCounts {
	if manual {
		c.manual++
	} else {
		c.model++
	}
	return c
}

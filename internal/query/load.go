// Package query answers questions over a KB's derived index: the
// .index/*.yaml files loaded record by record, and each query's payload
// shaped for its result document.
package query

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// requiredFiles are the index files a query loads.
var requiredFiles = []string{kb.IndexClaims, kb.IndexEdges, kb.IndexStrengthenBy, kb.IndexCites, kb.IndexSubtreeAggregates}

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

// Load reads the derived index of the KB src reads. A missing or unreadable
// file, a line that is not a record of the file's shape, a node of no known
// kind, and an index that would answer a legal edge with no bearing token are
// a result.Refusal naming each one. A KB loaded at an older format is refused
// once: its index is derived, and the refresh that rewrites the KB writes it.
func Load(src *kb.Source) (*Index, error) {
	if src.Migrated() {
		return nil, result.Refusal{{Check: index.CheckIndex, Path: src.KBPath(kb.EntryPointFile), Remedy: index.RefreshRemedy,
			Detail: fmt.Sprintf("the KB's metadata format is %s and kbase reads %s; the index is derived, and a refresh rewrites the KB in %s",
				src.Version(), kb.FormatVersion, kb.FormatVersion)}}
	}
	var missing []result.Item
	for _, name := range requiredFiles {
		if p := src.KBPath(kb.IndexDir + "/" + kb.IndexFileName(name)); !src.IsFile(p) {
			missing = append(missing, result.Item{Check: index.CheckIndex, Path: p, Remedy: index.RefreshRemedy, Detail: "index file missing: " + p})
		}
	}
	if missing != nil {
		return nil, result.Refusal(missing)
	}
	ix := &Index{src: src}
	var bad []result.Item
	ix.nodes = loadRows(src, &bad, func(n *kb.NodeRow) error {
		if !slices.Contains(kb.NodeKinds, n.NodeType) {
			return fmt.Errorf("unknown node_type %q on record id %q; known types: %v", n.NodeType, n.ID, kb.NodeKinds)
		}
		if n.NodeType == kb.NodeKindAnalysis {
			n.BuildBand = index.BuildBand(n.Strength)
		}
		return nil
	})
	ix.edges = loadRows[kb.EdgeRow](src, &bad, nil)
	ix.strengthenBy = loadRows[kb.StrengthenByRow](src, &bad, nil)
	ix.cites = loadRows[kb.CiteRow](src, &bad, nil)
	ix.aggregates = loadRows[kb.AggregateRow](src, &bad, nil)
	if bad != nil {
		return nil, result.Refusal(bad)
	}
	if err := ix.checkBearings(); err != nil {
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

// checkBearings refuses an index whose legal scoring edge carries no bearing
// token, and one whose scoring edge leaves its source kind unknown: the column
// is derived, so a hole in it is an index a refresh must rewrite, never a state
// to answer from.
func (ix *Index) checkBearings() error {
	kinds := ix.kinds()
	var bad []result.Item
	for _, e := range ix.edges {
		if !kb.Scoring(e.Relation) {
			continue
		}
		key := e.Source + " → " + e.Target
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
// here rather than reading the index files a second time.
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

// loadRows is T's rows, adding a refusal item to bad for the file where it
// does not read and for each line that is not a row.
func loadRows[T kb.IndexRowType](src *kb.Source, bad *[]result.Item, accept func(*T) error) []T {
	path, rows, problems, err := kb.ReadIndex(src, accept)
	if err != nil {
		*bad = append(*bad, result.Item{Check: index.CheckIndex, Path: path, Detail: fmt.Sprintf("index file unreadable: %v", err)})
		return nil
	}
	for _, p := range problems {
		*bad = append(*bad, result.Item{Check: index.CheckIndex, Path: path, Line: p.Line, Detail: p.Err.Error()})
	}
	return rows
}

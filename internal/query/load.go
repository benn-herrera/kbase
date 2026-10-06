// Package query answers kb_tools' kb_cmd questions over a KB's derived index:
// the .index/*.yaml files loaded record by record as kb_cmd loads its
// index, and each query's payload shaped as kb_cmd --json shapes it.
package query

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// requiredFiles are the index files a query loads; supported-by is not read.
var requiredFiles = []string{kb.IndexClaims, kb.IndexDependsOn, kb.IndexStrengthenBy, kb.IndexCites, kb.IndexSubtreeAggregates}

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
// file, a line that is not a record of the file's shape, and a node of no
// known kind are a result.Refusal naming each one.
func Load(src *kb.Source) (*Index, error) {
	var missing []result.Item
	for _, name := range requiredFiles {
		if p := src.KBPath(kb.IndexDir + "/" + kb.IndexFileName(name)); !src.IsFile(p) {
			missing = append(missing, result.Item{Check: checkIndex, Path: p, Remedy: index.RefreshRemedy, Detail: "index file missing: " + p})
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
		if n.NodeType == kb.NodeKindSupport {
			n.BuildBand = index.BuildBand(n.Solidity)
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
	slices.SortStableFunc(ix.nodes, func(a, b kb.NodeRow) int {
		if a.ID != b.ID {
			return strings.Compare(a.ID, b.ID)
		}
		return slices.Index(kb.NodeKinds, a.NodeType) - slices.Index(kb.NodeKinds, b.NodeType)
	})
	return ix, nil
}

// loadRows is T's rows, adding a refusal item to bad for the file where it
// does not read and for each line that is not a row.
func loadRows[T kb.IndexRowType](src *kb.Source, bad *[]result.Item, accept func(*T) error) []T {
	path, rows, problems, err := kb.ReadIndex(src, accept)
	if err != nil {
		*bad = append(*bad, result.Item{Check: checkIndex, Path: path, Detail: fmt.Sprintf("index file unreadable: %v", err)})
		return nil
	}
	for _, p := range problems {
		*bad = append(*bad, result.Item{Check: checkIndex, Path: path, Line: p.Line, Detail: p.Err.Error()})
	}
	return rows
}

// checkIndex is the refusal class of an index a query cannot load.
const checkIndex = "index"

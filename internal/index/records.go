package index

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// bearingValue is an edge's bearing column: the text BearingText returns where
// its relation allows a bearing, null where it allows none.
func bearingValue(e kb.Edge, sourceKind string) any {
	if !kb.BearingLegal(e.Relation, sourceKind, e.TargetKind) {
		return nil
	}
	return kb.BearingText(e.Bearing)
}

// edgeRows is edges as index rows, their sources' kinds read off kinds, by
// source, target and context.
func edgeRows(edges []kb.Edge, kinds map[string]string) []kb.EdgeRow {
	var out []kb.EdgeRow
	for _, e := range edges {
		out = append(out, kb.EdgeRow{Source: e.Source, Target: e.Target, Relation: e.Relation, TargetKind: e.TargetKind,
			Bearing: bearingValue(e, kinds[e.Source]), Context: e.Context, ManuallyScored: e.ManuallyScored})
	}
	context := func(r kb.EdgeRow) string {
		if r.Context == nil {
			return ""
		}
		return *r.Context
	}
	slices.SortStableFunc(out, func(a, b kb.EdgeRow) int {
		return cmp.Or(strings.Compare(a.Source, b.Source), strings.Compare(a.Target, b.Target), strings.Compare(context(a), context(b)))
	})
	return out
}

// isUnder is whether a document lies within the directory of an index node.
func isUnder(doc, indexPath string) bool {
	dir := path.Dir(indexPath)
	return dir == "." || strings.HasPrefix(doc, dir+"/")
}

func sortedSet(set map[string]bool) []string {
	out := []string{}
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

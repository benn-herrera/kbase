package query

import (
	"cmp"
	"fmt"
	"slices"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// Each *Payload function is one query's answer, keyed as kb_cmd --json keys
// it: a list, or one mapping for show and stats.

// ResultsKey holds a query's answer in its result document.
const ResultsKey = "results"

// DefaultLimit is how many results a list query returns when no limit is
// given.
const DefaultLimit = 50

// statsRankLimit is how many rows each ranked section of stats carries.
const statsRankLimit = 10

// Page is the window of a list answer from offset holding at most limit
// results, all of them where limit is 0; count is the whole answer's length
// and truncated whether results lie past the window.
func Page(list any, offset, limit int) (window any, count int, truncated bool) {
	switch x := list.(type) {
	case []string:
		w, t := pageOf(x, offset, limit)
		return w, len(x), t
	case []result.Record:
		w, t := pageOf(x, offset, limit)
		return w, len(x), t
	}
	panic(fmt.Sprintf("query: %T is no list answer", list))
}

func pageOf[T any](s []T, offset, limit int) ([]T, bool) {
	start := min(offset, len(s))
	end := len(s)
	if limit > 0 {
		end = min(start+limit, len(s))
	}
	return append([]T{}, s[start:end]...), end < len(s)
}

// aboveEveryScore is a solidity bar every scored claim clears: scores top
// out at the framework's 1.0.
const aboveEveryScore = 1.1

func opt(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func optString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func identity(n kb.NodeRow) result.Record {
	return result.Record{{Key: "node_type", Value: n.NodeType}, {Key: "id", Value: n.ID}, {Key: "title", Value: n.Title},
		{Key: "canonical_path", Value: n.CanonicalPath}, {Key: "canonical_anchor", Value: n.CanonicalAnchor}}
}

// nodeFields is each node kind's record, total over kb.NodeKinds.
var nodeFields = map[string]func(kb.NodeRow) result.Record{
	kb.NodeKindClaim: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Record{{Key: "confidence", Value: opt(n.Confidence)}, {Key: "solidity", Value: opt(n.Solidity)},
			{Key: "build_status", Value: optString(n.BuildStatus)}, {Key: "build_band", Value: n.BuildBand},
			{Key: "rationale", Value: n.Rationale}, {Key: "depends_on_count", Value: n.DependsOnCount},
			{Key: "strengthen_by_count", Value: n.StrengthenByCount}, {Key: "citation_count", Value: n.CitationCount}}...)
	},
	kb.NodeKindSupport: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Record{{Key: "quality", Value: opt(n.Quality)}, {Key: "solidity", Value: opt(n.Solidity)},
			{Key: "build_band", Value: n.BuildBand}}...)
	},
	kb.NodeKindExperiment: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Field{Key: "status", Value: n.Status})
	},
	kb.NodeKindInvariant: identity,
	kb.NodeKindAxiom:     identity,
	kb.NodeKindWork: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Field{Key: "strength", Value: opt(n.Strength)})
	},
}

func weakPointRecord(wp WeakPoint) result.Record {
	return result.Record{{Key: "id", Value: wp.Claim.ID}, {Key: "solidity", Value: opt(wp.Claim.Solidity)},
		{Key: "build_band", Value: wp.Claim.BuildBand}, {Key: "dependents", Value: wp.Dependents}, {Key: "title", Value: wp.Claim.Title}}
}

// DepsPayload is the edges sourced at id — relation, target, a rests-on
// edge's fraction as its applicability, and the row's context (null where it
// has none), keyed as kb_cmd --json keys them — or, inverse, the ids of
// every node with an edge to id.
func DepsPayload(ix *Index, id string, inverse bool) any {
	if inverse {
		return nonNil(ix.DependentsOf(id))
	}
	out := []result.Record{}
	for _, e := range ix.DependsOnEdges(id) {
		var applicability any
		if e.Relation == kb.RelationRestsOn {
			applicability = e.Fraction
		}
		out = append(out, result.Record{{Key: "relation", Value: e.Relation}, {Key: "target", Value: e.Target},
			{Key: "applicability", Value: applicability}, {Key: "context", Value: optString(e.Context)}})
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// GatedOnPayload is the claims whose strengthen-by items mention id.
func GatedOnPayload(ix *Index, id string) []string { return nonNil(ix.GatedOn(id)) }

// CitedByPayload is id's citations.
func CitedByPayload(ix *Index, id string) []result.Record {
	out := []result.Record{}
	for _, c := range ix.CitedBy(id) {
		out = append(out, result.Record{{Key: "claim_id", Value: c.ClaimID}, {Key: "leaf_path", Value: c.LeafPath},
			{Key: "leaf_kind", Value: c.LeafKind}, {Key: "tier2_marked", Value: c.Tier2Marked}})
	}
	return out
}

// FindPayload is the claims whose title or anchor contains query.
func FindPayload(ix *Index, query string) []result.Record {
	out := []result.Record{}
	for _, c := range ix.Find(query) {
		out = append(out, result.Record{{Key: "id", Value: c.ID}, {Key: "title", Value: c.Title},
			{Key: "solidity", Value: opt(c.Solidity)}, {Key: "build_band", Value: c.BuildBand}})
	}
	return out
}

// ReferencedByPayload is the leaves linking to the leaf that first cites id.
func ReferencedByPayload(ix *Index, id string) ([]string, error) {
	leaves, err := ix.ReferencedBy(id)
	return nonNil(leaves), err
}

// SolidityBelowPayload is the full record of each scored claim under
// threshold.
func SolidityBelowPayload(ix *Index, threshold float64) []result.Record {
	out := []result.Record{}
	for _, c := range ix.SolidityBelow(threshold) {
		out = append(out, nodeFields[kb.NodeKindClaim](c))
	}
	return out
}

// SubtreePayload is the claims under nodePath.
func SubtreePayload(ix *Index, nodePath string) []string {
	return nonNil(ix.SubtreeClaims(nodePath))
}

// ShowPayload is the record of the node carrying id, or false where none
// does. A claim's record carries, after strengthen_by_count, strengthen_by:
// its strengthen-by items (item_idx, text, mentioned_ids) in item_idx order,
// as kb_cmd --json returns them.
func ShowPayload(ix *Index, id string) (result.Record, bool) {
	n, ok := ix.Node(id)
	if !ok {
		return nil, false
	}
	rec := nodeFields[n.NodeType](n)
	if n.NodeType != kb.NodeKindClaim {
		return rec, true
	}
	var rows []kb.StrengthenByRow
	for _, r := range ix.strengthenBy {
		if r.ClaimID == id {
			rows = append(rows, r)
		}
	}
	slices.SortStableFunc(rows, func(a, b kb.StrengthenByRow) int { return cmp.Compare(a.ItemIdx, b.ItemIdx) })
	items := []result.Record{}
	for _, r := range rows {
		items = append(items, result.Record{{Key: "item_idx", Value: r.ItemIdx}, {Key: "text", Value: r.Text},
			{Key: "mentioned_ids", Value: nonNil(r.MentionedIDs)}})
	}
	at := slices.IndexFunc(rec, func(f result.Field) bool { return f.Key == "strengthen_by_count" }) + 1
	return slices.Insert(rec, at, result.Field{Key: "strengthen_by", Value: items}), true
}

// UnknownNode is show's refusal of an id no node carries.
func UnknownNode(id string) result.Item {
	return result.Item{Check: "unknown-id", Key: "<id>", Detail: fmt.Sprintf("unknown node id: %s", id)}
}

// WeakPointsPayload is the scored, load-bearing claims under maxSolidity.
func WeakPointsPayload(ix *Index, maxSolidity float64, minDependents int) []result.Record {
	out := []result.Record{}
	for _, wp := range ix.WeakPoints(maxSolidity, minDependents) {
		out = append(out, weakPointRecord(wp))
	}
	return out
}

// StatsPayload is the census — one count per node kind, then each index
// file's, the demoted edges beside the depends-on rows — the claims per
// build band, and the ten weak points and ten
// lowest-solidity claims.
func StatsPayload(ix *Index) result.Record {
	var out result.Record
	for _, kind := range kb.NodeKinds {
		n := 0
		for _, node := range ix.nodes {
			if node.NodeType == kind {
				n++
			}
		}
		out = append(out, result.Field{Key: kind + "s", Value: n})
	}
	demoted := 0
	for _, e := range ix.edges {
		if e.Relation == kb.RelationDemoted {
			demoted++
		}
	}
	out = append(out, result.Field{Key: "depends_on_edges", Value: len(ix.edges)}, result.Field{Key: "demoted_edges", Value: demoted},
		result.Field{Key: "strengthen_by_items", Value: len(ix.strengthenBy)},
		result.Field{Key: "citation_edges", Value: len(ix.cites)},
		result.Field{Key: "subtree_aggregates", Value: len(ix.aggregates)})

	bands := map[string]int{}
	for _, c := range ix.claims() {
		bands[c.BuildBand]++
	}
	var dist result.Record
	for _, b := range kb.BuildBandLadder {
		dist = append(dist, result.Field{Key: b.Label, Value: bands[b.Slug]})
	}
	dist = append(dist, result.Field{Key: kb.PendingLiteral, Value: bands[kb.UnknownBandSlug]})

	leverage := []result.Record{}
	for _, wp := range first(ix.WeakPoints(DefaultMaxSolidity, DefaultMinDependents), statsRankLimit) {
		leverage = append(leverage, weakPointRecord(wp))
	}
	weakest := []result.Record{}
	for _, c := range first(ix.SolidityBelow(aboveEveryScore), statsRankLimit) {
		weakest = append(weakest, result.Record{{Key: "id", Value: c.ID}, {Key: "solidity", Value: opt(c.Solidity)}, {Key: "title", Value: c.Title}})
	}
	return append(out, result.Field{Key: "solidity_bands", Value: dist}, result.Field{Key: "highest_leverage", Value: leverage},
		result.Field{Key: "weakest", Value: weakest})
}

func first[T any](s []T, n int) []T { return s[:min(n, len(s))] }

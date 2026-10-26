package query

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// Each *Payload function is one query's answer: a list, or one mapping for
// show, stats and edge-census.

// ResultsKey holds a query's answer in its result document.
const ResultsKey = "results"

// DefaultLimit is how many results a list query returns when no limit is
// given.
const DefaultLimit = 50

// statsRankLimit is how many rows each ranked section of stats carries.
const statsRankLimit = 10

// manuallyScoredKey is the key of a record's manually-scored value.
const manuallyScoredKey = "manually_scored"

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

// aboveEveryScore is a strength bar every scored claim clears: scores top
// out at the framework's 1.0.
const aboveEveryScore = 1.1

func opt(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// displayStrength is a strength as a query result shows it: the display
// spelling's value, or null where the node has none.
func displayStrength(v *float64) any {
	if v == nil {
		return nil
	}
	rounded, err := strconv.ParseFloat(index.FormatStrengthDisplay(v), 64)
	if err != nil {
		return *v
	}
	return rounded
}

func optString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func optBool(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

// identity is a node record's leading fields; a node with no host, whose row
// holds no canonical path, has neither path nor anchor, each null.
func identity(n kb.NodeRow) result.Record {
	var path, anchor any
	if n.CanonicalPath != "" {
		path, anchor = n.CanonicalPath, n.CanonicalAnchor
	}
	return result.Record{{Key: "node_type", Value: n.NodeType}, {Key: "id", Value: n.ID}, {Key: "title", Value: n.Title},
		{Key: "canonical_path", Value: path}, {Key: "canonical_anchor", Value: anchor}}
}

// nodeFields is each node kind's record, total over kb.NodeKinds.
var nodeFields = map[string]func(kb.NodeRow) result.Record{
	kb.NodeKindClaim: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Record{{Key: "rigor", Value: opt(n.Rigor)}, {Key: "strength", Value: displayStrength(n.Strength)},
			{Key: "build_status", Value: optString(n.BuildStatus)}, {Key: "build_band", Value: n.BuildBand},
			{Key: "rationale", Value: n.Rationale}, {Key: "premise_count", Value: n.PremiseCount},
			{Key: "strengthen_by_count", Value: n.StrengthenByCount}, {Key: "citation_count", Value: n.CitationCount}}...)
	},
	kb.NodeKindAnalysis: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Record{{Key: "rigor", Value: opt(n.Rigor)}, {Key: "strength", Value: displayStrength(n.Strength)},
			{Key: "build_band", Value: n.BuildBand}}...)
	},
	kb.NodeKindExperiment: func(n kb.NodeRow) result.Record {
		return append(identity(n), result.Field{Key: "status", Value: n.Status}, result.Field{Key: "rigor", Value: opt(n.Rigor)})
	},
	kb.NodeKindAxiom: rigorRecord,
	kb.NodeKindWork:  rigorRecord,
}

// rigorRecord is the record of a node whose strength is its rigor: identity
// fields and the row's rigor, no derived value beside them.
func rigorRecord(n kb.NodeRow) result.Record {
	return append(identity(n), result.Field{Key: "rigor", Value: opt(n.Rigor)})
}

func weakPointRecord(wp WeakPoint) result.Record {
	return result.Record{{Key: "id", Value: wp.Claim.ID}, {Key: "strength", Value: displayStrength(wp.Claim.Strength)},
		{Key: "build_band", Value: wp.Claim.BuildBand}, {Key: "dependents", Value: wp.Dependents}, {Key: "title", Value: wp.Claim.Title}}
}

// compareRelations orders two relations as the edge file's identity orders
// them.
func compareRelations(a, b string) int {
	return slices.Compare(kb.EdgeKey(a, "", "", ""), kb.EdgeKey(b, "", "", ""))
}

// DepsPayload is every edge incident at id — each scoring edge in either
// direction, and id's own outgoing references and demoted edges — as the
// relation, the direction it runs (in where id is the supported end, out
// where it is the supporter), the other end, its bearing as the index writes
// it or null where its pairing carries none, its manually-scored value or null
// where its record carries none, and its context. Rows are in relation order,
// then by the other end's id, the direction and the context. Inverse, it is
// the ids of every node id's strength enters.
func DepsPayload(ix *Index, id string, inverse bool) any {
	if inverse {
		return nonNil(ix.DependentsOf(id))
	}
	type row struct {
		edge             kb.EdgeRow
		direction, other string
	}
	var rows []row
	for _, e := range ix.edges {
		var other, direction string
		switch {
		case e.Source == id && (kb.Scoring(e.Relation) || e.Relation == kb.RelationReferences || e.Relation == kb.RelationDemoted):
			other, direction = e.Target, "out"
		case e.Target == id && kb.Scoring(e.Relation):
			other, direction = e.Source, "in"
		default:
			continue
		}
		rows = append(rows, row{edge: e, direction: direction, other: other})
	}
	slices.SortStableFunc(rows, func(a, b row) int {
		return cmp.Or(compareRelations(a.edge.Relation, b.edge.Relation), strings.Compare(a.other, b.other),
			strings.Compare(a.direction, b.direction), strings.Compare(deref(a.edge.Context), deref(b.edge.Context)))
	})
	out := []result.Record{}
	for _, r := range rows {
		out = append(out, result.Record{{Key: "relation", Value: r.edge.Relation}, {Key: "direction", Value: r.direction},
			{Key: "target", Value: r.other}, {Key: "bearing", Value: r.edge.Bearing}, {Key: manuallyScoredKey, Value: optBool(r.edge.ManuallyScored)},
			{Key: "context", Value: optString(r.edge.Context)}})
	}
	return out
}

// deref is *p, "" where p is nil.
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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
		out = append(out, result.Record{{Key: "claim_id", Value: c.ClaimID}, {Key: "leaf_path", Value: c.LeafPath}})
	}
	return out
}

// FindPayload is the claims whose title or anchor contains query.
func FindPayload(ix *Index, query string) []result.Record {
	out := []result.Record{}
	for _, c := range ix.Find(query) {
		out = append(out, result.Record{{Key: "id", Value: c.ID}, {Key: "title", Value: c.Title},
			{Key: "strength", Value: displayStrength(c.Strength)}, {Key: "build_band", Value: c.BuildBand}})
	}
	return out
}

// ReferencedByPayload is the leaves linking to the leaf that first cites id.
func ReferencedByPayload(ix *Index, id string) ([]string, error) {
	leaves, err := ix.ReferencedBy(id)
	return nonNil(leaves), err
}

// StrengthBelowPayload is the full record of each scored claim under
// threshold.
func StrengthBelowPayload(ix *Index, threshold float64) []result.Record {
	out := []result.Record{}
	for _, c := range ix.StrengthBelow(threshold) {
		out = append(out, nodeFields[kb.NodeKindClaim](c))
	}
	return out
}

// SubtreePayload is the claims under nodePath.
func SubtreePayload(ix *Index, nodePath string) []string {
	return nonNil(ix.SubtreeClaims(nodePath))
}

// ShowPayload is the record of the node carrying id, or false where none
// does. A node whose record carries a manually-scored value carries it after
// rigor. A claim's record carries, after strengthen_by_count, strengthen_by:
// its strengthen-by items (item_idx, text, mentioned_ids) in item_idx order.
func ShowPayload(ix *Index, id string) (result.Record, bool) {
	n, ok := ix.Node(id)
	if !ok {
		return nil, false
	}
	rec := nodeFields[n.NodeType](n)
	if manual, carried := ix.nodeManual[id]; carried {
		rec = insertAfter(rec, "rigor", result.Field{Key: manuallyScoredKey, Value: manual})
	}
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
	return insertAfter(rec, "strengthen_by_count", result.Field{Key: "strengthen_by", Value: items}), true
}

// insertAfter is rec with f inserted after its field key.
func insertAfter(rec result.Record, key string, f result.Field) result.Record {
	at := slices.IndexFunc(rec, func(field result.Field) bool { return field.Key == key }) + 1
	return slices.Insert(rec, at, f)
}

// UnknownNode is show's refusal of an id no node carries.
func UnknownNode(id string) result.Item {
	return result.Item{Check: kb.CheckUnknownID, Key: "<id>", Detail: fmt.Sprintf("unknown node id: %s", id)}
}

// WeakPointsPayload is the scored, load-bearing claims under maxStrength.
func WeakPointsPayload(ix *Index, maxStrength float64, minDependents int) []result.Record {
	out := []result.Record{}
	for _, wp := range ix.WeakPoints(maxStrength, minDependents) {
		out = append(out, weakPointRecord(wp))
	}
	return out
}

// kindCountKey is a node kind's census key: its plural, analyses for analysis.
func kindCountKey(kind string) string {
	if kind == kb.NodeKindAnalysis {
		return "analyses"
	}
	return kind + "s"
}

// StatsPayload is the census — one count per node kind, then each index
// file's, the demoted edges beside the edge rows — the claims per build band,
// and the ten weak points and ten lowest-strength claims.
func StatsPayload(ix *Index) result.Record {
	var out result.Record
	for _, kind := range kb.NodeKinds {
		n := 0
		for _, node := range ix.nodes {
			if node.NodeType == kind {
				n++
			}
		}
		out = append(out, result.Field{Key: kindCountKey(kind), Value: n})
	}
	demoted := 0
	for _, e := range ix.edges {
		if e.Relation == kb.RelationDemoted {
			demoted++
		}
	}
	out = append(out, result.Field{Key: "edges", Value: len(ix.edges)}, result.Field{Key: "demoted_edges", Value: demoted},
		result.Field{Key: "strengthen_by_items", Value: len(ix.strengthenBy)},
		result.Field{Key: "citation_edges", Value: len(ix.cites)},
		result.Field{Key: "subtree_aggregates", Value: len(ix.aggregates)})
	var rigor, bearing flagCounts
	for _, manual := range ix.nodeManual {
		rigor = rigor.add(manual)
	}
	for _, e := range ix.edges {
		if e.ManuallyScored != nil {
			bearing = bearing.add(*e.ManuallyScored)
		}
	}
	out = append(out, result.Field{Key: "rigor_manual", Value: rigor.manual}, result.Field{Key: "rigor_model", Value: rigor.model},
		result.Field{Key: "bearing_manual", Value: bearing.manual}, result.Field{Key: "bearing_model", Value: bearing.model})

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
	for _, wp := range first(ix.WeakPoints(DefaultMaxStrength, DefaultMinDependents), statsRankLimit) {
		leverage = append(leverage, weakPointRecord(wp))
	}
	weakest := []result.Record{}
	for _, c := range first(ix.StrengthBelow(aboveEveryScore), statsRankLimit) {
		weakest = append(weakest, result.Record{{Key: "id", Value: c.ID}, {Key: "strength", Value: displayStrength(c.Strength)}, {Key: "title", Value: c.Title}})
	}
	return append(out, result.Field{Key: "strength_bands", Value: dist}, result.Field{Key: "highest_leverage", Value: leverage},
		result.Field{Key: "weakest", Value: weakest})
}

func first[T any](s []T, n int) []T { return s[:min(n, len(s))] }

// EdgeCensusPayload is, for each relation in kb.Relations order, its edge
// rows' count, whether the bearing pass walks it, and on a scoring relation
// the rows at each bearing they carry — numbers ascending, then pending, then
// null — each with, where the relation's records carry a manually-scored
// value, how many are manually scored and how many model-scored.
func EdgeCensusPayload(ix *Index) result.Record {
	out := result.Record{}
	for _, relation := range kb.Relations {
		count := 0
		atBearing := map[any]int{}
		flagged := map[any]flagCounts{}
		carried := false
		for _, e := range ix.edges {
			if e.Relation != relation {
				continue
			}
			count++
			atBearing[e.Bearing]++
			if e.ManuallyScored != nil {
				carried = true
				flagged[e.Bearing] = flagged[e.Bearing].add(*e.ManuallyScored)
			}
		}
		rec := result.Record{{Key: "edges", Value: count}, {Key: "walked", Value: kb.BearingWalk(relation)}}
		if kb.Scoring(relation) {
			bearings := slices.SortedFunc(maps.Keys(atBearing), compareBearings)
			records := []result.Record{}
			for _, b := range bearings {
				r := result.Record{{Key: "bearing", Value: b}, {Key: "edges", Value: atBearing[b]}}
				if carried {
					r = append(r, result.Field{Key: "manual", Value: flagged[b].manual}, result.Field{Key: "model", Value: flagged[b].model})
				}
				records = append(records, r)
			}
			rec = append(rec, result.Field{Key: "bearings", Value: records})
		}
		out = append(out, result.Field{Key: relation, Value: rec})
	}
	return out
}

// compareBearings orders index bearings, text or null, numerically where the
// text reads as a bearing's number — the text's own order is not numeric —
// then pending, then null. Distinct texts that tie fall back to text order.
func compareBearings(a, b any) int {
	rank := func(v any) (int, float64, string) {
		s, text := v.(string)
		if !text {
			return 2, 0, ""
		}
		if bearing, ok := kb.BearingFromValue(s); ok && !bearing.Pending {
			return 0, bearing.Value, s
		}
		return 1, 0, s
	}
	ra, na, sa := rank(a)
	rb, nb, sb := rank(b)
	return cmp.Or(cmp.Compare(ra, rb), cmp.Compare(na, nb), strings.Compare(sa, sb))
}

// EdgesPayload is the index's edge rows of relations, of every relation where
// relations is empty, each as its source, relation, target, bearing as the
// index writes it, manually-scored value or null where its record carries
// none, context and whether the bearing pass walks it. Rows are in relation
// order, then by source, target and context.
func EdgesPayload(ix *Index, relations []string) []result.Record {
	rows := slices.DeleteFunc(slices.Clone(ix.edges), func(e kb.EdgeRow) bool {
		return len(relations) > 0 && !slices.Contains(relations, e.Relation)
	})
	slices.SortStableFunc(rows, func(a, b kb.EdgeRow) int {
		return cmp.Or(slices.Compare(kb.EdgeKey(a.Relation, "", a.Source, a.Target), kb.EdgeKey(b.Relation, "", b.Source, b.Target)),
			strings.Compare(deref(a.Context), deref(b.Context)))
	})
	out := []result.Record{}
	for _, e := range rows {
		out = append(out, result.Record{{Key: "source", Value: e.Source}, {Key: "relation", Value: e.Relation},
			{Key: "target", Value: e.Target}, {Key: "bearing", Value: e.Bearing}, {Key: manuallyScoredKey, Value: optBool(e.ManuallyScored)},
			{Key: "context", Value: optString(e.Context)}, {Key: "walked", Value: kb.BearingWalk(e.Relation)}})
	}
	return out
}

package query

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// Each *Payload function is one query's answer: a list, or one mapping for
// show and stats. KB text a row carries — a title, a rationale, a context, a
// leaf's line — is a result.Verbatim, written as it stands on every surface.

// ResultsKey holds a query's answer in its result.
const ResultsKey = "results"

// StatsRankLimit is how many rows each ranked section of stats carries.
const StatsRankLimit = 10

// pendingOrValue is a value as it stands, or *pending* where it is pending.
func pendingOrValue(v *float64) any {
	if v == nil {
		return kb.PendingLiteral
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

// optVerbatim is KB text a result carries as it stands (result.Verbatim), or
// null where there is none.
func optVerbatim(s *string) any {
	if s == nil {
		return nil
	}
	return result.Verbatim(*s)
}

func optBool(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

// orNull is s, or null where s is "".
func orNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// identity is a node record's leading fields: its host leaf and the anchor of
// the section stating it, each null where it has no statement marker, and its
// summary-page entry as <page>#<anchor>, null where no page renders it.
func identity(ix *Index, n kb.NodeRow) result.Record {
	var summary any
	if page, ok := ix.registerPages[n.ID]; ok {
		summary = page + "#" + ix.registerAnchors[n.ID]
	}
	host := ix.hosts[n.ID]
	var hostAnchor any
	if host != "" {
		hostAnchor = ix.hostAnchors[n.ID]
	}
	return result.Record{{Key: "kind", Value: n.NodeType}, {Key: "id", Value: n.ID}, {Key: "title", Value: result.Verbatim(n.Title)},
		{Key: "host", Value: orNull(host)}, {Key: "host-anchor", Value: hostAnchor}, {Key: "summary", Value: summary}}
}

// nodeFields is each node kind's record, total over kb.NodeKinds.
var nodeFields = map[string]func(*Index, kb.NodeRow) result.Record{
	kb.NodeKindClaim: func(ix *Index, n kb.NodeRow) result.Record {
		return append(identity(ix, n), result.Record{{Key: "rigor", Value: pendingOrValue(n.Rigor)}, {Key: "strength", Value: pendingOrDisplay(n.Strength)},
			{Key: "build-status", Value: optString(n.BuildStatus)}, {Key: "strength-band", Value: n.StrengthBand},
			{Key: "rationale", Value: result.Verbatim(ix.rationales[n.ID])}, {Key: "premises", Value: n.PremiseCount}}...)
	},
	kb.NodeKindAnalysis: func(ix *Index, n kb.NodeRow) result.Record {
		return append(identity(ix, n), result.Record{{Key: "rigor", Value: pendingOrValue(n.Rigor)}, {Key: "strength", Value: pendingOrDisplay(n.Strength)},
			{Key: "strength-band", Value: n.StrengthBand}, {Key: "rationale", Value: result.Verbatim(ix.rationales[n.ID])}}...)
	},
	kb.NodeKindExperiment: func(ix *Index, n kb.NodeRow) result.Record {
		return append(identity(ix, n), result.Field{Key: "status", Value: n.Status}, result.Field{Key: "rigor", Value: pendingOrValue(n.Rigor)})
	},
	kb.NodeKindAxiom: func(ix *Index, n kb.NodeRow) result.Record {
		return append(identity(ix, n), result.Field{Key: "label", Value: nodeLabel(n)}, result.Field{Key: "rigor", Value: pendingOrValue(n.Rigor)})
	},
	kb.NodeKindWork: func(ix *Index, n kb.NodeRow) result.Record {
		return append(identity(ix, n), result.Field{Key: "label", Value: nodeLabel(n)}, result.Field{Key: "rigor", Value: pendingOrValue(n.Rigor)},
			result.Field{Key: "rationale", Value: result.Verbatim(ix.rationales[n.ID])})
	},
}

func weakPointRecord(n FoundNode) result.Record {
	return result.Record{{Key: "id", Value: n.Node.ID}, {Key: "strength", Value: displayStrength(n.Strength)},
		{Key: "strength-band", Value: n.Node.StrengthBand}, {Key: "dependents", Value: n.Dependents}, {Key: "title", Value: result.Verbatim(n.Node.Title)}}
}

// compareRelations orders two relations as the edge file's identity orders
// them.
func compareRelations(a, b string) int {
	return slices.Compare(kb.EdgeKey(a, "", "", ""), kb.EdgeKey(b, "", "", ""))
}

// CitationsOfPayload is id's citations, each its path, line and text, or
// false where no node carries id.
func CitationsOfPayload(ix *Index, id string) ([]result.Record, bool, error) {
	if _, ok := ix.Node(id); !ok {
		return nil, false, nil
	}
	cites, err := ix.CitationsOf(id)
	if err != nil {
		return nil, true, err
	}
	out := []result.Record{}
	for _, c := range cites {
		out = append(out, result.Record{{Key: "path", Value: c.Path}, {Key: "line", Value: c.Line}, {Key: "text", Value: result.Verbatim(c.Text)}})
	}
	return out, true, nil
}

// NoNode is the refusal of an id no node carries, named at key.
func NoNode(key, id string) result.Item {
	detail, remedy := kb.NoNode(key, id)
	return result.Item{Check: kb.CheckUnknownID, Key: key, Detail: detail, Remedy: remedy}
}

// FindPayload is a row per node sel selects (Find): its id, kind, title, label
// (null for a hosted kind), host leaf (null where it has none), strength
// (*pending* where pending) and dependents' count. It refuses, as usage, what
// FindUsage refuses.
func FindPayload(ix *Index, sel FindSelection) ([]result.Record, []result.Item) {
	if usage := FindUsage(sel); usage != nil {
		return nil, usage
	}
	out := []result.Record{}
	for _, n := range ix.Find(sel) {
		out = append(out, result.Record{{Key: "id", Value: n.Node.ID}, {Key: "kind", Value: n.Node.NodeType}, {Key: "title", Value: result.Verbatim(n.Node.Title)},
			{Key: "label", Value: nodeLabel(n.Node)}, {Key: "host", Value: orNull(n.Host)}, {Key: "strength", Value: pendingOrDisplay(n.Strength)},
			{Key: "dependents", Value: n.Dependents}})
	}
	return out, nil
}

// nodeLabel is a work's or an axiom's label, null for a node of a hosted kind.
func nodeLabel(n kb.NodeRow) any {
	if n.Label == "" {
		return nil
	}
	return result.Verbatim(n.Label)
}

// FindUsage is the usage refusals of sel: a kind, a sort or an order outside
// its vocabulary, Pending beside a bound, and a limit beside no sort or of 0.
func FindUsage(sel FindSelection) []result.Item {
	var usage []result.Item
	for _, k := range sel.Kinds {
		if !slices.Contains(kb.NodeKinds, k) {
			usage = append(usage, result.Item{Check: result.CheckUsage, Key: argmark.Flag("kind"), Allowed: kb.NodeKinds,
				Detail: argmark.FlagValue("kind", strconv.Quote(k)) + " names no node kind"})
		}
	}
	if sel.Pending && (sel.Min != nil || sel.Max != nil) {
		pending := argmark.Flag("pending")
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: pending,
			Detail: pending + " lists the nodes with no strength, which " + argmark.Flag("min") + " and " + argmark.Flag("max") +
				" cannot bound: drop " + pending + ", or drop both bounds"})
	}
	return append(usage, rankUsage(sel.Sort, FindSorts, sel.Order, sel.Limit)...)
}

// rankUsage is the usage refusals of a sort, an order and a limit: a sort
// outside sorts, an order outside Orders, a limit beside no sort, and a limit
// of 0.
func rankUsage(sort *string, sorts []string, order string, limit *int) []result.Item {
	var usage []result.Item
	sortFlag, orderFlag, limitFlag := argmark.Flag("sort"), argmark.Flag("order"), argmark.Flag("limit")
	if sort != nil && !slices.Contains(sorts, *sort) {
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: sortFlag, Allowed: sorts,
			Detail: argmark.FlagValue("sort", strconv.Quote(*sort)) + " names no sort key"})
	}
	if !slices.Contains(Orders, order) {
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: orderFlag, Allowed: Orders,
			Detail: argmark.FlagValue("order", strconv.Quote(order)) + " names no order"})
	}
	switch {
	case limit != nil && sort == nil:
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: limitFlag,
			Detail: limitFlag + " keeps the first or last N of an explicit " + sortFlag + ", and none is given: add " + sortFlag + ", one of " + strings.Join(sorts, ", ")})
	case limit != nil && *limit == 0:
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: limitFlag,
			Detail: argmark.FlagValue("limit", "0") + " keeps nothing: a positive N keeps the first N of the sort, a negative N the last N"})
	}
	return usage
}

// ShowPayload is the record of the node carrying id, or false where none
// does. A node whose record carries a manually-scored value carries it after
// rigor.
func ShowPayload(ix *Index, id string) (result.Record, bool) {
	n, ok := ix.Node(id)
	if !ok {
		return nil, false
	}
	rec := nodeFields[n.NodeType](ix, n)
	if manual, carried := ix.nodeManual[id]; carried {
		rec = insertAfter(rec, "rigor", result.Field{Key: kb.ManuallyScoredKey, Value: manual})
	}
	return rec, true
}

// insertAfter is rec with f inserted after its field key.
func insertAfter(rec result.Record, key string, f result.Field) result.Record {
	at := slices.IndexFunc(rec, func(field result.Field) bool { return field.Key == key }) + 1
	return slices.Insert(rec, at, f)
}

// kindCountKey is a node kind's census key: its plural, analyses for analysis.
func kindCountKey(kind string) string {
	if kind == kb.NodeKindAnalysis {
		return "analyses"
	}
	return kind + "s"
}

// StatsPayload is the census — one count per node kind, then the edge rows,
// the demoted edges among them and the subtree aggregates, the manual and
// model counts and each relation's edges (relationCensus) — the claims per
// strength band, and the heads of the HighestLeverage and Weakest lists.
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
	out = append(out, result.Field{Key: "edges", Value: len(ix.edges)}, result.Field{Key: "demoted-edges", Value: demoted},
		result.Field{Key: "subtree-aggregates", Value: volumeAggregates(ix)})
	var rigor, bearing flagCounts
	for _, manual := range ix.nodeManual {
		rigor = rigor.add(manual)
	}
	for _, e := range ix.edges {
		if e.ManuallyScored != nil {
			bearing = bearing.add(*e.ManuallyScored)
		}
	}
	out = append(out, result.Field{Key: "rigor-manual", Value: rigor.manual}, result.Field{Key: "rigor-model", Value: rigor.model},
		result.Field{Key: "bearing-manual", Value: bearing.manual}, result.Field{Key: "bearing-model", Value: bearing.model},
		result.Field{Key: "relations", Value: relationCensus(ix)})

	bands := map[string]int{}
	for _, c := range ix.claims() {
		bands[c.StrengthBand]++
	}
	var dist result.Record
	for _, b := range kb.StrengthBandLadder {
		dist = append(dist, result.Field{Key: b.Slug, Value: bands[b.Slug]})
	}
	dist = append(dist, result.Field{Key: kb.UnknownBandSlug, Value: bands[kb.UnknownBandSlug]})

	leverage := []result.Record{}
	for _, n := range ix.Find(HighestLeverage()) {
		leverage = append(leverage, weakPointRecord(n))
	}
	weakest := []result.Record{}
	for _, n := range ix.Find(Weakest()) {
		weakest = append(weakest, result.Record{{Key: "id", Value: n.Node.ID}, {Key: "strength", Value: displayStrength(n.Strength)},
			{Key: "title", Value: result.Verbatim(n.Node.Title)}})
	}
	return append(out, result.Field{Key: "strength-bands", Value: dist}, result.Field{Key: "highest-leverage", Value: leverage},
		result.Field{Key: "weakest", Value: weakest})
}

// volumeAggregates is one record per volume, in path order: the claims and
// the experiments whose statement marker stands in a leaf under it.
func volumeAggregates(ix *Index) result.Record {
	out := result.Record{}
	for _, v := range slices.Sorted(maps.Keys(kb.Volumes(slices.Collect(maps.Keys(ix.documentKinds))))) {
		claims := map[string]bool{}
		for _, c := range ix.cites {
			if hostedUnder(c.LeafPath, v) {
				claims[c.ClaimID] = true
			}
		}
		experiments := 0
		for _, n := range ix.nodes {
			if n.NodeType == kb.NodeKindExperiment && hostedUnder(ix.hosts[n.ID], v) {
				experiments++
			}
		}
		out = append(out, result.Field{Key: v, Value: result.Record{{Key: "claims", Value: len(claims)}, {Key: "experiments", Value: experiments}}})
	}
	return out
}

// HighestLeverage is the find question stats' highest-leverage previews: the
// claims below DefaultMaxStrength with at least DefaultMinDependents
// dependents, the most dependents first; its first StatsRankLimit.
func HighestLeverage() FindSelection {
	bar, sort, limit := DefaultMaxStrength, SortDependents, StatsRankLimit
	return FindSelection{Kinds: []string{kb.NodeKindClaim}, Max: &bar, MinDependents: DefaultMinDependents,
		Sort: &sort, Order: OrderDescending, Limit: &limit}
}

// Weakest is the find question stats' weakest previews: the scored claims,
// lowest strength first; its first StatsRankLimit.
func Weakest() FindSelection {
	floor, sort, limit := 0.0, SortStrength, StatsRankLimit
	return FindSelection{Kinds: []string{kb.NodeKindClaim}, Min: &floor, Sort: &sort, Order: OrderAscending, Limit: &limit}
}

// relationCensus is, for each relation in kb.Relations order, its edge rows'
// count, whether the bearing pass walks it, and on a scoring relation the rows
// at each bearing they carry — numbers ascending, then pending, then null.
func relationCensus(ix *Index) result.Record {
	out := result.Record{}
	for _, relation := range kb.Relations {
		count := 0
		atBearing := map[any]int{}
		for _, e := range ix.edges {
			if e.Relation == relation {
				count++
				atBearing[e.Bearing]++
			}
		}
		rec := result.Record{{Key: "edges", Value: count}, {Key: "walked", Value: kb.BearingWalk(relation)}}
		if kb.Scoring(relation) {
			records := []result.Record{}
			for _, b := range slices.SortedFunc(maps.Keys(atBearing), compareBearings) {
				records = append(records, result.Record{{Key: "bearing", Value: b}, {Key: "edges", Value: atBearing[b]}})
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

// The directions edges and trace read from their node: in where the node is
// the edge's target, out where it is the source, both for either.
const (
	DirectionIn   = "in"
	DirectionOut  = "out"
	DirectionBoth = "both"
)

// EdgeDirections is the directions edges reads.
var EdgeDirections = []string{DirectionIn, DirectionOut, DirectionBoth}

// EdgesSelection is edges' question: the edge rows of Relations, of every
// relation where it is empty, and where ID is not nil those at the node it
// names in Direction, "" where none is given, which reads both.
type EdgesSelection struct {
	ID        *string
	Direction string
	Relations []string
}

// EdgesPayload is the edge rows sel selects (EdgeRowRecord), in relation
// order, then by source and target, a demoted record's replaced relation
// last. It refuses, as usage, a relation or a direction outside its
// vocabulary, and a direction beside no node; then an id no node carries.
func EdgesPayload(ix *Index, sel EdgesSelection) ([]result.Record, []result.Item) {
	if usage := edgesUsage(sel); usage != nil {
		return nil, usage
	}
	if sel.ID != nil {
		if _, ok := ix.Node(*sel.ID); !ok {
			return nil, []result.Item{NoNode(argmark.Flag("id"), *sel.ID)}
		}
	}
	rows := slices.DeleteFunc(slices.Clone(ix.edges), func(e kb.EdgeRow) bool {
		if len(sel.Relations) > 0 && !slices.Contains(sel.Relations, e.Relation) {
			return true
		}
		if sel.ID == nil {
			return false
		}
		in, out := e.Target == *sel.ID, e.Source == *sel.ID
		switch sel.Direction {
		case DirectionIn:
			return !in
		case DirectionOut:
			return !out
		}
		return !in && !out
	})
	slices.SortStableFunc(rows, compareEdgeRows)
	out := []result.Record{}
	for _, e := range rows {
		out = append(out, EdgeRowRecord(e))
	}
	return out, nil
}

// edgesUsage is the usage refusals of sel: a relation or a direction outside
// its vocabulary, and a direction beside no node.
func edgesUsage(sel EdgesSelection) []result.Item {
	var unknown []string
	var usage []result.Item
	for _, r := range sel.Relations {
		if !slices.Contains(kb.Relations, r) && !slices.Contains(unknown, r) {
			unknown = append(unknown, r)
			usage = append(usage, result.Item{Check: result.CheckUsage, Key: argmark.Flag("relation"), Allowed: kb.Relations,
				Detail: argmark.FlagValue("relation", strconv.Quote(r)) + " names no relation"})
		}
	}
	direction, id := argmark.Flag("direction"), argmark.Flag("id")
	switch {
	case sel.Direction != "" && !slices.Contains(EdgeDirections, sel.Direction):
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: direction, Allowed: EdgeDirections,
			Detail: argmark.FlagValue("direction", strconv.Quote(sel.Direction)) + " names no direction"})
	case sel.Direction != "" && sel.ID == nil:
		usage = append(usage, result.Item{Check: result.CheckUsage, Key: direction,
			Detail: direction + " reads the edges at the node " + id + " names, and none is given: add " + id + ", or drop " + direction})
	}
	return usage
}

// compareEdgeRows orders edge rows by relation, then source, then target, a
// demoted record's replaced relation last: edges' fixed order.
func compareEdgeRows(a, b kb.EdgeRow) int {
	return cmp.Or(compareRelations(a.Relation, b.Relation), strings.Compare(a.Source, b.Source), strings.Compare(a.Target, b.Target),
		compareRelations(a.ReplacedRelation, b.ReplacedRelation))
}

// EdgeRowRecord is one edge row as edges answers it: its source, target and
// relation, its bearing as the index writes it or null where the relation
// carries none, its context or null for none, its manually-scored value or
// null where its record carries none, and the relation a demoted record
// replaced, null on every other.
func EdgeRowRecord(e kb.EdgeRow) result.Record {
	return result.Record{{Key: "source", Value: e.Source}, {Key: "target", Value: e.Target}, {Key: "relation", Value: e.Relation},
		{Key: "bearing", Value: e.Bearing}, {Key: "context", Value: optVerbatim(e.Context)},
		{Key: kb.ManuallyScoredKey, Value: optBool(e.ManuallyScored)}, {Key: "replaced-relation", Value: orNull(e.ReplacedRelation)}}
}

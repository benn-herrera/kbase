package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// edgeOpsGraph holds two nodes of each kind, so every pairing of kinds has a
// source and a distinct target.
func edgeOpsGraph() kb.Graph {
	return kb.Graph{
		Claims:      []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "A"}, {ID: "clm-bbbbbb", Title: "B"}},
		Analyses:    []kb.AnalysisNode{{ID: "ana-aaaaaa", Title: "An A"}, {ID: "ana-bbbbbb", Title: "An B"}},
		Experiments: []kb.ExperimentNode{{ID: "exp-aaaaaa", Title: "Ex A", Status: kb.ExperimentRun}, {ID: "exp-bbbbbb", Title: "Ex B", Status: kb.ExperimentRun}},
		Works:       []kb.WorkNode{{ID: "wrk-s00000", Label: "Smith2020", Title: "W A"}, {ID: "wrk-j00000", Label: "Jones2021", Title: "W B"}},
		Axioms:      []kb.AxiomNode{{ID: "axm-s10000", Label: "S1", Title: "Ax A"}, {ID: "axm-s20000", Label: "S2", Title: "Ax B"}},
	}
}

// edgeOpsNodes is edgeOpsGraph's source and target of each kind.
var edgeOpsNodes = map[string][2]string{
	kb.NodeKindClaim:      {"clm-aaaaaa", "clm-bbbbbb"},
	kb.NodeKindAnalysis:   {"ana-aaaaaa", "ana-bbbbbb"},
	kb.NodeKindExperiment: {"exp-aaaaaa", "exp-bbbbbb"},
	kb.NodeKindWork:       {"wrk-s00000", "wrk-j00000"},
	kb.NodeKindAxiom:      {"axm-s10000", "axm-s20000"},
}

// joins is page §1.2's relations table: the scoring relations a source of
// each kind carries into a claim or an analysis. references joins any two
// kinds.
var joins = map[string][]string{
	kb.NodeKindClaim:      {kb.RelationSupports, kb.RelationRefutes},
	kb.NodeKindAnalysis:   {kb.RelationLifts, kb.RelationRefutes},
	kb.NodeKindExperiment: {kb.RelationSupports, kb.RelationLifts, kb.RelationRefutes},
	kb.NodeKindWork:       {kb.RelationFounds},
	kb.NodeKindAxiom:      {kb.RelationFounds},
}

// joining is the relations joining a source of sourceKind to a target of
// targetKind, in the relations table's order.
func joining(sourceKind, targetKind string) []string {
	var out []string
	for _, r := range []string{kb.RelationSupports, kb.RelationFounds, kb.RelationLifts, kb.RelationRefutes, kb.RelationReferences} {
		scored := (targetKind == kb.NodeKindClaim || targetKind == kb.NodeKindAnalysis) && slices.Contains(joins[sourceKind], r)
		if scored || r == kb.RelationReferences {
			out = append(out, r)
		}
	}
	return out
}

// edgeEntry is one mixed-batch entry of op naming an edge, extra its further
// keys, each a "key: value" line.
func edgeEntry(op, source, target, relation string, extra ...string) string {
	s := "- operation: " + op + "\n  source: " + source + "\n  target: " + target + "\n  relation: " + relation + "\n"
	for _, x := range extra {
		s += "  " + x + "\n"
	}
	return s
}

// runEdges runs the mixed batch of entries over root, as a caller's.
func runEdges(t *testing.T, root string, entries ...string) Result {
	t.Helper()
	return runRecordOp(t, root, Batch, "entry:\n"+strings.Join(entries, ""), ProvenanceCaller)
}

// edgeRecord is the record g holds of relation from source to target, nil
// where none does.
func edgeRecord(g kb.Graph, relation, source, target string) *kb.EdgeRecord {
	for _, r := range g.Edges() {
		if r.Relation == relation && r.Source == source && r.Target == target {
			return &r
		}
	}
	return nil
}

// TestAddEdgeJoinsWhatItsRelationJoins: add-edge writes each relation between
// every pairing of kinds it joins, reporting source, target and relation, a
// left-out bearing pending and unmarked; every other pairing is refused at
// relation, allowed the relations that join the two kinds.
func TestAddEdgeJoinsWhatItsRelationJoins(t *testing.T) {
	for _, sourceKind := range kb.NodeKinds {
		for _, targetKind := range kb.NodeKinds {
			allowed := joining(sourceKind, targetKind)
			for _, relation := range []string{kb.RelationSupports, kb.RelationFounds, kb.RelationLifts, kb.RelationRefutes, kb.RelationReferences} {
				source, target := edgeOpsNodes[sourceKind][0], edgeOpsNodes[targetKind][1]
				root := recordKB(t, edgeOpsGraph())
				res := runEdges(t, root, edgeEntry("add-edge", source, target, relation))
				if !slices.Contains(allowed, relation) {
					if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != "relation" ||
						!slices.Equal(res.Refusals[0].Allowed, allowed) {
						t.Errorf("%s %s → %s: %+v, want refused at relation allowing %v", relation, source, target, res, allowed)
					}
					continue
				}
				if res.Outcome != result.Done || len(res.Entries) != 1 {
					t.Errorf("%s %s → %s: %+v, want done", relation, source, target, res)
					continue
				}
				if e := res.Entries[0]; e.Source != source || e.Target != target || e.Relation != relation {
					t.Errorf("%s %s → %s: entry %+v, want its source, target and relation", relation, source, target, e)
				}
				r := edgeRecord(records(t, root), relation, source, target)
				if r == nil || r.ManuallyScored || (r.Bearing != nil) != kb.Scoring(relation) || r.Bearing != nil && !r.Bearing.Pending {
					t.Errorf("%s %s → %s: record %+v, want it standing, any bearing pending and unmarked", relation, source, target, r)
				}
			}
		}
	}
}

// TestAddEdgeWritesItsValues: a supplied bearing, *pending* included, marks a
// supports, lifts or refutes edge manually scored, judged over its ends'
// statements; a founds
// edge carries no mark; context is collapsed; a bearing on references, a
// bearing out of domain, a node no record carries and relation demoted are
// refused, writing nothing.
func TestAddEdgeWritesItsValues(t *testing.T) {
	root := recordKB(t, edgeOpsGraph())
	res := runEdges(t, root,
		edgeEntry("add-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, "bearing: 0.4", "context: by   the lemma"),
		edgeEntry("add-edge", "ana-aaaaaa", "clm-bbbbbb", kb.RelationLifts, "bearing: '*pending*'"),
		edgeEntry("add-edge", "wrk-s00000", "clm-bbbbbb", kb.RelationFounds, "bearing: 0.9"))
	if res.Outcome != result.Done {
		t.Fatalf("adds: %+v", res)
	}
	g := records(t, root)
	want := []kb.ScorableEdge{
		{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "by the lemma", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true,
			JudgedHash: edgeJudgedNow(t, root, "clm-aaaaaa", "clm-bbbbbb")},
		{Relation: kb.RelationLifts, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}, ManuallyScored: true,
			JudgedHash: edgeJudgedNow(t, root, "ana-aaaaaa", "clm-bbbbbb")},
	}
	if !slices.Equal(g.Scorable, want) {
		t.Errorf("scorable %+v, want %+v", g.Scorable, want)
	}
	if !slices.Equal(g.Founds, []kb.BearingEdge{{Source: "wrk-s00000", Target: "clm-bbbbbb", Bearing: kb.Bearing{Value: 0.9}}}) {
		t.Errorf("founds %+v, want the work's, bearing 0.9", g.Founds)
	}

	before := snapshot(t, root)
	for _, c := range []struct{ name, entry, key string }{
		{"a bearing on references", edgeEntry("add-edge", "clm-aaaaaa", "axm-s10000", kb.RelationReferences, "bearing: 0.5"), "bearing"},
		{"a bearing above 1", edgeEntry("add-edge", "clm-bbbbbb", "ana-bbbbbb", kb.RelationSupports, "bearing: 1.5"), "bearing"},
		{"a source no node carries", edgeEntry("add-edge", "clm-cccccc", "clm-bbbbbb", kb.RelationSupports), "source"},
		{"a target no node carries", edgeEntry("add-edge", "clm-aaaaaa", "ana-cccccc", kb.RelationSupports), "target"},
		{"relation demoted", edgeEntry("add-edge", "clm-bbbbbb", "clm-aaaaaa", kb.RelationDemoted), "relation"},
		{"a context with a line break", edgeEntry("add-edge", "clm-bbbbbb", "ana-bbbbbb", kb.RelationSupports, `context: "two\nlines"`), "context"},
	} {
		res := runEdges(t, root, c.entry)
		if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key {
			t.Errorf("%s: %+v, want one refusal at %s", c.name, res, c.key)
		}
	}
	missing := runEdges(t, root, edgeEntry("add-edge", "clm-cccccc", "clm-bbbbbb", kb.RelationSupports))
	if r := missing.Refusals; len(r) != 1 || r[0].Check != kb.CheckUnknownID || r[0].Detail != "no node clm-cccccc" {
		t.Errorf("a source no node carries: %+v, want check %s, detail no node clm-cccccc", r, kb.CheckUnknownID)
	}
	demoted := runEdges(t, root, edgeEntry("add-edge", "clm-bbbbbb", "clm-aaaaaa", kb.RelationDemoted))
	if r := demoted.Refusals; len(r) != 1 || !strings.Contains(r[0].Remedy, RestoreDemoted) {
		t.Errorf("relation demoted: %+v, want a remedy naming %s", r, RestoreDemoted)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-edge wrote the KB")
	}
}

// TestAddEdgeRefusesACycleInEdgeDirection: an add closing a cycle among the
// scoring edges is refused, check dependency-cycle at the entry's target, its
// entry named, its detail naming the cycle in edge direction, writing nothing;
// a references edge closes none.
func TestAddEdgeRefusesACycleInEdgeDirection(t *testing.T) {
	root := recordKB(t, cycleGraph())
	before := snapshot(t, root)
	res := runEdges(t, root, edgeEntry("add-edge", demotedA, demotedB, kb.RelationReferences),
		edgeEntry("add-edge", demotedB, demotedA, kb.RelationSupports))
	if len(res.Refusals) != 1 || res.Refusals[0].Check != checkDependencyCycle || res.Refusals[0].Key != "target" || res.Refusals[0].Entry != 2 ||
		!strings.Contains(res.Refusals[0].Detail, demotedB+" → "+demotedA+" → "+demotedB) {
		t.Errorf("a cycle at entry 2: %+v, want dependency-cycle at target, entry 2, naming %s → %s → %s", res, demotedB, demotedA, demotedB)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused add-edge wrote the KB")
	}
	if res := runEdges(t, root, edgeEntry("add-edge", demotedB, demotedA, kb.RelationReferences)); res.Outcome != result.Done {
		t.Errorf("a references edge against the supports edge: %+v, want done", res)
	}
}

// TestAddEdgeOverACut: an add of the edge a demoted record replaced writes it
// with the add's own values and deletes that record, another relation's
// record from the same source to the same target standing.
func TestAddEdgeOverACut(t *testing.T) {
	g := edgeOpsGraph()
	g.Demoted = []kb.DemotedEdge{
		{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationSupports, Context: "cut", Bearing: kb.Bearing{Value: 0.2}, ManuallyScored: true},
		{Source: "clm-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: kb.RelationRefutes, Bearing: kb.Bearing{Value: 0.3}},
	}
	root := recordKB(t, g)
	if res := runEdges(t, root, edgeEntry("add-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, "bearing: 0.7", "context: back")); res.Outcome != result.Done {
		t.Fatalf("add-edge over a cut: %+v", res)
	}
	got := records(t, root)
	if want := []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "back",
		Bearing: kb.Bearing{Value: 0.7}, ManuallyScored: true, JudgedHash: edgeJudgedNow(t, root, "clm-aaaaaa", "clm-bbbbbb")}}; !slices.Equal(got.Scorable, want) {
		t.Errorf("scorable %+v, want %+v", got.Scorable, want)
	}
	if !slices.Equal(got.Demoted, g.Demoted[1:]) {
		t.Errorf("demoted %+v, want only the refutes record standing", got.Demoted)
	}
}

// TestEditEdge: a bearing given marks a supports, lifts or refutes edge
// manually scored, judged over its ends' statements, a context alone leaving
// the hash standing, and a founds edge carries no
// mark; a context alone leaves the bearing and mark; values that stand are
// unchanged; no edge of the identity, a bearing on references, relation
// demoted and an entry giving neither field are refused.
func TestEditEdge(t *testing.T) {
	g := edgeOpsGraph()
	hash := kb.JudgedHash{1}
	g.Scorable = []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "old",
		Bearing: kb.Bearing{Value: 0.5}, JudgedHash: hash}}
	g.Founds = []kb.BearingEdge{{Source: "axm-s10000", Target: "clm-bbbbbb", Bearing: kb.Bearing{Value: 0.5}}}
	g.References = []kb.UnscoredEdge{{Source: "clm-bbbbbb", Target: "wrk-s00000"}}
	root := recordKB(t, g)
	supports := func(extra ...string) string {
		return edgeEntry("edit-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports, extra...)
	}

	res := runEdges(t, root, supports("context: new"))
	if e := onlyEntryOf(t, res); e.Outcome != result.Done || e.Source != "clm-aaaaaa" || e.Target != "clm-bbbbbb" || e.Relation != kb.RelationSupports {
		t.Errorf("a context edit: %+v, want done naming the edge", e)
	}
	want := kb.ScorableEdge{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: "new", Bearing: kb.Bearing{Value: 0.5}, JudgedHash: hash}
	if got := records(t, root).Scorable; !slices.Equal(got, []kb.ScorableEdge{want}) {
		t.Errorf("after a context edit: %+v, want %+v", got, want)
	}
	if res := runEdges(t, root, supports("bearing: 0.5")); onlyEntryOf(t, res).Outcome != result.Done {
		t.Errorf("re-asserting an unmarked bearing: %+v, want done", res)
	}
	want.ManuallyScored, want.JudgedHash = true, edgeJudgedNow(t, root, "clm-aaaaaa", "clm-bbbbbb")
	if got := records(t, root).Scorable; !slices.Equal(got, []kb.ScorableEdge{want}) {
		t.Errorf("after a bearing edit: %+v, want %+v", got, want)
	}
	before := snapshot(t, root)
	if res := runEdges(t, root, supports("bearing: 0.5", "context: new")); res.Outcome != result.Unchanged || onlyEntryOf(t, res).Outcome != result.Unchanged ||
		!maps.Equal(before, snapshot(t, root)) {
		t.Errorf("an edit of values that stand: %+v, want unchanged, byte-identical", res)
	}
	if res := runEdges(t, root, edgeEntry("edit-edge", "axm-s10000", "clm-bbbbbb", kb.RelationFounds, "bearing: 0.8")); res.Outcome != result.Done ||
		!slices.Equal(records(t, root).Founds, []kb.BearingEdge{{Source: "axm-s10000", Target: "clm-bbbbbb", Bearing: kb.Bearing{Value: 0.8}}}) {
		t.Errorf("a founds bearing edit: %+v, founds %+v", res, records(t, root).Founds)
	}

	before = snapshot(t, root)
	for _, c := range []struct{ name, entry, key, detail string }{
		{"no edge", edgeEntry("edit-edge", "clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports, "bearing: 0.5"), "relation",
			"no edge " + index.EdgeName(kb.RelationSupports, "clm-bbbbbb", "clm-aaaaaa")},
		{"a bearing on references", edgeEntry("edit-edge", "clm-bbbbbb", "wrk-s00000", kb.RelationReferences, "bearing: 0.5"), "bearing", ""},
		{"relation demoted", edgeEntry("edit-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationDemoted, "context: x"), "relation", ""},
	} {
		res := runEdges(t, root, c.entry)
		if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Key != c.key || c.detail != "" && res.Refusals[0].Detail != c.detail {
			t.Errorf("%s: %+v, want one refusal at %q", c.name, res, c.key)
		}
	}
	if r := runEdges(t, root, edgeEntry("edit-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationDemoted, "context: x")).Refusals; len(r) != 1 ||
		!strings.Contains(r[0].Remedy, RestoreDemoted) || !strings.Contains(r[0].Remedy, "remove-edge") {
		t.Errorf("relation demoted: %+v, want a remedy naming %s and remove-edge", r, RestoreDemoted)
	}
	_, refusals := parseValues([]byte("entry:\n"+supports()), Batch)
	if len(refusals) != 1 || refusals[0].Entry != 1 || !slices.Equal(refusals[0].Allowed, []string{"bearing", "context"}) {
		t.Errorf("neither field: %+v, want one shape refusal allowing bearing and context", refusals)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused edit-edge wrote the KB")
	}
}

// removeGraph holds one edge of each written relation and two demoted
// records from one claim to another beside a third elsewhere.
func removeGraph() kb.Graph {
	g := edgeOpsGraph()
	g.Scorable = []kb.ScorableEdge{
		{Relation: kb.RelationSupports, Source: "clm-aaaaaa", Target: "ana-aaaaaa", Context: "kept", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true, JudgedHash: kb.JudgedHash{2}},
		{Relation: kb.RelationLifts, Source: "ana-aaaaaa", Target: "clm-bbbbbb", Bearing: kb.Bearing{Pending: true}},
	}
	g.Founds = []kb.BearingEdge{{Source: "wrk-s00000", Target: "clm-aaaaaa", Bearing: kb.Bearing{Value: 0.6}}}
	g.References = []kb.UnscoredEdge{{Source: "clm-aaaaaa", Target: "axm-s10000", Context: "see"}}
	g.Demoted = []kb.DemotedEdge{
		{Source: "clm-bbbbbb", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}},
		{Source: "clm-bbbbbb", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationRefutes, Bearing: kb.Bearing{Value: 0.3}, ManuallyScored: true},
		{Source: "exp-aaaaaa", Target: "clm-aaaaaa", ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Value: 0.1}},
	}
	return g
}

// TestRemoveEdge: remove-edge removes the one record it names, and with
// relation demoted every demoted record from source to target, every other
// record standing byte for byte; a re-issue is refused, kb-root/ identical.
func TestRemoveEdge(t *testing.T) {
	cases := []struct {
		name, source, target, relation string
		without                        func(kb.Graph) kb.Graph
	}{
		{"supports", "clm-aaaaaa", "ana-aaaaaa", kb.RelationSupports, func(g kb.Graph) kb.Graph { g.Scorable = g.Scorable[1:]; return g }},
		{"lifts", "ana-aaaaaa", "clm-bbbbbb", kb.RelationLifts, func(g kb.Graph) kb.Graph { g.Scorable = g.Scorable[:1]; return g }},
		{"founds", "wrk-s00000", "clm-aaaaaa", kb.RelationFounds, func(g kb.Graph) kb.Graph { g.Founds = nil; return g }},
		{"references", "clm-aaaaaa", "axm-s10000", kb.RelationReferences, func(g kb.Graph) kb.Graph { g.References = nil; return g }},
		{"demoted", "clm-bbbbbb", "clm-aaaaaa", kb.RelationDemoted, func(g kb.Graph) kb.Graph { g.Demoted = g.Demoted[2:]; return g }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := recordKB(t, removeGraph())
			nodes := recordFileText(t, root, kb.GraphNodesFile)
			res := runEdges(t, root, edgeEntry("remove-edge", c.source, c.target, c.relation))
			if e := onlyEntryOf(t, res); e.Outcome != result.Done || e.Source != c.source || e.Target != c.target || e.Relation != c.relation {
				t.Fatalf("remove: %+v, want done naming the edge", res)
			}
			if got, want := recordFileText(t, root, kb.GraphEdgesFile), string(kb.EncodeEdges(c.without(removeGraph()))); got != want {
				t.Errorf("edges file:\n%s\nwant every other record byte for byte:\n%s", got, want)
			}
			if recordFileText(t, root, kb.GraphNodesFile) != nodes {
				t.Error("the node file moved")
			}
			before := snapshot(t, root)
			again := runEdges(t, root, edgeEntry("remove-edge", c.source, c.target, c.relation))
			if len(again.Refusals) != 1 || again.Refusals[0].Detail != "no edge "+index.EdgeName(c.relation, c.source, c.target) ||
				!maps.Equal(before, snapshot(t, root)) {
				t.Errorf("a re-issue: %+v, want refused, no edge, kb-root/ identical", again)
			}
		})
	}
	root := recordKB(t, removeGraph())
	for _, c := range []struct{ name, entry, key string }{
		{"a source no node carries", edgeEntry("remove-edge", "clm-cccccc", "ana-aaaaaa", kb.RelationSupports), "source"},
		{"a target no node carries", edgeEntry("remove-edge", "clm-aaaaaa", "ana-cccccc", kb.RelationReferences), "target"},
		{"a scoring relation not joining the kinds", edgeEntry("remove-edge", "ana-aaaaaa", "clm-bbbbbb", kb.RelationSupports), "relation"},
	} {
		if res := runEdges(t, root, c.entry); len(res.Refusals) != 1 || res.Refusals[0].Key != c.key {
			t.Errorf("%s: %+v, want one refusal at %s", c.name, res, c.key)
		}
	}
}

// TestARepeatedAddEdge: an add-edge re-issued differing in its bearing, its
// context or its bearing's being given is refused, edge exists, remedy
// edit-edge, kb-root/ left identical (TestIdempotence re-issues one
// identically). Within one batch an identical second entry is unchanged and a
// differing one refused at the repeat.
func TestARepeatedAddEdge(t *testing.T) {
	add := func(extra ...string) string {
		return edgeEntry("add-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationRefutes, extra...)
	}
	root := recordKB(t, edgeOpsGraph())
	if res := runEdges(t, root, add("bearing: 0.2")); res.Outcome != result.Done {
		t.Fatalf("add: %+v", res)
	}
	before := snapshot(t, root)
	for _, entry := range []string{add("bearing: 0.9"), add("bearing: 0.2", "context: by it"), add()} {
		res := runEdges(t, root, entry)
		if len(res.Refusals) != 1 || res.Refusals[0].Detail != "edge exists: "+index.EdgeName(kb.RelationRefutes, "clm-aaaaaa", "clm-bbbbbb") ||
			res.Refusals[0].Key != "relation" || !strings.Contains(res.Refusals[0].Remedy, "edit-edge") {
			t.Errorf("a differing re-issue: %+v, want edge exists at relation, remedy edit-edge", res)
		}
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a re-issued add-edge changed kb-root/")
	}

	res := runEdges(t, recordKB(t, edgeOpsGraph()), add("bearing: 0.2"), add("bearing: 0.2"))
	if res.Outcome != result.Done || len(res.Entries) != 2 || res.Entries[1].Outcome != result.Unchanged {
		t.Errorf("one batch adding twice: %+v, want the repeat unchanged", res)
	}
	res = runEdges(t, recordKB(t, edgeOpsGraph()), add("bearing: 0.2"), add("bearing: 0.9"))
	if res.Outcome != result.Refused || len(res.Refusals) != 1 || res.Refusals[0].Entry != 2 {
		t.Errorf("one batch adding a differing repeat: %+v, want refused at entry 2", res)
	}
}

// TestAnAddAfterARemoveInOneBatch: an edge removed by an earlier entry closes
// no cycle for a later one, though an add before the removal checked one.
func TestAnAddAfterARemoveInOneBatch(t *testing.T) {
	root := recordKB(t, edgeOpsGraph())
	res := runEdges(t, root,
		edgeEntry("add-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports),
		edgeEntry("remove-edge", "clm-aaaaaa", "clm-bbbbbb", kb.RelationSupports),
		edgeEntry("add-edge", "clm-bbbbbb", "clm-aaaaaa", kb.RelationSupports))
	if res.Outcome != result.Done {
		t.Fatalf("add, remove, add the reverse: %+v, want done", res)
	}
	if g := records(t, root); len(g.Scorable) != 1 || g.Scorable[0].Source != "clm-bbbbbb" {
		t.Errorf("scorable %+v, want the reverse edge alone", g.Scorable)
	}
}

// TestAWorkOrAnAxiomAddedTakesItsEdgeInASecondBatch: an add mints a work's
// and an axiom's id as it does every kind's, so its edge comes in a second
// batch, from the id the first batch's result gives: the edge stands.
func TestAWorkOrAnAxiomAddedTakesItsEdgeInASecondBatch(t *testing.T) {
	for _, add := range []string{
		"- operation: add-work\n  label: New2024\n  title: New Work\n  rationale: Cited.\n",
		"- operation: add-axiom\n  label: S9\n  title: New Axiom\n",
	} {
		root := recordKB(t, edgeOpsGraph())
		id := onlyEntry(t, runEdges(t, root, add)).ID
		if res := runEdges(t, root, edgeEntry("add-edge", id, "clm-aaaaaa", kb.RelationFounds)); res.Outcome != result.Done {
			t.Errorf("%s's edge in a second batch: %+v, want done", id, res)
			continue
		}
		if edgeRecord(records(t, root), kb.RelationFounds, id, "clm-aaaaaa") == nil {
			t.Errorf("%s founds clm-aaaaaa does not stand", id)
		}
	}
}

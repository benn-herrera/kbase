package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

const (
	demotedA, demotedB, demotedC, demotedD = "clm-aaaaaa", "clm-bbbbbb", "clm-cccccc", "clm-dddddd"
)

// demotedGraph holds four claims: B rests on A, and demoted records stand for
// a supports edge B → A that closes a cycle with it, a judged refutation
// C → A, and supports edges C → D and D → C that close one together.
func demotedGraph() kb.Graph {
	return kb.Graph{
		Claims: []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}, {ID: demotedC, Title: "C"}, {ID: demotedD, Title: "D"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: demotedA, Target: demotedB, Bearing: kb.Bearing{Value: 0.5}},
		},
		Demoted: []kb.DemotedEdge{
			{Source: demotedB, Target: demotedA, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}},
			{Source: demotedC, Target: demotedA, Context: "a cut", ReplacedRelation: kb.RelationRefutes, Bearing: kb.Bearing{Value: 0.3}, ManuallyScored: true},
			{Source: demotedC, Target: demotedD, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}},
			{Source: demotedD, Target: demotedC, ReplacedRelation: kb.RelationSupports, Bearing: kb.Bearing{Pending: true}},
		},
	}
}

func restore(source, target string) string {
	return "entry:\n- source: " + source + "\n  target: " + target + "\n  relation: " + kb.RelationDemoted + "\n"
}

// TestARefusedRestoreNamesTheStandingEdge: a restore refused "no edge" where a
// scoring edge from source to target stands — one written directly, or one
// an earlier restore put back — names that edge and the edges call that reads
// it in its remedy; one where none stands keeps the no-edge remedy.
func TestARefusedRestoreNamesTheStandingEdge(t *testing.T) {
	root := recordKB(t, demotedGraph())
	if res := runRecordOp(t, root, RestoreDemoted, restore(demotedC, demotedA), ProvenanceCaller); res.Outcome != result.Done {
		t.Fatalf("restoring %s → %s: %+v", demotedC, demotedA, res)
	}
	for _, tc := range []struct{ source, target, relation string }{
		{demotedA, demotedB, kb.RelationSupports},
		{demotedC, demotedA, kb.RelationRefutes},
	} {
		res := runRecordOp(t, root, RestoreDemoted, restore(tc.source, tc.target), ProvenanceCaller)
		if res.Outcome != result.Refused || len(res.Refusals) != 1 {
			t.Fatalf("restoring %s → %s: %+v, want one refusal", tc.source, tc.target, res)
		}
		remedy := res.Refusals[0].Remedy
		if !strings.Contains(remedy, index.EdgeName(tc.relation, tc.source, tc.target)) ||
			!strings.Contains(remedy, argmark.Call("edges", map[string]any{"id": tc.source, "direction": "out"})) {
			t.Errorf("restoring %s → %s: remedy %q, want it naming the standing %s edge and the edges call", tc.source, tc.target, remedy, tc.relation)
		}
	}
	res := runRecordOp(t, root, RestoreDemoted, restore(demotedB, demotedC), ProvenanceCaller)
	if len(res.Refusals) != 1 || res.Refusals[0].Remedy != noEdgeRemedy(RestoreDemoted) {
		t.Errorf("restoring %s → %s, no edge standing: %+v, want the no-edge remedy", demotedB, demotedC, res)
	}
}

// TestRestoreDemoted: a restore writes the replaced edge exactly — its
// relation, context, bearing and manually-scored — and deletes the record,
// reporting the relations it put back; issued twice, the second is refused,
// no edge, kb-root/ byte-identical; a restore that would close a cycle, alone
// or with the batch's earlier restores, is refused naming the cycle, and its
// remedy's remove-edge call lands; a batch naming one record twice, a
// relation other than demoted, an edge standing with no record and an entry
// with neither are refused; and an edit-edge of the replaced edge, refused
// with the no-edge remedy naming restore-demoted, lands once it is restored.
func TestRestoreDemoted(t *testing.T) {
	root := recordKB(t, demotedGraph())
	noEdge := func(source, target string) string {
		return "no edge " + index.EdgeName(kb.RelationDemoted, source, target)
	}
	twice := func(values string, restored ...string) {
		t.Helper()
		first := runRecordOp(t, root, RestoreDemoted, values, ProvenanceCaller)
		if first.Outcome != result.Done || len(first.Entries) != 1 || !slices.Equal(first.Entries[0].Restored, restored) {
			t.Fatalf("%q: %+v, want done, restoring %v", values, first, restored)
		}
		before := snapshot(t, root)
		second := runRecordOp(t, root, RestoreDemoted, values, ProvenanceCaller)
		if second.Outcome != result.Refused || len(second.Refusals) != 1 || !strings.HasPrefix(second.Refusals[0].Detail, "no edge ") ||
			!maps.Equal(before, snapshot(t, root)) {
			t.Errorf("%q again: %+v, want refused, no edge, byte-identical", values, second)
		}
	}

	before := snapshot(t, root)
	cycle := runRecordOp(t, root, RestoreDemoted, restore(demotedB, demotedA), ProvenanceCaller)
	if len(cycle.Refusals) != 1 || cycle.Refusals[0].Check != checkDependencyCycle ||
		!strings.Contains(cycle.Refusals[0].Detail, demotedB+" → "+demotedA+" → "+demotedB) {
		t.Errorf("a restore closing a cycle: %+v", cycle)
	}
	batch := runRecordOp(t, root, RestoreDemoted, "entry:\n- source: "+demotedC+"\n  target: "+demotedD+"\n  relation: "+kb.RelationDemoted+"\n"+
		"- source: "+demotedD+"\n  target: "+demotedC+"\n  relation: "+kb.RelationDemoted+"\n", ProvenanceCaller)
	if len(batch.Refusals) != 1 || batch.Refusals[0].Check != checkDependencyCycle || batch.Refusals[0].Entry != 2 ||
		!strings.Contains(batch.Refusals[0].Detail, demotedD+" → "+demotedC+" → "+demotedD) {
		t.Errorf("a batch whose second restore closes a cycle with its first: %+v", batch)
	}
	repeated := runRecordOp(t, root, RestoreDemoted, restore(demotedC, demotedD)+"- source: "+demotedC+"\n  target: "+demotedD+"\n  relation: "+kb.RelationDemoted+"\n", ProvenanceCaller)
	if repeated.Outcome != result.Refused || len(repeated.Refusals) != 1 || repeated.Refusals[0].Entry != 2 ||
		repeated.Refusals[0].Detail != noEdge(demotedC, demotedD) {
		t.Errorf("a batch naming one record twice: %+v, want refused at entry 2, no edge", repeated)
	}
	wrongRelation := runRecordOp(t, root, RestoreDemoted,
		"entry:\n- source: "+demotedB+"\n  target: "+demotedA+"\n  relation: "+kb.RelationSupports+"\n", ProvenanceCaller)
	if wrongRelation.Outcome != result.Refused || len(wrongRelation.Refusals) != 1 || wrongRelation.Refusals[0].Key != "relation" ||
		!slices.Equal(wrongRelation.Refusals[0].Allowed, []string{kb.RelationDemoted}) {
		t.Errorf("a relation other than demoted: %+v, want refused at relation, allowing demoted", wrongRelation)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused restore-demoted wrote the KB")
	}

	twice(restore(demotedC, demotedA), kb.RelationRefutes)
	g := records(t, root)
	rebuilt := kb.ScorableEdge{Relation: kb.RelationRefutes, Source: demotedC, Target: demotedA, Context: "a cut",
		Bearing: kb.Bearing{Value: 0.3}, ManuallyScored: true}
	if !slices.Contains(g.Scorable, rebuilt) || slices.ContainsFunc(g.Demoted, func(d kb.DemotedEdge) bool { return d.Source == demotedC && d.Target == demotedA }) {
		t.Errorf("the restore did not rebuild %+v from its demoted record: scorable %+v, demoted %+v", rebuilt, g.Scorable, g.Demoted)
	}

	for _, ends := range [][2]string{{demotedA, demotedB}, {demotedC, demotedB}} {
		if res := runRecordOp(t, root, RestoreDemoted, restore(ends[0], ends[1]), ProvenanceCaller); res.Outcome != result.Refused ||
			len(res.Refusals) != 1 || res.Refusals[0].Detail != noEdge(ends[0], ends[1]) {
			t.Errorf("restoring %s → %s, no demoted record standing: %+v, want refused, no edge", ends[0], ends[1], res)
		}
	}

	t.Run("the cycle remedy, followed", func(t *testing.T) {
		root := recordKB(t, demotedGraph())
		refused := runRecordOp(t, root, RestoreDemoted, restore(demotedB, demotedA), ProvenanceCaller)
		if len(refused.Refusals) != 1 || !strings.Contains(refused.Refusals[0].Remedy,
			"remove-edge, source "+demotedB+", target "+demotedA+" and relation "+kb.RelationDemoted) {
			t.Fatalf("the cycle refusal: %+v, want a remedy naming the remove-edge call", refused)
		}
		if res := runEdges(t, root, edgeEntry("remove-edge", demotedB, demotedA, kb.RelationDemoted)); res.Outcome != result.Done {
			t.Fatalf("the remedy's remove-edge: %+v", res)
		}
		if slices.ContainsFunc(records(t, root).Demoted, func(d kb.DemotedEdge) bool { return d.Source == demotedB && d.Target == demotedA }) {
			t.Error("the demoted record from B to A still stands after the remedy's remove-edge")
		}
	})

	t.Run("the edit-edge remedy, followed", func(t *testing.T) {
		root := recordKB(t, demotedGraph())
		bearing := edgeEntry(EditEdge, demotedC, demotedA, kb.RelationRefutes, "bearing: 0.2")
		refused := runEdges(t, root, bearing)
		if len(refused.Refusals) != 1 || refused.Refusals[0].Detail != "no edge "+index.EdgeName(kb.RelationRefutes, demotedC, demotedA) ||
			!strings.Contains(refused.Refusals[0].Remedy, RestoreDemoted) {
			t.Fatalf("edit-edge over a demoted record: %+v, want no edge, the remedy naming %s", refused, RestoreDemoted)
		}
		if res := runRecordOp(t, root, RestoreDemoted, restore(demotedC, demotedA), ProvenanceCaller); res.Outcome != result.Done {
			t.Fatalf("the remedy's restore: %+v", res)
		}
		if res := runEdges(t, root, bearing); res.Outcome != result.Done {
			t.Errorf("edit-edge after the restore: %+v, want done", res)
		}
	})
}

// TestDemoteClaimEdge: a standing scoring edge becomes the demoted record that
// keeps it whole — its relation, context, bearing and manually-scored — and
// another edge between the same two nodes stands; a re-issue over the record
// alone is unchanged and byte-identical, and an edge with neither is refused.
func TestDemoteClaimEdge(t *testing.T) {
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}, {ID: demotedC, Title: "C"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: demotedA, Target: demotedB, Context: "the seed it rests on", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true},
			{Relation: kb.RelationRefutes, Source: demotedC, Target: demotedA, Bearing: kb.Bearing{Value: 0.2}},
			{Relation: kb.RelationSupports, Source: demotedC, Target: demotedA, Bearing: kb.Bearing{Value: 0.6}},
		},
	})
	demote := func(source, target, relation string) string {
		return "entry:\n- source: " + source + "\n  target: " + target + "\n  relation: " + relation + "\n"
	}
	kept := kb.DemotedEdge{Source: demotedA, Target: demotedB, Context: "the seed it rests on", ReplacedRelation: kb.RelationSupports,
		Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true}
	for _, c := range []struct {
		source, target, relation string
		want                     kb.DemotedEdge
	}{
		{demotedA, demotedB, kb.RelationSupports, kept},
		{demotedC, demotedA, kb.RelationRefutes, kb.DemotedEdge{Source: demotedC, Target: demotedA, ReplacedRelation: kb.RelationRefutes, Bearing: kb.Bearing{Value: 0.2}}},
	} {
		if res := runRecordOp(t, root, DemoteClaimEdge, demote(c.source, c.target, c.relation), ProvenanceMachine); res.Outcome != result.Done {
			t.Fatalf("demote %s %s → %s: %+v", c.relation, c.source, c.target, res)
		}
		g := records(t, root)
		if !slices.Contains(g.Demoted, c.want) || slices.ContainsFunc(g.Scorable, func(r kb.ScorableEdge) bool {
			return r.Relation == c.relation && r.Source == c.source && r.Target == c.target
		}) {
			t.Errorf("demote %s %s → %s: scorable %+v, demoted %+v; want the edge replaced by %+v", c.relation, c.source, c.target, g.Scorable, g.Demoted, c.want)
		}
		before := snapshot(t, root)
		if again := runRecordOp(t, root, DemoteClaimEdge, demote(c.source, c.target, c.relation), ProvenanceMachine); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
			t.Errorf("demote %s %s → %s again: %+v, want unchanged, byte-identical", c.relation, c.source, c.target, again)
		}
	}
	g := records(t, root)
	if !slices.Contains(g.Scorable, kb.ScorableEdge{Relation: kb.RelationSupports, Source: demotedC, Target: demotedA, Bearing: kb.Bearing{Value: 0.6}}) {
		t.Errorf("the supports edge %s → %s did not stand: scorable %+v", demotedC, demotedA, g.Scorable)
	}

	absent := runRecordOp(t, root, DemoteClaimEdge, demote(demotedC, demotedB, kb.RelationSupports), ProvenanceMachine)
	if absent.Outcome != result.Refused || len(absent.Refusals) != 1 || absent.Refusals[0].Key != "target="+demotedB {
		t.Errorf("demoting an edge that does not stand: %+v, want one refusal at target=%s", absent, demotedB)
	}
}

// TestAnAddOverACutWhoseCycleStands: an add-edge of the edge a demoted record
// replaced, while the edge closing the cut cycle with it still stands, is
// refused for that cycle, writing nothing.
func TestAnAddOverACutWhoseCycleStands(t *testing.T) {
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}},
		Scorable: []kb.ScorableEdge{{Relation: kb.RelationSupports, Source: demotedB, Target: demotedA,
			Bearing: kb.Bearing{Pending: true}}},
		Demoted: []kb.DemotedEdge{{Source: demotedA, Target: demotedB, ReplacedRelation: kb.RelationSupports,
			Bearing: kb.Bearing{Pending: true}}},
	})
	before := snapshot(t, root)
	res := runEdges(t, root, edgeEntry(AddEdge, demotedA, demotedB, kb.RelationSupports))
	if len(res.Refusals) != 1 || res.Refusals[0].Check != checkDependencyCycle || !maps.Equal(before, snapshot(t, root)) {
		t.Errorf("an add whose cycle stands: %+v, want one dependency-cycle refusal and nothing written", res)
	}
}

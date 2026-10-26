package write

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

// insertClaim inserts a claim titled title into the seed KB and returns its id.
func insertClaim(t *testing.T, root, title, extra string) string {
	t.Helper()
	res := run(t, root, "insert-claim-entry", "entry:\n- title: "+title+"\n  rigor: 0.5\n  rationale: Shown.\n"+extra)
	if res.Outcome != result.Done || len(res.IDs) != 1 {
		t.Fatalf("insert %s: %+v", title, res)
	}
	return res.IDs[0]
}

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

func resolve(id, target, action string) string {
	return "entry:\n- id: " + id + "\n  target: " + target + "\n  action: " + action + "\n"
}

// TestResolveDemoted: a restore writes the replaced edge exactly — its
// relation, context, bearing and manually-scored — and a remove deletes the
// record, each issued twice, the second unchanged and byte-identical; a
// restore that would close a cycle, alone or with the batch's earlier
// restores, is refused naming the path; a batch naming one pair twice is
// refused; a pair with no demoted record is unchanged where the edge stands
// or the action is remove, and refused where a restore has nothing to restore.
func TestResolveDemoted(t *testing.T) {
	root := recordKB(t, demotedGraph())
	twice := func(values string, want []Resolution) {
		t.Helper()
		first := runRecordOp(t, root, ResolveDemoted, values, ProvenanceCaller)
		if first.Outcome != result.Done || !slices.Equal(first.Resolved, want) {
			t.Fatalf("%q: %+v, want done resolving %v", values, first, want)
		}
		before := snapshot(t, root)
		second := runRecordOp(t, root, ResolveDemoted, values, ProvenanceCaller)
		if second.Outcome != result.Unchanged || len(second.Resolved) != 0 || !maps.Equal(before, snapshot(t, root)) {
			t.Errorf("%q again: %+v, want unchanged resolving nothing, byte-identical", values, second)
		}
	}

	before := snapshot(t, root)
	cycle := runRecordOp(t, root, ResolveDemoted, resolve(demotedA, demotedB, actionRestore), ProvenanceCaller)
	if len(cycle.Refusals) != 1 || cycle.Refusals[0].Check != checkDependencyCycle ||
		!strings.Contains(cycle.Refusals[0].Detail, demotedB+" → "+demotedA+" → "+demotedB) {
		t.Errorf("a restore closing a cycle: %+v", cycle)
	}
	batch := runRecordOp(t, root, ResolveDemoted, "entry:\n- id: "+demotedD+"\n  target: "+demotedC+"\n  action: restore\n"+
		"- id: "+demotedC+"\n  target: "+demotedD+"\n  action: restore\n", ProvenanceCaller)
	if len(batch.Refusals) != 1 || batch.Refusals[0].Check != checkDependencyCycle || batch.Refusals[0].Entry != 2 ||
		!strings.Contains(batch.Refusals[0].Detail, demotedD+" → "+demotedC+" → "+demotedD) {
		t.Errorf("a batch whose second restore closes a cycle with its first: %+v", batch)
	}
	repeated := runRecordOp(t, root, ResolveDemoted, "entry:\n- id: "+demotedD+"\n  target: "+demotedC+"\n  action: remove\n"+
		"- id: "+demotedD+"\n  target: "+demotedC+"\n  action: restore\n", ProvenanceCaller)
	if repeated.Outcome != result.Refused || len(repeated.Refusals) != 1 || repeated.Refusals[0].Entry != 2 || repeated.Refusals[0].Key != "target" {
		t.Errorf("a batch naming one pair twice: %+v, want refused at entry 2", repeated)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("a refused resolve-demoted wrote the KB")
	}

	twice(resolve(demotedA, demotedC, actionRestore), []Resolution{{demotedA, demotedC, resolvedRestore}})
	g := records(t, root)
	rebuilt := kb.ScorableEdge{Relation: kb.RelationRefutes, Source: demotedC, Target: demotedA, Context: "a cut",
		Bearing: kb.Bearing{Value: 0.3}, ManuallyScored: true}
	if !slices.Contains(g.Scorable, rebuilt) || slices.ContainsFunc(g.Demoted, func(d kb.DemotedEdge) bool { return d.Source == demotedC && d.Target == demotedA }) {
		t.Errorf("the restore did not rebuild %+v from its demoted record: scorable %+v, demoted %+v", rebuilt, g.Scorable, g.Demoted)
	}

	twice(resolve(demotedA, demotedB, actionRemove), []Resolution{{demotedA, demotedB, resolvedRemove}})
	g = records(t, root)
	if slices.ContainsFunc(g.Demoted, func(d kb.DemotedEdge) bool { return d.Source == demotedB && d.Target == demotedA }) ||
		slices.ContainsFunc(g.Scorable, func(r kb.ScorableEdge) bool { return r.Source == demotedB && r.Target == demotedA }) {
		t.Errorf("the removed demoted record stands, or was restored: scorable %+v, demoted %+v", g.Scorable, g.Demoted)
	}

	if res := runRecordOp(t, root, ResolveDemoted, resolve(demotedB, demotedA, actionRestore), ProvenanceCaller); res.Outcome != result.Unchanged {
		t.Errorf("restoring a pair whose edge stands: %+v, want unchanged", res)
	}
	if res := runRecordOp(t, root, ResolveDemoted, resolve(demotedB, demotedC, actionRestore), ProvenanceCaller); res.Outcome != result.Refused ||
		len(res.Refusals) != 1 || res.Refusals[0].Key != "target="+demotedC {
		t.Errorf("restoring a pair with neither: %+v, want one refusal at target=%s", res, demotedC)
	}
}

// TestDemoteClaimEdge: a standing scoring edge becomes the demoted record that
// keeps it whole — its relation, context, bearing and manually-scored — and a
// re-issue over the record alone is unchanged and byte-identical; an edge
// standing again beside its record goes, the record as it stood; a pair with
// neither is refused.
func TestDemoteClaimEdge(t *testing.T) {
	root := recordKB(t, kb.Graph{
		Claims: []kb.ClaimNode{{ID: demotedA, Title: "A"}, {ID: demotedB, Title: "B"}, {ID: demotedC, Title: "C"}},
		Scorable: []kb.ScorableEdge{
			{Relation: kb.RelationSupports, Source: demotedA, Target: demotedB, Context: "the seed it rests on", Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true},
			{Relation: kb.RelationRefutes, Source: demotedC, Target: demotedA, Bearing: kb.Bearing{Value: 0.2}},
		},
	})
	demote := func(id, target string) string { return "entry:\n- id: " + id + "\n  target: " + target + "\n" }
	kept := kb.DemotedEdge{Source: demotedA, Target: demotedB, Context: "the seed it rests on", ReplacedRelation: kb.RelationSupports,
		Bearing: kb.Bearing{Value: 0.4}, ManuallyScored: true}
	for _, c := range []struct {
		id, target string
		want       kb.DemotedEdge
	}{
		{demotedB, demotedA, kept},
		{demotedA, demotedC, kb.DemotedEdge{Source: demotedC, Target: demotedA, ReplacedRelation: kb.RelationRefutes, Bearing: kb.Bearing{Value: 0.2}}},
	} {
		if res := runRecordOp(t, root, DemoteClaimEdge, demote(c.id, c.target), ProvenanceMachine); res.Outcome != result.Done {
			t.Fatalf("demote %s → %s: %+v", c.target, c.id, res)
		}
		g := records(t, root)
		if !slices.Contains(g.Demoted, c.want) || slices.ContainsFunc(g.Scorable, func(r kb.ScorableEdge) bool { return r.Source == c.target && r.Target == c.id }) {
			t.Errorf("demote %s → %s: scorable %+v, demoted %+v; want the edge replaced by %+v", c.target, c.id, g.Scorable, g.Demoted, c.want)
		}
		before := snapshot(t, root)
		if again := runRecordOp(t, root, DemoteClaimEdge, demote(c.id, c.target), ProvenanceMachine); again.Outcome != result.Unchanged || !maps.Equal(before, snapshot(t, root)) {
			t.Errorf("demote %s → %s again: %+v, want unchanged, byte-identical", c.target, c.id, again)
		}
	}

	if res := runRecordOp(t, root, AddBuildEdges, "entry:\n- id: "+demotedB+"\n  rests-on:\n  - id: "+demotedA+"\n", ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("re-planting the edge beside its demoted record: %+v", res)
	}
	if res := runRecordOp(t, root, DemoteClaimEdge, demote(demotedB, demotedA), ProvenanceMachine); res.Outcome != result.Done {
		t.Fatalf("demoting the edge standing beside its record: %+v", res)
	}
	g := records(t, root)
	if slices.ContainsFunc(g.Scorable, func(r kb.ScorableEdge) bool { return r.Source == demotedA && r.Target == demotedB }) ||
		!slices.Contains(g.Demoted, kept) || len(g.Demoted) != 2 {
		t.Errorf("record and edge both standing: scorable %+v, demoted %+v; want the edge gone and the record as it stood", g.Scorable, g.Demoted)
	}

	absent := runRecordOp(t, root, DemoteClaimEdge, demote(demotedB, demotedC), ProvenanceMachine)
	if absent.Outcome != result.Refused || len(absent.Refusals) != 1 || absent.Refusals[0].Key != "target="+demotedC {
		t.Errorf("demoting an edge that does not stand: %+v, want one refusal at target=%s", absent, demotedC)
	}
}

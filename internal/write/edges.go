package write

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/argmark"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// nodeKind is the kind of the node record id names, refused where none does.
func (b *recordBatch) nodeKind(key, id string) (string, error) {
	kind, ok := index.GraphKinds(b.g)[id]
	if !ok {
		return "", noRecord(kb.CheckUnknownID, key, id)
	}
	return kind, nil
}

// addBuildEdges adds the edges an entry's rests-on list carries into its node
// and its references list out of it. It refuses no cycle: the build's walk
// cuts what its edges close.
func addBuildEdges(b *recordBatch, e entry) error {
	id := e.str("id")
	kind, err := b.nodeKind("id", id)
	if err != nil {
		return err
	}
	if !e.has(kb.ListRestsOn) && !e.has(kb.RelationReferences) {
		return opRefusal("id="+id, "names neither a rests-on nor a references list, and this op adds the edges those lists carry",
			"restore: supply a rests-on list, a references list, or both, then re-run")
	}
	if e.has(kb.RelationReferences) && kind != kb.NodeKindClaim {
		return opRefusal(kb.RelationReferences, id+" carries a references list, and a reference is one claim of this corpus naming another",
			"restore: drop the references list, or name a claim as the id, then re-run")
	}
	edges := listEdges(e, kind)
	if err := b.resolveEnds(edges); err != nil {
		return err
	}
	return b.addEdges(e, id, edges)
}

// dropEdges removes every edge record match accepts.
func (b *recordBatch) dropEdges(match func(relation, source, target string) bool) {
	b.depends = nil
	g := &b.g
	g.References = slices.DeleteFunc(g.References, func(r kb.UnscoredEdge) bool {
		return match(kb.RelationReferences, r.Source, r.Target)
	})
	g.Founds = slices.DeleteFunc(g.Founds, func(r kb.BearingEdge) bool { return match(kb.RelationFounds, r.Source, r.Target) })
	g.Scorable = slices.DeleteFunc(g.Scorable, func(r kb.ScorableEdge) bool { return match(r.Relation, r.Source, r.Target) })
}

// takeDemoted removes every demoted record from source to target and returns
// them.
func (b *recordBatch) takeDemoted(source, target string) []kb.DemotedEdge {
	var taken []kb.DemotedEdge
	b.g.Demoted = slices.DeleteFunc(b.g.Demoted, func(d kb.DemotedEdge) bool {
		if d.Source == source && d.Target == target {
			taken = append(taken, d)
			return true
		}
		return false
	})
	return taken
}

// edgeLister is the query a "no edge" remedy points at, edges' call.
var edgeLister = "list the edge records that stand with " + argmark.Call("edges", nil)

// noEdgeRemedy is how op's entry naming an edge no record holds is put right:
// a removal's record may already be gone, and any other op's edge may be added
// by a later entry of the batch.
func noEdgeRemedy(op string) string {
	if removal(op) {
		return edgeLister + ", demoted ones included. Where an earlier batch or an earlier entry of this one already removed or restored " +
			"this record, nothing is left to do: drop the entry. Otherwise correct source, target or relation, then re-run"
	}
	return edgeLister + ". Where it lists this edge as demoted, put a restore-demoted entry before this one. " +
		"Where a later entry of this batch adds this edge, move that add before this entry. " +
		"Otherwise correct source, target or relation, then re-run"
}

// noEdge refuses op's entry naming an edge identity no record holds, at its
// relation.
func noEdge(op, relation, source, target string) result.Refusal {
	return result.Refusal{{Key: "relation", Detail: "no edge " + index.EdgeName(relation, source, target), Remedy: noEdgeRemedy(op)}}
}

// cycleRemedy is how a cycle the named edge would close is cleared: remove
// another edge of it first.
var cycleRemedy = "remove another edge of the named cycle with remove-edge first — a step X → Y is source X and target Y, " +
	"and " + argmark.Call("edges", nil) + " lists its relation in the row with source X and target Y — then re-run"

// cycleRefusal refuses the edge named, which would close cycle, remedy its
// clearing.
func cycleRefusal(name string, cycle []string, remedy string) error {
	return result.Refusal{{Check: checkDependencyCycle, Key: "target", Remedy: remedy,
		Detail: fmt.Sprintf("%s would close the cycle %s", name, strings.Join(cycle, " → "))}}
}

// edgeEnds is the kinds of the nodes source and target name, op's entry
// refused where either names none.
func (b *recordBatch) edgeEnds(op, source, target string) (sourceKind, targetKind string, err error) {
	kinds := index.GraphKinds(b.g)
	for _, end := range [][2]string{{"source", source}, {"target", target}} {
		if _, ok := kinds[end[1]]; !ok {
			return "", "", noNode(op, end[0], end[1])
		}
	}
	return kinds[source], kinds[target], nil
}

// joiningRelations is the relations an op writes that join a sourceKind node
// to a targetKind node, in the relations table's order.
func joiningRelations(sourceKind, targetKind string) []string {
	var out []string
	for _, r := range authoredRelations {
		if Joins(r, sourceKind, targetKind) {
			out = append(out, r)
		}
	}
	return out
}

// Joins is whether an op may write a relation edge from a sourceKind node to a
// targetKind node: references joins any kind to any kind, a scoring relation
// the kinds whose edge may carry a bearing, and demoted none.
func Joins(relation, sourceKind, targetKind string) bool {
	return relation == kb.RelationReferences || kb.BearingLegal(relation, sourceKind, targetKind)
}

// unjoined refuses relation, which does not join a sourceKind node to a
// targetKind node.
func unjoined(relation, sourceKind, targetKind string) result.Item {
	return result.Item{Key: "relation", Remedy: "correct relation, source or target, then re-run",
		Detail: fmt.Sprintf("%s does not join a %s source to a %s target", relation, sourceKind, targetKind)}
}

// referencesBearing refuses a bearing on a references edge.
func referencesBearing() error {
	return result.Refusal{{Key: "bearing", Remedy: "drop bearing, then re-run",
		Detail: "a references edge carries no bearing"}}
}

// addEdge writes one edge, refused where its ends name no node, where its
// relation does not join their kinds, where a references edge is given a
// bearing, and where a scoring edge would close a cycle. A left-out bearing is
// pending; a demoted record of the edge is deleted, the add's own values
// written. Over an edge of its identity it writes that edge's record, and
// stands only where that moves no byte.
func addEdge(b *recordBatch, e entry) error {
	source, target, relation := e.str("source"), e.str("target"), e.str("relation")
	sourceKind, targetKind, err := b.edgeEnds(e.Op, source, target)
	if err != nil {
		return err
	}
	if joins := joiningRelations(sourceKind, targetKind); !slices.Contains(joins, relation) {
		item := unjoined(relation, sourceKind, targetKind)
		item.Allowed = joins
		return result.Refusal{item}
	}
	if relation == kb.RelationReferences && e.has("bearing") {
		return referencesBearing()
	}
	name := index.EdgeName(relation, source, target)
	held := b.holds(relation, source, target)
	if !held && relation != kb.RelationReferences {
		if cycle := b.closesCycle(source, target); cycle != nil {
			return cycleRefusal(name, cycle, cycleRemedy)
		}
	}
	ends := func(s, t string) bool { return s == source && t == target }
	context, bearing := e.str("context"), bearingOf(e.score("bearing"))
	switch relation {
	case kb.RelationReferences:
		b.g.References = put(b.g.References, kb.UnscoredEdge{Source: source, Target: target, Context: context},
			func(r kb.UnscoredEdge) bool { return ends(r.Source, r.Target) })
	case kb.RelationFounds:
		b.g.Founds = put(b.g.Founds, kb.BearingEdge{Source: source, Target: target, Context: context, Bearing: bearing},
			func(r kb.BearingEdge) bool { return ends(r.Source, r.Target) })
	default:
		b.g.Demoted = slices.DeleteFunc(b.g.Demoted, kb.DemotedMatch(relation, source, target))
		b.g.Scorable = put(b.g.Scorable, kb.ScorableEdge{Relation: relation, Source: source, Target: target, Context: context,
			Bearing: bearing, ManuallyScored: b.judged(e.has("bearing"))},
			func(r kb.ScorableEdge) bool { return r.Relation == relation && ends(r.Source, r.Target) })
		if e.has("bearing") {
			b.valueSet(scoredRecord{relation: relation, source: source, target: target}, bearing.Pending)
		}
	}
	if held {
		return &existsError{result.Refusal{{Key: "relation", Detail: "edge exists: " + name,
			Remedy: "edit-edge changes a standing edge's bearing or context; " + argmark.Call("edges", nil) + " reads its values"}}}
	}
	return nil
}

// editEdge replaces the bearing and the context its entry gives of the edge it
// names, refused where no record holds it and where a references edge is given
// a bearing. A bearing given marks a supports, lifts or refutes edge manually
// scored; its judged hash stands.
func editEdge(b *recordBatch, e entry) error {
	source, target, relation := e.str("source"), e.str("target"), e.str("relation")
	ends := func(s, t string) bool { return s == source && t == target }
	var context *string
	switch relation {
	case kb.RelationReferences:
		if e.has("bearing") {
			return referencesBearing()
		}
		if r := record(b.g.References, func(r kb.UnscoredEdge) bool { return ends(r.Source, r.Target) }); r != nil {
			context = &r.Context
		}
	case kb.RelationFounds:
		if r := record(b.g.Founds, func(r kb.BearingEdge) bool { return ends(r.Source, r.Target) }); r != nil {
			context = &r.Context
			if e.has("bearing") {
				r.Bearing = bearingOf(e.score("bearing"))
			}
		}
	default:
		if r := record(b.g.Scorable, func(r kb.ScorableEdge) bool { return r.Relation == relation && ends(r.Source, r.Target) }); r != nil {
			context = &r.Context
			if e.has("bearing") {
				r.Bearing, r.ManuallyScored = bearingOf(e.score("bearing")), b.judged(true)
				b.valueSet(scoredRecord{relation: relation, source: source, target: target}, r.Bearing.Pending)
			}
		}
	}
	if context == nil {
		return noEdge(e.Op, relation, source, target)
	}
	if e.has("context") {
		*context = e.str("context")
	}
	return nil
}

// removeEdge removes the one edge record its entry names, or, for demoted,
// every demoted record from source to target, leaving the edge out of the
// graph. It is refused where an end names no node, where a scoring relation
// does not join their kinds, and where no such record stands.
func removeEdge(b *recordBatch, e entry) error {
	source, target, relation := e.str("source"), e.str("target"), e.str("relation")
	sourceKind, targetKind, err := b.edgeEnds(e.Op, source, target)
	if err != nil {
		return err
	}
	if kb.Scoring(relation) && !kb.BearingLegal(relation, sourceKind, targetKind) {
		return result.Refusal{unjoined(relation, sourceKind, targetKind)}
	}
	if relation == kb.RelationDemoted {
		if len(b.takeDemoted(source, target)) == 0 {
			return noEdge(e.Op, relation, source, target)
		}
		return nil
	}
	if !b.holds(relation, source, target) {
		return noEdge(e.Op, relation, source, target)
	}
	b.dropEdges(func(r, s, t string) bool { return r == relation && s == source && t == target })
	return nil
}

// restoreDemoted puts back the edge each demoted record from source to target
// replaced, exactly, deleting the record, and reports the relations it put
// back. It refuses an entry no demoted record from source to target answers —
// its remedy naming the scoring edge from source to target where one stands,
// which nothing records the origin of — and a restore that would close a
// cycle.
func restoreDemoted(b *recordBatch, e entry) error {
	source, target := e.str("source"), e.str("target")
	demoted := b.takeDemoted(source, target)
	if len(demoted) == 0 {
		refusal := noEdge(e.Op, kb.RelationDemoted, source, target)
		if r := record(b.g.Scorable, func(r kb.ScorableEdge) bool { return r.Source == source && r.Target == target }); r != nil {
			refusal[0].Remedy = fmt.Sprintf("the edge %s stands; %s reads it. No demoted record from %s to %s is left to restore: "+
				"drop the entry, or correct source or target, then re-run", index.EdgeName(r.Relation, source, target),
				argmark.Call("edges", map[string]any{"id": source, "direction": "out"}), source, target)
		}
		return refusal
	}
	var restored []string
	for _, d := range demoted {
		if cycle := b.closesCycle(d.Source, d.Target); cycle != nil {
			return cycleRefusal("restoring "+index.EdgeName(d.ReplacedRelation, d.Source, d.Target), cycle,
				cycleRemedy+fmt.Sprintf("; or, to leave the edge out of the graph, remove the demoted record with remove-edge, source %s, target %s and relation %s",
					source, target, kb.RelationDemoted))
		}
		b.g.Scorable = append(b.g.Scorable, kb.ScorableEdge{Relation: d.ReplacedRelation, Source: d.Source, Target: d.Target,
			Context: d.Context, Bearing: d.Bearing, ManuallyScored: d.ManuallyScored})
		restored = append(restored, d.ReplacedRelation)
	}
	b.report = entryReport{Restored: restored}
	return nil
}

// demoteEdge replaces the standing scoring edge of relation from source to
// target with the demoted record that keeps it whole. Where only that record
// stands the edge is left as it stands, and an edge with neither is refused.
func demoteEdge(b *recordBatch, e entry) error {
	source, target, relation := e.str("source"), e.str("target"), e.str("relation")
	i := slices.IndexFunc(b.g.Scorable, func(r kb.ScorableEdge) bool {
		return r.Relation == relation && r.Source == source && r.Target == target
	})
	if i < 0 {
		if slices.ContainsFunc(b.g.Demoted, kb.DemotedMatch(relation, source, target)) {
			return nil
		}
		return opRefusal("target="+target, fmt.Sprintf("no %s edge runs from %s to %s, and this op demotes a standing edge", relation, source, target),
			"restore: add the edge, or correct source, target or relation, then re-run")
	}
	r := b.g.Scorable[i]
	b.g.Scorable = slices.Delete(b.g.Scorable, i, i+1)
	b.depends = nil
	b.g.Demoted = append(b.g.Demoted, kb.DemotedEdge{Source: r.Source, Target: r.Target, Context: r.Context,
		ReplacedRelation: r.Relation, Bearing: r.Bearing, ManuallyScored: r.ManuallyScored})
	return nil
}

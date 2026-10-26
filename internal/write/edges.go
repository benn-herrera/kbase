package write

import (
	"fmt"
	"slices"
	"strings"

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

// addRestsOn adds the edges an entry's rests-on list carries into its node
// and its references list out of it, refusing one that would close a cycle.
func addRestsOn(b *recordBatch, e entry) error { return addListEdges(b, e, true) }

// addBuildEdges is addRestsOn without the cycle refusal: the build's walk
// cuts what its edges close.
func addBuildEdges(b *recordBatch, e entry) error { return addListEdges(b, e, false) }

func addListEdges(b *recordBatch, e entry, refuseCycles bool) error {
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
	edges := listEdges(e, kind, kb.ListRestsOn, kb.RelationReferences)
	if err := b.resolveEnds(edges); err != nil {
		return err
	}
	return b.addEdges(e, id, edges, refuseCycles)
}

// pairListsBySource is the pair lists add-pairs takes from a source of each
// kind.
var pairListsBySource = map[string][]string{
	kb.NodeKindClaim:      {kb.RefutesKey},
	kb.NodeKindAnalysis:   {kb.LiftsKey, kb.RefutesKey},
	kb.NodeKindExperiment: {kb.SupportsKey, kb.LiftsKey, kb.RefutesKey},
	kb.NodeKindWork:       nil,
	kb.NodeKindAxiom:      nil,
}

// addPairs adds the scoring edges an entry's pair lists carry from its node,
// refusing a list its source's kind does not author and an edge that would
// close a cycle.
func addPairs(b *recordBatch, e entry) error {
	id := e.str("id")
	kind, err := b.nodeKind("id", id)
	if err != nil {
		return err
	}
	var keys []string
	for _, f := range opFields["add-pairs"] {
		if f.name == "id" || !e.has(f.name) {
			continue
		}
		if !slices.Contains(pairListsBySource[kind], f.name) {
			return result.Refusal{{Key: f.name, Entry: e.Index, Allowed: pairListsBySource[kind],
				Remedy: fmt.Sprintf("supply only the lists a %s authors, then re-run", kind),
				Detail: fmt.Sprintf("is a list a %s does not author: %s is a %s", kind, id, kind)}}
		}
		keys = append(keys, f.name)
	}
	if len(keys) == 0 {
		return opRefusal("id="+id, "names no pair list, and this op adds the edges those lists carry",
			"restore: supply a "+strings.Join(pairListsBySource[kind], ", ")+" list, then re-run")
	}
	edges := listEdges(e, kind, keys...)
	if err := b.resolveEnds(edges); err != nil {
		return err
	}
	return b.addEdges(e, id, edges, true)
}

// ownEdge is whether a record of relation from source to target is one of the
// claim id's own edge lists: a supports or founds edge into it, its rests-on
// list, or a references edge out of it.
func ownEdge(id, relation, source, target string) bool {
	switch relation {
	case kb.RelationSupports, kb.RelationFounds:
		return target == id
	case kb.RelationReferences:
		return source == id
	}
	return false
}

// inboundEdge is whether a record of relation from source to target is
// another node's edge naming the claim id: a supports edge out of it, a
// references, lifts or refutes edge into it, or an experiment's support of it.
func inboundEdge(id, relation, source, target string) bool {
	switch relation {
	case kb.RelationSupports:
		kind, _ := kb.NodeIDKind(source)
		return source == id || target == id && kind == kb.NodeKindExperiment
	case kb.RelationReferences, kb.RelationLifts, kb.RelationRefutes:
		return target == id
	}
	return false
}

// dropEdges removes every edge record match accepts.
func (b *recordBatch) dropEdges(match func(relation, source, target string) bool) {
	g := &b.g
	g.References = slices.DeleteFunc(g.References, func(r kb.UnscoredEdge) bool {
		return match(kb.RelationReferences, r.Source, r.Target)
	})
	g.Founds = slices.DeleteFunc(g.Founds, func(r kb.BearingEdge) bool { return match(kb.RelationFounds, r.Source, r.Target) })
	g.Scorable = slices.DeleteFunc(g.Scorable, func(r kb.ScorableEdge) bool { return match(r.Relation, r.Source, r.Target) })
}

// dropClaimEdges removes a claim's own edges of the class named, or of both:
// the supports and founds edges into it its rests-on list carries, and the
// references edges out of it.
func dropClaimEdges(b *recordBatch, e entry) error {
	id, class := e.str("id"), e.str("edge-class")
	if _, err := b.nodeKind("id", id); err != nil {
		return err
	}
	b.dropEdges(func(relation, source, target string) bool {
		if !ownEdge(id, relation, source, target) {
			return false
		}
		switch class {
		case kb.ListRestsOn:
			return relation != kb.RelationReferences
		case kb.RelationReferences:
			return relation == kb.RelationReferences
		}
		return true
	})
	return nil
}

// dropInboundEdges removes every other node's edge naming a claim.
func dropInboundEdges(b *recordBatch, e entry) error {
	id := e.str("id")
	if _, err := b.nodeKind("id", id); err != nil {
		return err
	}
	b.dropEdges(func(relation, source, target string) bool { return inboundEdge(id, relation, source, target) })
	return nil
}

// refuseRepeatedPairs refuses a resolve-demoted batch naming one pair twice,
// whatever the actions.
func refuseRepeatedPairs(entries []entry) error {
	named := map[[2]string]int{}
	for _, e := range entries {
		id, target := e.str("id"), e.str("target")
		if first, ok := named[[2]string{id, target}]; ok {
			return result.Refusal{{Entry: e.Index, Key: "target", Remedy: "drop one of the two entries, then re-run",
				Detail: fmt.Sprintf("names %s → %s, which entry %d already names; a pair takes one action per batch", id, target, first)}}
		}
		named[[2]string{id, target}] = e.Index
	}
	return nil
}

// resolveDemoted removes the demoted records of the edge from target into id,
// or restores each to the edge it replaced, exactly, refusing a restore that
// would close a cycle. A pair already as asked — no demoted record to remove,
// or the edge standing and none to restore — is left as it stands.
func resolveDemoted(b *recordBatch, e entry) error {
	id, target, action := e.str("id"), e.str("target"), e.str("action")
	for _, end := range [][2]string{{"id", id}, {"target", target}} {
		if _, err := b.nodeKind(end[0], end[1]); err != nil {
			return err
		}
	}
	ends := func(source, into string) bool { return source == target && into == id }
	var demoted []kb.DemotedEdge
	b.g.Demoted = slices.DeleteFunc(b.g.Demoted, func(d kb.DemotedEdge) bool {
		if ends(d.Source, d.Target) {
			demoted = append(demoted, d)
			return true
		}
		return false
	})
	if len(demoted) == 0 {
		if action == actionRemove || slices.ContainsFunc(b.g.Scorable, func(r kb.ScorableEdge) bool { return ends(r.Source, r.Target) }) {
			return nil
		}
		return opRefusal("target="+target, fmt.Sprintf("%s takes no demoted edge from %s, so there is nothing to restore; "+
			"only the build writes a demoted edge", id, target),
			"restore: add the edge with kbase add-rests-on or kbase add-pairs, or correct target, then re-run")
	}
	done := resolvedRemove
	if action == actionRestore {
		done = resolvedRestore
		for _, d := range demoted {
			if b.holds(d.ReplacedRelation, d.Source, d.Target) {
				continue
			}
			if path := b.closesCycle(d.Source, d.Target); path != nil {
				return result.Refusal{{Check: checkDependencyCycle, Entry: e.Index, Key: "target",
					Remedy: "kbase resolve-demoted with action remove for this pair, or remove an edge on the named path first",
					Detail: fmt.Sprintf("restoring %s would close the cycle %s", index.EdgeName(d.ReplacedRelation, d.Source, d.Target),
						strings.Join(append(path, d.Source), " → "))}}
			}
			b.g.Scorable = append(b.g.Scorable, kb.ScorableEdge{Relation: d.ReplacedRelation, Source: d.Source, Target: d.Target,
				Context: d.Context, Bearing: d.Bearing, ManuallyScored: d.ManuallyScored})
		}
	}
	b.resolved = append(b.resolved, Resolution{Source: id, Target: target, Action: done})
	return nil
}

// demoteEdge replaces each standing scoring edge from target into id with the
// demoted record that keeps it whole. Where that record already stands the
// edge standing beside it goes; where only demoted records stand the pair is
// left as it stands, and a pair with neither is refused.
func demoteEdge(b *recordBatch, e entry) error {
	id, target := e.str("id"), e.str("target")
	ends := func(source, into string) bool { return source == target && into == id }
	var standing []kb.ScorableEdge
	b.g.Scorable = slices.DeleteFunc(b.g.Scorable, func(r kb.ScorableEdge) bool {
		if ends(r.Source, r.Target) {
			standing = append(standing, r)
			return true
		}
		return false
	})
	if len(standing) == 0 {
		if slices.ContainsFunc(b.g.Demoted, func(d kb.DemotedEdge) bool { return ends(d.Source, d.Target) }) {
			return nil
		}
		return opRefusal("target="+target, fmt.Sprintf("%s takes no scoring edge from %s, and this op demotes a standing edge; "+
			"only the build writes a demoted edge, and add-rests-on or add-pairs adds the edge", id, target),
			"restore: add the edge, or correct target, then re-run")
	}
	for _, r := range standing {
		if slices.ContainsFunc(b.g.Demoted, func(d kb.DemotedEdge) bool {
			return d.ReplacedRelation == r.Relation && ends(d.Source, d.Target)
		}) {
			continue
		}
		b.g.Demoted = append(b.g.Demoted, kb.DemotedEdge{Source: r.Source, Target: r.Target, Context: r.Context,
			ReplacedRelation: r.Relation, Bearing: r.Bearing, ManuallyScored: r.ManuallyScored})
	}
	return nil
}

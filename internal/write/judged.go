package write

import (
	"kbase/internal/asks"
	"kbase/internal/kb"
)

// scoredRecord is a scored record an entry set a value on: a claim, analysis
// or experiment by id, or a supports, lifts or refutes edge by its relation
// and ends.
type scoredRecord struct{ id, relation, source, target string }

// valueSet records that the entry applying now set r's rigor or bearing,
// pending where it set the pending literal. Once the entry has applied, r
// records the judged hash of what the value was set against (hashValuesSet).
// A value is judged where a caller supplied it, the pending literal included,
// and where a run set a number; a run's pending literal is no judgement — the
// build writes it for score to judge — and leaves r's hash as it stands.
func (b *recordBatch) valueSet(r scoredRecord, pending bool) {
	if pending && b.provenance != ProvenanceCaller {
		return
	}
	b.valued = append(b.valued, r)
}

// rigorSet records the rigor e gives the node id, where it gives one.
func (b *recordBatch) rigorSet(e entry, id string) {
	if e.has("rigor") {
		b.valueSet(scoredRecord{id: id}, e.score("rigor") == nil)
	}
}

// hashValuesSet gives each record valueSet named the judged hash of its text
// as the entry left the batch — a node's statement, an edge's two ends'
// statements — and clears the list. It runs once the entry has applied, so a
// node an add marks in the same entry is hashed over the statement its marker
// gives it.
func (b *recordBatch) hashValuesSet() error {
	valued := b.valued
	b.valued = nil
	for _, r := range valued {
		if r.id != "" {
			if err := b.hashNode(r.id); err != nil {
				return err
			}
			continue
		}
		source, err := b.statement(r.source)
		if err != nil {
			return err
		}
		target, err := b.statement(r.target)
		if err != nil {
			return err
		}
		h, err := asks.EdgeJudgedHash(source, target)
		if err != nil {
			return err
		}
		if e := record(b.g.Scorable, func(e kb.ScorableEdge) bool {
			return e.Relation == r.relation && e.Source == r.source && e.Target == r.target
		}); e != nil {
			e.JudgedHash = h
		}
	}
	return nil
}

// hashNode gives the claim, analysis or experiment record of id the judged
// hash of its statement.
func (b *recordBatch) hashNode(id string) error {
	kind, _ := kb.NodeIDKind(id)
	statement, err := b.statement(id)
	if err != nil {
		return err
	}
	h, err := asks.NodeJudgedHash(kind, statement)
	if err != nil {
		return err
	}
	switch kind {
	case kb.NodeKindClaim:
		if n := record(b.g.Claims, func(n kb.ClaimNode) bool { return n.ID == id }); n != nil {
			n.JudgedHash = h
		}
	case kb.NodeKindAnalysis:
		if n := record(b.g.Analyses, func(n kb.AnalysisNode) bool { return n.ID == id }); n != nil {
			n.JudgedHash = h
		}
	case kb.NodeKindExperiment:
		if n := record(b.g.Experiments, func(n kb.ExperimentNode) bool { return n.ID == id }); n != nil {
			n.JudgedHash = h
		}
	}
	return nil
}

// statement is node id's statement as the batch leaves its host leaf, "" where
// no leaf hosts it — a work, an axiom, a node with no marker.
func (b *recordBatch) statement(id string) (string, error) {
	host, ok := b.hosts[id]
	if !ok {
		return "", nil
	}
	l, opened := b.leaves[host]
	if !opened {
		var err error
		if l, err = b.open(host); err != nil {
			return "", err
		}
	}
	return kb.Statement(l.text, id, b.lifts), nil
}

// lifts is whether the batch's records hold a lifts edge from analysis to
// claim.
func (b *recordBatch) lifts(analysis, claim string) bool {
	return b.holds(kb.RelationLifts, analysis, claim)
}

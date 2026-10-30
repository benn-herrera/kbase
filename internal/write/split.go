package write

import "kbase/internal/kb"

// splitClaim gives a claim an analysis that lifts it and carries its premises:
// the analysis already lifting the claim, else a fresh one marked at the given
// line, the claim's marker moving to byte statement-end of line statement-line
// (its own line where none is given) where statement-end is given. Every
// supports and founds record into the claim, and every demoted record that
// replaced a supports edge into it, then retargets onto the analysis. The lift
// is written pending, and no cycle check runs: a path through the analysis is
// the path that ran through the claim.
func splitClaim(b *recordBatch, e entry) error {
	claim := e.str("id")
	ana := ""
	for _, r := range b.g.Scorable {
		if kind, _ := kb.NodeIDKind(r.Source); r.Relation == kb.RelationLifts && r.Target == claim && kind == kb.NodeKindAnalysis {
			ana = r.Source
			break
		}
	}
	if ana == "" {
		ana = b.mint(kb.AnalysisPrefix)
		b.g.Analyses = append(b.g.Analyses, kb.AnalysisNode{ID: ana, Title: collapse(e.str("title")), Rationale: collapse(e.str("rationale"))})
		if err := b.markAtLine(e.str("document"), e.count("line"), ana); err != nil {
			return err
		}
		if e.has("statement-end") {
			if err := b.moveMarker(e.str("document"), claim, e.count("statement-line"), e.count("statement-end")); err != nil {
				return err
			}
		}
	}
	for i, r := range b.g.Scorable {
		if r.Relation == kb.RelationSupports && r.Target == claim {
			b.g.Scorable[i].Target = ana
		}
	}
	for i, r := range b.g.Founds {
		if r.Target == claim {
			b.g.Founds[i].Target = ana
		}
	}
	for i, r := range b.g.Demoted {
		if r.ReplacedRelation == kb.RelationSupports && r.Target == claim {
			b.g.Demoted[i].Target = ana
		}
	}
	if !b.holds(kb.RelationLifts, ana, claim) {
		b.g.Scorable = append(b.g.Scorable, kb.ScorableEdge{Relation: kb.RelationLifts, Source: ana, Target: claim, Bearing: kb.Bearing{Pending: true}})
	}
	return nil
}

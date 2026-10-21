package asks

// The score asks: one call carries one node or one edge, and answers the faults
// its text commits — one charge line each, or `none`, or `flt-pending`. The
// caller assembles the closed text of that one subject — the renderers that do so
// live with the runner — and hands it in whole, so each template's one dyn. slot
// is all it needs. A node's class decides which rigor template its call renders:
// a claim's text is a statement and an analysis's or an experiment's a method.
// The ids, the weights and the maximum are `kb`'s; a template names the ids its
// class accepts and no number at all.
const (
	scoreRigorStatementTemplate = "score-rigor-statement.tmpl.md"
	scoreRigorMethodTemplate    = "score-rigor-method.tmpl.md"
	scoreBearingTemplate        = "score-bearing.tmpl.md"
)

// ScoreRigorStatement is the ask for one claim's own rigor — the assertion its
// title and statement make, and whether it is fixed, consistent and single-sensed.
// nodeText is the title and statement the caller assembled for the node.
func ScoreRigorStatement(nodeText string) (string, error) {
	return Render(scoreRigorStatementTemplate, map[string]string{"node": nodeText}, nil, nil)
}

// ScoreRigorMethod is the ask for one analysis's or one experiment's own rigor —
// the method its text displays, against what the node sets out to establish.
// nodeText is the title and statement the caller assembled for the node.
func ScoreRigorMethod(nodeText string) (string, error) {
	return Render(scoreRigorMethodTemplate, map[string]string{"node": nodeText}, nil, nil)
}

// ScoreBearing is the ask for one scoring edge's bearing — how much of what the
// source states bears on the target's subject, the source taken as given.
// edgeText is the two statements and the edge's context the caller assembled.
func ScoreBearing(edgeText string) (string, error) {
	return Render(scoreBearingTemplate, map[string]string{"edge": edgeText}, nil, nil)
}

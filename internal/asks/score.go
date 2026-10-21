package asks

// The score asks: one call carries one node or one edge, and answers the faults
// its text commits — one charge line each, or `none`, or `flt-pending`. The
// caller assembles the closed text of that one subject — the renderers that do
// so live with the runner — and hands it in whole, so each template's one dyn.
// slot is all it needs. The ids, the weights and the maximum are `kb`'s; a
// template names the ids its pass accepts and no number at all.
const (
	scoreRigorTemplate   = "score-rigor.tmpl.md"
	scoreBearingTemplate = "score-bearing.tmpl.md"
)

// ScoreRigor is the ask for one node's own rigor — what the node states, and how
// far its own text carries that. nodeText is the title and statement the caller
// assembled for the node.
func ScoreRigor(nodeText string) (string, error) {
	return Render(scoreRigorTemplate, map[string]string{"node": nodeText}, nil, nil)
}

// ScoreBearing is the ask for one scoring edge's bearing — how much of what the
// source states bears on the target's subject, the source taken as given.
// edgeText is the two statements and the edge's context the caller assembled.
func ScoreBearing(edgeText string) (string, error) {
	return Render(scoreBearingTemplate, map[string]string{"edge": edgeText}, nil, nil)
}

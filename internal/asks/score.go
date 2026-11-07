package asks

import "strings"

// The score asks: one call carries one node or one edge, and answers the faults
// its text commits — one charge line each, or `none`. The
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

// The four alternative slots each score template closes on, in order, and the
// fragment each takes on a re-ask: the head, the refusal's case, the quote of
// the reply lines it names, and the close. A first ask fills all four with "".
// They are four slots because composition expands one level: no fragment can
// hold the other three.
const (
	correctionHead       = "correction-head"
	refusalCase          = "refusal-case"
	correctionQuote      = "correction-quote"
	correctionClose      = "correction-close"
	scoreCorrectionHead  = "score-correction-head"
	scoreCorrectionQuote = "score-correction-quote"
	scoreCorrectionClose = "score-correction-close"
	// correctionLines is the quote fragment's slot, the quoted lines joined by
	// newlines.
	correctionLines = "correction-lines"
)

// The refusal cases a score reply's parse names, one per refusal site, each
// stated to the model by its own fragment (refusalFragment).
const (
	RefusalEmptyReply        = "empty-reply"
	RefusalNoneBesideCharges = "none-beside-charges"
	RefusalNotAChargeLine    = "not-a-charge-line"
	RefusalBareNumber        = "bare-number"
	RefusalIDNotOffered      = "id-not-offered"
	RefusalIDWithoutLine     = "id-without-line"
	RefusalFieldNamesNoLine  = "field-names-no-line"
	RefusalLineNotShown      = "line-not-shown"
	RefusalRangeBackwards    = "range-backwards"
	RefusalRangeCrossesParts = "range-crosses-parts"
	RefusalRangeCrossesGap   = "range-crosses-gap"
	RefusalOverlappingRanges = "overlapping-ranges"
)

// refusalCases is every refusal case, in the registry's order, which is the
// order the instruction set renders them in.
var refusalCases = []string{
	RefusalEmptyReply, RefusalNoneBesideCharges, RefusalNotAChargeLine, RefusalBareNumber,
	RefusalIDNotOffered, RefusalIDWithoutLine, RefusalFieldNamesNoLine, RefusalLineNotShown,
	RefusalRangeBackwards, RefusalRangeCrossesParts, RefusalRangeCrossesGap, RefusalOverlappingRanges,
}

// The dyn. slots a refusal case's fragment may declare, by bare name: numbers
// as the reply wrote them, or the fault id it charged. A Correction's Values
// fills exactly the ones its case's fragment declares.
const (
	SlotReplyLine      = "reply-line"
	SlotOtherReplyLine = "other-reply-line"
	SlotID             = "id"
	SlotCitedFrom      = "cited-from"
	SlotCitedTo        = "cited-to"
)

// refusalFragment is the fragment that states the refusal case c.
func refusalFragment(c string) string { return "score-refusal-" + c }

// refusalFragments is every refusal case's fragment, in the registry's order.
func refusalFragments() []string {
	out := make([]string, len(refusalCases))
	for i, c := range refusalCases {
		out[i] = refusalFragment(c)
	}
	return out
}

// Correction is what a score re-ask tells the model of its refused reply: the
// refusal's case, the values its fragment's dyn. slots take, by bare slot name,
// and the reply lines the case names, as written. Lines empty quotes none.
type Correction struct {
	Case   string
	Values map[string]string
	Lines  []string
}

// ScoreRigorStatement is the ask for one claim's own rigor — the assertion its
// title and statement make, and whether it is fixed, consistent and single-sensed.
// nodeText is the title and statement the caller assembled for the node; c is
// the correction a re-ask carries, nil on a first ask.
func ScoreRigorStatement(nodeText string, c *Correction) (string, error) {
	return renderScore(scoreRigorStatementTemplate, "node", nodeText, c)
}

// ScoreRigorMethod is the ask for one analysis's or one experiment's own rigor —
// the method its text displays, against what the node sets out to establish.
// nodeText is the title and statement the caller assembled for the node; c is
// the correction a re-ask carries, nil on a first ask.
func ScoreRigorMethod(nodeText string, c *Correction) (string, error) {
	return renderScore(scoreRigorMethodTemplate, "node", nodeText, c)
}

// ScoreBearing is the ask for one scoring edge's bearing — how much of what the
// source states bears on the target's subject, the source taken as given.
// edgeText is the two statements and the edge's context the caller assembled;
// c is the correction a re-ask carries, nil on a first ask.
func ScoreBearing(edgeText string, c *Correction) (string, error) {
	return renderScore(scoreBearingTemplate, "edge", edgeText, c)
}

// renderScore renders a score template with its subject in slot, closed by the
// correction c: nothing for nil; otherwise the head, c's case, the quote where
// c names lines, and the close.
func renderScore(template, slot, subject string, c *Correction) (string, error) {
	slots := map[string]string{slot: subject}
	picks := map[string]string{correctionHead: "", refusalCase: "", correctionQuote: "", correctionClose: ""}
	if c != nil {
		picks[correctionHead], picks[refusalCase], picks[correctionClose] = scoreCorrectionHead, refusalFragment(c.Case), scoreCorrectionClose
		for k, v := range c.Values {
			slots[k] = v
		}
		if len(c.Lines) > 0 {
			picks[correctionQuote], slots[correctionLines] = scoreCorrectionQuote, strings.Join(c.Lines, "\n")
		}
	}
	return Render(template, slots, nil, picks)
}

// ScoreInstructionSet is every fixed byte the score ask renders can send, for
// an empty subject, in order: the first ask, then for each refusal case in the
// registry's order its re-ask quoting no line and its re-ask quoting one, every
// dyn. value "". Rendering substitutes and never inspects a value, so this
// covers every byte of the class's instructions and nothing an item varies.
func ScoreInstructionSet(ask func(subject string, c *Correction) (string, error)) (string, error) {
	var b strings.Builder
	write := func(c *Correction) error {
		s, err := ask("", c)
		b.WriteString(s)
		return err
	}
	if err := write(nil); err != nil {
		return "", err
	}
	for _, rc := range refusalCases {
		values, err := emptyValues(fragmentFile(refusalFragment(rc)))
		if err != nil {
			return "", err
		}
		for _, lines := range [][]string{nil, {""}} {
			if err := write(&Correction{Case: rc, Values: values, Lines: lines}); err != nil {
				return "", err
			}
		}
	}
	return b.String(), nil
}

// emptyValues is "" for each dyn. slot the fragment source declares, by bare
// name.
func emptyValues(source string) (map[string]string, error) {
	text, err := load(source)
	if err != nil {
		return nil, err
	}
	fields, err := slotsOf(text, source)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, f := range fields {
		if bare, ok := strings.CutPrefix(f, dynamicPrefix); ok {
			values[bare] = ""
		}
	}
	return values, nil
}

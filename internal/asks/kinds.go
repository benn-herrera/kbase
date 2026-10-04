package asks

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/kb"
)

// One letter-to-meaning table per kind. A question offering fewer meanings
// withholds their letters and never relabels the rest. The letters reach a
// template only as composer constants, so the parse and the prompt hold one
// spelling.
const (
	LetterClaim    = "A"
	LetterNotClaim = "B"

	LetterSupportedBy = "A"
	LetterInSupportOf = "B"
	LetterMention     = "C"
)

var letterConstants = map[string]string{
	"letter-claim":         LetterClaim,
	"letter-not-a-claim":   LetterNotClaim,
	"letter-supported-by":  LetterSupportedBy,
	"letter-in-support-of": LetterInSupportOf,
	"letter-mention":       LetterMention,
}

const (
	paragraphTemplate = "paragraph.tmpl.md"
	classifyTemplate  = "classify.tmpl.md"
	overviewTemplate  = "overview-passage.tmpl.md"
)

// returnedMaxChars is how much of an unreadable reply a re-ask shows back.
const returnedMaxChars = 400

// correction is a first ask's empty correction, or a re-ask's carrying what
// came back, cut short.
func correction(returned *string) (slots, alternatives map[string]string) {
	if returned == nil {
		return map[string]string{}, map[string]string{"correction": ""}
	}
	r := []rune(kb.Strip(*returned))
	return map[string]string{"returned": string(r[:min(len(r), returnedMaxChars)])}, map[string]string{"correction": "letter-correction"}
}

// ParagraphItem is one paragraph: its label range, and its labelled lines.
type ParagraphItem struct {
	Paragraph, Text string
}

// ParagraphAsks is one leaf's paragraph asks, the leaf named by its
// kb-root-relative path and shown as its labelled render, each item named by
// its label range and offered both letters.
func ParagraphAsks(document, body string, items []ParagraphItem) []LetterItem {
	return paragraphAsks(load, document, body, items)
}

// paragraphAsks is ParagraphAsks over the templates load reads.
func paragraphAsks(load func(string) (string, error), document, body string, items []ParagraphItem) []LetterItem {
	out := make([]LetterItem, len(items))
	for i, it := range items {
		out[i] = LetterItem{Item: it.Paragraph, Offered: []string{LetterClaim, LetterNotClaim}, Compose: func(returned *string) (string, error) {
			slots, alternatives := correction(returned)
			slots["document"] = document
			slots["body"] = strings.TrimRight(body, "\n")
			slots["paragraph"] = it.Paragraph
			slots["paragraph-text"] = strings.TrimRight(it.Text, "\n")
			return render(load, paragraphTemplate, slots, letterConstants, alternatives)
		}}
	}
	return out
}

// Claim is a claim as an ask names it: its id, title, the kb-root-relative
// document stating it, and where in that document, "" where unknown.
type Claim struct {
	ID, Title, Document, Locator string
}

func (c Claim) line() string {
	where := ""
	if c.Locator != "" {
		where = ": " + c.Locator
	}
	return fmt.Sprintf("- `%s` — %s (stated in `%s`%s)", c.ID, c.Title, c.Document, where)
}

// ClassifyItem is one edge candidate: its target, the target's statement,
// the passages that produced it, and the letters it is offered.
type ClassifyItem struct {
	Target    Claim
	Statement string
	Passages  []string
	Offered   []string
}

// classifyOptions is the closing question by the letters a candidate is
// offered: every offered set holds supported-by and mention.
func classifyOptions(offered []string) (string, error) {
	switch {
	case slices.Equal(offered, []string{LetterSupportedBy, LetterInSupportOf, LetterMention}):
		return "classify-options-three", nil
	case slices.Equal(offered, []string{LetterSupportedBy, LetterMention}):
		return "classify-options-two", nil
	}
	return "", ComposeError{classifyTemplate, fmt.Sprintf("no closing question offers %q", offered)}
}

// ClassifyAsks is one source claim's classify asks, each named by its
// target's id. The group's passages are every item's, sorted, deduplicated
// and numbered P1…, so each ask names its own against one shared list.
func ClassifyAsks(source Claim, statement string, items []ClassifyItem) []LetterItem {
	return classifyAsks(load, source, statement, items)
}

// classifyAsks is ClassifyAsks over the templates load reads.
func classifyAsks(load func(string) (string, error), source Claim, statement string, items []ClassifyItem) []LetterItem {
	var passages []string
	for _, it := range items {
		passages = append(passages, it.Passages...)
	}
	slices.Sort(passages)
	passages = slices.Compact(passages)
	number := map[string]int{}
	lines := make([]string, len(passages))
	for i, p := range passages {
		number[p] = i + 1
		lines[i] = fmt.Sprintf("P%d: %s", i+1, p)
	}
	out := make([]LetterItem, len(items))
	for i, it := range items {
		var offered []string
		for _, l := range []string{LetterSupportedBy, LetterInSupportOf, LetterMention} {
			if slices.Contains(it.Offered, l) {
				offered = append(offered, l)
			}
		}
		var own []int
		for _, p := range it.Passages {
			own = append(own, number[p])
		}
		slices.Sort(own)
		own = slices.Compact(own)
		named := make([]string, len(own))
		for j, n := range own {
			named[j] = fmt.Sprintf("P%d", n)
		}
		out[i] = LetterItem{Item: it.Target.ID, Offered: offered, Compose: func(returned *string) (string, error) {
			options, err := classifyOptions(offered)
			if err != nil {
				return "", err
			}
			slots, alternatives := correction(returned)
			slots["claim-line"] = source.line()
			slots["claim-text"] = strings.TrimRight(statement, "\n")
			slots["reference-lines"] = strings.Join(lines, "\n")
			slots["candidate-line"] = it.Target.line()
			slots["candidate-text"] = strings.TrimRight(it.Statement, "\n")
			slots["candidate-passages"] = strings.Join(named, ", ")
			alternatives["classify-options"] = options
			return render(load, classifyTemplate, slots, letterConstants, alternatives)
		}}
	}
	return out
}

// OverviewPrompt is the overview passage's ask over excerpts: on a re-ask,
// the lines the previous reply was refused for quoted back; none, the
// unchanged ask.
func OverviewPrompt(excerpts string, rejected []string) (string, error) {
	slots := map[string]string{"excerpts": excerpts}
	alternatives := map[string]string{"passage-correction": ""}
	if len(rejected) > 0 {
		slots["rejected-lines"] = strings.Join(rejected, "\n")
		alternatives["passage-correction"] = "overview-correction"
	}
	return Render(overviewTemplate, slots, nil, alternatives)
}

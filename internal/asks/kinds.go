package asks

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// One letter-to-meaning table per kind. A question offering fewer meanings
// withholds their letters and never relabels the rest. The letters reach a
// question file only as composer constants, so the file and the offered set
// hold one spelling.
const (
	LetterClaim    = "A"
	LetterNotClaim = "B"

	LetterSupportedBy = "A"
	LetterInSupportOf = "B"
	LetterMention     = "C"

	LetterPoints       = "A"
	LetterDoesNotPoint = "B"

	LetterSecondAddsNothing = "A"
	LetterFirstAddsNothing  = "B"
	LetterDistinct          = "C"
)

var letterConstants = map[string]string{
	"letter-claim":               LetterClaim,
	"letter-not-a-claim":         LetterNotClaim,
	"letter-supported-by":        LetterSupportedBy,
	"letter-in-support-of":       LetterInSupportOf,
	"letter-mention":             LetterMention,
	"letter-points":              LetterPoints,
	"letter-does-not-point":      LetterDoesNotPoint,
	"letter-second-adds-nothing": LetterSecondAddsNothing,
	"letter-first-adds-nothing":  LetterFirstAddsNothing,
	"letter-distinct":            LetterDistinct,
}

const (
	paragraphTemplate = "paragraph.tmpl.md"
	classifyTemplate  = "classify.tmpl.md"
	unmarkedTemplate  = "unmarked.tmpl.md"
	overviewTemplate  = "overview-passage.tmpl.md"
	colinearTemplate  = "colinear.tmpl.md"

	paragraphQuestion     = "questions/paragraph.question.yaml"
	classifyThreeQuestion = "questions/classify-three.question.yaml"
	classifyTwoQuestion   = "questions/classify-two.question.yaml"
	unmarkedQuestion      = "questions/unmarked.question.yaml"
	colinearQuestion      = "questions/colinear.question.yaml"
)

// Question is one ask's question as the reader puts it to the decision
// model: the type of answer it takes, the offered letter whose criterion a
// noul question's "true" is, the instructions, and the per-option guidance
// keyed by the offered letter on a choice question and by true/false on a
// noul one. Key names the question to the wire; resolve fills it with the
// item's name.
type Question struct {
	Key          string
	Type         string
	TrueLetter   string
	Instructions string
	Criteria     map[string]string
}

// questionFile is a question file's shape on the shelf.
type questionFile struct {
	Type         string            `yaml:"type"`
	TrueLetter   string            `yaml:"true-letter"`
	Instructions string            `yaml:"instructions"`
	Criteria     map[string]string `yaml:"criteria"`
}

// loadQuestion is the named question file's question, its composer constants
// filled and its spelling held to the offered letters: a noul question's
// criteria are keyed true and false and its true-letter is one of the offered
// ones; a choice question's criteria are keyed exactly by the offered
// letters. Anything else is a defect in kbase, refused at load.
func loadQuestion(load func(string) (string, error), name string, offered []string) (Question, error) {
	b, err := load(name)
	if err != nil {
		return Question{}, err
	}
	var f questionFile
	dec := yaml.NewDecoder(strings.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return Question{}, ComposeError{name, fmt.Sprintf("not a question file: %v", err)}
	}
	if f.Type != "noul" && f.Type != "choice" {
		return Question{}, ComposeError{name, fmt.Sprintf("question type %q, want noul or choice", f.Type)}
	}
	instructions, err := fillConstants(f.Instructions, name)
	if err != nil {
		return Question{}, err
	}
	trueLetter, err := fillConstants(f.TrueLetter, name)
	if err != nil {
		return Question{}, err
	}
	criteria := make(map[string]string, len(f.Criteria))
	keys := make([]string, 0, len(f.Criteria))
	for key, guidance := range f.Criteria {
		k, err := fillConstants(key, name)
		if err != nil {
			return Question{}, err
		}
		v, err := fillConstants(guidance, name)
		if err != nil {
			return Question{}, err
		}
		criteria[k] = v
		keys = append(keys, k)
	}
	slices.Sort(keys)
	switch f.Type {
	case "noul":
		if !slices.Equal(keys, []string{"false", "true"}) {
			return Question{}, ComposeError{name, fmt.Sprintf("a noul question's criteria are keyed %q, want \"true\" and \"false\"", keys)}
		}
		if !slices.Contains(offered, trueLetter) {
			return Question{}, ComposeError{name, fmt.Sprintf("its true-letter %q is none of the offered %q", trueLetter, offered)}
		}
	case "choice":
		want := slices.Clone(offered)
		slices.Sort(want)
		if !slices.Equal(keys, want) {
			return Question{}, ComposeError{name, fmt.Sprintf("its criteria are keyed %q, want the offered %q", keys, want)}
		}
	}
	return Question{Type: f.Type, TrueLetter: trueLetter, Instructions: instructions, Criteria: criteria}, nil
}

// fillConstants is text with its composer constants filled; a slot it spells
// that names no constant is a defect in the question file, refused at load.
func fillConstants(text, source string) (string, error) {
	for _, m := range slotRE.FindAllStringSubmatch(text, -1) {
		if _, ok := letterConstants[m[1]]; !ok {
			return "", ComposeError{source, "unfilled slot @!" + m[1] + "!@"}
		}
	}
	return fill(text, letterConstants), nil
}

// Kinds is every multichoice ask kind, in the order a build's stages ask
// them.
var Kinds = []Kind{Paragraph, Unmarked, Colinear, Classify}

// kindFiles is the template kind composes through and the question file it
// asks for the offered letters.
func kindFiles(kind Kind, offered []string) (template, question string, err error) {
	switch kind {
	case Paragraph:
		return paragraphTemplate, paragraphQuestion, nil
	case Unmarked:
		return unmarkedTemplate, unmarkedQuestion, nil
	case Colinear:
		return colinearTemplate, colinearQuestion, nil
	case Classify:
		question, err := classifyQuestionFile(offered)
		return classifyTemplate, question, err
	}
	return "", "", ComposeError{string(kind), "no such multichoice ask kind"}
}

// multichoiceItem is the item of kind named name, offered offered, whose
// state its kind's template renders from slots, asking its kind's question
// for those letters.
func multichoiceItem(kind Kind, name string, offered []string, slots map[string]string) (MultichoiceItem, error) {
	template, file, err := kindFiles(kind, offered)
	if err != nil {
		return MultichoiceItem{}, err
	}
	q, err := loadQuestion(load, file, offered)
	if err != nil {
		return MultichoiceItem{}, err
	}
	return MultichoiceItem{Item: name, Offered: offered, Question: q, Inputs: slots, template: template}, nil
}

// ParagraphItem is one paragraph: its label range, and its labelled lines.
type ParagraphItem struct {
	Paragraph, Text string
}

// ParagraphAsks is one leaf's paragraph asks, the leaf named by its
// kb-root-relative path and shown as its labelled render, each item named by
// its label range and offered both letters.
func ParagraphAsks(document, body string, items []ParagraphItem) []MultichoiceItem {
	offered := []string{LetterClaim, LetterNotClaim}
	out := make([]MultichoiceItem, len(items))
	for i, it := range items {
		item, err := multichoiceItem(Paragraph, it.Paragraph, offered, map[string]string{
			"document":       document,
			"body":           strings.TrimRight(body, "\n"),
			"paragraph":      it.Paragraph,
			"paragraph-text": strings.TrimRight(it.Text, "\n"),
		})
		if err != nil {
			return dyingItems(err)
		}
		out[i] = item
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

// classifyQuestionFile is the classify question file for the letters a
// candidate is offered: every offered set holds supported-by and mention.
func classifyQuestionFile(offered []string) (string, error) {
	switch {
	case slices.Equal(offered, []string{LetterSupportedBy, LetterInSupportOf, LetterMention}):
		return classifyThreeQuestion, nil
	case slices.Equal(offered, []string{LetterSupportedBy, LetterMention}):
		return classifyTwoQuestion, nil
	}
	return "", ComposeError{classifyTemplate, fmt.Sprintf("no question file offers %q", offered)}
}

// ClassifyAsks is one source claim's classify asks, each named by its
// target's id. The group's passages are every item's, sorted, deduplicated
// and numbered P1…, so each ask names its own against one shared list. The
// two-letter and three-letter offered sets ask their own question files.
func ClassifyAsks(source Claim, statement string, items []ClassifyItem) []MultichoiceItem {
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
	out := make([]MultichoiceItem, len(items))
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
		item, err := multichoiceItem(Classify, it.Target.ID, offered, map[string]string{
			"claim-line":         source.line(),
			"claim-text":         strings.TrimRight(statement, "\n"),
			"reference-lines":    strings.Join(lines, "\n"),
			"candidate-line":     it.Target.line(),
			"candidate-text":     strings.TrimRight(it.Statement, "\n"),
			"candidate-passages": strings.Join(named, ", "),
		})
		if err != nil {
			return dyingItems(err)
		}
		out[i] = item
	}
	return out
}

// UnmarkedItem is one shortlisted target, and its statement.
type UnmarkedItem struct {
	Target    Claim
	Statement string
}

// UnmarkedAsks is one source claim's unmarked asks — does the source's own
// text point at the target's result — the source shown with its statement
// and the kb-root-relative document stating it as its labelled render, each
// item named by its target's id and offered both letters.
func UnmarkedAsks(document, body string, source Claim, statement string, items []UnmarkedItem) []MultichoiceItem {
	offered := []string{LetterPoints, LetterDoesNotPoint}
	out := make([]MultichoiceItem, len(items))
	for i, it := range items {
		item, err := multichoiceItem(Unmarked, it.Target.ID, offered, map[string]string{
			"document":       document,
			"body":           strings.TrimRight(body, "\n"),
			"claim-line":     source.line(),
			"claim-text":     strings.TrimRight(statement, "\n"),
			"candidate-line": it.Target.line(),
			"candidate-text": strings.TrimRight(it.Statement, "\n"),
		})
		if err != nil {
			return dyingItems(err)
		}
		out[i] = item
	}
	return out
}

// ColinearItem is one linked pair, its claims in reading order with their
// statements, and the name its ask goes by.
type ColinearItem struct {
	Key                   string
	First, Second         Claim
	FirstText, SecondText string
}

// ColinearAsks is one group's co-linearity asks — does the second claim add
// nothing to the first, the first nothing to the second, or is each a step of
// its own — each item named by its Key and offered all three letters.
func ColinearAsks(items []ColinearItem) []MultichoiceItem {
	offered := []string{LetterSecondAddsNothing, LetterFirstAddsNothing, LetterDistinct}
	out := make([]MultichoiceItem, len(items))
	for i, it := range items {
		item, err := multichoiceItem(Colinear, it.Key, offered, map[string]string{
			"first-line":  it.First.line(),
			"first-text":  strings.TrimRight(it.FirstText, "\n"),
			"second-line": it.Second.line(),
			"second-text": strings.TrimRight(it.SecondText, "\n"),
		})
		if err != nil {
			return dyingItems(err)
		}
		out[i] = item
	}
	return out
}

// dyingItems is what a group of asks is where its question file does not
// load: one item whose every compose returns the load error, so the ask
// fails at its call site — a defect in kbase, never an outcome of a build.
func dyingItems(err error) []MultichoiceItem {
	return []MultichoiceItem{{loadErr: err}}
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

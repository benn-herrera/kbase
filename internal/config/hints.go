package config

import "strings"

// The guidance kbase gives about the models a [models] category wants, in one
// home: the config.toml template's [models] comment block is rendered from
// these rows, and so is the kbase://model-hints resource the MCP server
// offers. Examples are examples — a row names the capability a category wants
// and ids that have served it, and binds no provider surface to a model class.

// modelHint is one row: the category it is about, what the build asks that
// category for, the capability the row wants, ids that have served it, and the
// one note that qualifies it.
type modelHint struct {
	category   string
	does       string
	capability string
	examples   []string
	note       string
}

// modelHints is every row, in the order both renderings state them.
var modelHints = []modelHint{
	{
		category:   CategoryDecision,
		does:       "one letter per ask",
		capability: "a mid-weight reasoning model",
		examples:   []string{"clef-flash", "d3-flash"},
		note:       "the full-weight variants (clef, d3) are not currently recommended; YMMV",
	},
	{
		category:   CategoryText,
		does:       "the overview passage",
		capability: "a ~30B instruct model",
		examples:   []string{"Qwen3.8-27B", "flint", "Ornith-1.5-122B", "Qwen3.8-Flash-Next"},
		note:       "kbase sets passage length; dense logic warrants one of the larger models",
	},
	{
		category:   CategoryScoring,
		does:       "the KB's rigor and bearing, one item per ask",
		capability: "the strongest instruct model on offer that honors its thinking setting (thinking = \"off\", or a thinking budget)",
		examples:   []string{"Ornith-1.5-122B", "Qwen3.8-Flash-Next"},
		note:       "a scoring model may differ from text's; nothing falls back to it",
	},
}

// modelHintsShared and modelHintsVendor hold for every row.
const (
	modelHintsShared = "One model may serve more than one role where the provider accepts its catalogue on " +
		"the decision surface (oMLX does); check decision quality before settling."
	modelHintsVendor = "OpenAI's own /v1/decisions currently accepts gpt-6-luna alone; " +
		"OpenAI-compatible servers decide their own sets, selected by the entry's decisionsFlavor."
)

// hintsCommentWidth is the column the template's comment block wraps at,
// matching the rest of the template.
const hintsCommentWidth = 76

// modelHintsComment is the [models] comment block: a paragraph per row, then
// the notes that hold for every row.
func modelHintsComment() string {
	blocks := make([]string, 0, len(modelHints)+2)
	for _, h := range modelHints {
		text := h.does + " — " + h.capability + " (e.g. " + strings.Join(h.examples, ", ") + ")"
		if h.note != "" {
			text += "; " + h.note
		}
		blocks = append(blocks, hintCommentBlock("# "+h.category+": ", text+"."))
	}
	return strings.Join(append(blocks, hintCommentBlock("# ", modelHintsShared), hintCommentBlock("# ", modelHintsVendor)), "\n")
}

// hintCommentBlock renders text as TOML comment lines whose first line follows
// lead and whose rest align under the text lead introduces.
func hintCommentBlock(lead, text string) string {
	indent := "# " + strings.Repeat(" ", len(lead)-2)
	lines := []string{}
	line := lead
	for _, word := range strings.Fields(text) {
		switch {
		case line == lead:
			line += word
		case len(line)+1+len(word) <= hintsCommentWidth:
			line += " " + word
		default:
			lines = append(lines, line)
			line = indent + word
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// ModelHintsMarkdown is the same guidance as a document: the rows as a table,
// then the notes that hold for every row. It is what the kbase://model-hints
// resource serves.
func ModelHintsMarkdown() string {
	var b strings.Builder
	b.WriteString("# Model hints\n\n")
	b.WriteString("The models kbase's `[models]` categories want. The capability is what to match; " +
		"the ids are examples, not requirements, and no decision surface is bound to a model class.\n\n")
	b.WriteString("| Category | What it does | Capability | Examples | Note |\n|---|---|---|---|---|\n")
	for _, h := range modelHints {
		b.WriteString("| `" + h.category + "` | " + h.does + " | " + h.capability + " | " +
			hintCodeSpans(h.examples) + " | " + h.note + " |\n")
	}
	b.WriteString("\n")
	for _, note := range []string{modelHintsShared, modelHintsVendor} {
		b.WriteString("- " + note + "\n")
	}
	return b.String()
}

// hintCodeSpans renders ids as markdown code spans, comma-separated.
func hintCodeSpans(ids []string) string {
	spans := make([]string, len(ids))
	for i, id := range ids {
		spans[i] = "`" + id + "`"
	}
	return strings.Join(spans, ", ")
}

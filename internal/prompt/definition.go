package prompt

import "strings"

// CriticalWordCap is the slot-8 trailer cap in words (ARCHITECTURE.md §9,
// "CRITICAL section cap"; provisional). It covers the whole trailer — the
// authored `## CRITICAL` text plus the acceptance criteria the builder
// injects — because the trailer's value is that it is short enough to sit in
// the recency zone at well under 1% of a target call. Words are
// whitespace-separated fields, matching how the cap reads to a human author.
const CriticalWordCap = 100

// criticalHeading is the exact heading that opens the section, matched
// case-sensitively at the start of a line. Strict on purpose: `## Critical`
// or an indented variant is an authoring slip, and a near-miss that silently
// produces no trailer is worse than a loud one — with zero sections legal,
// there is no other signal that the section was meant to exist.
const criticalHeading = "## CRITICAL"

// Definition is a parsed agent definition.
type Definition struct {
	// Body is the authored markdown, unmodified. The `## CRITICAL` section
	// is deliberately left in place: slot 2 renders the definition exactly
	// as its author wrote it, and the trailer is the second render of the
	// same text, not a relocation of it (§7 dual-render).
	Body string

	// Critical is the extracted section text, without its heading, for the
	// slot-8 trailer. Empty when the definition has no such section, which
	// is legal — not every stage has a characteristic failure worth a
	// trailer.
	Critical string
}

// ParseDefinition extracts the `## CRITICAL` section from an agent
// definition. The section runs from its heading to the next level-1 or
// level-2 heading, or to end of input; deeper headings stay inside it, so a
// section may have subsections.
//
// Parsing is textual and does not track fenced code blocks: a definition that
// quotes a `## CRITICAL` heading inside an example trips the exactly-one rule
// and fails loudly at validation. That is the intended trade — the failure is
// at dev time, on a file a human is editing, and the alternative is a parser
// that quietly disagrees with the human reading of the file.
func ParseDefinition(md string) (Definition, error) {
	lines := strings.Split(md, "\n")
	var starts []int
	for i, line := range lines {
		if strings.TrimRight(line, " \t\r") == criticalHeading {
			starts = append(starts, i)
		}
	}
	if len(starts) > 1 {
		return Definition{}, ErrMultipleCritical{Count: len(starts)}
	}
	def := Definition{Body: md}
	if len(starts) == 0 {
		return def, nil
	}

	start := starts[0] + 1
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if lvl := headingLevel(lines[i]); lvl == 1 || lvl == 2 {
			end = i
			break
		}
	}
	def.Critical = trimBlock(strings.Join(lines[start:end], "\n"))
	return def, nil
}

// ValidateDefinition is the dev-time gate on an agent definition: it parses,
// checks the trailer cap against the authored section alone, and rejects
// malformed placeholders. The embedded-definition test walks this; the
// builder repeats the cap check at runtime because user-adapted copies never
// pass through here.
func ValidateDefinition(md string) error {
	def, err := ParseDefinition(md)
	if err != nil {
		return err
	}
	if n := wordCount(def.Critical); n > CriticalWordCap {
		return ErrCriticalOverCap{Words: n, Cap: CriticalWordCap}
	}
	if _, err := scanPlaceholders(md); err != nil {
		return err
	}
	return nil
}

// headingLevel returns the ATX heading level of a line, or 0 if the line is
// not a heading.
func headingLevel(line string) int {
	line = strings.TrimRight(line, " \t\r")
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n == len(line) || line[n] == ' ' {
		return n
	}
	return 0
}

// trimBlock strips the blank lines and trailing whitespace around a block.
// Leading whitespace on the first content line is preserved — it may be
// list indentation, which is structure, not padding.
func trimBlock(s string) string {
	return strings.TrimRight(strings.TrimLeft(s, "\n\r"), " \t\n\r")
}

// wordCount counts whitespace-separated fields. Script-agnostic by
// construction: text without spaces (CJK, for instance) counts as one word
// per run, which is what a human eyeballing a 100-word cap also sees.
func wordCount(s string) int { return len(strings.Fields(s)) }

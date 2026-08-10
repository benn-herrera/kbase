package survey

import "strings"

// WordCap bounds every string the artifact copies out of a document. A gist
// is a routing hint — enough for the taxonomy stage to tell a section about
// installation from one about scripting — and every one of them is paid for
// in the whole-corpus artifact that stage reads, so the cap is deliberately
// short.
const WordCap = 40

// CapEllipsis marks a label the cap cut short. Cutting at a word boundary and
// marking the cut are both deliberate: a label truncated mid-word reads as a
// typo, and one truncated invisibly reads as a complete thought that was not.
const CapEllipsis = "…"

// CapWords is the one normalization every copied string in the artifact goes
// through: whitespace runs collapse to a single space (a title wrapped across
// source lines is one line here), and the result is cut to WordCap words with
// the cut marked.
//
// It lives in the neutral package because "capped labels" is a promise the
// artifact makes, not a habit one adapter happens to have — a second adapter
// that capped differently would produce an artifact with different rules
// under the same schema id. Adapters call it; nobody re-implements it.
//
// Titles run through it as well as gists. A title is paid for in the same
// whole-corpus artifact and is bounded by nothing in the source — a Markdown
// setext heading's title is the entire paragraph above the underline, so one
// 300-word paragraph would otherwise buy a 300-word title. Truncating is safe
// because a title is a label; the offsets it labels are untouched.
func CapWords(s string) string {
	words := strings.Fields(s)
	if len(words) > WordCap {
		return strings.Join(words[:WordCap], " ") + CapEllipsis
	}
	return strings.Join(words, " ")
}

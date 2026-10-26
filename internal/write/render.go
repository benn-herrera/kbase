// Package write is the KB's metadata write API: values in, records and
// statement markers out, each placed marker read back before it replaces a
// file.
//
// values checks a values document against an op's closed key vocabulary;
// records applies an op's entries to the records and the leaves it marks,
// reads the markers back and writes what moved; ops gives each op its
// semantics.
package write

import "kbase/internal/kb"

// collapse is the reader's whitespace normalization applied at write time,
// so what is stored is what the reader returns.
func collapse(text string) string { return kb.NormalizeSpace(text) }

// formatScore renders an authored value: nil is the pending literal, a number
// its shortest round-tripping form, so the stored text reads back as the same
// number.
func formatScore(v *float64) string {
	if v == nil {
		return kb.PendingLiteral
	}
	return kb.PyFloatRepr(*v)
}

// bearingOf is a values document's optional bearing as the edge's: an absent
// value is the pending judgement.
func bearingOf(v *float64) kb.Bearing {
	if v == nil {
		return kb.Bearing{Pending: true}
	}
	return kb.Bearing{Value: *v}
}

// pair is one entry of a pair list: the node the edge points at, the author's
// context, and the bearing. A nil score is the pending judgement; Judged is
// whether a values document supplied the score.
type pair struct {
	ID, Context string
	Score       *float64
	Judged      bool
}

// renderCitation is the sanctioned authority-citation form.
func renderCitation(excerpt, kbPath, anchor string) string {
	return `["` + collapse(excerpt) + `"](` + kbPath + "#" + anchor + `)`
}

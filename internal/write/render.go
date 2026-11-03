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

// bearingOf is a values document's optional bearing as the edge's: an absent
// value is the pending judgement.
func bearingOf(v *float64) kb.Bearing {
	if v == nil {
		return kb.Bearing{Pending: true}
	}
	return kb.Bearing{Value: *v}
}

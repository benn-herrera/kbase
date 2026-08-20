// Package text holds the small string utilities more than one package needs
// and none of them owns. Nothing here knows what a survey, a prompt or a
// pipeline is: a utility that needed to would belong to whichever package
// knows.
package text

import "strings"

// CapEllipsis marks a label the cap cut short. Cutting at a word boundary
// and marking the cut are both deliberate: a label truncated mid-word reads
// as a typo, and one truncated invisibly reads as a complete thought that
// was not.
const CapEllipsis = "…"

// CapWords collapses whitespace runs in s to a single space (a title wrapped
// across source lines becomes one line) and cuts the result to at most n
// words, marking the cut with CapEllipsis.
//
// It lives in a neutral package because "capped labels" is a promise the
// caller's own artifact makes, not a habit one adapter happens to have — two
// cappers that rounded differently would produce two sets of rules under one
// schema id. Callers pass their own cap and call this; nobody re-implements
// it.
//
// n is a cap, so it is expected to be positive; every caller passes a named
// constant.
func CapWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		return strings.Join(words[:n], " ") + CapEllipsis
	}
	return strings.Join(words, " ")
}

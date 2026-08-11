package dissect

import (
	"fmt"
	"strings"
	"testing"

	"kbase/internal/survey"
)

// The synthetic corpus for this package's tests.
//
// It builds documents out of labelled blocks and hands back the candidate set
// an adapter would have enumerated for them — block starts, never block
// interiors. That is deliberately NOT a Markdown parse: this package is
// format-neutral, and a fixture that went through an adapter would test the
// adapter's enumeration a second time while making the adversarial shapes
// (a block with no candidates in it at all, a span whose only candidates are
// bunched at one end) hard to write. The real-corpus property test alongside
// it is where actual adapter output is exercised.

// blk is one block of a synthetic document.
type blk struct {
	kind survey.CutKind
	text string
}

// buildDoc joins blocks with a blank line and returns the source together
// with a candidate at the first byte of every block but the first — the same
// contract survey.Assemble enforces: interior, ascending, whitespace-adjacent.
//
// The candidates are offset by one from the blocks: cands[i] is where
// blocks[i+1] starts, and carries that block's kind. A document's first byte
// is not a cut in it.
func buildDoc(blocks ...blk) ([]byte, []survey.CutCandidate) {
	var sb strings.Builder
	var cands []survey.CutCandidate
	for i, b := range blocks {
		if i > 0 {
			sb.WriteString("\n")
			cands = append(cands, survey.CutCandidate{Offset: sb.Len(), Kind: b.kind})
		}
		sb.WriteString(b.text)
		sb.WriteString("\n")
	}
	return []byte(sb.String()), cands
}

// words returns n space-separated words of filler, tagged so a failure
// message says which block it came from.
func words(tag string, n int) string {
	parts := make([]string, 0, n)
	for i := range n {
		parts = append(parts, fmt.Sprintf("%s%02d", tag, i%100))
	}
	return strings.Join(parts, " ")
}

// tokensOf is the estimate the package itself uses, for tests that need to
// size a block against minTokens or a budget.
func tokensOf(s string) int { return estimate([]byte(s)) }

// wholeSpan is the span covering a whole document.
func wholeSpan(src []byte) survey.Range { return survey.Range{Start: 0, End: len(src)} }

// assertVerifies fails the test unless the cut list passes the same Verify
// production runs, under the windows derived from itself.
func assertVerifies(t *testing.T, src []byte, span survey.Range, cands []survey.CutCandidate, cuts []survey.Range) {
	t.Helper()
	if err := Verify(src, span, cands, cuts, Windows(src, cuts)); err != nil {
		t.Fatalf("cut list %+v does not verify: %v", cuts, err)
	}
	// The tiling check is arithmetic; this is the same claim read out of the
	// BYTES, which is what the dissector will actually hand downstream.
	var b strings.Builder
	for _, leaf := range Dissect(src, cuts) {
		b.Write(leaf)
	}
	if got, want := b.String(), string(src[span.Start:span.End]); got != want {
		t.Fatalf("the dissected leaves do not reproduce the span:\n got %q\nwant %q", got, want)
	}
}

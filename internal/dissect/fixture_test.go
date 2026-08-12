package dissect

import (
	"fmt"
	"strconv"
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

// params is the stage parameter value the tests run under: the default
// estimator, plus whatever budget the case is about.
func params(budget int) Params { return Params{BudgetTokens: budget} }

// tokensOf is the estimate the package itself uses, for tests that need to
// size a block against minTokens or a budget.
func tokensOf(s string) int { return params(0).estimate([]byte(s)) }

// wholeSpan is the span covering a whole document.
func wholeSpan(src []byte) survey.Range { return survey.Range{Start: 0, End: len(src)} }

// offsetsOf is every candidate's byte offset — the numbers §5 says never reach
// the model.
func offsetsOf(cands []survey.CutCandidate) []int {
	out := make([]int, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.Offset)
	}
	return out
}

// assertNoOffsets fails unless s renders none of these byte offsets as a
// decimal number.
//
// A plain substring check is the strict reading and it is the right one here:
// the fixtures' filler words are "a00 a01 …", so a multi-digit offset cannot
// collide with document text by accident, and a guard that can only fail on
// the exact string a bug would emit is the guard that catches the bug.
//
// A single-digit offset is not evidence of anything — it is also what a menu
// number, a boundary count and a word of filler look like — so it is skipped
// rather than asserted on. The only one that arises is a span starting at byte
// zero.
func assertNoOffsets(t *testing.T, s string, offsets ...int) {
	t.Helper()
	for _, off := range offsets {
		if off < 10 {
			continue
		}
		if d := fmt.Sprint(off); strings.Contains(s, d) {
			t.Errorf("text renders the raw byte offset %s; the model's vocabulary is menu numbers (§5)", d)
		}
	}
}

// assertNoMenuNumbers fails unless s renders no integer in the MENU's own
// range as a standalone number.
//
// It is assertNoOffsets' sibling and closes the same hole from the other end.
// A corrective note is a prompt, and the model's whole vocabulary at this seam
// is a number in 1..menuCap (§5) — so a note reading "section 3 is under the
// minimum" or "not one of the 5 listed positions" puts a legal answer in front
// of a model that is most likely to be pattern-matching precisely when its
// last answer was rejected. A number outside the range is harmless, which is
// why the minimum's own 64 may stay.
func assertNoMenuNumbers(t *testing.T, s string) {
	t.Helper()
	for _, run := range digitRuns(s) {
		n, err := strconv.Atoi(run)
		if err == nil && n >= 1 && n <= menuCap {
			t.Errorf("text renders %d, which is a legal menu answer: %q", n, s)
		}
	}
}

// digitRuns is every maximal run of digits in s. Maximal, so "64" is one
// number and not a 6 and a 4.
func digitRuns(s string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(s); i++ {
		digit := i < len(s) && s[i] >= '0' && s[i] <= '9'
		switch {
		case digit && start < 0:
			start = i
		case !digit && start >= 0:
			out = append(out, s[start:i])
			start = -1
		}
	}
	return out
}

// assertVerifies fails the test unless the cut list passes the same Verify
// production runs, under the windows derived from itself.
func assertVerifies(t *testing.T, src []byte, span survey.Range, cands []survey.CutCandidate, cuts []survey.Range, p Params) {
	t.Helper()
	if err := Verify(src, span, cands, cuts, Windows(src, cuts, p), p); err != nil {
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

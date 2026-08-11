package markdown

import (
	"bytes"
	"cmp"
	"slices"

	"github.com/yuin/goldmark/ast"

	"kbase/internal/survey"
)

// This file is the adapter's half of the cut-candidate contract
// (ARCHITECTURE.md §5): which byte offsets of a Markdown document are legal
// places to cut it. The neutral half — sorted, unique, interior,
// whitespace-adjacent — is survey.Assemble's, and it is checked over the raw
// custody bytes with no Markdown knowledge at all.
//
// Every candidate this file produces is the first byte of a LINE, which is
// what makes the neutral whitespace-adjacency rule hold here by construction
// rather than by luck: an interior line start is always preceded by the
// newline that ended the line before it. The property is asserted in the
// tests as well, because "by construction" is a claim about code that can be
// changed.
//
// Enumeration is deliberately incomplete where placement is uncertain (an
// empty fence, a heading goldmark records no line for). A candidate set is a
// menu of positions that are legal, never a promise of every position that
// would be: dropping one costs the splitter a choice, while inventing one
// costs the pipeline a cut through the middle of something.

// fenceEdges returns a fenced code block's OUTER boundaries: the first byte
// of its opening fence line, and the first byte after its closing fence line.
// Both are rebased by base into the original file.
//
// goldmark records segments for the block's CONTENT lines only — the fences
// themselves are markers it consumed — so the edges are found by stepping one
// line out from the content in each direction, which is exactly one line in
// both cases. Stepping by line rather than by segment arithmetic is the point:
// a segment's end is a parser detail (does it include the newline?), while
// "the line after this one" means the same thing in every version of it.
//
// A block whose content goldmark did not record (an empty fence) yields
// nothing: an edge that cannot be placed is not enumerated. An unterminated
// block runs to the end of the file, where the closing edge lands on len(body)
// and is dropped later for not being interior.
func fenceEdges(body []byte, base int, n *ast.FencedCodeBlock) []survey.CutCandidate {
	lines := n.Lines()
	if lines.Len() == 0 {
		return nil
	}
	first := lineStart(body, lines.At(0).Start)
	if first == 0 {
		// Content on the file's first line means no opening fence line
		// above it, which is not a shape this block can have. Refusing to
		// guess is cheaper than a candidate inside a code sample.
		return nil
	}
	open := lineStart(body, first-1)
	last := lineStart(body, lines.At(lines.Len()-1).Start)
	// One line past the content is the closing fence; one past that is the
	// first byte the block no longer covers.
	after := nextLine(body, nextLine(body, last))
	return []survey.CutCandidate{
		{Offset: base + open, Kind: survey.CutFence},
		{Offset: base + after, Kind: survey.CutFence},
	}
}

// nextLine returns the offset of the first byte after the line containing at:
// past its newline, or len(body) when nothing terminates it.
func nextLine(body []byte, at int) int {
	if at >= len(body) {
		return len(body)
	}
	if i := bytes.IndexByte(body[at:], '\n'); i >= 0 {
		return at + i + 1
	}
	return len(body)
}

// finalizeCuts turns the walk's raw candidates into the artifact's list:
// sorted, interior to the file, and one entry per offset.
//
// Duplicates are real — a fence's closing edge is often the next paragraph's
// first byte — and the survivor is the strongest kind at that offset
// (CutKind.Rank), because the kind is what a splitter reads when it chooses
// between candidates, and the offset is legal either way.
//
// Sorting a slice with a total order keeps the artifact byte-identical across
// runs; nothing here iterates a map.
func finalizeCuts(cuts []survey.CutCandidate, size int) []survey.CutCandidate {
	slices.SortFunc(cuts, func(a, b survey.CutCandidate) int {
		if a.Offset != b.Offset {
			return cmp.Compare(a.Offset, b.Offset)
		}
		return cmp.Compare(a.Kind.Rank(), b.Kind.Rank())
	})
	out := make([]survey.CutCandidate, 0, len(cuts))
	for _, c := range cuts {
		// Offset 0 and len(src) are the file's own ends, not cuts in it.
		if c.Offset <= 0 || c.Offset >= size {
			continue
		}
		if len(out) > 0 && out[len(out)-1].Offset == c.Offset {
			continue
		}
		out = append(out, c)
	}
	return out
}

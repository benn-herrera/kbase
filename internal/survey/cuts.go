package survey

import "fmt"

// CutKind classifies a legal cut candidate by the structure that makes it
// legal. The set is neutral vocabulary: an adapter maps its own format
// knowledge onto these (a Markdown heading line, a LaTeX \section) and a
// later format may need a kind that does not exist yet, which is a row added
// here rather than a concept invented downstream.
type CutKind string

const (
	// CutHeading is the first byte of a heading line — the strongest
	// boundary a document offers, because the heading names what follows.
	CutHeading CutKind = "heading"
	// CutFence is the outer edge of a verbatim block: the first byte of its
	// opening line, or the first byte after its closing line. Never an
	// offset inside one — a fence cut in half is two broken code samples.
	CutFence CutKind = "fence"
	// CutParagraph is the first byte of a top-level paragraph.
	CutParagraph CutKind = "paragraph"
)

// cutKinds is the single table behind both questions asked of a kind: whether
// it is one of the enumerated kinds at all (membership), and how strong a
// boundary it is (position, strongest first). Two tables would be two places
// for a new kind to be half-added.
var cutKinds = []CutKind{CutHeading, CutFence, CutParagraph}

// Rank reports how strong a boundary the kind is — lower is stronger, so a
// splitter preferring structure sorts by it. An unrecognized kind ranks past
// every known one, which is also what Known reads.
func (k CutKind) Rank() int {
	for i, known := range cutKinds {
		if k == known {
			return i
		}
	}
	return len(cutKinds)
}

// Known reports whether k is one of the enumerated kinds.
func (k CutKind) Known() bool { return k.Rank() < len(cutKinds) }

// CutCandidate is one byte offset a section may legally be cut at, and the
// structure that makes it legal.
//
// The candidates are artifact data rather than something stage 4 recomputes
// (ruled 2026-08-10): refinement's entire input is then reproducible from
// stamped artifacts, and the model's authority is clamped to a set that was
// enumerated by the format adapter and checked by Assemble — bisecting a
// heading is unrepresentable rather than merely detectable (ARCHITECTURE.md
// §5).
type CutCandidate struct {
	Offset int     `json:"offset"`
	Kind   CutKind `json:"kind"`
}

// WhitespaceAdjacent reports whether at least one of the bytes neighbouring
// off is whitespace — the neutral, format-free rule behind the §5 tripwire.
//
// It is one function because it is one rule read twice: Assemble applies it
// to every candidate an adapter enumerates, and stage 4 applies it again to
// every cut a model chose. Both sides asking the same question of the same
// raw bytes is what makes the second reading meaningful — a cut that is a
// legal candidate and still lands between two non-whitespace bytes means the
// two readings were taken over different buffers, which is a defect in our
// offset pipeline and not a judgment call anybody made.
//
// The document's own ends count as boundaries: there is no neighbour there to
// be non-whitespace. Callers that require an interior offset check that
// separately, and say so in their own terms.
func WhitespaceAdjacent(src []byte, off int) bool {
	if off <= 0 || off >= len(src) {
		return true
	}
	return isSpace(src[off-1]) || isSpace(src[off])
}

// isSpace is ASCII whitespace. Byte offsets into UTF-8 text never land a
// multi-byte rune's continuation byte here — every continuation byte has its
// high bit set and none of these do — so a byte test is the whole rule.
func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// verifyCuts checks a file's candidate list against the source under custody:
// every offset interior to the file, strictly ascending (so sorted and
// deduped in one condition), of a kind the artifact knows, and
// whitespace-adjacent.
//
// Assemble runs it in production as well as in tests, for the same reason it
// runs verifyTiling: it is a handful of byte comparisons, and what it catches
// — an adapter computing offsets against the metadata-stripped body without
// rebasing them, say — is otherwise invisible until a leaf is cut in the
// middle of a word several stages later. Failures name the offset, because
// "some candidate is wrong" is not a diagnosis.
func verifyCuts(cuts []CutCandidate, src []byte) error {
	prev := 0
	for i, c := range cuts {
		if !c.Kind.Known() {
			return fmt.Errorf("cut candidate %d at offset %d has unknown kind %q", i, c.Offset, c.Kind)
		}
		if c.Offset <= 0 || c.Offset >= len(src) {
			return fmt.Errorf("cut candidate %d at offset %d is not interior to the %d-byte file",
				i, c.Offset, len(src))
		}
		if c.Offset <= prev {
			return fmt.Errorf("cut candidate %d at offset %d does not follow the previous one at %d "+
				"(candidates must be sorted and unique)", i, c.Offset, prev)
		}
		if !WhitespaceAdjacent(src, c.Offset) {
			return fmt.Errorf("cut candidate %d at offset %d has non-whitespace on both sides (%q)",
				i, c.Offset, src[c.Offset-1:c.Offset+1])
		}
		prev = c.Offset
	}
	return nil
}

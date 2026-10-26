package kb

import (
	"crypto/rand"
	"slices"
	"strings"

	"kbase/internal/formats/format300"
)

// Derived-field placeholders: the identity value of each field refresh
// computes, written where no value is available.
const (
	StrengthPendingLine         = "- strength: " + PendingLiteral
	LeafReferencesPrefix        = "> **Leaf references:**"
	LeafReferencesPendingFooter = LeafReferencesPrefix + " *(none)*"
)

// BuildBand is one rung of the strength ladder; Threshold is its inclusive
// lower bound.
type BuildBand struct {
	Slug         string
	Threshold    float64
	StatusPhrase string
	Label        string
}

// BuildBandLadder is in descending threshold order. Its last rung states an
// outcome and not a cause: every strength under 0.20 lands there from either
// direction — a refuter's cap, or a minimum no premise carries — and what it
// says is that nothing a claim built on this would inherit substantiates the
// claim. The cause, where a reader needs it, is the fault record and the
// derivation trace's own.
var BuildBandLadder = []BuildBand{
	{"ok-to-build", 0.85, "ok to build on", "ok to build on"},
	{"ok-with-caveats", 0.65, "ok to build on, see caveats", "ok to build on, see caveats"},
	{"input-only", 0.45, "use as input only, don't build deeper", "use as input only"},
	{"do-not-build", 0.20, "do not build on, rework needed", "do not build on, rework needed"},
	{"unsubstantiated", 0.00, "unsubstantiated", "unsubstantiated"},
}

// UnknownBandSlug is the band of an unassessed strength.
const UnknownBandSlug = "unknown"

// BandFor is the band a strength falls in, or nil for a pending one. A value
// below the ladder falls to its last rung.
func BandFor(strength *float64) *BuildBand {
	if strength == nil {
		return nil
	}
	for i := range BuildBandLadder {
		if *strength >= BuildBandLadder[i].Threshold {
			return &BuildBandLadder[i]
		}
	}
	return &BuildBandLadder[len(BuildBandLadder)-1]
}

// The id body's alphabet and length, as IDBody spells them.
const (
	HashAlphabet = format300.IDBodyAlphabet
	HashLen      = format300.IDBodyLen
)

// IDPlaceholders are authored placeholder ids, never real nodes: each minted
// kind's prefix over a body of x.
var IDPlaceholders = func() map[string]bool {
	out := map[string]bool{}
	for _, kind := range IDKinds {
		out[kind+"-"+strings.Repeat("x", HashLen)] = true
	}
	return out
}()

// MintID is a fresh id of kind: a random body of HashLen characters drawn
// uniformly from HashAlphabet, re-drawn until taken does not hold it.
func MintID(kind string, taken func(id string) bool) string {
	// The largest multiple of the alphabet's size a byte holds: a byte at or
	// above it is redrawn, so every character is equally likely.
	limit := byte(256 - 256%len(HashAlphabet))
	var b [1]byte
	for {
		body := make([]byte, 0, HashLen)
		for len(body) < HashLen {
			// crypto/rand.Read never returns an error; it aborts the process instead.
			rand.Read(b[:])
			if b[0] < limit {
				body = append(body, HashAlphabet[int(b[0])%len(HashAlphabet)])
			}
		}
		if id := kind + "-" + string(body); !taken(id) {
			return id
		}
	}
}

// LabelName is the name a labelled block's label line carries — letters, in
// words single-spaced — as a pattern fragment: a label line is read only where
// its name is this.
const LabelName = format300.LabelName

// LabelLine is the name line carries where it is a labelled block's label
// line as it is read; ok false for any other line.
func LabelLine(line string) (name string, ok bool) { return format300.LabelLine(line) }

// The document kinds a frontmatter block's kind: names, by path shape alone:
// the entry point, a document with children, one without.
const (
	DocumentEntryPoint = "entry-point"
	DocumentIndex      = "index"
	DocumentLeaf       = "leaf"
)

// DocumentKinds is every document kind.
var DocumentKinds = []string{DocumentLeaf, DocumentIndex, DocumentEntryPoint}

// DocumentKind is the kind of the document at kb-root-relative path p, whose
// child list names a child where hasChildren.
func DocumentKind(p string, hasChildren bool) string {
	switch {
	case p == EntryPointFile:
		return DocumentEntryPoint
	case hasChildren:
		return DocumentIndex
	}
	return DocumentLeaf
}

// WorkKey is the citation key inside a work id, or "" for any other id.
func WorkKey(nodeID string) string {
	prefix, key, _ := strings.Cut(nodeID, "-")
	if prefix != WorkPrefix {
		return ""
	}
	return key
}

// AxiomRigor is an axiom's strength by rule: an axiom has no premises, so its
// strength is its rigor, 1.0, which no authored file carries.
const AxiomRigor = 1.0

// EquationLabel is the \label an equation node's title carries, or "" with
// ok false for any other title.
func EquationLabel(title string) (string, bool) { return format300.EquationLabel(title) }

// EquationLocator is the text that locates the line inside an equation's
// display-maths fence carrying its label, where the equation's statement
// marker goes.
func EquationLocator(label string) string { return format300.EquationLocator(label) }

// DisplayMathInfo is the info string of a displayed equation's fence.
const DisplayMathInfo = format300.DisplayMathInfo

// SheetDigestDrawn is whether a KB whose node rows are these holds the volume
// digest sheet and the per-volume sheets: two or more volumes hold nodes, a
// node's volume being the top-level directory of its canonical path and the
// kb-root-level nodes ("") not a volume (SPEC §3, the sheet rules).
// internal/sheet's drawing decision is the same predicate over the same rows;
// keeping both spellings honest is this function's one
// reader rule and the verify cross-check over a built KB, whose
// README must not link a sheet the KB does not carry.
func SheetDigestDrawn(nodes []NodeRow) bool {
	volumes := map[string]bool{}
	for _, n := range nodes {
		if dir, _, ok := strings.Cut(n.CanonicalPath, "/"); ok {
			volumes[dir] = true
		}
	}
	return len(volumes) >= 2
}

// ScoringRelations is the relations the walk reads, in listing order: the ones
// whose edge carries a bearing.
var ScoringRelations = []string{RelationSupports, RelationFounds, RelationLifts, RelationRefutes}

// Scoring is whether an edge of relation carries a bearing: supports, founds,
// lifts and refutes do; references and demoted stand outside the walk.
func Scoring(relation string) bool { return slices.Contains(ScoringRelations, relation) }

// BearingWalkRelations is the relations a score run's bearing pass asks about,
// in listing order: the edges a model may judge from the KB itself.
var BearingWalkRelations = []string{RelationSupports, RelationLifts, RelationRefutes}

// BearingWalk is whether a score run's bearing pass asks about an edge of
// relation: supports, lifts and refutes do. A founds edge carries a bearing —
// Scoring says so, and the values table prices it — but its source is a work or
// an axiom lying outside the corpus, so a model shown only the KB cannot judge
// the link: that bearing is the user's, written by hand through set-bearing, as
// a work's rigor is through set-rigor.
func BearingWalk(relation string) bool { return slices.Contains(BearingWalkRelations, relation) }

package kb

import (
	"crypto/rand"
	"regexp"
	"slices"
	"strings"
)

// PendingLiteral marks a value as not yet assessed, authored and derived alike.
const PendingLiteral = "*pending*"

// Derived-field placeholders: the identity value of each field refresh
// computes, written where no value is available.
const (
	StrengthPendingLine         = "- strength: " + PendingLiteral
	StrengthAnnotationPending   = "(strength " + PendingLiteral + ")"
	LeafReferencesPrefix        = "> **Leaf references:**"
	LeafReferencesPendingFooter = LeafReferencesPrefix + " *(none — entry has no citing leaf; bidirectional-coverage check will fail)*"
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

// IDKinds are the minted node-id kinds, in the order an alternation keeps.
var IDKinds = []string{"clm", "exp", "ana"}

// IDPlaceholders are authored placeholder ids, never real nodes.
var IDPlaceholders = map[string]bool{"clm-xxxxxx": true, "exp-xxxxxx": true, "ana-xxxxxx": true}

// IDBody is the pattern body of a node id of the given kinds (all minted
// kinds when none are given), with no anchors and no capture group.
func IDBody(kinds ...string) string {
	if len(kinds) == 0 {
		kinds = IDKinds
	}
	var ordered []string
	for _, k := range IDKinds {
		for _, want := range kinds {
			if k == want {
				ordered = append(ordered, k)
				break
			}
		}
	}
	prefix := ordered[0]
	if len(ordered) > 1 {
		prefix = "(?:" + strings.Join(ordered, "|") + ")"
	}
	return prefix + "-[a-z0-9]{6}"
}

// The id body's alphabet and length: every minted id is <kind>-[a-z0-9]{6}.
const (
	HashAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	HashLen      = 6
)

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
const LabelName = `[A-Za-z]+(?: [A-Za-z]+)*`

// labelLineRE is a labelled block's label line, bare or wrapped in the
// identifier the source's \label became.
var labelLineRE = PyRE(`^>\s*(?:<span id="[^"]*">)?\*\*(` + LabelName + `)\*\*(?:</span>)?\s*$`)

// LabelLine is the name line carries where it is a labelled block's label
// line as it is read; ok false for any other line.
func LabelLine(line string) (name string, ok bool) {
	m := labelLineRE.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// The document kinds a frontmatter block's kind: names, by path shape alone:
// the entry point, a document with children, one without.
const (
	DocumentEntryPoint = "entry-point"
	DocumentIndex      = "index"
	DocumentLeaf       = "leaf"
)

// DocumentKinds is every document kind.
var DocumentKinds = []string{DocumentLeaf, DocumentIndex, DocumentEntryPoint}

// WorkPrefix opens every external-work id; the rest of the id is the
// citation key.
const WorkPrefix = "wrk"

// WorkIDPattern is an external-work id: the prefix and a citation key whose
// ends are alphanumeric.
const WorkIDPattern = WorkPrefix + `-[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`

// WorkKey is the citation key inside a work id, or "" for any other id.
func WorkKey(nodeID string) string {
	prefix, key, _ := strings.Cut(nodeID, "-")
	if prefix != WorkPrefix {
		return ""
	}
	return key
}

// AxiomPrefix opens every axiom id; the rest of the id is the label as
// authored.
const AxiomPrefix = "axm-"

// AxiomLabel is an axiom's label as authored: the body of its axm- id and the
// label of its Axiom … citation.
const AxiomLabel = `[A-Za-z0-9][A-Za-z0-9._-]*`

// AxiomRigor is an axiom's strength by rule: an axiom has no premises, so its
// strength is its rigor, 1.0, which no authored file carries.
const AxiomRigor = 1.0

var equationTitleRE = regexp.MustCompile("(?s)^Equation \\(`([^`]+)`\\) — .+$")

// EquationLabel is the \label an equation node's title carries, or "" with
// ok false for any other title.
func EquationLabel(title string) (string, bool) {
	m := equationTitleRE.FindStringSubmatch(title)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// The values a node_type takes.
const (
	NodeKindClaim      = "claim"
	NodeKindAnalysis   = "analysis"
	NodeKindExperiment = "experiment"
	NodeKindAxiom      = "axiom"
	NodeKindWork       = "work"
)

// NodeKinds is every value a node_type takes, in census order.
var NodeKinds = []string{NodeKindClaim, NodeKindAnalysis, NodeKindExperiment, NodeKindAxiom, NodeKindWork}

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

// The relations an edge carries. Source is the supporter, Target the
// supported. Supports and founds compose the min, lifts max above it and
// refutes cap it from above; references and demoted stand outside the walk.
const (
	RelationSupports   = "supports"
	RelationFounds     = "founds"
	RelationLifts      = "lifts"
	RelationRefutes    = "refutes"
	RelationReferences = "references"
	RelationDemoted    = "demoted"
)

// Relations is every relation an edge carries, in listing order.
var Relations = []string{RelationSupports, RelationFounds, RelationLifts, RelationRefutes, RelationReferences, RelationDemoted}

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

// BearingLegal is whether an edge of relation from a source of sourceKind to a
// target of targetKind may carry a bearing: a number in [0, 1] or pending. It
// answers that question only — a pairing it refuses may still be a legal edge
// (a reference), and one it allows promises no edge.
func BearingLegal(relation, sourceKind, targetKind string) bool {
	claimOrAnalysis := targetKind == NodeKindClaim || targetKind == NodeKindAnalysis
	switch relation {
	case RelationSupports:
		return (sourceKind == NodeKindClaim || sourceKind == NodeKindExperiment) && claimOrAnalysis
	case RelationFounds:
		return (sourceKind == NodeKindWork || sourceKind == NodeKindAxiom) && claimOrAnalysis
	case RelationLifts:
		return (sourceKind == NodeKindAnalysis || sourceKind == NodeKindExperiment) && claimOrAnalysis
	case RelationRefutes:
		return (sourceKind == NodeKindClaim || sourceKind == NodeKindAnalysis || sourceKind == NodeKindExperiment) && claimOrAnalysis
	}
	return false
}

// A demoted edge's origin, in the words the claim-graph sheet names an edge's
// provenance with: cited where the text marked the dependency, inferred where
// the build found it unmarked.
const (
	OriginCited    = "cited"
	OriginInferred = "inferred"
)

// Origins is every origin a demoted edge may carry.
var Origins = []string{OriginCited, OriginInferred}

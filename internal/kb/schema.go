package kb

import (
	"crypto/rand"
	"regexp"
	"strings"
)

// PendingLiteral marks a value as not yet assessed, authored and derived alike.
const PendingLiteral = "*pending*"

// Derived-field placeholders: the identity value of each field refresh
// computes, written where no value is available.
const (
	SolidityPendingLine         = "- solidity: " + PendingLiteral
	SolidityAnnotationPending   = "(solidity " + PendingLiteral + ")"
	LeafReferencesPrefix        = "> **Leaf references:**"
	LeafReferencesPendingFooter = LeafReferencesPrefix + " *(none — entry has no citing leaf; bidirectional-coverage check will fail)*"
)

// BuildBand is one rung of the solidity ladder; Threshold is its inclusive
// lower bound.
type BuildBand struct {
	Slug         string
	Threshold    float64
	StatusPhrase string
	Label        string
}

// BuildBandLadder is in descending threshold order.
var BuildBandLadder = []BuildBand{
	{"ok-to-build", 0.85, "ok to build on", "ok to build on"},
	{"ok-with-caveats", 0.65, "ok to build on, see caveats", "ok to build on, see caveats"},
	{"input-only", 0.45, "use as input only, don't build deeper", "use as input only"},
	{"do-not-build", 0.20, "do not build on, rework needed", "do not build on, rework needed"},
	{"refuted", 0.00, "refuted, do not use", "refuted, do not use"},
}

// UnknownBandSlug is the band of an unassessed solidity.
const UnknownBandSlug = "unknown"

// BandFor is the band a solidity falls in, or nil for a pending one. A value
// below the ladder falls to its last rung.
func BandFor(solidity *float64) *BuildBand {
	if solidity == nil {
		return nil
	}
	for i := range BuildBandLadder {
		if *solidity >= BuildBandLadder[i].Threshold {
			return &BuildBandLadder[i]
		}
	}
	return &BuildBandLadder[len(BuildBandLadder)-1]
}

// IDKinds are the minted node-id kinds, in the order an alternation keeps.
var IDKinds = []string{"clm", "exp", "sup"}

// IDPlaceholders are authored placeholder ids, never real nodes.
var IDPlaceholders = map[string]bool{"clm-xxxxxx": true, "exp-xxxxxx": true, "sup-xxxxxx": true}

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
// words single-spaced — as a pattern fragment: kb_tools' reader reads a label
// line only where its name is this.
const LabelName = `[A-Za-z]+(?: [A-Za-z]+)*`

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
const WorkPrefix = "work"

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

// NodeKinds is every value a node_type takes, in census order.
var NodeKinds = []string{"claim", "support", "experiment", "invariant", "axiom", "work"}

// FrameworkKinds are the bedrock kinds: solidity 1.0, no scoring fields.
var FrameworkKinds = []string{"invariant", "axiom"}

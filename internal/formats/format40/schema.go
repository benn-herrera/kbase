package format40

import (
	"encoding/hex"
	"strconv"
	"strings"
)

// PendingLiteral marks a value as not yet assessed, authored and derived alike.
const PendingLiteral = "*pending*"

// RigorKey is the node record field a node's local value is written under.
const RigorKey = "rigor"

// StatusKey is the experiment record field its status is written under.
const StatusKey = "status"

// BearingKey is the edge record field a scoring or founds edge's bearing is
// written under.
const BearingKey = "bearing"

// ManuallyScoredKey is the record field saying whether a person's call wrote
// the record's rigor or bearing.
const ManuallyScoredKey = "manually-scored"

// JudgedHashKey is the record field holding the hash of what a scored record's
// value was last judged against.
const JudgedHashKey = "judged-hash"

// JudgedHash is a SHA-256 digest, written as 64 lowercase hex digits. Its zero
// value, all zeros, is the hash of a value never judged: no computed hash
// matches it.
type JudgedHash [32]byte

// String is h as a record file writes it.
func (h JudgedHash) String() string { return hex.EncodeToString(h[:]) }

// ParseJudgedHash is the hash s spells, ok false where s is not 64 lowercase
// hex digits.
func ParseJudgedHash(s string) (h JudgedHash, ok bool) {
	if len(s) != 2*len(h) || strings.ToLower(s) != s {
		return h, false
	}
	if _, err := hex.Decode(h[:], []byte(s)); err != nil {
		return JudgedHash{}, false
	}
	return h, true
}

// Bearing is one scoring edge's scalar, or the judgement that it has not been
// made yet: a number in [0, 1], or pending. Zero Value is the judgement 0.0;
// whether a bearing was made is Pending's, never Value's.
type Bearing struct {
	Value   float64
	Pending bool
}

// Every node kind's id prefix, each joined to its id's minted body by a
// hyphen; IDBody spells the whole id's pattern, prefix included.
const (
	ClaimPrefix      = "clm"
	AnalysisPrefix   = "ana"
	ExperimentPrefix = "exp"
	WorkPrefix       = "wrk"
	AxiomPrefix      = "axm"
)

// IDKinds are every node kind's id prefix, in the order an alternation keeps.
var IDKinds = []string{ClaimPrefix, ExperimentPrefix, AnalysisPrefix, WorkPrefix, AxiomPrefix}

// HostedIDKinds are the id prefixes of the kinds a statement marker hosts: a
// claim, an experiment and an analysis.
var HostedIDKinds = []string{ClaimPrefix, ExperimentPrefix, AnalysisPrefix}

// The body of a minted id: IDBodyLen characters of IDBodyAlphabet, the
// alphabet IDBody spells as the class a-z0-9.
const (
	IDBodyAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	IDBodyLen      = 6
)

// IDBody is the pattern of a whole node id of the given kinds (every kind
// when none are given), prefix included, with no anchors and no capture
// group.
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
	return prefix + "-[a-z0-9]{" + strconv.Itoa(IDBodyLen) + "}"
}

// WorkLabelPattern is a work's label, its citation key: it opens and closes
// alphanumeric.
const WorkLabelPattern = `[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`

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

package format300

import (
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

// Bearing is one scoring edge's scalar, or the judgement that it has not been
// made yet: a number in [0, 1], or pending. Zero Value is the judgement 0.0;
// whether a bearing was made is Pending's, never Value's.
type Bearing struct {
	Value   float64
	Pending bool
}

// The minted node kinds' id prefixes, each joined to its id's body by a
// hyphen; IDBody spells the whole minted id's pattern, prefix included.
const (
	ClaimPrefix      = "clm"
	AnalysisPrefix   = "ana"
	ExperimentPrefix = "exp"
)

// IDKinds are the minted node-id kinds, in the order an alternation keeps.
var IDKinds = []string{ClaimPrefix, ExperimentPrefix, AnalysisPrefix}

// The body of a minted id: IDBodyLen characters of IDBodyAlphabet, the
// alphabet IDBody spells as the class a-z0-9.
const (
	IDBodyAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	IDBodyLen      = 6
)

// IDBody is the pattern of a whole minted node id of the given kinds (all
// minted kinds when none are given), prefix included, with no anchors and no
// capture group.
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

// WorkPrefix opens every external-work id; after its hyphen, the rest of the
// id is the citation key.
const WorkPrefix = "wrk"

// WorkIDPattern is an external-work id: the prefix and a citation key whose
// ends are alphanumeric.
const WorkIDPattern = WorkPrefix + `-[A-Za-z0-9](?:[A-Za-z0-9_.:+/-]*[A-Za-z0-9])?`

// AxiomPrefix opens every axiom id; after its hyphen, the rest of the id is
// the label as authored.
const AxiomPrefix = "axm"

// AxiomLabel is an axiom's label as authored: the body of its axm- id and the
// label of its Axiom … citation.
const AxiomLabel = `[A-Za-z0-9][A-Za-z0-9._-]*`

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

package format_2_0_0

import (
	"strings"
)

// PendingLiteral marks a value as not yet assessed, authored and derived alike.
const PendingLiteral = "*pending*"

// Derived-field placeholders: the identity value of each field refresh
// computes, written where no value is available.
const (
	LeafReferencesPrefix = "> **Leaf references:**"
)

// IDKinds are the minted node-id kinds, in the order an alternation keeps.
var IDKinds = []string{"clm", "exp", "ana"}

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

// The values a node_type takes.
const (
	NodeKindClaim      = "claim"
	NodeKindAnalysis   = "analysis"
	NodeKindExperiment = "experiment"
	NodeKindAxiom      = "axiom"
	NodeKindWork       = "work"
)

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

package kb

import "kbase/internal/formats/format300"

// IndexFileExt is the extension of every file under .index/.
const IndexFileExt = format300.IndexFileExt

// The rows index.Derive yields from the KB's records, one type per row set.

// NodeRow is one node's row. Which fields a row carries is its kind's.
type NodeRow struct {
	NodeType           string   `json:"node_type"`
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	CanonicalPath      string   `json:"canonical_path"`
	CanonicalAnchor    string   `json:"canonical_anchor"`
	Status             string   `json:"status"`
	Rigor              *float64 `json:"rigor"`
	DerivationStrength *float64 `json:"derivation_strength"`
	Strength           *float64 `json:"strength"`
	BuildStatus        *string  `json:"build_status"`
	BuildBand          string   `json:"build_band"`
	Rationale          string   `json:"rationale"`
	PremiseCount       int      `json:"premise_count"`
	StrengthenByCount  int      `json:"strengthen_by_count"`
	CitationCount      int      `json:"citation_count"`
}

// EdgeRow is one edge's row. Bearing is a number in [0, 1], the
// PendingLiteral, or nil where the relation allows none. ManuallyScored is the
// record's manually-scored flag, nil where its type carries none.
type EdgeRow struct {
	Source         string  `json:"source"`
	Target         string  `json:"target"`
	Relation       string  `json:"relation"`
	TargetKind     string  `json:"target_kind"`
	Bearing        any     `json:"bearing"`
	Context        *string `json:"context"`
	ManuallyScored *bool   `json:"manually_scored"`
}

// StrengthenByRow is one strengthen-by item's row.
type StrengthenByRow struct {
	ClaimID      string   `json:"claim_id"`
	ItemIdx      int      `json:"item_idx"`
	Text         string   `json:"text"`
	MentionedIDs []string `json:"mentioned_ids"`
}

// CiteRow is one citation's row.
type CiteRow struct {
	ClaimID  string `json:"claim_id"`
	LeafPath string `json:"leaf_path"`
}

// AggregateRow is one index node's subtree row.
type AggregateRow struct {
	NodePath           string   `json:"node_path"`
	NodeKind           string   `json:"node_kind"`
	SubtreeClaims      []string `json:"subtree_claims"`
	SubtreeExperiments []string `json:"subtree_experiments"`
}

package kb

import "kbase/internal/formats/format40"

// IndexFileExt is the extension of every file under .index/.
const IndexFileExt = format40.IndexFileExt

// The rows index.Derive yields from the KB's records, one type per row set.

// NodeRow is one node's row. Which fields a row carries is its kind's.
type NodeRow struct {
	NodeType           string
	ID                 string
	Title              string
	Label              string
	CanonicalPath      string
	CanonicalAnchor    string
	Status             string
	Rigor              *float64
	DerivationStrength *float64
	Strength           *float64
	BuildStatus        *string
	StrengthBand       string
	Rationale          string
	PremiseCount       int
	CitationCount      int
}

// EdgeRow is one edge's row. Bearing is a number in [0, 1], the
// PendingLiteral, or nil where the relation allows none. ManuallyScored is the
// record's manually-scored flag, nil where its type carries none.
// ReplacedRelation is a demoted record's, "" on every other.
type EdgeRow struct {
	Source           string
	Target           string
	Relation         string
	TargetKind       string
	Bearing          any
	Context          *string
	ManuallyScored   *bool
	ReplacedRelation string
}

// CiteRow is one citation's row.
type CiteRow struct {
	ClaimID  string
	LeafPath string
}

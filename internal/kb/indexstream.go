package kb

import (
	"encoding/json"
	"errors"
	"strings"
)

// The derived index's files are YAML streams, .index/<name>.yaml: each record
// one physical line, the document marker and the record as a JSON object, a
// YAML flow mapping that a reader holding only a JSON parser reads by
// stripping the marker.
const (
	IndexFileExt      = ".yaml"
	IndexRecordMarker = "--- "
)

// The derived index's files, by name.
const (
	IndexClaims            = "claims"
	IndexDependsOn         = "depends-on"
	IndexStrengthenBy      = "strengthen-by"
	IndexSupportedBy       = "supported-by"
	IndexCites             = "cites"
	IndexSubtreeAggregates = "subtree-aggregates"
)

// IndexFiles is the derived index's file inventory, in emission order.
var IndexFiles = []string{IndexClaims, IndexDependsOn, IndexStrengthenBy, IndexSupportedBy, IndexCites, IndexSubtreeAggregates}

// IndexFileName is the file of the derived index named name.
func IndexFileName(name string) string { return name + IndexFileExt }

// IndexRecordJSON is an index line's record — the JSON object after the
// marker — ok false where the line does not open with the marker.
func IndexRecordJSON(line string) (string, bool) {
	return strings.CutPrefix(line, IndexRecordMarker)
}

// IndexField is one key of an index record and its value: nil, a string, a
// float64, an int, a bool or a []string.
type IndexField struct {
	Key   string
	Value any
}

// The index rows, one type per file. A row's IndexFields are the record the
// writer emits, keys in written order; its json tags are the same keys in the
// same order, as every reader decodes them.

// NodeRow is one claims record. Which fields a record carries is its kind's.
type NodeRow struct {
	NodeType             string   `json:"node_type"`
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	CanonicalPath        string   `json:"canonical_path"`
	CanonicalAnchor      string   `json:"canonical_anchor"`
	Confidence           *float64 `json:"confidence"`
	Quality              *float64 `json:"quality"`
	DerivationSolidity   *float64 `json:"derivation_solidity"`
	ExperimentalSolidity *float64 `json:"experimental_solidity"`
	Solidity             *float64 `json:"solidity"`
	BuildStatus          *string  `json:"build_status"`
	BuildBand            string   `json:"build_band"`
	Rationale            string   `json:"rationale"`
	DependsOnCount       int      `json:"depends_on_count"`
	StrengthenByCount    int      `json:"strengthen_by_count"`
	CitationCount        int      `json:"citation_count"`
	Status               string   `json:"status"`
	Strength             *float64 `json:"strength"`
}

// IndexFields is the record of the row's kind: the identity every node
// carries, then its kind's own fields.
func (r NodeRow) IndexFields() []IndexField {
	out := []IndexField{{"node_type", r.NodeType}, {"id", r.ID}, {"title", r.Title},
		{"canonical_path", r.CanonicalPath}, {"canonical_anchor", r.CanonicalAnchor}}
	switch r.NodeType {
	case "claim":
		return append(out, IndexField{"confidence", optFloat(r.Confidence)}, IndexField{"derivation_solidity", optFloat(r.DerivationSolidity)},
			IndexField{"experimental_solidity", optFloat(r.ExperimentalSolidity)}, IndexField{"solidity", optFloat(r.Solidity)},
			IndexField{"build_status", optString(r.BuildStatus)}, IndexField{"build_band", r.BuildBand},
			IndexField{"rationale", r.Rationale}, IndexField{"depends_on_count", r.DependsOnCount},
			IndexField{"strengthen_by_count", r.StrengthenByCount}, IndexField{"citation_count", r.CitationCount})
	case "experiment":
		return append(out, IndexField{"status", r.Status})
	case "support":
		return append(out, IndexField{"quality", optFloat(r.Quality)}, IndexField{"solidity", optFloat(r.Solidity)})
	case "work":
		return append(out, IndexField{"strength", optFloat(r.Strength)})
	}
	return out
}

// EdgeRow is one depends-on record. Fraction is a float64, PendingLiteral or
// nil; Origin is a demoted row's alone.
type EdgeRow struct {
	Source                 string   `json:"source"`
	Target                 string   `json:"target"`
	Relation               string   `json:"relation"`
	TargetKind             string   `json:"target_kind"`
	TargetSolidityRecorded *float64 `json:"target_solidity_recorded"`
	Strength               *float64 `json:"strength"`
	Context                *string  `json:"context"`
	Fraction               any      `json:"fraction"`
	Origin                 *string  `json:"origin"`
}

// IndexFields is the edge's record; a demoted row closes with its origin.
func (r EdgeRow) IndexFields() []IndexField {
	out := []IndexField{{"source", r.Source}, {"target", r.Target}, {"relation", r.Relation}, {"target_kind", r.TargetKind},
		{"target_solidity_recorded", optFloat(r.TargetSolidityRecorded)}, {"strength", optFloat(r.Strength)},
		{"context", optString(r.Context)}, {"fraction", r.Fraction}}
	if r.Relation == RelationDemoted {
		out = append(out, IndexField{"origin", optString(r.Origin)})
	}
	return out
}

// StrengthenByRow is one strengthen-by record.
type StrengthenByRow struct {
	ClaimID      string   `json:"claim_id"`
	ItemIdx      int      `json:"item_idx"`
	Text         string   `json:"text"`
	MentionedIDs []string `json:"mentioned_ids"`
}

// IndexFields is the item's record.
func (r StrengthenByRow) IndexFields() []IndexField {
	return []IndexField{{"claim_id", r.ClaimID}, {"item_idx", r.ItemIdx}, {"text", r.Text}, {"mentioned_ids", r.MentionedIDs}}
}

// SupportedByRow is one supported-by record; Fraction as EdgeRow's.
type SupportedByRow struct {
	ClaimID     string   `json:"claim_id"`
	SupID       string   `json:"sup_id"`
	Fraction    any      `json:"fraction"`
	SupSolidity *float64 `json:"sup_solidity"`
}

// IndexFields is the pair's record.
func (r SupportedByRow) IndexFields() []IndexField {
	return []IndexField{{"claim_id", r.ClaimID}, {"sup_id", r.SupID}, {"fraction", r.Fraction}, {"sup_solidity", optFloat(r.SupSolidity)}}
}

// CiteRow is one cites record.
type CiteRow struct {
	ClaimID     string `json:"claim_id"`
	LeafPath    string `json:"leaf_path"`
	LeafKind    string `json:"leaf_kind"`
	Tier2Marked bool   `json:"tier2_marked"`
}

// IndexFields is the citation's record.
func (r CiteRow) IndexFields() []IndexField {
	return []IndexField{{"claim_id", r.ClaimID}, {"leaf_path", r.LeafPath}, {"leaf_kind", r.LeafKind}, {"tier2_marked", r.Tier2Marked}}
}

// AggregateRow is one subtree-aggregates record.
type AggregateRow struct {
	NodePath           string   `json:"node_path"`
	NodeKind           string   `json:"node_kind"`
	SubtreeClaims      []string `json:"subtree_claims"`
	SubtreeExperiments []string `json:"subtree_experiments"`
}

// IndexFields is the index node's record.
func (r AggregateRow) IndexFields() []IndexField {
	return []IndexField{{"node_path", r.NodePath}, {"node_kind", r.NodeKind}, {"subtree_claims", r.SubtreeClaims},
		{"subtree_experiments", r.SubtreeExperiments}}
}

func optFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func optString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// IndexRowType is every index row type.
type IndexRowType interface {
	NodeRow | EdgeRow | StrengthenByRow | SupportedByRow | CiteRow | AggregateRow
}

// IndexLineError is a line of an index file that is not the document marker
// and a JSON object its file's row decodes, or whose row accept refused.
type IndexLineError struct {
	Line int
	Err  error
}

var errNotIndexRecord = errors.New("expected the document marker and a JSON object")

// ReadIndex is every row of T's index file as kb_cmd loads it: one per line
// not blank under Python's whitespace, a key the record lacks taking kb_cmd's
// default — a node's node_type "claim", an edge's relation "depends" and
// target_kind "claim". accept, where not nil, sees each decoded row and may
// refuse it. A line that does not decode or that accept refuses is a problem,
// its row dropped; err is the file's read error.
func ReadIndex[T IndexRowType](src *Source, accept func(*T) error) (path string, rows []T, problems []IndexLineError, err error) {
	path = src.KBPath(IndexDir + "/" + IndexFileName(indexFileOf[T]()))
	text, err := src.ReadText(path)
	if err != nil {
		return path, nil, nil, err
	}
	for n, line := range strings.Split(text, "\n") {
		if Strip(line) == "" {
			continue
		}
		record, marked := IndexRecordJSON(line)
		record = Strip(record)
		var row T
		seedIndexRow(&row)
		err := errNotIndexRecord
		if marked && strings.HasPrefix(record, "{") {
			err = json.Unmarshal([]byte(record), &row)
		}
		if err == nil && accept != nil {
			err = accept(&row)
		}
		if err != nil {
			problems = append(problems, IndexLineError{Line: n + 1, Err: err})
			continue
		}
		rows = append(rows, row)
	}
	return path, rows, problems, nil
}

// indexFileOf is the index file whose rows T is.
func indexFileOf[T IndexRowType]() string {
	var row T
	switch any(row).(type) {
	case NodeRow:
		return IndexClaims
	case EdgeRow:
		return IndexDependsOn
	case StrengthenByRow:
		return IndexStrengthenBy
	case SupportedByRow:
		return IndexSupportedBy
	case CiteRow:
		return IndexCites
	}
	return IndexSubtreeAggregates
}

// seedIndexRow sets the keys kb_cmd defaults where a record lacks them.
func seedIndexRow(row any) {
	switch r := row.(type) {
	case *NodeRow:
		r.NodeType = "claim"
	case *EdgeRow:
		r.Relation, r.TargetKind = RelationDepends, "claim"
	}
}

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
	IndexEdges             = "edges"
	IndexStrengthenBy      = "strengthen-by"
	IndexCites             = "cites"
	IndexSubtreeAggregates = "subtree-aggregates"
)

// IndexFiles is the derived index's file inventory, in emission order.
var IndexFiles = []string{IndexClaims, IndexEdges, IndexStrengthenBy, IndexCites, IndexSubtreeAggregates}

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

// IndexFields is the record of the row's kind: the identity every node
// carries, then its kind's own fields.
func (r NodeRow) IndexFields() []IndexField {
	out := []IndexField{{"node_type", r.NodeType}, {"id", r.ID}, {"title", r.Title},
		{"canonical_path", r.CanonicalPath}, {"canonical_anchor", r.CanonicalAnchor}}
	switch r.NodeType {
	case "claim":
		return append(out, IndexField{"rigor", optFloat(r.Rigor)}, IndexField{"derivation_strength", optFloat(r.DerivationStrength)},
			IndexField{"strength", optFloat(r.Strength)},
			IndexField{"build_status", optString(r.BuildStatus)}, IndexField{"build_band", r.BuildBand},
			IndexField{"rationale", r.Rationale}, IndexField{"premise_count", r.PremiseCount},
			IndexField{"strengthen_by_count", r.StrengthenByCount}, IndexField{"citation_count", r.CitationCount})
	case "experiment":
		return append(out, IndexField{"status", r.Status}, IndexField{"rigor", optFloat(r.Rigor)})
	case "analysis":
		return append(out, IndexField{"rigor", optFloat(r.Rigor)}, IndexField{"strength", optFloat(r.Strength)})
	case "work", "axiom":
		return append(out, IndexField{"rigor", optFloat(r.Rigor)})
	}
	return out
}

// EdgeRow is one edges record. Bearing is a number in [0, 1], the
// PendingLiteral, or nil where the relation allows none; Origin is a demoted
// row's alone.
type EdgeRow struct {
	Source                 string   `json:"source"`
	Target                 string   `json:"target"`
	Relation               string   `json:"relation"`
	TargetKind             string   `json:"target_kind"`
	SourceStrengthRecorded *float64 `json:"source_strength_recorded"`
	Bearing                any      `json:"bearing"`
	Context                *string  `json:"context"`
	Origin                 *string  `json:"origin"`
}

// IndexFields is the edge's record; a demoted row closes with its origin.
func (r EdgeRow) IndexFields() []IndexField {
	out := []IndexField{{"source", r.Source}, {"target", r.Target}, {"relation", r.Relation}, {"target_kind", r.TargetKind},
		{"source_strength_recorded", optFloat(r.SourceStrengthRecorded)}, {"bearing", r.Bearing},
		{"context", optString(r.Context)}}
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
	NodeRow | EdgeRow | StrengthenByRow | CiteRow | AggregateRow
}

// IndexLineError is a line of an index file that is not the document marker
// and a JSON object its file's row decodes, or whose row accept refused.
type IndexLineError struct {
	Line int
	Err  error
}

var (
	errNotIndexRecord = errors.New("expected the document marker and a JSON object")
	errNoRelation     = errors.New("edge record carries no relation")
)

// ReadIndex is every row of T's index file: one per line not blank under
// Python's whitespace, a key the record lacks taking its default — a node's
// node_type "claim", an edge's target_kind "claim". An edge's relation has no
// default: a record that omits it is a problem on its line. accept, where not
// nil, sees each decoded row and may refuse it. A line that does not decode,
// that seedIndexRow refuses, or that accept refuses is a problem, its row
// dropped; err is the file's read error.
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
		err := errNotIndexRecord
		if marked && strings.HasPrefix(record, "{") {
			err = json.Unmarshal([]byte(record), &row)
		}
		if err == nil {
			err = seedIndexRow(&row)
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
		return IndexEdges
	case StrengthenByRow:
		return IndexStrengthenBy
	case CiteRow:
		return IndexCites
	}
	return IndexSubtreeAggregates
}

// seedIndexRow fills in the defaults a record takes where it lacks them, and
// refuses an edge record that carries no relation: no default stands in for
// one. It runs after the decode, so a default chosen unconditionally would
// overwrite what the record carried.
func seedIndexRow(row any) error {
	if r, ok := row.(*NodeRow); ok && r.NodeType == "" {
		r.NodeType = "claim"
	}
	if r, ok := row.(*EdgeRow); ok {
		if r.Relation == "" {
			return errNoRelation
		}
		if r.TargetKind == "" {
			r.TargetKind = "claim"
		}
	}
	return nil
}

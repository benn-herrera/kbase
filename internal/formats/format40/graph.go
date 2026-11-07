package format40

import (
	"regexp"
	"slices"
	"strings"
)

// The record files' field names beside RigorKey, StatusKey, BearingKey,
// ManuallyScoredKey and JudgedHashKey.
const (
	fieldID               = "id"
	fieldLabel            = "label"
	fieldTitle            = "title"
	fieldRationale        = "rationale"
	fieldRelation         = "relation"
	fieldSource           = "source"
	fieldTarget           = "target"
	fieldContext          = "context"
	fieldReplacedRelation = "replaced-relation"
)

// nodeFields is each node kind's record fields, in field order.
var nodeFields = map[string][]string{
	NodeKindClaim:      {fieldID, fieldTitle, RigorKey, ManuallyScoredKey, fieldRationale, JudgedHashKey},
	NodeKindAnalysis:   {fieldID, fieldTitle, RigorKey, ManuallyScoredKey, fieldRationale, JudgedHashKey},
	NodeKindExperiment: {fieldID, fieldTitle, StatusKey, RigorKey, ManuallyScoredKey, JudgedHashKey},
	NodeKindWork:       {fieldID, fieldLabel, fieldTitle, RigorKey, fieldRationale},
	NodeKindAxiom:      {fieldID, fieldLabel, fieldTitle},
}

// edgeFields is the record fields of an edge of relation, in field order; nil
// for a relation no edge carries.
func edgeFields(relation string) []string {
	ends := []string{fieldRelation, fieldSource, fieldTarget, fieldContext}
	switch relation {
	case RelationReferences:
		return ends
	case RelationDemoted:
		return append(ends, fieldReplacedRelation, BearingKey, ManuallyScoredKey)
	case RelationFounds:
		return append(ends, BearingKey)
	case RelationSupports, RelationLifts, RelationRefutes:
		return append(ends, BearingKey, ManuallyScoredKey, JudgedHashKey)
	}
	return nil
}

// An experiment's status: whether it has been run.
const (
	ExperimentRun     = "run"
	ExperimentPlanned = "planned"
)

// ExperimentStatuses is every status an experiment carries.
var ExperimentStatuses = []string{ExperimentRun, ExperimentPlanned}

// DemotedReplacedRelations is every relation a demoted record may have
// replaced: the scoring relations that can close a cycle, a founds source
// having no incoming scoring edge.
var DemotedReplacedRelations = []string{RelationSupports, RelationLifts, RelationRefutes}

// A node record's fields. Rigor is nil where it is pending; a JudgedHash left
// unset is the hash of a value never judged.

// ClaimNode is one claim record.
type ClaimNode struct {
	ID, Title      string
	Rigor          *float64
	ManuallyScored bool
	Rationale      string
	JudgedHash     JudgedHash
}

// AnalysisNode is one analysis record.
type AnalysisNode struct {
	ID, Title      string
	Rigor          *float64
	ManuallyScored bool
	Rationale      string
	JudgedHash     JudgedHash
}

// ExperimentNode is one experiment record.
type ExperimentNode struct {
	ID, Title, Status string
	Rigor             *float64
	ManuallyScored    bool
	JudgedHash        JudgedHash
}

// WorkNode is one work record. Label is its citation key.
type WorkNode struct {
	ID, Label, Title string
	Rigor            *float64
	Rationale        string
}

// AxiomNode is one axiom record. Label is any one line of text.
type AxiomNode struct {
	ID, Label, Title string
}

// An edge record's fields. Source is the supporter and Target the supported;
// a type whose relation is fixed does not carry it.

// UnscoredEdge is one references record.
type UnscoredEdge struct {
	Source, Target, Context string
}

// DemotedEdge is one demoted record: the replaced edge whole, its relation in
// ReplacedRelation.
type DemotedEdge struct {
	Source, Target, Context, ReplacedRelation string
	Bearing                                   Bearing
	ManuallyScored                            bool
}

// BearingEdge is one founds record.
type BearingEdge struct {
	Source, Target, Context string
	Bearing                 Bearing
}

// ScorableEdge is one supports, lifts or refutes record.
type ScorableEdge struct {
	Relation, Source, Target, Context string
	Bearing                           Bearing
	ManuallyScored                    bool
	JudgedHash                        JudgedHash
}

// Graph is the KB's records as the strict load reads them, each type in file
// order.
type Graph struct {
	Claims      []ClaimNode
	Analyses    []AnalysisNode
	Experiments []ExperimentNode
	Works       []WorkNode
	Axioms      []AxiomNode
	References  []UnscoredEdge
	Demoted     []DemotedEdge
	Founds      []BearingEdge
	Scorable    []ScorableEdge
}

// EdgeRecord is one edge record of any relation. ReplacedRelation is "" but on
// a demoted record; Bearing is nil on a references record, which carries
// none; ManuallyScored is false and JudgedHash zero where the record's type
// carries no such field.
type EdgeRecord struct {
	Relation, ReplacedRelation, Source, Target, Context string
	Bearing                                             *Bearing
	ManuallyScored                                      bool
	JudgedHash                                          JudgedHash
}

// Edges is every edge record of g in the order EncodeEdges writes them: by
// identity, records of one identity in g's order.
func (g Graph) Edges() []EdgeRecord {
	var out []EdgeRecord
	for _, e := range g.References {
		out = append(out, EdgeRecord{Relation: RelationReferences, Source: e.Source, Target: e.Target, Context: e.Context})
	}
	for _, e := range g.Demoted {
		out = append(out, EdgeRecord{Relation: RelationDemoted, ReplacedRelation: e.ReplacedRelation, Source: e.Source,
			Target: e.Target, Context: e.Context, Bearing: &e.Bearing, ManuallyScored: e.ManuallyScored})
	}
	for _, e := range g.Founds {
		out = append(out, EdgeRecord{Relation: RelationFounds, Source: e.Source, Target: e.Target, Context: e.Context, Bearing: &e.Bearing})
	}
	for _, e := range g.Scorable {
		out = append(out, EdgeRecord{Relation: e.Relation, Source: e.Source, Target: e.Target, Context: e.Context,
			Bearing: &e.Bearing, ManuallyScored: e.ManuallyScored, JudgedHash: e.JudgedHash})
	}
	slices.SortStableFunc(out, func(a, b EdgeRecord) int {
		return slices.Compare(EdgeKey(a.Relation, a.ReplacedRelation, a.Source, a.Target), EdgeKey(b.Relation, b.ReplacedRelation, b.Source, b.Target))
	})
	return out
}

// kindPrefixes is each node kind's id prefix.
var kindPrefixes = map[string]string{
	NodeKindClaim:      ClaimPrefix,
	NodeKindAnalysis:   AnalysisPrefix,
	NodeKindExperiment: ExperimentPrefix,
	NodeKindAxiom:      AxiomPrefix,
	NodeKindWork:       WorkPrefix,
}

// idGrammar is a whole node id of any kind.
var idGrammar = regexp.MustCompile(`^` + IDBody() + `$`)

// NodeIDKind is the kind id's prefix names, and whether id is in the id
// grammar; kind is "" where no kind's prefix opens it.
func NodeIDKind(id string) (kind string, ok bool) {
	for _, k := range NodeKinds {
		if strings.HasPrefix(id, kindPrefixes[k]+"-") {
			return k, idGrammar.MatchString(id)
		}
	}
	return "", false
}

// WorkLabel is a work's label as the strict load reads it.
var WorkLabel = regexp.MustCompile(`^` + WorkLabelPattern + `$`)

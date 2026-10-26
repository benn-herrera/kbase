package kb

import (
	"fmt"

	"kbase/internal/formats/format300"
)

// The KB's two authored record files, kb-root-relative: every node in one,
// every edge in the other.
const (
	GraphNodesFile = format300.NodesFile
	GraphEdgesFile = format300.EdgesFile
)

// The refusal classes the strict load raises: a record file that breaks its
// schema, and one a stamped KB does not carry; and the class of a load that
// completes degraded, its integrity faults resolved for the walk.
const (
	CheckMetadata = format300.CheckMetadata
	CheckIndex    = "index"
	CheckDegraded = "degraded"
)

// RestoreFromGitRemedy is the remedy for a record file a KB no longer carries.
const RestoreFromGitRemedy = "restore it from git"

// LoadGraph is the KB's records, read strictly from its two record files: a
// file that is absent or does not parse, an unknown or missing field, a value
// of the wrong type or outside its domain, an id outside its kind's grammar, a
// node id twice and a pairing BearingLegal refuses are each a GraphFault, and
// any fault refuses the load as one GraphLoadError. A rigor above
// RigorMaximum reads as RigorMaximum.
func LoadGraph(src *Source) (Graph, error) {
	g, err := LoadAuthoredGraph(src)
	if err != nil {
		return Graph{}, err
	}
	for i := range g.Claims {
		g.Claims[i].Rigor = AuthoredRigor(g.Claims[i].Rigor)
	}
	for i := range g.Analyses {
		g.Analyses[i].Rigor = AuthoredRigor(g.Analyses[i].Rigor)
	}
	for i := range g.Experiments {
		g.Experiments[i].Rigor = AuthoredRigor(g.Experiments[i].Rigor)
	}
	for i := range g.Works {
		g.Works[i].Rigor = AuthoredRigor(g.Works[i].Rigor)
	}
	return g, nil
}

// LoadAuthoredGraph is LoadGraph with every rigor as its file holds it, one
// above RigorMaximum included: the records a writer re-renders a record file
// from, so the file keeps a rigor it found.
func LoadAuthoredGraph(src *Source) (Graph, error) {
	var g Graph
	var faults []GraphFault
	for _, file := range []struct {
		rel    string
		decode func(data []byte, rel string, g *Graph) []GraphFault
	}{{GraphNodesFile, format300.DecodeNodes}, {GraphEdgesFile, format300.DecodeEdges}} {
		data, err := src.ReadFile(src.KBPath(file.rel))
		if ErrNotExist(err) {
			faults = append(faults, GraphFault{Check: CheckIndex, Path: file.rel, Remedy: RestoreFromGitRemedy,
				Detail: fmt.Sprintf("%s is missing, and a KB at %s carries it", file.rel, FormatVersion)})
			continue
		}
		if err != nil {
			return Graph{}, err
		}
		faults = append(faults, file.decode(data, file.rel, &g)...)
	}
	if len(faults) > 0 {
		return Graph{}, &GraphLoadError{Faults: faults}
	}
	return g, nil
}

// The live format's definitions — its record types and faults, field names,
// id grammar, node kinds, relations and encoder — are format300's, named here
// under the same names.

type (
	Graph          = format300.Graph
	ClaimNode      = format300.ClaimNode
	AnalysisNode   = format300.AnalysisNode
	ExperimentNode = format300.ExperimentNode
	WorkNode       = format300.WorkNode
	AxiomNode      = format300.AxiomNode
	UnscoredEdge   = format300.UnscoredEdge
	DemotedEdge    = format300.DemotedEdge
	BearingEdge    = format300.BearingEdge
	ScorableEdge   = format300.ScorableEdge
	EdgeRecord     = format300.EdgeRecord
	Bearing        = format300.Bearing
	GraphFault     = format300.GraphFault
	GraphLoadError = format300.GraphLoadError
)

const (
	PendingLiteral    = format300.PendingLiteral
	RigorKey          = format300.RigorKey
	StatusKey         = format300.StatusKey
	BearingKey        = format300.BearingKey
	ManuallyScoredKey = format300.ManuallyScoredKey
	ClaimPrefix       = format300.ClaimPrefix
	AnalysisPrefix    = format300.AnalysisPrefix
	ExperimentPrefix  = format300.ExperimentPrefix
	WorkPrefix        = format300.WorkPrefix
	WorkIDPattern     = format300.WorkIDPattern
	AxiomPrefix       = format300.AxiomPrefix
	AxiomLabel        = format300.AxiomLabel

	NodeKindClaim      = format300.NodeKindClaim
	NodeKindAnalysis   = format300.NodeKindAnalysis
	NodeKindExperiment = format300.NodeKindExperiment
	NodeKindAxiom      = format300.NodeKindAxiom
	NodeKindWork       = format300.NodeKindWork

	RelationSupports   = format300.RelationSupports
	RelationFounds     = format300.RelationFounds
	RelationLifts      = format300.RelationLifts
	RelationRefutes    = format300.RelationRefutes
	RelationReferences = format300.RelationReferences
	RelationDemoted    = format300.RelationDemoted

	ExperimentRun     = format300.ExperimentRun
	ExperimentPending = format300.ExperimentPending
)

var (
	IDKinds            = format300.IDKinds
	NodeKinds          = format300.NodeKinds
	Relations          = format300.Relations
	ExperimentStatuses = format300.ExperimentStatuses
)

func IDBody(kinds ...string) string { return format300.IDBody(kinds...) }

func NodeIDKind(id string) (kind string, ok bool) { return format300.NodeIDKind(id) }

func BearingLegal(relation, sourceKind, targetKind string) bool {
	return format300.BearingLegal(relation, sourceKind, targetKind)
}

func PyFloatRepr(f float64) string { return format300.PyFloatRepr(f) }

func EdgeKey(relation, replaced, source, target string) []string {
	return format300.EdgeKey(relation, replaced, source, target)
}

func EncodeNodes(g Graph) []byte { return format300.EncodeNodes(g) }

func EncodeEdges(g Graph) []byte { return format300.EncodeEdges(g) }

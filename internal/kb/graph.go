package kb

import (
	"fmt"
	"slices"
	"strings"

	"kbase/internal/formats/format40"
)

// The KB's two authored record files, kb-root-relative: every node in one,
// every edge in the other.
const (
	GraphNodesFile = format40.NodesFile
	GraphEdgesFile = format40.EdgesFile
)

// The refusal classes the strict load raises: a record file that breaks its
// schema or holds records that do not form a graph, and one a stamped KB does
// not carry.
const (
	CheckMetadata = format40.CheckMetadata
	CheckIndex    = "index"
)

// RestoreFromGitRemedy is the remedy for a record file a KB no longer carries.
const RestoreFromGitRemedy = "restore it from git"

// ConsistentRecordsRemedy is the remedy for records no write op produces —
// one edge identity on two records, an edge naming no node, a cycle among the
// scoring edges — which kbase does not resolve: the recovery is in git.
const ConsistentRecordsRemedy = "return the record files to a consistent commit in git, such as the one before a merge, and reapply the other side's changes through write ops"

// LoadGraph is the KB's records, read strictly from its two record files: a
// file that is absent or does not parse, an unknown or missing field, a value
// of the wrong type or outside its domain — a rigor above RigorMaximum
// included — an id outside its kind's grammar, a node id twice, a pairing
// BearingLegal refuses, and, where the files decode clean, one edge identity
// on two records and an edge naming a node no record carries are each a
// GraphFault, and any fault refuses the load as one GraphLoadError. A cycle
// among the scoring edges is the index's to find.
func LoadGraph(src *Source) (Graph, error) {
	var g Graph
	var faults []GraphFault
	for _, file := range []struct {
		rel    string
		decode func(data []byte, rel string, g *Graph) []GraphFault
	}{{GraphNodesFile, format40.DecodeNodes}, {GraphEdgesFile, format40.DecodeEdges}} {
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
		if file.rel == GraphNodesFile {
			faults = append(faults, rigorAboveMaximum(g)...)
		}
	}
	if len(faults) == 0 {
		faults = edgeIntegrity(g)
	}
	if len(faults) > 0 {
		return Graph{}, &GraphLoadError{Faults: faults}
	}
	return g, nil
}

// edgeIntegrity is a fault per edge record of g that names a node no record
// carries, and per record after the first of one edge identity, in identity
// order.
func edgeIntegrity(g Graph) []GraphFault {
	nodes := map[string]bool{}
	for _, n := range g.Claims {
		nodes[n.ID] = true
	}
	for _, n := range g.Analyses {
		nodes[n.ID] = true
	}
	for _, n := range g.Experiments {
		nodes[n.ID] = true
	}
	for _, n := range g.Works {
		nodes[n.ID] = true
	}
	for _, n := range g.Axioms {
		nodes[n.ID] = true
	}
	var faults []GraphFault
	fault := func(format string, args ...any) {
		faults = append(faults, GraphFault{Check: CheckMetadata, Path: GraphEdgesFile, Remedy: ConsistentRecordsRemedy,
			Detail: fmt.Sprintf(format, args...)})
	}
	var previous []string
	for _, e := range g.Edges() {
		relation := e.Relation
		if e.ReplacedRelation != "" {
			relation += " " + e.ReplacedRelation
		}
		name := relation + " " + e.Source + " → " + e.Target
		var missing []string
		for _, id := range []string{e.Source, e.Target} {
			if !nodes[id] && !slices.Contains(missing, id) {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			fault("the edge %s names %s, which no node record carries", name, strings.Join(missing, " and "))
		}
		key := EdgeKey(e.Relation, e.ReplacedRelation, e.Source, e.Target)
		if slices.Equal(key, previous) {
			fault("the edge %s is on more than one record, and one edge identity is one record", name)
		}
		previous = key
	}
	return faults
}

// rigorAboveMaximum is a fault per node record whose rigor stands above
// RigorMaximum: the decoder's rigor domain has no top, and the scale's is
// RigorMaximum.
func rigorAboveMaximum(g Graph) []GraphFault {
	var faults []GraphFault
	check := func(id string, rigor *float64) {
		if rigor != nil && *rigor > RigorMaximum {
			faults = append(faults, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Key: RigorKey,
				Remedy: "kbase never writes this value; whatever wrote it is the way to undo it. No kbase op loads this KB until then.",
				Detail: fmt.Sprintf("%s's %s %s is outside its domain: above the maximum, %s", id, RigorKey, PyFloatRepr(*rigor), PyFloatRepr(RigorMaximum))})
		}
	}
	for _, n := range g.Claims {
		check(n.ID, n.Rigor)
	}
	for _, n := range g.Analyses {
		check(n.ID, n.Rigor)
	}
	for _, n := range g.Experiments {
		check(n.ID, n.Rigor)
	}
	for _, n := range g.Works {
		check(n.ID, n.Rigor)
	}
	return faults
}

// The live format's definitions — its record types and faults, field names,
// id grammar, node kinds, relations and encoder — are format40's, named here
// under the same names.

type (
	Graph          = format40.Graph
	ClaimNode      = format40.ClaimNode
	AnalysisNode   = format40.AnalysisNode
	ExperimentNode = format40.ExperimentNode
	WorkNode       = format40.WorkNode
	AxiomNode      = format40.AxiomNode
	UnscoredEdge   = format40.UnscoredEdge
	DemotedEdge    = format40.DemotedEdge
	BearingEdge    = format40.BearingEdge
	ScorableEdge   = format40.ScorableEdge
	EdgeRecord     = format40.EdgeRecord
	Bearing        = format40.Bearing
	JudgedHash     = format40.JudgedHash
	GraphFault     = format40.GraphFault
	GraphLoadError = format40.GraphLoadError
)

const (
	PendingLiteral    = format40.PendingLiteral
	RigorKey          = format40.RigorKey
	StatusKey         = format40.StatusKey
	BearingKey        = format40.BearingKey
	ManuallyScoredKey = format40.ManuallyScoredKey
	JudgedHashKey     = format40.JudgedHashKey
	ClaimPrefix       = format40.ClaimPrefix
	AnalysisPrefix    = format40.AnalysisPrefix
	ExperimentPrefix  = format40.ExperimentPrefix
	WorkPrefix        = format40.WorkPrefix
	AxiomPrefix       = format40.AxiomPrefix
	WorkLabelPattern  = format40.WorkLabelPattern

	NodeKindClaim      = format40.NodeKindClaim
	NodeKindAnalysis   = format40.NodeKindAnalysis
	NodeKindExperiment = format40.NodeKindExperiment
	NodeKindAxiom      = format40.NodeKindAxiom
	NodeKindWork       = format40.NodeKindWork

	RelationSupports   = format40.RelationSupports
	RelationFounds     = format40.RelationFounds
	RelationLifts      = format40.RelationLifts
	RelationRefutes    = format40.RelationRefutes
	RelationReferences = format40.RelationReferences
	RelationDemoted    = format40.RelationDemoted

	ExperimentRun     = format40.ExperimentRun
	ExperimentPlanned = format40.ExperimentPlanned
)

var (
	IDKinds            = format40.IDKinds
	HostedIDKinds      = format40.HostedIDKinds
	WorkLabel          = format40.WorkLabel
	NodeKinds          = format40.NodeKinds
	Relations          = format40.Relations
	ExperimentStatuses = format40.ExperimentStatuses
)

// KindWithArticle is the node kind as prose names one of it.
func KindWithArticle(kind string) string { return kindArticles[kind] }

var kindArticles = map[string]string{
	NodeKindClaim:      "a claim",
	NodeKindAnalysis:   "an analysis",
	NodeKindExperiment: "an experiment",
	NodeKindAxiom:      "an axiom",
	NodeKindWork:       "a work",
}

func IDBody(kinds ...string) string { return format40.IDBody(kinds...) }

func NodeIDKind(id string) (kind string, ok bool) { return format40.NodeIDKind(id) }

func BearingLegal(relation, sourceKind, targetKind string) bool {
	return format40.BearingLegal(relation, sourceKind, targetKind)
}

func PyFloatRepr(f float64) string { return format40.PyFloatRepr(f) }

func EdgeKey(relation, replaced, source, target string) []string {
	return format40.EdgeKey(relation, replaced, source, target)
}

func EncodeNodes(g Graph) []byte { return format40.EncodeNodes(g) }

func EncodeEdges(g Graph) []byte { return format40.EncodeEdges(g) }

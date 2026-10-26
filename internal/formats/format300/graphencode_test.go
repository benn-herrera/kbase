package format300

import (
	"reflect"
	"testing"
)

func f64(v float64) *float64 { return &v }

// TestRecordFilesRoundTrip: the strict decoder reads back exactly the records
// the encoder wrote, with no fault — every record type and field shape, a
// pending value beside a judged 0, a rigor above the rigor maximum as a file
// may hold it, an empty list beside a full one, a demoted pair's two records,
// and strings carrying what the encoder escapes. The records are given in the
// order the files keep, so the decoded graph compares whole.
func TestRecordFilesRoundTrip(t *testing.T) {
	escaped := "a \"quoted\" \\ back\\slash, a\ttab, \u0001, \u007f, \u0085, é and " + PendingLiteral
	for _, r := range []rune{0x2028, 0x2029, 0xfeff, 0xfffe, 0xffff} {
		escaped += ", " + string(r)
	}
	pending := Bearing{Pending: true}
	g := Graph{
		Claims: []ClaimNode{
			{ID: "clm-aaaaaa", Title: escaped, Rigor: f64(0), ManuallyScored: true, Rationale: escaped, NoEdge: "",
				StrengthenBy: []string{"Tighten the constant.", escaped}},
			{ID: "clm-bbbbbb", Title: "B", Rigor: nil, Rationale: "", NoEdge: "Stands alone."},
			{ID: "clm-cccccc", Title: "C", Rigor: f64(1.25), Rationale: "Above the maximum."},
		},
		Analyses: []AnalysisNode{
			{ID: "ana-aaaaaa", Title: "An", Rigor: f64(0.5), ManuallyScored: true, Rationale: "R", NoEdge: escaped},
		},
		Experiments: []ExperimentNode{
			{ID: "exp-aaaaaa", Title: "Ran", Status: "run", Rigor: f64(1), ManuallyScored: true},
			{ID: "exp-bbbbbb", Title: "Waits", Status: "pending"},
		},
		Works:  []WorkNode{{ID: "wrk-Smith2020:b/c+d", Title: "Smith (2020)", Rigor: f64(0.75), Rationale: escaped}},
		Axioms: []AxiomNode{{ID: "axm-S1.b", Title: "Closure"}},
		References: []UnscoredEdge{
			{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Context: escaped},
			{Source: "clm-bbbbbb", Target: "clm-cccccc"},
		},
		Demoted: []DemotedEdge{
			{Source: "ana-aaaaaa", Target: "clm-cccccc", Context: "a cut", ReplacedRelation: RelationLifts, Bearing: Bearing{Value: 0.3}, ManuallyScored: true},
			{Source: "ana-aaaaaa", Target: "clm-cccccc", ReplacedRelation: RelationRefutes, Bearing: pending},
		},
		Founds: []BearingEdge{
			{Source: "axm-S1.b", Target: "clm-aaaaaa", Bearing: Bearing{Value: 1}},
			{Source: "wrk-Smith2020:b/c+d", Target: "ana-aaaaaa", Context: escaped, Bearing: pending},
		},
		Scorable: []ScorableEdge{
			{Relation: RelationSupports, Source: "clm-bbbbbb", Target: "clm-aaaaaa", Bearing: Bearing{Value: 0}, ManuallyScored: true},
			{Relation: RelationSupports, Source: "exp-aaaaaa", Target: "clm-aaaaaa", Context: escaped, Bearing: pending},
			{Relation: RelationLifts, Source: "exp-aaaaaa", Target: "ana-aaaaaa", Bearing: Bearing{Value: 0.25}},
			{Relation: RelationRefutes, Source: "clm-cccccc", Target: "clm-bbbbbb", Bearing: Bearing{Value: 0.1}, ManuallyScored: true},
		},
	}
	var got Graph
	faults := append(DecodeNodes(EncodeNodes(g), NodesFile, &got), DecodeEdges(EncodeEdges(g), EdgesFile, &got)...)
	if len(faults) > 0 {
		t.Fatalf("the encoder's own output decodes with faults: %+v", faults)
	}
	if !reflect.DeepEqual(got, g) {
		t.Errorf("decoded\n%+v\nwant\n%+v", got, g)
	}
}

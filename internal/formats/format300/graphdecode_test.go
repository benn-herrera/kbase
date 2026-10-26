package format300

import "testing"

// TestDecoderRefusesWhatNoReaderMayGuess: a scoring edge with no bearing — the
// zero-value hazard, which a lenient reader takes for the judgement 0 — a
// bearing on a references record, and an infinite rigor, which the encoder
// would write back as a plain scalar the next load reads as a string.
// Each is one fault naming its file, line and field.
func TestDecoderRefusesWhatNoReaderMayGuess(t *testing.T) {
	claim := func(rigor string) string {
		return "- id: \"clm-aaaaaa\"\n  title: \"T\"\n  rigor: " + rigor +
			"\n  manually-scored: false\n  rationale: \"\"\n  no-edge: \"\"\n  strengthen-by: []\n"
	}
	edge := func(relation, rest string) string {
		return "- relation: \"" + relation + "\"\n  source: \"clm-aaaaaa\"\n  target: \"clm-bbbbbb\"\n  context: \"\"\n" + rest
	}
	for _, c := range []struct {
		name, nodes, edges string
		want               GraphFault
	}{
		{name: "a scoring edge with no bearing", edges: edge(RelationSupports, "  manually-scored: false\n"),
			want: GraphFault{Path: EdgesFile, Line: 1, Key: BearingKey}},
		{name: "a bearing on a references record", edges: edge(RelationReferences, "  bearing: 0.5\n"),
			want: GraphFault{Path: EdgesFile, Line: 5, Key: BearingKey, Allowed: "relation, source, target, context"}},
		{name: "rigor .inf", nodes: claim(".inf"), want: GraphFault{Path: NodesFile, Line: 3, Key: RigorKey, Allowed: scoreDomain}},
		{name: "rigor -.inf", nodes: claim("-.inf"), want: GraphFault{Path: NodesFile, Line: 3, Key: RigorKey, Allowed: scoreDomain}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var g Graph
			var faults []GraphFault
			if c.nodes != "" {
				faults = append(faults, DecodeNodes([]byte(c.nodes), NodesFile, &g)...)
			}
			if c.edges != "" {
				faults = append(faults, DecodeEdges([]byte(c.edges), EdgesFile, &g)...)
			}
			if len(faults) != 1 {
				t.Fatalf("faults %+v, want one", faults)
			}
			got := faults[0]
			if got.Detail == "" {
				t.Errorf("fault %+v carries no detail", got)
			}
			got.Detail = ""
			want := c.want
			want.Check = CheckMetadata
			if got != want {
				t.Errorf("fault = %+v, want %+v", got, want)
			}
		})
	}
}

package format40

import (
	"reflect"
	"strings"
	"testing"
)

// TestDecoderRefusesWhatNoReaderMayGuess: a scoring edge with no bearing — the
// zero-value hazard, which a lenient reader takes for the judgement 0 — a
// bearing on a references record, an infinite rigor, which the encoder
// would write back as a plain scalar the next load reads as a string, a
// scored record with no judged hash, which a lenient reader takes for a value
// never judged, a judged hash that is no SHA-256 spelling, and an experiment
// status outside the two. Each is one fault naming its file, line and field.
func TestDecoderRefusesWhatNoReaderMayGuess(t *testing.T) {
	zeros := `"` + strings.Repeat("0", 64) + `"`
	claim := func(rigor, hash string) string {
		out := "- id: \"clm-aaaaaa\"\n  title: \"T\"\n  rigor: " + rigor + "\n  manually-scored: false\n  rationale: \"\"\n"
		if hash != "" {
			out += "  judged-hash: " + hash + "\n"
		}
		return out
	}
	experiment := func(status string) string {
		return "- id: \"exp-aaaaaa\"\n  title: \"E\"\n  status: \"" + status + "\"\n  rigor: \"*pending*\"\n" +
			"  manually-scored: false\n  judged-hash: " + zeros + "\n"
	}
	edge := func(relation, rest string) string {
		return "- relation: \"" + relation + "\"\n  source: \"clm-aaaaaa\"\n  target: \"clm-bbbbbb\"\n  context: \"\"\n" + rest
	}
	for _, c := range []struct {
		name, nodes, edges string
		want               GraphFault
	}{
		{name: "a scoring edge with no bearing", edges: edge(RelationSupports, "  manually-scored: false\n  judged-hash: "+zeros+"\n"),
			want: GraphFault{Path: EdgesFile, Line: 1, Key: BearingKey}},
		{name: "a bearing on a references record", edges: edge(RelationReferences, "  bearing: 0.5\n"),
			want: GraphFault{Path: EdgesFile, Line: 5, Key: BearingKey, Allowed: []string{"relation", "source", "target", "context"}}},
		{name: "rigor .inf", nodes: claim(".inf", zeros), want: GraphFault{Path: NodesFile, Line: 3, Key: RigorKey}},
		{name: "rigor -.inf", nodes: claim("-.inf", zeros), want: GraphFault{Path: NodesFile, Line: 3, Key: RigorKey}},
		{name: "a claim with no judged hash", nodes: claim("0.5", ""), want: GraphFault{Path: NodesFile, Line: 1, Key: JudgedHashKey}},
		{name: "a scoring edge with no judged hash", edges: edge(RelationRefutes, "  bearing: 0.5\n  manually-scored: false\n"),
			want: GraphFault{Path: EdgesFile, Line: 1, Key: JudgedHashKey}},
		{name: "a judged hash in capitals", nodes: claim("0.5", `"`+strings.Repeat("A", 64)+`"`),
			want: GraphFault{Path: NodesFile, Line: 6, Key: JudgedHashKey}},
		{name: "a judged hash one digit short", nodes: claim("0.5", `"`+strings.Repeat("0", 63)+`"`),
			want: GraphFault{Path: NodesFile, Line: 6, Key: JudgedHashKey}},
		{name: "a 3.0.0 experiment status", nodes: experiment("pending"),
			want: GraphFault{Path: NodesFile, Line: 3, Key: StatusKey, Allowed: ExperimentStatuses}},
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
			if !reflect.DeepEqual(got, want) {
				t.Errorf("fault = %+v, want %+v", got, want)
			}
		})
	}
}

// TestDecoderRefusesThe300Fields: a 3.0.0 claim record, carrying no-edge and
// strengthen-by and no judged hash, is refused field by field: 4.0.0 has no
// place for either field.
func TestDecoderRefusesThe300Fields(t *testing.T) {
	nodes := "- id: \"clm-aaaaaa\"\n  title: \"T\"\n  rigor: 0.5\n  manually-scored: false\n  rationale: \"\"\n" +
		"  no-edge: \"\"\n  strengthen-by: []\n"
	var g Graph
	var keys []string
	for _, f := range DecodeNodes([]byte(nodes), NodesFile, &g) {
		keys = append(keys, f.Key)
	}
	if want := []string{"no-edge", "strengthen-by", JudgedHashKey}; !reflect.DeepEqual(keys, want) {
		t.Errorf("faults name %q, want %q", keys, want)
	}
}

// TestParseJudgedHash: 64 lowercase hex digits read back as the hash they
// spell, and the zero hash spells all zeros.
func TestParseJudgedHash(t *testing.T) {
	if got := (JudgedHash{}).String(); got != strings.Repeat("0", 64) {
		t.Errorf("the zero hash spells %q", got)
	}
	var h JudgedHash
	for i := range h {
		h[i] = byte(i * 7)
	}
	if got, ok := ParseJudgedHash(h.String()); !ok || got != h {
		t.Errorf("ParseJudgedHash(%q) = %x, %t", h.String(), got, ok)
	}
	for _, bad := range []string{"", strings.Repeat("0", 63), strings.Repeat("0", 65), strings.Repeat("g", 64), strings.Repeat("F", 64)} {
		if _, ok := ParseJudgedHash(bad); ok {
			t.Errorf("ParseJudgedHash(%q) accepted it", bad)
		}
	}
}

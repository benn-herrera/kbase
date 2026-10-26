package kb

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// loadGraphFiles runs LoadGraph over a KB holding the record files given; a
// nil file is absent.
func loadGraphFiles(t *testing.T, nodes, edges *string) (Graph, error) {
	t.Helper()
	return LoadGraph(graphSource(t, nodes, edges))
}

// graphSource is a KB holding the record files given; a nil file is absent.
func graphSource(t *testing.T, nodes, edges *string) *Source {
	t.Helper()
	root := filepath.Join(t.TempDir(), KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	held := map[string][]byte{}
	for rel, text := range map[string]*string{GraphNodesFile: nodes, GraphEdgesFile: edges} {
		if text != nil {
			held[KBDir+"/"+rel] = []byte(*text)
		}
	}
	return MigratedSource(root, FormatVersion, held, nil)
}

func ptr[T any](v T) *T { return &v }

const noRecords = "[]\n"

// TestGraphRoundTrip: each record type encodes to its design bytes, and those
// bytes load back as the record.
func TestGraphRoundTrip(t *testing.T) {
	cases := []struct {
		name         string
		g            Graph
		nodes, edges string
	}{
		{"claim", Graph{Claims: []ClaimNode{{ID: "clm-aaa001", Title: "Theorem 1", Rigor: ptr(0.9), Rationale: "Fixture.",
			StrengthenBy: []string{"State the noise distributions."}}}},
			"- id: \"clm-aaa001\"\n  title: \"Theorem 1\"\n  rigor: 0.9\n  manually-scored: false\n  rationale: \"Fixture.\"\n" +
				"  no-edge: \"\"\n  strengthen-by:\n    - \"State the noise distributions.\"\n", noRecords},
		{"claim, nothing to strengthen and pending", Graph{Claims: []ClaimNode{{ID: "clm-aaa002", Title: "Lemma 2", NoEdge: "Stands alone."}}},
			"- id: \"clm-aaa002\"\n  title: \"Lemma 2\"\n  rigor: \"*pending*\"\n  manually-scored: false\n  rationale: \"\"\n" +
				"  no-edge: \"Stands alone.\"\n  strengthen-by: []\n", noRecords},
		{"analysis", Graph{Analyses: []AnalysisNode{{ID: "ana-aaa007", Title: "Sensitivity of the flow bound", Rigor: ptr(0.7),
			ManuallyScored: true, Rationale: "Fixture."}}},
			"- id: \"ana-aaa007\"\n  title: \"Sensitivity of the flow bound\"\n  rigor: 0.7\n  manually-scored: true\n" +
				"  rationale: \"Fixture.\"\n  no-edge: \"\"\n", noRecords},
		{"experiment", Graph{Experiments: []ExperimentNode{{ID: "exp-aaa008", Title: "Flow measurement", Status: "run"}}},
			"- id: \"exp-aaa008\"\n  title: \"Flow measurement\"\n  status: \"run\"\n  rigor: \"*pending*\"\n  manually-scored: false\n", noRecords},
		{"work", Graph{Works: []WorkNode{{ID: "wrk-smith2020", Title: "Smith, A. 2020. *Markets*. Fixture Press.", Rigor: ptr(0.9),
			Rationale: "Fixture Press."}}},
			"- id: \"wrk-smith2020\"\n  title: \"Smith, A. 2020. *Markets*. Fixture Press.\"\n  rigor: 0.9\n  rationale: \"Fixture Press.\"\n", noRecords},
		{"axiom", Graph{Axioms: []AxiomNode{{ID: "axm-S1", Title: "Agents act on what they observe"}}},
			"- id: \"axm-S1\"\n  title: \"Agents act on what they observe\"\n", noRecords},
		{"unscored edge", Graph{References: []UnscoredEdge{{Source: "clm-aaa004", Target: "clm-bbb003"}}}, noRecords,
			"- relation: \"references\"\n  source: \"clm-aaa004\"\n  target: \"clm-bbb003\"\n  context: \"\"\n"},
		{"demoted edge", Graph{Demoted: []DemotedEdge{{Source: "clm-aaa004", Target: "clm-bbb001", Context: "the circle's other half",
			ReplacedRelation: RelationSupports, Bearing: Bearing{Pending: true}}}}, noRecords,
			"- relation: \"demoted\"\n  source: \"clm-aaa004\"\n  target: \"clm-bbb001\"\n  context: \"the circle's other half\"\n" +
				"  replaced-relation: \"supports\"\n  bearing: \"*pending*\"\n  manually-scored: false\n"},
		{"bearing edge", Graph{Founds: []BearingEdge{{Source: "wrk-smith2020", Target: "clm-bbb002", Bearing: Bearing{Value: 0.75}}}}, noRecords,
			"- relation: \"founds\"\n  source: \"wrk-smith2020\"\n  target: \"clm-bbb002\"\n  context: \"\"\n  bearing: 0.75\n"},
		{"scorable edge", Graph{Scorable: []ScorableEdge{{Relation: RelationSupports, Source: "clm-aaa002", Target: "clm-aaa001",
			Context: "first use; second use", Bearing: Bearing{Value: 1}}}}, noRecords,
			"- relation: \"supports\"\n  source: \"clm-aaa002\"\n  target: \"clm-aaa001\"\n  context: \"first use; second use\"\n" +
				"  bearing: 1.0\n  manually-scored: false\n"},
		{"scorable edge judged zero, manually", Graph{Scorable: []ScorableEdge{{Relation: RelationLifts, Source: "exp-aaa008",
			Target: "clm-aaa003", Bearing: Bearing{Value: 0}, ManuallyScored: true}}}, noRecords,
			"- relation: \"lifts\"\n  source: \"exp-aaa008\"\n  target: \"clm-aaa003\"\n  context: \"\"\n  bearing: 0.0\n  manually-scored: true\n"},
		{"escaped strings", Graph{Axioms: []AxiomNode{{ID: "axm-S2", Title: "a \"quote\", a \\ and\ta tab" + string(rune(0x2028)) + "\u0085\u0007"}}},
			"- id: \"axm-S2\"\n  title: \"a \\\"quote\\\", a \\\\ and\ta tab\\u2028\\u0085\\u0007\"\n", noRecords},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(EncodeNodes(c.g)); got != c.nodes {
				t.Errorf("EncodeNodes =\n%s\nwant\n%s", got, c.nodes)
			}
			if got := string(EncodeEdges(c.g)); got != c.edges {
				t.Errorf("EncodeEdges =\n%s\nwant\n%s", got, c.edges)
			}
			got, err := loadGraphFiles(t, &c.nodes, &c.edges)
			if err != nil || !reflect.DeepEqual(got, c.g) {
				t.Errorf("LoadGraph = %+v, %v; want %+v", got, err, c.g)
			}
		})
	}
}

// TestGraphEncodeOrder: nodes in id byte order; edges by identity — relation in
// Relations order, a demoted record's replaced relation in the same order,
// then source, then target.
func TestGraphEncodeOrder(t *testing.T) {
	g := Graph{
		Claims:   []ClaimNode{{ID: "clm-bbbbbb", Title: "B"}},
		Analyses: []AnalysisNode{{ID: "ana-aaaaaa", Title: "A"}},
		Axioms:   []AxiomNode{{ID: "axm-S1", Title: "S"}},
		Demoted: []DemotedEdge{
			{Source: "ana-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: RelationRefutes},
			{Source: "ana-aaaaaa", Target: "clm-bbbbbb", ReplacedRelation: RelationLifts},
		},
		References: []UnscoredEdge{{Source: "clm-bbbbbb", Target: "ana-aaaaaa"}},
		Scorable: []ScorableEdge{
			{Relation: RelationRefutes, Source: "ana-aaaaaa", Target: "clm-bbbbbb"},
			{Relation: RelationSupports, Source: "clm-cccccc", Target: "clm-bbbbbb"},
			{Relation: RelationSupports, Source: "clm-bbbbbb", Target: "clm-dddddd"},
		},
	}
	nodes, edges := string(EncodeNodes(g)), string(EncodeEdges(g))
	wantNodes := "- id: \"ana-aaaaaa\"\n  title: \"A\"\n  rigor: \"*pending*\"\n  manually-scored: false\n  rationale: \"\"\n  no-edge: \"\"\n" +
		"- id: \"axm-S1\"\n  title: \"S\"\n" +
		"- id: \"clm-bbbbbb\"\n  title: \"B\"\n  rigor: \"*pending*\"\n  manually-scored: false\n  rationale: \"\"\n  no-edge: \"\"\n  strengthen-by: []\n"
	if nodes != wantNodes {
		t.Errorf("EncodeNodes =\n%s\nwant\n%s", nodes, wantNodes)
	}
	want := "- relation: \"supports\"\n  source: \"clm-bbbbbb\"\n  target: \"clm-dddddd\"\n" +
		"  context: \"\"\n  bearing: 0.0\n  manually-scored: false\n" +
		"- relation: \"supports\"\n  source: \"clm-cccccc\"\n  target: \"clm-bbbbbb\"\n" +
		"  context: \"\"\n  bearing: 0.0\n  manually-scored: false\n" +
		"- relation: \"refutes\"\n  source: \"ana-aaaaaa\"\n  target: \"clm-bbbbbb\"\n" +
		"  context: \"\"\n  bearing: 0.0\n  manually-scored: false\n" +
		"- relation: \"references\"\n  source: \"clm-bbbbbb\"\n  target: \"ana-aaaaaa\"\n  context: \"\"\n" +
		"- relation: \"demoted\"\n  source: \"ana-aaaaaa\"\n  target: \"clm-bbbbbb\"\n  context: \"\"\n" +
		"  replaced-relation: \"lifts\"\n  bearing: 0.0\n  manually-scored: false\n" +
		"- relation: \"demoted\"\n  source: \"ana-aaaaaa\"\n  target: \"clm-bbbbbb\"\n  context: \"\"\n" +
		"  replaced-relation: \"refutes\"\n  bearing: 0.0\n  manually-scored: false\n"
	if edges != want {
		t.Errorf("EncodeEdges =\n%s\nwant\n%s", edges, want)
	}
}

// TestGraphRigorAboveTheMaximum: a rigor above the maximum, on every kind that
// carries one, reads as the maximum and is no refusal; the authored load reads
// it as the file holds it.
func TestGraphRigorAboveTheMaximum(t *testing.T) {
	g := Graph{
		Claims:      []ClaimNode{{ID: "clm-aaaaaa", Title: "C", Rigor: ptr(1.2)}},
		Analyses:    []AnalysisNode{{ID: "ana-aaaaaa", Title: "A", Rigor: ptr(1.2)}},
		Experiments: []ExperimentNode{{ID: "exp-aaaaaa", Title: "E", Status: "run", Rigor: ptr(1.2)}},
		Works:       []WorkNode{{ID: "wrk-k", Title: "W", Rigor: ptr(1.2)}},
	}
	nodes, edges := string(EncodeNodes(g)), noRecords
	rigors := func(g Graph) []*float64 {
		return []*float64{g.Claims[0].Rigor, g.Analyses[0].Rigor, g.Experiments[0].Rigor, g.Works[0].Rigor}
	}
	for _, c := range []struct {
		name string
		load func(*Source) (Graph, error)
		want float64
	}{{"LoadGraph", LoadGraph, RigorMaximum}, {"LoadAuthoredGraph", LoadAuthoredGraph, 1.2}} {
		got, err := c.load(graphSource(t, &nodes, &edges))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for i, r := range rigors(got) {
			if r == nil || *r != c.want {
				t.Errorf("%s: rigor %d = %v, want %v", c.name, i, r, c.want)
			}
		}
	}
}

// TestGraphRefusals: one case per refusal class, each naming its check, file,
// field and line.
func TestGraphRefusals(t *testing.T) {
	claim := func(field string) string {
		return "- id: \"clm-aaaaaa\"\n  title: \"T\"\n  rigor: 0.5\n  manually-scored: false\n  rationale: \"\"\n  no-edge: \"\"\n  strengthen-by: []\n" + field
	}
	edge := func(relation, source, target, rest string) string {
		return "- relation: \"" + relation + "\"\n  source: \"" + source + "\"\n  target: \"" + target + "\"\n  context: \"\"\n" + rest
	}
	none := noRecords
	cases := []struct {
		name         string
		nodes, edges *string
		want         GraphFault
	}{
		{"parse failure", ptr("- id: \"axm-S1\"\n\ttitle: \"T\"\n"), &none, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 2}},
		{"a key twice", ptr(claim("  title: \"U\"\n")), &none, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 8, Key: "title"}},
		{"unknown field", ptr(claim("  kind: \"claim\"\n")), &none, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 8, Key: "kind",
			Allowed: "id, title, rigor, manually-scored, rationale, no-edge, strengthen-by"}},
		{"missing field", ptr("- id: \"axm-S1\"\n"), &none, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 1, Key: "title"}},
		{"wrong type: a quoted number", ptr("- id: \"wrk-k\"\n  title: \"W\"\n  rigor: \"0.7\"\n  rationale: \"\"\n"), &none,
			GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 3, Key: "rigor"}},
		{"domain: a bearing above 1", &none, ptr(edge(RelationFounds, "wrk-k", "clm-aaaaaa", "  bearing: 1.5\n")),
			GraphFault{Check: CheckMetadata, Path: GraphEdgesFile, Line: 5, Key: "bearing", Allowed: "a number in [0, 1], or \"*pending*\""}},
		{"domain: a status", ptr("- id: \"exp-aaaaaa\"\n  title: \"E\"\n  status: \"done\"\n  rigor: 0.5\n  manually-scored: false\n"), &none,
			GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 3, Key: "status", Allowed: "run, pending"}},
		{"id grammar", ptr("- id: \"clm-abc\"\n  title: \"T\"\n"), &none, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 1, Key: "id"}},
		{"duplicate id", ptr(claim("") + claim("")), &none, GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 8, Key: "id"}},
		{"illegal pairing", &none, ptr(edge(RelationSupports, "wrk-k", "clm-aaaaaa", "  bearing: 0.5\n  manually-scored: false\n")),
			GraphFault{Check: CheckMetadata, Path: GraphEdgesFile, Line: 1, Key: "relation"}},
		{"illegal demoted pairing", &none, ptr(edge(RelationDemoted, "clm-aaaaaa", "clm-bbbbbb",
			"  replaced-relation: \"lifts\"\n  bearing: 0.5\n  manually-scored: false\n")),
			GraphFault{Check: CheckMetadata, Path: GraphEdgesFile, Line: 1, Key: "relation"}},
		{"absent file", &none, nil, GraphFault{Check: CheckIndex, Path: GraphEdgesFile, Remedy: RestoreFromGitRemedy}},
		{"a string holding a line break", ptr("- id: \"axm-S1\"\n  title: \"one\\ntwo\"\n"), &none,
			GraphFault{Check: CheckMetadata, Path: GraphNodesFile, Line: 2, Key: "title"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := loadGraphFiles(t, c.nodes, c.edges)
			var le *GraphLoadError
			if !errors.As(err, &le) || len(le.Faults) != 1 {
				t.Fatalf("LoadGraph error = %v, want one fault", err)
			}
			got := le.Faults[0]
			if got.Detail == "" {
				t.Errorf("fault %+v carries no detail", got)
			}
			got.Detail = ""
			if got != c.want {
				t.Errorf("fault = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestGraphRefusalListsEveryFault: one load names every fault across both
// files, not the first alone.
func TestGraphRefusalListsEveryFault(t *testing.T) {
	nodes := "- id: \"clm-abc\"\n  title: \"T\"\n- id: \"axm-S1\"\n  title: \"\"\n"
	edges := "- relation: \"cites\"\n  source: \"clm-aaaaaa\"\n  target: \"clm-bbbbbb\"\n  context: \"\"\n"
	_, err := loadGraphFiles(t, &nodes, &edges)
	var le *GraphLoadError
	if !errors.As(err, &le) || len(le.Faults) != 3 {
		t.Fatalf("LoadGraph error = %v, want three faults", err)
	}
}

package write

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
	"kbase/internal/result"
)

const seedClaim = "clm-aaaaaa"

// seedKB writes a minimal KB at the current format: an entry point listing one
// leaf, the leaf marking one claim, and the claim's record.
func seedKB(t *testing.T) string {
	t.Helper()
	root := recordKB(t, kb.Graph{Claims: []kb.ClaimNode{{ID: seedClaim, Title: "Seed Result", Rigor: f64(0.9), Rationale: "Seeded."}}})
	files := map[string]string{
		kb.EntryPointFile: "# Entry\n\n- [Leaf](leaf.md)\n",
		kb.FormatFile:     string(kb.EncodeFormatStamp(kb.FormatVersion)),
		"leaf.md": "[↑ Entry](entry-point.md)\n\n# Leaf\n\nThe first result holds here. <!-- kbase-id: " + seedClaim + " -->\n\n" +
			"A second paragraph names a bound.  \n\n## Bounds\n\nThe bound is tight in the limit.\n",
	}
	for rel, text := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func run(t *testing.T, root, op, values string) Result {
	t.Helper()
	return Run(op, Options{KBRoot: root, Values: []byte(values)})
}

// TestIdempotence issues a sequence of ops twice each with the same values:
// a re-issued removal is refused and a re-issued add or edit is unchanged,
// each leaving kb-root byte-identical, an add or an edit reporting the id the
// first call did. A marker an add placed on a line ending in whitespace stands
// once, ahead of that whitespace.
func TestIdempotence(t *testing.T) {
	root := seedKB(t)
	ids := map[string]string{}
	edge := func(op, source, target, relation string, extra ...string) string {
		return "entry:\n" + strings.Replace(edgeEntry(op, source, target, relation, extra...), "- operation: "+op+"\n  ", "- ", 1)
	}
	steps := []struct {
		op, values string
		mints      string
		again      string
	}{
		{AddClaim, "entry:\n- title: A New Result\n  rigor: 0.5\n  rationale: Derived from the seed in two steps.\n" +
			"  document: leaf.md\n  line: 7\n  snippet: names a bound\n", "claim", result.Unchanged},
		{AddEdge, edge(AddEdge, seedClaim, "{claim}", kb.RelationSupports, "context: the bound it rests on", "bearing: 0.5"), "", result.Unchanged},
		{EditClaim, "entry:\n- id: {claim}\n  rigor: 0.75\n", "", result.Unchanged},
		{EditClaim, "entry:\n- id: {claim}\n  rationale: Rewritten whole.\n", "", result.Unchanged},
		{AddEdge, edge(AddEdge, "{claim}", seedClaim, kb.RelationReferences), "", result.Unchanged},
		{AddWork, "entry:\n- label: Smith2020\n  title: Smith (2020)\n  rigor: \"*pending*\"\n  rationale: Cited for the bound.\n", "work", result.Unchanged},
		{AddEdge, edge(AddEdge, "{work}", "{claim}", kb.RelationFounds), "", result.Unchanged},
		{EditEdge, edge(EditEdge, "{work}", "{claim}", kb.RelationFounds, "bearing: 0.5"), "", result.Unchanged},
		{AddAnalysis, "entry:\n- title: An Analysis\n  rigor: \"*pending*\"\n  rationale: Lifts the new result.\n" +
			"  document: leaf.md\n  line: 11\n  snippet: bound is tight\n", "analysis", result.Unchanged},
		{AddEdge, edge(AddEdge, "{analysis}", "{claim}", kb.RelationLifts, "bearing: 0.5"), "", result.Unchanged},
		{EditEdge, edge(EditEdge, "{analysis}", "{claim}", kb.RelationLifts, "bearing: 0.25"), "", result.Unchanged},
		{AddEdge, edge(AddEdge, "{analysis}", seedClaim, kb.RelationRefutes, "context: the bound is loose"), "", result.Unchanged},
		{AddExperiment, "entry:\n- title: A Trial\n  status: run\n  rigor: 0.9\n  document: leaf.md\n  line: 3\n  snippet: Leaf\n", "experiment", result.Unchanged},
		{AddEdge, edge(AddEdge, "{experiment}", "{claim}", kb.RelationLifts, "bearing: 0.5"), "", result.Unchanged},
		{RemoveEdge, edge(RemoveEdge, "{analysis}", seedClaim, kb.RelationRefutes), "", result.Refused},
	}
	for i, s := range steps {
		values := s.values
		for name, id := range ids {
			values = strings.ReplaceAll(values, "{"+name+"}", id)
		}
		first := run(t, root, s.op, values)
		if first.Outcome != result.Done {
			t.Fatalf("step %d %s: first call %+v", i, s.op, first)
		}
		if s.mints != "" {
			if len(first.IDs()) != 1 {
				t.Fatalf("step %d %s: ids %v, want one minted", i, s.op, first.IDs())
			}
			ids[s.mints] = first.IDs()[0]
		}
		before := snapshot(t, root)
		second := run(t, root, s.op, values)
		switch s.again {
		case result.Refused:
			if second.Outcome != result.Refused || len(second.Refusals) != 1 {
				t.Errorf("step %d %s: second call %+v, want one refusal", i, s.op, second)
			}
		case result.Unchanged:
			if second.Outcome != result.Unchanged || !slices.Equal(second.IDs(), first.IDs()) {
				t.Errorf("step %d %s: second call %+v, want unchanged, ids %v", i, s.op, second, first.IDs())
			}
		}
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Errorf("step %d %s: the second call changed kb-root", i, s.op)
		}
	}
	leaf := snapshot(t, root)["leaf.md"]
	if n := strings.Count(leaf, kb.StatementMarker(ids["claim"])); n != 1 {
		t.Errorf("leaf.md carries the new claim's marker %d times, want 1:\n%s", n, leaf)
	}
	if !strings.Contains(leaf, "names a bound."+kb.StatementMarker(ids["claim"])+"  \n") {
		t.Errorf("the marker did not land ahead of the line's trailing whitespace:\n%s", leaf)
	}
}

// TestValuesRefusalShape: every refusal names its key and position, and
// carries allowed where the key or the value comes from a closed set (the
// table's key names for a key, the vocabulary for a closed-set value); the
// whole batch is refused.
func TestValuesRefusalShape(t *testing.T) {
	addWork := fieldNames(opFields[AddWork])
	premises := fieldNames(restsOnFields)
	top := []string{commitKey, dryRunKey, entryKey}
	const work = "entry:\n- label: L\n  title: T\n  rationale: R\n"
	for _, tc := range []struct {
		name, op, values string
		want             []result.Item
	}{
		{"unknown key", AddWork, work + "  rigour: 0.5\n",
			[]result.Item{{Key: "rigour", Entry: 1, Line: 5, Column: 3, Allowed: addWork}}},
		{"a key 4.0.0 drops", AddClaim, "entry:\n- title: T\n  rigor: 0.5\n  rationale: R\n  no-edge: cited as contrast\n" +
			"  document: leaf.md\n  line: 5\n  snippet: S\n",
			[]result.Item{{Key: "no-edge", Entry: 1, Line: 5, Column: 3, Allowed: fieldNames(opFields[AddClaim])}}},
		{"missing key", AddWork, "entry:\n- title: T\n  rationale: R\n",
			[]result.Item{{Key: "label", Entry: 1, Line: 2, Column: 3, Allowed: addWork}}},
		{"out of domain", EditClaim, "entry:\n- id: clm-abcdef\n  rigor: 1.5\n",
			[]result.Item{{Key: "rigor", Entry: 1, Line: 3, Column: 10}}},
		{"unreadable number", EditClaim, "entry:\n- id: clm-abcdef\n  rigor: 0.00001\n",
			[]result.Item{{Key: "rigor", Entry: 1, Line: 3, Column: 10}}},
		{"outside a closed vocabulary", EditExperiment, "entry:\n- id: exp-abcdef\n  status: bogus\n",
			[]result.Item{{Key: "status", Entry: 1, Line: 3, Column: 11, Allowed: kb.ExperimentStatuses}}},
		{"second entry, nested key", AddBuildEdges,
			"entry:\n- id: clm-abcdef\n  rests-on:\n  - id: clm-cccccc\n- id: clm-bbbbbb\n  rests-on:\n  - id: clm-cccccc\n    weight: 2\n",
			[]result.Item{{Key: "rests-on[1].weight", Entry: 2, Line: 8, Column: 5, Allowed: premises}}},
		{"form feed from JSON", AddWork,
			`{"entry": [{"title": "the \frac rule", "label": "L", "rationale": "R"}]}`,
			[]result.Item{{Key: "title", Entry: 1, Line: 1, Column: 22}}},
		{"backspace", AddWork,
			"entry:\n- title: \"\\beta decay\"\n  label: L\n  rationale: R\n",
			[]result.Item{{Key: "title", Entry: 1, Line: 2, Column: 10}}},
		{"blank line names the line", SplitClaim,
			"entry:\n- id: clm-abcdef\n  title: T\n  rationale: |\n    one\n\n    two\n  document: leaf.md\n  line: 5\n",
			[]result.Item{{Key: "rationale", Entry: 1, Line: 6}}},
		{"op named in the batch", EditClaim, "op: edit-claim\nentry:\n- id: clm-abcdef\n  rigor: 0.5\n",
			[]result.Item{{Key: "op", Line: 1, Column: 1, Allowed: top}}},
		{"empty top-level key", EditClaim, "\"\": 1\nentry:\n- id: clm-abcdef\n  rigor: 0.5\n",
			[]result.Item{{Key: `""`, Line: 1, Column: 1, Allowed: top}}},
		{"operation on an op's own entry", AddWork, "entry:\n- operation: add-work\n  label: L\n  title: T\n  rationale: R\n",
			[]result.Item{{Key: "operation", Entry: 1, Line: 2, Column: 3, Allowed: addWork}}},
		{"dry-run not a boolean", EditClaim, "dry-run: yes please\nentry: []\n",
			[]result.Item{{Key: dryRunKey, Line: 1, Column: 10}}},
		{"commit not a boolean", EditClaim, "commit: 1\nentry: []\n",
			[]result.Item{{Key: commitKey, Line: 1, Column: 9}}},
		{"empty entry key", AddWork, work + "  \"\": 1\n",
			[]result.Item{{Key: `""`, Entry: 1, Line: 5, Column: 3, Allowed: addWork}}},
		{"empty key twice", AddWork, work + "  \"\": 1\n  \"\": 2\n",
			[]result.Item{{Key: `""`, Entry: 1, Line: 6, Column: 3, Allowed: addWork}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, got := parseValues([]byte(tc.values), tc.op)
			if b.entries != nil {
				t.Errorf("a refused batch returned entries %v", b.entries)
			}
			for i := range got {
				got[i].Detail = ""
			}
			if !slices.EqualFunc(got, tc.want, func(a, b result.Item) bool {
				return a.Key == b.Key && a.Entry == b.Entry && a.Line == b.Line && a.Column == b.Column && slices.Equal(a.Allowed, b.Allowed)
			}) {
				t.Errorf("refusals %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestValuesAccepted: YAML and JSON carry the same values, an integer is a
// number, and the pending literal is nil.
func TestValuesAccepted(t *testing.T) {
	for _, doc := range []string{
		"entry:\n- id: clm-abcdef\n  rigor: 0\n- id: clm-bbbbbb\n  rigor: \"*pending*\"\n",
		`{"entry": [{"id": "clm-abcdef", "rigor": 0}, {"id": "clm-bbbbbb", "rigor": "*pending*"}]}`,
	} {
		b, refusals := parseValues([]byte(doc), EditClaim)
		entries := b.entries
		if refusals != nil || len(entries) != 2 {
			t.Fatalf("%q: %v %v", doc, entries, refusals)
		}
		if v := entries[0].score("rigor"); v == nil || *v != 0 || kb.PyFloatRepr(*v) != "0.0" {
			t.Errorf("rigor 0 read as %v", v)
		}
		if entries[1].score("rigor") != nil {
			t.Error("the pending literal did not read as nil")
		}
	}
}

// TestIDVocabulary: every op's id checks take the kinds' prefixes and no
// retired one, and an axiom premise is named by its axm- id and carries its
// bearing.
func TestIDVocabulary(t *testing.T) {
	for _, tc := range []struct {
		name, op, values string
		accepted         bool
	}{
		{"claim entry", RemoveNode, "entry:\n- id: clm-abcdef\n", true},
		{"analysis entry", RemoveNode, "entry:\n- id: ana-abcdef\n", true},
		{"retired analysis id", RemoveNode, "entry:\n- id: sup-abcdef\n", false},
		{"work entry", RemoveNode, "entry:\n- id: wrk-s00000\n", true},
		{"3.0.0 work id", RemoveNode, "entry:\n- id: wrk-smith2020\n", false},
		{"retired work id", RemoveNode, "entry:\n- id: work-smith2020\n", false},
		{"experiment entry", RemoveNode, "entry:\n- id: exp-abcdef\n", true},
		{"axiom source", AddEdge, "entry:\n- source: axm-s10000\n  target: clm-abcdef\n  relation: founds\n  bearing: 0.8\n", true},
		{"analysis source", AddEdge, "entry:\n- source: ana-abcdef\n  target: clm-abcdef\n  relation: lifts\n  bearing: 0.8\n", true},
		{"analysis target", AddEdge, "entry:\n- source: clm-abcdef\n  target: ana-abcdef\n  relation: supports\n  bearing: 0.8\n", true},
		{"retired analysis target", AddEdge, "entry:\n- source: clm-abcdef\n  target: sup-abcdef\n  relation: supports\n  bearing: 0.8\n", false},
		{"axiom premise", AddBuildEdges, "entry:\n- id: clm-abcdef\n  rests-on:\n  - id: axm-s10000\n    bearing: 0.8\n", true},
		{"3.0.0 axiom premise", AddBuildEdges, "entry:\n- id: clm-abcdef\n  rests-on:\n  - id: axm-S1\n    bearing: 0.8\n", false},
		{"label-less axiom premise", AddBuildEdges, "entry:\n- id: clm-abcdef\n  rests-on:\n  - id: axm-\n    bearing: 0.8\n", false},
		{"retired axiom token premise", AddBuildEdges, "entry:\n- id: clm-abcdef\n  rests-on:\n  - id: Axiom S1\n    bearing: 0.8\n", false},
		{"retired invariant premise", AddBuildEdges, "entry:\n- id: clm-abcdef\n  rests-on:\n  - id: INVARIANT-S1\n    bearing: 0.8\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, refusals := parseValues([]byte(tc.values), tc.op)
			entries := b.entries
			if tc.accepted {
				if refusals != nil || len(entries) != 1 {
					t.Fatalf("refused %v", refusals)
				}
				return
			}
			if entries != nil || len(refusals) != 1 {
				t.Fatalf("accepted %v", entries)
			}
		})
	}
}

func TestSpliceLines(t *testing.T) {
	for _, tc := range []struct {
		name, doc  string
		start, end int
		lines      []string
		want       string
	}{
		{"crlf kept", "a\r\nb\r\nc\r\n", 1, 2, []string{"B"}, "a\r\nB\r\nc\r\n"},
		{"append past an unterminated end", "a\nb", 2, 2, []string{"c"}, "a\nb\nc"},
		{"replace an unterminated last line", "a\nb", 1, 2, []string{"B"}, "a\nB"},
		{"form feed is a line of its own", "a\fb\nc\n", 1, 2, []string{"B"}, "a\fB\nc\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := spliceLines(tc.doc, tc.start, tc.end, tc.lines); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExcerptLines(t *testing.T) {
	doc := "---\nkind: leaf\n---\n\n# Title\n\n> **Theorem 1.** The *bound* holds\n> for every $n$.\n\nSee [the proof](p.md#x) of the bound holds.\n"
	for _, tc := range []struct {
		excerpt string
		want    []int
	}{
		{"The *bound* holds for every", []int{6}},
		{"the bound holds for every n", []int{6}},
		{"of the bound holds", []int{9}},
		{"bound holds", []int{9}},
		{"BOUND, holds", []int{6, 9}},
		{"absent words", nil},
	} {
		if got := excerptLines(doc, tc.excerpt); !slices.Equal(got, tc.want) {
			t.Errorf("excerptLines(%q) = %v, want %v", tc.excerpt, got, tc.want)
		}
	}
}

// TestOpsAreTheCallerWriteOps: Ops is the caller-issued write ops, sorted,
// and no build-only op.
func TestOpsAreTheCallerWriteOps(t *testing.T) {
	want := []string{AddAnalysis, AddAxiom, AddCitation, AddClaim, AddEdge, AddExperiment, AddWork,
		EditAnalysis, EditAxiom, EditClaim, EditEdge, EditExperiment, EditWork,
		MoveNode, RemoveEdge, RemoveNode, RestoreDemoted}
	if got := Ops(); !slices.Equal(got, want) {
		t.Errorf("Ops() = %q, want %q", got, want)
	}
}

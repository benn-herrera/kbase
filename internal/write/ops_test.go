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

// seedKB writes a minimal KB: an entry point, one leaf citing one claim, and
// the register holding it.
func seedKB(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), kb.KBDir)
	files := map[string]string{
		kb.EntryPointFile: "---\nkind: entry-point\nkb-format: \"1.0.0\"\n---\n# Entry\n\n- [Leaf](leaf.md)\n",
		"leaf.md": "---\nkind: leaf\nclaims: [" + seedClaim + "]\n---\n[↑ Entry](entry-point.md)\n\n" +
			"# Leaf\n\nThe first result holds here. <!-- claim-quality: " + seedClaim + " -->\n\nA second paragraph names a bound.  \n\n## Bounds\n\nThe bound is tight in the limit.\n",
		kb.RegisterFile: "# Register\n\n" + renderEntry(registerEntry{NodeID: seedClaim, Title: "Seed Result", ScoreField: "confidence",
			Score: f64(0.9), Rationale: "Seeded."}) + "\n",
	}
	for rel, text := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
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

func run(t *testing.T, root, op, values string, create bool) Result {
	t.Helper()
	return Run(op, Options{KBRoot: root, Values: []byte(values), Create: create})
}

// TestIdempotence issues every op twice with the same values: the second
// call reports unchanged and leaves kb-root byte-identical, an insert
// adopting the entry the first wrote.
func TestIdempotence(t *testing.T) {
	root := seedKB(t)
	ids := map[string]string{}
	steps := []struct {
		op, values string
		mints      string
	}{
		{"insert-claim-entry", "entry:\n- register: claim-quality.md\n  title: A New Result\n  rigor: 0.5\n  rationale: |\n    Derived from the seed\n    in two steps.\n  depends-on:\n  - id: " + seedClaim + "\n    context: the bound it rests on\n  strengthen-by:\n  - Tighten the constant.\n", "claim"},
		{"set-rigor", "entry:\n- id: {claim}\n  rigor: 0.75\n", ""},
		{"set-rationale", "entry:\n- id: {claim}\n  rationale: Rewritten whole.\n", ""},
		{"add-depends-on", "entry:\n- id: {claim}\n  references:\n  - id: " + seedClaim + "\n", ""},
		{"set-frontmatter", "entry:\n- document: leaf.md\n  kind: leaf\n  claims: [" + seedClaim + ", {claim}]\n", ""},
		{"mark-claim-in-leaf", "entry:\n- document: leaf.md\n  id: {claim}\n  locator: a second paragraph names a bound\n", ""},
		{"insert-work-entry", "entry:\n- register: claim-quality.md\n  key: Smith2020\n  title: Smith (2020)\n  strength: \"*pending*\"\n  rationale: Cited for the bound.\n", "work"},
		{"set-work-strength", "entry:\n- id: work-Smith2020\n  strength: 0.8\n", ""},
		{"add-depends-on", "entry:\n- id: {claim}\n  depends-on:\n  - id: work-Smith2020\n", ""},
		{"set-applicability", "entry:\n- id: {claim}\n  work: work-Smith2020\n  applicability: 0.5\n", ""},
		{"insert-support-entry", "entry:\n- register: claim-quality.md\n  title: A Support\n  rigor: \"*pending*\"\n  rationale: Supports the new result.\n  supports:\n  - id: {claim}\n    fraction: 0.5\n", "support"},
		{"set-on-point-fraction", "entry:\n- id: {support}\n  claim: {claim}\n  fraction: 0.25\n", ""},
		{"insert-experiment-entry", "entry:\n- document: leaf.md\n  status: run\n  strengthens:\n  - id: {claim}\n    strength: 0.5\n", "experiment"},
	}
	for i, s := range steps {
		values := s.values
		for name, id := range ids {
			values = strings.ReplaceAll(values, "{"+name+"}", id)
		}
		first := run(t, root, s.op, values, false)
		if first.Outcome != result.Done {
			t.Fatalf("step %d %s: first call %+v", i, s.op, first)
		}
		if s.mints != "" {
			wantMinted := 1
			if s.op == "insert-work-entry" {
				wantMinted = 0
			}
			if len(first.IDs) != 1 || len(first.Minted) != wantMinted {
				t.Fatalf("step %d %s: ids %v minted %v", i, s.op, first.IDs, first.Minted)
			}
			ids[s.mints] = first.IDs[0]
		}
		before := snapshot(t, root)
		second := run(t, root, s.op, values, false)
		if second.Outcome != result.Unchanged {
			t.Errorf("step %d %s: second call %+v, want unchanged", i, s.op, second)
		}
		if !slices.Equal(second.IDs, first.IDs) || len(second.Minted) != 0 {
			t.Errorf("step %d %s: second call ids %v minted %v, want %v and none", i, s.op, second.IDs, second.Minted, first.IDs)
		}
		if after := snapshot(t, root); !maps.Equal(before, after) {
			t.Errorf("step %d %s: the second call changed kb-root", i, s.op)
		}
	}
	leaf := snapshot(t, root)["leaf.md"]
	if n := strings.Count(leaf, renderTier2Marker(ids["claim"])); n != 1 {
		t.Errorf("leaf.md carries the new claim's marker %d times, want 1:\n%s", n, leaf)
	}
	if !strings.Contains(leaf, "names a bound. "+renderTier2Marker(ids["claim"])+"  \n") {
		t.Errorf("the marker did not land ahead of the line's trailing whitespace:\n%s", leaf)
	}
}

// TestInsertAdoptsByTitle: an insert whose title the register already keys
// lands on that entry, changes nothing, and names the values it differs in.
func TestInsertAdoptsByTitle(t *testing.T) {
	root := seedKB(t)
	run(t, root, "insert-claim-entry", "entry:\n- register: claim-quality.md\n  title: Other\n  rigor: 0.2\n  rationale: Other.\n", false)
	before := snapshot(t, root)
	res := run(t, root, "insert-claim-entry", "entry:\n- register: claim-quality.md\n  title: Seed Result\n  rigor: 0.1\n  rationale: Seeded.\n", false)
	if res.Outcome != result.Unchanged || !slices.Equal(res.IDs, []string{seedClaim}) || len(res.Minted) != 0 {
		t.Fatalf("got %+v", res)
	}
	if len(res.Adopted) != 1 || res.Adopted[0].Entry != 1 || res.Adopted[0].ID != seedClaim || !slices.Equal(res.Adopted[0].Differs, []string{"rigor"}) {
		t.Errorf("adopted %+v, want entry 1 adopting %s and differing in rigor", res.Adopted, seedClaim)
	}
	if !maps.Equal(before, snapshot(t, root)) {
		t.Error("an adopting insert wrote")
	}
}

// TestValuesRefusalShape: every refusal names its key, its position and the
// key vocabulary of the table it sits in, and the whole batch is refused.
func TestValuesRefusalShape(t *testing.T) {
	setRigor := fieldNames(opFields["set-rigor"])
	insert := fieldNames(opFields["insert-claim-entry"])
	depends := fieldNames(dependsOnFields)
	for _, tc := range []struct {
		name, op, values string
		want             []result.Item
	}{
		{"unknown key", "set-rigor", "entry:\n- id: clm-abcdef\n  rigor: 0.5\n  rigour: 0.5\n",
			[]result.Item{{Key: "rigour", Entry: 1, Line: 4, Column: 3, Allowed: setRigor}}},
		{"missing key", "set-rigor", "entry:\n- id: clm-abcdef\n",
			[]result.Item{{Key: "rigor", Entry: 1, Line: 2, Column: 3, Allowed: setRigor}}},
		{"out of domain", "set-rigor", "entry:\n- id: clm-abcdef\n  rigor: 1.5\n",
			[]result.Item{{Key: "rigor", Entry: 1, Line: 3, Column: 10, Allowed: setRigor}}},
		{"unreadable number", "set-rigor", "entry:\n- id: clm-abcdef\n  rigor: 0.00001\n",
			[]result.Item{{Key: "rigor", Entry: 1, Line: 3, Column: 10, Allowed: setRigor}}},
		{"second entry, nested key", "insert-claim-entry",
			"entry:\n- register: r.md\n  title: T\n  rigor: 0.5\n  rationale: R\n- register: r.md\n  title: T\n  rigor: 0.5\n  rationale: R\n  depends-on:\n  - id: clm-abcdef\n    weight: 2\n",
			[]result.Item{{Key: "depends-on[1].weight", Entry: 2, Line: 12, Column: 5, Allowed: depends}}},
		{"form feed from JSON", "insert-claim-entry",
			`{"entry": [{"register": "r.md", "title": "the \frac rule", "rigor": 0.5, "rationale": "R"}]}`,
			[]result.Item{{Key: "title", Entry: 1, Line: 1, Column: 42, Allowed: insert}}},
		{"backspace", "insert-claim-entry",
			"entry:\n- register: r.md\n  title: \"\\beta decay\"\n  rigor: 0.5\n  rationale: R\n",
			[]result.Item{{Key: "title", Entry: 1, Line: 3, Column: 10, Allowed: insert}}},
		{"blank line names the line", "insert-claim-entry",
			"entry:\n- register: r.md\n  title: T\n  rigor: 0.5\n  rationale: |\n    one\n\n    two\n",
			[]result.Item{{Key: "rationale", Entry: 1, Line: 7, Allowed: insert}}},
		{"op named in the document", "set-rigor", "op: set-rigor\nentry:\n- id: clm-abcdef\n  rigor: 0.5\n",
			[]result.Item{{Key: "op", Line: 1, Column: 1, Allowed: []string{entryKey}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, got := parseValues([]byte(tc.values), tc.op)
			if entries != nil {
				t.Errorf("a refused batch returned entries %v", entries)
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
		"entry:\n- id: clm-abcdef\n  rigor: 1\n- id: sup-abcdef\n  rigor: \"*pending*\"\n",
		`{"entry": [{"id": "clm-abcdef", "rigor": 1}, {"id": "sup-abcdef", "rigor": "*pending*"}]}`,
	} {
		entries, refusals := parseValues([]byte(doc), "set-rigor")
		if refusals != nil || len(entries) != 2 {
			t.Fatalf("%q: %v %v", doc, entries, refusals)
		}
		if v := entries[0].score("rigor"); v == nil || *v != 1 || formatScore(v) != "1.0" {
			t.Errorf("rigor 1 read as %v", v)
		}
		if entries[1].score("rigor") != nil {
			t.Error("the pending literal did not read as nil")
		}
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
	doc := "<!-- kb-frontmatter\nkind: leaf\n-->\n\n# Title\n\n> **Theorem 1.** The *bound* holds\n> for every $n$.\n\nSee [the proof](p.md#x) of the bound holds.\n"
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

func TestRenderCitation(t *testing.T) {
	root := seedKB(t)
	res := run(t, root, RenderCitation, "entry:\n- excerpt: The bound is tight\n  cited-document: leaf.md\n  anchor: bounds\n  citing-document: sub/notes.md\n", false)
	if res.Outcome != result.Refused || res.Refusals[0].Key != "citing-document" {
		t.Fatalf("a citing document in a missing directory: %+v", res)
	}
	res = run(t, root, RenderCitation, "entry:\n- excerpt: The bound is tight\n  cited-document: leaf.md\n  anchor: bounds\n  citing-document: notes.md\n", false)
	if res.Outcome != result.Done || !slices.Equal(res.Citations, []string{`["The bound is tight"](leaf.md#bounds)`}) {
		t.Fatalf("got %+v", res)
	}
	res = run(t, root, RenderCitation, "entry:\n- excerpt: The bound is loose in the limit\n  cited-document: leaf.md\n  anchor: bounds\n  citing-document: notes.md\n", false)
	if res.Outcome != result.Refused || res.Refusals[0].Key != "excerpt" {
		t.Fatalf("a misquote: %+v", res)
	}
}

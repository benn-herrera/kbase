package kb

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// jsonKeys is T's json tag names, in field order.
func jsonKeys[T any]() []string {
	var keys []string
	rt := reflect.TypeFor[T]()
	for i := range rt.NumField() {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		keys = append(keys, name)
	}
	return keys
}

func fieldKeys(fields []IndexField) []string {
	var keys []string
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	return keys
}

// checkRowSchema: every record the writer emits for rows is the reader's keys
// in the reader's order, and every key the reader decodes is emitted by some
// row.
func checkRowSchema[T interface{ IndexFields() []IndexField }](t *testing.T, rows []T) {
	t.Helper()
	tags := jsonKeys[T]()
	emitted := map[string]bool{}
	for _, r := range rows {
		keys := fieldKeys(r.IndexFields())
		var inTagOrder []string
		for _, tag := range tags {
			if slices.Contains(keys, tag) {
				inTagOrder = append(inTagOrder, tag)
			}
		}
		if !slices.Equal(keys, inTagOrder) {
			t.Errorf("%T %+v writes %q; the reader's keys in their order are %q", r, r, keys, inTagOrder)
		}
		for _, k := range keys {
			emitted[k] = true
		}
	}
	for _, tag := range tags {
		if !emitted[tag] {
			t.Errorf("%T decodes %q, which no record the writer emits carries", *new(T), tag)
		}
	}
}

func TestIndexRowsWriteWhatTheyRead(t *testing.T) {
	var nodes []NodeRow
	for _, kind := range NodeKinds {
		nodes = append(nodes, NodeRow{NodeType: kind})
	}
	checkRowSchema(t, nodes)
	var edges []EdgeRow
	for _, relation := range Relations {
		edges = append(edges, EdgeRow{Relation: relation})
	}
	checkRowSchema(t, edges)
	checkRowSchema(t, []StrengthenByRow{{}})
	checkRowSchema(t, []CiteRow{{}})
	checkRowSchema(t, []AggregateRow{{}})
}

func TestIndexFileOfEveryRowType(t *testing.T) {
	got := []string{indexFileOf[NodeRow](), indexFileOf[EdgeRow](), indexFileOf[StrengthenByRow](),
		indexFileOf[CiteRow](), indexFileOf[AggregateRow]()}
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(IndexFiles))) {
		t.Errorf("row types read %q, the index's files are %q", got, IndexFiles)
	}
}

// TestReadIndexKeepsWhatTheRecordCarries: a decoded row's kind, target kind
// and relation survive; only a key the record lacks takes its default.
func TestReadIndexKeepsWhatTheRecordCarries(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	if err := os.MkdirAll(filepath.Join(root, IndexDir), 0o755); err != nil {
		t.Fatal(err)
	}
	src := MigratedSource(root, FormatVersion, map[string][]byte{
		"kb-root/.index/claims.yaml": []byte(`--- {"node_type": "analysis", "id": "ana-aaaaaa", "rigor": 0.5}` + "\n" +
			`--- {"node_type": "experiment", "id": "exp-aaaaaa", "status": "run"}` + "\n" +
			`--- {"node_type": "work", "id": "wrk-x"}` + "\n" +
			`--- {"node_type": "axiom", "id": "axm-S1"}` + "\n" +
			`--- {"node_type": "claim", "id": "clm-aaaaaa"}` + "\n" +
			`--- {"id": "clm-bbbbbb"}` + "\n"),
		"kb-root/.index/edges.yaml": []byte(`--- {"source": "wrk-x", "target": "clm-aaaaaa", "relation": "founds", "target_kind": "work", "bearing": "0.5"}` + "\n" +
			`--- {"source": "clm-aaaaaa", "target": "ana-aaaaaa", "relation": "supports", "target_kind": "analysis", "bearing": "0.5"}` + "\n" +
			`--- {"source": "clm-aaaaaa", "target": "clm-bbbbbb", "relation": "references"}` + "\n"),
	}, nil)
	_, nodes, problems, err := ReadIndex[NodeRow](src, nil)
	if err != nil || problems != nil {
		t.Fatalf("nodes: %v %v", err, problems)
	}
	var kinds []string
	for _, n := range nodes {
		kinds = append(kinds, n.NodeType)
	}
	if want := []string{"analysis", "experiment", "work", "axiom", "claim", "claim"}; !slices.Equal(kinds, want) {
		t.Errorf("node types = %q, want %q (the last a record that carries none)", kinds, want)
	}
	_, edges, problems, err := ReadIndex[EdgeRow](src, nil)
	if err != nil || problems != nil || len(edges) != 3 {
		t.Fatalf("edges = %+v, %v %v", edges, err, problems)
	}
	for i, want := range []struct{ relation, targetKind string }{
		{"founds", "work"}, {"supports", "analysis"}, {"references", "claim"},
	} {
		if got := edges[i]; got.Relation != want.relation || got.TargetKind != want.targetKind {
			t.Errorf("edge %d = %s/%s, want %s/%s (the last target_kind a record that carries none)", i, got.Relation, got.TargetKind, want.relation, want.targetKind)
		}
	}
}

// TestReadIndexRefusesAnEdgeWithNoRelation: an edge record that omits its
// relation is a problem on its line, not a row with an empty one.
func TestReadIndexRefusesAnEdgeWithNoRelation(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	if err := os.MkdirAll(filepath.Join(root, IndexDir), 0o755); err != nil {
		t.Fatal(err)
	}
	src := MigratedSource(root, FormatVersion, map[string][]byte{
		"kb-root/.index/edges.yaml": []byte("--- {\"source\": \"clm-aaaaaa\", \"target\": \"clm-bbbbbb\", \"target_kind\": \"claim\"}\n" +
			"--- {\"source\": \"clm-aaaaaa\", \"target\": \"clm-cccccc\", \"relation\": \"references\", \"target_kind\": \"claim\"}\n"),
	}, nil)
	_, rows, problems, err := ReadIndex[EdgeRow](src, nil)
	if err != nil || len(rows) != 1 || rows[0].Target != "clm-cccccc" {
		t.Fatalf("rows = %+v, err = %v; want the one row that carries its relation", rows, err)
	}
	if len(problems) != 1 || problems[0].Line != 1 || !errors.Is(problems[0].Err, errNoRelation) {
		t.Errorf("problems = %+v, want one on line 1", problems)
	}
}

package kb

import (
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
	checkRowSchema(t, []SupportedByRow{{}})
	checkRowSchema(t, []CiteRow{{}})
	checkRowSchema(t, []AggregateRow{{}})
}

func TestIndexFileOfEveryRowType(t *testing.T) {
	got := []string{indexFileOf[NodeRow](), indexFileOf[EdgeRow](), indexFileOf[StrengthenByRow](),
		indexFileOf[SupportedByRow](), indexFileOf[CiteRow](), indexFileOf[AggregateRow]()}
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(IndexFiles))) {
		t.Errorf("row types read %q, the index's files are %q", got, IndexFiles)
	}
}

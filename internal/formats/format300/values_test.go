package format300

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	// formatsTestdata holds the 3.0.0 trees and their decoded-values goldens,
	// <tree>.values.jsonl beside each <tree>/.
	formatsTestdata = "../testdata"
	// valuesGotDir is where a differing decode is written under its golden's
	// name, so a golden is written by copying it in.
	valuesGotDir = "../../../test-data/transient/unit_tests/formats/got"
)

// field is one key of a record and its decoded value.
type field struct {
	key   string
	value any
}

// valuesLine is a record as one JSON object, its keys in fields' order.
func valuesLine(t *testing.T, fields []field) string {
	t.Helper()
	marshal := func(v any) []byte {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
		return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(marshal(f.key))
		b.WriteByte(':')
		b.Write(marshal(f.value))
	}
	b.WriteString("}\n")
	return b.String()
}

func rigorValue(r *float64) any {
	if r == nil {
		return PendingLiteral
	}
	return *r
}

func bearingValue(b Bearing) any {
	if b.Pending {
		return PendingLiteral
	}
	return b.Value
}

// decodedValues is every record g holds, one JSON line each in its record
// file's order — nodes by id, then edges by identity — each with its type's
// fields in the record file's order.
func decodedValues(t *testing.T, g Graph) string {
	t.Helper()
	type node struct {
		id     string
		fields []field
	}
	var nodes []node
	for _, n := range g.Claims {
		nodes = append(nodes, node{n.ID, []field{{fieldID, n.ID}, {fieldTitle, n.Title}, {RigorKey, rigorValue(n.Rigor)},
			{ManuallyScoredKey, n.ManuallyScored}, {fieldRationale, n.Rationale}, {fieldNoEdge, n.NoEdge},
			{fieldStrengthenBy, append([]string{}, n.StrengthenBy...)}}})
	}
	for _, n := range g.Analyses {
		nodes = append(nodes, node{n.ID, []field{{fieldID, n.ID}, {fieldTitle, n.Title}, {RigorKey, rigorValue(n.Rigor)},
			{ManuallyScoredKey, n.ManuallyScored}, {fieldRationale, n.Rationale}, {fieldNoEdge, n.NoEdge}}})
	}
	for _, n := range g.Experiments {
		nodes = append(nodes, node{n.ID, []field{{fieldID, n.ID}, {fieldTitle, n.Title}, {StatusKey, n.Status},
			{RigorKey, rigorValue(n.Rigor)}, {ManuallyScoredKey, n.ManuallyScored}}})
	}
	for _, n := range g.Works {
		nodes = append(nodes, node{n.ID, []field{{fieldID, n.ID}, {fieldTitle, n.Title}, {RigorKey, rigorValue(n.Rigor)},
			{fieldRationale, n.Rationale}}})
	}
	for _, n := range g.Axioms {
		nodes = append(nodes, node{n.ID, []field{{fieldID, n.ID}, {fieldTitle, n.Title}}})
	}
	slices.SortStableFunc(nodes, func(a, b node) int { return strings.Compare(a.id, b.id) })
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(valuesLine(t, n.fields))
	}
	for _, e := range g.Edges() {
		fields := []field{{fieldRelation, e.Relation}, {fieldSource, e.Source}, {fieldTarget, e.Target}, {fieldContext, e.Context}}
		switch e.Relation {
		case RelationReferences:
		case RelationDemoted:
			fields = append(fields, field{fieldReplacedRelation, e.ReplacedRelation}, field{BearingKey, bearingValue(*e.Bearing)},
				field{ManuallyScoredKey, e.ManuallyScored})
		case RelationFounds:
			fields = append(fields, field{BearingKey, bearingValue(*e.Bearing)})
		default:
			fields = append(fields, field{BearingKey, bearingValue(*e.Bearing)}, field{ManuallyScoredKey, e.ManuallyScored})
		}
		b.WriteString(valuesLine(t, fields))
	}
	return b.String()
}

// TestRecordFilesDecodeToTheirValues: each 3.0.0 tree's two record files
// decode, with no fault, to the values its golden <tree>.values.jsonl holds —
// one JSON object per record, in the files' order, keys in each type's field
// order, a pending value as the literal. A reader of the dialect in any
// language is conformant where it reads the same record files to the same
// values.
func TestRecordFilesDecodeToTheirValues(t *testing.T) {
	trees, err := filepath.Glob(filepath.Join(formatsTestdata, "*-"+Version300))
	if err != nil || len(trees) == 0 {
		t.Fatalf("no %s trees under %s: %v", Version300, formatsTestdata, err)
	}
	for _, tree := range trees {
		name := filepath.Base(tree) + ".values.jsonl"
		t.Run(name, func(t *testing.T) {
			var g Graph
			for _, file := range []struct {
				rel    string
				decode func([]byte, string, *Graph) []GraphFault
			}{{NodesFile, DecodeNodes}, {EdgesFile, DecodeEdges}} {
				data, err := os.ReadFile(filepath.Join(tree, "kb-root", filepath.FromSlash(file.rel)))
				if err != nil {
					t.Fatal(err)
				}
				if faults := file.decode(data, file.rel, &g); len(faults) > 0 {
					t.Fatalf("%s: %+v", file.rel, faults)
				}
			}
			got := decodedValues(t, g)
			want, err := os.ReadFile(filepath.Join(formatsTestdata, name))
			if err == nil && got == string(want) {
				return
			}
			if err := os.MkdirAll(valuesGotDir, 0o755); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(valuesGotDir, name)
			if err := os.WriteFile(out, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Errorf("the decoded values differ from %s (%v); the decode is at %s", name, err, out)
		})
	}
}

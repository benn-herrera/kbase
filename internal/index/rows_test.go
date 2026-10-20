package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// roundTripState holds a record of every index file, every node kind and
// every relation.
func roundTripState() kb.State {
	ctx := "a reading"
	return kb.State{
		ClaimEntries: []kb.ClaimEntry{{
			ID: "clm-aaaaaa", Title: "A", CanonicalPath: "a/leaf.md", CanonicalAnchor: "a", Rigor: f(0.8), Rationale: "why",
			Premises: []kb.Edge{
				{Source: "clm-bbbbbb", Target: "clm-aaaaaa", Relation: kb.RelationSupports, TargetKind: "claim", Context: &ctx, Bearing: kb.Bearing{Value: 0.5}},
				{Source: "axm-S1", Target: "clm-aaaaaa", Relation: kb.RelationFounds, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.8}},
				{Source: "wrk-x", Target: "clm-aaaaaa", Relation: kb.RelationFounds, TargetKind: "claim", Bearing: kb.Bearing{Pending: true}},
			},
			References:   []kb.Edge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationReferences, TargetKind: "claim"}},
			Demoted:      []kb.Edge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationDemoted, TargetKind: "claim", Origin: kb.OriginCited}},
			StrengthenBy: []kb.StrengthenByItem{{ClaimID: "clm-aaaaaa", Text: "Run exp-cccccc.", MentionedIDs: []string{"exp-cccccc"}}},
		}, {ID: "clm-bbbbbb", Title: "B", CanonicalPath: "a/leaf.md", CanonicalAnchor: "b", Rigor: f(0.5)}},
		Axioms: []kb.AxiomNode{{NodeType: kb.NodeKindAxiom, ID: "axm-S1", Title: "S1", CanonicalPath: "axioms.md", CanonicalAnchor: "axiom-s1-the-rule"}},
		Works:  []kb.ExternalWork{{ID: "wrk-x", Title: "X", CanonicalPath: "works.md", CanonicalAnchor: "x", Rigor: f(0.7)}},
		Analyses: []kb.AnalysisNode{{ID: "ana-dddddd", Title: "D", CanonicalPath: "a/leaf.md", CanonicalAnchor: "d", Rigor: f(0.6),
			Lifts: []kb.Edge{{Source: "ana-dddddd", Target: "clm-bbbbbb", Relation: kb.RelationLifts, TargetKind: "claim", Bearing: kb.Bearing{Pending: true}}}}},
		Experiments: []kb.ExperimentNode{{ID: "exp-cccccc", Title: "C", CanonicalPath: "a/leaf.md", CanonicalAnchor: "c", Status: "run", Rigor: f(1),
			Refutes: []kb.Edge{{Source: "exp-cccccc", Target: "clm-aaaaaa", Relation: kb.RelationRefutes, TargetKind: "claim", Bearing: kb.Bearing{Value: 0.9}}}}},
		Leaves:  []kb.LeafRecord{{Path: "a/leaf.md", Kind: kb.DocumentLeaf, Claims: []string{"clm-aaaaaa", "clm-bbbbbb"}, Tier2Marked: map[string]bool{"clm-aaaaaa": true}}},
		Indexes: []kb.IndexRecord{{Path: kb.EntryPointFile, Kind: kb.DocumentEntryPoint}, {Path: "a/index.md", Kind: kb.DocumentIndex}},
	}
}

// roundTrip reads T's file of the index under src and writes its rows back:
// the bytes are the writer's.
func roundTrip[T interface {
	kb.IndexRowType
	IndexFields() []kb.IndexField
}](t *testing.T, src *kb.Source, recs Records) {
	t.Helper()
	path, rows, problems, err := kb.ReadIndex[T](src, nil)
	if err != nil || problems != nil {
		t.Fatalf("%s: %v %v", path, err, problems)
	}
	name := strings.TrimSuffix(filepath.Base(path), kb.IndexFileExt)
	want := Serialize(recs[name])
	if want == "" {
		t.Errorf("%s: the fixture writes no record", name)
	}
	if got := Serialize(fieldsOf(rows)); got != want {
		t.Errorf("%s reads back as\n%s\nthe writer wrote\n%s", name, got, want)
	}
}

// TestIndexReadsBackAsWritten: every index file the writer emits decodes
// through kb's row types into rows that emit the same bytes.
func TestIndexReadsBackAsWritten(t *testing.T) {
	recs, err := BuildRecords(roundTripState())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, kb.IndexDir), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range kb.IndexFiles {
		if err := os.WriteFile(filepath.Join(root, kb.IndexDir, kb.IndexFileName(name)), []byte(Serialize(recs[name])), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := kb.OnDisk(root)
	roundTrip[kb.NodeRow](t, src, recs)
	roundTrip[kb.EdgeRow](t, src, recs)
	roundTrip[kb.StrengthenByRow](t, src, recs)
	roundTrip[kb.CiteRow](t, src, recs)
	roundTrip[kb.AggregateRow](t, src, recs)
}

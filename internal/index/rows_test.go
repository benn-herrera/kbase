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
			ID: "clm-aaaaaa", Title: "A", CanonicalPath: "a/leaf.md", CanonicalAnchor: "a", Confidence: f(0.8), Rationale: "why",
			DependsOn: []kb.Edge{
				{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationDepends, TargetKind: "claim", Context: &ctx},
				{Source: "clm-aaaaaa", Target: "INV-1", Relation: kb.RelationDepends, TargetKind: "invariant"},
				{Source: "clm-aaaaaa", Target: "work-x", Relation: kb.RelationRestsOn, TargetKind: "work", Fraction: kb.Fraction{Set: true, Value: 0.5}},
			},
			References:   []kb.Edge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationReferences, TargetKind: "claim"}},
			Demoted:      []kb.Edge{{Source: "clm-aaaaaa", Target: "clm-bbbbbb", Relation: kb.RelationDemoted, TargetKind: "claim", Origin: kb.OriginCited}},
			StrengthenBy: []kb.StrengthenByItem{{ClaimID: "clm-aaaaaa", Text: "Run exp-cccccc.", MentionedIDs: []string{"exp-cccccc"}}},
		}, {ID: "clm-bbbbbb", Title: "B", CanonicalPath: "a/leaf.md", CanonicalAnchor: "b", Confidence: f(0.5)}},
		FrameworkNodes: []kb.FrameworkNode{{NodeType: "invariant", ID: "INV-1", Title: "I", CanonicalPath: "invariants.md", CanonicalAnchor: "inv-1"}},
		Works:          []kb.ExternalWork{{ID: "work-x", Title: "X", CanonicalPath: "works.md", CanonicalAnchor: "x", Strength: f(0.7)}},
		Supports: []kb.SupportNode{{ID: "sup-dddddd", Title: "D", CanonicalPath: "a/leaf.md", CanonicalAnchor: "d", Quality: f(0.6),
			Supports: []kb.SupportPair{{ClaimID: "clm-bbbbbb", Fraction: kb.Fraction{Set: true, Pending: true}}}}},
		Experiments: []kb.ExperimentNode{{ID: "exp-cccccc", Title: "C", CanonicalPath: "a/leaf.md", CanonicalAnchor: "c", Status: "planned",
			Strengthens: []kb.StrengthensPair{{ClaimID: "clm-aaaaaa", Strength: 0.9}}}},
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
	roundTrip[kb.SupportedByRow](t, src, recs)
	roundTrip[kb.CiteRow](t, src, recs)
	roundTrip[kb.AggregateRow](t, src, recs)
}

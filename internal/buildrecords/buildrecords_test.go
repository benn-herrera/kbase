package buildrecords

import (
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/asks"
	"kbase/internal/kb"
)

// TestRecordRead reads a record from its YAML file beside kb-root; a JSON
// file of the superseded spelling is not read.
func TestRecordRead(t *testing.T) {
	repo := t.TempDir()
	src := kb.OnDisk(filepath.Join(repo, kb.KBDir))
	if err := os.WriteFile(filepath.Join(repo, "kb-build-node-pass.json"),
		[]byte(`{"about": "old", "leaves": {"b.md": {"state": "unread", "outcome": null, "claims": [], "verdicts": []}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := ReadNodePass(src); ok || err != nil {
		t.Fatalf("with only the JSON spelling standing the record read as present (%t, %v)", ok, err)
	}
	if err := os.WriteFile(filepath.Join(repo, NodePassFile),
		[]byte("about: new\nleaves:\n  a.md:\n    state: landed\n    outcome: no-claim\n    claims: []\n    verdicts: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, ok, err := ReadNodePass(src)
	if err != nil || !ok || r.Leaves["a.md"].State != "landed" {
		t.Errorf("the YAML record read back as %+v, %t, %v", r, ok, err)
	}
}

// TestInferredLinks: an unmarked yes counts in its written direction only, a
// cull link either way round, an unmarked no not at all.
func TestInferredLinks(t *testing.T) {
	yes, no := asks.LetterPoints, asks.LetterDoesNotPoint
	inferred := InferredLinks(
		UnmarkedRecord{Pairs: []CandidateEntry{{Source: "u1", Target: "u2", Letter: &yes}, {Source: "n1", Target: "n2", Letter: &no}, {Source: "d1", Target: "d2"}}},
		CullRecord{Links: []CullLink{{A: "c1", B: "c2", Harvest: "adjacent"}}},
		yes)
	for _, tc := range []struct {
		name, source, target string
		want                 bool
	}{
		{"unmarked yes, written direction", "u1", "u2", true},
		{"unmarked yes, reversed", "u2", "u1", false},
		{"unmarked no", "n1", "n2", false},
		{"unmarked, no letter", "d1", "d2", false},
		{"cull link, as recorded", "c1", "c2", true},
		{"cull link, reversed", "c2", "c1", true},
		{"in no record", "x1", "x2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := inferred(tc.source, tc.target); got != tc.want {
				t.Errorf("inferred(%s, %s) = %t, want %t", tc.source, tc.target, got, tc.want)
			}
		})
	}
	if InferredLinks(UnmarkedRecord{}, CullRecord{}, yes)("u1", "u2") {
		t.Error("with every record absent an edge read as inferred")
	}
}

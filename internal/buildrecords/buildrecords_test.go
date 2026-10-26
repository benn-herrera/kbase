package buildrecords

import (
	"os"
	"path/filepath"
	"testing"

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

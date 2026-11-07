package claimgraph

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/kb"
	"kbase/internal/result"
)

// TestNodePassWrittenAsReference checks the writer's normalization: leaves by
// path, verdicts by line, nil lists written as [].
func TestNodePassWrittenAsReference(t *testing.T) {
	cause := CauseNoLetter
	got, err := nodePassBytes(buildrecords.NodePassRecord{Leaves: map[string]buildrecords.LeafEntry{
		"b.md": {State: ReadLanded, Verdicts: []buildrecords.ParagraphVerdict{{Line: 9, Verdict: JudgementDefaulted, Cause: &cause}, {Line: 2, Verdict: JudgementNotAClaim}}},
		"a.md": {State: ReadUnread},
	}})
	want := "about: \"" + nodePassAbout + "\"\nleaves:\n" +
		"  a.md:\n    state: unread\n    outcome: null\n    claims: []\n    verdicts: []\n" +
		"  b.md:\n    state: landed\n    outcome: null\n    claims: []\n    verdicts:\n" +
		"      - line: 2\n        verdict: not-a-claim\n        cause: null\n" +
		"      - line: 9\n        verdict: defaulted\n        cause: no-letter\n"
	if err != nil || string(got) != want {
		t.Errorf("nodePassBytes = %s, %v; want %s", got, err, want)
	}
}

// TestBuildRecordWriteRefusesASymlink: a build record that is a symbolic link
// out of kb-root/ is refused at its write, naming it, and the file the link
// names keeps its bytes.
func TestBuildRecordWriteRefusesASymlink(t *testing.T) {
	repo := t.TempDir()
	link := filepath.Join(repo, filepath.FromSlash(buildrecords.NodePassFile))
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "node-pass.yaml")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	err := WriteNodePass(repo, buildrecords.NodePassRecord{})
	var r result.Refusal
	if !errors.As(err, &r) || len(r) != 1 || r[0].Check != kb.CheckSymlink || r[0].Path != link {
		t.Fatalf("WriteNodePass over a symlinked record = %v, want one %s refusal naming %s", err, kb.CheckSymlink, link)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside\n" {
		t.Errorf("%s = %q (%v), want it untouched", outside, data, err)
	}
}

package claimgraph

import (
	"testing"

	"kbase/internal/buildrecords"
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

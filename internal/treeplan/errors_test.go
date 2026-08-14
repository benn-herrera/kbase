package treeplan

import (
	"strings"
	"testing"

	"kbase/internal/dissect"
	"kbase/internal/survey"
)

// The two renderings, and the difference between them. Error is
// operator-facing and locates the failure; Note is model-facing and names
// nothing a model could act on.
func TestErrorRenderings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		wantIn  []string
		wantOut []string
	}{{
		name:   "a located rejection",
		err:    RejectionError{Subject: "guide/sync.md", Reason: "too many entries under one heading"},
		wantIn: []string{"treeplan:", "guide/sync.md", "too many entries"},
	}, {
		name:    "a rejection with nothing to locate",
		err:     RejectionError{Reason: "no groups; every candidate belongs to one group"},
		wantIn:  []string{"treeplan:", "no groups"},
		wantOut: []string{"::"},
	}, {
		name:   "a located defect",
		err:    DefectError{Subject: "g0007", Reason: "part 2 is on no page"},
		wantIn: []string{"treeplan:", "g0007", "part 2"},
	}, {
		name:    "a defect with nothing to locate",
		err:     DefectError{Reason: "the artifact holds no nodes"},
		wantIn:  []string{"treeplan:", "no nodes"},
		wantOut: []string{"::"},
	}, {
		name: "a starved span",
		err: StarvedRejection{Subject: "Property Type Support", File: "properties.md",
			Starved: dissect.StarvedRejection{Span: survey.Span{Start: 281, End: 4736}, Tokens: 1114, Budget: 800}},
		wantIn: []string{"treeplan:", "Property Type Support", "properties.md", "281"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			for _, want := range tc.wantIn {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, missing %q", got, want)
				}
			}
			for _, unwanted := range tc.wantOut {
				if strings.Contains(got, unwanted) {
					t.Errorf("Error() = %q, carries %q", got, unwanted)
				}
			}
			if r, ok := AsRejection(tc.err); ok {
				assertNoteIsPromptable(t, r)
				if strings.Contains(r.Note(), "treeplan:") {
					t.Errorf("Note() = %q carries the operator prefix", r.Note())
				}
			}
		})
	}
}

// A defect is not a rejection: the seam maps it to a loud abort, so misfiling
// one as retryable would re-ask a question that was never asked wrong.
func TestDefectIsNotARejection(t *testing.T) {
	if _, ok := AsRejection(DefectError{Reason: "two nodes share a path"}); ok {
		t.Fatal("a defect classified as a rejection would be retried instead of aborted")
	}
}

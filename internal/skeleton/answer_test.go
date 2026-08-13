package skeleton

import (
	"errors"
	"strings"
	"testing"
)

// group is one answer group, spelled short because these tables are all shape.
func group(title string, kind Kind, members ...int) AnswerGroup {
	return AnswerGroup{Title: title, Scope: "one line about " + title, Kind: kind, Members: members}
}

func TestCheckAnswer(t *testing.T) {
	v, _ := verifierFor(t, testParams(), docSpec{path: "a.md", title: "A", secs: []secSpec{{title: "One", paras: 1, words: 40}}})

	for _, tc := range []struct {
		name       string
		answer     Answer
		candidates int
		depth      int
		wantOK     bool
		wantDefect bool
		wantIn     string
	}{{
		name:       "a partition of three into two",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0, 2), group("Reference", KindIndex, 1)}},
		candidates: 3, depth: 2, wantOK: true,
	}, {
		name:       "one group of one is legal",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0)}},
		candidates: 1, depth: 2, wantOK: true,
	}, {
		name:       "a dropped candidate",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0)}},
		candidates: 2, depth: 2, wantIn: "left out",
	}, {
		name:       "a duplicated candidate",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0), group("Other", KindLeaf, 0, 1)}},
		candidates: 2, depth: 2, wantIn: "two groups",
	}, {
		name:       "a candidate nobody offered",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0, 7)}},
		candidates: 2, depth: 2, wantIn: "not on the list",
	}, {
		name:       "an empty group",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0, 1), group("Empty", KindLeaf)}},
		candidates: 2, depth: 2, wantIn: "holds no entries",
	}, {
		name:       "no groups at all",
		answer:     Answer{},
		candidates: 2, depth: 2, wantIn: "no groups",
	}, {
		name: "over the fan-out cap",
		answer: Answer{Groups: []AnswerGroup{
			group("A", KindLeaf, 0), group("B", KindLeaf, 1), group("C", KindLeaf, 2),
			group("D", KindLeaf, 3), group("E", KindLeaf, 4),
		}},
		candidates: 5, depth: 2, wantIn: "too many groups",
	}, {
		name:       "an untitled group",
		answer:     Answer{Groups: []AnswerGroup{group("   ", KindLeaf, 0)}},
		candidates: 1, depth: 2, wantIn: "no title",
	}, {
		name: "two groups sharing a title",
		answer: Answer{Groups: []AnswerGroup{
			group("Setup", KindLeaf, 0), group("Setup", KindLeaf, 1)}},
		candidates: 2, depth: 2, wantIn: "share a title",
	}, {
		name: "a group with no scope line",
		answer: Answer{Groups: []AnswerGroup{
			{Title: "Setup", Kind: KindLeaf, Members: []int{0}}}},
		candidates: 1, depth: 2, wantIn: "no one-line scope",
	}, {
		name:       "a group that is neither page nor section",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", "annex", 0)}},
		candidates: 1, depth: 2, wantIn: "neither a page nor a section",
	}, {
		name:       "a section asked for below the last level",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindIndex, 0)}},
		candidates: 1, depth: 5, wantIn: "no level left",
	}, {
		name:       "a page at the last level is fine",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0)}},
		candidates: 1, depth: 5, wantOK: true,
	}, {
		name:       "a candidate list past the cap is our defect",
		answer:     Answer{Groups: []AnswerGroup{group("Setup", KindLeaf, 0)}},
		candidates: testBudgets().CandidateCap + 1, depth: 2, wantDefect: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := v.CheckAnswer(tc.answer, tc.candidates, tc.depth)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("want accepted, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("want refused, got accepted")
			}
			var defect DefectError
			if got := errors.As(err, &defect); got != tc.wantDefect {
				t.Fatalf("defect = %v, want %v (err: %v)", got, tc.wantDefect, err)
			}
			if tc.wantDefect {
				return
			}
			r, ok := AsRejection(err)
			if !ok {
				t.Fatalf("a model-attributable failure is a Rejection; got %T", err)
			}
			if !strings.Contains(r.Note(), tc.wantIn) {
				t.Fatalf("note %q does not mention %q", r.Note(), tc.wantIn)
			}
			assertNoteIsPromptable(t, r)
		})
	}
}

// A note becomes the corrective note of the one informed retry
// (pipeline.correctiveNoteWords), so it is short, it names no node, and it
// carries no path — a path in it would be a node name the model is forbidden
// to emit, sitting in the very next prompt (I-2).
func assertNoteIsPromptable(t *testing.T, r Rejection) {
	t.Helper()
	note := r.Note()
	if note == "" {
		t.Fatal("a rejection with no note gives the retry nothing to work with")
	}
	if n := len(strings.Fields(note)); n > 12 {
		t.Errorf("note is %d words, over the 12 the retry carries: %q", n, note)
	}
	for _, banned := range []string{"/", ".md", "\n"} {
		if strings.Contains(note, banned) {
			t.Errorf("note carries %q, which is node-naming or multi-line: %q", banned, note)
		}
	}
}

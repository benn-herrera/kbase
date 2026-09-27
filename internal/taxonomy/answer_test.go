package taxonomy

import (
	"strings"
	"testing"

	"kbase/internal/treeplan"
)

// The line grammar, read: what parseAnswer takes and what it refuses.
//
// The wrapper cases are the point of the reshape and they are grouped first: a
// fence, a preamble, a closing remark and a bulleted line are all noise the
// reader steps over, so the failure class that produced them cannot reject an
// answer any more. What remains refusable is the shape of a line that CLAIMS to
// be a group, and a response with no group line in it at all.
func TestParseAnswerReadsTheLineGrammar(t *testing.T) {
	const clean = "group :: Setup :: how to get started :: page :: 1, 2\n" +
		"group :: Reference :: every option, listed :: section :: 3"

	want := []treeplan.AnswerGroup{
		{Title: "Setup", Scope: "how to get started", Kind: treeplan.KindLeaf, Members: []int{0, 1}},
		{Title: "Reference", Scope: "every option, listed", Kind: treeplan.KindIndex, Members: []int{2}},
	}

	for _, tc := range []struct {
		name     string
		response string
	}{
		{"clean answer", clean},
		{"fenced answer", "```\n" + clean + "\n```"},
		{"prose-wrapped answer", "Here is the grouping:\n\n" + clean + "\n\nLet me know if that works."},
		{"bulleted lines", "- " + strings.ReplaceAll(clean, "\n", "\n- ")},
		{"emphasised keyword", strings.ReplaceAll(clean, "group ::", "**group** ::")},
		{"loose spacing", strings.ReplaceAll(clean, " :: ", "::")},
		{"trailing comma in the members", strings.ReplaceAll(clean, "1, 2", "1, 2,")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAnswer(tc.response)
			if err != nil {
				t.Fatalf("parseAnswer: %v", err)
			}
			if len(got.Groups) != len(want) {
				t.Fatalf("%d groups, want %d: %+v", len(got.Groups), len(want), got.Groups)
			}
			for i, g := range got.Groups {
				if g.Title != want[i].Title || g.Scope != want[i].Scope || g.Kind != want[i].Kind {
					t.Errorf("group %d = %+v, want %+v", i+1, g, want[i])
				}
				if len(g.Members) != len(want[i].Members) {
					t.Fatalf("group %d members = %v, want %v", i+1, g.Members, want[i].Members)
				}
				for j, m := range g.Members {
					if m != want[i].Members[j] {
						t.Errorf("group %d members = %v, want %v", i+1, g.Members, want[i].Members)
					}
				}
			}
		})
	}
}

// TestParseAnswerRefusesADefectiveLine: the strict half. A line that claims to
// be a group is held to the grammar, and the note the retry carries quotes it.
func TestParseAnswerRefusesADefectiveLine(t *testing.T) {
	for _, tc := range []struct {
		name, response, wantNote string
	}{
		{"no group line at all", "I would group these by topic.", "answer one group line per group"},
		{"too few fields", "group :: Setup :: how to get started :: page", "five fields per group line"},
		{"too many fields", "group :: Setup :: what :: it :: does :: page :: 1", "five fields per group line"},
		{"an entry number that is not a number", "group :: Setup :: how :: page :: one", "entry numbers are plain numbers"},
		{"an entry range", "group :: Setup :: how :: page :: 1-3", "entry numbers are plain numbers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAnswer(tc.response)
			if err == nil {
				t.Fatal("parseAnswer accepted a defective answer")
			}
			rej, ok := treeplan.AsRejection(err)
			if !ok {
				t.Fatalf("parseAnswer returned %v, want a rejection the retry can carry", err)
			}
			if !strings.Contains(rej.Note(), tc.wantNote) {
				t.Errorf("note = %q, want it to state %q", rej.Note(), tc.wantNote)
			}
		})
	}
}

// TestADefectiveLineIsNamedInTheNote: the retry has to be able to say WHICH
// line was wrong, or the model is being asked to guess which of its lines to
// rewrite. The quotation is capped (noteLineWords) so the mechanical fact
// survives the runner's note budget; what is asserted is that the line's own
// opening words are in there.
func TestADefectiveLineIsNamedInTheNote(t *testing.T) {
	_, err := parseAnswer("group :: Configuring the daemon :: what it does\ngroup :: A :: B :: page :: 1")
	rej, ok := treeplan.AsRejection(err)
	if !ok {
		t.Fatalf("parseAnswer returned %v, want a rejection", err)
	}
	if !strings.Contains(rej.Note(), "Configuring") {
		t.Errorf("note = %q, want the offending line quoted in it", rej.Note())
	}
	// And the operator's rendering carries the line whole, with its ordinal.
	if got := err.Error(); !strings.Contains(got, "line 1") || !strings.Contains(got, "what it does") {
		t.Errorf("operator rendering = %q, want the line's ordinal and the line", got)
	}
}

// TestEmptyValuesReachTheSemanticVerifier: the parser does not re-state a rule
// CheckAnswer already owns. An empty title, an empty scope, an unknown kind and
// an empty member list all PARSE — and every one of them is refused by the
// verifier, in its own words. Two spellings of one rule is how one of them ends
// up wrong.
func TestEmptyValuesReachTheSemanticVerifier(t *testing.T) {
	got, err := parseAnswer("group ::  ::  :: chapter :: ")
	if err != nil {
		t.Fatalf("parseAnswer: %v", err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("%d groups, want the line to have parsed", len(got.Groups))
	}
	g := got.Groups[0]
	if g.Title != "" || g.Scope != "" || len(g.Members) != 0 {
		t.Errorf("group = %+v, want the empty values passed through", g)
	}
	if g.Kind == treeplan.KindLeaf || g.Kind == treeplan.KindIndex {
		t.Errorf("kind %q was mapped onto the closed set; the verifier owns that refusal", g.Kind)
	}
}

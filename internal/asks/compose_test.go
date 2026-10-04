package asks

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestRenderFillsEverySlotOnce(t *testing.T) {
	body := `S1: a \frac{a}{b} with @!dyn.body!@ and {braces}`
	got, err := Render(paragraphTemplate, map[string]string{
		"document": "vol/leaf.md", "body": body, "paragraph": "S1", "paragraph-text": "S1: text", "returned": "",
	}, letterConstants, map[string]string{"correction": "letter-correction"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"`vol/leaf.md`", body, "The paragraph is S1:", "**A — states a result**", "Answer A if it does, B if it does not",
		"## An earlier answer to this question could not be used"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered prompt lacks %q", want)
		}
	}
	if strings.Count(got, "@!dyn.body!@") != 1 {
		t.Error("a filled value was rescanned, or a slot left unfilled")
	}
}

func TestRenderIsStrictBothWays(t *testing.T) {
	full := func() map[string]string {
		return map[string]string{"document": "d", "body": "b", "paragraph": "S1", "paragraph-text": "t"}
	}
	none := map[string]string{"correction": ""}
	for _, tc := range []struct {
		name         string
		slots        map[string]string
		alternatives map[string]string
		want         string
	}{
		{"a dyn slot unsupplied", func() map[string]string { s := full(); delete(s, "body"); return s }(), none, "no value for @!dyn.body!@"},
		{"a value never used", func() map[string]string { s := full(); s["excerpts"] = "x"; return s }(), none, "never uses"},
		{"a composer slot supplied", func() map[string]string { s := full(); s["letter-claim"] = "Z"; return s }(), none, "which the composer fills"},
		{"no choice for a declared alternative", full(), map[string]string{}, "no alternative chosen for slot correction"},
		{"a choice for an undeclared slot", full(), map[string]string{"correction": "", "classify-options": "classify-options-two"}, "does not declare"},
		{"an unregistered choice", full(), map[string]string{"correction": "overview-correction"}, "takes one of"},
		{"the alternative's own slot unsupplied", full(), map[string]string{"correction": "letter-correction"}, "no value for @!dyn.returned!@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Render(paragraphTemplate, tc.slots, letterConstants, tc.alternatives)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Render = %v, want an error naming %q", err, tc.want)
			}
		})
	}
	if _, err := Render(paragraphTemplate, full(), nil, none); err == nil || !strings.Contains(err.Error(), "unfilled slot") {
		t.Errorf("Render with no constants = %v, want an unfilled slot", err)
	}
}

func TestSlotsOfRefusesStrayDelimiters(t *testing.T) {
	for _, text := range []string{"an @!open slot", "a close!@ alone", "@!Bad_Name!@"} {
		if _, err := slotsOf(text, "t"); err == nil {
			t.Errorf("slotsOf(%q) accepted a stray delimiter", text)
		}
	}
	if got, err := slotsOf("@!a!@ @!dyn.a!@ @!a!@", "t"); err != nil || !slices.Equal(got, []string{"a", "dyn.a"}) {
		t.Errorf("slotsOf = %q, %v", got, err)
	}
}

// TestFragmentsAreEachRegisteredOnce: every file under fragments/ is a
// composer-resolved fragment or a registered alternative, exactly one of the
// two, and every registered name is a file on the shelf.
func TestFragmentsAreEachRegisteredOnce(t *testing.T) {
	files, err := shelfFiles()
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]int{}
	for _, f := range fragmentSlots {
		registered[fragmentFile(f)]++
	}
	for _, choices := range alternativeSlots {
		for _, c := range choices {
			if c != "" {
				registered[fragmentFile(c)]++
			}
		}
	}
	for f, n := range registered {
		if n != 1 || !slices.Contains(files, f) {
			t.Errorf("%s registered %d times, on the shelf %t", f, n, slices.Contains(files, f))
		}
	}
	for _, f := range files {
		if strings.HasPrefix(f, fragmentsDir+"/") && registered[f] == 0 {
			t.Errorf("%s is on the shelf and registered nowhere", f)
		}
	}
}

func TestSystemPromptsAreTheFragmentsWhole(t *testing.T) {
	for _, name := range fragmentSlots {
		got, err := System(name)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := load(fragmentFile(name))
		if got != want {
			t.Errorf("System(%s) = %q, want the fragment's bytes", name, got)
		}
	}
	if _, err := System("letter-correction"); err == nil {
		t.Error("an alternative was served as a system prompt")
	}
}

func TestLint(t *testing.T) {
	if found, err := Lint(nil); err != nil || len(found) != 0 {
		t.Errorf("Lint over the shelf = %q, %v; want no stray delimiter", found, err)
	}
	found, err := Lint(map[string]*regexp.Regexp{"knowledge base": regexp.MustCompile(`(?i)knowledge base`)})
	if err != nil || len(found) == 0 || !strings.Contains(found[0], `names "knowledge base"`) {
		t.Errorf("Lint with a prohibited phrase the shelf spells = %q, %v", found, err)
	}
}

func TestClassifyAsksNumberPassagesOnce(t *testing.T) {
	src := Claim{ID: "clm-aaaaaa", Title: "Theorem 1", Document: "v/a.md", Locator: "**Theorem 1**."}
	items := ClassifyAsks(src, "the statement", []ClassifyItem{
		{Target: Claim{ID: "clm-bbbbbb", Title: "Lemma 2", Document: "v/b.md"}, Statement: "lemma", Passages: []string{"zeta", "alpha"},
			Offered: []string{LetterMention, LetterSupportedBy, LetterInSupportOf}},
		{Target: Claim{ID: "clm-cccccc", Title: "(1)", Document: "v/b.md"}, Statement: "x = y", Passages: []string{"zeta"},
			Offered: []string{LetterSupportedBy, LetterMention}},
	})
	if !slices.Equal(items[0].Offered, []string{"A", "B", "C"}) || !slices.Equal(items[1].Offered, []string{"A", "C"}) {
		t.Errorf("offered = %q, %q", items[0].Offered, items[1].Offered)
	}
	first, err := items[0].Compose(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := items[1].Compose(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- `clm-aaaaaa` — Theorem 1 (stated in `v/a.md`: **Theorem 1**.)", "P1: alpha\nP2: zeta", "points at it in P1, P2.", "does the candidate need the source"} {
		if !strings.Contains(first, want) {
			t.Errorf("first ask lacks %q", want)
		}
	}
	for _, want := range []string{"- `clm-cccccc` — (1) (stated in `v/b.md`)", "points at it in P2.", "This question offers two letters."} {
		if !strings.Contains(second, want) {
			t.Errorf("second ask lacks %q", want)
		}
	}
	if sharedPrefix([]string{first, second}) == "" || !strings.HasPrefix(first, sharedPrefix([]string{first, second})) {
		t.Error("a group's asks share no prefix")
	}
	if !strings.Contains(sharedPrefix([]string{first, second}), "P1: alpha") {
		t.Error("the group's passages are not part of its shared prefix")
	}
}

func TestOverviewPrompt(t *testing.T) {
	first, err := OverviewPrompt("==> Knowledge Base <==", nil)
	if err != nil || !strings.HasSuffix(first, "name it by the title the excerpts give.\n") {
		t.Errorf("first ask = %q, %v", first, err)
	}
	again, err := OverviewPrompt("==> Knowledge Base <==", []string{"# Overview", "- a list"})
	if err != nil || !strings.Contains(again, "````````````text\n# Overview\n- a list\n````````````") {
		t.Errorf("re-ask = %q, %v", again, err)
	}
}

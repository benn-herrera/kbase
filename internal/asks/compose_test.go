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
		"document": "vol/leaf.md", "body": body, "paragraph": "S2-S5", "paragraph-text": "S2: text",
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"vol/leaf.md", body, "S2-S5", "S2: text"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered state lacks %q", want)
		}
	}
	if strings.Count(got, "@!dyn.body!@") != 1 || strings.Count(got, "@!") != strings.Count(body, "@!") {
		t.Error("a filled value was rescanned, or a slot left unfilled")
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
	for _, name := range systemSlots {
		got, err := System(name)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := load(fragmentFile(name))
		if got != want {
			t.Errorf("System(%s) = %q, want the fragment's bytes", name, got)
		}
	}
	for _, name := range []string{"overview-correction", scoreAnswerOccurrences, scoreAnswerClose} {
		if _, err := System(name); err == nil {
			t.Errorf("%s was served as a system prompt", name)
		}
	}
}

func TestLint(t *testing.T) {
	if found, err := Lint(nil); err != nil || len(found) != 0 {
		t.Errorf("Lint over the shelf = %q, %v; want no stray delimiter", found, err)
	}
	found, err := Lint(map[string]*regexp.Regexp{"the body slot": regexp.MustCompile(`@!dyn\.body!@`)})
	if err != nil || len(found) == 0 || !strings.Contains(found[0], `names "the body slot"`) {
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
	// The two offered sets carry distinct questions keyed by their criteria:
	// the three-letter question names all three letters, the two-letter one
	// names supported-by and mention only, never in-support-of.
	if !slices.Contains(keys(items[0].Question.Criteria), LetterInSupportOf) {
		t.Errorf("the three-letter question offers %q", keys(items[0].Question.Criteria))
	}
	if slices.Contains(keys(items[1].Question.Criteria), LetterInSupportOf) {
		t.Errorf("the two-letter question offers in-support-of: %q", keys(items[1].Question.Criteria))
	}
	first, err := items[0].Compose()
	if err != nil {
		t.Fatal(err)
	}
	second, err := items[1].Compose()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- `clm-aaaaaa` — Theorem 1 (stated in `v/a.md`: **Theorem 1**.)", "P1: alpha\nP2: zeta", "P1, P2"} {
		if !strings.Contains(first, want) {
			t.Errorf("first ask lacks %q", want)
		}
	}
	if !strings.Contains(second, "- `clm-cccccc` — (1) (stated in `v/b.md`)") {
		t.Error("second ask lacks its candidate line")
	}
	// named counts a passage number outside the numbered list.
	named := func(prompt, n string) int { return len(regexp.MustCompile(`\b`+n+`\b[^:]`).FindAllString(prompt, -1)) }
	if named(second, "P1") != 0 || named(second, "P2") != 1 {
		t.Errorf("second ask names P1 %d times and P2 %d times, want P2 alone", named(second, "P1"), named(second, "P2"))
	}
	if sharedPrefix([]string{first, second}) == "" || !strings.HasPrefix(first, sharedPrefix([]string{first, second})) {
		t.Error("a group's asks share no prefix")
	}
	if !strings.Contains(sharedPrefix([]string{first, second}), "P1: alpha") {
		t.Error("the group's passages are not part of its shared prefix")
	}
}

// keys is m's keys, sorted.
func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestUnmarkedAsksShareEverythingBeforeTheQuestion(t *testing.T) {
	src := Claim{ID: "clm-aaaaaa", Title: "Theorem 1", Document: "vol/a.md", Locator: "**Theorem 1**."}
	targets := []UnmarkedItem{
		{Target: Claim{ID: "clm-bbbbbb", Title: "Lemma 2", Document: "vol/b.md", Locator: "**Lemma 2**."}, Statement: "**Lemma 2**. The map is a contraction."},
		{Target: Claim{ID: "clm-cccccc", Title: "Equation (3)", Document: "vol/b.md"}, Statement: "$$ k = \\sup |f'| $$"},
	}
	items := UnmarkedAsks("vol/a.md", "S1: We study the map.\n\n> **Theorem 1**. The map has a unique fixed point.\n", src,
		"**Theorem 1**. The map has a unique fixed point.\n", targets)
	var states []string
	for i, it := range items {
		if want := []string{"clm-bbbbbb", "clm-cccccc"}[i]; it.Item != want || !slices.Equal(it.Offered, []string{"A", "B"}) {
			t.Errorf("item %d = %s offered %q, want %s offered A and B", i, it.Item, it.Offered, want)
		}
		if it.Question.Type != "noul" || it.Question.TrueLetter != LetterPoints {
			t.Errorf("item %d's question = %+v, want noul with %s true", i, it.Question, LetterPoints)
		}
		p, err := it.Compose()
		if err != nil {
			t.Fatal(err)
		}
		states = append(states, p)
	}
	prefix := sharedPrefix(states)
	for _, want := range []string{"vol/a.md", "S1: We study the map.", "- `clm-aaaaaa` — Theorem 1 (stated in `vol/a.md`: **Theorem 1**.)", "The map has a unique fixed point."} {
		if !strings.Contains(prefix, want) {
			t.Errorf("the group's shared prefix lacks %q:\n%s", want, prefix)
		}
	}
	for i, p := range states {
		rest := p[len(prefix):]
		if !strings.HasPrefix(rest, targets[i].Target.line()) || !strings.Contains(rest, targets[i].Statement) {
			t.Errorf("ask %d does not go on from the shared prefix with its candidate:\n%s", i, rest)
		}
	}
}

// TestColinearAsksOfferThreeLettersAndCompose: each item goes by the key its
// caller gave, is offered the three letters in their order under one choice
// question keyed by exactly those letters, and composes from the four slots,
// the first claim before the second.
func TestColinearAsksOfferThreeLettersAndCompose(t *testing.T) {
	items := ColinearAsks([]ColinearItem{
		{
			Key:        "clm-bbbbbb",
			First:      Claim{ID: "clm-aaaaaa", Title: "Theorem 1", Document: "vol/a.md", Locator: "**Theorem 1**."},
			FirstText:  "**Theorem 1**. The map has a unique fixed point.\n",
			Second:     Claim{ID: "clm-bbbbbb", Title: "Remark", Document: "vol/a.md"},
			SecondText: "So the fixed point is unique.\n",
		},
		{
			Key:        "clm-cccccc",
			First:      Claim{ID: "clm-cccccc", Title: "Lemma 2", Document: "vol/b.md"},
			FirstText:  "The map is a contraction.",
			Second:     Claim{ID: "clm-dddddd", Title: "Corollary 3", Document: "vol/c.md"},
			SecondText: "The fixed point is attracting.",
		},
	})
	offered := []string{LetterSecondAddsNothing, LetterFirstAddsNothing, LetterDistinct}
	for i, it := range items {
		if want := []string{"clm-bbbbbb", "clm-cccccc"}[i]; it.Item != want || !slices.Equal(it.Offered, offered) {
			t.Errorf("item %d = %s offered %q, want %s offered %q", i, it.Item, it.Offered, want, offered)
		}
		if want := slices.Sorted(slices.Values(offered)); it.Question.Type != "choice" || !slices.Equal(keys(it.Question.Criteria), want) {
			t.Errorf("item %d's question is %s keyed %q, want choice keyed %q", i, it.Question.Type, keys(it.Question.Criteria), want)
		}
	}
	state, err := items[0].Compose()
	if err != nil {
		t.Fatal(err)
	}
	order := []string{
		"- `clm-aaaaaa` — Theorem 1 (stated in `vol/a.md`: **Theorem 1**.)", "The map has a unique fixed point.\n`",
		"- `clm-bbbbbb` — Remark (stated in `vol/a.md`)", "So the fixed point is unique.\n`",
	}
	at := 0
	for _, want := range order {
		n := strings.Index(state[at:], want)
		if n < 0 {
			t.Fatalf("composed state lacks %q after offset %d:\n%s", want, at, state)
		}
		at += n + len(want)
	}
	if strings.Contains(state, "Lemma 2") {
		t.Error("item 0's composed state carries item 1's pair")
	}
}

func TestOverviewPrompt(t *testing.T) {
	first, err := OverviewPrompt("==> Knowledge Base <==", nil)
	if err != nil || !strings.Contains(first, "==> Knowledge Base <==") || strings.Contains(first, "@!") {
		t.Errorf("first ask = %q, %v", first, err)
	}
	again, err := OverviewPrompt("==> Knowledge Base <==", []string{"# Overview", "- a list"})
	if err != nil || !strings.HasPrefix(again, first[:len(first)-1]) || !strings.Contains(again[len(first)-1:], "# Overview\n- a list") {
		t.Errorf("re-ask = %q, %v", again, err)
	}
}

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
		"document": "vol/leaf.md", "body": body, "paragraph": "S2-S5", "paragraph-text": "S2: text", "returned": "an unreadable reply",
	}, letterConstants, map[string]string{"correction": "letter-correction"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"vol/leaf.md", body, "S2-S5", "S2: text", "an unreadable reply"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered prompt lacks %q", want)
		}
	}
	if strings.Count(got, "@!dyn.body!@") != 1 || strings.Count(got, "@!") != strings.Count(body, "@!") {
		t.Error("a filled value was rescanned, or a slot left unfilled")
	}
}

// closingNames reports whether the prompt's last paragraph names letter as a
// word: the closing question carries the letters its ask offers.
func closingNames(prompt, letter string) bool {
	p := strings.TrimRight(prompt, "\n")
	return regexp.MustCompile(`\b` + letter + `\b`).MatchString(p[strings.LastIndex(p, "\n\n")+1:])
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
	first, err := items[0].Compose(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := items[1].Compose(nil)
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
	for _, c := range []struct {
		prompt  string
		letters map[string]bool
	}{{first, map[string]bool{"A": true, "B": true, "C": true}}, {second, map[string]bool{"A": true, "B": false, "C": true}}} {
		for l, offered := range c.letters {
			if closingNames(c.prompt, l) != offered {
				t.Errorf("closing names %s: %t, want %t:\n%s", l, !offered, offered, c.prompt)
			}
		}
	}
	if sharedPrefix([]string{first, second}) == "" || !strings.HasPrefix(first, sharedPrefix([]string{first, second})) {
		t.Error("a group's asks share no prefix")
	}
	if !strings.Contains(sharedPrefix([]string{first, second}), "P1: alpha") {
		t.Error("the group's passages are not part of its shared prefix")
	}
}

func TestUnmarkedAsksShareEverythingBeforeTheQuestion(t *testing.T) {
	src := Claim{ID: "clm-aaaaaa", Title: "Theorem 1", Document: "vol/a.md", Locator: "**Theorem 1**."}
	targets := []UnmarkedItem{
		{Target: Claim{ID: "clm-bbbbbb", Title: "Lemma 2", Document: "vol/b.md", Locator: "**Lemma 2**."}, Statement: "**Lemma 2**. The map is a contraction."},
		{Target: Claim{ID: "clm-cccccc", Title: "Equation (3)", Document: "vol/b.md"}, Statement: "$$ k = \\sup |f'| $$"},
	}
	items := UnmarkedAsks("vol/a.md", "S1: We study the map.\n\n> **Theorem 1**. The map has a unique fixed point.\n", src,
		"**Theorem 1**. The map has a unique fixed point.\n", targets)
	var prompts []string
	for i, it := range items {
		if want := []string{"clm-bbbbbb", "clm-cccccc"}[i]; it.Item != want || !slices.Equal(it.Offered, []string{"A", "B"}) {
			t.Errorf("item %d = %s offered %q, want %s offered A and B", i, it.Item, it.Offered, want)
		}
		p, err := it.Compose(nil)
		if err != nil {
			t.Fatal(err)
		}
		prompts = append(prompts, p)
	}
	prefix := sharedPrefix(prompts)
	for _, want := range []string{"vol/a.md", "S1: We study the map.", "- `clm-aaaaaa` — Theorem 1 (stated in `vol/a.md`: **Theorem 1**.)", "The map has a unique fixed point."} {
		if !strings.Contains(prefix, want) {
			t.Errorf("the group's shared prefix lacks %q:\n%s", want, prefix)
		}
	}
	for i, p := range prompts {
		rest := p[len(prefix):]
		if !strings.HasPrefix(rest, targets[i].Target.line()) || !strings.Contains(rest, targets[i].Statement) {
			t.Errorf("ask %d does not go on from the shared prefix with its candidate:\n%s", i, rest)
		}
		if !closingNames(p, "A") || !closingNames(p, "B") {
			t.Errorf("ask %d's closing does not name both letters:\n%s", i, p)
		}
	}
	again, err := items[0].Compose(new(string))
	if err != nil || !strings.HasPrefix(again, prompts[0][:len(prompts[0])-1]) || again == prompts[0] {
		t.Errorf("the re-ask is not the first ask with its correction after it: %v", err)
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

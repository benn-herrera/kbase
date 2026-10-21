package asks

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// scoreAsk is one score class as the tests put it: the template it renders, the
// ask that renders it, and the closed set it names.
type scoreAsk struct {
	name, template string
	ask            func(string) (string, error)
	table          kb.FaultTable
}

// scoreAsks is the three classes: a claim's statement, an analysis's or an
// experiment's method, and an edge's bearing.
func scoreAsks() []scoreAsk {
	return []scoreAsk{
		{"statement", scoreRigorStatementTemplate, ScoreRigorStatement, kb.RigorStatement},
		{"method", scoreRigorMethodTemplate, ScoreRigorMethod, kb.RigorMethod},
		{"bearing", scoreBearingTemplate, ScoreBearing, kb.BearingFaults},
	}
}

// TestScoreAsksComposeFromTheOneSlotTheyFill: each score ask declares one dyn.
// slot, fills it with the text it is given, splices the shared fragments, and
// leaves no slot or delimiter unfilled.
func TestScoreAsksComposeFromTheOneSlotTheyFill(t *testing.T) {
	for _, c := range scoreAsks() {
		t.Run(c.name, func(t *testing.T) {
			text, err := load(c.template)
			if err != nil {
				t.Fatal(err)
			}
			fields, err := slotsOf(text, c.template)
			if err != nil {
				t.Fatal(err)
			}
			var dyn []string
			for _, f := range fields {
				if strings.HasPrefix(f, dynamicPrefix) {
					dyn = append(dyn, f)
				}
			}
			if len(dyn) != 1 {
				t.Fatalf("%s declares %q, want one dyn. slot", c.template, dyn)
			}

			body := "clm-aaaaaa — Theorem 1\n\n> The map has a unique fixed point."
			got, err := c.ask(body)
			if err != nil {
				t.Fatalf("composing from exactly the declared slots: %v", err)
			}
			if !strings.Contains(got, body) {
				t.Error("the composed ask does not carry the text it was given")
			}
			if strings.Contains(got, "@!") {
				t.Error("the composed ask leaves a slot or a delimiter unfilled")
			}
			for _, name := range []string{ScoreSystem, scoreAnswerOccurrences, scoreAnswerClose} {
				fragment, err := load(fragmentFile(name))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(got, fragment) {
					t.Errorf("the composed ask does not splice the %s fragment", name)
				}
			}
		})
	}
}

// TestScoreAsksAreFixedAroundTheirSubject: rendering substitutes and never
// inspects the text, so one class's instruction is the same bytes whatever the
// subject's size. That is what lets a run's header digest the empty subject and
// cover every item the pass can send.
func TestScoreAsksAreFixedAroundTheirSubject(t *testing.T) {
	short, long := "clm-zzzzzz — A Subject Nothing Else Names", strings.Repeat("A long subject.\n", 500)
	for _, c := range scoreAsks() {
		t.Run(c.name, func(t *testing.T) {
			fixed := func(body string) string {
				t.Helper()
				got, err := c.ask(body)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(got, body) != 1 {
					t.Fatalf("the composed ask carries its subject %d times, want once", strings.Count(got, body))
				}
				return strings.Replace(got, body, "", 1)
			}
			if a, b := fixed(short), fixed(long); a != b {
				t.Errorf("the instruction moves with the subject's size:\n%s\n---\n%s", a, b)
			}
		})
	}
}

// TestScoreAsksRefuseAMissingSlot: a dyn. slot the caller leaves unsupplied is a
// ComposeError naming the template, never an empty splice.
func TestScoreAsksRefuseAMissingSlot(t *testing.T) {
	for _, c := range scoreAsks() {
		_, err := Render(c.template, nil, nil, nil)
		var composeErr ComposeError
		if !errors.As(err, &composeErr) || composeErr.Template != c.template {
			t.Errorf("Render(%s) from no slots = %v, want a ComposeError naming the template", c.template, err)
		}
	}
}

// TestScoreAsksNameTheirClasssIDs: each template names exactly its class's closed
// set — its own ids and the pending id — and carries no digit at all, so the
// weights, the maximum and the bands stay in `kb` and out of what a model reads.
// The set is exact, which is also what keeps one class from naming another's ids:
// a statement id on a method's ask, or a method id on a statement's, is a charge
// its own parse would refuse.
func TestScoreAsksNameTheirClasssIDs(t *testing.T) {
	tokenRE := regexp.MustCompile(`flt-[a-z0-9-]+`)
	digitRE := regexp.MustCompile(`[0-9]`)
	for _, c := range scoreAsks() {
		t.Run(c.name, func(t *testing.T) {
			text, err := c.ask("")
			if err != nil {
				t.Fatal(err)
			}
			named := tokenRE.FindAllString(text, -1)
			var set []string
			for _, id := range named {
				if !slices.Contains(set, id) {
					set = append(set, id)
				}
			}
			slices.Sort(set)
			want := slices.Sorted(slices.Values(c.table.IDs()))
			if !slices.Equal(set, want) {
				t.Errorf("%s names %v, and its class accepts %v", c.template, set, want)
			}
			if hit := digitRE.FindString(text); hit != "" {
				t.Errorf("%s carries the digit %q: a template names no number, weight, maximum or band", c.template, hit)
			}
			if !strings.Contains(text, "`"+kb.ChargeNone+"`") {
				t.Errorf("%s does not name the answer %s", c.template, kb.ChargeNone)
			}
			if !lineFormRE.MatchString(text) {
				t.Errorf("%s states no charge line of the form `flt-id: citation`", c.template)
			}
		})
	}
}

// lineFormRE is a charge line as the templates state the answer's form: an id, a
// colon and a span, in one of their fenced examples.
var lineFormRE = regexp.MustCompile(`(?m)^flt-[a-z0-9-]+: \S`)

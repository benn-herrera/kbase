package asks

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// TestScoreAsksComposeFromTheOneSlotTheyFill: each score ask declares one dyn.
// slot, fills it with the text it is given, splices the shared system fragment,
// and leaves no slot or delimiter unfilled.
func TestScoreAsksComposeFromTheOneSlotTheyFill(t *testing.T) {
	for _, c := range []struct {
		name, template string
		ask            func(string) (string, error)
	}{
		{"rigor", scoreRigorTemplate, ScoreRigor},
		{"bearing", scoreBearingTemplate, ScoreBearing},
	} {
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
			system, err := System(ScoreSystem)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, system) {
				t.Error("the composed ask does not splice the score system fragment")
			}
		})
	}
}

// TestScoreAsksRefuseAMissingSlot: a dyn. slot the caller leaves unsupplied is a
// ComposeError naming the template, never an empty splice.
func TestScoreAsksRefuseAMissingSlot(t *testing.T) {
	for _, template := range []string{scoreRigorTemplate, scoreBearingTemplate} {
		_, err := Render(template, nil, nil, nil)
		var composeErr ComposeError
		if !errors.As(err, &composeErr) || composeErr.Template != template {
			t.Errorf("Render(%s) from no slots = %v, want a ComposeError naming the template", template, err)
		}
	}
}

// TestScoreAsksNameTheirPassesIDs: each template names exactly its pass's closed
// set — the six ids and the pending id — and carries no digit at all, so the
// weights, the maximum and the bands stay in `kb` and out of what a model reads.
func TestScoreAsksNameTheirPassesIDs(t *testing.T) {
	tokenRE := regexp.MustCompile(`flt-[a-z0-9-]+`)
	digitRE := regexp.MustCompile(`[0-9]`)
	for _, c := range []struct {
		name, template string
		table          kb.FaultTable
	}{
		{"rigor", scoreRigorTemplate, kb.RigorFaults},
		{"bearing", scoreBearingTemplate, kb.BearingFaults},
	} {
		t.Run(c.name, func(t *testing.T) {
			text, err := load(c.template)
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
				t.Errorf("%s names %v, and its pass accepts %v", c.template, set, want)
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

// lineFormRE is a charge line as both templates state the answer's form: an id,
// a colon and a span, in one of their fenced examples.
var lineFormRE = regexp.MustCompile(`(?m)^flt-[a-z0-9-]+: \S`)

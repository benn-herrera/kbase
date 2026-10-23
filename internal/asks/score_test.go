package asks

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
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
// set — its own ids and the pending id — and carries no figure or word that reads
// as a judgement on the answer, so the weights, the maximum and the bands stay in
// `kb` and out of what a model reads. A quantity of the subject's own is not a
// judgement: an example's rate and threshold are what make underspecification
// concrete. The set is exact, which is also what keeps one class from naming
// another's ids: a statement id on a method's ask, or a method id on a
// statement's, is a charge its own parse would refuse.
func TestScoreAsksNameTheirClasssIDs(t *testing.T) {
	tokenRE := regexp.MustCompile(`flt-[a-z0-9-]+`)
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
			if read, hit := scoreJudgement(text); hit != "" {
				t.Errorf("%s carries %q, read as %s: a template names no score, weight, maximum or band", c.template, hit, read)
			}
			if !strings.Contains(text, "`"+kb.ChargeNone+"`") {
				t.Errorf("%s does not name the answer %s", c.template, kb.ChargeNone)
			}
		})
	}
}

// TestScoreSystemStatesTheChargeLineForm: the shared system fragment states the
// answer's form once — an id field, the colon score's parse cuts on, and a line
// number or a range of them — so a model is told the shape every ask's parse
// accepts. The id is a placeholder and never a sample: which ids a class takes
// is TestScoreAsksNameTheirClasssIDs' claim, and the number is a line of the
// text, not a charge.
func TestScoreSystemStatesTheChargeLineForm(t *testing.T) {
	text, err := load(fragmentFile(ScoreSystem))
	if err != nil {
		t.Fatal(err)
	}
	var single, span int
	matches := chargeLineRE.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		id, lines := m[1], m[2]
		if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, ">") || strings.HasPrefix(id, "flt-") {
			t.Errorf("the stated form names the id %q, want the placeholder an id stands in", id)
		}
		if strings.ContainsAny(lines, "-–—−") {
			span++
			continue
		}
		single++
	}
	if len(matches) == 0 || single == 0 {
		t.Errorf("%s states no charge line `<id>: <line>`:\n%s", ScoreSystem, text)
	}
	if span == 0 {
		t.Errorf("%s states no range `<id>: <line>-<line>`:\n%s", ScoreSystem, text)
	}
}

// chargeLineRE is a charge line as a template states one: the id field, the
// colon the parse cuts on, and one line number or a range of two — the shape
// score.linesRE accepts, with the backticks the prose wraps an example in kept
// out of the id field.
var chargeLineRE = regexp.MustCompile("([^\\s`]+): ([0-9]+(?:[-–—−][0-9]+)?)")

// scoreJudgement returns what a template's first judgement figure or word reads
// as, and the text carrying it; both empty where the template carries none. A
// decimal in [0,1] reads as a score, a weight or a threshold, and a percentage
// the same. A quantity of the subject's own reads as the claim's, not the
// answer's, however it is spelled — an example's 2.8×10⁹ scent molecules per gram
// per degree, or its 270 K.
func scoreJudgement(text string) (read, hit string) {
	for _, n := range scoreNumberRE.FindAllString(text, -1) {
		v, err := strconv.ParseFloat(n, 64)
		if err == nil && strings.Contains(n, ".") && v >= 0 && v <= 1 {
			return "a decimal in [0,1], which reads as a score, a weight or a threshold", n
		}
	}
	for _, j := range scoreJudgementREs {
		if m := j.re.FindString(text); m != "" {
			return j.read, m
		}
	}
	return "", ""
}

// scoreNumberRE is a number as a template's prose spells one: its digits and,
// where it has one, its fraction.
var scoreNumberRE = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?|\.[0-9]+`)

// scoreJudgementREs are the spellings scoreJudgement refuses besides a decimal,
// each keyed by what it reads as. A band's words are kb.BuildBandLadder's: its
// slugs spell them hyphenated and its phrases spaced, and a template states
// neither.
var scoreJudgementREs = []struct {
	read string
	re   *regexp.Regexp
}{
	{"a percentage, which reads as a score", regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?\s?%`)},
	{"a build band's word, which reads as the outcome", regexp.MustCompile(`(?i)\b(?:ok|caveats|input[ -]only|not[ -]build|unsubstantiated)\b`)},
	{"a weight or a maximum, which reads as a term of the score", regexp.MustCompile(`(?i)\b(?:weights?|weighted|maximum|max)\b`)},
}

// TestScoreJudgementTellsAJudgementFromASubjectsOwnQuantity: the scan a template
// is held to refuses a score however it is spelled and lets pass the figures an
// example of the subject carries.
func TestScoreJudgementTellsAJudgementFromASubjectsOwnQuantity(t *testing.T) {
	for _, c := range []struct {
		name, text, want string
	}{
		{"the statement example's own quantities",
			"emits 2.8×10⁹ scent molecules per gram per degree above 270 K", ""},
		{"a decimal score", "a claim that fixes every parameter lands 0.75", "0.75"},
		{"a score at the top of the range", "a bare assertion keeps 1.0", "1.0"},
		{"a fraction without a leading digit", "landing .5 on the item", ".5"},
		{"a percentage", "charges 40% of the item", "40%"},
		{"a band word", "reply `none` where the strength is ok", "ok"},
		{"a band's phrase", "ok to build on, see caveats", "ok"},
		{"a band's hyphenated slug", "the band is do-not-build", "not-build"},
		{"a weight", "one charge, weight one", "weight"},
		{"a maximum", "the maximum is reached", "maximum"},
		{"an integer quantity of the subject's", "above 270 K", ""},
		{"a word carrying a band's letters", "the source engages the book's subject", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			read, hit := scoreJudgement(c.text)
			if hit != c.want {
				t.Errorf("scoreJudgement(%q) = %q, %q; want the hit %q", c.text, read, hit, c.want)
			}
		})
	}
}

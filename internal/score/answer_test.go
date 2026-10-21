package score

import (
	"strings"
	"testing"
	"unicode"

	"kbase/internal/kb"
)

// rigorTextOf is a closed text shaped like one rigor call's: a title and a
// statement, in the two ends endPassages records them as, assembled by the
// renderer's own rule so a charge's offset is the assembled text's.
func rigorTextOf(t *testing.T, title, statement string) closedText {
	t.Helper()
	ends := nodeEnds{title: passageTitle, statement: passageStatement}
	ct, err := assemble([]passage{
		{end: ends.title, label: len("# "), text: "# " + title},
		{end: ends.statement, text: statement},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// bearingTextOf is a closed text shaped like one bearing call's: the two ends'
// titles and statements, then the edge's context, which is carried and is no
// citation's home. Each title opens with its end's label, which only bearingText
// emits.
func bearingTextOf(t *testing.T, sourceTitle, source, targetTitle, target, context string) closedText {
	t.Helper()
	ct, err := assemble([]passage{
		{end: passageSource, label: len(sourceLabel), text: sourceLabel + sourceTitle},
		{end: passageSource, text: source},
		{end: passageTarget, label: len(targetLabel), text: targetLabel + targetTitle},
		{end: passageTarget, text: target},
		{label: len(contextLabel), text: contextLabel},
		{text: context},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// assertLocated is what an offset owes: the text from it begins the citation, as
// the lookup reads both.
func assertLocated(t *testing.T, ct closedText, c Charge) {
	t.Helper()
	if c.Offset < 0 || c.Offset > len(ct.text) {
		t.Fatalf("the %s charge's offset %d is outside the closed text", c.ID, c.Offset)
	}
	have, _ := normalise(ct.text[c.Offset:])
	want, _ := normalise(c.Citation)
	if !strings.HasPrefix(have, want) {
		t.Errorf("the offset %d reads %q, which does not begin the citation %q", c.Offset, have, c.Citation)
	}
}

// TestParseAnswerReadsChargeLines: a charge line is an answer — the id in its
// own spelling, the passage its citation lies in, the span as written and the
// offset of its first match — and the table prices the ids. Duplicates collapse,
// disjoint spans of one id are two occurrences, two ids on one span are both
// charged, and `none` is the empty list that reaches the maximum.
func TestParseAnswerReadsChargeLines(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous and bounded.\nEvery chart commutes.")
	bearing := bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "Every chart commutes.", "a bridge")
	for _, c := range []struct {
		name    string
		text    closedText
		table   kb.FaultTable
		reply   string
		faults  []Charge
		moot    int
		passage []string
		value   kb.Bearing
	}{
		{
			name: "one charge, priced", text: rigor, table: kb.RigorStatement,
			reply:   "flt-overreach: continuous and bounded",
			faults:  []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "continuous and bounded", Offset: 22}},
			passage: []string{passageStatement}, value: kb.Bearing{Value: 0.65},
		},
		{
			name: "the reply's own spelling kept, its case folded for the id", text: rigor, table: kb.RigorStatement,
			reply:   "FLT-OVERREACH: Continuous and Bounded",
			faults:  []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "Continuous and Bounded"}},
			passage: []string{passageStatement}, value: kb.Bearing{Value: 0.65},
		},
		{
			name: "a re-wrapped citation", text: rigor, table: kb.RigorMethod,
			reply:   "flt-claimed-step: bounded. Every",
			faults:  []Charge{{ID: kb.FaultClaimedStep, Passage: passageStatement, Citation: "bounded. Every"}},
			passage: []string{passageStatement}, value: kb.Bearing{Value: 0.70},
		},
		{
			name: "the form's brackets stripped", text: rigor, table: kb.RigorStatement,
			reply:   "[flt-overreach]: [continuous and bounded]",
			faults:  []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "continuous and bounded"}},
			passage: []string{passageStatement}, value: kb.Bearing{Value: 0.65},
		},
		{
			name: "a title citation", text: rigor, table: kb.RigorMethod,
			reply:   "flt-unaddressed: A Claim",
			faults:  []Charge{{ID: kb.FaultUnaddressed, Passage: passageTitle, Citation: "A Claim"}},
			passage: []string{passageTitle}, value: kb.Bearing{Value: 0.00},
		},
		{
			name: "the same span under two ids, the offset shared", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: continuous and bounded\nflt-contradiction: continuous and bounded",
			faults: []Charge{
				{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "continuous and bounded"},
				{ID: kb.FaultContradiction, Passage: passageStatement, Citation: "continuous and bounded"},
			},
			passage: []string{passageStatement, passageStatement}, value: kb.Bearing{Value: 0.20},
		},
		{
			name: "one occurrence written twice collapses", text: rigor, table: kb.RigorStatement,
			reply:   "flt-overreach: continuous and bounded\nflt-overreach: continuous and bounded",
			faults:  []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Citation: "continuous and bounded"}},
			passage: []string{passageStatement}, value: kb.Bearing{Value: 0.65},
		},
		{
			name: "two disjoint spans of one id are two occurrences", text: rigor, table: kb.RigorMethod,
			reply: "flt-bare-measurement: The map is\nflt-bare-measurement: Every chart",
			faults: []Charge{
				{ID: kb.FaultBareMeasurement, Passage: passageStatement, Citation: "The map is"},
				{ID: kb.FaultBareMeasurement, Passage: passageStatement, Citation: "Every chart"},
			},
			passage: []string{passageStatement, passageStatement}, value: kb.Bearing{Value: 0.40},
		},
		{
			name: "none alone", text: rigor, table: kb.RigorStatement,
			reply: "none", faults: []Charge{}, value: kb.Bearing{Value: 1.00},
		},
		{
			name: "none among blank lines", text: rigor, table: kb.RigorStatement,
			reply: "\n  none  \n\n", faults: []Charge{}, value: kb.Bearing{Value: 1.00},
		},
		{
			name: "bearing's maximum", text: bearing, table: kb.BearingFaults,
			reply: "none", faults: []Charge{}, value: kb.Bearing{Value: 1.00},
		},
		{
			name: "a source citation", text: bearing, table: kb.BearingFaults,
			reply:   "flt-transfer: continuous",
			faults:  []Charge{{ID: kb.FaultTransfer, Passage: passageSource, Citation: "continuous"}},
			passage: []string{passageSource}, value: kb.Bearing{Value: 0.65},
		},
		{
			name: "a target citation", text: bearing, table: kb.BearingFaults,
			reply:   "flt-transfer: chart commutes",
			faults:  []Charge{{ID: kb.FaultTransfer, Passage: passageTarget, Citation: "chart commutes"}},
			passage: []string{passageTarget}, value: kb.Bearing{Value: 0.65},
		},
		{
			name: "a span both ends carry is both", table: kb.BearingFaults,
			text:    bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "The map is continuous.", "a bridge"),
			reply:   "flt-referent: The map is continuous",
			faults:  []Charge{{ID: kb.FaultReferent, Passage: passageBoth, Citation: "The map is continuous"}},
			passage: []string{passageBoth}, value: kb.Bearing{Value: 0.50},
		},
		{
			name: "a rigor span the title and the statement share is both", text: rigorTextOf(t, "A Claim", "A claim is made here."), table: kb.RigorStatement,
			reply:   "flt-overreach: a claim",
			faults:  []Charge{{ID: kb.FaultOverreach, Passage: passageBoth, Citation: "a claim"}},
			passage: []string{passageBoth}, value: kb.Bearing{Value: 0.65},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAnswer(c.reply, c.text, c.table)
			if err != nil {
				t.Fatalf("ParseAnswer(%q): %v", c.reply, err)
			}
			if len(got.Faults) != len(c.faults) {
				t.Fatalf("the reply records %v, want %v", got.Faults, c.faults)
			}
			for i, want := range c.faults {
				have := got.Faults[i]
				if have.ID != want.ID || have.Passage != want.Passage || have.Citation != want.Citation {
					t.Errorf("charge %d = %+v, want %+v", i, have, want)
				}
				if want.Passage == passageBoth {
					// The offset is the earlier match, which is the first passage
					// of the text's order that carries the span.
					assertLocated(t, c.text, have)
					continue
				}
				assertLocated(t, c.text, have)
			}
			if got.Value != c.value {
				t.Errorf("the value = %+v, want %+v", got.Value, c.value)
			}
		})
	}
}

// TestParseAnswerOffsetsAreTheFirstMatch: a span the text carries twice is
// recorded at its first match, and a span found at both ends of a bearing pair
// at the earlier end's.
func TestParseAnswerOffsetsAreTheFirstMatch(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous. The map is continuous.")
	got, err := ParseAnswer("flt-overreach: continuous", rigor, kb.RigorStatement)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.Index(rigor.text, "continuous")
	if len(got.Faults) != 1 || got.Faults[0].Offset != first {
		t.Errorf("the charge = %v, want the offset of the first match %d", got.Faults, first)
	}

	bearing := bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "The map is continuous.", "")
	got, err = ParseAnswer("flt-referent: The map is continuous", bearing, kb.BearingFaults)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Faults) != 1 {
		t.Fatalf("the charge = %v, want one", got.Faults)
	}
	// The source's statement comes before the target's in the closed text, so
	// the earlier match is the source's.
	if want := strings.Index(bearing.text, "The map is continuous."); got.Faults[0].Offset != want {
		t.Errorf("the offset = %d, want the earlier end's %d", got.Faults[0].Offset, want)
	}
	if got.Faults[0].Passage != passageBoth {
		t.Errorf("the passage = %q, want %q", got.Faults[0].Passage, passageBoth)
	}
}

// TestParseAnswerRecordsTheRigorPassage: a rigor charge records which of the
// node's two passages its citation lies in, ended as the renderer ends them — a
// span only the statement carries is statement, one only the title carries is
// title, and one both carry is both at the earlier match.
func TestParseAnswerRecordsTheRigorPassage(t *testing.T) {
	src := testKB(t)
	text, err := rigorText(src, testGraph(t, src), "clm-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, reply, passage string }{
		{"a span only the statement carries", "flt-overreach: own paragraph", passageStatement},
		{"a span only the title carries", "flt-underspecified: Claim A", passageTitle},
		{"a span both carry", "flt-contradiction: claim", passageBoth},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAnswer(c.reply, text, kb.RigorStatement)
			if err != nil {
				t.Fatalf("ParseAnswer(%q): %v", c.reply, err)
			}
			if len(got.Faults) != 1 || got.Faults[0].Passage != c.passage {
				t.Fatalf("the charges = %v, want one recorded as %s", got.Faults, c.passage)
			}
			assertLocated(t, text, got.Faults[0])
			if c.passage != passageBoth {
				return
			}
			// The offset is the earlier match, which is the title's extent.
			if title := text.passages[0]; title.end != passageTitle ||
				got.Faults[0].Offset < title.from || got.Faults[0].Offset >= title.to {
				t.Errorf("the offset %d does not lie in the title's extent %+v, which is the earlier match",
					got.Faults[0].Offset, title)
			}
		})
	}
}

// TestParseAnswerRefusesTheEndLabels: a bearing text's `## Source:` and
// `## Target:` are the frame's labels — carried ahead of each end's text and
// inside no citation's extent — so a reply citing one finds nothing there.
func TestParseAnswerRefusesTheEndLabels(t *testing.T) {
	src := testKB(t)
	e, ok := indexedEdges(t, src)[kb.RelationFounds+" axm-S1→clm-cccccc"]
	if !ok {
		t.Fatal("the index holds no founds edge from axm-S1 to clm-cccccc")
	}
	text, err := bearingText(src, testGraph(t, src), e)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"## Source:", "## Target:"} {
		t.Run(label, func(t *testing.T) {
			if !strings.Contains(text.text, label) {
				t.Fatalf("the text does not carry %q:\n%s", label, text.text)
			}
			got, err := ParseAnswer("flt-transfer: "+label, text, kb.BearingFaults)
			if err == nil {
				t.Fatalf("the charge recording %v was read, want the label refused", got.Faults)
			}
			if !strings.Contains(err.Error(), "not in the text") || !strings.Contains(err.Error(), label) {
				t.Errorf("the refusal %q does not refuse %q as not in the text", err, label)
			}
		})
	}
}

// TestParseAnswerPending: `flt-pending`, alone or in one bracket pair, is one
// record carrying the id and nothing else, and it lands the pending literal. A
// reply carrying it beside charge lines keeps those lines as moot — parsed the
// same way, priced nowhere — and records pending.
func TestParseAnswerPending(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous and bounded.")
	for _, c := range []struct {
		name     string
		reply    string
		faults   []Charge
		moot     []string
		passages []string
	}{
		{name: "alone", reply: kb.FaultPending, faults: []Charge{{ID: kb.FaultPending}}},
		{name: "in one bracket pair", reply: "[" + kb.FaultPending + "]", faults: []Charge{{ID: kb.FaultPending}}},
		{name: "case folded", reply: "FLT-PENDING", faults: []Charge{{ID: kb.FaultPending}}},
		{
			name:     "beside charges, which are moot",
			reply:    "flt-pending\nflt-overreach: continuous and bounded\nflt-underspecified: The map",
			faults:   []Charge{{ID: kb.FaultPending}},
			moot:     []string{kb.FaultOverreach, kb.FaultUnderspecified},
			passages: []string{passageStatement, passageStatement},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAnswer(c.reply, rigor, kb.RigorStatement)
			if err != nil {
				t.Fatalf("ParseAnswer(%q): %v", c.reply, err)
			}
			if len(got.Faults) != len(c.faults) {
				t.Fatalf("faults = %v, want %v", got.Faults, c.faults)
			}
			for i, want := range c.faults {
				have := got.Faults[i]
				if have.ID != want.ID || have.Passage != "" || have.Citation != "" || have.Offset != 0 {
					t.Errorf("the pending record = %+v, want the id and no citation", have)
				}
			}
			if !got.Value.Pending {
				t.Errorf("the value = %+v, want the pending literal", got.Value)
			}
			if len(got.Moot) != len(c.moot) {
				t.Fatalf("moot = %v, want %v", got.Moot, c.moot)
			}
			for i, id := range c.moot {
				if got.Moot[i].ID != id || got.Moot[i].Passage != c.passages[i] {
					t.Errorf("moot %d = %+v, want %s in %s", i, got.Moot[i], id, c.passages[i])
				}
				assertLocated(t, rigor, got.Moot[i])
			}
		})
	}
}

// TestParseAnswerRefusesTheOtherClasssID: the item's class fixes the accepted set
// before the call, so a statement id on a method's text and a method id on a
// statement's are each malformed — the refusal naming the id and the call's own
// ids as allowed — and an id from the retired single vocabulary is no longer
// allowed anywhere.
func TestParseAnswerRefusesTheOtherClasssID(t *testing.T) {
	statement := rigorTextOf(t, "A Claim", "The map is continuous and bounded.")
	method := rigorTextOf(t, "An Analysis", "The theorem follows from the two lemmas above.")
	for _, c := range []struct {
		name  string
		text  closedText
		table kb.FaultTable
		id    string
	}{
		{"a statement id on an analysis", method, kb.RigorMethod, kb.FaultUnderspecified},
		{"a statement id on an experiment's method", method, kb.RigorMethod, kb.FaultEquivocation},
		{"a method id on a claim", statement, kb.RigorStatement, kb.FaultUnaddressed},
		{"a method id on a claim, beside a statement one", statement, kb.RigorStatement, kb.FaultClaimedStep},
		{"a method id no class holds but the method's", statement, kb.RigorStatement, kb.FaultBareMeasurement},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.table.Accepts(c.id) {
				t.Fatalf("%s is in the call's own set, so the case tests nothing", c.id)
			}
			_, err := ParseAnswer(c.id+": The", c.text, c.table)
			if err == nil {
				t.Fatalf("the reply charging %s beside a %s call was read", c.id, c.table.IDs()[0])
			}
			if !strings.Contains(err.Error(), `"`+c.id+`"`) || !strings.Contains(err.Error(), "allowed:") {
				t.Errorf("the refusal %q does not name the id and the allowed set", err)
			}
			for _, id := range c.table.IDs() {
				if !strings.Contains(err.Error(), id) {
					t.Errorf("the refusal %q does not name %s among the call's ids", err, id)
				}
			}
		})
	}
}

// TestParseAnswerPricesUnderTheClasssTable: one charge list is priced by the
// table the item's class holds, so a free parameter costs the statement's step
// and a claimed step the method's, while an id both rigor vocabularies share
// costs the same under either.
func TestParseAnswerPricesUnderTheClasssTable(t *testing.T) {
	statement := rigorTextOf(t, "A Claim", "The map is continuous and bounded.")
	method := rigorTextOf(t, "An Analysis", "The map is continuous and bounded.")
	for _, c := range []struct {
		name  string
		text  closedText
		table kb.FaultTable
		reply string
		want  float64
	}{
		{"a claim's one free parameter", statement, kb.RigorStatement,
			"flt-underspecified: continuous", 0.95},
		{"a claim's four free parameters", statement, kb.RigorStatement,
			"flt-underspecified: The map\nflt-underspecified: is continuous\nflt-underspecified: and bounded\nflt-underspecified: .", 0.80},
		{"a method's unaddressed text", method, kb.RigorMethod,
			"flt-unaddressed: An Analysis", 0.00},
		{"a method's claimed step", method, kb.RigorMethod,
			"flt-claimed-step: continuous", 0.70},
		{"a shared id under the statement table", statement, kb.RigorStatement,
			"flt-overreach: continuous", 0.65},
		{"the same id under the method table", method, kb.RigorMethod,
			"flt-overreach: continuous", 0.65},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAnswer(c.reply, c.text, c.table)
			if err != nil {
				t.Fatalf("ParseAnswer(%q): %v", c.reply, err)
			}
			if want := (kb.Bearing{Value: c.want}); got.Value != want {
				t.Errorf("the value = %+v, want %+v", got.Value, want)
			}
		})
	}
}

// TestParseAnswerRefusesMalformed: every reply the form does not read is
// refused, naming the case and — where the case names them — the id and the
// span, and nothing is recorded of it.
func TestParseAnswerRefusesMalformed(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous and bounded.")
	bearing := bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "Every chart commutes.", "a bridge neither text states")
	for _, c := range []struct {
		name  string
		text  closedText
		table kb.FaultTable
		reply string
		want  []string
	}{
		{name: "a decimal", text: rigor, table: kb.RigorStatement, reply: "0.75", want: []string{"number", "0.75"}},
		{name: "the pending literal as a value", text: rigor, table: kb.RigorStatement, reply: kb.PendingLiteral, want: []string{"number"}},
		{name: "a percentage", text: rigor, table: kb.RigorStatement, reply: "75%", want: []string{"number"}},
		{name: "a whole number", text: rigor, table: kb.RigorStatement, reply: "1", want: []string{"number"}},
		{name: "a fraction", text: rigor, table: kb.RigorStatement, reply: "3/4", want: []string{"number"}},
		{name: "an empty reply", text: rigor, table: kb.RigorStatement, reply: "", want: []string{"no answer"}},
		{name: "whitespace only", text: rigor, table: kb.RigorStatement, reply: " \n\t\n ", want: []string{"no answer"}},
		{name: "an id outside the set", text: rigor, table: kb.RigorStatement, reply: "flt-nonsense: A Claim",
			want: []string{`"flt-nonsense"`, "allowed:", kb.FaultUnderspecified, kb.FaultPending}},
		{name: "the other pass's id", text: rigor, table: kb.RigorStatement, reply: "flt-no-engagement: A Claim",
			want: []string{kb.FaultNoEngagement, "allowed:"}},
		{name: "a line that is not a charge line", text: rigor, table: kb.RigorStatement, reply: "The text is fine.",
			want: []string{"not a charge line", "The text is fine."}},
		{name: "an id with no separator", text: rigor, table: kb.RigorStatement, reply: "flt-overreach",
			want: []string{kb.FaultOverreach, "carries no citation"}},
		{name: "nothing after the colon", text: rigor, table: kb.RigorStatement, reply: "flt-overreach:",
			want: []string{"carries no citation", kb.FaultOverreach}},
		{name: "pending with a colon and nothing", text: rigor, table: kb.RigorStatement, reply: kb.FaultPending + ":",
			want: []string{"carries no citation", kb.FaultPending}},
		{name: "pending with a citation", text: rigor, table: kb.RigorStatement, reply: kb.FaultPending + ": A Claim",
			want: []string{kb.FaultPending, "takes no citation", `"A Claim"`}},
		{name: "a citation not in the text", text: rigor, table: kb.RigorStatement, reply: "flt-overreach: the conclusion follows",
			want: []string{kb.FaultOverreach, `"the conclusion follows"`, "not in the text"}},
		{name: "a citation crossing a passage boundary", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: Claim The map", want: []string{"not in the text", `"Claim The map"`}},
		{name: "a citation in the edge's context", text: bearing, table: kb.BearingFaults,
			reply: "flt-transfer: a bridge neither text states", want: []string{"not in the text", "a bridge neither text states"}},
		{name: "none beside a charge", text: rigor, table: kb.RigorStatement, reply: "none\nflt-overreach: A Claim",
			want: []string{"none", "beside other lines"}},
		{name: "none beside nothing at all, twice", text: rigor, table: kb.RigorStatement, reply: "none\nnone",
			want: []string{"none", "beside other lines"}},
		{name: "one id over overlapping spans", text: rigor, table: kb.RigorStatement,
			reply: "flt-underspecified: The map is\nflt-underspecified: map is continuous",
			want:  []string{"overlap", `"The map is"`, `"map is continuous"`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAnswer(c.reply, c.text, c.table)
			if err == nil {
				t.Fatalf("ParseAnswer(%q) = %+v, want a refusal", c.reply, got)
			}
			if len(got.Faults) != 0 || len(got.Moot) != 0 {
				t.Errorf("a refused reply recorded %v / %v", got.Faults, got.Moot)
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestNormaliseFoldsAndCollapses: the lookup's reading of a text folds case and
// collapses runs of whitespace, and its offsets map every byte back to the text
// it came from.
func TestNormaliseFoldsAndCollapses(t *testing.T) {
	text := "The  MAP\n\tis"
	norm, at := normalise(text)
	if norm != "the map is" {
		t.Errorf("normalise(%q) = %q, want %q", text, norm, "the map is")
	}
	if len(at) != len(norm)+1 || at[len(norm)] != len(text) {
		t.Fatalf("the offset map = %v for a text of %d bytes", at, len(text))
	}
	for i, off := range at[:len(norm)] {
		raw := strings.ToLower(string(text[off]))
		if norm[i] == ' ' {
			// A collapsed run is represented by its first whitespace byte.
			if !unicode.IsSpace(rune(text[off])) {
				t.Errorf("normalised byte %d is a space, and maps to %q", i, text[off])
			}
			continue
		}
		if raw != string(norm[i]) {
			t.Errorf("normalised byte %d (%q) maps to %q, which is not it", i, norm[i], text[off])
		}
	}
}

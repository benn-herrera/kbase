package score

import (
	"strings"
	"testing"

	"kbase/internal/kb"
)

// rigorTextOf is a closed text shaped like one rigor call's: the heading mark on a
// line of its own, then the node's title and statement, in the passages
// endPassages records them as and by the renderer's own rule — so a charge's
// numbers are the numbers the call writes.
func rigorTextOf(t *testing.T, title, statement string) closedText {
	t.Helper()
	ends := nodeEnds{title: passageTitle, statement: passageStatement}
	ct, err := assemble([]passage{
		{label: true, text: titleLabel},
		{end: ends.title, text: title},
		{end: ends.statement, text: statement},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// bearingTextOf is a closed text shaped like one bearing call's: each end's label
// on a line of its own, its title and its statement, then the edge's context,
// which is carried and is no citation's home.
func bearingTextOf(t *testing.T, sourceTitle, source, targetTitle, target, context string) closedText {
	t.Helper()
	ct, err := assemble([]passage{
		{label: true, text: sourceLabel},
		{end: passageSource, text: sourceTitle},
		{end: passageSource, text: source},
		{label: true, text: targetLabel},
		{end: passageTarget, text: targetTitle},
		{end: passageTarget, text: target},
		{label: true, text: contextLabel},
		{text: context},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// assertCite is what a charge owes the text it cites: its quote is the text at its
// offset, and the numbers it records resolve to that quote, that offset and that
// passage.
func assertCite(t *testing.T, ct closedText, c Charge) {
	t.Helper()
	if c.Offset < 0 || c.Offset > len(ct.text) || !strings.HasPrefix(ct.text[c.Offset:], c.Citation) {
		t.Fatalf("the %s charge's offset %d does not hold its quote %q", c.ID, c.Offset, c.Citation)
	}
	site, why := ct.look(c.Lines)
	if why != "" {
		t.Fatalf("the %s charge records the lines %q: %s", c.ID, c.Lines, why)
	}
	if site.citation != c.Citation || site.offset != c.Offset || site.end != c.Passage {
		t.Errorf("the %s charge records %+v, and its lines resolve to %+v", c.ID, c, site)
	}
}

// TestParseAnswerReadsChargeLines: a charge line is an answer — the id in its own
// spelling and the numbered lines that show it — and the table prices the ids.
// Duplicates collapse, disjoint lines of one id are two occurrences, two ids over
// one line set are both charged, and `none` is the empty list that reaches the
// maximum.
func TestParseAnswerReadsChargeLines(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous and bounded.\nEvery chart commutes.")
	bearing := bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "Every chart commutes.", "a bridge")
	for _, c := range []struct {
		name   string
		text   closedText
		table  kb.FaultTable
		reply  string
		faults []Charge
		moot   int
		value  kb.Bearing
	}{
		{
			name: "one line, priced", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: 2",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2",
				Citation: "The map is continuous and bounded."}},
			value: kb.Bearing{Value: 0.65},
		},
		{
			name: "the id's case folded", text: rigor, table: kb.RigorStatement,
			reply:  "FLT-OVERREACH: 2",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"}},
			value:  kb.Bearing{Value: 0.65},
		},
		{
			name: "a range of one passage, quoted whole", text: rigor, table: kb.RigorMethod,
			reply: "flt-bare-measurement: 2-3",
			faults: []Charge{{ID: kb.FaultBareMeasurement, Passage: passageStatement, Lines: "2-3",
				Citation: "The map is continuous and bounded.\nEvery chart commutes."}},
			value: kb.Bearing{Value: 0.70},
		},
		{
			name: "the form's brackets stripped", text: rigor, table: kb.RigorStatement,
			reply:  "[flt-overreach]: [2]",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"}},
			value:  kb.Bearing{Value: 0.65},
		},
		{
			name: "the title's line", text: rigor, table: kb.RigorMethod,
			reply:  "flt-unaddressed: 1",
			faults: []Charge{{ID: kb.FaultUnaddressed, Passage: passageTitle, Lines: "1", Citation: "A Claim"}},
			value:  kb.Bearing{Value: 0.00},
		},
		{
			name: "the same line set under two ids, the quote shared", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: 2\nflt-contradiction: 2",
			faults: []Charge{
				{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"},
				{ID: kb.FaultContradiction, Passage: passageStatement, Lines: "2"},
			},
			value: kb.Bearing{Value: 0.20},
		},
		{
			name: "one line set written twice collapses", text: rigor, table: kb.RigorStatement,
			reply:  "flt-overreach: 2\nflt-overreach: 2",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"}},
			value:  kb.Bearing{Value: 0.65},
		},
		{
			name: "two lines of one paragraph under one id are two occurrences", text: rigor, table: kb.RigorMethod,
			reply: "flt-bare-measurement: 2\nflt-bare-measurement: 3",
			faults: []Charge{
				{ID: kb.FaultBareMeasurement, Passage: passageStatement, Lines: "2"},
				{ID: kb.FaultBareMeasurement, Passage: passageStatement, Lines: "3"},
			},
			value: kb.Bearing{Value: 0.40},
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
			name: "a source's line", text: bearing, table: kb.BearingFaults,
			reply:  "flt-transfer: 2",
			faults: []Charge{{ID: kb.FaultTransfer, Passage: passageSource, Lines: "2", Citation: "The map is continuous."}},
			value:  kb.Bearing{Value: 0.65},
		},
		{
			name: "a target's line", text: bearing, table: kb.BearingFaults,
			reply:  "flt-transfer: 4",
			faults: []Charge{{ID: kb.FaultTransfer, Passage: passageTarget, Lines: "4", Citation: "Every chart commutes."}},
			value:  kb.Bearing{Value: 0.65},
		},
		{
			// The two ends carry the same words, and each end's line is its own:
			// what a span the pair shared used to record as both is now the line
			// the charge names.
			name:  "the ends' like text is the line the charge names",
			text:  bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "The map is continuous.", ""),
			table: kb.BearingFaults, reply: "flt-referent: 2",
			faults: []Charge{{ID: kb.FaultReferent, Passage: passageSource, Lines: "2", Citation: "The map is continuous."}},
			value:  kb.Bearing{Value: 0.50},
		},
		{
			name:  "the same words at the target, cited there",
			text:  bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "The map is continuous.", ""),
			table: kb.BearingFaults, reply: "flt-referent: 4",
			faults: []Charge{{ID: kb.FaultReferent, Passage: passageTarget, Lines: "4", Citation: "The map is continuous."}},
			value:  kb.Bearing{Value: 0.50},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assertNumbered(t, c.text)
			got, err := ParseAnswer(c.reply, c.text, c.table)
			if err != nil {
				t.Fatalf("ParseAnswer(%q): %v", c.reply, err)
			}
			if len(got.Faults) != len(c.faults) {
				t.Fatalf("the reply records %v, want %v", got.Faults, c.faults)
			}
			for i, want := range c.faults {
				have := got.Faults[i]
				if have.ID != want.ID || have.Passage != want.Passage || have.Lines != want.Lines {
					t.Errorf("charge %d = %+v, want %+v", i, have, want)
				}
				if want.Citation != "" && have.Citation != want.Citation {
					t.Errorf("charge %d quotes %q, want %q", i, have.Citation, want.Citation)
				}
				assertCite(t, c.text, have)
			}
			if got.Value != c.value {
				t.Errorf("the value = %+v, want %+v", got.Value, c.value)
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
			reply:    "flt-pending\nflt-overreach: 2\nflt-underspecified: 1",
			faults:   []Charge{{ID: kb.FaultPending}},
			moot:     []string{kb.FaultOverreach, kb.FaultUnderspecified},
			passages: []string{passageStatement, passageTitle},
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
				if have.ID != want.ID || have.Passage != "" || have.Lines != "" || have.Citation != "" || have.Offset != 0 {
					t.Errorf("the pending record = %+v, want the id and no lines", have)
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
				assertCite(t, rigor, got.Moot[i])
			}
		})
	}
}

// TestParseAnswerRefusesTheLabels: a bearing text's `## Source:` and `## Target:`
// are the frame's own lines — carried ahead of each end's text and holding no
// number — so a charge naming one names no line of the text.
func TestParseAnswerRefusesTheLabels(t *testing.T) {
	text := bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "Every chart commutes.", "")
	for _, label := range []string{sourceLabel, targetLabel, contextLabel} {
		t.Run(label, func(t *testing.T) {
			got, err := ParseAnswer("flt-transfer: "+label, text, kb.BearingFaults)
			if err == nil {
				t.Fatalf("the charge recording %v was read, want the label refused", got.Faults)
			}
			if !strings.Contains(err.Error(), "names no line") || !strings.Contains(err.Error(), label) {
				t.Errorf("the refusal %q does not refuse %q as naming no line", err, label)
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
			_, err := ParseAnswer(c.id+": 1", c.text, c.table)
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
	four := rigorTextOf(t, "A Claim", "one\ntwo\nthree")
	for _, c := range []struct {
		name  string
		text  closedText
		table kb.FaultTable
		reply string
		want  float64
	}{
		{"a claim's one free parameter", statement, kb.RigorStatement,
			"flt-underspecified: 2", 0.95},
		{"a claim's four free parameters", four, kb.RigorStatement,
			"flt-underspecified: 1\nflt-underspecified: 2\nflt-underspecified: 3\nflt-underspecified: 4", 0.80},
		{"a method's unaddressed text", method, kb.RigorMethod,
			"flt-unaddressed: 1", 0.00},
		{"a method's claimed step", method, kb.RigorMethod,
			"flt-claimed-step: 2", 0.70},
		{"a shared id under the statement table", statement, kb.RigorStatement,
			"flt-overreach: 2", 0.65},
		{"the same id under the method table", method, kb.RigorMethod,
			"flt-overreach: 2", 0.65},
	} {
		t.Run(c.name, func(t *testing.T) {
			assertNumbered(t, c.text)
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
// refused, naming the case and — where the case names them — the id and the lines,
// and nothing is recorded of it.
func TestParseAnswerRefusesMalformed(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous and bounded.\nEvery chart commutes.")
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
		{name: "a whole number on its own", text: rigor, table: kb.RigorStatement, reply: "1",
			want: []string{"number", "names no fault id"}},
		{name: "a fraction", text: rigor, table: kb.RigorStatement, reply: "3/4", want: []string{"number"}},
		{name: "an empty reply", text: rigor, table: kb.RigorStatement, reply: "", want: []string{"no answer"}},
		{name: "whitespace only", text: rigor, table: kb.RigorStatement, reply: " \n\t\n ", want: []string{"no answer"}},
		{name: "an id outside the set", text: rigor, table: kb.RigorStatement, reply: "flt-nonsense: 1",
			want: []string{`"flt-nonsense"`, "allowed:", kb.FaultUnderspecified, kb.FaultPending}},
		{name: "the other pass's id", text: rigor, table: kb.RigorStatement, reply: "flt-no-engagement: 1",
			want: []string{kb.FaultNoEngagement, "allowed:"}},
		{name: "a line that is not a charge line", text: rigor, table: kb.RigorStatement, reply: "The text is fine.",
			want: []string{"not a charge line", "The text is fine."}},
		{name: "an id with no field", text: rigor, table: kb.RigorStatement, reply: "flt-overreach",
			want: []string{kb.FaultOverreach, "carries no line"}},
		{name: "nothing after the colon", text: rigor, table: kb.RigorStatement, reply: "flt-overreach:",
			want: []string{"carries no line", kb.FaultOverreach}},
		{name: "pending with a colon and nothing", text: rigor, table: kb.RigorStatement, reply: kb.FaultPending + ":",
			want: []string{"carries no line", kb.FaultPending}},
		{name: "pending with a line", text: rigor, table: kb.RigorStatement, reply: kb.FaultPending + ": 2",
			want: []string{kb.FaultPending, "takes no line", `"2"`}},
		{name: "words in the field", text: rigor, table: kb.RigorStatement, reply: "flt-overreach: the conclusion follows",
			want: []string{kb.FaultOverreach, `"the conclusion follows"`, "names no line"}},
		{name: "a line the call does not show", text: rigor, table: kb.RigorStatement, reply: "flt-overreach: 9",
			want: []string{kb.FaultOverreach, "line 9 is not one the call shows"}},
		{name: "a range the call does not show", text: rigor, table: kb.RigorStatement, reply: "flt-overreach: 2-9",
			want: []string{kb.FaultOverreach, "line 9 is not one the call shows"}},
		{name: "a line zero", text: rigor, table: kb.RigorStatement, reply: "flt-overreach: 0",
			want: []string{kb.FaultOverreach, "line 0 is not one the call shows"}},
		{name: "a range across the title and the statement", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: 1-2", want: []string{"crosses from one passage to another", "1-2"}},
		{name: "a range backwards", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: 3-2", want: []string{"runs backwards", "3-2"}},
		{name: "the edge's note, which holds no number", text: bearing, table: kb.BearingFaults,
			reply: "flt-transfer: a bridge neither text states",
			want:  []string{"names no line", "a bridge neither text states"}},
		{name: "a number the note's lines do not carry", text: bearing, table: kb.BearingFaults,
			reply: "flt-transfer: 5", want: []string{"line 5 is not one the call shows"}},
		{name: "none beside a charge", text: rigor, table: kb.RigorStatement, reply: "none\nflt-overreach: 1",
			want: []string{"none", "beside other lines"}},
		{name: "none beside nothing at all, twice", text: rigor, table: kb.RigorStatement, reply: "none\nnone",
			want: []string{"none", "beside other lines"}},
		{name: "one id over overlapping line sets", text: rigor, table: kb.RigorStatement,
			reply: "flt-underspecified: 2-3\nflt-underspecified: 2",
			want:  []string{"overlap without coinciding", "2-3 and 2"}},
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

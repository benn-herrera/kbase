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
	}{
		{
			name: "one line, priced", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: 2",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2",
				Citation: "The map is continuous and bounded."}},
		},
		{
			name: "the id's case folded", text: rigor, table: kb.RigorStatement,
			reply:  "FLT-OVERREACH: 2",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"}},
		},
		{
			name: "a range of one passage, quoted whole", text: rigor, table: kb.RigorMethod,
			reply: "flt-bare-measurement: 2-3",
			faults: []Charge{{ID: kb.FaultBareMeasurement, Passage: passageStatement, Lines: "2-3",
				Citation: "The map is continuous and bounded.\nEvery chart commutes."}},
		},
		{
			name: "the form's brackets stripped", text: rigor, table: kb.RigorStatement,
			reply:  "[flt-overreach]: [2]",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"}},
		},
		{
			name: "the title's line", text: rigor, table: kb.RigorMethod,
			reply:  "flt-unaddressed: 1",
			faults: []Charge{{ID: kb.FaultUnaddressed, Passage: passageTitle, Lines: "1", Citation: "A Claim"}},
		},
		{
			name: "the same line set under two ids, the quote shared", text: rigor, table: kb.RigorStatement,
			reply: "flt-overreach: 2\nflt-contradiction: 2",
			faults: []Charge{
				{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"},
				{ID: kb.FaultContradiction, Passage: passageStatement, Lines: "2"},
			},
		},
		{
			name: "one line set written twice collapses", text: rigor, table: kb.RigorStatement,
			reply:  "flt-overreach: 2\nflt-overreach: 2",
			faults: []Charge{{ID: kb.FaultOverreach, Passage: passageStatement, Lines: "2"}},
		},
		{
			name: "two lines of one paragraph under one id are two occurrences", text: rigor, table: kb.RigorMethod,
			reply: "flt-bare-measurement: 2\nflt-bare-measurement: 3",
			faults: []Charge{
				{ID: kb.FaultBareMeasurement, Passage: passageStatement, Lines: "2"},
				{ID: kb.FaultBareMeasurement, Passage: passageStatement, Lines: "3"},
			},
		},
		{
			name: "none alone", text: rigor, table: kb.RigorStatement,
			reply: "none", faults: []Charge{},
		},
		{
			name: "none among blank lines", text: rigor, table: kb.RigorStatement,
			reply: "\n  none  \n\n", faults: []Charge{},
		},
		{
			name: "none on a link", text: bearing, table: kb.BearingFaults,
			reply: "none", faults: []Charge{},
		},
		{
			name: "a source's line", text: bearing, table: kb.BearingFaults,
			reply:  "flt-transfer: 2",
			faults: []Charge{{ID: kb.FaultTransfer, Passage: passageSource, Lines: "2", Citation: "The map is continuous."}},
		},
		{
			name: "a target's line", text: bearing, table: kb.BearingFaults,
			reply:  "flt-transfer: 4",
			faults: []Charge{{ID: kb.FaultTransfer, Passage: passageTarget, Lines: "4", Citation: "Every chart commutes."}},
		},
		{
			// The two ends carry the same words, and each end's line is its own:
			// what a span the pair shared used to record as both is now the line
			// the charge names.
			name:  "the ends' like text is the line the charge names",
			text:  bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "The map is continuous.", ""),
			table: kb.BearingFaults, reply: "flt-referent: 2",
			faults: []Charge{{ID: kb.FaultReferent, Passage: passageSource, Lines: "2", Citation: "The map is continuous."}},
		},
		{
			name:  "the same words at the target, cited there",
			text:  bearingTextOf(t, "A Lemma", "The map is continuous.", "A Claim", "The map is continuous.", ""),
			table: kb.BearingFaults, reply: "flt-referent: 4",
			faults: []Charge{{ID: kb.FaultReferent, Passage: passageTarget, Lines: "4", Citation: "The map is continuous."}},
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
			// The price is the class's table's over the ids the reply recorded,
			// so a charge's own weight is the scheme's to state, not this test's.
			ids := make([]string, 0, len(c.faults))
			for _, f := range c.faults {
				ids = append(ids, f.ID)
			}
			if want := c.table.Price(ids); got.Value != want {
				t.Errorf("the value = %+v, want %+v", got.Value, want)
			}
		})
	}
}

// TestParseAnswerRefusesPending: `flt-pending` is no table's id any more — a
// text that asserts nothing is a charge against it — so a reply naming it is an
// id outside the set, alone, in one bracket pair, case-folded, or beside charge
// lines that would otherwise land.
func TestParseAnswerRefusesPending(t *testing.T) {
	rigor := rigorTextOf(t, "A Claim", "The map is continuous and bounded.")
	for _, reply := range []string{
		kb.FaultPending,
		"[" + kb.FaultPending + "]",
		"FLT-PENDING",
		kb.FaultPending + "\nflt-overreach: 2",
		kb.FaultPending + ": 2",
	} {
		t.Run(reply, func(t *testing.T) {
			got, err := ParseAnswer(reply, rigor, kb.RigorStatement)
			if err == nil {
				t.Fatalf("ParseAnswer(%q) = %+v, want an id outside the set", reply, got)
			}
			if !strings.Contains(err.Error(), `"`+kb.FaultPending+`"`) || !strings.Contains(err.Error(), "allowed:") {
				t.Errorf("the refusal %q does not name %s as outside the set", err, kb.FaultPending)
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
			want: []string{`"flt-nonsense"`, "allowed:", kb.FaultUnderspecified, kb.FaultContradiction}},
		{name: "the other pass's id", text: rigor, table: kb.RigorStatement, reply: "flt-inapplicable: 1",
			want: []string{kb.FaultInapplicable, "allowed:"}},
		{name: "a line that is not a charge line", text: rigor, table: kb.RigorStatement, reply: "The text is fine.",
			want: []string{"not a charge line", "The text is fine."}},
		{name: "an id with no field", text: rigor, table: kb.RigorStatement, reply: "flt-overreach",
			want: []string{kb.FaultOverreach, "carries no line"}},
		{name: "nothing after the colon", text: rigor, table: kb.RigorStatement, reply: "flt-overreach:",
			want: []string{"carries no line", kb.FaultOverreach}},
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

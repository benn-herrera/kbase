package score

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"kbase/internal/kb"
)

// The passages a charge's citation can lie in, as the ledger's `passage` records
// them. A rigor call's pair is the node's title and its statement, and a bearing
// call's the source's text and the target's. passageBoth records what the lookup
// found where the span lies in passages of both ends — the two texts carry it
// alike, and whether a longer span would have separated them is the ask's for
// the model to weigh, never the parse's to ask.
const (
	passageTitle     = "title"
	passageStatement = "statement"
	passageSource    = "source"
	passageTarget    = "target"
	passageBoth      = "both"
)

// Charge is one charge line as parsed: the fault id, the passage its citation
// was found in, the span as the model wrote it, and the span's offset in the
// closed text at its first match. The fields are the ledger's, in its order.
type Charge struct {
	ID       string `json:"id"`
	Passage  string `json:"passage"`
	Citation string `json:"citation"`
	Offset   int    `json:"offset"`
}

// Answer is a reply read as the charges it records: the charges themselves, the
// charge lines a pending answer makes moot, and the value the pass's table gives
// the answer's ids. The charges are what the ledger keeps beside the reply; the
// value is what lands.
type Answer struct {
	Faults []Charge
	Moot   []Charge
	Value  kb.Bearing
}

// faultIDRE is what a charge line's first field spells to be a fault id at all.
// It separates an id outside the call's set — named as such — from a line that is
// not a charge line.
var faultIDRE = regexp.MustCompile(`^flt-[a-z0-9-]+$`)

// numberRE is the shape of a reply that answers the decimal question the asks no
// longer pose: a decimal, a whole number, a percentage or a fraction. A bare one
// names no id and no citation, and the attempt records that it did.
var numberRE = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?%?$|^[+-]?[0-9]+/[0-9]+$`)

// charged is one charge line as the parse holds it before the duplicate and
// overlap rules: the record, the line it was written on, and where the span ends
// in the closed text — which is what "the same span twice" and "two spans that
// overlap" are read from.
type charged struct {
	charge Charge
	line   int
	to     int
}

// ParseAnswer reads reply as the answer it records against the closed text the
// call carried and the pass's table: one charge line per occurrence, `none`
// alone, or `flt-pending` alone. Blank lines are ignored and any other line that
// is not a charge line is refused, so nothing is read out of prose. A refused
// reply is malformed for this attempt — no charges and no judgement, the reason
// naming the case and, where it names one, the id and span — and the item is
// asked again on a resume.
func ParseAnswer(reply string, closed closedText, table kb.FaultTable) (Answer, error) {
	var (
		charges []charged
		pending bool
		nones   int
	)
	for i, raw := range strings.Split(reply, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		no := i + 1
		head, field, hasField := strings.Cut(line, ":")
		id := strings.ToLower(fieldText(head))
		if !hasField {
			switch {
			case id == kb.ChargeNone:
				nones++
			case id == kb.FaultPending:
				pending = true
			case table.Accepts(id):
				// The id and no separator: only the pending id takes a line
				// without a citation, and it is matched above this case.
				return Answer{}, malformedf("line %d: %s carries no citation, and a charge is the id and the span that shows it", no, id)
			case id == kb.PendingLiteral || numberRE.MatchString(line):
				return Answer{}, malformedf("line %d answers with the number %q: a number names no fault and no citation, and a reply offering one answers a question no longer asked", no, line)
			case faultIDRE.MatchString(id):
				return Answer{}, notInSet(no, id, table)
			default:
				return Answer{}, malformedf("line %d is not a charge line: %q", no, line)
			}
			continue
		}
		if !table.Accepts(id) {
			if faultIDRE.MatchString(id) {
				return Answer{}, notInSet(no, id, table)
			}
			return Answer{}, malformedf("line %d is not a charge line: %q", no, line)
		}
		citation := fieldText(field)
		if citation == "" {
			return Answer{}, malformedf("line %d: %s carries no citation, and a charge is the id and the span that shows it", no, id)
		}
		if id == kb.FaultPending {
			return Answer{}, malformedf("line %d: %s takes no citation, and this line names the span %q", no, id, citation)
		}
		found, from, to, ok := closed.locate(citation)
		if !ok {
			return Answer{}, malformedf("line %d: the %s charge's citation %q is not in the text the call carried, whole within one passage", no, id, citation)
		}
		charges = append(charges, charged{charge: Charge{ID: id, Passage: found, Citation: citation, Offset: from}, line: no, to: to})
	}
	switch {
	case nones > 0 && (nones > 1 || pending || len(charges) > 0):
		return Answer{}, malformedf("%s stands beside other lines: %s is the whole of an answer, and the parse has no ground to prefer either half", kb.ChargeNone, kb.ChargeNone)
	case nones == 1:
		return Answer{Faults: []Charge{}, Value: table.Price(nil)}, nil
	case !pending && len(charges) == 0:
		// Nothing at all: an empty reply, or one of blank lines only. It states
		// no charge and neither answer's word, so it is no answer — silence is
		// never read as a clean pass.
		return Answer{}, malformedf("the reply holds no line: it names no charge, no %s and no %s", kb.ChargeNone, kb.FaultPending)
	}
	faults, err := settled(charges)
	if err != nil {
		return Answer{}, err
	}
	ids := make([]string, 0, len(faults))
	for _, c := range faults {
		ids = append(ids, c.ID)
	}
	if !pending {
		return Answer{Faults: faults, Value: table.Price(ids)}, nil
	}
	// The pending test comes before any criterion: where the pair holds no
	// proposition, nothing else can become the judgement. The charge lines are
	// kept for what they say about the reply, and priced nowhere.
	pendingRecord := []Charge{{ID: kb.FaultPending}}
	return Answer{Faults: pendingRecord, Moot: faults, Value: table.Price([]string{kb.FaultPending})}, nil
}

// settled reads the charge lines as the charges they record: an identical pair
// collapses to one — the answer's own evidence cannot tell the two lines apart —
// two spans of one id that overlap without coinciding is the reply the count
// cannot be read from, and two ids on one span are both charged, the offset
// shared.
func settled(charges []charged) ([]Charge, error) {
	var out []charged
	for _, c := range charges {
		collapses := false
		for _, kept := range out {
			if kept.charge.ID != c.charge.ID {
				continue
			}
			if kept.charge.Offset == c.charge.Offset && kept.to == c.to {
				collapses = true
				break
			}
			if kept.charge.Offset < c.to && c.charge.Offset < kept.to {
				return nil, malformedf("lines %d and %d charge %s over spans that overlap without coinciding: %q and %q, so the count cannot be read from the reply",
					kept.line, c.line, c.charge.ID, kept.charge.Citation, c.charge.Citation)
			}
		}
		if !collapses {
			out = append(out, c)
		}
	}
	faults := make([]Charge, len(out))
	for i, c := range out {
		faults[i] = c.charge
	}
	return faults, nil
}

// notInSet is a line naming an id the pass does not accept, its refusal stating
// the id and the pass's ids as the allowed set.
func notInSet(line int, id string, table kb.FaultTable) error {
	return malformedf("line %d names the id %q, which this pass does not accept; allowed: %s",
		line, id, strings.Join(table.IDs(), ", "))
}

// malformedf is a reply the parse refuses: the case, and where the case names
// one the id and span the line carried. A caller records the attempt failed with
// this reason and keeps no text of it.
func malformedf(format string, args ...any) error {
	return fmt.Errorf("the reply is no answer: "+format, args...)
}

// fieldText is one field of a charge line as the form's bracket rule leaves it:
// a bracket pair around the whole field is a delimiter and is stripped, and the
// field is trimmed. A span that itself begins with [ or ends with ] is written
// unbracketed, so one pair is all the rule ever strips.
func fieldText(field string) string {
	field = strings.TrimSpace(field)
	if len(field) >= 2 && strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]") {
		field = strings.TrimSpace(field[1 : len(field)-1])
	}
	return field
}

// locate finds a citation in the closed text: the passage it lies whole within
// and the extent of its first match, ok false where it lies in none. Searching
// one passage at a time is what keeps a span that crosses from one to another
// unfound. The lookup collapses runs of whitespace to one space and folds case,
// so a re-wrapped or re-capitalised quotation is found, and a span the texts of
// both ends carry is recorded as both, at the earlier match.
func (ct closedText) locate(citation string) (end string, from, to int, ok bool) {
	want, _ := normalise(citation)
	if want == "" {
		return "", 0, 0, false
	}
	var ends []string
	for _, e := range ct.passages {
		norm, at := normalise(ct.text[e.from:e.to])
		i := strings.Index(norm, want)
		if i < 0 {
			continue
		}
		if !slices.Contains(ends, e.end) {
			ends = append(ends, e.end)
		}
		if ok {
			continue
		}
		from, to, ok = e.from+at[i], e.from+at[i+len(want)], true
	}
	if !ok {
		return "", 0, 0, false
	}
	if len(ends) > 1 {
		return passageBoth, from, to, true
	}
	return ends[0], from, to, true
}

// normalise is text as the citation lookup reads it — runs of whitespace
// collapsed to one space and case folded — with the raw offset of each of its
// bytes and, as a last entry, text's length, so a match's end maps back to the
// closed text as its start does.
func normalise(text string) (norm string, at []int) {
	var b strings.Builder
	space := false
	for i, r := range text {
		if unicode.IsSpace(r) {
			if space {
				continue
			}
			space = true
			b.WriteByte(' ')
			at = append(at, i)
			continue
		}
		space = false
		lower := strings.ToLower(string(r))
		for range len(lower) {
			at = append(at, i)
		}
		b.WriteString(lower)
	}
	return b.String(), append(at, len(text))
}

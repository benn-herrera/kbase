package score

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kbase/internal/asks"
	"kbase/internal/kb"
)

// The passages a charge's lines can lie in, as the ledger's `passage` records
// them. A rigor call's pair is the node's title and its statement, and a bearing
// call's the source's text and the target's. A numbered line lies in one passage,
// so a charge records the one its lines lie in: `both` — the value a span the two
// ends carried alike used to be recorded as — is retired and no longer produced.
// A line written before the change may record it, and still means what it said.
const (
	passageTitle     = "title"
	passageStatement = "statement"
	passageSource    = "source"
	passageTarget    = "target"
)

// Charge is one charge line as parsed: the fault id, the passage its lines lie
// in, the numbers as the ledger records them, the span of the carried text they
// quote and that span's offset in the text. The fields are the ledger's, in its
// order.
type Charge struct {
	ID       string `json:"id"`
	Passage  string `json:"passage"`
	Lines    string `json:"lines"`
	Citation string `json:"citation"`
	Offset   int    `json:"offset"`
}

// Answer is a reply read as the charges it records: the charges themselves, the
// charge lines a retired pending answer once made moot (no reply produces them
// now, and an old ledger line keeps them), and the value the item's class's
// table gives the answer's ids. The charges are what the ledger keeps beside the
// reply; the value is what lands.
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
// longer pose: a decimal, a whole number, a percentage or a fraction. A line that is
// only one of these names no fault id, which is a different fault from a number in
// a charge's field: a field's number is a line of the text, and a line answering
// with a number and no id answers no question the call asks.
var numberRE = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?%?$|^[+-]?[0-9]+/[0-9]+$`)

// charged is one charge line as the parse holds it before the duplicate and
// overlap rules: the record, the line it was written on, and the numbers it
// cites, which is what "the same lines twice" and "two line sets that overlap"
// are read from.
type charged struct {
	charge Charge
	line   int
	lo, hi int
}

// ParseAnswer reads reply as the answer it records against the text the call
// carried and the item's class's table: one charge line per occurrence, or
// `none` alone. A charge line names numbered lines of that text, never words of
// it: the tool quotes what the numbers name. Blank lines are ignored and any
// other line that is not a charge line is refused, so nothing is read out of
// prose. A refused reply is malformed for this attempt — no charges and no
// judgement, the reason naming the case and, where it names one, the id and the
// lines — and is a malformedError naming the reply lines its case names.
func ParseAnswer(reply string, closed closedText, table kb.FaultTable) (Answer, error) {
	var (
		charges []charged
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
			case table.Accepts(id):
				// An id and no separator: every charge names the lines where
				// it shows, so this one carries none.
				return Answer{}, fieldless(no, id)
			case id == kb.PendingLiteral || numberRE.MatchString(line):
				return Answer{}, malformedAt(asks.RefusalBareNumber, atReplyLine(no), []int{no}, "line %d answers with a number: a number names no fault id, and a charge is the id and the line where it shows", no)
			case faultIDRE.MatchString(id):
				return Answer{}, notInSet(no, id, table)
			default:
				return Answer{}, notAChargeLine(no)
			}
			continue
		}
		if !table.Accepts(id) {
			if faultIDRE.MatchString(id) {
				return Answer{}, notInSet(no, id, table)
			}
			return Answer{}, notAChargeLine(no)
		}
		cited := fieldText(field)
		if cited == "" {
			return Answer{}, fieldless(no, id)
		}
		site, fault := closed.look(cited)
		if fault != nil {
			values := chargingAt(no, id)
			if fault.from != "" {
				values[asks.SlotCitedFrom] = fault.from
			}
			if fault.to != "" {
				values[asks.SlotCitedTo] = fault.to
			}
			return Answer{}, malformedAt(fault.refusal, values, []int{no}, "line %d: the %s charge's %s", no, id, fault.why)
		}
		charges = append(charges, charged{
			charge: Charge{ID: id, Passage: site.end, Lines: site.lines, Citation: site.citation, Offset: site.offset},
			line:   no, lo: site.lo, hi: site.hi,
		})
	}
	switch {
	case nones > 0 && (nones > 1 || len(charges) > 0):
		return Answer{}, malformedAt(asks.RefusalNoneBesideCharges, nil, nil, "`%s` stands beside other lines: `%s` is the whole of an answer, and the parse has no ground to prefer either half", kb.ChargeNone, kb.ChargeNone)
	case nones == 1:
		return Answer{Faults: []Charge{}, Value: table.Price(nil)}, nil
	case len(charges) == 0:
		// Nothing at all: an empty reply, or one of blank lines only. It states
		// no charge and not the answer's word, so it is no answer — silence is
		// never read as a clean pass.
		return Answer{}, malformedAt(asks.RefusalEmptyReply, nil, nil, "the reply holds no line: it names no charge and no `%s`", kb.ChargeNone)
	}
	faults, err := settled(charges)
	if err != nil {
		return Answer{}, err
	}
	ids := make([]string, 0, len(faults))
	for _, c := range faults {
		ids = append(ids, c.ID)
	}
	return Answer{Faults: faults, Value: table.Price(ids)}, nil
}

// settled reads the charge lines as the charges they record: an identical line
// set collapses to one — the answer's own evidence cannot tell the two lines
// apart — two line sets of one id that overlap without coinciding is the reply
// the count cannot be read from, and two ids over one line set are both charged,
// the quote and its offset shared.
func settled(charges []charged) ([]Charge, error) {
	var out []charged
	for _, c := range charges {
		collapses := false
		for _, kept := range out {
			if kept.charge.ID != c.charge.ID {
				continue
			}
			if kept.lo == c.lo && kept.hi == c.hi {
				collapses = true
				break
			}
			if kept.lo <= c.hi && c.lo <= kept.hi {
				values := chargingAt(kept.line, c.charge.ID)
				values[asks.SlotOtherReplyLine] = strconv.Itoa(c.line)
				return nil, malformedAt(asks.RefusalOverlappingRanges, values, []int{kept.line, c.line}, "lines %d and %d charge %s over line sets that overlap without coinciding: %s and %s, so the count cannot be read from the reply",
					kept.line, c.line, c.charge.ID, kept.charge.Lines, c.charge.Lines)
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

// notInSet is a line naming an id the call does not accept, its refusal stating
// the id and the call's own ids as the allowed set.
func notInSet(line int, id string, table kb.FaultTable) error {
	return malformedAt(asks.RefusalIDNotOffered, chargingAt(line, id), []int{line}, "line %d names the id %q, which this call does not accept; allowed: %s",
		line, id, strings.Join(table.IDs(), ", "))
}

// fieldless is the refusal of a charge naming no line at all: an id with no field,
// or a field empty or blank.
func fieldless(line int, id string) error {
	return malformedAt(asks.RefusalIDWithoutLine, chargingAt(line, id), []int{line}, "line %d: %s carries no line, and a charge is the fault id and the line number where it shows", line, id)
}

// notAChargeLine is the refusal of a line that is no charge line at all.
func notAChargeLine(line int) error {
	return malformedAt(asks.RefusalNotAChargeLine, atReplyLine(line), []int{line}, "line %d is not a charge line", line)
}

// atReplyLine is the slot values of a case naming the reply line line alone.
func atReplyLine(line int) map[string]string {
	return map[string]string{asks.SlotReplyLine: strconv.Itoa(line)}
}

// chargingAt is the slot values of a case naming the reply line line and the
// id it charges.
func chargingAt(line int, id string) map[string]string {
	values := atReplyLine(line)
	values[asks.SlotID] = id
	return values
}

// malformedError is a reply the parse refuses. Its text is the reason: the
// case, and where the case names one the id and the lines, and no other text of
// the reply. A caller records the attempt failed with this reason and keeps no
// text of the reply. refusal is the case as an asks.Refusal… constant, values
// the slot values its fragment states it with, by bare slot name, and lines the
// 1-based numbers of the reply lines the case names, none where it names none.
type malformedError struct {
	reason  string
	refusal string
	values  map[string]string
	lines   []int
}

func (e malformedError) Error() string { return "the reply is no answer: " + e.reason }

// malformedAt is the refusal of a reply under the case refusal, stated with
// values, naming the reply lines lines, its reason format filled with args.
func malformedAt(refusal string, values map[string]string, lines []int, format string, args ...any) error {
	return malformedError{reason: fmt.Sprintf(format, args...), refusal: refusal, values: values, lines: lines}
}

// fieldText is one field of a charge line as the form's bracket rule leaves it:
// a bracket pair around the whole field is a delimiter and is stripped, and the
// field is trimmed.
func fieldText(field string) string {
	field = strings.TrimSpace(field)
	if len(field) >= 2 && strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]") {
		field = strings.TrimSpace(field[1 : len(field)-1])
	}
	return field
}

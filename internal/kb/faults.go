package kb

// The fault ids the three vocabularies charge with, in one namespace. An id
// spells the criterion it names and carries no weight of its own — the item's
// table does, per occurrence. The statement and
// the method vocabularies share flt-contradiction and flt-overreach, which name
// the same fault at different subjects and carry the same weight in both tables,
// so a ledger line's price never depends on which rigor vocabulary read it; every
// other id belongs to one vocabulary, and the bearing set is disjoint from both.
const (
	// The statement vocabulary: a claim's title read with the statement under
	// it.
	FaultContradiction  = "flt-contradiction"
	FaultOverreach      = "flt-overreach"
	FaultEquivocation   = "flt-equivocation"
	FaultUnderspecified = "flt-underspecified"

	// The method vocabulary: an analysis's or an experiment's derivation or
	// procedure.
	FaultUnaddressed     = "flt-unaddressed"
	FaultClaimedStep     = "flt-claimed-step"
	FaultUnstatedPremise = "flt-unstated-premise"
	FaultBareMeasurement = "flt-bare-measurement"

	// The bearing vocabulary: one scoring edge's link.
	FaultReferent     = "flt-referent"
	FaultInapplicable = "flt-inapplicable"
	FaultNarrower     = "flt-narrower"
	FaultTransfer     = "flt-transfer"
	FaultConditional  = "flt-conditional"

	// FaultPending is the id a ledger line written before the scoring revision
	// carries for a text that held no proposition to judge. No table
	// accepts it any more: a text that asserts nothing is a charge against it
	// (an empty claim is flt-underspecified, a method that displays nothing
	// flt-unaddressed), and an answer that would have been pending is refused as
	// an id outside the set. Price still reads it, so a recorded line keeps the
	// value it was written with.
	FaultPending = "flt-pending"
)

// ChargeNone is the answer that charges nothing: a word, not an id, and the one
// answer that reaches a table's maximum. It is the whole of an answer — beside a
// charge line the two halves contradict each other — and its record's charge
// list is empty.
const ChargeNone = "none"

// RigorMaximum is the most rigor a node carries, and what a rigor charge list
// with no charge prices to: the rigor tables' maximum, the value a text charged
// with nothing takes. It is the ceiling of every write that takes a node's own
// rigor, and it is the whole of the scale — an axiom's rule value, a pass's
// largest price and an authored maximum are one number, and nothing is held
// back from it. Bearing has no reserve either: BearingMaximum is its table's.
const RigorMaximum = 1.0

// rigorMaximumHundredths is rigorMaximum in the tables' integer hundredths:
// the price of an answer that names no fault. No charge in either rigor table
// costs it now — every one is fifteen hundredths — so a text reaches zero by
// accumulating findings, not by one verdict.
const rigorMaximumHundredths = int(RigorMaximum * 100)

// BearingMaximum is the most bearing a link carries: the bearing table's whole
// maximum, which reserves nothing the way the rigor one does.
const BearingMaximum = 1.0

// bearingMaximumHundredths is BearingMaximum in the tables' integer hundredths:
// the price of a link charged with nothing. No charge in the bearing table costs
// it: every one is a tenth of it, so a link reaches zero by accumulating parts
// that do no work.
const bearingMaximumHundredths = int(BearingMaximum * 100)

// AuthoredRigor is an authored value as the system reads it: above
// RigorMaximum it reads as RigorMaximum — a hand-edited 1.20 reads 1.00, a
// value the write path refuses and a file edited by hand may still carry — and
// nil is no value, pending staying pending. A value at the maximum is no
// special case: an authored 1.00 is in domain and reads as itself, which is
// what a converted experiment carries. The clamp is a reading rule and not a
// rewrite: the file keeps the bytes it had, and refresh leaves its rigor as it
// found it. A value the write path refuses therefore reads at the ceiling
// rather than making its KB unreadable.
func AuthoredRigor(v *float64) *float64 {
	if v == nil || *v <= RigorMaximum {
		return v
	}
	maximum := RigorMaximum
	return &maximum
}

// FaultTable is one vocabulary's closed set of fault ids, in walk order, and
// what one occurrence of each charges. The weight is per occurrence and the table
// caps no count: a text committing one fault three times loses three weights.
// Weights and the maximum are integer hundredths of the value's range, so every
// sum and every value is exact at two decimals and nothing rounds.
type FaultTable struct {
	// maximum is what a charge list with no charge takes:
	// rigorMaximumHundredths under either rigor vocabulary, and
	// bearingMaximumHundredths for bearing, which reserves nothing.
	maximum int
	faults  []fault
}

// fault is one criterion a vocabulary charges: its id and what one occurrence of
// it costs, in hundredths.
type fault struct {
	id     string
	weight int
}

// The three tables, each in the walk order the criteria are written in. A node's
// class selects which rigor table its charges are priced under; a bearing call
// has the one.
var (
	// RigorStatement is the statement vocabulary's: what a claim's own text is
	// charged with. Every criterion costs the same fifteen hundredths, so the
	// value is the proportion of findings the text avoided and nothing dies of one
	// judgement: a claim with a single loose parameter reads 0.85, and zero takes
	// seven findings.
	RigorStatement = FaultTable{maximum: rigorMaximumHundredths, faults: []fault{
		{FaultContradiction, 15},
		{FaultOverreach, 15},
		{FaultEquivocation, 15},
		{FaultUnderspecified, 15},
	}}
	// RigorMethod is the method vocabulary's: what an analysis's or an
	// experiment's own text is charged with. Every criterion costs the same
	// fifteen hundredths. FaultUnaddressed is not the table's whole maximum: a text
	// that displays nothing is one finding per respect, like any other, and seven
	// findings are what price a text at nothing.
	RigorMethod = FaultTable{maximum: rigorMaximumHundredths, faults: []fault{
		{FaultUnaddressed, 15},
		{FaultContradiction, 15},
		{FaultOverreach, 15},
		{FaultClaimedStep, 15},
		{FaultUnstatedPremise, 15},
		{FaultBareMeasurement, 15},
	}}
	// BearingFaults is the bearing vocabulary's: how much of a link's source
	// does work for the target. Every charge is one part of the source that does
	// none — one part that asserts nothing, reaches none of the target, runs
	// through an unstated bridge or depends on an unstated condition — and every
	// charge costs the same ten hundredths whatever its reason, so the value is
	// the proportion of parts that do work and nothing dies of one judgement.
	// Nothing a model can write in one charge prices a link at nothing: that
	// takes ten. The weight is absolute, so a long argument gathers more dings
	// than a short one for the same judged weakness; pricing the fraction the
	// dings are of the whole would take a second ask quantifying the argument's
	// volume, which no reliable answer gives, so the bias stands until one does.
	BearingFaults = FaultTable{maximum: bearingMaximumHundredths, faults: []fault{
		{FaultReferent, 10},
		{FaultNarrower, 10},
		{FaultTransfer, 10},
		{FaultConditional, 10},
		{FaultInapplicable, 10},
	}}
)

// IDs is the table's closed set in walk order: its own ids, and nothing else.
func (t FaultTable) IDs() []string {
	out := make([]string, 0, len(t.faults))
	for _, f := range t.faults {
		out = append(out, f.id)
	}
	return out
}

// Accepts is whether id is one of the table's ids. The caller case-folds what a
// reply wrote before asking; the table holds the ids in their own spelling.
func (t FaultTable) Accepts(id string) bool {
	for _, f := range t.faults {
		if f.id == id {
			return true
		}
	}
	return false
}

// Price is the value a charge list takes: the table's maximum less the weights
// its ids charge, floored at zero — a list whose weights reach the maximum is a
// text with nothing left to carry the assertion, and 0 is what the register says
// for that — or the pending literal where the list names the pending fault. The
// empty list is the answer ChargeNone, and it is the only one that reaches the
// maximum.
//
// The whole input is the ids a ledger line recorded, a run resumed after an
// update continues under the current ask, and an id the table does not hold
// charges nothing. FaultPending is no table's id, and this still prices a line
// that records it as pending: a value written before the id was retired keeps
// what it was written as.
func (t FaultTable) Price(ids []string) Bearing {
	sum := 0
	for _, id := range ids {
		if id == FaultPending {
			return Bearing{Pending: true}
		}
		for _, f := range t.faults {
			if f.id == id {
				sum += f.weight
				break
			}
		}
	}
	if sum > t.maximum {
		sum = t.maximum
	}
	return Bearing{Value: float64(t.maximum-sum) / 100}
}

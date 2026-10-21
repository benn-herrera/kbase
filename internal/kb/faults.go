package kb

// The fault ids the two scoring passes charge with: twelve in one namespace, six
// to each judgement, and the pending id both share. An id spells the criterion
// it names and carries no weight of its own — the pass's table does, per
// occurrence — and the two sets are disjoint, so a charge's id names its fault
// wherever it appears and a ledger line is read without a table.
const (
	FaultUnaddressed     = "flt-unaddressed"
	FaultContradiction   = "flt-contradiction"
	FaultOverreach       = "flt-overreach"
	FaultClaimedStep     = "flt-claimed-step"
	FaultUnstatedPremise = "flt-unstated-premise"
	FaultBareMeasurement = "flt-bare-measurement"

	FaultNoEngagement = "flt-no-engagement"
	FaultReferent     = "flt-referent"
	FaultPartial      = "flt-partial"
	FaultNarrower     = "flt-narrower"
	FaultTransfer     = "flt-transfer"
	FaultConditional  = "flt-conditional"

	// FaultPending is both passes' id for a text that holds no proposition to
	// judge: nothing is identifiable, so no other id applies. It takes no
	// citation, and its value is the pending literal whatever the table.
	FaultPending = "flt-pending"
)

// ChargeNone is the answer that charges nothing: a word, not an id, and the one
// answer that reaches a pass's maximum. It is the whole of an answer — beside a
// charge line the two halves contradict each other — and its record's charge
// list is empty.
const ChargeNone = "none"

// FaultTable is one pass's closed set of fault ids, in walk order, and what one
// occurrence of each charges. The weight is per occurrence and the table caps no
// count: a text committing one fault three times loses three weights. Weights
// and the maximum are integer hundredths of the pass's range, so every sum and
// every value is exact at two decimals and nothing rounds.
type FaultTable struct {
	// maximum is what a charge list with no charge takes: 0.95 for rigor, the
	// largest multiple of 0.05 below the 1.0 an axiom's rigor holds by rule, and
	// 1.00 for bearing, which reserves nothing.
	maximum int
	faults  []fault
}

// fault is one criterion a pass charges: its id and what one occurrence of it
// costs, in hundredths.
type fault struct {
	id     string
	weight int
}

// RigorFaults and BearingFaults are the two passes' tables, each in the walk
// order the criteria are written in, with the pending id last.
var (
	RigorFaults = FaultTable{maximum: 95, faults: []fault{
		{FaultUnaddressed, 95},
		{FaultContradiction, 45},
		{FaultOverreach, 35},
		{FaultClaimedStep, 30},
		{FaultUnstatedPremise, 30},
		{FaultBareMeasurement, 30},
	}}
	BearingFaults = FaultTable{maximum: 100, faults: []fault{
		{FaultNoEngagement, 100},
		{FaultReferent, 50},
		{FaultPartial, 50},
		{FaultNarrower, 50},
		{FaultTransfer, 35},
		{FaultConditional, 35},
	}}
)

// IDs is the pass's closed set in walk order: the six ids and FaultPending.
func (t FaultTable) IDs() []string {
	out := make([]string, 0, len(t.faults)+1)
	for _, f := range t.faults {
		out = append(out, f.id)
	}
	return append(out, FaultPending)
}

// Accepts is whether id is one of the pass's ids. The caller case-folds what a
// reply wrote before asking; the table holds the ids in their own spelling.
func (t FaultTable) Accepts(id string) bool {
	if id == FaultPending {
		return true
	}
	for _, f := range t.faults {
		if f.id == id {
			return true
		}
	}
	return false
}

// Price is the value a charge list takes: the pass's maximum less the weights
// its ids charge, floored at zero — a list whose weights reach the maximum is a
// text with nothing left to carry the assertion, and 0 is what the register says
// for that — or the pending literal where the list names the pending fault. The
// empty list is the answer ChargeNone, and it is the only one that reaches the
// maximum.
//
// The whole input is the ids a ledger line recorded, a run resumed after an
// update continues under the current ask, and an id the table does not hold
// charges nothing.
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

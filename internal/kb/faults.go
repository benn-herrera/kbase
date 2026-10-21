package kb

// The fault ids the three vocabularies charge with, in one namespace, and the
// pending id all three share. An id spells the criterion it names and carries no
// weight of its own — the item's table does, per occurrence. The statement and
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
	FaultNoEngagement = "flt-no-engagement"
	FaultReferent     = "flt-referent"
	FaultPartial      = "flt-partial"
	FaultNarrower     = "flt-narrower"
	FaultTransfer     = "flt-transfer"
	FaultConditional  = "flt-conditional"

	// FaultPending is every vocabulary's id for a text that holds no
	// proposition to judge: nothing is identifiable, so no other id applies. It
	// takes no citation, and its value is the pending literal whatever the
	// table.
	FaultPending = "flt-pending"
)

// ChargeNone is the answer that charges nothing: a word, not an id, and the one
// answer that reaches a table's maximum. It is the whole of an answer — beside a
// charge line the two halves contradict each other — and its record's charge
// list is empty.
const ChargeNone = "none"

// RigorMaximum is the most rigor an authored node carries, and what a rigor
// charge list with no charge prices to: the rigor tables' maximum, the largest
// multiple of 0.05 below the 1.0 an axiom's rigor holds by rule. It is the
// ceiling of every write that takes a node's own rigor, and no authored value
// reaches the reserve. Bearing has none — its table's maximum is 1.00.
const RigorMaximum = 0.95

// FaultTable is one vocabulary's closed set of fault ids, in walk order, and
// what one occurrence of each charges. The weight is per occurrence and the table
// caps no count: a text committing one fault three times loses three weights.
// Weights and the maximum are integer hundredths of the value's range, so every
// sum and every value is exact at two decimals and nothing rounds.
type FaultTable struct {
	// maximum is what a charge list with no charge takes: RigorMaximum under
	// either rigor vocabulary, and 1.00 for bearing, which reserves nothing.
	maximum int
	faults  []fault
}

// fault is one criterion a vocabulary charges: its id and what one occurrence of
// it costs, in hundredths.
type fault struct {
	id     string
	weight int
}

// The three tables, each in the walk order the criteria are written in, with the
// pending id named by none of them and accepted by all. A node's class selects
// which rigor table its charges are priced under; a bearing call has the one.
var (
	// RigorStatement is the statement vocabulary's: what a claim's own text is
	// charged with.
	RigorStatement = FaultTable{maximum: int(RigorMaximum * 100), faults: []fault{
		{FaultContradiction, 45},
		{FaultOverreach, 35},
		{FaultEquivocation, 25},
		{FaultUnderspecified, 5},
	}}
	// RigorMethod is the method vocabulary's: what an analysis's or an
	// experiment's own text is charged with.
	RigorMethod = FaultTable{maximum: int(RigorMaximum * 100), faults: []fault{
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

// IDs is the table's closed set in walk order: its own ids and FaultPending.
func (t FaultTable) IDs() []string {
	out := make([]string, 0, len(t.faults)+1)
	for _, f := range t.faults {
		out = append(out, f.id)
	}
	return append(out, FaultPending)
}

// Accepts is whether id is one of the table's ids. The caller case-folds what a
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

// Price is the value a charge list takes: the table's maximum less the weights
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

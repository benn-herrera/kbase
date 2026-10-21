package kb

import (
	"maps"
	"math"
	"slices"
	"testing"
)

// TestFaultTablesAreTheSchemes: the three closed sets, their weights and their
// maxima are the scheme's — each in walk order, the pending id shared by all
// three and named by none, and nothing else any of them accepts.
func TestFaultTablesAreTheSchemes(t *testing.T) {
	for _, c := range []struct {
		name    string
		table   FaultTable
		ids     []string
		maximum float64
	}{
		{"statement", RigorStatement, []string{
			FaultContradiction, FaultOverreach, FaultEquivocation, FaultUnderspecified, FaultPending}, 1.00},
		{"method", RigorMethod, []string{
			FaultUnaddressed, FaultContradiction, FaultOverreach,
			FaultClaimedStep, FaultUnstatedPremise, FaultBareMeasurement, FaultPending}, 1.00},
		{"bearing", BearingFaults, []string{
			FaultNoEngagement, FaultReferent, FaultPartial,
			FaultNarrower, FaultTransfer, FaultConditional, FaultPending}, 1.00},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.table.IDs(); !slices.Equal(got, c.ids) {
				t.Errorf("the set = %v, want %v", got, c.ids)
			}
			if got := c.table.Price(nil); got.Pending || got.Value != c.maximum {
				t.Errorf("the empty list prices %+v, want the maximum %v", got, c.maximum)
			}
			if got := c.table.Price([]string{FaultPending}); !got.Pending {
				t.Errorf("the pending fault prices %+v, want the pending literal", got)
			}
			for _, id := range c.ids {
				if !c.table.Accepts(id) {
					t.Errorf("the set does not accept its own id %s", id)
				}
			}
		})
	}
}

// TestTheChargeThatLeavesNothingTracksTheCeiling: the charge that means "this
// leaves nothing" is one of each table's faults and is the table's whole
// maximum, not a number chosen on its own — so charging it alone prices the
// text to zero, whatever the ceiling is. A weight written back as a literal
// beside a moved ceiling keeps pricing the old gap and fails here.
func TestTheChargeThatLeavesNothingTracksTheCeiling(t *testing.T) {
	for _, c := range []struct {
		name  string
		table FaultTable
		id    string
	}{
		{"method", RigorMethod, FaultUnaddressed},
		{"bearing", BearingFaults, FaultNoEngagement},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.table.Price([]string{c.id}); got.Pending || got.Value != 0 {
				t.Errorf("Price(%s) = %+v, want 0: the charge that leaves nothing is the table's whole maximum", c.id, got)
			}
			if w := weightOf(c.table, c.id); w != c.table.maximum {
				t.Errorf("%s weighs %d, want the table's maximum %d", c.id, w, c.table.maximum)
			}
		})
	}
}

// TestFaultIDsNameOneFaultEach: the bearing set is disjoint from both rigor sets,
// the two rigor sets meet in the two ids that name one fault at different
// subjects, and each of those carries one weight — so a ledger line's price does
// not depend on which rigor vocabulary read it.
func TestFaultIDsNameOneFaultEach(t *testing.T) {
	var shared []string
	for _, id := range RigorStatement.IDs() {
		if id == FaultPending {
			continue
		}
		if !RigorMethod.Accepts(id) {
			continue
		}
		shared = append(shared, id)
		if w, other := weightOf(RigorStatement, id), weightOf(RigorMethod, id); w != other {
			t.Errorf("%s weighs %d in the statement table and %d in the method one", id, w, other)
		}
	}
	if want := []string{FaultContradiction, FaultOverreach}; !slices.Equal(shared, want) {
		t.Errorf("the rigor vocabularies share %v, want %v", shared, want)
	}
	for _, id := range append(RigorStatement.IDs(), RigorMethod.IDs()...) {
		if id != FaultPending && BearingFaults.Accepts(id) {
			t.Errorf("%s is in a rigor set and the bearing set", id)
		}
	}
}

// weightOf is id's weight in table, and zero where the table does not hold it.
func weightOf(table FaultTable, id string) int {
	for _, f := range table.faults {
		if f.id == id {
			return f.weight
		}
	}
	return 0
}

// TestFaultPricesAreTheSchemes: each weight charges per occurrence, the sum is
// floored at zero, and every value is a multiple of 0.05 — each table's own
// arithmetic, on the charge lists the scheme tabulates.
func TestFaultPricesAreTheSchemes(t *testing.T) {
	for _, c := range []struct {
		name  string
		table FaultTable
		ids   []string
		want  float64
	}{
		{"a bare statement", RigorStatement, nil, 1.00},
		{"one underspecification", RigorStatement, []string{FaultUnderspecified}, 0.95},
		{"two underspecifications", RigorStatement, []string{FaultUnderspecified, FaultUnderspecified}, 0.90},
		{"one equivocation", RigorStatement, []string{FaultEquivocation}, 0.75},
		{"one statement overreach", RigorStatement, []string{FaultOverreach}, 0.65},
		{"one statement contradiction", RigorStatement, []string{FaultContradiction}, 0.55},
		{"a contradiction and an underspecification", RigorStatement, []string{FaultContradiction, FaultUnderspecified}, 0.50},
		{"two equivocations", RigorStatement, []string{FaultEquivocation, FaultEquivocation}, 0.50},
		{"an equivocation and an overreach", RigorStatement, []string{FaultEquivocation, FaultOverreach}, 0.40},
		{"two statement overreaches", RigorStatement, []string{FaultOverreach, FaultOverreach}, 0.30},
		{"an overreach and a contradiction", RigorStatement, []string{FaultOverreach, FaultContradiction}, 0.20},
		{"two statement contradictions", RigorStatement, []string{FaultContradiction, FaultContradiction}, 0.10},
		{"three contradictions", RigorStatement, []string{FaultContradiction, FaultContradiction, FaultContradiction}, 0.00},

		{"no method charge", RigorMethod, nil, 1.00},
		{"one gap", RigorMethod, []string{FaultClaimedStep}, 0.70},
		{"one method overreach", RigorMethod, []string{FaultOverreach}, 0.65},
		{"one method contradiction", RigorMethod, []string{FaultContradiction}, 0.55},
		{"two gaps", RigorMethod, []string{FaultClaimedStep, FaultBareMeasurement}, 0.40},
		{"a gap and an overreach", RigorMethod, []string{FaultUnstatedPremise, FaultOverreach}, 0.35},
		{"two method overreaches", RigorMethod, []string{FaultOverreach, FaultOverreach}, 0.30},
		{"a gap and a contradiction", RigorMethod, []string{FaultBareMeasurement, FaultContradiction}, 0.25},
		{"three gaps", RigorMethod, []string{FaultClaimedStep, FaultUnstatedPremise, FaultBareMeasurement}, 0.10},
		{"two method contradictions", RigorMethod, []string{FaultContradiction, FaultContradiction}, 0.10},
		{"the unaddressed text", RigorMethod, []string{FaultUnaddressed}, 0.00},
		{"a method sum past the maximum", RigorMethod, []string{FaultUnaddressed, FaultOverreach}, 0.00},

		{"one step weight", BearingFaults, []string{FaultTransfer}, 0.65},
		{"one half weight", BearingFaults, []string{FaultPartial}, 0.50},
		{"two step weights", BearingFaults, []string{FaultTransfer, FaultConditional}, 0.30},
		{"a half and a step", BearingFaults, []string{FaultReferent, FaultTransfer}, 0.15},
		{"two half weights", BearingFaults, []string{FaultNarrower, FaultPartial}, 0.00},
		{"the no-engagement link", BearingFaults, []string{FaultNoEngagement}, 0.00},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.table.Price(c.ids); got.Pending || got.Value != c.want {
				t.Errorf("Price(%v) = %+v, want %v", c.ids, got, c.want)
			}
		})
	}
}

// TestFaultPriceReachesOnlyTheSchemesValues: enumerating every charge list up to
// one charge per id reaches exactly the statement vocabulary's twenty-one
// multiples of 0.05, the method vocabulary's twelve values and bearing's six,
// and nothing else. A statement value is checked by the table's arithmetic
// rather than membership in a short list, because underspecification stacks; a
// method value off its twelve is a defect of a landing, not a judgement.
func TestFaultPriceReachesOnlyTheSchemesValues(t *testing.T) {
	var statement []float64
	for i := 0; i <= 20; i++ {
		statement = append(statement, float64(i)/20)
	}
	for _, c := range []struct {
		name   string
		table  FaultTable
		values []float64
	}{
		{"statement", RigorStatement, statement},
		{"method", RigorMethod, []float64{0.00, 0.05, 0.10, 0.20, 0.25, 0.30, 0.35, 0.40, 0.55, 0.65, 0.70, 1.00}},
		{"bearing", BearingFaults, []float64{0.00, 0.15, 0.30, 0.50, 0.65, 1.00}},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Every charge list up to the length at which the heaviest count
			// saturates the table. A weight is charged per occurrence, so one id
			// may be charged more than once; the sum depends on the multiset
			// alone, so the walk takes each multiset once, ids non-decreasing.
			longest := 1
			for _, f := range c.table.faults {
				longest = max(longest, (c.table.maximum+f.weight-1)/f.weight)
			}
			values := map[float64]bool{}
			var walk func(ids []string, left, from int)
			walk = func(ids []string, left, from int) {
				if got := c.table.Price(ids); !got.Pending {
					values[got.Value] = true
				}
				if left == 0 {
					return
				}
				for i := from; i < len(c.table.faults); i++ {
					walk(append(slices.Clone(ids), c.table.faults[i].id), left-1, i)
				}
			}
			walk(nil, longest, 0)
			got := slices.Sorted(maps.Keys(values))
			if !slices.Equal(got, c.values) {
				t.Errorf("the table reaches %v, want exactly %v", got, c.values)
			}
			for v := range values {
				if math.Abs(v*100-math.Round(v*100)) > 1e-9 {
					t.Errorf("%v is finer than a hundredth", v)
				}
			}
		})
	}
}

// TestBottomRungStatesAnOutcome: the ladder's last rung is `unsubstantiated` —
// slug, status phrase and label — at the standing cut, and a strength below the
// rung above it falls there whatever carried it.
func TestBottomRungStatesAnOutcome(t *testing.T) {
	rung := BuildBandLadder[len(BuildBandLadder)-1]
	if rung.Slug != "unsubstantiated" || rung.StatusPhrase != "unsubstantiated" || rung.Label != "unsubstantiated" {
		t.Errorf("the bottom rung = %+v, want unsubstantiated in every field", rung)
	}
	if rung.Threshold != 0.00 {
		t.Errorf("the bottom rung's cut = %v, want 0.00", rung.Threshold)
	}
	for _, v := range []float64{0, 0.05, 0.19} {
		if b := BandFor(&v); b == nil || b.Slug != "unsubstantiated" {
			t.Errorf("a strength of %v reads %+v, want the bottom rung", v, b)
		}
	}
	v := 0.20
	if b := BandFor(&v); b == nil || b.Slug != "do-not-build" {
		t.Errorf("a strength of 0.20 reads %+v, want the rung its cut names", b)
	}
}

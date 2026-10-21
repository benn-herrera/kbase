package kb

import (
	"maps"
	"math"
	"slices"
	"testing"
)

// TestFaultTablesAreTheSchemes: the two closed sets, their weights and their
// maxima are the scheme's — six ids each in walk order, the pending id shared,
// and nothing else either pass accepts.
func TestFaultTablesAreTheSchemes(t *testing.T) {
	for _, c := range []struct {
		name    string
		table   FaultTable
		ids     []string
		maximum float64
	}{
		{"rigor", RigorFaults, []string{
			FaultUnaddressed, FaultContradiction, FaultOverreach,
			FaultClaimedStep, FaultUnstatedPremise, FaultBareMeasurement, FaultPending}, 0.95},
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
	// The sets are disjoint but for the pending id: an id names one fault
	// wherever it appears.
	for _, id := range RigorFaults.IDs() {
		if id != FaultPending && BearingFaults.Accepts(id) {
			t.Errorf("%s is in both passes' sets", id)
		}
	}
	for _, id := range BearingFaults.IDs() {
		if id != FaultPending && RigorFaults.Accepts(id) {
			t.Errorf("%s is in both passes' sets", id)
		}
	}
}

// TestFaultPricesAreTheSchemes: each weight charges per occurrence, the sum is
// floored at zero, and every value is a multiple of 0.05 — the table's own
// arithmetic, on the charge lists the scheme tabulates.
func TestFaultPricesAreTheSchemes(t *testing.T) {
	for _, c := range []struct {
		name  string
		table FaultTable
		ids   []string
		want  float64
	}{
		{"one gap", RigorFaults, []string{FaultClaimedStep}, 0.65},
		{"one overreach", RigorFaults, []string{FaultOverreach}, 0.60},
		{"one contradiction", RigorFaults, []string{FaultContradiction}, 0.50},
		{"two gaps", RigorFaults, []string{FaultClaimedStep, FaultBareMeasurement}, 0.35},
		{"a gap and an overreach", RigorFaults, []string{FaultUnstatedPremise, FaultOverreach}, 0.30},
		{"two overreaches", RigorFaults, []string{FaultOverreach, FaultOverreach}, 0.25},
		{"a gap and a contradiction", RigorFaults, []string{FaultBareMeasurement, FaultContradiction}, 0.20},
		{"an overreach and a contradiction", RigorFaults, []string{FaultOverreach, FaultContradiction}, 0.15},
		{"three gaps", RigorFaults, []string{FaultClaimedStep, FaultUnstatedPremise, FaultBareMeasurement}, 0.05},
		{"two contradictions", RigorFaults, []string{FaultContradiction, FaultContradiction}, 0.05},
		{"the unaddressed text", RigorFaults, []string{FaultUnaddressed}, 0.00},
		{"a sum past the maximum", RigorFaults, []string{FaultUnaddressed, FaultOverreach}, 0.00},
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
// one charge per id reaches exactly the scheme's eleven rigor values over
// [0, 0.95] and its six bearing values, and nothing else. A rigor value off that
// list is a defect of a landing, not a judgement.
func TestFaultPriceReachesOnlyTheSchemesValues(t *testing.T) {
	for _, c := range []struct {
		name   string
		table  FaultTable
		values []float64
	}{
		{"rigor", RigorFaults, []float64{0.00, 0.05, 0.15, 0.20, 0.25, 0.30, 0.35, 0.50, 0.60, 0.65, 0.95}},
		{"bearing", BearingFaults, []float64{0.00, 0.15, 0.30, 0.50, 0.65, 1.00}},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Every charge list up to the length at which the heaviest count
			// saturates the table. A weight is charged per occurrence, so one id
			// may be charged more than once, and the walk carries every list the
			// table prices.
			longest := 1
			for _, f := range c.table.faults {
				longest = max(longest, (c.table.maximum+f.weight-1)/f.weight)
			}
			values := map[float64]bool{}
			var walk func(ids []string, left int)
			walk = func(ids []string, left int) {
				if got := c.table.Price(ids); !got.Pending {
					values[got.Value] = true
				}
				if left == 0 {
					return
				}
				for _, f := range c.table.faults {
					walk(append(slices.Clone(ids), f.id), left-1)
				}
			}
			walk(nil, longest)
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

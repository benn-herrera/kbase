package kb

import (
	"slices"
	"testing"
)

// TestNoChargeLeavesNothingAlone: no id in any table costs the table's whole
// maximum — a text or a link reaches zero by accumulating findings, never by one
// verdict — and an empty claim and an empty analysis price the same, so neither
// vocabulary treats silence as a special kind of badness. This asserts the
// decision, not the numbers: any uniform weight satisfies it.
func TestNoChargeLeavesNothingAlone(t *testing.T) {
	for _, table := range []FaultTable{RigorStatement, RigorMethod, BearingFaults} {
		for _, id := range table.IDs() {
			if w := weightOf(table, id); w == table.maximum {
				t.Errorf("%s weighs the table's whole maximum: one finding must not price it at nothing", id)
			}
			if got := table.Price([]string{id}); got.Pending || got.Value == 0 {
				t.Errorf("Price(%s) = %+v, want more than nothing", id, got)
			}
		}
	}
	if a, b := RigorStatement.Price([]string{FaultUnderspecified}), RigorMethod.Price([]string{FaultUnaddressed}); a != b {
		t.Errorf("an empty claim prices %+v and an empty analysis %+v, want one value for both", a, b)
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

package format200

// RigorMaximum is the most rigor a node carries, and what a rigor charge list
// with no charge prices to: the rigor tables' maximum, the value a text charged
// with nothing takes. It is the ceiling of every write that takes a node's own
// rigor, and it is the whole of the scale — an axiom's rule value, a pass's
// largest price and an authored maximum are one number, and nothing is held
// back from it. Bearing has no reserve either: BearingMaximum is its table's.
const RigorMaximum = 1.0

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

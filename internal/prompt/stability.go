package prompt

// CheckStability is the per-call churn tripwire (ARCHITECTURE.md §7, layer 3
// of phase-gated churn enforcement): every slot the current phase declares
// stable must hash identically to the previous call, and a mismatch is a loud
// refusal rather than a quietly halved cache hit rate.
//
// The stable set is a FRONTIER, not an arbitrary set, and the parameter says
// so: prefix caching only ever pays for a contiguous run of slots from the
// front, so "slots 1 and 3 are stable while 2 churns" is not a weaker
// guarantee — it is a destroyed cache reported as holding. Taking a single
// Slot makes that state unrepresentable. Every slot up to and including
// frontier is checked; the zero frontier declares nothing stable and passes.
//
// It is pure and cheap — a few array comparisons per call — so it runs
// always, in production, not behind a debug flag. This is the runtime form of
// the prefix-stability property the builder's tests assert; the frontier comes
// from the orchestrator's phase matrix, which is the single source both
// consult.
//
// First call in a phase: pass SlotTotal (the zero frontier) — there is no
// previous call, and nothing is claimed. Passing a real frontier with a nil
// prev is ErrNoPreviousHashes, because the way that happens in practice is a
// refused Build handing back a hashless BuiltCall.
func CheckStability(frontier Slot, prev, cur map[Slot][32]byte) error {
	if frontier <= SlotTotal {
		return nil
	}
	if prev == nil {
		return ErrNoPreviousHashes{Frontier: frontier}
	}
	for _, s := range allSlots {
		if s > frontier {
			break
		}
		p, ok := prev[s]
		if !ok {
			return ErrMissingHash{Slot: s}
		}
		c, ok := cur[s]
		if !ok {
			return ErrMissingHash{Slot: s}
		}
		if p != c {
			return ErrSlotChurn{Slot: s}
		}
	}
	return nil
}

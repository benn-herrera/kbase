package prompt

// CheckStability is the per-call churn tripwire (ARCHITECTURE.md §7, layer 3
// of phase-gated churn enforcement): the slots the current phase declares
// stable must hash identically to the previous call, and a mismatch is a loud
// refusal rather than a quietly halved cache hit rate.
//
// It is pure and cheap — a few array comparisons per call — so it runs
// always, in production, not behind a debug flag. This is the runtime form of
// the prefix-stability property the builder's tests assert; the stable set
// comes from the orchestrator's phase matrix, which is the single source both
// consult.
//
// A nil prev means there is no previous call in this phase (the first call
// after a transition), and there is nothing to compare. An empty stable set
// declares nothing stable and always passes.
func CheckStability(stable []Slot, prev, cur map[Slot][32]byte) error {
	if prev == nil {
		return nil
	}
	for _, s := range stable {
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

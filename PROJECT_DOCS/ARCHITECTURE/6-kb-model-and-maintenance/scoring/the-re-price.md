#### The re-price

**Pattern:** Projection rebuild from stored evidence.

`Resume` takes the same `Write` interface a run's writing is opened with, and `reprice` recomputes every
written line's value from the ids its line records, under the table the line's own class holds. A run
written under one rigor table is recomputed under the class table. Only a line that records `judged`
and its charges is recomputed; a line without them says nothing of what text its value judged and is left
as it is, as is a line for an item `skip` names.

A value the running table gives differently is written first through the batch above, under the KB write
lock, and only then does the line record it, so no line claims a value the KB did not take. A value the
write discards (`errTextChanged` or `errItemGone`) rewrites its line with that reason through
`attempt.fail`, as a loop's discarded write does: `failed` for a changed item, which the loop that
follows sends again, and `skipped` for a removed one, which nothing sends again. Any other refused write
fails the resume with the ledger as it stood; the next resume recomputes the same change, the write being
idempotent. Each changed line is listed in `repriced.yaml` in the run's directory, written only where a
value was written: the run, the time, and per line the item, the value it held and the value it took.
`run.yaml` is written once, so `repriced.yaml` is where a recomputation is recorded. No model is asked: the ids are the whole input of the value, and the
model never saw a weight. Behaviour: SPEC §8 `ask_sha256` records the ask set and gates nothing.


#### Writing the results

**Pattern:** Optimistic concurrency with partial-batch retry.

`internal/score`'s `landValues` writes a run's answered values in one write batch, under one hold of the
KB write lock taken through `write.LockKB` ([§8](../../8-ledger-state-store-and-monitoring/locks.md); wait: `write.LockWait`, [§12](../../12-constants/index.md)).
It is the single path to the KB: the loop's end, a resume's merge of stopped results and a resume's
recomputation all reach it through the `Write` interface (SPEC §8 Writing the results).

Under the lock and before any batch, `landValues` opens the KB afresh through the interface the verb
supplies (`Invocation.Open`, `kbload.Open` in `cmd/score.go`) and reads it through `query.Load` and
`graphOf` (`graphAt`). Each value is then checked against the KB it finds:

- A KB that does not open or load fails every value with its error.
- A value whose node or edge is not in the KB fails with `errItemGone`, which `attempt.fail` records as
  status `skipped`, not `failed`, so `ReadRun` counts it skipped and `ResumeTarget` does not resume
  for it. A resume's loop records an item of its run that is not in the in-memory index it loaded the
  same way, once (`Runner.Run`, `inKB`), and makes no call for it.
- A value whose `Landing.Judged` (the judged hash `prepare` recorded as its call was composed, carried
  on the answer file and the ledger line as `judged`) differs from `judgedHash` over the text the KB now
  holds fails with `errTextChanged`: a statement edited by hand while no lock kept the edit out. A
  resume sends it again.

Only the remaining values go into the batch, so the hash the edit op records is always the hash of the
text the value judged. They form one mixed write batch (`write.Batch`) through `write.Run`, an entry per
value in the order given. `opOf` names the edit op: `edit-claim`, `edit-analysis` or `edit-experiment` by
the node's kind with `id` and `rigor`, or `edit-edge` with `source`, `target`, `relation` and `bearing`.
The batch is applied under `Provenance: write.ProvenanceMachine`, so a written value has
`manually-scored: false`, and it rebuilds the derived views inside the batch, so the records and the
derived views are written together. A `done` or `unchanged` batch has written its values.

A refused batch writes nothing (SPEC §8 Write batches). Where the refusal names its entries, those items
fail and the batch is retried without them, until it is written or empty. Where it names none, the
batch's values are retried one at a time under the same hold, each refusal then failing its own item;
this covers a refusal the reopen did not foresee, a hand edit between the reopen and the batch, or a
check the write makes that a load does not. A failed value's error is its details joined and formatted
for the command line (`valueError`), so no ledger line carries a marker. A batch that fails, as against
being refused, fails every item it contained. A lock that cannot be taken fails every item too: one held
past the wait fails each with the detail of `write.HeldLockItem`, the item a verb answers `retry` with,
and any other lock failure fails each with the lock's own error. The run continues. `landValues` returns
one error per value, nil where it was written. It checks for no run lock: it writes as the run holding
that lock, so no run can be holding the KB out.

Each value the batch sets records its judged hash in `internal/write` (`judged.go`). An entry that sets
a rigor or a bearing names the record through `valueSet`: a caller's value, `*pending*` included, or a
run's number. A run's `*pending*`, which the build writes for `score` to judge, is no judgement and names
none. Once the entry has applied, `hashValuesSet` gives each named record `asks.NodeJudgedHash` or
`asks.EdgeJudgedHash` over the statements the batch leaves, read through `kb.Statement` from the host
leaf, so a node an add marks in the same entry is hashed over the statement its marker gives it.


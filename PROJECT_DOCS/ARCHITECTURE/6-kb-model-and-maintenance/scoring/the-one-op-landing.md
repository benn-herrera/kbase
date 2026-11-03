#### The one-op landing

`internal/score`'s `landValues` lands a run's answered values in one landing,
under one hold of the KB write lock taken through `write.LockKB`. Under that hold, before any
batch, it opens the KB afresh through the seam the verb supplies (`Invocation.Open`, `kbload.Open`
in `cmd/score.go`) and reads it through `query.Load` and `graphOf` (`graphAt`). A KB that does not
open or load fails every value with its error. A value whose node or edge is no longer in the KB
fails with `errItemGone` ("the node or edge is no longer in the KB; its value is discarded and a
resume does not ask it again"), which `attempt.fail` records as status `skipped`, not `failed`, so
`ReadRun` counts it skipped and `ResumeTarget` does not resume for it. A resume's walk records an
item of its run that is no longer in the index it loaded the same way, once (`Runner.Run`, `inKB`),
and asks nothing of it. A value whose `Landing.Judged` — the judged hash `prepare` recorded as its
call was composed, carried on the result file and the ledger line as `judged` — differs from
`judgedHash` over the text the KB now holds fails with `errTextChanged` ("its text was edited after
it was asked; the value is discarded and a resume asks it again"): a statement edited by hand while
no lock kept the edit out. Only the values left go into the batch, so the hash the edit op records
is always the hash of the text the value judged. It issues one mixed write batch
(`write.Batch`) through `write.Run`, an entry per value in the order given — `opOf` names the edit
op: `edit-claim`, `edit-analysis` or `edit-experiment` by the node's kind with `id` and `rigor`,
`edit-edge` with `source`, `target`, `relation` and `bearing` — under
`Provenance: write.ProvenanceMachine`, so a landed value is written `manually-scored: false`, and
with the derivation, so the records and the derived files are written together. A `done` or
`unchanged` batch lands. A refused batch writes nothing (SPEC §8): where the refusal names its
entries, those items fail and the batch is re-issued without them, until it lands or is empty;
where it names none, the batch's values are re-issued one at a time under the same hold, each
refusal then failing its own item. That case is a refusal the reopen did not foresee — a hand edit
landing between the reopen and the batch, or a check the write makes that a load does not. A
failed value's error is its details joined and spelled for the command line (`valueError`), so no
ledger line carries a marker. A batch that fails, as against being refused, fails every item
it carried. A lock that cannot be taken fails every item too: one held past the wait fails each
with the detail of `write.HeldLockItem`, the item a verb answers `retry` with, and any other lock
failure fails each with the lock's own error. The run goes on. `landValues` returns one error per
value, nil where it landed. The landing probes for no operation lock: it writes as the run
holding that lock, so no run can be holding the KB out.

Each value the batch sets records its judged hash in `internal/write` (`judged.go`): an entry
that sets a rigor or a bearing names the record through `valueSet` — a caller's value, `*pending*`
included, and a run's number; a run's `*pending*`, which the build writes for `score` to judge, is
no judgement and names none — and once the entry has applied, `hashValuesSet` gives each named
record `asks.NodeJudgedHash` or `asks.EdgeJudgedHash` over the statements the batch leaves, read
through `kb.Statement` from the host leaf, so a node an add marks in the same entry is hashed over
the statement its marker gives it.

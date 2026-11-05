### Write API

**Pattern:** All-or-none validated batch with dry run and per-file atomic writes.

Every write is a batch (SPEC §8 Write batches), run by `write.Run`: `write.Batch` for `kbase write`'s
mixed batch, an op's name for a batch of that op alone. `Run` takes no lock; its caller holds the KB
write lock across it ([§8](../../8-ledger-state-store-and-monitoring/locks.md)). A batch reads the KB, applies every entry in memory, rebuilds the
derived views in memory, then writes each changed file once. The phases:

1. *Shape.* `parseValues` checks every entry against its op's fixed key vocabulary, `opFields` in
   `values.go`, the single source of per-op keys; `OpVocabulary` hands it to the help text and the MCP
   tool schema. A mixed entry's op is read from its `operation` key and must be in `Ops()`. Each op's
   own checks over its entries follow (`batchChecks`; an edit's `refuseFieldless`). Every fault is
   returned at once.
2. *Apply.* `Run` opens the KB through `kbload.Open`, reads it (`index.Derive` reads every document and
   both record files) and refuses what `Derive`, `index.PendingConversion` and `index.Precheck`
   refuse. `applyEntries` then
   loads both record files through `kb.LoadGraph`, keeping the bytes the disk held, and applies each
   entry in order to the batch's `kb.Graph` and to the leaves it marks (`leafEdit`), each judged
   against what earlier entries left. The first refusal stops the batch and names its entry. An
   entry's outcome is `done` or `unchanged` by whether the rendered record files or leaves changed under
   it. Facts only known while an op applies (a minted id, `Restored`, `RemovedEdges`, a citation's
   document, line and link) travel on `entryReport`, which `applyEntries` copies into `EntryResult`;
   the names an entry's values give are read from the values afterward, and so are an edge entry's
   `source`, `target` and `relation` once it has applied. The record files are rendered
   whole through `kb.EncodeEdges` and `kb.EncodeNodes`, and a file whose bytes did not move is no
   change. That the decoder reads back what the encoder wrote is `format400`'s round-trip unit test,
   not a runtime check. `readBackMarkers` refuses a leaf whose composed text does not contain each marker
   the batch placed in it.
3. *Rebuild the derived views.* The changed record files and leaves go into the `kb.Source` overlay,
   unwritten, and `index.PlanDerivation` computes the summary pages and sheets from it, writing
   nothing. Every batch does this, the build's and the scoring run's own writes included (SPEC §8
   Writes leave nothing stale); no option skips it. The one exception is a batch of a build-only op
   that leaves only cycles among the scoring edges ([§6](../build-only-ops.md)).
4. *Write.* `writeChanges` writes `edges.yaml`, `nodes.yaml`, the leaves, then the derived files, each
   temp file then rename (`index.WriteKBFile`, `atomicfile`), and removes what the rebuild drops.
   A dry run (SPEC §8 Write batches) gives the same result with nothing written.

A failure in any phase before the write writes nothing. The write is atomic per file and not as a set:
a failure part-way is `failed`, naming what was already written (`interruptedError`), and recovery is git
(SPEC §6 Git holds the recoverable record).

**Pattern:** Optimistic concurrency check (compare before write) layered on a lock. Before the write,
`stillMatches` compares each file with the bytes the batch read; a difference is `retry`
(`RetryRemedy`) with nothing written. For a committing batch the comparison runs again once
`Preflight` returns, so a hook that edited a file meanwhile turns the batch into `retry`.

A committing batch (the document's `commit`, or `write.Options.Commit`; SPEC §8 Write ops) calls the
caller's `write.Options.Preflight` before its first write, and its error stops the batch with nothing
written. A committing dry run calls it too, which opens the repository and commits nothing.
`cmd/write_op.go` supplies it: `kbCommits.preflight` makes the snapshot commits of `kb-root/`'s
uncommitted changes outside `.index/` (owned `kb-root` less `gitledger.Exclude("kb-root/.index")`),
then those inside it, each only where `Repo.Dirty` finds any. Once `write.Run` returns `done`,
`kbCommits.final` commits `kb-root/` under the label `batchLabel` composes from the entries' ops,
their ids (`EntryResult.ID`) and the ids their values name (`EntryResult.Names`). Each commit's
`paths` are what `Repo.Paths` reads back from it.

The remaining mechanisms are per op family. No op accepts a preformatted record or marker: every value
is a values key, and the marker is `kb.StatementMarker`'s. `kb.Bearing` keeps `Value` and `Pending`
apart, so a written `0` reads back as `0` and a written `*pending*` as pending. An op is one per node
kind for `add-<kind>` and `edit-<kind>`, whose keys differ by kind in `opFields` (an experiment's
`status`, a work's and an axiom's `label`); `remove-node`, `move-node` and `add-citation` take any
kind by `id`.

Contents:

- [§6 Adds](adds.md)
- [§6 Edits and renames](edits-and-renames.md)
- [§6 Edge ops](edge-ops.md)
- [§6 Removal and move](removal-and-move.md)
- [§6 Citations](citations.md)

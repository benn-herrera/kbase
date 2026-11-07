### Write batches

Every write goes through a batch, on every interface. An entry is one item of a batch.

**Shape.** A batch is an ordered list of entries under `entry`. Each entry names its op under `operation` and has that op's keys; a batch may mix ops. `dry-run` and `commit` sit beside `entry` as batch options. An empty batch is valid: it writes no record and rebuilds the derived views ([Writes leave nothing stale](writes-leave-nothing-stale.md)), which brings them up to date after an author's edits to leaves.

**Interfaces.** Over MCP, the `write` tool takes a batch ([§11](../../11-the-mcp-server/index.md)). At the command line, `kbase write` takes a mixed batch, each entry naming its op, with `--dry-run` and `--commit`. Each write op also has its own subcommand (`kbase add-edge`, …), which takes a batch of that op alone with the same options; its entries have no `operation`, since the subcommand names the op.

**Order.** Entries apply in the order given. Each entry is checked against the records as the earlier entries left them. kbase never reorders a batch; the caller orders it so that each entry is valid at its position.

**No reference between entries.** No entry names an id that another entry of the same batch mints. Every add mints its node's id, for every kind ([§10](../../10-metadata-format-version/five-node-types.md)), and the result gives each minted id beside the entry that minted it. Adding nodes and their edges takes two batches: the first adds the nodes, the second the edges. Between the two the KB is consistent, since a node with no edges is a valid node.

**Two phases.** Before anything applies, every entry's shape is checked: an unknown op, an unknown or missing key, a value's type, and a value outside a fixed set (`relation`, `status`, …). Every shape error is reported at once. Then the entries apply in order, each checked in full against the records as they are at that point, and the batch stops at the first refusal. An entry that would leave the records invalid is refused even if a later entry would repair it; the refusal's remedy names the order that works. No separate end-of-batch check exists. Each refusal item has the keys of [§7](../../7-tool-result-contract-and-exit-codes/item.md), with `entry` the entry's 1-based position in the batch.

**All or nothing.** A batch that is refused or fails writes nothing. The rebuild of the derived views is part of the batch, so the records are never written while their derived views are stale. Each file a batch writes is replaced atomically — written whole, then renamed into place — so a crash leaves each file either old or new. A crash between the writes of two files is not atomic across files; git recovers that case ([Git holds the recoverable record](../../6-build-state-ledger-and-monitoring/git-holds-the-recoverable-record.md)).

**Refused before it applies.** A batch is refused, whatever its entries, over:
- a KB whose load fails (`check: metadata`, [§10](../../10-metadata-format-version/the-load-every-fault-refused.md));
- a KB in an older format that is not yet converted (`check: index`, remedy `kbase update-format`; [§10](../../10-metadata-format-version/one-version-read-and-written.md)); a dry run is refused too;
- a lock ([One write lock](one-write-lock.md));
- a `kb-root/CLAUDE.md` that is not the redirect (`check: agents-redirect`, [§3](../../3-compatibility-contract/stamped-documents.md)).

**Dry run.** `dry-run: true` does everything but write. Its result is identical to a real run's, refusals and removed edges included, and says `dry-run: true`. A dry run never commits: with `commit` given it makes no commit, and its result's `commits` is empty.

**Result.** The result's keys are the `write` row of [§7](../../7-tool-result-contract-and-exit-codes/item.md). An entry's outcome is `done` or `unchanged`. The batch outcome is `unchanged` where every entry is `unchanged` and the rebuild of the derived views wrote nothing, and `done` otherwise.

Contents:

- [§8 Values input](values-input.md)
- [§8 Idempotence](idempotence.md)
- [§8 Writes leave nothing stale](writes-leave-nothing-stale.md)
- [§8 One write lock](one-write-lock.md)
- [§8 Write ops](write-ops.md)

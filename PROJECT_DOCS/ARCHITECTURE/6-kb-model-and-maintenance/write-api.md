### Write API

Every write is a batch (SPEC §8), run by `write.Run` — `write.Batch` for `kbase write`'s mixed
batch, an op's name for a batch of that op alone — in the phases [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) orders:

1. *Shape.* `parseValues` checks every entry against its op's closed vocabulary, a mixed entry's op
   read from its `operation` key, then each op's own checks over its entries (`batchChecks`: an
   edit's `refuseFieldless`), and returns every fault at once.
2. *Apply.* `applyEntries` reads both record files through `kb.LoadGraph` and applies each entry
   in order to the batch's `kb.Graph` and to the leaves it marks; the first refusal stops the
   batch and names its entry. Each entry's report is
   whether the record files or leaves moved under it (`done` or `unchanged`) and, on an add, the
   id it minted (nothing is adopted: an add of what stands is refused, SPEC §8), or, on `restore-demoted`, the relations it put back, or, on
   `remove-node`, the edge records it removed, or, on `add-citation`, the node it cites and the
   document, line and link it wrote. The two files are rendered whole through `kb.EncodeEdges` and
   `kb.EncodeNodes`, and a file whose bytes did not move is no change.
3. *Derive.* The changed record files and leaves are put into the `kb.Source` overlay, unwritten,
   and `index.PlanDerivation` computes the register pages and sheets from it, writing nothing.
4. *Write.* Once `readBackMarkers` has found each marker where it was placed and the disk still
   holds what was read, `writeChanges` writes `edges.yaml`, `nodes.yaml`, the leaves, then the
   derived files, each temp-then-rename, and removes what the derivation drops — unless the batch
   is a dry run, whose result is the same with nothing written. A committing batch (the document's
   `commit`, or `write.Options.Commit`) first calls the caller's `write.Options.Preflight`, and its
   error stops the batch with nothing written; a committing dry run calls it too, which opens the
   repository and commits nothing. Once a real run's pre-flight returns, the disk is checked again against what was read, so a hook that edited a file during the pre-flight
   turns the batch into `retry`. `cmd/write_op.go`
   supplies it: `kbCommits.preflight` commits `kb-root/`'s uncommitted changes outside `.index/`
   (owned `kb-root` less `gitledger.Exclude("kb-root/.index")`), then those inside it, each only
   where `Repo.Dirty` finds any. Once `write.Run` returns `done`, `kbCommits.final` commits
   `kb-root/` under the label `batchLabel` composes from the entries' ops, their ids
   (`EntryResult.ID`) and the ids their values name (`EntryResult.Names`); each commit's `paths`
   are what `Repo.Paths` reads back from it ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).

A failure in any phase before the write writes nothing; one inside the write is `failed`, naming
what already landed. Every batch derives, the build's and the scoring run's own writes included
(SPEC §5, [page §2.3](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)); no option skips phase 3. The one batch written without its
derivation is a build-only op's (`add-build-edges`, `demote-claim-edge`) that leaves the scoring
edges holding a cycle (`holdsOnlyCycles` in `ops.go`): the derivation cannot read such records,
and the build's ring cut demotes the cycle in the batch after, whose derivation brings the derived
files current before the stage's boundary. The build's node writes are `add-claim` and
`add-work`, its culls one mixed batch of `remove-node` entries ordered so a culled claim whose
rationale mentions another is removed first (`write.Mentions`, the test `remove-node` refuses
by), an `edit-claim` clearing a rationale where culled claims mention each other round a loop
(`internal/claimgraph/elision.go`); a title its kind holds already takes a counter, ` (2)` on
(`uniqueTitle` in `internal/claimgraph/write.go`). A block claim so countered no longer meets
its block by title in `readGraph`; it joins the block of its leaf whose display line its marker
stands on — the line `add-claim` was given, found by the same `write.ExcerptLines` search within
the block's extent — where no claim of that leaf carries the block's own title: an exact,
positional match, never the counter read back. That the decoder reads back
exactly what the encoder wrote is `format400`'s round-trip unit test, not a runtime check. A batch
that derives first refuses what `index.Precheck` refuses (SPEC §8). No op
accepts a preformatted record or marker: every value is a values key, and the marker is
`kb.StatementMarker`'s. A scoring edge's judgement is written by `add-edge` and `edit-edge` through their `bearing`, and a
node's local value by `add-<kind>` and `edit-<kind>` through their `rigor`; each refuses a value
outside the domain or the pending literal, and the edge ops a pairing the relations table does not join and a
`bearing` on `references`, and `edit-edge` a `demoted` record (SPEC §8). A written `0` re-reads
as `0` and a written `*pending*` as pending: `kb.Bearing` carries `Value` and `Pending` apart.


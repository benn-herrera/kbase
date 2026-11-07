### Build-only ops

**Pattern:** Internal privileged ops.

Three write ops are the build's alone: `add-build-edges` (`write.AddBuildEdges`), `demote-claim-edge`
(`write.DemoteClaimEdge`) and `split-claim` (`write.SplitClaim`); a fourth, `cite-bare-ids`
(`write.CiteBareIDs`), is `update-format`'s alone. All four are declared `buildOnly`: they are absent
from `write.Ops()`, have no subcommand and are refused in a mixed batch, so the public write
operations cannot create or retarget a `demoted` record. `restore-demoted` is public, and
`remove-edge` removes a `demoted` record.

- `add-build-edges` writes the build's edges as `add-edge` writes one, without its cycle refusal. A
  `rests-on` premise `founds` the node where `kb.BearingLegal` allows `founds` from its source and
  `supports` it otherwise; `references` are accepted on a claim only. An edge the records already
  contain is left as it is, and an edge with a `demoted` record of its relation is written with these
  values and the record deleted.
- `demote-claim-edge` replaces one standing scoring edge, named by its `source`, `target` and
  `relation`, with the `kb.DemotedEdge` that keeps it whole. Where only that record exists the edge
  is left as it is; an edge with neither is refused.
- `split-claim` handles one claim. Where an analysis already lifts the claim it uses that one;
  otherwise it mints an analysis and places its marker at a given line of a given leaf, and, where
  given a `statement-end`, relocates the claim's marker to that byte offset of line `statement-line`,
  or of the claim's own line where none is given. It retargets every `supports` and `founds` record
  into the claim, and every `demoted` record that replaced a `supports` edge into it, onto the
  analysis, and adds the analysis's `lifts` of the claim at `*pending*`. It runs no cycle check: a
  path through the analysis is the path that ran through the claim.
- `cite-bare-ids` takes one entry with no keys. It reads every bare id of a node (`kb.TextBareIDs`,
  the reader `verify`'s `BareIDFindings` and `remove-node`'s `refuseMentions` share) in every
  document `eachDocument` visits, and every cited node's link target (`targetOf`) before it
  rewrites any document, since `targetOf` opens the host leaf; an id whose node has no target
  stays bare. It then replaces each id, or the whole link whose text it is, with the naming form
  from the pieces `addCitation` composes (`kb.Citation`, `titleOf`, `readsAsOneLink`) — not through
  `addCitation`, which refuses a snippet standing twice on a line — and re-points every link into a
  heading the citing moved (`movedHeadings`, `repoint`). Its entry reports each id it cited
  (`EntryResult.Cited`). It writes no rigor or bearing, so it is no machine op.
  `write.CheckCiteBareIDs` runs the same citing in memory over a `kb.Source`, a converted one
  included, and returns its refusal, writing nothing.

`add-build-edges` and `demote-claim-edge` run over records holding a cycle among the scoring edges
(`index.DeriveOverCycles`). The build's edge batch closes such a cycle and cycle breaking
([§6](cycle-breaking.md)) removes it within the same stage: its removals reach the records through
`demote-claim-edge`, at the tail of `depends-attributed` ([§4](../4-stage-table/index.md)). `split-claim` is
applied by `analyses-split`. A batch of `add-build-edges` or `demote-claim-edge` that leaves records
the rebuild refuses for a cycle among the scoring edges alone (`holdsOnlyCycles` in `ops.go`) is
written without its rebuild of the derived views, because the rebuild cannot read such records; the
cycle-breaking batch after it rebuilds them before the stage's boundary.

The build's other writes are `add-claim`, `add-work`, and `remove-node` and `edit-claim` for culls.
None of the build's own writes takes the KB write lock; the build takes it once, at start, to wait out
a write in progress ([§8](../8-ledger-state-store-and-monitoring/locks.md)).

- **Culls.** A cull is one mixed batch of `remove-node` entries, ordered so that a culled claim whose
  rationale mentions another is removed first (`write.Mentions`, the test `remove-node` refuses by).
  Where culled claims mention each other round a loop, an `edit-claim` entry clears a rationale first
  (`cullEntries` in `internal/claimgraph/elision.go`).
- **Titles.** A title its kind already has takes a counter, ` (2)` on (`uniqueTitle` in
  `internal/claimgraph/write.go`). An `add-claim` entry whose title a claim in its document already
  has takes that claim's id and writes nothing.
- **Block claims.** A block claim given a counter does not meet its block by title in `readGraph`. It
  joins the block of its leaf whose display line its marker is on, where no claim of that leaf has
  the block's own title. That line is the one `add-claim` was given, found by the same
  `write.ExcerptLines` search within the block's extent: an exact, positional match, never the
  counter read back.


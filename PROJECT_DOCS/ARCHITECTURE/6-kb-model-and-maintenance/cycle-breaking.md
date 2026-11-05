### Cycle breaking

**Pattern:** Greedy feedback-arc removal by DFS back edges.

`index.BreakCycles(g, hosted)` returns the graph with no cycle among its scoring edges, the edges it
removed in order, and a warning per removal. Each removed edge becomes its `kb.DemotedEdge`. SPEC §10
Cycles: cut as the walk finds them defines the rule; the mechanism is:

- The node order is `hosted` (`index.HostedOrder`, the order the summary pages render nodes) and then
  every other node by id; an edge sorts by its relation's place in `kb.Relations`, then its source's,
  then its target's.
- The search is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, from every
  node in turn, following each node's edges in that order (`closeCycle`); `demoted` records are not
  followed.
- An edge reaching a node on the current path closes a cycle. The edge removed is the first, in that
  order, of the cycle's `supports` edges into an analysis, else the closing edge.
- After each removal the search starts over, so the removals depend on the edge set and the node
  order alone.
- One edge is removed at a time, never the whole cycle, and each removal is kept as a `demoted`
  record, so `restore-demoted` puts the edge back exactly and repairing a cycle is at most two edits.
- Why a `supports` edge into an analysis is preferred, and why the removal set is not a minimum
  feedback arc set, is not recorded.

Three callers use it:

- `Derive` runs it over every load and only detects: a cycle in loaded records is a load error, one
  item per edge the search would remove (`deriveGraph`, `loadDerivation`).
- `DeriveOverCycles` accepts such records and computes strength over `Rows.Resolved`, the graph with
  those edges removed.
- The build's cycle breaking (`ringRow` in `internal/claimgraph/stages.go`) takes the removals over
  `index.HostedOrder` and applies them in one `demote-claim-edge` batch, in removal order, because the
  search computes every removal over one graph before any is applied.

Public ops refuse an edge that would close a cycle ([§6](write-api/edge-ops.md)), so only the build breaks cycles.


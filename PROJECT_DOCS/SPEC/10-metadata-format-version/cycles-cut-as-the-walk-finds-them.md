### Cycles: cut as the walk finds them

The build breaks the cycles its own edges close, within the stage that writes them, by one
depth-first search; a cycle the records hold at load is a load error (The load). Cycle breaking
removes the first `supports` edge into an analysis on the cycle where there is one, else the back edge the search finds, replacing it with its `demoted` record.

- The search order is the node order of the summary pages, which no minted id decides, so a build's
  removals are reproducible modulo ids ([§5](../5-build-behaviour/determinism.md)): hosted nodes in the reading order of
  their markers, then every other node by id — a work or an axiom, which no scoring edge enters,
  lies on no cycle. Edges compare by relation in the order `supports`, `founds`, `lifts`,
  `refutes`, then source, then target, each end in that node order.
- The search is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, starting at
  each node and following each node's edges in the search order; `demoted` records are not
  followed.
- Each time the search closes a cycle — an edge reaching a node on the current path, the back edge —
  it removes one edge of that cycle: the first, in the search order, of the cycle's `supports` edges
  into an analysis, where one exists; else the back edge. The removed edge becomes its `demoted`
  record, with a warning naming the edge and the cycle it broke. Nothing else is dropped.
- After each removal the search starts over, until no cycle remains, so the removals depend on the
  edge set and the documents' order alone.

Every public op that adds a scoring edge refuses one that would close a cycle ([§8](../8-maintenance-subcommands/index.md)), so only the
build breaks cycles. Restoring a `demoted` edge takes at most two edits: remove or redirect another
edge of its cycle, then restore it with `restore-demoted`, which refuses a restore that still closes
a cycle.

---


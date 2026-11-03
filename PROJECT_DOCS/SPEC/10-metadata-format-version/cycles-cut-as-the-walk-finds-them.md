### Cycles: cut as the walk finds them

The build's ring row cuts the cycles the build's own edges close, within the stage that writes them,
by one walk; a cycle the records hold at load is a load error
([page §2.4](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)).

- The walk order is the node order of the register pages, which no minted id decides, so a build's
  cuts are reproducible modulo ids ([§5](../5-build-behaviour/index.md)): hosted nodes in the reading order of their markers, then
  every other node by id — a work or an axiom, which no scoring edge enters, so it lies on no
  cycle. Edges
  compare by relation in the relations table's order, then source, then target, each end in that
  node order.
- The walk is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, starting at
  each node and following each node's edges in the walk order; `demoted` records are not walked.
- Each time the walk closes a cycle — an edge reaching a node on the current path — it cuts one edge
  of that cycle: the first, in the walk order, of the cycle's `supports` edges into an analysis,
  where one stands; else the edge the walk is on. The cut edge becomes its `demoted` record, with a
  warning naming the edge and the cycle it broke. Nothing else is dropped.
- After each cut the walk starts over, until no cycle remains, so the cuts depend on the edge set and
  the documents' order alone.

Every public op that adds a scoring edge refuses one that would close a cycle ([§8](../8-maintenance-subcommands/index.md)), so only the
build reaches the walk. Restoring a demoted edge takes at most two edits: remove or redirect another
edge of its cycle, then restore it with `restore-demoted`, which refuses a restore that still closes
a cycle.

---


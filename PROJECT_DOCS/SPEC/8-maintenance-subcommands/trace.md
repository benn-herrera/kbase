### `trace`

`trace` follows the scoring relations transitively from one node. `in` lists everything that bears on the node's strength; `out` lists everything the node's strength reaches. It never follows `references` or `demoted` records. `edges` gives one hop; `trace` gives the full traversal.

**Arguments**:
- `<id>`: required.
- `--direction`: `in` or `out`, required.
- `--sort`: `depth` (the default, reading outward from the node) or `contribution`. `--order`: `ascending` (the default) or `descending`. `--limit`: allowed only beside an explicit `--sort` ([§7](../7-tool-result-contract-and-exit-codes/bounded-results.md)).

**Rows**: one per edge reached — an `edges` row, then `depth` (its distance from the node), `strength` (the source's strength; for a work, an experiment or an axiom, its rigor) and `contribution`. The answer is complete, sorted and limited as [§7](../7-tool-result-contract-and-exit-codes/bounded-results.md) states: `count` and `results`.

- An edge reached by more than one path is one row, at its least depth.
- `contribution` is the edge's input to its target's strength as strength propagation computes it. It is `*pending*` where the edge's bearing or its source's strength is pending, and null where the edge takes no part, as a planned experiment's does.
- `--sort contribution` orders every relation together by `contribution`. On a `refutes` edge the contribution is its cap, 1.0 minus the refutation's strength, so a refutation that holds its target down hard sorts low and one that barely limits it sorts high. Ascending puts the inputs that hold their targets lowest first; descending, the ones that leave them highest. Each row names its relation.
- `--order` turns the named key only: ties break by ascending depth, then `edges`' order.
- `strength` and `contribution` read at two decimals, as `find`'s `strength` does; the sort reads the full values.
- A refusal is `check: usage` at the flag: `--direction` missing or outside `in` and `out`, with them as `allowed` (over MCP the schema requires `direction`, so a call missing it is refused as the `query` tool refuses any missing argument, [§11](../11-the-mcp-server/tools.md)); `--sort` and `--order` outside their vocabularies, `--limit` beside no `--sort`, and `--limit 0`, as `find`'s. An `<id>` that names no node is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([Idempotence](write-batches/idempotence.md)).


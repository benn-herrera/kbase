### Edge direction and authoring

Every edge is stored as its record says, the scoring edges push:
a `supports` or `founds` record runs from the premise into the claim or analysis it supports, and a
`lifts` or `refutes` record from the lifter or refuter. `add-edge` ([§8](../8-maintenance-subcommands/edge-ops.md)) writes one edge as its record runs, over
[page §1.2](../../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)'s legal pairings. An analysis lifts or refutes and never
supplies a premise, so an `add-edge` naming one as the source of a `supports` or `founds` edge is
refused as a pairing no relation joins, as the load refuses such a record. `edges` presents the edges at a node
from the reader's side, by `--direction`: `in` where the node is the target, `out` where it is the
source ([§8](../8-maintenance-subcommands/edges.md)); `trace` follows them transitively.


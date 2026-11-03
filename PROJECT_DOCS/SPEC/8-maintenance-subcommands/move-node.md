### `move-node`

`move-node` is [page §3.1](../../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/move-node.md)'s marker move: its keys, its result's `id`, `unchanged` where the
marker already ends the named line, and its refusals, the named line's being the add ops' ([§8](add-kind.md)).
The marker goes where [page §1.4](../../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) places it, and each citation of the node is re-pointed to its
new host and section anchor ([page §1.5](../../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md)). What this release adds to the page and holds:

- A work's or an axiom's `id` is refused at `id`; an `id` no node carries is refused as an edit's
  is ([§8](edit-kind.md)); a node with no statement marker is refused `check: statement-marker` at
  `id`.
- Every other occurrence of the node's marker is taken from its line, and a line it leaves without
  prose goes with it, so a node holding more than one marker is left with one.
- Every citation of the node outside code, in every document but the summary pages, points at its
  new target after the move, written relative to the citing document, whatever it pointed at
  before; its link text stands.

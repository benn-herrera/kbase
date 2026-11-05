### `show`

`show` returns the record of one node of any kind.

- **Arguments**: `id`, required.
- **Record**: `node_type`, `id`, `title`, `host`, `host_anchor` and `summary`, then the kind's own fields ([§7](../7-tool-result-contract-and-exit-codes/item.md)). `host` is `find`'s.
- `host_anchor` is the anchor of the section holding the node's first statement marker in its host ([§10](../10-metadata-format-version/statements-and-anchors.md)). `host` and `host_anchor` are null for a work, an axiom and a node with no statement marker.
- `summary` is the node's summary-page entry, written `<page>#<slug>`; null where the node has none.
- A pending `rigor` or `strength` reads `*pending*`, as in every query; null is only for a value that does not apply.
- An `id` that names no node is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([Idempotence](write-batches/idempotence.md)).


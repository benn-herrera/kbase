#### `citations-of` (query op)

This lists where prose cites a node, ordered by path, then line. It reads each citation link by
the node id in its link title.

- **Arguments**: `id` (required).
- **Results** rows: `path` (the citing document), `line`, `text` (the link text as written). `move-node` keeps the node's own citations current ([§3.1](../3.1-node-writes/move-node.md)). The only check on a
  citation after it is placed is that its link resolves (`verify`'s dead-link check).
- **Refusals**: an `id` no node carries.

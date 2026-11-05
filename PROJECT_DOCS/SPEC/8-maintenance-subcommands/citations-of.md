### `citations-of`

`citations-of` lists where prose cites a node.

- **Arguments**: `id`, required.
- **Rows**: `path` (the citing document), `line` and `text` (the link text as the document writes it), ordered by path, then line. The answer is complete: `count` and `results`.
- A citation is read by the id in its link title ([§10](../10-metadata-format-version/citations.md)), wherever its link points, outside code, in every document but the summary pages, which the rebuild of the derived views renders.
- An `id` that names no node is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([Idempotence](write-batches/idempotence.md)).


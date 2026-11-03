### `remove-node`

`remove-node` is [page §3.1](../../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md)'s node removal: its key, its result's `id` and `removed-edges`,
and its refusals, checked at the point the entry applies ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)). An `id` no node carries
is refused "no node" ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)), so a re-issued removal is refused. What this release adds to
the page and holds:

- The "no node" item is as an edit's ([§8](edit-kind.md)): `check: unknown-id` at `id`, `detail`
  `no node <id>`.
- An axiom's removal takes every `### Axiom` heading carrying its marker out of `axioms.md`, and
  nothing else of that file.
- A removal that shifts another summary entry's or `axioms.md` heading's repeat suffix — a later
  heading of the same slug taking a lower one — re-points every link into the shifted anchor in the
  same write, as an edit does ([§8](edit-kind.md)). A link into the removed node's own entry or
  heading has no new destination and stands: it is not refused unless it is a citation.
- Each `removed-edges` row is the row `edges` answers for that record ([page §3.4](../../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md)), ordered
  by source, target and context, demoted records included.
- A mention refusal is one item per citing line and per mentioning field, each at `id`. A citation is read outside code, in every
  document but the summary pages, which the derivation renders; it is named by `path` and `line`. A
  prose field mentions the node where it cites it or holds its id bare, and is named by its record
  file as `path` and by record and field in `detail`; its remedy names the `edit-<kind>` of the
  record's kind or `edit-edge`, and for a demoted record's `context` a `restore-demoted` then
  `edit-edge`, or a `remove-edge` with `relation: demoted`.
- A work's span refusal reads each citation span's `data-cites` outside code, in every document
  but the summary pages, one item per span line at `id`, named by `path` and `line`; a
  reference-list entry of the label's key in any such document lets the removal through.

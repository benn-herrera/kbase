### `remove-node`

`remove-node` removes a node of any kind and every record naming it: the node record, its statement marker (or an axiom's heading in `axioms.md`), every edge naming it at either end, and every demoted record naming it. kbase keeps no undo: git restores what a removal took, and a dry run previews it.

- **Keys**: `id`, required.
- **Result**: `id`, and `removed-edges`: every edge record removed, each as the full row `edges` answers ([§8](edges.md)) — `bearing` and `context` included — ordered by source, target and context, demoted records included.
- An axiom's removal takes every `### Axiom` heading that ends in its marker out of `axioms.md`, and nothing else of that file.
- A removal that shifts another summary entry's or `axioms.md` heading's repeat suffix — a later heading of the same slug taking a lower one — updates every link into the shifted anchor in the same write, as an edit does ([§8](edit-kind.md)). A link into the removed node's own entry or heading has no new destination and stays; it is not refused unless it is a citation.

**Refusals**, checked at the point the entry applies:
- an `id` no node has is refused "no node": `check: unknown-id` at `id`, `detail` `no node <id>`; so a repeated removal is refused;
- the id is still mentioned. One item per citing line and per mentioning field, each at `id`:
  - a citation of the node, read outside code in every document but the summary pages, which the rebuild renders, named by `path` and `line`. The remedy is to edit that prose;
  - the node's id standing bare ([§10](../10-metadata-format-version/citations.md)), read outside code in every document but the summary pages, one item per line holding it, named by `path` and `line`. The remedy is to edit that prose;
  - for a work, a citation span whose `data-cites` names its label where no reference-list entry in any such document has that key, one item per span line, named by `path` and `line`. The work record is that span's only bibliography entry, so without it the export prints the span as plain text. The remedy is to edit that prose;
  - a prose field of a record that survives the removal (`title`, `rationale`, an edge's `context`) that cites the node or contains its id bare, named by its record file as `path` and by record and field in `detail`. The remedy names the `edit-<kind>` of the record's kind or `edit-edge`, an entry earlier in the same batch; for a demoted record's `context`, a `restore-demoted` then `edit-edge`, or a `remove-edge` with `relation: demoted`.


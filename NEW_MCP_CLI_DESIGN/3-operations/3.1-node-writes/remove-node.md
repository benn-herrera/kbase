#### `remove-node` (write op)

This removes a node and every record naming it: the node record, its statement marker, every edge
naming it at either end, and every demoted record naming it. It takes `id` only, for every kind. No
undo is built; git restores what a removal took, and a dry run ([§2.2](../../2-conventions/2.2-write-batches.md)) previews it.

- **Keys**: `id` (required).
- **Result**: `id`; `removed-edges`, every edge record removed, each named by `source`, `target`
  and `relation`, as a full edge row (`bearing` and `context` included), so an edge can be re-added
  onto a replacement node without reading `edges` before the removal. A node that does not stand
  is refused ([§2.10](../../2-conventions/2.10-existence.md)).
- **Refusals**: while the id is still mentioned, at the point the entry applies, by:
  - a citation of the node, found by the id in its link title (as `citations-of` finds it), named
    `path:line`. Remedy: edit that prose.
  - for a work, a `data-cites` span carrying its label where no reference-list entry has that key,
    named `path:line`. The work record is that span's only bibliography entry, so without it the
    export prints the span as plain text. Remedy: edit that prose.
  - a prose field of a record that survives the removal (`title`, `rationale`, an edge's
    `context`) holding a citation of the node or its bare id, named by record and field. Remedy:
    an `edit-<kind>` or `edit-edge` entry earlier in the same batch; for a demoted record's
    `context`, `restore-demoted` then `edit-edge`, or `remove-edge` on the demoted record.

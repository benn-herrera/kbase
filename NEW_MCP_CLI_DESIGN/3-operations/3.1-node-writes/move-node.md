#### `move-node` (write op)

This moves a hosted node's statement marker to the end of the named line ([§1.4](../../1-model/1.4-markers.md)), taking it from
where it stood. It is one op for every kind that has a marker. In the same entry it re-points every
citation of the node (found by the node id in each link title, [§1.5](../../1-model/1.5-citations.md)) to the node's new path and
anchor, changing only the link target, never the citing words. The node's citations are links
kbase computed from its host, so re-pointing them is part of the smallest consistent move. A
citation an author's own edit breaks, such as by a heading rename, is a broken link, which
`verify`'s dead-link check reports like any other.

- **Keys**: `id` (required), `document`, `line`, `snippet` (all required).
- **Result**: `id`; `unchanged` where the marker already ends that line.
- **Refusals**: a work or an axiom (no statement marker); a node with no marker to move (a
  `statement-marker` fault, [§1.4](../../1-model/1.4-markers.md)); an `id` no node carries; and the placement refusals of the add
  ops.

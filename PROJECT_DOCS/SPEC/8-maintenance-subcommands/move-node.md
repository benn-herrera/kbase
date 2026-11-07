### `move-node`

`move-node` moves a hosted node's statement marker to the end of the line `document`, `line` and `snippet` name ([§10](../10-metadata-format-version/statements-and-anchors.md)), removing it from its previous line, and updates each citation of the node to its new host and section anchor in the same entry.

- **Keys**, all required: `id`, `document`, `line`, `snippet`.
- **Result**: `id`; `unchanged` where the marker already ends the named line.
- **Refusals**: the named line's refusals are those of the add ops ([§8](add-kind.md)). A work's or an axiom's `id` is refused at `id`, since it has no statement marker; an `id` no node has is refused as an edit's is ([§8](edit-kind.md)); a node with no statement marker is refused `check: statement-marker` at `id`.
- Every other occurrence of the node's marker is taken from its line, and a line it leaves without prose goes with it, so a node with more than one marker is left with one.
- Every citation of the node outside code, in every document but the summary pages, points at its new target after the move, written relative to the citing document, whatever it pointed at before; only the link target changes, its link text stays. A citation an author's own edit breaks, such as by a heading rename, is a broken link, which `verify`'s `dead-link` check reports like any other.


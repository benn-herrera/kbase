#### `edit-edge` (write op)

This changes a standing edge's `bearing`, its `context`, or both.

- **Keys**: `source`, `target`, `relation` (required); `bearing`, `context` (at least one).
- A bearing given on `supports`, `lifts` or `refutes` marks the edge manually scored ([§2.9](../../2-conventions/2.9-manually-scored-and-judged-hash.md)).
- **Result**: `source`, `target`, `relation`; `unchanged` where the values already stand.
- **Refusals**:
  - no edge of that identity ([§2.10](../../2-conventions/2.10-existence.md));
  - `relation: demoted`: remedy `restore-demoted`, or `remove-edge`;
  - a `bearing` on `references`;
  - a bearing out of domain;
  - neither field given: shape.

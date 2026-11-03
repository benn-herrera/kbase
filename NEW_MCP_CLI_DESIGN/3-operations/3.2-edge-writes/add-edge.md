#### `add-edge` (write op)

This adds one edge. Many edges are many entries.

- **Keys**:
  - `source` (node id, required);
  - `target` (node id, required);
  - `relation` (`supports` | `founds` | `lifts` | `refutes` | `references`, required);
  - `bearing` (number in [0,1] | `*pending*`, optional; scoring relations only; default
    `*pending*`);
  - `context` (string, optional).
- Over a standing edge of that identity, the entry is refused, "edge exists", whatever its values.
  Changing them is `edit-edge`'s job ([§2.10](../../2-conventions/2.10-existence.md)).
- Over a demoted record of that relation from source to target, the add writes the edge with its own
  values and deletes the record.
- **Result**: `source`, `target`, `relation`.
- **Refusals**:
  - a `source` or `target` no node carries;
  - a pairing the relation does not join ([§1.2](../../1-model/1.2-edges-and-relations.md)), with `allowed` the relations that join these kinds;
  - `relation: demoted`: remedy `restore-demoted`;
  - a `bearing` on `references`;
  - a bearing out of domain;
  - an edge that would close a cycle (`check: dependency-cycle`, the detail naming the cycle in edge
    direction): remedy, remove another edge of the cycle first.

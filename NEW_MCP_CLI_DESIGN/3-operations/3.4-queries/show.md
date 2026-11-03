#### `show` (query op)

This returns the full record of one node, of any kind.

- **Arguments**: `id` (required).
- **Results**, a mapping:
  - `node_type`, `id`, `title`;
  - `host`: the host leaf's path, the same value as `find`'s `host`, and `host_anchor`, the
    section anchor where the node is stated ([§1.4](../../1-model/1.4-markers.md)). Both null for works, axioms and a node with no
    statement marker. One key holds one shape on every surface.
  - `summary`: its `kb-summary.md` entry as `<page>#<slug>`. Null where it has none.
  - then the kind's own fields:
    - claim: `rigor`, `manually_scored`, `strength`, `build_status`, `strength_band`, `rationale`,
      `premise_count`;
    - analysis: `rigor`, `manually_scored`, `strength`, `strength_band`, `rationale`;
    - experiment: `status`, `rigor`, `manually_scored`;
    - work: `label`, `rigor`, `rationale`;
    - axiom: `label`, `rigor` (`1.0`).
- `strength` is rounded to two decimals; `rigor` is the value the record holds.
- **Refusals**: an `id` no node carries.

#### `trace` (query op)

This follows the scoring relations transitively from one node. `in` gives everything that bears on
the node's strength; `out` gives everything the node's strength reaches. It never walks
`references` or `demoted` records.

- **Arguments**: `id` (required); `direction` (`in` | `out`, required); `sort` (`depth`, the
  default, reading outward from the node; or `contribution`); `order` ([§2.8](../../2-conventions/2.8-results-and-exit-codes.md)); `limit` (only beside
  an explicit `sort`, [§2.8](../../2-conventions/2.8-results-and-exit-codes.md)). `sort: contribution` orders by each input's effect on its target's
  strength, the `contribution` as computed: on `refutes` that is the cap, 1.0 minus the
  refutation's strength, so a refutation that holds its target down hard sorts low and one that
  barely limits it sorts high. Ascending puts the inputs that hold their targets lowest first;
  descending the ones that leave them highest. `trace` is always the full transitive walk; `edges` gives one hop. `contribution`
  is the value the scoring code computes as the edge's input to its target's expression,
  reusing that code, never a restated formula. Every relation sorts together in one ordering;
  each row names its relation.
- **Results**: one row per edge reached, in `edges` row form plus `depth` (its distance from the
  node), the source's `strength` ([§1.1](../../1-model/1.1-nodes.md); for a work, an experiment or an axiom, its rigor) and
  `contribution` (`*pending*` where the edge's bearing or source strength is pending; null where
  the edge takes no part, as a planned experiment's), so the ordering shows. Every row copies into
  an edge op as [§2.1](../../2-conventions/2.1-naming.md) states.
- **Refusals**: an id no node carries; `limit` without `sort`.

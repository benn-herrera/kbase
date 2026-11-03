#### `stats` (query op)

This gives a complete snapshot in one call:
- one count per node kind (`claims`, `analyses`, `experiments`, `axioms`, `works`);
- `edges` and `demoted_edges`;
- `subtree_aggregates`, per volume;
- `rigor_manual`, `rigor_model`, `bearing_manual`, `bearing_model`;
- `relations`: per relation, its row count, whether a score run's bearing pass asks about it, and,
  on a scoring relation, the edges at each bearing;
- `strength_bands`;
- `highest_leverage`: the ten claims below `0.65` with the most dependents;
- `weakest`: the ten lowest-strength claims.

Its help points to `find`'s strength filters for the full lists that the two ten-item lists
preview.

- **Arguments**: none.

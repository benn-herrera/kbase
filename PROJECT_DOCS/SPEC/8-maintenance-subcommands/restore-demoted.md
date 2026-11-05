### `restore-demoted`

`restore-demoted` puts back exactly the edge each demoted record from `source` to `target` replaced, with its relation, context, bearing and `manually-scored` flag, and deletes the record ([§10](../10-metadata-format-version/the-demoted-relation.md)).

- **Keys**, all required: `source` and `target` (the ids of the demoted record's ends; each a claim, an analysis or an experiment) and `relation`, which must be `demoted`. Any other relation is refused at `relation`.
- **Result**: `source`, `target`, `relation`, and `restored`, the relations put back, in the records' order.
- **Refusals**:
  - an entry with no demoted record from `source` to `target` is refused "no edge", as the edge writes are ([§8](edge-ops.md)): no `check`, `key` `relation`, `detail` `no edge demoted <source> → <target>`. So a repeated restore, and a repeat of an entry within one batch, is refused, `entry` the repeat's number. Where a scoring edge from `source` to `target` now exists, the remedy says so and names `edges` to read it; it does not say whether that edge came from a restore, which nothing records;
  - a restore that would close a cycle: `check: dependency-cycle`, `key` `target`, the `detail` naming the cycle in edge direction. The remedy names removing another edge of the cycle and, to leave the edge out of the graph, `remove-edge` with `relation: demoted`, which leaves no record of it.


### `edges`

`edges` lists edge records.

**Arguments**, all optional:
- `--id`: the node. With no node given, every edge is listed.
- `--direction`: `in` (the node is the target), `out` (the node is the source) or `both`, the default. It needs `--id`.
- `--relation`: any of the six relations ([§10](../10-metadata-format-version/four-scoring-relations-one-arithmetic.md)); repeatable.

**Rows**: `source`, `target`, `relation`, `bearing` (as the record holds it; null where the relation has none), `context` (null for none), `manually-scored` (null where the relation has no flag) and `replaced-relation` (null except on a `demoted` row). The flag tells a caller that `score --edge` will need `--rescore-manual`; whether an edge is stale is `score --dry-run`'s answer. The answer is complete, in a fixed order: `count` and `results` ([§7](../7-tool-result-contract-and-exit-codes/bounded-results.md)).

- Rows order by relation (in the order `supports`, `founds`, `lifts`, `refutes`, `references`, `demoted`), then source, then target. Rows that share relation, source and target — demoted records replacing different relations — order by `replaced-relation`, in the relations' order.
- A `demoted` row's `bearing` and `manually-scored` are the ones its record keeps of the replaced edge ([§10](../10-metadata-format-version/the-demoted-relation.md)).
- A usage refusal is `check: usage` at the flag: `--relation` and `--direction` outside their vocabularies, each with it as `allowed`, an unknown relation once however often it repeats; and `--direction` beside no `--id`.
- An `--id` that names no node is refused `check: unknown-id` at `--id`, `detail` `no node <id>` ([Idempotence](write-batches/idempotence.md)).


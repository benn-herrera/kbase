#### `edges` (query op)

This lists edge records, ordered by relation, source and target.

- **Arguments**, all optional:
  - `id`, the node, with `direction` (`in` = the node is the target, `out` = it is the source,
    `both`, the default). The node is the query's subject, so `id` names it ([§2.1](../../2-conventions/2.1-naming.md)); `source` and `target`
    name the ends in the rows;
  - `relation` (repeatable; any of the six).

  With no node given, every edge is listed.
- **Results** rows: `source`, `target`, `relation`, `bearing` (as the record holds it; null where the
  relation carries none), `context` (null for none), `manually_scored` (null where the relation
  carries no mark), and `replaced_relation` (null except on a `demoted` row). The mark tells a
  caller that `score --edge` will need `--rescore-manual`; whether an edge is dirty is
  `score --dry-run`'s answer.
- A node's dependents ([§1.2](../../1-model/1.2-edges-and-relations.md)) are the targets of `edges` with that node, `direction: out` and the
  four scoring relations.
- **Refusals**: `direction` without a node (usage); an unknown
  relation.

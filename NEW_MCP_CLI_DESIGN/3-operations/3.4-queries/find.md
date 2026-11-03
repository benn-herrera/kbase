#### `find` (query op)

This lists the nodes matching its filters, across every kind. With no filter it lists every node.
`find` locates nodes; `show` reads one.

- **Arguments**, all optional:
  - `kind` (`claim` | `analysis` | `experiment` | `work` | `axiom`, repeatable);
  - `title` (text, matched case-insensitively as a substring);
  - `label` (text, matched case-insensitively as a substring, as `title` is; only works and axioms
    carry one);
  - `path`: a kb-root-relative directory, matching every node hosted under it, or a leaf, matching
    the nodes it hosts. `""` and `.` name `kb-root/`, matching every hosted node. Works and axioms
    never match it.
  - `min` (strength at or above);
  - `max` (strength strictly below);
  - `pending` (nodes with no strength yet; refused beside `min` or `max`);
  - `min-dependents` (integer);
  - `sort`: `id` (default), `strength` or `dependents`, ties broken by strength, then id; `order`
    ([§2.8](../../2-conventions/2.8-results-and-exit-codes.md));
  - `limit` (only beside an explicit `sort`, [§2.8](../../2-conventions/2.8-results-and-exit-codes.md)).
- `strength` is [§1.1](../../1-model/1.1-nodes.md)'s for every kind: derived for a claim or an analysis, the rigor for a work, an
  experiment or an axiom. `min`, `max` and `pending` apply to every kind. `min` and `max` compare
  scored strengths only, so a pending node never matches either, and `min: 0` keeps exactly the
  scored nodes.
- **Results** rows: `id`, `node_type`, `title`, `label` (null for hosted kinds), `host` (the host leaf's path; null for works, axioms and
  a node with no statement marker), `strength` (as above, `*pending*` where pending), and
  `dependents` ([§1.2](../../1-model/1.2-edges-and-relations.md)).
  `find` sorts and filters by strength and dependents, so its rows show both; that also makes `find` the full list behind
  `stats`' `highest_leverage`.
- **Refusals**: `pending` beside `min` or `max` (usage); a value outside a vocabulary.

### Edge ops

`add-edge`, `edit-edge` and `remove-edge` each write one edge; many edges are many entries. An edge is named by `source`, `target` and `relation`, over the relations and legal pairings of [§10](../10-metadata-format-version/four-scoring-relations-one-arithmetic.md); a `demoted` record is defined in [§10](../10-metadata-format-version/the-demoted-relation.md).

| Op | Keys |
|---|---|
| `add-edge` | `source`, `target` (node ids) and `relation` (`supports`, `founds`, `lifts`, `refutes` or `references`), required; `bearing` (a number in `[0, 1]` or `*pending*`; scoring relations only; default `*pending*`) and `context` (a string, one line, `""` for none), optional |
| `edit-edge` | `source`, `target` and `relation`, required; `bearing` and `context`, at least one |
| `remove-edge` | `source`, `target` and `relation` (any of the six, `demoted` included), required |

**Results**: `source`, `target`, `relation` for each.

**`add-edge`**
- Over an edge of that identity the entry is `unchanged` where its `bearing` and `context`, as the add would write them, are the edge's own, and the edge's flag and judged hash are those the add would write ([§8](write-batches/idempotence.md)); otherwise it is refused "edge exists", and `edit-edge` changes the values. Over a demoted record of that relation from `source` to `target`, the add writes the edge with its own values and deletes the record.
- The `manually-scored` flag and the `judged-hash` an edge write sets are defined in [§10](../10-metadata-format-version/provenance-and-the-judged-hash.md). A `bearing` given on a `supports`, `lifts` or `refutes` edge hashes the edge's two ends' statements as the batch leaves them.

**`edit-edge`** changes an existing edge's `bearing`, `context`, or both. `unchanged` where it changes no value, no flag and no judged hash. A `bearing` given on a `supports`, `lifts` or `refutes` edge hashes the edge's two ends' statements as `add-edge` does; an edit giving none keeps the record's `judged-hash`.

**`remove-edge`** removes the one edge record the entry names. With `relation: demoted` it removes every demoted record from `source` to `target`, leaving the edge out of the graph. Every other record stays byte for byte as it was.

**Refusals**:
- "edge exists" (`add-edge`) and "no edge" (`edit-edge`, `remove-edge`, where no record of that identity exists) are items with no `check` and `key` `relation`; their `detail` is `edge exists: <relation> <source> → <target>` and `no edge <relation> <source> → <target>`, an "edge exists" remedy naming `edit-edge` and a "no edge" remedy naming `edges`;
- a `source` or `target` no node has: on `add-edge` and `remove-edge`, `check: unknown-id` at that key, `detail` `no node <id>`; on `edit-edge` the entry is refused "no edge";
- a pairing the relation does not join: at `relation`. On `add-edge` its `allowed` is the relations joining the two kinds, in the relations table's order, `references` always among them. `remove-edge` refuses it for a scoring relation only;
- `relation: demoted` on `add-edge` or `edit-edge`: a shape refusal at `relation`, `allowed` the five other relations, its remedy naming `restore-demoted` (and, on `edit-edge`, `remove-edge`);
- a `bearing` on `references`: at `bearing`. An `edit-edge` entry giving neither `bearing` nor `context`: a shape refusal whose `allowed` is `bearing` and `context`. A `bearing` outside its domain: at `bearing`;
- a cycle among the scoring edges that the edge would close: `check: dependency-cycle`, `key` `target`, the `detail` naming the cycle in edge direction; the remedy is to remove another edge of the cycle first.


### Edge ops

`add-edge`, `edit-edge` and `remove-edge` are [page §3.2](../../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/index.md)'s edge writes: each op's keys,
result and refusals, over [page §1.2](../../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md)'s relations and legal pairings and [page §1.3](../../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s demoted records.
An add of an edge that stands is refused "edge exists", and a removal or an edit of one no record
holds is refused "no edge" ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)); the mark a bearing writes is [page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)'s, and
`context` takes [page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s "Values" check. What this release adds to the page and holds:

- "edge exists" and "no edge" items carry no `check` and `key` `relation`
  ([§7](../7-tool-result-contract-and-exit-codes/item.md)); their `detail` is
  `edge exists: <relation> <source> → <target>` and `no edge <relation> <source> → <target>`, an
  "edge exists" remedy naming `edit-edge` and a "no edge" remedy naming `edges`.
- A `source` or `target` no node carries is refused `check: unknown-id` at that key, `detail`
  `no node <id>`, as an edit's `id` is ([§8](edit-kind.md)).
- A pairing the relation does not join is refused at `relation`; on `add-edge` its `allowed` is the
  relations joining the two kinds, in the relations table's order, `references` always among them.
- `relation: demoted` on `add-edge` or `edit-edge` is a shape refusal at `relation`, `allowed` the
  five other relations, its remedy naming `restore-demoted` (and, on `edit-edge`, `remove-edge`).
- A bearing on `references` is refused at `bearing`. An `edit-edge` entry giving neither `bearing`
  nor `context` is a shape refusal whose `allowed` is `bearing` and `context`.
- A cycle refusal names `target` as its key. A `bearing` given on a `supports`, `lifts` or `refutes`
  edge, `*pending*` included, records the judged hash of its two ends' statements as the batch
  leaves them ([page §2.9](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)); an added edge given none writes the never-judged one, all zeros, and
  an edit giving none keeps the record's `judged-hash` as it stands.

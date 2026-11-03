### `edges`

`edges` is [page §3.4](../../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/edges.md)'s query: its node `--id` with `--direction`, its repeatable `--relation`,
its rows and their fixed order, and its refusals. Its answer is complete as [page §2.8](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s "Lists"
states: `count` and `results`. What this release adds to the page and holds:

- Rows that share relation, source and target — demoted records replacing different relations —
  order by `replaced_relation`, in the relations' order.
- A `demoted` row's `bearing` and `manually_scored` are the ones its record keeps of the replaced
  edge ([page §1.3](../../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)).
- A usage refusal is `check: usage` at the flag: `--relation` and `--direction` outside their
  vocabularies, each with it as `allowed`, an unknown relation once however often it repeats; and
  `--direction` beside no `--id`.
- An `--id` no node carries is refused `check: unknown-id` at `--id`, `detail` `no node <id>`
  ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).

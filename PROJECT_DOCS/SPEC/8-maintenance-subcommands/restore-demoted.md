### `restore-demoted`

`restore-demoted` is [page §3.2](../../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/restore-demoted.md)'s restore of [page §1.3](../../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md)'s demoted records: its keys, its result's
`restored`, and its refusals. An entry no demoted record from `source` to `target` answers is refused
"no edge" ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)), so a re-issued restore, and a repeat of an entry within one batch, is
refused, `entry` the repeat's number. The derivation follows it as it follows every batch
([page §2.3](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)). What this release adds to the page and holds:

- The "no edge" item is as the edge writes' ([§8](edge-ops.md)): no `check`, `key` `relation`, `detail`
  `no edge demoted <source> → <target>`.
- The cycle refusal names `target` as its key, and its remedy names, beside removing another edge of
  the cycle, `remove-edge` with `relation: demoted`, which leaves the edge out of
  the graph, with no record of it.
- `restored` lists the replaced relations in the records' order.

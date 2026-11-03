### `show`

`show` is [page §3.4](../../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/show.md)'s query: its record of one node of any kind — `host`, `host_anchor` and
`summary`, then its kind's own fields — and its refusal. What this release adds to the page and
holds:

- `host_anchor` is the anchor of the section holding the node's first statement marker in its host.
- A pending `rigor` or `strength` reads `*pending*`, as in every query; null is only for a value
  that does not apply.
- An `<id>` no node carries is refused `check: unknown-id` at `<id>`, `detail` `no node <id>` ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).

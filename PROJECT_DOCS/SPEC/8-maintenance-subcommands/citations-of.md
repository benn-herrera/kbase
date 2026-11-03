### `citations-of`

`citations-of` is [page §3.3](../../../NEW_MCP_CLI_DESIGN/3-operations/3.3-citations/citations-of.md)'s query: its `id`, its rows `path`, `line` and `text` ordered by
path, then line, and its refusal. Its answer is complete ([page §2.8](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)): `count` and `results`. What this release adds to the page and holds:

- A citation is read by the id in its link title, wherever its link points, outside code, in every
  document but the summary pages, which the derivation renders; `text` is its link text as the
  document writes it.
- An `id` no node carries is refused `check: unknown-id` at `<id>`, `detail` `no node <id>`.

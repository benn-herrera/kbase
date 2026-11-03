### `trace`

`trace` is [page §3.4](../../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/trace.md)'s query: its required `<id>` and `--direction`, its full walk over the
scoring relations, its rows — an `edges` row, then `depth`, `strength` and `contribution` — and its
refusals. Its answer is complete, sorted and limited as [page §2.8](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s "Lists" and "Sorting" state:
`count` and `results`. What this release adds to the page and holds:

- An edge reached by more than one path is one row, at its least depth.
- `--order` turns the named key only: ties break by ascending depth, then `edges`' order.
- `strength` and `contribution` read at two decimals, as `find`'s `strength` does; the sort reads
  the full values.
- A refusal is `check: usage` at the flag: `--direction` missing or outside `in` and `out`, with
  them as `allowed` (over MCP the schema requires `direction`, so a call missing it is refused
  as the `query` tool refuses any missing argument, [§11](../11-the-mcp-server.md)); `--sort` and `--order` outside their vocabularies, `--limit` beside no
  `--sort`, and `--limit 0`, as `find`'s. An `<id>` no node carries is refused `check: unknown-id`
  at `<id>`, `detail` `no node <id>` ([page §2.10](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)).

### `find`

`find` is [page §3.4](../../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/find.md)'s query: its filters over every kind, `label` among them, its rows `id`,
`node_type`, `title`, `label`, `host`, `strength` and `dependents`, and its refusals. Its answer is complete, sorted and limited as
[page §2.8](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.8-results-and-exit-codes.md)'s "Lists" and "Sorting" state: `count` and `results`. What this
release adds to the page and holds:

- A row's `strength` reads the derivation, so a bearing that moves a claim moves the list. Its
  `dependents` and `show`'s `premise_count` are structural — counts of edges, which no bearing
  changes.
- `--order` turns the named key only: ties under `--sort strength` or `--sort dependents` are
  broken by ascending strength, then ascending id, in either order.
- A refusal is `check: usage` at the flag: `--kind`, `--sort` and `--order` outside their
  vocabularies, each with it as `allowed`; `--pending` beside `--min` or `--max`; `--limit` beside no
  `--sort`, and `--limit 0`.

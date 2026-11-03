### `stats`

`stats` is [page §3.4](../../../NEW_MCP_CLI_DESIGN/3-operations/3.4-queries/stats.md)'s query, its keys as [§7](../7-tool-result-contract-and-exit-codes/item.md) lists them. Its help names the `find` question each
ranked section previews: `highest_leverage` is the first ten of `find --kind claim --max 0.65
--min-dependents 1 --sort dependents --order descending`, and `weakest` of `find --kind claim
--min 0 --sort strength`. What this release adds to the page and holds: `subtree_aggregates` is
one record per volume, keyed by the volume's directory in path order, each `claims` and
`experiments`, the counts of those whose statement marker stands in a leaf under it; a KB with no
volume has `{}`.

`stats` reads the derivation in `strength_bands`, `highest_leverage` and `weakest`, and is
structural in its per-kind counts, `edges`, `demoted_edges` and the edge counts of `relations`.

`stats`' `relations` and the `edges` query read the index's edge rows and no derivation: the
bearings they report are the values the rows hold, which `edit-edge` moves. `relations` reports
`walked` per relation: whether a score run's bearing pass asks about that relation's edges —
`supports`, `lifts` and `refutes` — where a `founds` bearing is the user's, written by
`add-edge` or `edit-edge`.

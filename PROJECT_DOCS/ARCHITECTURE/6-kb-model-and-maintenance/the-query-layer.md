### The query layer

**Pattern:** Read model.

`query.Load` is the rows `index.Derive` yields, as an `Index` sorted by id; `Derive`'s refusals are
`Load`'s, and a KB at an older format answers as converted. Queries read the `Index` and nothing else.
`query.ScoringSet` builds the scoring set ([§6](scoring/the-scoring-set.md)).

`query.EdgesPayload` is the single incidence adapter. It selects the `Index`'s edge rows by relation and,
where `EdgesSelection` names a node, by `Direction` (`in` the rows whose target it is, `out` those whose
source it is, else both), and presents each as `query.EdgeRowRecord`: `source`, `target`, `relation`,
`bearing` (a number or the pending literal, null only where the pairing has none), `context`,
`manually_scored` and `replaced_relation`. The order is fixed: relation, then source and target
(SPEC §8 `edges`).

An answer is the whole selection: no list query slices it, and `find` and `trace` truncate only where
`--limit` is given beside `--sort` (`rankUsage` refuses a limit without a sort). No query verb returns
statement text: the rows contain none.

`contribution` in a `trace` row is `index.EdgeContribution`, never a restated formula (`tracedEdge`).

- **`find`.** `FindUsage` checks the selection's vocabulary values and `rankUsage` its sort; `Find`
  then filters the nodes, orders them by the sort key and cuts to the limit. `compareRanks` orders a
  pending value below every number (above every number when descending) and a null after every
  other value in either direction; ties break by id, and for `--sort dependents` by strength first.
- **`show`.** `ShowPayload` takes a node's host and section anchor from the host leaf's marker, read
  once per leaf at `Load` (`Rows.Hosts`, `Rows.HostAnchors`), and its summary entry from
  `Rows.RegisterPages` and `Rows.RegisterAnchors`.
- **`stats`.** Its ranked sections, `highest_leverage` and `weakest`, are `find` questions
  (`HighestLeverage`, `Weakest`), each cut to `query.StatsRankLimit` ([§12](../12-constants/index.md)).


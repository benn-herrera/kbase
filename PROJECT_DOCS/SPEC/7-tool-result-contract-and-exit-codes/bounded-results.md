### Bounded results

`find`, `edges`, `trace` and `citations-of` return their complete answers, with `count` and
`results`: there is no default cut, no paging and no `offset`. A smaller answer comes from a
narrower question, a filter that means something. `edges` and `citations-of` return a fixed order,
stated with each. `find` and `trace` take `sort` and `limit`.

- `limit N`, a single integer, is allowed only beside an explicit `sort`, and returns the first N of
  that order: the N weakest, the N strongest. A negative N returns the last N, still in the sort's
  order, as a Python slice does. `limit` without `sort` is refused (`check: usage`), and so is
  `limit: 0`.
- `sort` names a key and `order` (`ascending` | `descending`) sets the direction; every key
  defaults to `ascending`, and every key is available both ways, so `limit` can take either end of
  any ranking. A pending value sorts below every number, for ordering only; its row still shows
  `*pending*`. A null, a value that does not apply to the row, sorts last in either order. Each
  query states its own sort keys.

`stats`' ranked sections stay at 10, and the `bearings` of its `relations` contain
one record per distinct bearing value. No other subcommand's result grows with the KB; `build`'s
per-stage detail goes to the `report` file, not stdout.

**Exit codes** derive from `outcome`:

| Outcome | Exit | Meaning |
|---|---|---|
| `done`, `unchanged`, `bounded` | 0 | Success, including a `--through`-bounded run |
| `refused` (including verify faults and usage errors) | 1 | Wrong input or KB state; nothing written past the last checkpoint |
| `retry` | 2 | `check: lock` where another writer holds the KB write lock past the wait (`path` the repository root); a run holds the run lock but has not yet recorded its state dir; another repository's run holds a state store shared through `--state-dir`; or `cancel` finds the run not yet recorded, or the run does not let go within the wait ([§6](../6-build-state-ledger-and-monitoring/index.md)); retry identically later. A run that holds the run lock and has recorded its state dir is not `retry`: it is `refused` with the `lock` item ([§6](../6-build-state-ledger-and-monitoring/index.md)) |
| `failed` | 3 | Defect in the tool, its input or the environment; retrying will not fix it |
| `cancelled` | 4 | Stopped on request; resumable |


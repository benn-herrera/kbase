### `kbase cancel`

`cancel`'s argument, result and refusal are [page §3.5 `cancel`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/cancel.md)'s: it signals the run that
holds the state store, of either kind — a build or a scoring run, as the holder record names it —
and the run continues only when asked to, by `kbase build --resume` or `kbase score --resume`.

What the page leaves to this document: the event is logged, the lock is released, and the run
process exits `cancelled`. Cancel costs only the calls in flight: an answer already returned stands
— in a build's answer cache, and in a score run's ledger; a cancelled score run's `resume` is the
call read from its result document. A store no run holds is `refused`: `check: lock`, `path` the
store's holder lock, detail "no run holds the state store".

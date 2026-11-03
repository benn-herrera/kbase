#### Outcomes

`done` when the run finished; `cancelled` when SIGINT or SIGTERM stopped it, with no line recorded
for an item whose call was in flight or not yet made, and a line for every item answered before
the stop; `failed` when an item's call or write failed, or the start refresh did; `retry` while
another run holds the state store, or where the start refresh could not take the KB write lock past
the wait; `refused` where the run cannot go on — [page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s refusals, and besides them
a `--state-dir` inside `kb-root/`, `--node` beside `--edge`, an `--edge` that names no edge, and an
index the load cannot answer.

#### The closing record

A `failed` run closes on `failures`. Where the walk itself failed, it holds one
`check: error` item per item the ledger records `failed`, `detail` the item as the document names it
and the reason its line recorded — `<node id>: <reason>`, or `<relation> <source> → <target>:
<reason>` — in the walk's order, with the `remedy` the line records where it records one and the
line's `note`, where it records one, ending the `detail` before the outcome's ask ([§7](../../7-tool-result-contract-and-exit-codes/index.md)); the reason is the line's reply as a
bounded, control-stripped summary, the ledger's line being the one place a reply stands whole; a run the start refresh stopped closes
on one `check: error` item naming the refresh's own error. A `cancelled` run closes on `cancelled`,
whose `item` is the walk's first item carrying no attempt, and `{}` where
every item of the walk carries one. A `refused` or `retry` run closes on its `refusals`.

**`resume`** carries the call that continues the run — `score` with `--resume` — present wherever
an item failed or was not reached; a `skipped` item is no such item. It stands after `counts` and
`scored` and before the closing record. It is absent from a dry run and from a stop before the
walk: the start refresh's, the ledger's or the index load's. It names no argument of the scope —
the run's `run.yaml` records it — and carries `--state-dir`, `--config-dir` wherever the
invocation named one — the resumed run must read that directory and no other, or the same run
could resolve another reference — `--project-dir` wherever the invocation named one, and
`--scoring-model` whenever the invocation did — the call must reach the same model, or it is a
different judgement under the same run id. A run started under an earlier release whose header
reads `backend: environment` is resumed on whatever the files now resolve, and its new lines
record that.

**`--dry-run`** is [page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s. It writes `run.yaml` and `items.jsonl` (one pending line
per item of its set — the items it would ask and the dirty manually scored ones it would skip),
dials no provider, resolves no role, and writes no KB file. Its document lists `ask`, every item
the run would ask, and `skip`, every dirty manually scored item it would skip, each as `scored`
names an item and in the walk's order, after `counts`: `total` the two together, `remaining` the
items to ask, `skipped` the items to skip, `done` `0`. Its `scored` is `[]`, it carries no `resume`
and no `roles`, and its outcome is `done`. It writes no charge record and no capture. Its run id
is `dry-<id>`, and the runs kept are counted by kind ([§6](../../6-build-state-ledger-and-monitoring/index.md)), so a dry run counts only against dry
runs. `--resume` with `--dry-run` is a usage refusal naming `--resume`. A dry run on an older KB is
refused as a real run is (`check: index`, remedy `kbase update-format`).

Result keys: [§7](../../7-tool-result-contract-and-exit-codes/index.md).

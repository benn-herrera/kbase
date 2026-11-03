#### Skipped items

A value whose record is `manually-scored: true` ([page §2.9](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md)) is a person's judgement standing.
What `--rescore-manual` reaches is [page §3.5 `score`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/score.md)'s; the mark is read from the KB the run loaded
at its start. A full run without it walks a dirty manually scored item and skips it: the item is
asked nothing and lands nothing, its ledger line records status `skipped`, reason
`manually-scored`, with no value, charges or call, and so no capture. It counts under
`counts.skipped`, never under `scored` or `failures`, and does not make `resume` present. A value
`--rescore-manual` re-judges lands `false`, so later runs re-judge it too. `run.yaml` records the
flag (`rescore_manual: true`), and `--resume` continues the run with it, appending no second
`skipped` line for an item; a re-price leaves a skipped item's landed line as it is. A `--dry-run`
lists the dirty manually scored values it would skip under `skip`, counted under
`counts.skipped`.

An item whose node or edge is no longer in the KB is skipped too, once, counted the same way: its
line records status `skipped`, reason "the node or edge is no longer in the KB; its value is
discarded and a resume does not ask it again", and nothing is asked of it or landed on it.

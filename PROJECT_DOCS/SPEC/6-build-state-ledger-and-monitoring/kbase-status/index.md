### `kbase status`

`status`' argument, its build result and its score result, and its refusals are [page §3.5
`status`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/status.md)'s: it reports the current or last run, a build or a score run, whichever is newest.
A build's `stages` is a mapping: `done` and `not-done`, the stage ids each way in walk order, and
`rewind`, the one line saying that each completed stage's commit, with the command that rewinds
to it, is in `kbase build-trail` ([page §3.5 `build-trail`](../../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)). The stages read are the newest build's
trail ([§5 Starting over](../../5-build-behaviour/starting-over.md)). Where `kb-root/` is absent, or the newest change to
`kb-root/` removed it and no stage boundary follows, and no build run holds the state store,
`state` is `none`, no stage is done, and `current`, `pid`, `started`, `updated`, `ended` and
`resume` are null. `recent-refusals`, `recent-fallbacks`, `cache-entries` and `cache-bytes` still
read the state store: they record earlier invocations, not a standing build.

What the page leaves to this document: a build's `resume` is absent where no identical re-issue
continues the work ([§7](../../7-tool-result-contract-and-exit-codes/index.md)) and absent where the recorded command does not read as a `kbase build`
invocation — at least two elements, `kbase` then `build`. A run whose record fails that supplies
none, as a run that recorded none does. `state` and `resume` are read from the newest run the build
stands at: a refusal or `retry` that recorded no boundary wrote nothing, so the run before it is the
one reported. `interrupted` is the one state the build never writes: it is derived from a run event
with no end event and a holder that is not live.
Progress is persisted to a file so a late or reconnecting monitor catches up. An inference stage's
units and fallbacks are recorded once its asks have all returned, so while they are in flight
`current` names the stage and its total with none done.

Contents:

- [§6 A score run's status](a-score-run-s-status.md)

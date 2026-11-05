### `kbase status`

`status` reports the current or last run, a build or a score run, whichever is newest. Its one
argument is `state-dir`; its keys are in [§7](../../7-tool-result-contract-and-exit-codes/item.md). It is refused outside a git worktree
(`check: worktree`), with a `--state-dir` inside `kb-root/` (`check: usage`, `key` `--state-dir`)
and over a store newer than this kbase (`check: state-dir`, [above](../the-state-store-is-versioned.md)).

A build's `state` is one of:

- `none`: no run is recorded in the store; or `kb-root/` is absent, or the newest change to it
  removed it and no stage boundary follows, and no run holds the run lock;
- `running`: a run holds the run lock;
- `finished`: the newest run ended `done` or `unchanged`;
- `bounded`: it ended `bounded`;
- `cancelled`: it ended `cancelled`;
- `failed`: it ended `failed`, `refused` or `retry`;
- `interrupted`: it recorded a start but no end, and no run holds the run lock. It is the one state
  the build never writes.

A build's `stages` is a mapping: `done` and `not-done`, the stage ids each way in stage order, and
`rewind`, the one line saying that each completed stage's commit, with the command that rewinds
to it, is in `kbase build-trail`. The stages read are the newest build's checkpoint commits
([§5](../../5-build-behaviour/starting-over.md)). Where `state` is `none` because `kb-root/` is absent or was
removed, no stage is done, and `current`, `pid`, `started`, `updated`, `ended` and `resume` are
null. `recent-refusals`, `recent-fallbacks`, `cache-entries` and `cache-bytes` still read the state
store: they record earlier invocations, not a current build.

A build's `resume` is null where no identical retry continues the work ([§7](../../7-tool-result-contract-and-exit-codes/index.md)) and where the
recorded command does not parse as a `kbase build` invocation — at least two elements, `kbase` then
`build`. A run whose record fails that supplies none, as a run that recorded none does. `state` and
`resume` come from the build's newest run that counts: a refusal or `retry` that recorded no
boundary wrote nothing, so the run before it is the one reported.
Progress is persisted to a file so a late or reconnecting monitor catches up. An inference stage's
units and fallbacks are recorded once its model calls have all returned, so while they are in
flight `current` names the stage and its total with none done.

Contents:

- [§6 A score run's status](a-score-run-s-status.md)

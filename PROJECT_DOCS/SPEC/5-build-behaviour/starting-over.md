### Starting over

`build`'s arguments, results and refusals, the fresh build over an existing `kb-root/` and a
`resume` with nothing to resume among them, are [page §3.5 `build`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build.md)'s; a fresh build over any `kb-root/`
is refused with `init`'s refusal ([page §3.7 `init`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.7-setup-and-configuration/init.md)), and the way to start over is to remove
`kb-root/`. `build-trail` ([page §3.5 `build-trail`](../../../NEW_MCP_CLI_DESIGN/3-operations/3.5-runs-and-run-control/build-trail.md)) lists the commands that rewind: a boundary's is
`git reset --keep <commit>` then the build resumed with the flags its last run was launched with,
the bound excepted, and `started-at`'s the reset alone; kbase runs none of them.

What the page leaves to this document:
- A fresh build over a trail a removed `kb-root/` left behind starts a trail of its own: a resume
  continues the newest build's trail, read back to its `start` boundary, and `status` and
  `build-trail` read that trail alone.
- `status`, `build-trail` and a resume read no trail a later removal of `kb-root/` ends: the trail
  they read lies above the newest commit that removed `kb-root/` and is no `kb-build:` boundary.
  With `kb-root/` absent, or the trail so ended, and no build run holding the state store,
  `status` reports `none` with no stage done and no `current`, and `build-trail` no boundary and
  `started-at` null; a live run is reported as it stands. A build that ends with `kb-root/` absent
  hands out no `resume` in its result. A reset to before the removal restores `kb-root/` and the
  old trail with it.
- A `resume` with nothing to resume — an empty trail, one whose newest boundary records no
  inputs, or a `kb-root/` that does not exist — is one item, `key` `--resume`, `path` the
  repository root: while `kb-root/` stands, `check: kb-root`, remedy "remove kb-root/, then
  run a fresh build, to start over"; otherwise `check: usage`, `detail` "kb-root/ does not
  exist: there is no build to resume", remedy `kbase build <volume-root>...`, whatever the trail
  records.
- A fresh build where `kb-root/` is absent counts no deleted path under it as dirt, committed or
  not, and its first boundary commit records the removal; any other dirty path it owns is refused
  (`check: dirty-paths`), remedy "commit the change or restore the path to the last commit, then
  re-run".
- A trail that records a stage while leaving an earlier stage in walk order unrecorded was written
  by a build walking another stage order, and a build over it is refused before anything is
  restored or written: one item, `check: trail`, `path` the repository root, `detail` naming the
  recorded stage and the unrecorded one before it, remedy "start over from a commit before the
  trail". `build-trail` refuses it the same way.
- `kbase build --resume` over a finished trail is `unchanged`.

### Starting over

To start over, remove `kb-root/`, then run a fresh build. kbase does not inspect half-built state.
`build-trail` ([§6](../6-build-state-ledger-and-monitoring/kbase-build-trail.md)) lists the commands that rewind to a checkpoint commit;
kbase runs none of them.

**`build`'s refusals.** Each is outcome `refused`:

- Outside a git worktree: `check: worktree`, remedy `git init`.
- A fresh build where `kb-root/` exists in any state, an empty or scaffold-only one and one containing
  an interrupted build's records included: `check: kb-root`, `path` the `kb-root/`, no remedy. It
  is the refusal of `init` over an existing `kb-root/` ([§8](../8-maintenance-subcommands/init.md)), and the one refusal of a
  fresh build over existing ground.
- A `--resume` with nothing to resume — an empty set of checkpoint commits, one whose newest
  boundary records no inputs, or a `kb-root/` that does not exist — is one item, `key` `--resume`,
  `path` the repository root. While `kb-root/` exists: `check: kb-root`, remedy "remove kb-root/,
  then run a fresh build, to start over". Otherwise: `check: usage`, `detail` "kb-root/ does not
  exist: there is no build to resume", remedy `kbase build <volume-root>...`, whatever the
  checkpoint commits record.
- A `--resume` given a top `.tex` file or a `--bibliography`, and a build given neither top `.tex`
  files nor `--resume`: `check: usage` ([Inputs](inputs.md)).
- A `--ask-concurrency` under 1: `check: usage`. A `--through` that is neither a stage id nor a
  stage's display name: `check: usage`, `key` `--through`, `allowed` the stage ids.
- A dirty `kb-root/` that is not an interrupted stage's own uncommitted work: `check: dirty-paths`,
  one item per path, remedy "commit the change or restore the path to the last commit, then
  re-run".
- Checkpoint commits that record a stage while leaving an earlier stage unrecorded were written by
  a build running another stage order, and a build over them is refused before anything is restored
  or written: one item, `check: trail`, `path` the repository root, `detail` naming the recorded
  stage and the unrecorded one before it, remedy "start over from a commit before the trail".
- A run holding the run lock: `check: lock` ([§6](../6-build-state-ledger-and-monitoring/git-holds-the-recoverable-record.md)).
- A state store newer than this kbase, or whose run record names neither verb: `check: state-dir`
  ([§6](../6-build-state-ledger-and-monitoring/the-state-store-is-versioned.md)). A `--state-dir` inside `kb-root/`: `check: usage`, `key`
  `--state-dir`.
- A resume over an existing KB in an older format whose conversion is pending: `check: index`,
  remedy `kbase update-format`. Over a KB whose format version is newer than kbase: `check: kb-format`
  ([§10](../10-metadata-format-version/one-version-read-and-written.md)).
- No provider configured for a build that has a model-calling stage left to run, which includes a
  missing `decision` or `text` role without `--no-inference` (`check: provider`,
  [Inference preflight](inference-preflight.md)).
- Source refusals before the first stage: an unparseable paper (`check: source`), an include the
  build cannot load or that resolves outside the top `.tex` file's directory (`check: include`), an
  unclassified metadata key (`check: metadata-key`) and a pandoc that is missing or out of range
  (`check: pandoc`) ([Inference preflight](inference-preflight.md)).

A few readings of the checkpoint commits follow from these rules:

- A fresh build over checkpoint commits a removed `kb-root/` left behind starts a set of its own: a
  resume continues the newest build's checkpoint commits, read back to its `start` boundary, and
  `status` and `build-trail` read those alone.
- `status`, `build-trail` and a resume read no checkpoint commits that a later removal of
  `kb-root/` ends: the commits they read lie above the newest commit that removed `kb-root/` and is
  no `kb-build:` boundary. With `kb-root/` absent, or the commits so ended, and no build run holding
  the run lock, `status` reports `none` with no stage done and no `current`, and `build-trail` no
  boundary and `started-at` null; a live run is reported as it is. A build that ends with
  `kb-root/` absent hands out no `resume` in its result. A reset to before the removal restores
  `kb-root/` and the old checkpoint commits with it.
- A fresh build where `kb-root/` is absent counts no deleted path under it as dirt, committed or
  not, and its first boundary commit records the removal; any other dirty path under `kb-root/` is
  refused (`check: dirty-paths`).
- `kbase build --resume` over a finished build's checkpoint commits is `unchanged`.


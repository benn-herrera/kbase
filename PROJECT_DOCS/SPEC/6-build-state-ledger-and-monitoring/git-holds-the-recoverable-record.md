### Git holds the recoverable record

A build's **checkpoint commits** are its `kb-build:` commits on the current branch, one per stage
boundary, in the user's repository. Each has subject `kb-build: <stage-id> | <display name>` and a
structured body. Every boundary's body ends with the build's inputs: a `volume-root: <path>` line
per top `.tex` file, then a `bibliography: <path>` line per `--bibliography`, each in the order the
build reads them ([§5](../5-build-behaviour/index.md)), each path absolute. The `start` boundary's body also records `started-at:
<commit>`, the commit the build began from: the clean, ready-to-build state a fresh build over
other inputs starts again from. Resetting the branch to `started-at` discards the checkpoint
commits. On a branch with no commit, a fresh build first makes one empty commit, subject
`kb-start: the repository before the build` — outside `kb-build:`, so not a checkpoint commit —
after every refusal it can give and before its `start` stage, and records that commit as
`started-at`. A commit includes `kb-root/`, the build records in its `.build/` included
([§5](../5-build-behaviour/output-and-placement.md)): the node-pass record `kb-build-node-pass.yaml`, the unmarked record
`kb-build-unmarked.yaml`, the classification record `kb-build-classification.yaml` and the cull
record `kb-build-cull.yaml`, YAML at `1.0.0` ([§10](../10-metadata-format-version/index.md)). The cull record has `about`; `entries`, one per
co-linearity model call, sorted by `pass`, `a`, `b`, each with `a` and `b` (the two claim ids, `a`
before `b` in id order), `pass` (1 or 2), `verdict` (`a-adds-nothing`, `b-adds-nothing` or
`distinct`) and `outcome` (`answered`, `re-asked` or `defaulted`); `culls`, sorted by `id`, each
with `id` (a culled claim) and `into` (the claim it was culled into); and `links`, sorted by `a`,
`b`, every inferred link that survived the cull, each with `a` and `b` (id-sorted, with no
direction) and `harvest` (`unmarked`, `adjacent` or `culled`). The declared pass writes it with
every list empty. Resume position is read from the checkpoint commits. A user may reset to a stage
commit and resume from there. Per-leaf work inside an inference stage
stays uncommitted until the stage boundary; a resume over an interrupted stage restores `kb-root/`
to the last stage commit and re-runs the stage. **A build in flight owns
`kb-root/`:** a resume restores every path under it to the last boundary, whatever was done to it
since, and nothing vets what it finds there. An edit made while a build is interrupted is the
editor's to preserve in git, by waiting for the build or by reverting the commit that reset it; a write
op or `update-format` attempted while a build or a scoring run runs is refused ([§8](../8-maintenance-subcommands/index.md)).
Commits are scoped by pathspec to `kb-root/`.

**The state store contains what does not belong in the repository** — the run record, which `status`
and `cancel` read: the run's pid, its start, the verb that owns it (`build` or `score`) and, for a
scoring run, its run id — `progress.jsonl`, per-call evidence, the document-graph records ([§5](../5-build-behaviour/index.md)), the
scoring runs under `score/` (each with its ledger and `captures.jsonl`, every call's item text,
prompt and reply, failures included; [§8](../8-maintenance-subcommands/index.md)), a per-unit answer cache keyed by input hash, and the
captured output of a build the MCP server starts ([§11](../11-the-mcp-server/index.md)). Nothing in it is authoritative for
position. The verb is required of the run record: one naming neither verb is a store another
version wrote, `refused` with `check: state-dir`, `path` the record, and never read as a build's.
The store is at `$XDG_STATE_HOME/kbase/<key>/` (`~/.local/state/kbase/<key>/` where the variable
is unset or not an absolute path), where `<key>` is the first 16 hex digits of the SHA-256 of the
absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; a relative `--state-dir`
resolves against the repository root, and one inside `kb-root/` is refused (`check: usage`, `key`
`--state-dir`). The store contains cached model answers and survives `git clean`.

**The run lock** is the file `kbase-operation.lock` in the repository's git directory, recording
the state dir, and the lock file in the state store beside the run record. A repository has one
run at a time — a build or a scoring run, never both — whatever `--state-dir` each names. A clean
exit removes the lock files, cancel included, so a file in the store is never evidence of a live
run. Given no `--state-dir`, `status`, `cancel`, `wait` and `build-trail` read the store the run
lock names while its file exists and names one, else the default store; a new run never reuses it.
A second run is `refused` with `check: lock`, `path` the running run's state dir, and no remedy,
its `detail` naming the running run's verb and pid: "a build (pid 4321) is running; its state-dir
is …", or "a score run (pid 4321) is running; …". Where the run lock is held but names no state
dir yet, the second run gets `retry`; where it names one whose run record is not yet written, the
refusal says a run is starting. A run whose `--state-dir` store another repository's run holds gets `retry` (`check: lock`, `path` the store's lock file, `detail` naming the other run's verb, pid and start). `cancel`'s `retry` cases are at [§6](kbase-cancel.md). Every lock is advisory and exists on Unix only; on Windows `status`
cannot see a running run and no lock-based refusal or `retry` is given.

A maintenance operation over an existing KB that runs no git command — the queries, `verify`,
`export-latex` and a write batch without `commit` — works without a git repository. The commands
that run git are refused `check: worktree` outside a git worktree; they are listed at
[Write ops](../8-maintenance-subcommands/write-batches/write-ops.md).


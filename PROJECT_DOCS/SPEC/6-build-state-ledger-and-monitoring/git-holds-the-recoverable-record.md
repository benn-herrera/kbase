### Git holds the recoverable record

Each stage boundary is a commit in the user's repository, with
subject `kb-build: <stage-id> | <display name>` and a structured body. Every boundary's body ends
with the build's inputs: a `volume-root: <path>` line per volume root, then a `bibliography: <path>`
line per `--bibliography`, each in the order the build reads them ([§5](../5-build-behaviour/index.md)), each path absolute. The
`start` boundary's body also records `started-at: <commit>`, the commit the build began from: the
clean, ready-to-build state a fresh build over other inputs starts again from. On a branch with no
commit, a fresh build first makes one empty commit, subject `kb-start: the repository before the
build` — outside `kb-build:`, so no part of the trail — after every refusal it can give and before
its `start` stage, and records that commit as `started-at`. A commit carries `kb-root/`, the tracked build records in its `.build/` included ([page §2.5](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.5-domain-rule.md)):
the node-pass record `kb-build-node-pass.yaml`, the unmarked record
`kb-build-unmarked.yaml`, the classification record `kb-build-classification.yaml` and the cull
record `kb-build-cull.yaml`, YAML at `1.0.0` ([§10](../10-metadata-format-version/index.md)). The cull record holds `about`; `entries`, one
per co-linearity ask, sorted by `pass`, `a`, `b`, each with `a` and `b` (the two claim ids, `a`
before `b` in id order), `pass` (1 or 2), `verdict` (`a-adds-nothing`, `b-adds-nothing` or
`distinct`) and `outcome` (`answered`, `re-asked` or `defaulted`); `culls`, sorted by `id`, each
with `id` (a culled claim) and `into` (the claim it was culled into); and `links`, sorted by `a`,
`b`, every inferred link that survived the cull, each with `a` and `b` (id-sorted, carrying no
direction) and `harvest` (`unmarked`, `adjacent` or `culled`). The declared pass writes it with
every list empty. Resume position is read from the commit trail. A user may reset to a stage
commit and resume from there. Per-leaf work inside an inference stage
stays uncommitted until the stage boundary; a resume over an interrupted stage restores the
kbase-owned paths to the last stage commit and re-runs the stage. **A build in flight owns
`kb-root/`:** a resume restores every path under it to the last boundary, whatever was done to it
since, and nothing vets what it finds there. An edit made while a build is interrupted is the
editor's to carry in git, by waiting for the build or by reverting the commit that reset it; a write
op or `update-format` attempted while a build or a scoring run runs is refused ([§8](../8-maintenance-subcommands/index.md)).
Commits are scoped by pathspec to kbase-owned paths.

**The state store holds what does not belong in the repository** — the holder lock and the record
the holder writes, which `status` and `cancel` read: the holder's pid, its start, the verb that owns
the run (`build` or `score`) and, for a scoring run, its run id — `progress.jsonl`, per-call
evidence, the docgraph records ([§5](../5-build-behaviour/index.md)), the scoring runs under `score/` (each with its ledger and
`captures.jsonl`, every call's closed text, prompt and reply, failures included; [§8](../8-maintenance-subcommands/index.md)), a
per-unit answer cache keyed by input hash, and the captured output of a build the MCP server starts
([§11](../11-the-mcp-server.md)). Nothing in it is authoritative for position. A clean exit removes the holder lock's file,
cancel included, so a file standing in the store is never evidence of a live holder. The kind is
required of the record: one naming neither verb is a store another version wrote, `refused` with
`check: state-dir`, `path` the record, and never read as a build's. The store
lives at `$XDG_STATE_HOME/kbase/<key>/`, where `<key>` is the first 16 hex digits of the SHA-256 of
the absolute, symlink-resolved `kb-root/` path, overridable with `--state-dir`; it holds paid-for
inference and survives `git clean`. **The operation lock is in the repository's git directory**,
`kbase-operation.lock`, recording the state-dir: a repository carries one run at a time — a build or
a scoring run, never both — whatever `--state-dir` each names. Given no `--state-dir`, `status`,
`cancel`, `wait` and `build-trail` read the store the operation lock names while its file stands
and names one, else the default store; a new run never adopts it. A second run is `refused` with
`check: lock`, `path` the running run's state-dir, and no remedy, its `detail` naming the holder's verb and pid: "a
build (pid 4321) is running; its state-dir is …", or "a score run (pid 4321) is running; …". Where
the operation lock is held but names no state-dir yet, the second run gets `retry`; where it names
one whose holder record is not yet written, the refusal says a run is starting. Every lock is
advisory and exists on Unix only; on Windows `status` cannot see a running run and no lock-based
refusal or `retry` is given.


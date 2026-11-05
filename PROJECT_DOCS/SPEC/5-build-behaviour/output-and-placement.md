### Output and placement

The KB is written only at `kb-root/` in the git worktree root. `kb-root/` and everything below it
is kbase's domain: kbase checks and commits nothing outside it, not in a build, not in a write
batch's `commit` ([§8](../8-maintenance-subcommands/write-batches/write-ops.md)), and not in `update-format`'s clean check, which also checks
the build records at the repository root that it would move ([§8](../8-maintenance-subcommands/update-format.md)). An output the
caller sends elsewhere, such as `export-latex`'s `--out`, is not kbase state. The state store ([§6](../6-build-state-ledger-and-monitoring/index.md))
is never written into `kb-root/`.

The build records — `kb-build-node-pass.yaml`, `kb-build-classification.yaml`,
`kb-build-unmarked.yaml` and `kb-build-cull.yaml` — live in `kb-root/.build/`, so every path the
build commits is under `kb-root/`. They are committed at every stage boundary, so a rewind restores
them with the KB, and they stay after the build finishes.

**Fresh versus resume** is derived from the `kb-build:` checkpoint commits ([§6](../6-build-state-ledger-and-monitoring/index.md)), never configured.
`kbase build <volume-root>...` starts a fresh build where no `kb-root/` exists; `kbase build
--resume` continues the checkpoint commits. A fresh build where `kb-root/` exists, in any state, is
refused ([§5](starting-over.md)). The KB a build writes keeps every title unique within
its kind ([§10](../10-metadata-format-version/five-node-types.md)), and its writes go through the write ops, each followed by the
rebuild of the derived views ([§8](../8-maintenance-subcommands/write-batches/writes-leave-nothing-stale.md)): the derived views are current at
every checkpoint commit from the `spine-seed` stage on. A link pandoc wraps around a
cross-reference, its destination the bare LaTeX label, is unwrapped to the cross-reference inside
it, which links the target document's heading.

`build` refuses outside a git worktree, naming `git init`. It refuses if `kb-root/` is dirty and the
dirt is not an interrupted stage's own uncommitted work ([§6](../6-build-state-ledger-and-monitoring/index.md)), and only then.


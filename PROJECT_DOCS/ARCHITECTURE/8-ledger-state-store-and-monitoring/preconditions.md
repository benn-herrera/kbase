### Preconditions

**Pattern:** Precondition checks.

`run` refuses a `--resume` with nothing to resume as soon as it has the checkpoint commits, `kb-root/`
absent among the causes (`takeInputs`, SPEC §5 Starting over). `prepare` refuses a fresh build over any
`kb-root/` that exists with `buildstore.KBRootExists`, the refusal `init` gives, and drops the paths
under the absent `kb-root/` from the fresh build's dirt check, since its `start` boundary commits the
removal. `buildstore.TrailRefusal` refuses commits that record a stage past one they leave unrecorded,
in `Stages` order (`check: trail`), right after reading which stages are recorded and before the
launch checks and any restore: walking the gap would run a stage over the products of stages meant to
follow it. A `.git/index.lock` left by a kill during a boundary commit is refused by name
(`Repo.IndexLock`).


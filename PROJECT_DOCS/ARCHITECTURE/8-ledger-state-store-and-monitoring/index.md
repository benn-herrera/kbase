## 8. Ledger, state store and monitoring

**Pattern:** Checkpoint commits, a content-addressed answer cache, and a background job with a status file.

`internal/gitledger` is the only package that execs git ([§2](../2-invariants.md), I1). It execs the host's
git, which SPEC §2 Given interfaces requires, so the user's configuration, hooks and attributes apply,
and no git library is a dependency. `internal/build` owns the stage pipeline, the state store's writers,
resume, `--no-inference`, `--through`, progress and cancel. `internal/buildstore` owns the stage
vocabulary, the store's reading, status and the reading of checkpoint commits. Which packages and `cmd`
files import `gitledger` is in [§11](../11-package-map/dependency-rules.md).

Contents:

- [§8 Checkpoint commits and resume](checkpoint-commits-and-resume.md)
- [§8 Preconditions](preconditions.md)
- [§8 Regenerating missing records](regenerating-missing-records.md)
- [§8 The answer cache](the-answer-cache.md)
- [§8 State store layout and format](state-store-layout-and-format.md)
- [§8 State store retention](state-store-retention.md)
- [§8 Locks](locks.md)
- [§8 Status, progress, cancel and wait](status-progress-cancel-and-wait.md)
- [§8 The build's development replay](the-build-s-development-replay.md)

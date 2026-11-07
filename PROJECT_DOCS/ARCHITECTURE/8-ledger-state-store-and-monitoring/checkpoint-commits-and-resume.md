### Checkpoint commits and resume

**Pattern:** Checkpoint/restart pipeline; stage = idempotent unit.

Whether a build is fresh or a resume is read from the checkpoint commits alone; nothing in the state
store is authoritative for position (SPEC §6 Git holds the recoverable record). The checkpoint commits
are `gitledger.Repo.Trail`: every boundary commit reachable from `HEAD` above `Repo.Removal`, the newest
non-boundary commit that took `kb-root/` out of the tree, so a removal ends the commits below it.
`buildstore.CurrentTrail` takes the newest build's part, back to its newest `start` boundary, so a fresh
build over a removed `kb-root/` starts a set of its own. The four tracked build records
(`kb-build-*.yaml`: node pass, unmarked, classification and cull) are read and written by `claimgraph`,
and `build` commits them.

- **Inputs.** Once it has found the repository, `build.Run` makes a fresh build's top `.tex` files and
  `--bibliography` files absolute against the repository root and sorts them by path (`inRepository`)
  before anything reads them: the launch checks, the document graph and the regeneration of records.
  Every boundary records them, and `start` also records `started-at`, the `HEAD` before it; on a branch
  with no commit it first makes an empty `kb-start:` commit. The line keys and their readers are
  `internal/buildstore`'s (`RecordedInputs`, `StartedAt`). `--resume` takes the inputs from the newest
  boundary in the order recorded, never re-sorted (`takeInputs`; SPEC §5 Inputs).
- **Document graph tree.** `docgraph.Build` writes it, then every leaf is rewritten with each link
  pandoc wraps around a cross-reference unwrapped to that cross-reference (`build.unwrapTree`), in the
  build and in the regeneration of records alike.
- **Interrupted stage.** A stage writes its build records and its `kb-root/` work once its model calls
  have all returned, so a stop during the calls leaves those paths as the boundary left them, and a kill
  during the write leaves them partial; either way they are uncommitted. A resume restores those paths
  to the last stage commit (`Repo.Restore`) and re-runs the stage whole. The answer cache supplies the
  answers already obtained, so the re-run makes no call it already has an answer for. The re-run mints fresh ids, and nothing outside the restored paths referenced
  the old ones. A classify prompt names claim ids, but those were minted in stages already committed
  and the restore reaches back only to the last boundary, so a resumed run's prompts are identical and
  the cache answers them; a fresh build's are not and it makes the calls again. Resume reads no build
  record for position. The progress record accounts for the dirt an interrupted stage leaves
  (`uncommittedIn`); the dirty-path refusal (SPEC §5 Starting over) applies to dirt it cannot account for.
- **Resume command.** The resume command is recorded as the argv `kbase build --resume` with every flag
  the launch had but `--through` and the inputs; `--config-dir` is written absolute. `walk.result`
  emits it only where an identical repeat continues the work: a `bounded` run gets none, a refusal that
  recorded no boundary leaves nothing to continue, and none is emitted where `kb-root/` does not exist
  when the run ends. A result renders it in the form each interface takes (see
  [per-interface rendering](../9-result-emitter/per-interface-rendering.md)). It is also emitted only where the recorded argv
  parses as a `kbase build` invocation (`buildstore.RecordedCommand`, SPEC §6 `kbase status`): the state
  store is a directory the caller names, so a record naming any other program is repository-supplied
  text, and no result renders it for a harness to run.
- **Progress units.** Units are leaves for the node pass, source groups for `references-found`, the
  multichoice question groups of each of `claims-culled`'s two passes (each announcing its own total) and
  question groups for classification. A defaulted item is a `fallback` event. A stage's `units` event
  is written before its model calls and its unit and `fallback` events in one append after them, so
  `status` changes between waves. Every build record is written once per stage, after
  its calls.
- **`--no-inference`.** The overview call is the dropped step and no `README.md` is written
  (SPEC §5 Determinism; [§4](../4-stage-table/index.md)).


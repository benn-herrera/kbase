### Status, progress, cancel and wait

**Pattern:** Background job with a status file, signal cancel and a polling wait.

- **`status`.** The verb is `score.Status`. `buildstore.Status` reads `<state-dir>/lock` and its record,
  `progress.jsonl` and the checkpoint commits, and returns the snapshot in SPEC §6 `kbase status`. Where
  the record names `score`, `score.Status` reports the run in its own shape, from the run's ledger and
  stored result, its `state` the stored outcome mapped by `buildstore.StateOf`, as a build's
  ending is, and counts the run's answer files while it is in its item loop. Where no live run
  holds the run lock and the record names no score run, it compares the start `score.NewestRun` reads from
  the newest run directory's `run.yaml` with the newest build invocation's in `progress.jsonl`
  (`buildstore.ReadState`), and reports the build where neither exists or the two do not order. A
  build's `stages` lists the stages done and not done in the newest build's checkpoint commits and names
  `build-trail` for their commits. Where no build run holds the run lock and `kb-root/` is absent, or the
  commits above a removal are empty, `status` reads no commits (`buildstore.noBuild`).
- **`build-trail`.** `buildstore.BuildTrail` reads the same checkpoint commits. Each boundary's command is
  `git reset --keep` and then the build resumed with the flags its last run was launched with: the resume
  argv the store's newest run recorded, through `RecordedCommand`, which is the `build` call with
  `resume` alone where no run recorded one that parses as a `kbase build` invocation. `started-at`'s is
  the reset alone (SPEC §6 `kbase build-trail`).
- **`cancel`.** `cmd`'s `cancelScore` signals the holder the record names. For a live scoring run it
  calls `build.StopHolder` and takes the resume call from `score.ResumeOf`; otherwise `build.Cancel`
  does the same through `StopHolder` and reads the resume call from the progress record. `StopHolder`
  sends `os.Interrupt` to the pid and polls `<state-dir>/lock` until it is let go, or ends `retry` after
  `cancelWait`. The holder abandons the calls in flight with nothing written for them, logs the event,
  releases the locks and exits `cancelled` (SPEC §6 `kbase cancel`).
- **`wait`.** `wait` polls `buildstore.ReadState` through `buildstore.WaitState` every `waitPoll`, the
  store alone, until the run it names is superseded or has ended, then renders `status`'s result
  through `buildstore.Status`. It takes no lock and reads no checkpoint commits until that final
  result. A score run's pid is waited for through `score.RunByPID` and the store's record
  (`score.WaitRun`), so `wait` returns the run's own stored result, re-emitted by
  `result.Reemit`. Values are in [§12](../12-constants/index.md).


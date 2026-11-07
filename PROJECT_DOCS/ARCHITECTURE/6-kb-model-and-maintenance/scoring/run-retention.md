#### Run retention

**Pattern:** Keep-last-N retention.

`KeptRuns` ([§12](../../12-constants/index.md)) runs of each kind remain under `score/`. A real run's id is minted
`run-<id>` and a dry run's `dry-<id>` (`kb.MintID`'s prefix, `RunKindReal` and `RunKindDry`). A
directory's kind is its name's prefix (`runKind`), so the two kinds are counted apart.
`score.PruneRuns(dir, run)` keeps the run the invocation names, created or resumed, and the newest
`KeptRuns-1` other directories of its kind. It orders them by the `started` each `run.yaml` records,
equal starts by name with the later name newer, and removes the rest of that kind whole with
`os.RemoveAll`, captures and replays with them, through `buildstore.PruneByStart`. It also keeps the
newest `KeptRuns` score-run captures under the state store's `reports/` (`buildstore.PruneCaptures`,
prefix `score-`). `buildstore.DirStarts` is the single per-directory read of that start and
`buildstore.CompareStarts` the single ordering, both shared with `NewestRun`, so `status`'s newest run
and the prune's agree. A directory whose `run.yaml` is absent or unreadable yields no start and is kept.

`score.Invoke` calls the prune once the run's ledger has opened (after `Create` on the dry path, after
`Create` or `Resume` on the real one), so the run it keeps exists, and an invocation refused before then
removes nothing. It runs under the run lock, and a prune error ends the invocation `failed`. The one
writer under `score/` the run lock does not exclude is a replay, which takes no lock: a prune may remove
the run a replay is reading, and the replay then fails rather than writing into a removed directory
([the development replay](the-development-replay.md)). `ResumeTarget` filters runs by kind, so a `dry-`
run is never a resume target; `status`, `wait` and `RunByPID` find a run by its directory or pid,
whatever the prefix.


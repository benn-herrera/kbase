#### Run retention

`KeptRuns` ([§12](../../12-constants/index.md)) runs of each kind stand under `score/`. A real run's id is minted
`run-<id>` and a dry run's `dry-<id>` (`kb.MintID`'s prefix, `RunKindReal` and `RunKindDry`), and a
directory's kind is its name's prefix (`runKind`), so the two are counted apart; a dry run an earlier kbase minted as `run-…` is
named, and counted, as a real run. `score.PruneRuns(dir, run)` keeps the run the invocation names —
created or resumed — and the newest `KeptRuns-1` other directories of its kind, ordered by the
`started` each `run.yaml` records, equal starts by name with the later name newer, and removes the
rest of that kind whole with `os.RemoveAll`, captures and replays with them, through
`buildstore.PruneByStart`. `buildstore.DirStarts` is the one per-directory read of that start and
`buildstore.CompareStarts` the one ordering, both shared with `NewestRun`,
so `status`'s newest run and the prune's agree; a directory whose `run.yaml` is absent or unreadable
yields none and stands. `score.Invoke` calls the prune once the run's ledger has opened — after `Create`
on the dry path, after `Create` or `Resume` on the real one — so the run it keeps stands, and an
invocation refused before then removes nothing; it runs under the holder lock, and a prune error ends
the invocation `failed`. The one writer under `score/` the holder lock does not exclude is a replay,
which takes no lock: a prune may remove the run a replay is walking, and the replay then fails
rather than writing into a removed directory (below). `runIDRE` accepts a `run-` id alone, so `--resume` of a
`dry-` id is refused by the id-shape check, whose detail names a real run's id; `status`, `wait` and
`RunByPID` find a run by its directory, whatever the prefix.


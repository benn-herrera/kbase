#### A score run's status

Where the store's run record names a score run, `status` reports that run
in its own shape instead of the build's: `state-dir`; `state` (`running` while the run lock is
held, else `interrupted` where the run left no result document, or the outcome
its result document records — `done`, `cancelled`, `failed`, `refused` or `retry`); `pid`;
`started`; `run`; `pass`; `counts` (`total`, `done`, `failed`, `skipped`, `remaining`, read from the
run's `items.jsonl`, the status `ledger.jsonl` last records for each, and, while the run is in its
item loop, each answered item's result file); `scored` (the items whose
value was written); `failures` (each failed item and the reason its line recorded, with the `remedy`
the line records where it records one, [§8](../../8-maintenance-subcommands/index.md)); and `resume` (the call
continuing the run, `score` with `--resume`, as its result document has it, where the run
stopped short of `done`). It is the run's own records re-read, not a second
document: `wait` prints the result document the run wrote beside its ledger. A store with no live
run, and whose run record names no score run, reports its newest run, whichever verb it was: the
later of the newest score run's start, read from its `run.yaml`, and the newest build invocation's,
read from the progress record — each the record the run already wrote. A run directory whose
`run.yaml` is absent or unreadable left no record of when it started and is skipped. Where the
store records neither run, or the two starts cannot be compared — a missing or unparsable stamp on
either side — it reports the build as above.


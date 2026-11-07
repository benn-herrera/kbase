#### Outcomes

`done` when the run finished; `cancelled` when SIGINT or SIGTERM stopped it, with no line recorded for an item whose call was in flight or not yet made, and a line for every item answered before the stop; `failed` when an item's call or write failed, or the start rebuild did; `retry` (`check: lock`, [§6](../../6-build-state-ledger-and-monitoring/git-holds-the-recoverable-record.md)) while another run holds the run lock and has not yet recorded itself, while another repository's run holds a state store shared through `--state-dir`, or where the start rebuild could not take the KB write lock past the wait; `refused` where the run cannot go on:
- no scoring role in the configuration or the arguments (`key: models.scoring`);
- a `--pass` outside `rigor`, `bearing` and `all`; `--pass` beside `--node` or `--edge`, since a named item's pass follows from its kind; `--node` beside `--edge`; an `--edge` that is not three non-empty comma-separated parts; `--provider`; an `--ask-concurrency` under 1 — each `check: usage`;
- `--node` naming a work or an axiom, and `--edge` naming a `founds`, `references` or `demoted` edge, since no pass scores them (`check: usage`);
- `--node` or `--edge` naming a manually scored item without `--rescore-manual` (`check: usage`);
- a `--node` or `--edge` that names no node or edge (`check: unknown-id`);
- a `--resume` the list under [The ledger and resume](the-ledger-and-resume.md) refuses;
- a run already holding the run lock ([The run's locks](the-run-s-locks.md));
- a pending conversion, and a load error ([Refresh and format](refresh-and-format.md));
- a `--state-dir` inside `kb-root/` (`check: usage`, [§6](../../6-build-state-ledger-and-monitoring/git-holds-the-recoverable-record.md)).


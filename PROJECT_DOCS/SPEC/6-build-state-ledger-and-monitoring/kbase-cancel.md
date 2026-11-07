### `kbase cancel`

`cancel` signals the run that holds the run lock, of either kind — a build or a scoring run, as the
run record names it — and waits for it to let go. Its one argument is `state-dir`; its keys are in
[§7](../7-tool-result-contract-and-exit-codes/item.md). The event is logged, the lock is released, and the run process exits `cancelled`. The
run continues only when asked to, by `kbase build --resume` or `kbase score --resume`. Cancel costs
only the calls in flight, whose item is abandoned with nothing written for it: an answer already
returned is kept — in a build's answer cache, and in a score run's ledger; a cancelled score run's
`resume` is the call read from its stored result. A store with no run is `refused`: `check:
lock`, `path` the store's lock file, detail "no run holds the state store". A run that holds the store but has not yet recorded itself, or that does not let go of it within 20 s of the signal, is `retry` (`check: lock`, `path` the store's lock file).


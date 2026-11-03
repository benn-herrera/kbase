#### The run's locks

A scoring run takes the KB's operation lock and the state store's holder lock
for its length, in the order a build takes them, before it writes anything and before its start
refresh: a second run over the KB — a build or a scoring run — is `refused` at once, `check: lock`,
`path` the holder's state-dir, its `detail` naming the holder's verb and pid ([§6](../../6-build-state-ledger-and-monitoring/index.md)), never held through
a wait the first run will outlast. Its holder record names `kind: score` and the run id, which is
what `status`, `cancel` and `wait` read ([§6](../../6-build-state-ledger-and-monitoring/index.md)).


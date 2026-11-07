#### The run's locks

A scoring run takes the run lock for its length ([§6](../../6-build-state-ledger-and-monitoring/index.md)), in the order a build takes it, before it writes anything and before its start rebuild: a second run over the KB — a build or a scoring run — is `refused` at once, `check: lock`, `path` the holder's state-dir, its `detail` naming the holder's verb and pid, and does not wait for a run that will outlast the wait. The run lock's record names `kind: score` and the run id, which is what `status`, `cancel` and `wait` read ([§6](../../6-build-state-ledger-and-monitoring/git-holds-the-recoverable-record.md)).


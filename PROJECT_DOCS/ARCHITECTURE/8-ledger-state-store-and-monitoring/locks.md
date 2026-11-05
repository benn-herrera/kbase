### Locks

**Pattern:** Repo-scoped mutex lockfile plus a pidfile-style holder record.

Two locks exclude concurrent writers of a KB: the run lock, held for the length of a build or a
scoring run, and the KB write lock, held for the length of a write. Both are `internal/filelock`
advisory locks. `filelock.Acquire` takes an exclusive `flock` on an existing directory or on a lock
file it creates, waiting up to a given time. The descriptor holds the lock, so the operating system
releases it when the holder exits however it exits. On a platform without `flock` (`filelock_other.go`)
nothing is locked and nothing is reported held, so `status` cannot see a running run (SPEC §6 Git holds the
recoverable record).

**The run lock.** The run lock is two lock files, taken in a fixed order (SPEC §6 Git holds the
recoverable record; SPEC §8 The run's locks):

1. `<git dir>/kbase-operation.lock` (`buildstore.OperationLockPath`, found by reading `.git` and
   following a `gitdir:` file, since a maintenance verb that checks it never requires git). The lock is
   per repository, so a second run through any `--state-dir` is refused. The file contains the state
   directory the run named. An empty file is the window between acquiring the lock and writing the state
   directory: a second run gets `retry` and a writer proceeds.
2. `<state-dir>/lock` (`buildstore.HolderPath`). Its holder record (`buildstore.Holder`) is JSON with the
   holder's pid, its start, the verb that owns the run (`Kind`, `build` or `score`) and, for a scoring
   run, the run id. `status` reads liveness from the lock and the run from the record, `wait` reads
   whether the run it waits for still holds the lock, and `cancel` signals the pid.

The liveness test is the lock, not the pid: `filelock.Held` takes and drops a shared lock, which an
exclusive holder refuses. `build.Hold` takes both files in the order above, and its release function
removes `<state-dir>/lock` first (`ReleaseRemoving`), so a clean exit, `cancel` included, leaves neither
file and a standing file is not by itself evidence of a live holder. Because the release removes the
file, a waiter can lock a file already unlinked; `filelock` checks after each acquisition that the
path still names the file it locked (`stillAt`) and retries where it does not, so two holders never
coexist. Each wait is `buildstore.LockWait` ([§12](../12-constants/index.md)). The record is written after the lock is taken, so a state directory named by the
first file whose record is not yet written is reported "a run is starting". `takeOperationLock` and
`runningRefusal` give the refusals (`check: lock`, `path` the state directory), naming the verb and pid
read from the store the first file names. A second run on the same store through a different repository,
where only `<state-dir>/lock` is held, is `retry` (`takeLock`). `buildstore.ReadHolder` refuses a record
naming no known `Kind` (`check: state-dir`, `path` the record): it is a store another version wrote and
is never read as a build's. A scoring run takes the lock through the same `build.Hold`, in
`cmd/score.go`, before it writes its first record or does its start rebuild.

A build's order is: refuse a store newer than this kbase at open; `<git dir>/kbase-operation.lock`;
`<state-dir>/lock`; ready the store (upgrade and write `format`); the run event; prune the captured
builds; wait out writers. The wait (`awaitWriters`) takes and releases the KB write lock, or ends `retry`
if it cannot, so `status` sees a waiting build. A build's own writes, its rebuilds of the derived views
included, take no KB write lock, so it cannot refuse itself.

Given no `--state-dir`, the monitors (`status`, `cancel`, `wait`, `build-trail`) resolve their store
through `buildstore.MonitorOptions.Open`, which reads the store `<git dir>/kbase-operation.lock` names
while that file exists and names one, else the default store. The MCP launch sets `DefaultStore`, so a
new run never reuses the named store.

**Pattern:** Advisory file lock with timeout.

**The KB write lock.** `write.LockKB` takes an exclusive lock on the directory that contains `kb-root/`,
which exists before `kb-root/` does and leaves nothing to clean up, waiting up to `write.LockWait`
([§12](../12-constants/index.md)); a holder that keeps it past the wait gives `filelock.ErrHeld`, which
`write.HeldLockItem` turns into the `retry` item, `check: lock` with `path` the repository root (SPEC §8
One write lock). `write.Run` takes no lock;
the caller holds `LockKB` across it, except the build, which holds the run lock instead. `verify` and
the queries take no lock. The callers are
`cmd`'s `holdKB` (write batches, `update-format` and `init`) and `holdWriting`, and
`score.landValues`. `holdKB` probes `build.RunningOperation` for a held run lock before it waits on the
write lock and again after taking it, so a run that started during the wait is refused at once. A scoring
run's own start rebuild (`scoreRefresh`) and `landValues` take the KB write lock and make no probe: they
write as the run that holds the run lock. A scoring run holds the KB write lock around writing its
results and around its start rebuild, never across the run ([§6](../6-kb-model-and-maintenance/scoring/writing-the-results.md)).


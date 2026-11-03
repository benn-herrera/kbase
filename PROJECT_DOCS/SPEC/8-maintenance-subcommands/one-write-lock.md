### One write lock

A write batch and `update-format` hold the KB write lock for the whole run: from loading
the KB to the result, the batch's derivation and `update-format`'s re-derivation included. `init` holds it from before `kb-root/` exists — the lock sits on the directory `kb-root/` stands in, so it
is available at birth — through the skeleton, the trailing refresh and the birth commit to the
result. `score` takes it around its one landing — every value the walk produced, after the walk — and
around its start refresh, never across the run ([§8](index.md)). A writer that cannot take it within 30 s gets `retry` with `check: lock`, `path` the
repository root, and the remedy. A write verb probes for a run holding the operation lock ([§6](../6-build-state-ledger-and-monitoring/index.md)) before
it waits on the write lock and again after taking it: where one runs it is `refused` with the `lock`
item at once, taking no wait. A build waits out a write in progress, up to the same 30 s, before it
writes (`retry` if it cannot); its own writes take no lock. A scoring run's landings and refreshes
make no operation-lock probe, writing as the run that holds it. `verify` and the queries take
no lock. On Windows no
advisory lock exists ([§6](../6-build-state-ledger-and-monitoring/index.md)): neither the `retry` nor the run-running refusal is available there, and
one writer at a time is the operator's to keep.

**Batches** are all-or-nothing, their derivation included ([page §2.2](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)–[§2.3](../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)).


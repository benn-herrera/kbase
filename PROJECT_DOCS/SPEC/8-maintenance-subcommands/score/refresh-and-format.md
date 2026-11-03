#### Refresh and format

A KB whose load fails ([page §2.4](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)) is refused before the run's start refresh, naming each fault. A KB
in an older format is refused before it too, dry run or not, while its conversion is pending
(`check: index`, remedy `kbase update-format`, [page §2.6](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.6-format-conversion.md)); its document carries `migration`. One
refresh opens a run that asks, before the index is read, so the derived files stand current before
the walk; a dry run writes nothing. Each landing is followed by the derivation inside its batch
([page §2.3](../../../../NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)), so the derived files are current whenever the run exits — finished, failed or
cancelled. The refresh holds the KB write lock and releases it before the walk; one that cannot
take the lock ends the run there, with the lock's own outcome — `retry` where another writer held
it past the wait — and the `lock` item, whose remedy is `re-run kbase score`. A crash inside a
landing's write can leave a stale index, which `verify` reports and the next run's start refresh
repairs.

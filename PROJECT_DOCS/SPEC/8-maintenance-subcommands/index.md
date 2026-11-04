## 8. Maintenance subcommands

Every subcommand below is reachable by an external agent harness as a builtin tool — its own, as
an operation of the `query` tool, or, a write op, as an entry of the `write` tool's batch ([§11](../11-the-mcp-server.md)) — `probe` excepted — a
tool would let an agent poll a model, which no KB operation asks for ([§11](../11-the-mcp-server.md)); `score` starts a detached
child that asks one model call per item and returns at once with the run's identity, so the turn is
not held for the walk's duration ([§11](../11-the-mcp-server.md)). The rules in this section govern each.

Contents:

- [§8 Values input](values-input.md)
- [§8 Idempotence](idempotence.md)
- [§8 Writes leave nothing stale](writes-leave-nothing-stale.md)
- [§8 One write lock](one-write-lock.md)
- [§8 `init`](init.md)
- [§8 Write ops](write-ops.md)
- [§8 `restore-demoted`](restore-demoted.md)
- [§8 `add-<kind>`](add-kind.md)
- [§8 `edit-<kind>`](edit-kind.md)
- [§8 Edge ops](edge-ops.md)
- [§8 `remove-node`](remove-node.md)
- [§8 `move-node`](move-node.md)
- [§8 `add-citation`](add-citation.md)
- [§8 Queries](queries.md)
- [§8 `citations-of`](citations-of.md)
- [§8 `find`](find.md)
- [§8 `show`](show.md)
- [§8 `stats`](stats.md)
- [§8 `edges`](edges.md)
- [§8 `trace`](trace.md)
- [§8 `export-latex`](export-latex/index.md)
- [§8 `score`](score/index.md)
- [§8 `probe`](probe.md)
- [§8 `update-format`](update-format.md)
- [§8 `verify`](verify.md)
- [§8 `status`, `cancel`](status-cancel.md)

### M — the edge ops (expert)

**Design:**
- [§1.2 Edges and relations](../../NEW_MCP_CLI_DESIGN/1-model/1.2-edges-and-relations.md) — relations, legal pairings and fields
- [§1.3 Demoted edges](../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md) — demoted records and their restore
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Values": the value check on `context`
- [§2.9 Manually scored, and the judged hash](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md) — the mark an edge op writes
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — an existing edge refuses `add-edge`, a missing one `remove-edge`
- [§3.2 Edge writes](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/index.md), [`add-edge`](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/add-edge.md), [`edit-edge`](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/edit-edge.md), [`remove-edge`](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/remove-edge.md), [`restore-demoted`](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/restore-demoted.md) — the four edge ops
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — MCP and CLI notes: the write ops' notes

**Scope:**
- `internal/write/edges.go` (new `addEdge`, `editEdge`, `removeEdge`; `restoreDemoted`);
- `internal/write/values.go`;
- `cmd/add-edge.go`, `cmd/edit-edge.go`, `cmd/remove-edge.go` (new);
- `cmd/restore-demoted.go`;
- `cmd/surface.go`;
- `internal/write/ops.go` (the op entries, and `EntryResult` fields for an edge entry's result);
- `internal/write/records.go` (the per-entry report `applyEntries` copies into `EntryResult`
  carries `restored`, known only while the entry applies);
- `internal/write/edits.go` and `inserts.go` where a refusal or lookup the edge ops share (the "no
  node `<id>`" refusal, the cycle cache) lives once;
- `cmd/write_op.go` (`batchFields`: an edge entry carries `source`, `target`, `relation`, and
  `restore-demoted`'s `restored`, beside the add and edit entries' `id`; `batchHelp`'s result
  sentence names them) and `cmd/write_op_test.go`.

**Facts acted on:**
- [Page §3.2](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/index.md): four ops, their keys, results and refusals. [Page §1.3](../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md): demoted records and the three
  ways to resolve one.
- [Page §2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md): an existing edge refuses `add-edge`, and a missing record refuses `remove-edge`.
- The edge ops' `context` key takes [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s value check, which package H added.
- Today `drop-edge` reports `unchanged` on an edge with no record ([SPEC.md:784–794](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/drop-edge.md)). [Page §2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)
  replaces that rule for the new ops. `drop-edge` itself stays until package S (stage 4).
- `restore-demoted`'s keys today: `source`, `target`, `relation: demoted` ([SPEC.md:692–707](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/restore-demoted.md)).
  `git grep -l 'restoreDemoted\|refuseRepeatedPairs' -- '*.go'` → `cmd/restore-demoted.go`,
  `cmd/surface.go`, `internal/write/edges.go`, `internal/write/ops.go`.

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): sections citing [page §3.2](../../NEW_MCP_CLI_DESIGN/3-operations/3.2-edge-writes/index.md).
- [SPEC §10](../../PROJECT_DOCS/SPEC/10-metadata-format-version/index.md): the demoted-record text, citing [page §1.3](../../NEW_MCP_CLI_DESIGN/1-model/1.3-demoted-edges.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.

**Tests:**
- `add-edge` for each relation and pairing, with `allowed` the relations that join the kinds.
- A cycle is refused with `check: dependency-cycle`, the cycle in edge direction.
- An `add-edge` over a demoted record writes the edge and deletes the record.
- `edit-edge` marks the edge manually scored on a bearing.
- `remove-edge` with `relation: demoted` removes every such record of the pair.
- Every other record stays byte for byte.
- A re-issue of add or remove is refused with `kb-root/` identical.

**Done-line** (after Rule 13's two): `just test-integration`.

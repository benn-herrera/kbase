### J — `add-<kind>` (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — the kinds, ids, fields, and titles unique within a kind
- [§1.4 Statement markers](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) — placement by `document`, `line` and `snippet`
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Values": the value check on prose keys
- [§2.9 Manually scored, and the judged hash](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md) — the mark an add writes
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — "node exists"
- [§3.1 Node writes](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md), [`add-claim`, `add-analysis`, `add-experiment`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-claim-add-analysis-add-experiment.md), [`add-work`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-work.md), [`add-axiom`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/add-axiom.md) — "Existing nodes" and the five add ops
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — MCP and CLI notes: the write ops' notes

Relies on stage 1: the `kbase-id` marker and its no-space attach (`format400`'s marker, re-exported
by `internal/kb`).

**Scope:**
- `internal/write/` (`inserts.go`: the new add ops beside the old inserts; `values.go` `opFields`;
  `ops.go` `writeOps`);
- `cmd/add-claim.go`, `cmd/add-analysis.go`, `cmd/add-experiment.go`, `cmd/add-work.go`,
  `cmd/add-axiom.go` (new);
- `cmd/surface.go`.

**Facts acted on:**
- [Page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md):
  - the keys table, defaults and refusals;
  - the identity an add checks: the title within its kind; a work's key; an axiom's label;
  - "node exists" ([§2.10](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md));
  - placement by `document`, `line` and `snippet` ([§1.4](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md));
  - `add-axiom` appends its heading;
  - no edge lists.
- The add ops' prose keys take [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s value check, which package H added.
- Today's inserts adopt by title and host ([SPEC §8 "Idempotence"](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/idempotence.md), the text opening "An insert of a
  claim, an analysis or an experiment adopts by title and host") and place by a quote `locator`
  (`markAtLocator`, `internal/write/inserts.go:339`). `markAfterHeading` (`:390`) is the old
  analysis and experiment inserts' rule.
- None of these is the add ops' behaviour. The old inserts keep working until package S (stage 4).
- The title-uniqueness check ([page §1.1](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md), [§3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md) "Existing nodes") binds the five add ops this package
  lands, and package K's title edits. The `insert-*-entry` ops keep their present adoption rule
  until package S removes them. So the build, which issues `insert-claim-entry` and
  `insert-work-entry` until package R (stage 3) moves it (`internal/claimgraph/write.go:79`,
  `:132`), is not refused by the check.
- The I6 restatement lands here (see "Decisions the record settles").

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): an `### add-<kind>` section citing [page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): a bullet.
- ARCHITECTURE I6 and §6.

**Tests:**
- Each op over its keys table.
- A title that its kind holds is refused, naming that node's id. The same title on another kind is
  accepted.
- A line that lacks the snippet is refused, and the refusal names the nearby lines where it occurs.
- A fence line is refused.
- The marker lands with no space, and a heading's anchor is unchanged.
- A re-issued add is refused and `kb-root/` is byte-identical.

**Done-line** (after Rule 13's two): `just test-integration`.

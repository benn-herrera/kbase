### L — `remove-node` and `move-node` (expert)

**Design:**
- [§1.4 Statement markers](../../NEW_MCP_CLI_DESIGN/1-model/1.4-markers.md) — where `move-node` places a marker
- [§1.5 Citations](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md) — the citations `move-node` re-points and `remove-node` checks
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Order": checks at the entry, not at the batch's end
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — a removal of something missing is refused
- [§3.1 Node writes](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md) — the section's common rules
- [§3.1 `remove-node`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md) and [`move-node`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/move-node.md)
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — notes and shared bodies: the write ops' notes, `remove-node`'s body

Relies on stage 1: the citation-link reader, in `format400` and re-exported by `internal/kb`
(`git grep -n 'func .*[Cc]itation' -- internal/formats/format400 internal/kb`).

**Scope:**
- `internal/write/marks.go` (`dropClaimNode`, `refuseRemainingMentions`, `markNode` stand until
  package S);
- new ops in `internal/write/`;
- `cmd/remove-node.go`, `cmd/move-node.go` (new);
- `cmd/surface.go`;
- `internal/write/ops.go` (the op entries, and `EntryResult` fields for `removed-edges`);
- `cmd/write_op.go` (`batchFields`: a `remove-node` entry carries `id` and `removed-edges`, full
  edge rows; a `move-node` entry `id`; `batchHelp`'s result sentence names them) and
  `cmd/write_op_test.go`.

**Facts acted on:**
- [Page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/remove-node.md) `remove-node`:
  - every record naming the node goes, demoted records included, and its marker or axiom heading;
  - `removed-edges` as full edge rows;
  - mention refusals at the point the entry applies: citations by link-title id, and prose fields of
    surviving records, whose remedy is an `edit-<kind>` (package K) or `edit-edge` (package M)
    entry earlier in the same batch.
- [Page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/move-node.md) `move-node`: one op for every kind with a marker; re-points the node's citations; the
  refusals.
- [Page §4.2](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md): `remove-node`'s help says plainly that it removes every edge naming the node, and
  points to the dry run (Rule 12).
- The mention check runs per entry under the batch: `applyEntries` (`internal/write/records.go`)
  calls an op's `finish` with that entry alone, so `drop-claim-node`'s `refuseRemainingMentions`
  already checks at the entry ([page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) has no end-of-batch check). `remove-node` checks the
  same way, against citations and every surviving prose field. `drop-claim-node` stays until
  package S.

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): sections citing [page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): bullets.

**Tests:**
- A node with edges at both ends and a demoted record is removed with all of them, and each shows in
  `removed-edges`.
- A citation refuses the removal, naming `path:line`.
- An `edit-edge` earlier in the same batch clears a `context` mention.
- A dry run previews the cascade.
- `move-node` re-points citations and leaves the citing words; a work or an axiom is refused.

**Done-line** (after Rule 13's two): `just test-integration`.

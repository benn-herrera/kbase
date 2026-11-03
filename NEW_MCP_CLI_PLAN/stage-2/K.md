### K — `edit-<kind>` (expert)

**Design:**
- [§1.1 Nodes](../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md) — the fields each kind carries, titles unique within a kind
- [§1.5 Citations](../../NEW_MCP_CLI_DESIGN/1-model/1.5-citations.md) — link targets and the naming form a title edit keeps live
- [§2.2 Write batches](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) — "Values": the value check on prose keys
- [§2.9 Manually scored, and the judged hash](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md) — the mark an edit writes
- [§2.10 Existence](../../NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md) — lookup first, `unchanged` for a value already in place
- [§3.1 Node writes](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/index.md) — the section's common rules
- [§3.1 `edit-<kind>`](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md) — the five edit ops
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — the four texts and
  renderings
- [§4.2 Tool descriptions, help resources and `--help`](../../NEW_MCP_CLI_DESIGN/4-served-text/4.2-tool-descriptions-and-help.md) — MCP and CLI notes: the write ops' notes

Relies on stage 1: the citation-link reader, link text, target and title id, in `format400` and
re-exported by `internal/kb` (`git grep -n 'func .*[Cc]itation' -- internal/formats/format400 internal/kb`).

**Scope:**
- `internal/write/` (new edit ops; `records.go` `setRigor`, `setRationale`, `setStatus` stand until
  package S);
- `cmd/edit-claim.go`, `cmd/edit-analysis.go`, `cmd/edit-experiment.go`, `cmd/edit-work.go`,
  `cmd/edit-axiom.go` (new);
- `cmd/surface.go`;
- `cmd/write_op.go` (`batchFields`: an edit's result entry carries `id`, as an add's does since
  package J's `write.Adds`; `batchHelp`'s result sentence names it);
- `internal/kb/leaf.go` where the axiom heading rewrite needs it: `kb.AxiomHeadingLine` composes
  the heading beside its parser.

**Facts acted on:**
- [Page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md) `edit-<kind>`:
  - the field table;
  - at least one field;
  - a title change re-points the node's citations and gives a naming-form citation the new title;
  - `edit-axiom` rewrites its heading;
  - an edit writes no other document;
  - the refusals, a title another node of the same kind carries included.
- The edit ops' prose keys take [page §2.2](../../NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)'s value check, which package H added.
- Manual marks follow [page §2.9](../../NEW_MCP_CLI_DESIGN/2-conventions/2.9-manually-scored-and-judged-hash.md): re-asserting the value of a record marked `false` marks it `true`,
  outcome `done`.
- The judged hash recorded on a manual set lands in package Q (stage 3). Here an edit keeps the
  record's hash as it stands.

**Contract updates:**
- [SPEC §8](../../PROJECT_DOCS/SPEC/8-maintenance-subcommands/index.md): an `### edit-<kind>` section citing [page §3.1](../../NEW_MCP_CLI_DESIGN/3-operations/3.1-node-writes/edit-claim-edit-analysis-edit-experiment-edit-work-edit-axiom.md).
- [SPEC §12](../../PROJECT_DOCS/SPEC/12-interim-releases/index.md): a bullet.

**Tests:**
- Each kind's allowed and refused fields.
- An id of another kind is refused, its remedy naming the op for its kind.
- A title edit onto a title its kind holds is refused.
- A title edit re-points an axiom's and a work's citations and renames naming-form text, leaving a
  quote as it stands.
- An unchanged value is `unchanged`.

**Done-line** (after Rule 13's two): `just test-integration`.

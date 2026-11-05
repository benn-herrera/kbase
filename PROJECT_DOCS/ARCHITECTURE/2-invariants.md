## 2. Invariants

Violating any is a blocking defect. Where a cell ends "Detail below", the detail follows the table.

| # | Invariant | Pattern |
|---|---|---|
| I1 | Exactly one package execs each external process: pandoc, git, Graphviz `dot` and kbase itself. The exec-policy test names each owner and scans the shipped code. Detail below. | One adapter per external process, enforced by a fitness test |
| I2 | Records, not leaf markup, are the source of the claim-graph inventory's reader and placement facts. Only the readers named below read leaf lines; maintenance ops read only the metadata layer. Detail below. | Single source of truth per fact; reads go through a repository |
| I3 | There are two pre-passes, in one registry ([§5.2](5-reader-and-transforms/5.2-pre-pass-registry.md)), each naming the reader-level hole it fills and emitting a count report. | Preprocessor registry with instrumentation |
| I4 | System compatibility per SPEC §3 Compatibility contract; byte equality only where a named reader needs it; presentation is untested. | Semantic or contract testing over golden fixtures |
| I5 | One result emitter owns stdout ([§9](9-result-emitter/index.md)). | A single structured-output writer; outcome → exit-code table |
| I6 | A repeated write leaves `kb-root/` unchanged (SPEC §8 Idempotence). Detail below. | Idempotent writes (create-if-absent / PUT semantics) |
| I7 | Every prompt template and fragment is an embedded file (`go:embed`). No prompt prose originates in code. Each call's system prompt is the fragment named for it. | Prompt templates as embedded assets (`go:embed`) |
| I8 | One metadata format version is read and written (SPEC §10 One version read and written). Every KB file is read through `kb.Source`, which `internal/kbload` alone builds for a command's read. Only `update-format` writes a conversion. Detail below. | Read-time upcasting plus an explicit migrate command; repository/loader monopoly |
| I9 | The MCP server exposes subcommands and adds no operation, and its stdout carries JSON-RPC frames and nothing else. Mechanism: [§12](12-constants/mcp-server-spec-11/index.md). | Generated API surface from a command registry |
| I10 | Every model call whose answer a build or a scoring run recorded can be re-rendered, for as long as its records are retained, under the binary's embedded templates, and its reply re-read as the run read it. `probe` is outside it. Detail below. | Record/replay reproducibility (request capture) |
| I11 | While a concurrent call batch is in flight, the only file written is one per result, unique to it, in the state store, written by the item that made the call after it released its permit. Every file that more than one result lands in is written after the batch has drained. Detail below. | Fan-out/fan-in: per-task output files, single-writer merge after join |
| I12 | Every model call an invocation makes goes through a `model.Slot`. Only the invocation's one `model.Queue` issues slots, from `Go`, and only while one of its N permits is free; nothing else bounds model calls in flight. Detail below. | Counting semaphore with a capability token |

**I1 detail.** The owners are `internal/latex/pandoc` (checks the version and `pandoc-api-version`,
with named errors), `internal/gitledger`, `internal/sheet` and `internal/mcp/launch.go` (the MCP
server's detached build and score runs). `cmd/execpolicy_test.go` binds non-test files; a test may
start the processes its fixtures need.

**I2 detail.** The inventory's page-defined readings ([§5.5](5-reader-and-transforms/5.5-records.md)) come from the leaf through
the page readers. Otherwise leaf lines are read only by:
- stage-A conformance: the link relation, through `kb`. It checks the leaf markup itself, so the
  markup is what it must read;
- the prose and statement-marker machinery: the marker reader `kb.MarkerLines` and the hosts the load
  derives from it, the write ops' placing and moving of a marker, and a scoring text's statement. A
  marker and the text it marks are leaf content; no record holds them;
- the sheet: a claim's kind, at its marker. The sheet draws a claim as its text appears — a block, a
  displayed equation or prose — which is a fact of the leaf's layout, not of the claim; a record would
  restate the leaf and go stale on the next edit. Where no leaf carries the claim's marker, the title
  decides, as a best guess for the drawing only.

Each exception reads what records cannot hold without restating the leaf ([§3](3-principles.md),
Metadata first). Maintenance ops read the node and edge records, the statement markers and the format
version.

**I6 detail.** An add of what exists and a removal of what is absent are `refused`; an edit to the
value already held is `unchanged`. Unlike PUT, an add of an existing item is refused, not a no-op.
`init` creates once, and `update-format` refuses a repeat until its output is committed.

**I8 detail.**
- Stage machinery reads the tree its own stages wrote through `kb.OnDisk` (`docgraph`'s `writtenTree`,
  `claimgraph`'s `builtTree`); every command's read of a KB goes through `kbload` ([§11](11-package-map/loading.md)).
- `kbload` is the one importer of `internal/formats`' root from outside the tree; it converts an older
  KB at load ([§12](12-constants/format-migration-spec-10/index.md)).
- The live format's definition is `internal/formats/format400`, which imports no kbase package and
  which `kb` alone imports and re-exports.
- Every write other than `update-format` refuses a source whose conversion is pending
  (`index.PendingConversion`).

**I10 detail.** A build leaves what it needs through its checkpoint commits, its records and the
answer cache's keying on the composed prompt, so re-running a stage from its boundary re-renders
every model call. Its group records hold each multichoice call's template inputs (those a group
shares once, the rest per item) and its `prompt-sha256`; each overview call's capture holds its
`excerpts` and, on a retry, its `rejected-lines`. A recorded call therefore re-renders without the
source tree. A scoring run leaves it through its `captures.jsonl` ([§6](6-kb-model-and-maintenance/scoring/the-captures.md)).

**I11 detail.** A concurrent call batch is a build stage's model calls (each cull pass and the
overview its own), a scoring run's item loop or a replay's item loop. The state-store files written
in flight are a build's answer and capture per call and a scoring run's result per item. The rule
takes the answer cache (one file per call) as its model, for three reasons: a killed run forfeits
only the calls in flight; every shared file is written in an order the stage or set already defines,
so the concurrency setting changes wall time and nothing else (SPEC §5 Determinism); and a stage
stopped mid-batch has written no shared file, so its resume restores to the boundary.

**I12 detail.** A `Slot` is an item's right to make calls on one of the queue's permits. An item holds
its permit across all of its calls and releases it before it writes a file. Within one consumer,
items take their permits in the order the consumer launches them. The
methods that put a request on the wire are unexported, so the compiler holds every other package to a
slot.

Deliberately unspecified: the internal AST representation, record field encoding, and layout
algorithms.


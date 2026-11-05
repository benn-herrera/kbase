# ARCHITECTURE – kbase

How this project's implementation meets requirements designated by SPEC

Must-see:
- No user-facing behaviors or outcomes are designed here, that belongs to SPEC
- No project-specific process is dictated, that belongs to CONVENTIONS
- Every mechanism stated here is decided and landed
  - §13 records the verdicts of the slice that settled the reader's design

## 1. Purpose

kbase makes a knowledge base auditable for an academic lab running local inference. The KB carries a
claim graph whose nodes and edges are drafted deterministically and refined by model calls, and every
refinement is checkable against the artifact it refined. kbase builds that KB as Go, keeping its own
provider management and inference invocation.

kbase is assembled from standard patterns at small scale. Each section below names its pattern in its
first line and states only where kbase departs from it, or what the pattern leaves open. Three parts
are novel, the three of THESIS's "What is new":

1. **What a claim graph is made of**: five node kinds and four scoring relations (SPEC §10 Five node
   types; SPEC §10 Four scoring relations, one arithmetic). Realized in the `claims-declared`,
   `claims-discovered` and `analyses-split` stages (§4) and in
   [§6](#the-node-vocabulary).
2. **The local judgments**: the fault criteria in the scoring prompt templates, and the bearing
   predicates that decide which edges are judged ([§6](#the-prompt-shelf),
   [§6](#bearing-predicates)). The templates, the input guard and the reply parse are standard
   machinery.
3. **How the judgments combine**: the strength arithmetic ([§6](#the-derived-layer)).

### Pattern map

One row per `##` and `###` heading of this document, in document order. `Novel:` marks the novel
parts above; every other row is a standard pattern. The last column names the row of THESIS's
building-blocks table that the section implements. A `####` heading carries its own pattern in its
opening line and has no row here.

| Section | Link | Pattern | THESIS building block |
|---|---|---|---|
| 1. Purpose | §1 | — | — |
| Pattern map | [§1](#pattern-map) | — | — |
| 2. Invariants | §2 | — | — |
| 3. Principles | §3 | — | — |
| 4. Stage table | §4 | Checkpoint/restart pipeline | Builds and scoring runs |
| 5. Reader and transforms | §5 | Pandoc as the parser; Go transforms over its JSON AST | Reading LaTeX |
| 5.1 pandoc seam | §5.1 | Wrapped external tool with a version gate | Reading LaTeX |
| 5.2 Pre-pass registry | §5.2 | Preprocessor registry with instrumentation | Reading LaTeX |
| 5.3 JSON transforms | §5.3 | Pandoc AST filter | Reading LaTeX |
| 5.4 Writer route | §5.4 | Off-the-shelf writer | Reading LaTeX |
| 5.5 Records | §5.5 | Sidecar metadata stream | The knowledge base |
| 5.6 Tree derivation, partition checks and `validate_build` | §5.6 | Round-trip conformance checks and a link checker | Checks |
| 5.7 LaTeX export | §5.7 | Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked output directory | — |
| 6. KB model and maintenance | §6 | — | The knowledge base |
| The `kb` package | [§6](#the-kb-package) | Domain model and schema package | The knowledge base |
| Write API | [§6](#write-api) | Transactional unit of work with dry run | Writes |
| Provenance | [§6](#provenance) | Manual-override flag | — |
| Build-only ops | [§6](#build-only-ops) | Internal privileged ops | Writes |
| The node vocabulary | [§6](#the-node-vocabulary) | Novel: Prefixed random ids | The claim graph |
| The derived layer | [§6](#the-derived-layer) | Novel: Topological fold over a DAG (Kahn's order); materialized views rebuilt after each write | Derived pages and sheets; The claim graph |
| Cycle breaking | [§6](#cycle-breaking) | Greedy feedback-arc removal by DFS back edges | The claim graph |
| The query layer | [§6](#the-query-layer) | Read model | Queries |
| Frontmatter reading | [§6](#frontmatter-reading) | Heuristic sniffing before parse | The knowledge base |
| Sheet | [§6](#sheet) | Graphviz rendering with a bounded process pool; a materialized view | Derived pages and sheets |
| Scratch creation | [§6](#scratch-creation) | Scaffold command | — |
| Scoring | [§6](#scoring) | Job runner with an append-only event log, resume from the last status per item, optimistic-concurrency commit, projection rebuild | Builds and scoring runs; Model calls; Asking a model |
| 7. Inference seams | §7 | Role-based model routing behind provider adapters | Model calls; Configuration and credentials |
| Probe | [§7](#probe) | Health-check / smoke call | Model calls |
| Wire clients | [§7](#wire-clients) | Provider adapters behind one interface; connection pooling | Model calls |
| Retry and attempt outcomes | [§7](#retry-and-attempt-outcomes) | Retry with backoff on retryable status | Model calls |
| Refused keys | [§7](#refused-keys) | Typed error classification at the boundary | Configuration and credentials |
| Layered configuration | [§7](#layered-configuration) | Layered configuration | Configuration and credentials |
| Resolution and the client registry | [§7](#resolution-and-the-client-registry) | Factory registry with lazy, shared clients | Model calls; Configuration and credentials |
| The decision API | [§7](#the-decision-api) | Constrained classification endpoint | Asking a model |
| The multichoice-ask interface | [§7](#the-multichoice-ask-interface) | Dependency injection with an architecture fitness test | Asking a model |
| Credentials | [§7](#credentials) | Local secrets file with a redacting secret type | Configuration and credentials |
| 8. Ledger, state store and monitoring | §8 | Checkpoint commits, a content-addressed answer cache, and a background job with a status file | Builds and scoring runs |
| Checkpoint commits and resume | [§8](#checkpoint-commits-and-resume) | Checkpoint/restart pipeline; stage = idempotent unit | Builds and scoring runs |
| Preconditions | [§8](#preconditions) | Precondition checks | Builds and scoring runs |
| Regenerating missing records | [§8](#regenerating-missing-records) | Reproducible-build verification | Checks |
| The answer cache | [§8](#the-answer-cache) | Content-addressed memoization cache | Builds and scoring runs; Asking a model |
| State store layout and format | [§8](#state-store-layout-and-format) | Versioned on-disk schema with sequential migrations | Builds and scoring runs |
| State store retention | [§8](#state-store-retention) | Keep-last-N retention | Builds and scoring runs |
| Locks | [§8](#locks) | Repo-scoped mutex lockfile plus a pidfile-style holder record | Builds and scoring runs |
| Status, progress, cancel and wait | [§8](#status-progress-cancel-and-wait) | Background job with a status file, signal cancel and a polling wait | Builds and scoring runs |
| The build's development replay | [§8](#the-builds-development-replay) | Offline eval / record-replay regression harness | Builds and scoring runs; Checks |
| 9. Result emitter | §9 | Single structured-output writer | The CLI and MCP server |
| Per-interface rendering | [§9](#per-interface-rendering) | Per-surface presentation layer | The CLI and MCP server |
| 10. Compatibility harness | §10 | Black-box integration tests with golden fixtures | Checks |
| 11. Package map | §11 | Package responsibility listing | — |
| Loading | [§11](#loading) | Schema validation at load, fail-closed | The knowledge base |
| The loader's file set and its other refusals | [§11](#the-loaders-file-set-and-its-other-refusals) | Schema validation at load, fail-closed | The knowledge base |
| The zero-value hazard | [§11](#the-zero-value-hazard) | Go zero-value pitfall; required-field decoding | — |
| Dependency rules | [§11](#dependency-rules) | Architecture fitness functions | Checks |
| 12. Constants | §12 | Named constants registry | — |
| Format migration (SPEC §10) | [§12](#format-migration-spec-10) | Chained schema migrations (upcasters) with golden tests | Format versions |
| MCP server (SPEC §11) | [§12](#mcp-server-spec-11) | MCP server generated from the command tree; JSON-RPC over stdio; detached child plus status polling | The CLI and MCP server |
| 13. Slice verdicts | §13 | Spike/prototype decision record | — |
| Slice-1 checks | [§13](#slice-1-checks) | — | — |
| 14. Limits | §14 | Known-limitations section | — |
| A conversion records no history | [§14](#a-conversion-records-no-history) | — | — |
| Merged metadata is refused, not merged | [§14](#merged-metadata-is-refused-not-merged) | — | — |
| No format sits between two majors | [§14](#no-format-sits-between-two-majors) | — | — |
| No refresh moves a judgement | [§14](#no-refresh-moves-a-judgement) | — | — |

## 2. Invariants

Violating any is a blocking defect. Where a cell ends "Detail below", the detail follows the table.

| # | Invariant | Pattern |
|---|---|---|
| I1 | Exactly one package execs each external process: pandoc, git, Graphviz `dot` and kbase itself. The exec-policy test names each owner and scans the shipped code. Detail below. | One adapter per external process, enforced by a fitness test |
| I2 | Records, not leaf markup, are the source of the claim-graph inventory's reader and placement facts. Only the readers named below read leaf lines; maintenance ops read only the metadata layer. Detail below. | Single source of truth per fact; reads go through a repository |
| I3 | There are two pre-passes, in one registry (§5.2), each naming the reader-level hole it fills and emitting a count report. | Preprocessor registry with instrumentation |
| I4 | System compatibility per SPEC §3 Compatibility contract; byte equality only where a named reader needs it; presentation is untested. | Semantic or contract testing over golden fixtures |
| I5 | One result emitter owns stdout (§9). | A single structured-output writer; outcome → exit-code table |
| I6 | A repeated write leaves `kb-root/` unchanged (SPEC §8 Idempotence). Detail below. | Idempotent writes (create-if-absent / PUT semantics) |
| I7 | Every prompt template and fragment is an embedded file (`go:embed`). No prompt prose originates in code. Each call's system prompt is the fragment named for it. | Prompt templates as embedded assets (`go:embed`) |
| I8 | One metadata format version is read and written (SPEC §10 One version read and written). Every KB file is read through `kb.Source`, which `internal/kbload` alone builds for a command's read. Only `update-format` writes a conversion. Detail below. | Read-time upcasting plus an explicit migrate command; repository/loader monopoly |
| I9 | The MCP server exposes subcommands and adds no operation, and its stdout carries JSON-RPC frames and nothing else. Mechanism: [§12](#mcp-server-spec-11). | Generated API surface from a command registry |
| I10 | Every model call whose answer a build or a scoring run recorded can be re-rendered, for as long as its records are retained, under the binary's embedded templates, and its reply re-read as the run read it. `probe` is outside it. Detail below. | Record/replay reproducibility (request capture) |
| I11 | While a concurrent call batch is in flight, the only file written is one per result, unique to it, in the state store, written by the item that made the call after it released its slot. Every file that more than one result lands in is written after the batch has drained. Detail below. | Fan-out/fan-in: per-task output files, single-writer merge after join |
| I12 | Every model call an invocation makes goes through a `model.Slot`. Only the invocation's one `model.Queue` issues slots, from `Go`, and only while one of its N slots is free; nothing else bounds model calls in flight. Detail below. | Counting semaphore with a capability token |

**I1 detail.** The owners are `internal/latex/pandoc` (checks the version and `pandoc-api-version`,
with named errors), `internal/gitledger`, `internal/sheet` and `internal/mcp/launch.go` (the MCP
server's detached build and score runs). `cmd/execpolicy_test.go` binds non-test files; a test may
start the processes its fixtures need.

**I2 detail.** The inventory's page-defined readings (§5.5) come from the leaf through
the page readers. Otherwise leaf lines are read only by:
- stage-A conformance: the link relation, through `kb`;
- the prose and statement-marker machinery: the marker reader `kb.MarkerLines` and the hosts the load
  derives from it, the write ops' placing and moving of a marker, and a scoring text's statement;
- the sheet: a claim's kind, at its marker.

Maintenance ops read the node and edge records, the statement markers and the format version. The
exceptions are enumerated here; the invariant does not state why each reads leaf lines.

**I6 detail.** An add of what exists and a removal of what is absent are `refused`; an edit to the
value already held is `unchanged`. Unlike PUT, an add of an existing item is refused, not a no-op.
`init` creates once, and `update-format` refuses a repeat until its output is committed.

**I8 detail.**
- Stage machinery reads the tree its own stages wrote through `kb.OnDisk` (`docgraph`'s `writtenTree`,
  `claimgraph`'s `builtTree`); every command's read of a KB goes through `kbload` ([§11](#loading)).
- `kbload` is the one importer of `internal/formats`' root from outside the tree; it converts an older
  KB at load ([§12](#format-migration-spec-10)).
- The live format's definition is `internal/formats/format400`, which imports no kbase package and
  which `kb` alone imports and re-exports.
- Every write other than `update-format` refuses a source whose conversion is pending
  (`index.PendingConversion`).

**I10 detail.** A build leaves what it needs through its checkpoint commits, its records and the
answer cache's keying on the composed prompt, so re-running a stage from its boundary re-renders
every model call. Its group records hold each multichoice call's template inputs (those a group
shares once, the rest per item) and its `prompt-sha256`; each overview call's capture holds its
`excerpts` and, on a retry, its `rejected-lines`. A recorded call therefore re-renders without the
source tree. A scoring run leaves it through its `captures.jsonl` ([§6](#the-captures)).

**I11 detail.** A concurrent call batch is a build stage's model calls (each cull pass and the
overview its own), a scoring run's item loop or a replay's item loop. The state-store files written
in flight are a build's answer and capture per call and a scoring run's result per item. No reason is
recorded for the rule.

**I12 detail.** An item holds its slot across all of its calls and releases it before it writes a
file. Within one consumer, items take their slots in the order the consumer launches them. The
methods that put a request on the wire are unexported, so the compiler holds every other package to a
slot.

Deliberately unspecified: the internal AST representation, record field encoding, and layout
algorithms.

## 3. Principles

These steer choices the invariants leave open.

- **Pattern:** Generate-then-validate. **Propose-and-verify at every model seam.** The model never
  mutates canonical bytes. It emits data with a deterministically checkable post-condition; code
  validates and executes.
- **Pattern:** Constrained output. **The model supplies values; the machine owns syntax.** A prompt
  presents its subject fully resolved, states which values are open, and takes the answer in a grammar
  the machine wrote. No prompt asks a model to construct a document.
- **Pattern:** Graceful degradation to a deterministic baseline. **Monotone safety.** Every
  model-refined artifact has a deterministic fallback that was already acceptable. Model failure
  degrades quality, never correctness. No stage exits on a model's opinion; every stage exits on a
  comparison of artifacts or a return code.
- **Pattern:** Hermetic, deterministic build. **Provenance and reproducibility are artifact
  properties.** The same inputs and pandoc version give a byte-identical document graph (tree and
  records) and a claim graph identical modulo the ids it mints (SPEC §5 Determinism); derived views
  are regenerable and freshness-checked. Ids are minted randomly, hence "modulo ids".
- **Pattern:** Reuse an existing parser. **No LaTeX parser is written here, partial or whole.** The
  reader is pandoc.
- **The kbase and personant integration is the target.** A prior decision that prevents it is
  removed, not designed around.
- **Pattern:** Self-describing API. **The served text is the operating contract.** What an agent needs
  to drive kbase reaches it from kbase itself: a command's own facts in its help; rules spanning
  commands in the MCP instructions, kept short because every session reads them; longer guidance as
  MCP resources, read on demand. The `kbase` skill supplies the workflow, as the instructions say
  (SPEC §11 Served text), chiefly when to engage kbase at all; a rule a command or the server can
  state is served, not left to the skill.

---

## 4. Stage table

**Pattern:** Checkpoint/restart pipeline.

The build walks these stages in order. A stage boundary is a checkpoint commit
(§8). The resume position is read from the checkpoint commits
alone, never from the state store (SPEC §6 Git holds the recoverable record). "Detail below" marks a
stage whose mechanism follows the table.

| Stage | Does | Pattern |
|---|---|---|
| `start` | pandoc and git preflight; on a fresh build, refuses any existing `kb-root/` (`buildstore.KBRootExists`, as `init` does); takes the run lock ([§8](#locks)); records `started-at` and the inputs, absolute and in path order, that every later stage reads (SPEC §5 Inputs; SPEC §6 Git holds the recoverable record) | Preflight checks plus a job manifest |
| `document-graph` | The tree from the top `.tex` files, in path order | Deterministic transform stage |
| `spine-seed` | Writes the format version where the tree carries none, then the two record files, empty, over that tree (`build.seedSpine`) | Deterministic transform stage |
| `claims-declared` | Claim-graph pass over author-declared claims | Novel: THESIS "What is new" item 1, defined in SPEC §10 Five node types. Machinery: AST extraction of environments |
| `claims-discovered` | Claim-graph pass over discovered claims | Novel: THESIS "What is new" item 1, defined in SPEC §5 A build marks every claim and SPEC §10 Five node types. Machinery: claim detection by a model classifier |
| `equations-minted` | Equation nodes minted | Mechanical extraction |
| `references-found` | Unmarked references found. Detail below. | Candidate generation then pairwise classification |
| `claims-culled` | Co-linearity cull (SPEC §5 Co-linearity cull (`claims-culled`)). Detail below. | Near-duplicate clustering with a canonical representative |
| `depends-attributed` | Dependency edges attributed (SPEC §5 Dependency attribution (`depends-attributed`)), then cycles broken. Detail below. | Pairwise relation classification, then cycle removal |
| `analyses-split` | Analyses split (SPEC §5 Analysis split (`analyses-split`)). Detail below. | Novel: THESIS "What is new" item 1, defined in SPEC §5 Analysis split (`analyses-split`). Machinery: deterministic text segmentation |
| `phase-3a` | Rebuilds the derived views, runs `verify`, and writes the readiness documents (SPEC §3 Stamped documents) | Materialized views rebuilt after each write, with a staleness check |
| `overview-drafted` | The overview document | Generate → validate → retry once |

**`references-found`.** Adjacency links are made by rule between consecutive non-equation claims of a
leaf, unasked. Each source claim gets its own shortlist of proximity slots followed by wildcard slots
(counts in §12):
- proximity: the nearest claims in its own paper by locality (same document, same directory, same
  paper), then by places apart in the sequence of every claim by reading position and id;
- wildcard: the best by TF-IDF cosine (`rankShortlist`, `internal/claimgraph/shortlist.go`) less the
  proximity picks.

No pool holds a marked pair or an adjacency link. A pair both ends pick is asked both ways, and a
proximity slot its paper cannot fill stays empty. One multichoice call per planned pair; a yes in
either direction is one undirected link for the cull. The stage writes nothing under `kb-root/`. The
slot counts have no recorded reason.

**`claims-culled`.** `Culled` (`internal/claimgraph/cull.go`) asks one `Colinear` call per link with a
prose end (marked pairs, unmarked yeses and adjacency links, unordered, computed over the tree
`references-found` read), grouped by the earlier claim in reading order (`p1-<id>`). One confirming
call per culled claim whose root is not its target, grouped by root (`p2-<root>`). The survivor rule
and the update of references are the pure `cullTargets`, `cullRoots`, `finalCulls` and `PlanCull`;
culls are applied through `IssueCulls` with no bridge and no claim-to-claim edge; the cull record is
written once, after the culls are applied. The survivor is the earliest partner in reading order
(SPEC §5 Co-linearity cull (`claims-culled`)); no reason is recorded for choosing it.

**`depends-attributed`.** One multichoice call per candidate. Every answer's edge is written in one
`add-build-edges` batch, cycles included; then `index.BreakCycles` walks the records in the tree's
hosted order and each edge it cuts is applied through `demote-claim-edge` (`claimgraph.ringRow`), its
warning in the stage report (SPEC §10 Cycles: cut as the walk finds them). Cycle breaking is
[§6](#cycle-breaking). Cycles are written first and cut afterwards because the cut reads the records
the edge batch left, cycle included.

**`analyses-split`.** `Split` (`internal/claimgraph/split.go`) takes a claim's bounds from
`claimBodies` and `bindProofClaims`. The statement proper ends at the sentence `opener` picks for a
prose claim's title. The analysis is hosted by `split-claim`'s `statement-line` and `statement-end`:
on the claim's own block, or, where the claim's own text holds nothing more, on its first bound proof
in reading order (`readingOrder`). A claim whose host block has no blank line is left unsplit and
counted (`unsplit-no-blank-line`). The analysis's title is the claim's title rebased to the
analysis's summary page. Every split is applied in one `split-claim` batch. The stage asks no model,
so no step of it drops under `--no-inference`, and it writes no build record.

Each claim-graph stage is one `internal/claimgraph` invocation. Every build record a stage writes
lands in `kb-root/.build/` (`buildrecords.Dir`), so the build owns `kb-root/` alone
(`buildstore.OwnedPaths`) and each boundary commit stays within it (SPEC §5 Output and placement).
The build's writes go through the write ops ([§6](#build-only-ops)).

- `--through <stage>` stops the build after the named stage; `--no-inference` drops the steps that
  make model calls and runs the rest (SPEC §5 Determinism). Which steps drop follows each step's
  classification in the step table (`buildstore.Steps`: `spendsOwnInference`, or a system prompt),
  never a stage id.
- A step marked `spendsInferenceInPart` (`depends.attribute`) is not dropped: part of it makes model
  calls, but the step still runs under `--no-inference` with a nil `claimgraph.Options.Launch`,
  which is what not asking is. The provider preflight asks `CallsAModel`, which counts such a step.
- `phase-3a` writes the readiness documents through `kbdocs.Stamp`. A build resuming over an older KB
  is refused while its conversion is pending (`loadFormat`, [§12](#format-migration-spec-10)).
- Stage boundaries are where expensive work becomes recoverable: a stage's per-leaf model work is
  uncommitted until its boundary.

---

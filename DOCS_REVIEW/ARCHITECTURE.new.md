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
refinement is checkable against the artifact it refined. kbase is a Go program that builds that KB. It
manages its own model providers and makes its own inference calls.

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
| Write API | [§6](#write-api) | All-or-none validated batch with dry run and per-file atomic writes | Writes |
| Provenance | [§6](#provenance) | Manual-override flag | — |
| Build-only ops | [§6](#build-only-ops) | Internal privileged ops | Writes |
| The node vocabulary | [§6](#the-node-vocabulary) | Novel: the node kinds. Machinery: prefixed random ids | The claim graph |
| The derived layer | [§6](#the-derived-layer) | Novel: the strength formula. Machinery: topological fold over a DAG (Kahn's order); materialized views rebuilt after each write | Derived pages and sheets; The claim graph |
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
| I11 | While a concurrent call batch is in flight, the only file written is one per result, unique to it, in the state store, written by the item that made the call after it released its permit. Every file that more than one result lands in is written after the batch has drained. Detail below. | Fan-out/fan-in: per-task output files, single-writer merge after join |
| I12 | Every model call an invocation makes goes through a `model.Slot`. Only the invocation's one `model.Queue` issues slots, from `Go`, and only while one of its N permits is free; nothing else bounds model calls in flight. Detail below. | Counting semaphore with a capability token |

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

**I12 detail.** A `Slot` is an item's right to make calls on one of the queue's permits. An item holds
its permit across all of its calls and releases it before it writes a file. Within one consumer,
items take their permits in the order the consumer launches them. The
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
prose claim's title. The analysis's marker goes on the first blank quoted line of the claim's block,
or at the end of its paragraph's last line; the claim's marker moves to just after its sentence, on
the line the sentence ends on (`split-claim`'s `statement-line` and `statement-end` move the claim's
marker, not the analysis's). Where the claim's own text holds nothing more, the analysis's marker
goes on the first blank quoted line of the first bound proof in reading order (`readingOrder`). A claim whose host block has no blank line is left unsplit and
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

## 5. Reader and transforms

**Pattern:** Pandoc as the parser; Go transforms over its JSON AST.

pandoc parses and kbase never does. `docgraph.Build` converts each top `.tex` file (one volume) in
this order, then writes the trees:

1. `prepass.Apply` scans the source text (§5.2).
2. `pandoc.ReadLaTeX` parses it to the JSON AST (§5.1).
3. `readVolume` walks the AST once and derives the outline and the records (§5.5).
4. `transform` rewrites the AST (§5.3).
5. `pandoc.WriteGFM` renders it (§5.4).
6. `buildTree` splits the rendering into documents and `place` places the records in them.
7. Five checks compare the products (§5.6).

After the last volume, `writeTrees` writes the tree and the entry point, `validateBuild` and the
dead-link check read it back, and `internal/build` rewrites each link wrapped around a
cross-reference to its text (`unwrapTree`). Every step is deterministic and runs no model.

### 5.1 pandoc seam

**Pattern:** Wrapped external tool with a version gate.

`internal/latex/pandoc` is the only package that execs pandoc (I1). Beyond the pattern:

- **Gate.** `Preflight` checks the version and `pandoc-api-version` against the accepted range
  (SPEC §2 pandoc). Each failure is a named error: `MissingError` names the install page,
  `VersionError`, `SourceError`, `BibliographyError`, `IncludeError` and `EscapeError`.
- **Reader.** `ReadLaTeX` runs LaTeX to JSON with no filters, and with `--citeproc` exactly when a
  bibliography is offered: without one, citeproc would render every citation unanswered.
- **Include check.** `ReadLaTeX` refuses an `\input` or `\include` that resolves outside the
  volume's directory before pandoc runs, reading every included file in turn so an escape below the
  first level is caught (SPEC §5 Inference preflight).
- **Writer.** `WriteGFM` is the writer interface. It runs standalone (`-s`), so the title and
  abstract arrive as the rendering's YAML metadata block.
- **Export.** `RunDefaults`, `pandoc -d <defaults file>`, is the export's one call, under the same
  version gate. The `export-latex` verb supplies it as a function value, and `internal/export` never
  reaches pandoc (§5.7).

### 5.2 Pre-pass registry

**Pattern:** Preprocessor registry with instrumentation.

`internal/latex/prepass` holds the two source scans, `theoremDisplayNames` and
`stripEnvironmentDeclarations`, in one registry, and nothing else changes source (I3). Each entry
names the reader-level hole it fills and emits a count report of what it found, plus the
declarations it could not read, which stay where the author wrote them. Adding an entry is an
ARCHITECTURE change: pandoc is the parser, and a pre-pass exists only to fill a hole in what pandoc
reads.

### 5.3 JSON transforms

**Pattern:** Pandoc AST filter.

`internal/docgraph` transforms the pandoc JSON in Go, not in a Lua filter: the forward transform
emits records (§5.5), which a filter cannot. The transforms are:

- Cite span wrapping. With no usable bibliography the span contains the citation's keys as text
  (`keysOnly`), since citeproc did not run.
- author-declared Div to labelled blockquote
- title-page drop
- display maths lifted out of emphasis

`read` repeats the read with no bibliographies when the reader cannot use the set offered
(SPEC §5 Bibliographies). Metadata keys are classified as content or apparatus, and a key in
neither is a `MetadataKeyError` (SPEC §5 Inference preflight).

Whole-volume rendering, the split into documents and deriving the outline also live here.

### 5.4 Writer route

**Pattern:** Off-the-shelf writer.

The route is pandoc's gfm writer over the transformed JSON with `--wrap=none`. The first slice
check settled it (§13): every load-bearing form the slice checked is produced
under it.

### 5.5 Records

**Pattern:** Sidecar metadata stream.

Alongside the Markdown leaves, Go emits records (JSONL) into the build state store
(§8), never into `kb-root/`: `records/docgraph.jsonl`, and
`records/prepass.jsonl` for the pre-pass count reports. `records.Write` replaces each file whole by
atomic rename. The claim graph takes the tree as its sole input; the records are kbase's channel
for the reader and placement facts, so nothing is recovered from leaf markup (I2). They contain:

- **reader facts:** display name, identifier, owning document, order; references (macro type as
  pandoc writes it, labels); the enclosing author-declared block of each reference and citation
  (pandoc's Div nesting); `\label` names inside display maths; citations (keys and state:
  resolved, unanswered, key-only); the bibliography's works; references in a proof's opening run;
- **placement facts**, set when the tree is split into documents: each reference's resolved
  document and fragment, and each copy of a heading's reference the navigation lines contain (the
  up-link line and the index child list).

A block record exists exactly where the written leaf has a label line `kb.LabelLine` reads: an
author block the transform emits as a top-level labelled blockquote whose name that reader's
environment group accepts (letters and single spaces), or, because the reader keys on the page, a
bold-only paragraph opening an author's blockquote, which it parses as a label line. A block nested
inside another, one rendered inside raw HTML (a figure), or one whose name the reader rejects
(`table*`) yields no record, and the facts inside it are hosted by the enclosing readable block.
One walk derives the outline and the records, so the rule is applied once.

Labels are recorded in the form the page writes them. pandoc escapes `&<>"'` in attributes, and
differently for a `Link` than for a `Span` or `Div`, so a record compared against the page has the
escaped form.

**Page-defined readings** are readings over the written leaf, taken from it through the page
readers: the locator span and title off a block's display line; line extents; the word the page
shows before a reference; a work's rendered text; a claim's statement text (its block extent, prose
paragraph, equation fence, or the section under a heading its marker ends) and the paragraph around
a reference, as model input; printed name-and-number mentions in node bodies; the printed number and
optional title off the display line, which no record contains; the source leaf's render, shown as the
unmarked model call's body. Claim-bearing classification and proof-to-subject binding remain
claim-graph rules. Field encoding is deliberately unspecified.

### 5.6 Tree derivation, partition checks and `validate_build`

**Pattern:** Round-trip conformance checks and a link checker.

`internal/docgraph` derives the tree and runs the build's own checks. Each compares one
deterministic product against another:

| Check (report name) | Compares |
|---|---|
| `A-ast-to-markdown` | pandoc's parsed document against pandoc's rendering: every word run of every leaf block and content-bearing metadata value reaches the rendering, in order |
| `B-markdown-to-tree` | that rendering against the tree kbase split from it |
| `math-survives` | pandoc's maths elements against the writer's three forms |
| `anchor-lands` | the label read back off the link pandoc wrote |
| `image-asset` | the author's image files on disk |
| `validate-build/4-reachability` (`validateBuild`) | the tree as it lies on disk: every document up-linked to its parent index and reachable by down-links from the entry point |
| `verify-md-links` | the dead-link check over `kb-root/`, shared with `verify` (`kb.DeadLinks`, [§6](#the-kb-package)) |

Findings land in the stage's report in the state store
(§8, `reports/`). A `FAIL` finding makes the outcome
`failed`; a `FACT` finding describes state and never fails the stage. kbase runs no fix loop.

### 5.7 LaTeX export

**Pattern:** Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked
output directory.

The mechanism of `export-latex` (SPEC §8 `export-latex`), which defines the files of `gen-tex/`, the
content rules, simplification and idempotence. The export emits no records, so pandoc's own filter
mechanism suffices where §5.3 uses Go, and the author can re-run the
conversion with plain pandoc.

#### One run, three owners

The verb (`cmd/export-latex.go`) opens the KB through `kbload.Open`, with the in-memory format
migration of [§12](#format-migration-spec-10) where the KB needs it. It refuses a load error, and
each `--volume` that is not a volume of `kb.Volumes` (`notVolumes`). It hands the KB to `export.Run`
with the pandoc calls as two function values it supplies: the version check (`pandoc.Preflight`)
and the one `pandoc.RunDefaults` call. The verb takes no KB write lock and runs no git.

`export.Run` is the export's one sequence, its steps unexported so nothing drives them but it:

1. `walk` traverses the KB.
2. `selectVolumes` narrows to `--volume`. A name the entry point does not list is an
   `UnknownVolumesError`, which the verb maps to a usage refusal.
3. `assemble` builds `gen-tex/`'s inputs in memory.
4. The version check runs, so a pandoc refusal precedes any write.
5. `writeOut` writes the inputs.
6. The defaults run is `pandoc -d defaults.yaml` in `gen-tex/`, so every path in the defaults file
   is relative and the author's re-run from there resolves the same way.
7. `writeReport` writes the report.

The verb maps what `Run` returns onto the result document.

| Owner | Owns |
|---|---|
| Go (`internal/export`) | What pandoc cannot know: the navigation tree's order and ranks, the label namespaces, the KB's contract forms as data. The stream files, the tables, the defaults file, assets, the manifest |
| The filter (`kbase-export.lua`) | Every rewrite of the parsed document: references, citations, links, labels, headings, theorem blocks, figures, maths fixes, the bibliography. Constant code over the tables |
| The template (`kbase-export.latex`) | The LaTeX preamble, package order, theorem machinery, running heads, front and main matter |

Both the filter and the template are embedded (`filter.go`) and written verbatim, so their bytes do
not vary with the KB.

#### Reader

`gfm+attributes-gfm_auto_identifiers`. The KB's maths is gfm's: pandoc 3.12.1's `markdown` reader
has `native_spans` but not `tex_math_gfm`, and `gfm` the reverse. `markdown` would not parse the KB's
maths, and `gfm` leaves the KB's contract forms (label spans, reference anchors, citation spans) as
raw-html open and close pairs, which the filter pairs itself. `attributes` supplies the `{#id}` the
document boundaries use; with `gfm_auto_identifiers` off, a heading without one has no identifier.

#### Streams and file scope

The stream is `kbase-export-front.md`, then one `kbase-export-vol{N}.md` per volume in order.

- The front file is the entry point's prose: its first heading is taken as the document `title`, its
  up-link and volume-list lines are removed, and every remaining heading is marked `{.unnumbered}`.
- A volume file contains the volume's index and every document of its navigation tree with
  navigation stripped (`export.Strip`).

`defaults.yaml` lists the files as `input-files` with `file-scope: true`, so pandoc parses each file
apart and prefixes its identifiers and `#`-link targets with `<file>__`. The scope is the volume,
for two reasons: one concatenated parse would collide the footnote labels of different volumes, and
a footnote's definition in a built KB can sit in a different document of its volume than the
reference, which a per-document scope would sever. The filter's first act removes the prefix, for
the file names in the tables' `streams` and no other `__`.

#### Document boundaries

The first heading of each document, outside code fences, has the document's label as a `{#d{N}-…}`
id, written by Go (`kb.FirstHeading` over `kb.StripCodeFences` finds the heading); a volume's index
has `vol{N}`. A document with no heading opens with one naming its path (the file name less its extension; the directory name for an index). Headings are used
rather than fenced divs because a `:::` line inside a code fence closes a fenced div early. Go marks
the boundary and the filter reads it: the document a node belongs to is the last heading before it
whose id is a `documents` key, and before the first one the front matter.

#### Tables

`kbase-xref.json` is generated from the traversal, each document's first heading and the
bibliography sources, and named to the filter by the metadata key `kbase-xref`. The field-level
schema is the head comment of `kbase-export.lua`; the keys:

- `documents`: heading id → path, volume `N`, rank;
- `paths`: kb-root-relative path → heading id (href resolution);
- `volumes`: in order, each with its `vol{N}` label and title;
- `theorems`: every display name `kb.LabelLine` reads off a label line of the front matter or an
  exported document, so the label-name grammar stays in Go;
- `bibliography`: citation key → entry body as gfm text. Each exported volume's `references.md`
  entries (`kb.BibliographyEntries`) and, where any exported volume has none, every work record (a
  work's title); the first entry of a key is kept;
- `report`: the path the filter writes its findings to;
- `assets`: kb-root-relative path of a copied file → its path under the out directory; the
  claim-graph sheets (SPEC §3 `claim-graph.svg`) are not copied, being drawings of the metadata;
- `entry_point`: the entry point's path, the front matter's source;
- `streams`: the stream file names in reading order.

#### The filter

Preparation, then two passes over the one document pandoc reads, then the end.

**Preparation.** The file-scope prefix is removed. Each block list is made native:

- A raw-html `figure` or `table` the gfm reader split at its blank lines is reassembled and parsed
  as html (its MathML as maths containing the annotation's TeX; spans left as raw pairs).
- A raw `<img>` or `<embed>` becomes an image, and the copy a built KB writes beside such a tag is
  dropped.
- A table with no caption or id followed by a paragraph ending in an empty id span (the form
  pandoc's gfm writer gives a captioned table, `{#tab:…}`) takes that paragraph as its caption and
  the id as its own.

**Pass 1** collects author labels under the volume of the document they sit in, keyed (volume,
label): every `\label{L}` in a maths body, every `<span id="L">` pair, every figure and table id,
and every id on a native span, link, div or code block, these being the elements whose id the
writer prints as a label. A figure that contains a table is not one. A label defined twice in one
volume is a finding, as are two labels of one volume formatted alike in LaTeX.

**Pass 2** rewrites in document order; anything a rule returns in place of its element is walked by
every rule, once.

- `<span id="L">…</span>` pair → span with id `v{N}-L`, which the writer prints as
  `\protect\phantomsection\label{…}`. A native span, link, div, code block, table or figure id is
  namespaced the same way.
- `<a data-reference-type=… data-reference=… href=…>…</a>` pair (also a link with the `reference`
  attribute, which the html reader makes of one inside a figure or table): `data-reference` names
  the target.
  - Where the href names an exported document, or is absent, and the target volume (that
    document's, else the current one) defines every label named → `\eqref` for `eqref`, one
    `\cref{…}` for `ref+label`, one `\Cref{…}` for `ref+Label`, `\ref` otherwise, with labels
    `v{M}-L`.
  - Where the href names an exported document but the volume does not define every label →
    `\ref{d{M}-…}`.
  - Otherwise the anchor's text and a finding.
- `<span class="citation" data-cites="k1 k2">…</span>` pair → `\cite{k1,k2}` where every key has a
  `bibliography` entry, keys recorded in first-emission order; otherwise the span's text and a
  finding, so no `\cite` is undefined and no text is dropped.
- A `\label`, `\ref`, `\eqref`, `\cref` or `\Cref` with a braceless argument inside a maths body is
  namespaced to its document's volume; a reference whose label the volume does not define is left as
  written and a finding. A blank line in a maths body becomes a newline, since maths mode cannot
  contain a paragraph break.
- A plain link to an exported document (`[text](x.md#frag)`) → a link to that document's heading,
  which the writer prints as `\hyperref`; to a copied asset → the copy; an external link passes; any
  other internal link → its text and a finding. A link whose content contains a reference anchor →
  its content alone, the anchor naming the target. An image is pointed at its copy under `assets/`;
  one naming no copied file becomes its alternative text and a finding.
- A labelled blockquote (its first paragraph a label line, `**Name**`, optionally inside a label
  span pair, with `Name` in `theorems`) is unwrapped; see [Theorem blocks](#theorem-blocks). Any
  other blockquote stays a quote.
- Headings are shifted by their document's rank: the document's own heading to level rank + 1 (a
  volume index, rank 0, is the chapter), in-document headings below it by their offset. Past level 6
  → a bold paragraph and a finding, a document's id kept as a label. Every heading id that is not a
  document's is removed, so only `vol{N}` and `d{N}-…` ids remain.
- A figure of more than nine blocks → its blocks one under another: pandoc 3.12.1's writer sets n
  blocks side by side at 0.9/n of the line and prints a width under 0.1 as `9.0e-2`, which LaTeX
  cannot parse. A figure containing a table → the table's body and caption unfloated, and a finding,
  since the writer drops a figure's float, caption and label around a table.
- A table whose column widths are all the writer's default, and whose columns' longest cells (a
  formula counted as its TeX less markup) with the gaps between them run past the template's line,
  taken as 96 characters → relative widths, so its cells wrap. A column whose longest cell fits an
  equal share of the width left keeps that width; the others share the rest by their longest cells,
  none narrower than its longest word and two fifths (a bold word, alone in its cell, cannot break).
  A table that fits keeps the writer's natural columns; one that has widths keeps them.
- Display maths that is one `eqnarray*`, `xalignat`, `xalignat*` or `xxalignat` environment → raw
  LaTeX: pandoc 3.12.1's writer wraps those, alone of LaTeX's and amsmath's display environments, in
  `\[…\]`. A maths body whose last line contains an unescaped `%` ends with a newline, since the
  writer closes the maths on that line and its own guard parses `\\%` as escaped.
- Raw html no rule claimed is a finding where it has `data-reference`, `data-cites` or an `id`, or
  is a `figure`, `table`, `math`, `img` or `embed` that did not parse as a native element: the writer
  drops raw html.

The writer does the rest unaided: display environments bare (but those above), inner ones
(`pmatrix`, `aligned`, `cases`) in `\[…\]`, inline maths in `\(…\)`; markers (raw html comments)
dropped. Prose is not matched: `(eq:L)`, `(Author, Year)` and a hand-maintained KB's bold-line
theorem blocks export as written, and `\tag{L}` stays, so the printed tag and the prose that names
it agree; such a KB's internal links resolve through the link rule and its work records form the
bibliography.

**The end.** `thebibliography` over the cited keys in first-emission order, each body rendered with
`pandoc.write(pandoc.read(body, "gfm"), "latex")`; the `\kbasetheorem` declarations
([Theorem blocks](#theorem-blocks)) in `header-includes`; and the findings, a JSON list of
`{form, path, reason}` (`path` the document's kb-root-relative path), written to the tables'
`report` path.

#### Labels

Three families that cannot collide: volume `vol{N}`; document `d{N}-{path-slug}`
(`export.DocumentLabel`: the path below the volume directory less its extension, each segment
lowercased with runs outside `[a-z0-9]` one hyphen, joined by hyphens; an index slugs as its
directory name and `index`); author `v{N}-{label}`. Go mints the first two; `selectVolumes` numbers
them after `--volume` narrows the list. A document label another path already owns is a traversal
finding, and that document gets no id. The filter's `namespaced` writes the author form, and every
rule that writes a label or a reference calls it, so a raw `\label` in maths and an id the writer
prints agree: it keeps ASCII letters, digits and `+-.:;=_` and writes any other character as
`ux{hex}`, whatever the label's own shape.

#### Theorem blocks

A labelled blockquote whose first body paragraph opens with its printed number, `**Display N**.` or
`**Display N** (Title).`, becomes `\begin{kb-<name>}[{Title}]\label{v{N}-L} … \end{kb-<name>}`.
`<name>` is the label line's name lowercased with spaces as hyphens; the opening is dropped and its
title kept. A bare-number opening takes the label line's name for `Display`. For each environment
used, `\kbasetheorem{kb-<name>}{Display}{Plural}` is added to `header-includes`, `Plural` by an
English rule in the filter; the `kb-` prefix is no LaTeX environment's, so the declaration is
unconditional.

Without that opening (a proof, which prints its own head and end mark; a LaTeX environment's name
such as `minipage`) the body blocks remain as they are, unquoted, and the label span's id is kept as
a label.

The template's `\kbasetheorem` owns the rest: the `definition` style, since the statements have
their own emphasis; one counter shared by every kind, numbered within the chapter; and the cleveref
names.

#### Template

`book`, `oneside` and `openany`, so every page is laid out alike and no blank page precedes a
chapter. Options for `hyperref`, `url`, `cleveref` and `geometry` (`margin=1in`, on LaTeX's default
letter paper) are passed before the class, so that an author's own load of them does not clash and
options the preamble passes them take effect.

Order of the preamble:

1. engine-branched fonts (`iftex`);
2. the maths packages, `amsthm` and `\kbasetheorem`;
3. the Unicode mappings for Greek, arrows, relations and sub- and superscript digits;
4. the writer-conditional blocks (tables, highlighting, strikeout and the rest, switched by
   pandoc's template variables);
5. `\InputIfFileExists{../preamble.tex}`;
6. `geometry`, `microtype`, `hyperref`, `bookmark`, `cleveref` last;
7. the `header-includes`.

The preamble's path is relative to the working directory, so the author compiles `main.tex` from
inside `gen-tex/`; where `../preamble.tex` is absent from the working directory, the template types
a notice and loads nothing.

The rest: mixed-case running heads naming the chapter on every page; the first numbered chapter
opens the main matter, everything before it being front matter, the title page its unnumbered page i
and the contents following from ii; `\backmatter` before the bibliography; and one generation
comment naming kbase's version (`kbase-version`, set by the verb in `defaults.yaml`'s `variables`).

#### `gen-tex/` and the preamble

`writeOut` owns `gen-tex/`. A `gen-tex/` containing files and no `kbase-export.yaml` is an
`ExportDirError` (the `export-dir` refusal) and nothing is written. The manifest lists every file
the export owns (the assembled files plus `main.tex`, `kbase-export-findings.json` and the report)
and is written after the files it names. A file whose bytes are already there is not rewritten; a
file the previous manifest listed and this export does not own is removed, with the directories the
removal empties; each manifest path is checked to lie inside `gen-tex/` before any removal.

The starter `preamble.tex` is written beside `gen-tex/` only where nothing exists at that name and
is never opened afterwards; the report checks its existence only. These are the only writes outside
`gen-tex/`.

#### Idempotence

Assembly is deterministic in the KB and the kbase version, so a repeat export reproduces every
assembled file byte for byte and `writeOut` writes none. pandoc rewrites `main.tex` and the
findings each run, so `export.Run` compares their bytes before and after the defaults run; the
outcome is `unchanged` when nothing was written, removed or rewritten with different bytes, the
report included.

#### The report

`kbase-export-report.yaml` is one YAML list: the traversal's findings (`export.Report`: a target
not walked, a path named twice, a label collision), then the filter's, then, where `preamble.tex`
exists, a line naming it as the author's. `writeReport` reads the filter's findings back from
`kbase-export-findings.json`, so the report exists only after pandoc has run.

## 6. KB model and maintenance

Each package and the subsection that states its mechanism.

| Package | Responsibility | Section |
|---|---|---|
| `internal/kb` | kb-root model, schema, record loader, link and marker readers | [§6](#the-kb-package) |
| `internal/write` | every write, as a batch of ops | [§6](#write-api) |
| `internal/index` | the rows `Derive` yields, strength, summary pages, `verify` checks | [§6](#the-derived-layer) |
| `internal/query` | read queries over the rows `Derive` yields | [§6](#the-query-layer) |
| `internal/sheet` | claim-graph sheets | [§6](#sheet) |
| `internal/kbdocs` | readiness documents and README assembly | below |
| `cmd/init.go` | KB creation | [§6](#scratch-creation) |
| `internal/score` | `kbase score` | [§6](#scoring) |

**Readiness documents (`internal/kbdocs`).** `Stamp` writes `AGENTS.md`, `CONVENTIONS.md` and the
`CLAUDE.md` redirect from `internal/kbdocs/templates/`, each only where absent, and refuses a
`CLAUDE.md` that is not the redirect before it writes. `Maintain` rewrites each such document's
maintained block to its template's: a document without exactly one ordered pair of fence lines is left
untouched and listed in `unfenced`, an absent `AGENTS.md` or `CONVENTIONS.md` is written whole, and
`README.md` is rewritten only where it exists. `AGENTS.md`'s scope pin is `AuthoredKBPin` where no build
record exists. `ComposeExcerpts` and `Overview` assemble the README's overview passage, and `NotProse`
checks the structure of a returned passage.

### The `kb` package

**Pattern:** Domain model and schema package.

`internal/kb` is the model of `kb-root/` that every other package reads through. It contains:

- **Names and paths.** The names readers key on (`KBDir`, `EntryPointFile`, `AxiomsFile`, the sheet
  file names) and the paths leaf discovery skips.
- **`Source`.** The read path for every KB file: the disk at the current format version, or the bytes a
  migration produced for an older one, with obsolete paths reading as absent. `Put` and `Removed`
  record a writer's change so later reads in the same command see it. `internal/kbload` builds it
  (§2, I8; [§11](#loading)).
- **Records.** The schema types, `LoadGraph` (the two record files read through `Source`, decoded by
  `format400`, a rigor above `RigorMaximum` refused) and `EncodeNodes` and `EncodeEdges`.
- **Schema.** The node kinds, relation names, id prefixes, `BearingLegal`, `MintID`, the band ladder
  (`StrengthBandLadder`, `BandFor`), `AxiomRigor` and `DocumentKind`, which reads a document's kind
  from its child list (`ChildList`, `ChildListLines`).
- **Scoring faults.** The fault ids, the three weight tables and `FaultTable.Price` (`faults.go`).
- **Row types.** `NodeRow`, `EdgeRow` and `CiteRow`, which `index.Derive` yields
  (`indexstream.go`).
- **Link and text readers.** The link grammar (`LinkRE`, `Links`, `Citation`, `BareIDs`) is the one
  pattern the dead-link check, the tree's link readers and the converter read links through. The
  readers over a document's text:
  - `TextLinks`: every inline link outside code, with its line and its offsets on the raw line;
  - `TextLinkDefinitions`: every reference definition outside code, with its destination's offsets;
  - `TextCitations`: the `TextLinks` whose title is a node id;
  - `CitingDocuments`: every Markdown document but those at a summary page position. `citations-of`,
    `remove-node`'s refusal, the edits' renames and every op's re-pointing share this one reading
    (SPEC §8);
  - `CitationKeyRefs`: each use of a citation key as a key, outside code, with a blockquote's
    prefixes read past: a citation span's `data-cites` token or key-only rendering, a reference-list
    entry's id, a link's `#ref-` fragment. A work's label edit and a work's removal read it.

  `BibliographyEntries` returns a document's rendered reference-list entries. The dead-link check
  (`DeadLinks`, `DeadLinksIn`) reads links through the same grammar. `HeadingAnchors` computes every heading's anchor through the slugger (`Slugify`, `Slugger`).
- **Statements.** The marker reader and writer (`MarkerLines`, `StatementMarker`), the hosts it derives
  (`Hosts`), a hosted node's statement as the layout around its marker gives it (`Statement`),
  `FirstHeading`, `AxiomHeadings` and `LabelLine`.
- **Summary pages' rule.** The file name, title and entry headings (`RegisterFile`, `SummaryTitle`,
  `SummaryEntryHeadings`), the volumes among a KB's documents (`Volumes`), the page a host's entry is
  on (`SummaryPage`) and every path a page can be at (`SummaryPagePositions`: kb-root's and each volume's), which `internal/index`
  renders by, the build rebases titles by and the converter predicts. The renderers of the derived lines
  (`RenderStrengthLine`, `RenderBearingAnnotation`) are in `render.go`.
- **Format stamp.** `FormatFile`, `EncodeFormatStamp`, and `FormatStamp`, the superseded stamp in the
  entry point's frontmatter.
- **The "no node" refusal.** `NoNode`, `NodeLister` and `CheckUnknownID`, the single source of its
  text and the query its remedy points at.

`format400` defines the live format's declarations (node kinds, relations, id prefixes, the link
grammar, the marker's format, the slugger, the summary pages' rule) and `kb` re-exports them under
the same names (SPEC §10). Outside the formats tree only `kb` and `internal/kbload` import
`internal/formats`, and `kb` imports only `log`, `argmark` and `format400`; the dependency rules
enforce both ([§11](#dependency-rules)).

### Write API

**Pattern:** All-or-none validated batch with dry run and per-file atomic writes.

Every write is a batch (SPEC §8 Write batches), run by `write.Run`: `write.Batch` for `kbase write`'s
mixed batch, an op's name for a batch of that op alone. `Run` takes no lock; its caller holds the KB
write lock across it ([§8](#locks)). A batch reads the KB, applies every entry in memory, rebuilds the
derived views in memory, then writes each changed file once. The phases:

1. *Shape.* `parseValues` checks every entry against its op's fixed key vocabulary, `opFields` in
   `values.go`, the single source of per-op keys; `OpVocabulary` hands it to the help text and the MCP
   tool schema. A mixed entry's op is read from its `operation` key and must be in `Ops()`. Each op's
   own checks over its entries follow (`batchChecks`; an edit's `refuseFieldless`). Every fault is
   returned at once.
2. *Apply.* `Run` opens the KB through `kbload.Open`, reads it (`index.Derive` reads every document and
   both record files) and refuses what `Derive`, `index.PendingConversion` and `index.Precheck`
   refuse. `applyEntries` then
   loads both record files through `kb.LoadGraph`, keeping the bytes the disk held, and applies each
   entry in order to the batch's `kb.Graph` and to the leaves it marks (`leafEdit`), each judged
   against what earlier entries left. The first refusal stops the batch and names its entry. An
   entry's outcome is `done` or `unchanged` by whether the rendered record files or leaves changed under
   it. Facts only known while an op applies (a minted id, `Restored`, `RemovedEdges`, a citation's
   document, line and link) travel on `entryReport`, which `applyEntries` copies into `EntryResult`;
   the names an entry's values give are read from the values afterward, and so are an edge entry's
   `source`, `target` and `relation` once it has applied. The record files are rendered
   whole through `kb.EncodeEdges` and `kb.EncodeNodes`, and a file whose bytes did not move is no
   change. That the decoder reads back what the encoder wrote is `format400`'s round-trip unit test,
   not a runtime check. `readBackMarkers` refuses a leaf whose composed text does not contain each marker
   the batch placed in it.
3. *Rebuild the derived views.* The changed record files and leaves go into the `kb.Source` overlay,
   unwritten, and `index.PlanDerivation` computes the summary pages and sheets from it, writing
   nothing. Every batch does this, the build's and the scoring run's own writes included (SPEC §8
   Writes leave nothing stale); no option skips it. The one exception is a batch of a build-only op
   that leaves only cycles among the scoring edges ([§6](#build-only-ops)).
4. *Write.* `writeChanges` writes `edges.yaml`, `nodes.yaml`, the leaves, then the derived files, each
   temp file then rename (`index.WriteKBFile`, `atomicfile`), and removes what the rebuild drops.
   A dry run (SPEC §8 Write batches) gives the same result with nothing written.

A failure in any phase before the write writes nothing. The write is atomic per file and not as a set:
a failure part-way is `failed`, naming what was already written (`interruptedError`), and recovery is git
(SPEC §6 Git holds the recoverable record).

**Pattern:** Optimistic concurrency check (compare before write) layered on a lock. Before the write,
`stillMatches` compares each file with the bytes the batch read; a difference is `retry`
(`RetryRemedy`) with nothing written. For a committing batch the comparison runs again once
`Preflight` returns, so a hook that edited a file meanwhile turns the batch into `retry`.

A committing batch (the document's `commit`, or `write.Options.Commit`; SPEC §8 Write ops) calls the
caller's `write.Options.Preflight` before its first write, and its error stops the batch with nothing
written. A committing dry run calls it too, which opens the repository and commits nothing.
`cmd/write_op.go` supplies it: `kbCommits.preflight` makes the snapshot commits of `kb-root/`'s
uncommitted changes outside `.index/` (owned `kb-root` less `gitledger.Exclude("kb-root/.index")`),
then those inside it, each only where `Repo.Dirty` finds any. Once `write.Run` returns `done`,
`kbCommits.final` commits `kb-root/` under the label `batchLabel` composes from the entries' ops,
their ids (`EntryResult.ID`) and the ids their values name (`EntryResult.Names`). Each commit's
`paths` are what `Repo.Paths` reads back from it.

The remaining mechanisms are per op family. No op accepts a preformatted record or marker: every value
is a values key, and the marker is `kb.StatementMarker`'s. `kb.Bearing` keeps `Value` and `Pending`
apart, so a written `0` reads back as `0` and a written `*pending*` as pending. An op is one per node
kind for `add-<kind>` and `edit-<kind>`, whose keys differ by kind in `opFields` (an experiment's
`status`, a work's and an axiom's `label`); `remove-node`, `move-node` and `add-citation` take any
kind by `id`.

#### Adds

An add fails if the item exists (SPEC §8 Idempotence) and upserts nothing. Against the records as the
batch's earlier entries left them, it refuses a title its kind already has (`titleHolder`) and, for a
work or an axiom, a label its kind already has (`labelHolder`); then it mints the id (`kb.MintID`) for
every kind. A
hosted node's marker is placed through `markAtLine` once `checkNamedLine` has refused a line past the
end, a fence's opening or closing line, and a line without the snippet.

A hosted add and `add-work` run through `keepSummaryLinks`, which updates references to every summary
entry whose repeat suffix the new entry shifts, comparing `anchored` before and after. `summaryMayShift`
first rules out, without a rebuild, an entry whose slug (repeat suffixes stripped) no other title and
no fixed rigor or strength heading shares, so a build's many adds rebuild nothing extra.

`add-axiom` marks the one unmarked heading of its text in `axioms.md`, or appends its marked heading,
and refuses two unmarked headings of its text. An appended heading goes after every heading of
`axioms.md` and shifts no summary link, so `add-axiom` skips `keepSummaryLinks`. `axioms.md` travels
with the leaves as a `leafEdit`.

#### Edits and renames

An edit (`edit-<kind>`) replaces the fields its entry gives, through `nodesOf`, which presents each
kind's records as their editable fields. Title uniqueness is judged by `titleHolder` and label
uniqueness by `labelHolder`, as in an add.

**Pattern:** Referential-integrity maintenance (rename refactoring). A title or label change
(`rename`) keeps every link into the node valid in the same batch. It reads each work's and axiom's
link target and every node's summary entry before and after the change, from `index.Derive` over a
source holding the batch's records and documents as earlier entries left them (`anchored`, one rebuild
each side). It then rewrites every document but the summary pages (`eachDocument`: `kb.CitingDocuments`
and each document an earlier entry created), each document it changes a `leafEdit`.

- `edit-axiom` rewrites its marked heading in `axioms.md` (`rewriteAxiomHeading`).
- A citation in the naming form takes the new title through `renameCitations`, which refuses at
  `title`, naming the citing `path:line`, a rewritten link that `readsAsOneLink` does not see whole.
- A work's new label replaces every use of the old key as a key (`rekey`, over
  `kb.CitationKeyRefs`), after `refuseKeyInUse` refuses a label that a citation span or a
  reference-list entry already uses.
- These rewrites change lines in place, so `movedHeadings` finds each heading whose anchor changed by
  comparing `kb.HeadingAnchors` line by line, before and after, and `movedAnchors.summaries` adds each
  summary entry whose anchor changed. `repoint` then gives every link outside code, citation or not,
  inline link or reference definition, one destination computed from the one it had before: the
  node's new target where a citation of it pointed at the old, otherwise the changed heading's or
  entry's new anchor, the fragment alone changing. The map is read once per link, so no link is
  updated twice and a shifted repeat suffix (`on-x-1` to `on-x`) is never chained.

#### Edge ops

`add-edge`, `edit-edge`, `remove-edge` and `restore-demoted` look an edge's ends up in the batch's
records (`edgeEnds`, refusing through `noNode`, the "no node" refusal the edits share) and its
identity through `holds` and `noEdge`. `Joins` decides whether a relation may join two kinds:
`references` any, a scoring relation what `kb.BearingLegal` allows, `demoted` none.

**Pattern:** DAG insert with cycle check (path finder). `index.GraphDependsGraph` builds the premise
graph over the scoring edges, and `index.DependsGraph.Path` is a shortest-path search over it.
`closesCycle` runs the search for every public op that adds a scoring edge and for `restore-demoted`,
and refuses an edge it finds closing a cycle, naming the path in edge direction (it reverses the
premise graph's path). The premise graph is built once per batch and cleared by every op that takes a
scoring edge out of the records or changes one (`dropEdges`, `demoteEdge`, `splitClaim`). `addEdge` deletes the `demoted` record (`kb.DemotedMatch`)
of a scoring edge it writes, so an add over a removed edge writes the edge with the add's values.

A scoring edge's judgement is written by `add-edge` and `edit-edge` through `bearing`, and a node's
local value by `add-<kind>` and `edit-<kind>` through `rigor` (SPEC §8 Values input).

#### Removal and move

`remove-node` reads the rows of the edges it removes from `derived` (`index.Derive` over the batch's
records and documents, which `targetOf` shares), drops them through `dropEdges`, and takes the record
and the marker out through `dropNode` inside `keepSummaryLinks`, or an axiom's headings out of
`axioms.md` through `dropAxiomHeadings`, which updates references to every later heading whose repeat
suffix the removal lowered. It then refuses, through `refuseMentions`, a mention that outlives the
node:

- a citation in any document but the summary pages (`eachDocument`, shared with `renameCitations`,
  `rekey` and `repoint`);
- for a work, a citation span with its label as the key where no reference-list entry has that key;
- a surviving record's prose field (`graphProse`, `Mentions`).

`move-node` checks the named line through `checkNamedLine`, places the marker with `appendMarker`,
takes every other occurrence out (`unmarkEverywhere`, keeping the named line), and updates every
citation, through `repoint`, to `targetOf`: a hosted node's host and its marker's section anchor
(`index.SectionSlug`, the slug the rebuild of the derived views uses).

A document the batch edits is a leaf as `index.Derive` reads it, by its child list (`leafAt`), so a
citation added to a leaf leaves it a leaf.

#### Citations

`add-citation` opens any document of the KB as a `leafEdit` (`citingDocument`), finds the one span of
the named line its snippet names outside code, links and markers (`citedSpan`; a missing span is
refused through `snippetAbsent`, which `checkNamedLine` shares), and links it to `targetOf`. It refuses
a link the reader would not see whole (`readsAsOneLink`) and checks a quote against `kb.Statement`,
the cited node's statement read from its host leaf by the layout around its marker, the `lifts` edges
the batch contains deciding the split layouts. A link spliced into a heading changes its anchor, so
`add-citation` runs `movedHeadings` and `repoint` after the splice, the new link among those updated.

### Provenance

**Pattern:** Manual-override flag.

`write.Options.Provenance` says who supplied the values a call writes and decides the `manually-scored`
flag each rigor and bearing is written with (SPEC §10 Provenance and the judged hash). `ProvenanceCaller`,
the zero value, sets the flag on every value its caller supplied, so an internal caller that names
none never lets a run overwrite a person's value. `ProvenanceMachine` writes `false`. The writers that use
it are `claimgraph`'s write runner, `internal/score`'s writing of results (`landValues`, a score run
and a recompute alike; [§6](#writing-the-results)) and the build-only `add-build-edges` and
`split-claim`, each a `recordEdit` marked `machine`.

Provenance is an options field and never a values key, a flag or a schema property:
`TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) asserts that no command flag and no
tool property names it.

A rigor or bearing that an add or an edit is given records the judged hash of the text as the entry
leaves it (`valueSet`, `hashValuesSet`, over `asks.NodeJudgedHash` and `asks.EdgeJudgedHash`). A
pending value set by a machine records none.

No build path writes a judgement (SPEC §5 A build records no judgement): `add-build-edges` writes
`*pending*` on every edge it adds and no number anywhere, and `split-claim` writes `*pending*` on the
analysis's rigor and on its lift.

### Build-only ops

**Pattern:** Internal privileged ops.

Three write ops are the build's alone: `add-build-edges` (`write.AddBuildEdges`), `demote-claim-edge`
(`write.DemoteClaimEdge`) and `split-claim` (`write.SplitClaim`). They are absent from `write.Ops()`,
have no subcommand and are refused in a mixed batch, so the public write operations cannot create or
retarget a `demoted` record. `restore-demoted` is public, and `remove-edge` removes a `demoted` record.

- `add-build-edges` writes the build's edges as `add-edge` writes one, without its cycle refusal. A
  `rests-on` premise `founds` the node where `kb.BearingLegal` allows `founds` from its source and
  `supports` it otherwise; `references` are accepted on a claim only. An edge the records already
  contain is left as it is, and an edge with a `demoted` record of its relation is written with these
  values and the record deleted.
- `demote-claim-edge` replaces one standing scoring edge, named by its `source`, `target` and
  `relation`, with the `kb.DemotedEdge` that keeps it whole. Where only that record exists the edge
  is left as it is; an edge with neither is refused.
- `split-claim` handles one claim. Where an analysis already lifts the claim it uses that one;
  otherwise it mints an analysis and places its marker at a given line of a given leaf, and, where
  given a `statement-end`, relocates the claim's marker to that byte offset of line `statement-line`,
  or of the claim's own line where none is given. It retargets every `supports` and `founds` record
  into the claim, and every `demoted` record that replaced a `supports` edge into it, onto the
  analysis, and adds the analysis's `lifts` of the claim at `*pending*`. It runs no cycle check: a
  path through the analysis is the path that ran through the claim.

`add-build-edges` and `demote-claim-edge` run over records holding a cycle among the scoring edges
(`index.DeriveOverCycles`). The build's edge batch closes such a cycle and cycle breaking
([§6](#cycle-breaking)) removes it within the same stage: its removals reach the records through
`demote-claim-edge`, at the tail of `depends-attributed` (§4). `split-claim` is
applied by `analyses-split`. A batch of `add-build-edges` or `demote-claim-edge` that leaves records
the rebuild refuses for a cycle among the scoring edges alone (`holdsOnlyCycles` in `ops.go`) is
written without its rebuild of the derived views, because the rebuild cannot read such records; the
cycle-breaking batch after it rebuilds them before the stage's boundary.

The build's other writes are `add-claim`, `add-work`, and `remove-node` and `edit-claim` for culls.
None of the build's own writes takes the KB write lock; the build takes it once, at start, to wait out
a write in progress ([§8](#locks)).

- **Culls.** A cull is one mixed batch of `remove-node` entries, ordered so that a culled claim whose
  rationale mentions another is removed first (`write.Mentions`, the test `remove-node` refuses by).
  Where culled claims mention each other round a loop, an `edit-claim` entry clears a rationale first
  (`cullEntries` in `internal/claimgraph/elision.go`).
- **Titles.** A title its kind already has takes a counter, ` (2)` on (`uniqueTitle` in
  `internal/claimgraph/write.go`). An `add-claim` entry whose title a claim in its document already
  has takes that claim's id and writes nothing.
- **Block claims.** A block claim given a counter does not meet its block by title in `readGraph`. It
  joins the block of its leaf whose display line its marker is on, where no claim of that leaf has
  the block's own title. That line is the one `add-claim` was given, found by the same
  `write.ExcerptLines` search within the block's extent: an exact, positional match, never the
  counter read back.

### The node vocabulary

**Novel.** What a claim graph is made of, THESIS "What is new" item 1: five node kinds and four
scoring relations, defined in SPEC §10 Five node types.

**Pattern:** Prefixed random ids.

`kb.NodeKinds` is five words, `claim`, `analysis`, `experiment`, `axiom` and `work`, and nothing else is
a `node_type` or a `target_kind`. `kb.BearingLegal(relation, sourceKind, targetKind)` is the single
source of whether a pairing may have a bearing: every scoring pairing may, `references` and `demoted`
may not. It answers that and never whether an edge may exist.

Every node id is minted: its kind's three-letter prefix over a body `kb.MintID` draws, `kb.HashLen`
characters uniform over `kb.HashAlphabet` and redrawn while taken. One grammar serves every kind
(`kb.IDBody`, over `kb.IDKinds`). Each prefix is a `format400` constant re-exported through `kb`
(`kb.ClaimPrefix`, `kb.AnalysisPrefix`, `kb.ExperimentPrefix`, `kb.WorkPrefix`, `kb.AxiomPrefix`);
`kb.NodeIDKind` reads an id's kind off them, and `kb.NodeIDs` finds an id of any kind in text. A KB
converted from `3.0.0` has its work and axiom ids minted by the migration ([§12](#format-migration-spec-10)).

A work's citation key and an axiom's label live in the record's `label` field (`WorkNode.Label`,
`AxiomNode.Label`), never in the id; the strict load requires a work's label to match `kb.WorkLabelPattern`.
An axiom's prose is `axioms.md`, its heading paired with its record by the axiom's marker.
`kb.AxiomHeadings` reads each heading containing an axiom's marker, and each unmarked `### Axiom `
heading, for `add-axiom`, `edit-axiom`, `remove-node` and `verify`'s `axiom-heading` check.
`kb.AxiomAnchor` gives `index.Derive` an axiom's anchor, that of its marked heading. A statement marker
hosts only a claim, an analysis or an experiment (`kb.HostedIDKinds`); an axiom's or a work's id in a
marker hosts nothing.

The legacy declaration forms are the superseded readers' alone (`internal/formats`). SPEC §10 is the
single source of the prose legend the authors' words map through.

### The derived layer

**Novel.** The formula that combines local judgments into a strength for every node, THESIS "What is
new" item 3, defined in SPEC §10 Four scoring relations, one arithmetic.

**Pattern:** Topological fold over a DAG (Kahn's order); materialized views rebuilt after each write.

#### Strength

`internal/index/strength.go` is the arithmetic's authority: the four relations, their three stations,
the pending rule and the value's two formats live there. [STRENGTH_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/strength/entry-point.md)
states the relations, the formula and its worked examples and is cited, not reproduced.

| Station | Relations | Function |
|---|---|---|
| minimum | `supports`, `founds` | `MinDependencyStrength` |
| maximum | `lifts` | `liftStrength` |
| cap | `refutes` | `capStrength` |

`EdgeContribution` is one edge's value, nil where its bearing or its source's strength is pending. Each
station function reduces it over its edges, omitting an excluded one; `ExperimentTakesPart` excludes
an experiment that is not `run`. `derive` composes one node's result. `graphStrength` fills a
`strengthWalk` from the records and `run` computes each claim into `Strength.Results` and each
analysis into `Strength.AnalysisResults`, in `derivationOrder`, which is Kahn's order: a node after
every node its premises name, from sources to sinks. Nodes a cycle keeps out of that order, and nodes
resting on one, derive nothing; the load refuses such records ([§6](#cycle-breaking)).
`RenderStrengthTrace` composes the station text for a trace. The `trace` query (SPEC §8 `trace`)
reports `EdgeContribution` per edge, `internal/query` reading each source's strength off `Derive`'s
node rows and leaving an experiment that is not `run` out as the propagation does.

The propagation computes at full precision, so a dependent multiplies its source's exact strength, and
nothing else rounds a strength. `FormatStrengthRecord` writes the value in the fewest decimals that
read back as it (a summary page's strength line and premise annotation), and a query result contains
the value `FormatStrengthDisplay` formats, where a trace shows it.

#### Derived views

`index.Derive` yields the rows in memory (`Rows`) from the records and the statement markers in the
leaves. The summary pages and sheets are views regenerated from them after every write and checked for
staleness.

- **Rebuild.** `renderRegisterPages` renders every summary page from the records and markers, never
  from a page already on disk. `Refresh` (load, pages, sheet) writes only the files whose bytes change
  and removes a page the KB does not call for. `PlanDerivation` is the same computation without writing,
  for a write batch. `UpdateFormat` is the save of a converted KB ([§12](#format-migration-spec-10)).
- **Anchors.** A summary page's entry anchors are computed at derive (`pageAnchors`): the page's title
  and every entry's headings (`kb.SummaryEntryHeadings`) through one `kb.Slugger` in page order. A
  claim's and a work's canonical anchor, a page's links and the dead-link check's page slugs read
  that one map, and a test holds the rendered page's headings to it.
- **Staleness.** `Verify` is the standard check: `LinkFindings`, `BareIDFindings`, then
  `MetadataFindings` (a frontmatter block, a KB loaded at an older format as `index-freshness`,
  statement-marker faults, disagreements between axiom records and `axioms.md`, and
  `RegisterFreshness`, a finding per summary page that is missing, differs, or has no node rendered on
  it). The build's `phase-3a` runs `Refresh` and `Verify` (`internal/claimgraph/gate.go`).
- **Remedies.** Each `Finding` has its own: `update-format` for `index-freshness`; the empty write
  batch (`EmptyBatchRemedy`) for `register-freshness`, but `update-format` over a KB whose conversion is
  pending, where the batch is refused; a write batch of an `add-citation` entry for `bare-id`.
- **Links.** `LinkFindings` checks a citation's title id among the ids it checks (`kb.CheckUnknownID`).
  It hands `kb.DeadLinksIn` the summary pages' derived slugs, which turns the fragment check on: a
  fragment into a summary page is judged against those slugs, one into any other Markdown document
  under `kb-root/` against its heading anchors and element ids (`documentAnchors`), read once per
  document. `kb.DeadLinks`, which the build's document-graph and conformance checks call, checks files
  alone.

### Cycle breaking

**Pattern:** Greedy feedback-arc removal by DFS back edges.

`index.BreakCycles(g, hosted)` returns the graph with no cycle among its scoring edges, the edges it
removed in order, and a warning per removal. Each removed edge becomes its `kb.DemotedEdge`. SPEC §10
Cycles: cut as the walk finds them defines the rule; the mechanism is:

- The node order is `hosted` (`index.HostedOrder`, the order the summary pages render nodes) and then
  every other node by id; an edge sorts by its relation's place in `kb.Relations`, then its source's,
  then its target's.
- The search is depth-first over the `supports`, `founds`, `lifts` and `refutes` records, from every
  node in turn, following each node's edges in that order (`closeCycle`); `demoted` records are not
  followed.
- An edge reaching a node on the current path closes a cycle. The edge removed is the first, in that
  order, of the cycle's `supports` edges into an analysis, else the closing edge.
- After each removal the search starts over, so the removals depend on the edge set and the node
  order alone.
- The removal set is not a minimum feedback arc set.

Three callers use it:

- `Derive` runs it over every load and only detects: a cycle in loaded records is a load error, one
  item per edge the search would remove (`deriveGraph`, `loadDerivation`).
- `DeriveOverCycles` accepts such records and computes strength over `Rows.Resolved`, the graph with
  those edges removed.
- The build's cycle breaking (`ringRow` in `internal/claimgraph/stages.go`) takes the removals over
  `index.HostedOrder` and applies them in one `demote-claim-edge` batch, in removal order, because the
  search computes every removal over one graph before any is applied.

Public ops refuse an edge that would close a cycle ([§6](#edge-ops)), so only the build breaks cycles.

### The query layer

**Pattern:** Read model.

`query.Load` is the rows `index.Derive` yields, as an `Index` sorted by id; `Derive`'s refusals are
`Load`'s, and a KB at an older format answers as converted. Queries read the `Index` and nothing else.
`query.ScoringSet` builds the scoring set ([§6](#the-scoring-set)).

`query.EdgesPayload` is the single incidence adapter. It selects the `Index`'s edge rows by relation and,
where `EdgesSelection` names a node, by `Direction` (`in` the rows whose target it is, `out` those whose
source it is, else both), and presents each as `query.EdgeRowRecord`: `source`, `target`, `relation`,
`bearing` (a number or the pending literal, null only where the pairing has none), `context`,
`manually_scored` and `replaced_relation`. The order is fixed: relation, then source and target
(SPEC §8 `edges`).

An answer is the whole selection: no list query slices it, and `find` and `trace` truncate only where
`--limit` is given beside `--sort` (`rankUsage` refuses a limit without a sort). No query verb returns
statement text: the rows contain none.

`contribution` in a `trace` row is `index.EdgeContribution`, never a restated formula (`tracedEdge`).

- **`find`.** `FindUsage` checks the selection's vocabulary values and `rankUsage` its sort; `Find`
  then filters the nodes, orders them by the sort key and cuts to the limit. `compareRanks` orders a
  pending value below every number (above every number when descending) and a null after every
  other value in either direction; ties break by id, and for `--sort dependents` by strength first.
- **`show`.** `ShowPayload` takes a node's host and section anchor from the host leaf's marker, read
  once per leaf at `Load` (`Rows.Hosts`, `Rows.HostAnchors`), and its summary entry from
  `Rows.RegisterPages` and `Rows.RegisterAnchors`.
- **`stats`.** Its ranked sections, `highest_leverage` and `weakest`, are `find` questions
  (`HighestLeverage`, `Weakest`), each cut to `query.StatsRankLimit` (§12).

### Frontmatter reading

**Pattern:** Heuristic sniffing before parse.

`kb.FindFrontmatter` takes a leading `---` block as frontmatter only when its first non-empty line is
key-shaped (`name:`, kebab-case). A block that then fails to parse as a YAML mapping is a
`MalformedError`, the malformed-frontmatter refusal; a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10 Recognising frontmatter).

The one frontmatter value kbase reads is the superseded stamp in the entry point (`kb.FormatStamp`),
which `internal/kbload` reads only where `kb.FormatFile` does not exist ([§11](#loading)). `verify`'s
`frontmatter` check uses the same recogniser to report a block on any document, the entry point
included. The build's conformance check (`claimgraph`) refuses any document whose frontmatter has a
field.

### Sheet

**Pattern:** Graphviz rendering with a bounded process pool; a materialized view.

`internal/sheet` emits Graphviz DOT and renders it by running `dot -Tsvg` on `PATH`. The SVG's bytes are
Graphviz's, so the goldens pin the DOT. `TestRenderDraws` renders through `dot`, skips without it, and
asserts only that each root is fitted to the viewer; `TestDOTQuoting` asserts, where `dot` is on
`PATH`, that the DOT it composes parses through `dot`. The goldens are a fixture and its three DOT files under
`internal/sheet/testdata/adjagent-sheet/`: a `3.0.0` KB and the DOT the sheet composes for it, with a
build record beside it that the sheet does not read. `TestAdjagentSheetGoldens` compares the emitted
DOT with them byte for byte.

The inputs are the rows `index.Derive` yields (nodes, edges, strength bands, cites), which `index`'s `planSheet` hands
it from `index.Derive`, never a read of `.index/`:

- the `demoted` rows, for the removed edges, and one style per relation;
- each node's `CanonicalPath` and `CanonicalAnchor`, for its link;
- the leaf, for a node's kind, with the title deciding where no marker is readable;
- the entry point's first heading, for the title and the cluster of nodes no volume hosts;
- each volume's `index.md` first heading, falling back to the directory name, for the volume's
  cluster title.

It reads no build record. Its non-test code imports `kb` alone today; the dependency rules only keep it
below the build ([§11](#dependency-rules)).

It produces the full sheet always, and the overview diagram and one sheet per volume where two or more
volumes contain nodes (SPEC §3 `claim-graph.svg`). It renders through a pool of `dot` processes bounded by
`sheet.renderConcurrency` (§12), keeping output order. `dot`'s spline router can fail
on a dense sheet, and straight lines bypass it, so the emitter then composes the same DOT with
straight-line splines and renders it once more; where both fail, the first error is reported. Without
`dot` it renders nothing, leaves existing sheets as they are, and yields the placeholder only for a root
that has no sheet.

Every rebuild of the derived views (`index.UpdateFormat`, every caller of `index.Refresh`: `init`,
`score`'s start rebuild, the build's `phase-3a`; and a write batch's `index.PlanDerivation`) reaches the
sheet through one interface, `index`'s `planSheet`, which renders every sheet on each call and yields
one only where its bytes change, with no ownership test on an existing sheet. A rebuild writes what it
yields; a batch writes it with its records. `verify` does not reach it.

### Scratch creation

**Pattern:** Scaffold command.

`cmd/init.go` is the only producer of a KB that is not the build. It composes existing mechanisms and
owns none of its own:

- the format stamp through `kb.EncodeFormatStamp`, the plain entry point, and the two empty record
  files through `kb.EncodeNodes` and `kb.EncodeEdges`, every file through `atomicfile.Write`;
- the loader's `Open` (§2, I8);
- the readiness documents through `kbdocs.Stamp` with `kbdocs.AuthoredKBPin` in its scope-pin slot, its
  `written` list being the paths `Stamp` returns;
- the derived views through `index.RefreshReporting`;
- the KB write lock through `holdKB`, which takes `write.LockKB` from before `kb-root/` exists
  ([§8](#locks));
- the create commit through `gitledger.Repo.Commit`, the plain-subject half of `gitledger` beside
  `Record`, its pathspec scoped the same so nothing the user staged rides along.

It refuses a multi-line `--title`, a directory outside a worktree and any existing `kb-root/` before
any write, and decides create-once again under the lock (SPEC §8 `init`). Its order is load-bearing:
the entry point and the stamp file exist before `Open`, the readiness documents before the rebuild,
and the rebuild before the commit. The commit's subject, `kb-init: knowledge base created`, is outside
`gitledger.Prefix`, so the KB has no checkpoint commits and a resume finds nothing to resume.

`init` runs no `verify` of its own: every input to the scaffold is kbase's, so a failing `verify` over
it is a kbase defect, which `test-integration-init` catches by running `verify` right after creation.

### Scoring

**Pattern:** Job runner with an append-only event log, resume from the last status per item, optimistic-concurrency commit, projection rebuild.

`kbase score` is defined in SPEC §8 `score`; this section states the mechanism. The packages that run it:

- `internal/score`: the run's entry, scope and stale check, the runner, the item text and its sections,
  the parse, the run's ledger and captures, run retention, the writing of results, the run's status
  reader and the development replay.
- `internal/kb`: the fault vocabulary (`kb.FaultTable`), the three weight tables with their value
  computation (`Price`), and the statement reader `kb.Statement`.
- `internal/asks`: the three templates, the check that each names its class's ids, and the judged hash
  (`asks.JudgedHash`).
- `internal/query`: the scoring set.
- `internal/write`: the edit ops a score run applies, the KB write lock, and the hash every value set
  records.
- `internal/statedir`: the state directory.
- `cmd/score.go`: the verb.

#### The runner

**Pattern:** Bounded worker pool over a job list, results written in one batch at the end.

`score.Runner` owns the item loop and no I/O of its own. It takes the enumerated items; the `Launch`
interface, which hands each item the `Ask` its one model call goes through; the `Write` interface; the
ledger; the model name the ledger lines record; and a logger. Because the item text is composed in this
package, it also takes the `kb.Source` and `query.Index` the renderers read, and the `graph` they
read, built once from them (`graphOf`); `Invoke` passes in the `graph` it built for the scope.

The pass is read once, before the loop, into its classes. Per item, one lookup of the item's class
yields both the instruction its call sends and the table its value is computed under, so the
composer, the parse's fixed set and the value computation all read that one fact. The launch loop
composes each item's prompt and launches the items in the set's order, at most as many at once as the
invocation's `model.Queue` has permits. Each launched item makes its call, parses the reply as the charges
it records against that item text, computes the value, keeps it in its own result cell and writes its
result file.
A text the input guard refuses, a reply the parse refuses, and a failed call or write each append
`failed` with nothing written, and the loop continues.

`prepare` returns the entry and the `call` the item will make: class, item text and prompt, or nil where
the class cannot be derived or the input guard refuses the text. `attempt` returns the entry and whether
the loop's stop interrupted the call. Nothing shared is written until the pool has drained (invariant
I11, §2); the ledger's `flush` then writes the cells' values, appends their captures,
then appends their lines, in the set's order. No reason for writing at the loop's end rather than per
item is written beyond that invariant. `Counts` comes from the ledger's last status per item, not from this
invocation's attempts, so a resumed run reports the whole run. A cancelled context stops the loop and
records nothing for an item without an answer; an interruption outranks a failure in the run's outcome.

#### The ask is one seam

**Pattern:** Dependency injection.

`Ask` takes the rendered prompt whole and returns a `score.Reply`: the reply's text and whether the
endpoint truncated it at the completion cap. The runner has no view of a provider. `cmd/importpolicy_test.go`
enforces that `internal/score` never imports `model` or `build`: `internal/score` imports `argmark`,
`asks`, `atomicfile`, `buildstore`, `kb`, `log`, `query`, `result` and `write`. A truncated reply is the
runner's to judge, not the `Ask`'s: `put` fails its item as it fails a call that returned an error,
whatever the text would parse to.

`scoringLaunch` in `cmd/score.go` fills `Launch`. It builds the invocation's one `model.Queue` and
returns a `Launch` that starts each item on a slot of it. The item's `Ask` makes one `Slot.Chat` call on
the scoring role's endpoint with an empty system prompt, applies `--timeout` from the call's start
(never over the wait for a permit), and reports `model.FinishLength` as the truncation. The `modelclient`
registry opens the client at the first call, so a run refused at its scope or holding no item connects
to nothing. A provider's refusal of the call's key is a `modelclient.KeyRefusedError`; `scoringLaunch`
turns it into a `score.RemedyError`, which the failed attempt's ledger line records. Retry and backoff
are the chat client's (§7); `scoringLaunch` supplies the pauses (`scoreBackoff`),
and the attempt count and pauses are in §12. The development replay takes its `Launch`
from the same function, so a replayed call carries the run's endpoint, timeout and truncation rule.

Each score template splices the `score-system` fragment into its own text, so the instruction is carried
once, in the user turn.

#### The prompt shelf

**Novel.** The fault criteria the templates carry realize THESIS item 2 (local judgments from a closed
list of faults), defined by the rigor and bearing scoring scheme and SPEC §8 The answer.

**Pattern:** Prompt templates with a lint test.

Three templates, one per class: `score-rigor-statement.tmpl.md`, `score-rigor-method.tmpl.md` and
`score-bearing.tmpl.md`. Each has one `dyn.` slot, which the runner fills with the assembled item text,
and splices four shared fragments (`fragmentSlots`): `fragments/score-system.tmpl.md`, the system
prompt; `score-disposition`, the class-neutral standard every score template judges by, which names
no id; and `score-answer-occurrences` and `score-answer-close`, the answer section's text, which all
three state alike so no vocabulary can drift from the parse or from its siblings. `asks.ScoreRigorStatement`,
`asks.ScoreRigorMethod` and `asks.ScoreBearing` render them through `asks.Render`; `asks.System`
serves the system fragments alone (`systemSlots`), never an answer fragment as a prompt. The templates
are embedded in the binary, written by kbase, with no upstream origin and no version pin.

The templates carry the criteria and the line form. They name no id the class does not accept, and no
figure or word a model could read as a judgement: no decimal in `[0,1]`, no percentage, no weight, no
maximum, no band's word. This is a deliberate blinding: the weights are `internal/kb`'s, the single
source of truth for them. A quantity of the subject's own, an example's rate or threshold,
is the claim's and stays.

Tests in `internal/asks` assert that each template declares one `dyn.` slot, fills it with the text the
runner composed, splices the shared fragments, names exactly its class's ids (`flt-[a-z0-9-]+` tokens
against `kb.RigorStatement.IDs()`, `kb.RigorMethod.IDs()` or `kb.BearingFaults.IDs()`), carries none of
those judgement forms, and keeps the instruction fixed around a subject of any size. `asks.Lint`
checks every file on the shelf for a delimiter that does not parse as a slot, and for any phrase its
caller prohibits; the tests prohibit none.

#### The closed text and its guard

**Pattern:** Line-numbered context for grounded citation; delimiter-based prompt-injection guard.

`internal/score/text.go`'s two unexported renderers assemble every character a call may see.

`rigorText` renders a node's title and its statement. The title section is read through the node's
canonical path (`src.KBPath(row.CanonicalPath)`). The statement is `kb.Statement`'s reading of the node
in the leaf hosting it, the same reader `add-citation`'s quote check uses. The run's `graph` contains it
for every hosted node, read once per leaf by `graphOf` with the `lifts` edges of the in-memory index,
markers and `>` prefixes removed, with the 1-based line of the leaf it opens on (`statementLine`: the
line nearest the node's marker whose text, markers and `>` prefixes removed, begins with the statement's
first line). A node no leaf hosts (a work, an axiom, a node with no marker) renders its title alone.
`bearingText` renders the source's and the target's sections by that same rule, then the edge's authored
`context` from the row `Index.Edge(ScoringEdge)` answers. Each end's label opens a line of its own, as
does the context's, so no numbered line begins with template-wrapper text.

`assemble` wraps every section of the item text at word boundaries to `wrapWidth` (§12),
keeping the section's own line breaks and rewriting nothing, and numbers the lines lying in a section
that a charge may cite, from 1 over the whole text. `closedText.numbered` writes `N| ` before a numbered
line; the gutter is the template wrapper's, not part of the text the call sends. A label line, a blank
separator line and the edge's context are written bare: a line with no number is no site.

**Pattern:** Delimiter-based prompt-injection guard. `scoreTokenRE` and `frameRE` scan each assembled
section; no guard pattern spans the blank line between two. A hit is `scoreTokenError`, naming the
document and line, and the item is `failed` with no call made (SPEC §8 One item, one quantity, one call).
The input guard refuses and never strips or escapes, because a rendered prompt stays a function of its
template. `scoreTokenRE` matches the four scoring tokens. `frameRE` matches the template wrapper's own
structure: a run of twelve backticks closes the fence the templates draw around the item text, after
which the text sits outside the instruction to judge it; a line opening `## Source:`, `## Target:` or
`## Context` forges the wrapper's map of the text. A label that a blockquote or an indent displaces from
the line's start opens nothing. An out-of-scope subject is refused by name and type: a `wrk-` or `axm-`
id on `rigorText`, a `references` or `demoted` relation on `bearingText`.

#### The parse resolves the numbered lines

**Pattern:** Structured-output validation against a fixed grammar.

`assemble` returns the item text and one `line` per line of it: the number the call writes, the section
its bytes lie in, and its byte range. A charge's `passage`, `lines` and `offset` are facts about that
rendering. A `rigor` call's two sections are a node's `title` and `statement`; a `bearing` call's are the
`source`'s and the `target`'s. `score.ParseAnswer` (`internal/score/answer.go`) reads a reply against
those lines and the item's class's `kb.FaultTable`. A charge line's field is one number or a range
`N-M`, which `closedText.look` resolves; a range that crosses sections, a blank line, a label or a
heading is `malformed`. The recorded `citation` is the cited lines as the call sent them, without the
gutter, joined by newlines, and `offset` is that quote's byte offset in the item text. The edge's
`## Context` has no number and is a site for no charge, so the `passage` value `both` is never produced.
Grammar, duplicates and overlaps: SPEC §8 The answer.

The three weight tables are `internal/kb`'s: `kb.RigorStatement`, `kb.RigorMethod` and
`kb.BearingFaults`, each a `kb.FaultTable` of ids and per-occurrence weights in integer hundredths, with
`IDs`, `Accepts` and `Price`. The parse yields `Answer{Faults, Moot, Value}`: the charges, `Moot` (charge
lines recorded beside a `flt-pending`, which no table holds; `ParseAnswer` produces none, and a recorded
line may carry them), and the `Price` of the ids. `Price` of a recorded line carrying `flt-pending` is
`*pending*`. The value is defined in SPEC §8 The value. The ids a line records are the whole input of the
value, which makes a written value checkable and a run recomputable without a model.

The grammar is fixed for as long as the ledger's format version is unchanged: a rule loosened later
would re-read lines already written under a new grammar (SPEC §8 The answer).

#### The scoring set

**Pattern:** Input-hash staleness, as in make.

`query.ScoringSet(ix, pass)` reads the in-memory index in-package and returns `[]query.Item`, each
holding a `ScoringNode` or a `ScoringEdge` and never both, in the loop's order. `query.PassRigor`,
`query.PassBearing` and `query.PassAll` are the three values the set and the `--pass` flag share
(`query.Passes`); `PassAll` is the node set and then the edge set, each in its own order. An item
contains identity only, so an edge's row comes from `Index.Edge` and no caller builds a `kb.EdgeRow` by
hand. The set has no verb: it is consumed in-process, and `--dry-run` is the inspection surface.
Membership: SPEC §8 The two passes.

`internal/score/scope.go` narrows the set to a run's scope. `scopeOf` returns the one named item, or
else each item of the pass that is stale or, under `--rescore-manual`, manually scored. `namedNode` and
`namedEdge` refuse an id or an edge that does not exist or that no pass asks; `namedItem` refuses a
manually scored item without `--rescore-manual`. `dirty` compares the hash each record carries
(`recordedHashes`, read once through `kb.LoadGraph`) with `judgedHash`, the hash of what the item would
be judged against now: `asks.NodeJudgedHash` over a node's statement, `asks.EdgeJudgedHash` over its two
ends' statements, each the one the run's `graph` read through `kb.Statement`. The judged hash is
defined in SPEC §10 Provenance and the judged hash. `asks.JudgedHash` hashes the digest of the
template the item is judged under, then each statement behind its length; the model is no part of it.
`RigorAsk` is the single mapping from node kind to template, which the hash and the call both read.

A resume takes no scope: its set is the run's `items.jsonl` (`readItems`). `ResumeTarget` picks the run
a `--resume` continues, the newest `run-` directory by its header's start (`newestRun`, filtered to
`RunKindReal`, so a `dry-` run is never one). It refuses where none exists, where `ReadRun` counts the
run finished, and where a build invocation in the store's progress record (`buildstore.ReadState`)
started after it.

**Pattern:** Filtered work set. `Runner.Skip` is `score.ManualSkip(ix, rescore)`, built from the run's
`--rescore-manual` and each item's `manually-scored` flag (`query.Index.ManuallyScored`). It says which
items the loop records `skipped` (status `score.StatusSkipped`, reason `manually-scored`; no call, no
capture). `score.Resume` takes the same predicate, so `Ledger.reprice` leaves a skipped item's written
line as it is. The loop also records `skipped`, once, an item that is not in the in-memory index it
loaded (`inKB`), and `attempt.fail` does the same for a write's `errItemGone`, each with that error's text as
the reason (SPEC §8 Skipped items).

#### Bearing predicates

**Novel.** The two predicates separate which edges carry a bearing from which a score run asks about,
which realizes THESIS item 1 (the four scoring relations) as defined in SPEC §10 Four scoring relations,
one arithmetic.

**Pattern:** Filtered work set.

Two predicates in `internal/kb` answer whether an edge carries a bearing, and they are not the same
question. `kb.Scoring`, over the four relations in `kb.ScoringRelations`, is whether an edge carries a
bearing at all. The readers of a stored bearing use it: the write package's relation checks, the
derived views and strength propagation in `internal/index`, `internal/query`'s trace and payloads, and
`bearingText`. `kb.BearingWalk`, over the three in `kb.BearingWalkRelations`, is whether a score run
asks for that bearing. Its readers are `query.ScoringSet`'s bearing pass, `namedEdge`, the
`manually-scored` flag on edge rows (`internal/index/derive.go`) and its description in the write
package, and `stats`' `walked` key. A `founds` edge carries a bearing and is never an item, its source
lying outside the corpus, so `edit-edge` alone writes it.

#### The ledger and the state directory

**Pattern:** Append-only job log with resume.

`score.Ledger` opens `<state-dir>/score/<run>/`, writes `run.yaml` through `atomicfile.Write`, and
appends the loop's lines in one write, with `O_APPEND` and `Sync`, when the loop ends. Before then, an
answered item writes its own result file, `results/<item index>.json` in the run's directory, the
index being the item's place in the set (`writeResult`); the loop's end folds these into the ledger
and removes them. It reads back
the last entry per item, with its status and the evidence behind it, so `--resume` skips what was written
and a failed run names each failed item's reason. A line is appended after the item's write and
validation, never before: after a crash between the two the item gets a new model call and an
idempotent write, where the reverse order would skip a value that was never written. `Create` never reuses a run directory: the run id is minted
past any that exists, and a directory that appears under it fails the call.

`Resume` refuses a `run.yaml` naming another KB root or ledger format. It takes the run's pass from the
header. It drops a tail torn after its last newline, so the item the line names gets a new model call, then
merges the result files a stopped invocation left (`fold`): each value is written through the run's
`Write` interface, recorded, and the file removed; one whose attempt the ledger already records is
removed alone. Then it recomputes values ([the re-price](#the-re-price)).

`items.jsonl` holds the set the run enumerated, one pending line per item, written when the run opens
its ledger, so status and a waiter count the whole run rather than the attempts made so far; it is all a
dry run writes beside `run.yaml`. The files, the line's keys and the optional `remedy`, `note` and
`judged` under one ledger format are SPEC §8 The ledger and resume. Two mechanisms behind them:

- A line contains `faults` (the parsed charges, in the order written), `moot` and `before`, the value
  its item had in the in-memory index the run read before the attempt wrote. `faults` is `[]` for the
  reply `none` alone, so an empty list always holds the model's positive statement that it found no
  fault. A failed attempt contains neither charge list and no reply text: its `reply` is the reason, and
  its line still carries `before`. `remedy` is the one the call's failure named, and `note` goes with it; a failure item appends the note
  after the bounded reason, so the bound never cuts it. Reply, remedy and note pass through
  `argmark.Spelling` ([§9](#per-interface-rendering)), so no line carries a marker.
- `backend` and `pass` sit on every line, because a resumed or mixed run's lines must say what answered
  each item and which class each judgement came from. `Create` records `ask_sha256` from the header's
  `Pass` (`askDigest`), as it records the format: one hash over every instruction the run can send, each
  rendered for an empty subject, concatenated in the order statement, method, bearing. It is recorded at
  one site, so no caller can open a run without the prompts its judgements answer, and a render that fails
  is a defect of the build, not a run written with no hash. `Resume` reads the header's hash and compares
  nothing against it (SPEC §8 `ask_sha256` records the ask set and gates nothing).

The state directory is `internal/statedir.Dir`'s: the override, else `$XDG_STATE_HOME/kbase/<key>`, and
never inside `kb-root/`. It is shared with the build so a run and a build agree on the state directory
for a KB; the build store's own format record is `internal/buildstore`'s (§8).
The run takes the run lock through `build.Hold` ([§8](#locks)); the lock's record names `kind: score` and
the run id.

#### The captures

**Pattern:** Request/response capture for replay.

The loop's end appends every capture to `captures.jsonl` in one write, beside the ledger, through
`appendLines`, with the `O_APPEND` and `Sync` the ledger's own lines take. A capture is written for each
attempt that returns a `call`. `attempt` returns none where the class cannot be derived or the input
guard refuses the text, and a call the context cancelled returns nothing, so none of the three leaves a
capture. Captures are written before the lines, and a capture that cannot be written ends the run
`failed`, as a ledger line that cannot be written does.

A capture's keys, in order: `run`, `pass`, the item's identity as the ledger line formats it (`identity`:
`kind` and `id`, or `source`, `target` and `relation`), `class` (`statement`, `method` or `bearing`),
`text` and `lines` (the item text whole, each line `{no, end, from, to}` as `closedText` holds it),
`prompt` (the bytes sent), `prompt_sha256`, `reply` (what the `Ask` returned; `""` where the call
errored or the reply was truncated), `error` (the call's, the parse's or the write's error text, absent
on a successful write), `model`, `backend`, `started` and `ended`. The ledger's failed line carries the
reason and no reply text; the capture is where the reply is kept (SPEC §8 The ledger and resume).

`Resume` removes a torn last line from `captures.jsonl`, as from `ledger.jsonl`, through `trimTorn`, the
single torn-tail trim, before anything appends, so a crash mid-capture never joins two captures into one malformed line.

A capture is self-sufficient for a replay. `readLastCaptures` (in the replay's tagged `replay.go`; the
raw read is `readComplete` in the untagged `ledger.go`) returns, per item, its last capture with the item
text rebuilt. It reads the file to its last complete line, so a run still appending is readable, and
refuses a line that names no class or whose line ranges fall outside its text. **The fidelity rule:**
`class.render` is the single site that renders an item text into a prompt, shared by `compose` and the
replay, so rendering a capture's item text through an unchanged template reproduces its `prompt_sha256`
byte for byte. `TestCapturesReRenderToTheirDigest` asserts it over captures the runner wrote.

#### Run retention

**Pattern:** Keep-last-N retention.

`KeptRuns` (§12) runs of each kind remain under `score/`. A real run's id is minted
`run-<id>` and a dry run's `dry-<id>` (`kb.MintID`'s prefix, `RunKindReal` and `RunKindDry`). A
directory's kind is its name's prefix (`runKind`), so the two kinds are counted apart.
`score.PruneRuns(dir, run)` keeps the run the invocation names, created or resumed, and the newest
`KeptRuns-1` other directories of its kind. It orders them by the `started` each `run.yaml` records,
equal starts by name with the later name newer, and removes the rest of that kind whole with
`os.RemoveAll`, captures and replays with them, through `buildstore.PruneByStart`. It also keeps the
newest `KeptRuns` score-run captures under the state store's `reports/` (`buildstore.PruneCaptures`,
prefix `score-`). `buildstore.DirStarts` is the single per-directory read of that start and
`buildstore.CompareStarts` the single ordering, both shared with `NewestRun`, so `status`'s newest run
and the prune's agree. A directory whose `run.yaml` is absent or unreadable yields no start and is kept.

`score.Invoke` calls the prune once the run's ledger has opened (after `Create` on the dry path, after
`Create` or `Resume` on the real one), so the run it keeps exists, and an invocation refused before then
removes nothing. It runs under the run lock, and a prune error ends the invocation `failed`. The one
writer under `score/` the run lock does not exclude is a replay, which takes no lock: a prune may remove
the run a replay is reading, and the replay then fails rather than writing into a removed directory
([the development replay](#the-development-replay)). `ResumeTarget` filters runs by kind, so a `dry-`
run is never a resume target; `status`, `wait` and `RunByPID` find a run by its directory or pid,
whatever the prefix.

#### Writing the results

**Pattern:** Optimistic concurrency with partial-batch retry.

`internal/score`'s `landValues` writes a run's answered values in one write batch, under one hold of the
KB write lock taken through `write.LockKB` ([§8](#locks); wait: `write.LockWait`, §12).
It is the single path to the KB: the loop's end, a resume's merge of stopped results and a resume's
recomputation all reach it through the `Write` interface (SPEC §8 Writing the results).

Under the lock and before any batch, `landValues` opens the KB afresh through the interface the verb
supplies (`Invocation.Open`, `kbload.Open` in `cmd/score.go`) and reads it through `query.Load` and
`graphOf` (`graphAt`). Each value is then checked against the KB it finds:

- A KB that does not open or load fails every value with its error.
- A value whose node or edge is not in the KB fails with `errItemGone`, which `attempt.fail` records as
  status `skipped`, not `failed`, so `ReadRun` counts it skipped and `ResumeTarget` does not resume
  for it. A resume's loop records an item of its run that is not in the in-memory index it loaded the
  same way, once (`Runner.Run`, `inKB`), and makes no call for it.
- A value whose `Landing.Judged` (the judged hash `prepare` recorded as its call was composed, carried
  on the result file and the ledger line as `judged`) differs from `judgedHash` over the text the KB now
  holds fails with `errTextChanged`: a statement edited by hand while no lock kept the edit out. A
  resume sends it again.

Only the remaining values go into the batch, so the hash the edit op records is always the hash of the
text the value judged. They form one mixed write batch (`write.Batch`) through `write.Run`, an entry per
value in the order given. `opOf` names the edit op: `edit-claim`, `edit-analysis` or `edit-experiment` by
the node's kind with `id` and `rigor`, or `edit-edge` with `source`, `target`, `relation` and `bearing`.
The batch is applied under `Provenance: write.ProvenanceMachine`, so a written value has
`manually-scored: false`, and it rebuilds the derived views inside the batch, so the records and the
derived views are written together. A `done` or `unchanged` batch has written its values.

A refused batch writes nothing (SPEC §8 Write batches). Where the refusal names its entries, those items
fail and the batch is retried without them, until it is written or empty. Where it names none, the
batch's values are retried one at a time under the same hold, each refusal then failing its own item;
this covers a refusal the reopen did not foresee, a hand edit between the reopen and the batch, or a
check the write makes that a load does not. A failed value's error is its details joined and formatted
for the command line (`valueError`), so no ledger line carries a marker. A batch that fails, as against
being refused, fails every item it contained. A lock that cannot be taken fails every item too: one held
past the wait fails each with the detail of `write.HeldLockItem`, the item a verb answers `retry` with,
and any other lock failure fails each with the lock's own error. The run continues. `landValues` returns
one error per value, nil where it was written. It checks for no run lock: it writes as the run holding
that lock, so no run can be holding the KB out.

Each value the batch sets records its judged hash in `internal/write` (`judged.go`). An entry that sets
a rigor or a bearing names the record through `valueSet`: a caller's value, `*pending*` included, or a
run's number. A run's `*pending*`, which the build writes for `score` to judge, is no judgement and names
none. Once the entry has applied, `hashValuesSet` gives each named record `asks.NodeJudgedHash` or
`asks.EdgeJudgedHash` over the statements the batch leaves, read through `kb.Statement` from the host
leaf, so a node an add marks in the same entry is hashed over the statement its marker gives it.

#### The re-price

**Pattern:** Projection rebuild from stored evidence.

`Resume` takes the same `Write` interface a run's writing is opened with, and `reprice` recomputes every
written line's value from the ids its line records, under the table the line's own class holds. A run
written under one rigor table is recomputed under the class table. Only a line that records `judged`
and its charges is recomputed; a line without them says nothing of what text its value judged and is left
as it is, as is a line for an item `skip` names.

A value the running table gives differently is written first through the batch above, under the KB write
lock, and only then does the line record it, so no line claims a value the KB did not take. A value the
write discards (`errTextChanged` or `errItemGone`) rewrites its line with that reason through
`attempt.fail`, as a loop's discarded write does: `failed` for a changed item, which the loop that
follows sends again, and `skipped` for a removed one, which nothing sends again. Any other refused write
fails the resume with the ledger as it stood; the next resume recomputes the same change, the write being
idempotent. Each changed line is listed in `repriced.yaml` in the run's directory, written only where a
value was written: the run, the time, and per line the item, the value it held and the value it took.
`run.yaml` is written once, so `repriced.yaml` is where a recomputation is recorded. `Ledger.Repriced()`
reports how many values changed. No model is asked: the ids are the whole input of the value, and the
model never saw a weight. Behaviour: SPEC §8 `ask_sha256` records the ask set and gates nothing.

#### The run's entry

**Pattern:** Command orchestration.

`score.Invoke` runs one invocation over a KB its caller has opened, rebuilt and locked. It runs
`query.Load`, builds the run's `graph`, and determines the set: the scope (the scoring set), or a resume's
own `items.jsonl` under the pass and `rescore_manual` its `run.yaml` records. Then either the dry run's
records and the listing of what it would ask and skip (`Report.Ask`, `Report.Skip`), or the loop over the
`Launch` the verb built (`Invocation.Launch`): the ledger created or resumed, the prune, the pid,
`items.jsonl`, and the loop, whose values are written by the run's one batch when the loop ends.

It returns a `score.Report`:

- the outcome, and `Asked`: whether the invocation reached the route its calls take (not a dry run, so
  `Launch` was handed to the loop);
- the run and its ledger directory once the ledger exists with the pid and `items.jsonl` beside it, and
  on a stop only where the loop stopped it;
- the counts and the items scored;
- on `failed`, one `check: error` item per item the ledger records failed, its detail the item's name
  and its line's reason; on `cancelled`, the loop's first item with no attempt, `{}` where every one has
  one;
- `Unfinished`: whether an item failed or was not reached.

`score.RunID` creates the runs directory and mints the id (`run-<id>`, or `dry-<id>` under `--dry-run`)
or takes the one `score.ResumeTarget` found, before the locks, because the run lock's record names the
run.

#### The verb

**Pattern:** Command orchestration; detached background job.

`runScore` calls `scoreRun`, the body. `cmd/score.go` keeps the role, the `Ask`, the locks, the start
rebuild and the resume call, in this order:

1. `scoreUsage`'s checks of the scope arguments together and of `--edge`'s form, then the
   `--ask-concurrency` check.
2. The KB root and the state directory; for `--resume`, `score.ResumeTarget` and the pass and
   `rescore_manual` its header records.
3. The scoring role, resolved before the KB is opened: `scoringRole`, through `config.Resolve`, with
   `--scoring-model` supplying or overriding the model. It is skipped under `--dry-run`, so a setup
   refusal writes nothing and mints no run.
4. `openKB` and the `migration` record; the load-error refusal; the pending-conversion refusal
   (`index.PendingConversion`).
5. `score.RunID`, then the run's locks through `build.Hold` ([§8](#locks)), before any record of the run
   is written.
6. One rebuild of the derived views through `holdWriting`, skipped under `--dry-run`, which releases the
   KB write lock before the loop.
7. The `Launch`, `scoringLaunch`, then `score.Invoke` given that `Launch`.

The result document is assembled in this order: `roles` where the report's `Asked` says the loop was
handed the `Launch`, then `run`, `ledger`, `counts`, a dry run's `ask` and `skip`, and `scored`;
`resume` (`scoreResume`) where an item failed or was not reached, ahead of the final record; and last
the report's `failures` on `failed`, or its `cancelled` on `cancelled`. The batch that writes the
results rebuilds the derived views, so no rebuild follows the loop. `persistScoreResult` writes the
document beside the ledger (`score.WriteResultDocument`) before the locks are released, so `wait` reads
the document the verb emitted. The records and `resume`'s reach: SPEC §8 Outcomes and The closing record.

Registration is `readWriteTool(scoreCmd)` in `cmd/surface.go`: the command is exposed as an MCP tool,
and the server starts it as a detached child (`mcp.ScoreLaunch`), as it starts `build`, returning the
run's identity and a `wait` command instead of holding the turn for the loop's duration. The tool
table and the detached child are [§12](#mcp-server-spec-11); the start wait is `launchStarting`
(§12). `--edge` is declared a mapping flag (`mcp.MappingFlag`): over MCP an object of
`source`, `target` and `relation`, joined into the `--edge` the child takes.

#### The development replay

**Pattern:** Offline prompt-regression harness.

`kbase score --replay RUN` repeats a run's captured model calls under the binary's embedded templates
and reports what the runtime would have done, so a change to a scoring template is `edit → just build →
replay` over the items that failed. SPEC states nothing of it. Its code and tests are
`cmd/score_replay.go` and `internal/score/replay.go` with their test files, each carrying
`//go:build kbasedev`. `cmd/score.go` reaches it only through the package-level `scoreReplay` hook, which
the tagged file's `init` sets and which is nil in the default build. That `init` also declares `--replay`,
`--failed` and `--limit` (a run takes no limit, so `--limit` is the replay's alone), each annotated
`kbase.mcp` = `excluded` through `mcp.ExcludeFlag` ([§12](#mcp-server-spec-11)), so dev and release builds
serve one tool table.

The default build is the release shape and excludes the replay's code. Dev builds pass `-tags kbasedev`,
as `just build`, `just test` and `just test-race` do. What the replay reads is not dev-only: a run's
ledger and captures are written by the untagged `internal/score/ledger.go`, so a replay in a dev build
reads the records a released kbase wrote. `edit-gate` vets both build tags, and `test-dist-no-replay`
runs `TestDistBuildHasNoReplay` (`cmd/dist_test.go`) on the untagged build, which asserts the three flags
absent and the hook nil, and the same of the build's development replay (§8).

`scoreReplayRun` takes `--pass`, `--failed`, `--limit`, `--scoring-model`, `--state-dir`, `--timeout` and
`--ask-concurrency` beside `--replay`, and the global `--config-dir`. Any other score flag with it is a
usage refusal naming `--replay`; `--failed` or `--limit` without `--replay` is one naming it; and a
`--replay` value outside `^(run|dry)-<id>$` (`replayRunRE`: the alphabet and length `kb.MintID` mints)
is one naming `--replay`. The replay opens no KB and takes no KB write lock or run lock; a run in
progress may be replayed. With `--state-dir` it needs no KB root and works from any directory. Without
it, it finds the store through the kb-root at or above the working directory and is refused, `check:
usage`, `key: --state-dir`, where there is none. Both development replays, this one and the build's,
resolve their store through `replayStateDir` in `cmd/replay.go`, beside `replayTakes`, which names the
flags a replay takes in its refusal. A relative `--state-dir` resolves against the project root where
the working directory has one, and against the working directory where it has none.

`score.OpenReplay` selects every item with a capture, in the order of the run's `items.jsonl`, each by
its last capture. `--pass rigor` or `--pass bearing` narrows the selection to that pass's calls, refused
where the run did not run it; `--pass all` or no flag takes the run's whole list. `--failed` selects only
the items the run's ledger last records failed, and `--limit N` takes the first N. A run with no
`captures.jsonl` (a dry run, one pruned, one that asked nothing) is `refused`, `check: usage`,
`key: --replay`. The scoring role is resolved by `scoringRole` and the `Launch` built by `scoringLaunch`,
the functions `runScore` uses.

`Replay.Walk` renders each captured item text through `class.render`, which uses the binary's embedded
template for the capture's class, sends the prompt through the `Ask` its `Launch` hands the item, and
reads the reply with `call.attempt`, the runner's own parse against the captured item text under the
class's table, then computes the value. It writes no result to the KB, and a per-item failure is counted,
not the replay's outcome. While it runs it logs its progress through the verb's logger, at info level,
as the message `replay progress` with the keys `done`, `failed`, `remaining` and `total`: at most once
per `ReplayProgressInterval` (§12) and once more when the loop ends. The counter and the
one goroutine that logs it are `buildstore.ReplayProgress`, which both development replays use. An item
only increments the counter as its call returns (an item whose status is `done` counts done, any other
answered item failed, an interrupted one neither), and no item and no progress line writes a file
(I11, §2).

The replay writes only under `score/<run>/replays/<replay-id>/`, each of its three files appended once,
when the loop ends, the id minted `replay-<id>` as a run's is:

- `replay.yaml`: `replay`, `run`, `backend`, `provider`, `model`, `pass`, `ask_sha256` (computed as
  `Create` computes it for the replay's pass) and `started`.
- `ledger.jsonl`: one line per item asked, in the run ledger's shape, its `run` the replay id.
- `captures.jsonl`: the replay's own captures, in the captures' shape, `run` the replay id.
- `comparison.jsonl`: per item, `item` (`ItemName`), `before` (the value the run's line recorded before
  its call), `original` and `replay` (each `status`, `value` and `faults`, the fault ids in order, `[]`
  for `none`) and `prompt_changed` (the replayed prompt's hash differs from the captured
  `prompt_sha256`).

Setup computes the hash and `replay.yaml` before creating anything, and creates `replays/` and the
replay's directory with `os.Mkdir`, never `MkdirAll`: a run the prune removed fails the replay rather
than being recreated without a `run.yaml`, which no prune would ever remove. After writing
`replay.yaml`, `Walk` keeps itself and the newest `KeptRuns-1` other replays of the run's `replays/` by
`replay.yaml`'s `started`, through `buildstore.PruneByStart`, which the run prune also uses, and after
the loop a replay whose directory no longer exists ends `failed` naming it. An item whose run records no
ledger line compares against an `original` with status `""` and is not counted in `changed`.

The result document is one SPEC §7 YAML document with the keys `outcome`, `state-dir`, `run`, `replay`,
`dir` (the replay directory, absolute), `counts` (`total`, `done`, `failed`, `skipped`) and `changed` (an
integer: the items whose status or value differs from the original's), then the final record the outcome
names. The outcome is `done` when every selected item was asked, `cancelled` when the context stopped the
loop (its `cancelled` record naming the first item it did not ask), and `failed` for a fault of the replay
itself, an unreadable capture file or a store write.

## 7. Inference seams

**Pattern:** Role-based model routing behind provider adapters.

`internal/config` loads the providers and the role choices, `internal/model` contains the wire clients
and the call queue, and `internal/modelclient` is the only constructor of a `model.Client`. Three roles
route calls: `decision` for the multichoice questions, `text` for the overview call and `scoring` for
`kbase score`'s calls (SPEC §9 The files are the one source of inference). Each role names its own
provider and model in `[models]` as a `<provider>/<model>` reference. No role falls back to another;
no reason is recorded for that.

Every model call is made through a `model.Slot`. `model.Queue` has N permits; `Queue.Go` waits for a free
permit and starts the item with a `Slot` on it (§2, I12). `Slot.Chat` makes the overview call and
each scoring call, and `Slot.Decision` makes each multichoice question. The `Client` interface's
inference methods are unexported, so the compiler restricts every other package to a slot, and a slot is
spent when its item returns. `build`, `score` and both replays build the queue from `--ask-concurrency`,
else `[asks] concurrency`, else `config.DefaultAskConcurrency` (§12). `askConcurrency`
in `cmd/askconcurrency.go` is the only resolver and refuses a flag under 1. The same N is the idle
connections kept per host: `modelclient.New` takes it and sets `model.Endpoint.IdleConns`, so parallel
calls reuse connections. A build shares one queue across its roles; `probe` runs a queue of one.
`walk.launch` checks that a provider resolves before the first stage when a stage left to run makes
model calls (SPEC §5 Inference preflight).

### Probe

**Pattern:** Health-check / smoke call.

`cmd/probe.go` makes one call on the path a run uses and reports it (SPEC §8 `probe`). It is a
diagnostic, the CLI command and the read-only MCP tool alike (§2, I9).

- **Selection.** `config.ResolveFlag` resolves one selection. `--model` supplies a reference and names
  itself in a refusal. A role flag (`--decision`, `--text`, `--scoring`) names the role and supplies no
  model, so the role's model comes from the configuration and a refusal names its key alone.
- **Client and call.** The client comes from the registry, as `score`'s does. A `--decision` selection
  makes one `Slot.Decision` call on the API its provider's `decisionsFlavor` names; every other
  selection makes one `Slot.Chat` call. `--prompt` replaces the default check of either path,
  `probeDecisionPrompt` or `probePrompt`. A chat request to a decision selection is the mismatch a probe
  exists to find.
- **One call.** The call has no backoff, so a repeat is immediate. `--timeout` bounds it, and the
  deadline and the `wall-ms` clock start inside the item, so neither counts a wait for a permit. The
  chat path reports `ChatAttempt.FirstToken`, measured in `chatOnce`: request start to the first
  non-empty content or reasoning delta, zero where none arrived. The decision API streams nothing, so
  its document has `first-token-ms` zero and `finish-reason` null.
- **Sampling.** A chat selection's client is the registry's for its role, so the call has that role's
  sampling (the `scoring` role's for `--model`), and the document reports `model.SamplingOf(client)`.
  Where the call ends `truncated-empty`, the chat path reports `ChatResult.Final` as the answer
  instead of failing.
- **Isolation.** `probe` opens no `kb.Source`, resolves no state directory and takes no lock.

### Wire clients

**Pattern:** Provider adapters behind one interface; connection pooling.

`internal/model` contains two implementations of the chat request. `model.OpenAIClient`
(`openai_client.go`) speaks `chat/completions`, and requests the usage chunk
(`stream_options.include_usage`): without it a streamed response carries no `usage` and every token
count reads zero. `model.AnthropicClient` (`anthropic_client.go`) speaks
the Messages API: `POST <baseUrl>/messages`, the `x-api-key` and `anthropic-version` headers, its own
event sequence and its own catalogue shape. A provider silent on `api` gets the Messages client
(SPEC §9.1 The chat protocol). `model.NewClient` dispatches on `model.Endpoint.Protocol`, a value
`modelclient` resolves from `config.Endpoint.Protocol` (`chatProtocol`); `Endpoint.Decisions` is
resolved from `decisionsFlavor` the same way (`decisionsSurface`). `internal/model` reads no
configuration string, imports no kbase package but `secret`, and touches no file (`TestModelImportsNoOS`).
The body each client sends is its protocol's, never shaped by host or provider name.

Each client's HTTP client has no total timeout, which would cap a streamed generation. The
bounds are the call's context deadline, `responseHeaderTimeout` for the handshake and
`streamIdleTimeout` for the gap between reads (§12). A non-2xx answer is a
`model.StatusError` from both, its body truncated at `errorBodyLimit` and scrubbed of any key the server
echoed (`statusError`).

**Pattern:** Vendor parameter mapping.

`model.Sampling` is the sampling type for both protocols. Each client resolves `Endpoint.Sampling` once, at
construction, with `Sampling.Resolved()`: `DefaultTemperature` and thinking off where the role states
none. `chat()` copies it onto every request through `Client.sampling()` and shapes no field of either
protocol; each protocol's wire rules live only in its body encoder (`encodeRequest`,
`encodeAnthropicRequest`). `model.SamplingOf` returns a copy, so no caller can alter what a client
sends. The fields each protocol sends are SPEC §9.1 The chat protocol. The Messages encoder sets the
required `max_tokens` to the thinking budget (`thinkingBudgetLow`, `thinkingBudgetMedium`,
`thinkingBudgetHigh`) plus an answer allowance: the larger of `maxTokensFloor` and a quarter of the
request's characters, rounded up. The quarter has no recorded reason (§12).

**Pattern:** Provider prompt-cache affinity.

`modelclient.New` takes a prompt-cache key. A scoring run and a scoring replay pass their id, and the
registry sets it on every endpoint it builds. The OpenAI client sends it as `prompt_cache_key` on every
request, so one run's calls reach one upstream and share a cache entry. A verb that names no run passes
none, and the field is omitted rather than sent empty. The Messages client has no such field and
sends none.

### Retry and attempt outcomes

**Pattern:** Retry with backoff on retryable status.

`chat` repeats a request identically while it does not complete, up to `model.ChatAttempts`. The
caller's `ChatRequest.Backoff` gives the pause before each repeat, its last entry repeating. The overview
call (`overviewBackoff`) and `kbase score`'s calls (`scoreBackoff`) pass a backoff; the build's
multichoice questions and `probe` pass none and repeat at once. The build's multichoice questions have
no pause and no reason is recorded for that. A pause the call's deadline cannot cover ends the call at
once, and its reason names the attempts it made (`gaveUp`). Values are in §12.

`retryableStatus`, a 429 or a 5xx, is the only non-2xx status repeated; any other ends the call on the
attempt that met it. So does an answer that reaches the completion cap before any answer text:
repeating the request identically reaches the cap the same way. `Slot.Decision` has its own loop with the same
count and the same status rule, no pause, no streaming and no `ErrChatIncomplete`.

Each attempt ends in one outcome constant: `ChatOK` (`ok`), `ChatServedNothing`,
`ChatTruncatedEmpty`, `ChatProviderRefused` (a non-2xx status) or `ChatTransportFailure` (no answer
from the provider). An attempt records whether the call reasoned, from a reasoning delta on the stream
or a usage block reporting reasoning tokens, so a call that thought without streaming the thought
counts. The error a failed call returns wraps `ErrChatIncomplete`, has the last attempt's outcome
and text, and wraps that attempt's `StatusError` where it met one. That text is what a failed item's
reason records (SPEC §8 A provider call's failure names what ended it).

### Refused keys

**Pattern:** Typed error classification at the boundary.

`StatusError.RefusedKey` (401 or 403) is the single predicate. Two sites match it: `asks/call`'s
`keyRemedy` and `modelclient.KeyRefused`, which `Registry.Catalogue`, `cmd/score.go`'s call and
`cmd/probe.go` apply. Each attaches `config.AuthAddRemedy` with the name of the provider the call went
to, and the result travels as data:

- `asks.IncompleteError.Remedy`, which `claimgraph`'s inference stop and the build's overview failure
  put on their items;
- `score.RemedyError`, whose remedy and `config.RefusedKeyNote` the attempt's ledger line records;
- `modelclient.KeyRefusedError`, which `cmd`'s `failed` puts on the failure item of `probe`, `models`
  and `configure-models`.

`asks/call` is told the provider it calls by `call.New`, from `build.Provider`'s per-role provider
names. No site recovers a status from a reason's text, and nothing checks a key before a call
(SPEC §9 A refused key).

### Layered configuration

**Pattern:** Layered configuration.

`config.Layers` resolves the directories and `config.Load` reads and merges the two files of each,
once, before a verb's own work. It returns one `Loaded`: the pool, its faults and the choices. The
layers are `<repository>/.kbase/` over `~/.config/kbase`, or the one directory `--config-dir` or
`$KBASE_CONFIG_DIR` names (`explicit` and `environment` in the source vocabulary), so an explicit choice
isolates a run from a repository's own `.kbase/`, which every test and pinned deployment needs. The
caller resolves the repository root with `kb.RepositoryRoot(workDir)`, which answers `""` outside a
worktree, so `internal/config` takes no git dependency and reads the global layer alone there.

- **Merge.** `takeProviders` replaces a provider's table whole by a higher layer's of the same name, so a
  repository entry cannot inherit the global entry's `apiKeyFile` for an endpoint it chose;
  `takeConfig` overlays `config.toml` one field at a time (SPEC §9 Merging). `parseProviders` folds each
  table name to lowercase as it reads it, after refusing a file with two names that differ only by case
  (SPEC §9.1 `providers.toml` — the endpoint pool). `config.ParseReference` and `config.Select` fold the
  names they are given the same way.
- **Faults.** `config.Load` drops a fault naming a provider a higher layer already supplies: the pool's
  provider is the higher one, so the fault describes a value nothing reads. A higher layer's own fault
  is kept, since its provider never entered the pool. A layer that fails the strict load refuses the
  whole load and no lower layer is read (SPEC §9.3 Strict-load contract; `rejectUnknownKeys`).
- **Provenance.** `Provider.Layer` and `Config.ModelLayer` record the layer each provider and each
  role's value came from. `ProviderFault.Path` names the `providers.toml` a fault came from, and a
  `Refusal`'s path names the file the lacking setting belongs in, so a layered configuration says which
  file to edit. `config.Layer` has a directory, its source (`explicit`, `environment`, `repository`,
  `global`) and, once `config.Report` has probed it, whether the directory and each file exist
  (`ConfigExists` beside `ProvidersExists`).
- **Write target.** `config.WriteTarget` is the directory `--config-dir` or `$KBASE_CONFIG_DIR` names,
  else the global one. It is where the configure commands write with no location flag
  (`configureLocation` in `cmd/provider.go`), so a refusal of a name the pool lacks, whose remedy is the
  `configure-providers` call that adds it, names the file that call writes. `configure-models` checks
  against the stack it writes into, `config.WrittenStack`, loaded by `loadWrittenStack`, so its refusal
  names the written layer's `providers.toml` and its remedy has the location flag the call had.
  `configure-providers --remove` loads the same stack to name what still refers to the provider. Both
  commands resolve the layers (`resolveVerbLayers`) without loading them, so no layer outside the stack
  written is read (SPEC §9 The configure commands' write location).
- **Writes.** `UpdateConfig`, `SetProvider` and `RemoveProvider` edit the existing file line by line.
  `verifyEdit` decodes the proposed file and requires it to equal the decoded original plus the
  intended change, else the write is refused. `installConfig` then calls `atomicfile.Write`: a temp file,
  synced, renamed into place (SPEC §9.5 The configure commands' targeted update (byte-preservation
  contract)).
- **Report.** `config.Report` produces the facts `show-config` reports: the pool's providers and faults,
  each provider's key kind and upgrade, the diagnostics, the files' presence and `config.toml`'s values.
  It reads no file of its own; the caller passes the existence probe. `cmd/show-config.go` opens no
  client (SPEC §9.6 `kbase show-config`).

### Resolution and the client registry

**Pattern:** Factory registry with lazy, shared clients.

`config.Resolve` (`internal/config/resolve.go`) answers one role's `Endpoint` from the provider its
reference names. For `text` and `scoring` the endpoint has `Temperature` and `Thinking` from
`[sampling.<role>]` with the layer of each, and a temperature outside the range the protocol accepts is
refused. Resolution reads only the loaded configuration: no file, no connection, no stderr. A refusal's
remedy may name the command that fixes it (`kbase configure-models`). `config.ResolveFlag` is the same
resolution with the flag that supplies a role's model named by the caller, so `probe`'s `--model` is
named in a refusal where `--scoring-model` would be. `config.Select` is `models`' choice of a provider
to send a request to: a flag, else the only provider (SPEC §9 Provider selection is the catalogue
verbs' alone). No other site applies a selection or a resolution rule.

A configuration refusal is a `result.Refusal` whose items name their check: `config.CheckProvider`
unless an item names its own, as `result.CheckUsage` does. A decision with more than one side to state
has several items. An item's remedy is its clearing text alone. `cmd`'s `refused` composes
`config.ShowConfigRemedy` over every `provider` item a verb refuses with; `show-config` refuses with
none, so it reports its role diagnostics with the clearing text alone; `walk.launch` composes it for the
build's preflight refusal and reads the key and path of a one-item refusal.

`modelclient.Registry` is the only constructor of a `model.Client`. It keeps one client per provider
name, wire protocol and resolved sampling, so roles whose resolved sampling is equal, an unset
setting being equal to its stated default, share one client and its connection pool. A client is opened
on first use, connects with the key the provider resolved to at load, and `CloseAll` closes every client
before the verb returns on every outcome. `modelclient.NewClientFunc` is the constructor the registry
calls, `model.NewClient` in production, so a verb runs without a network. `samplingOf` maps the
configuration's `thinking` onto `model.Sampling`.

### The decision API

**Pattern:** Constrained classification endpoint.

A multichoice question is one `POST` to the provider's decision API (SPEC §9.1 The decision API): a
`Slot.Decision` call with one `noul` or `choice` question, whose answer is read by the key it echoes, so
no reply text is parsed. `decisionsFlavor` selects between two request shapes, and a provider silent on
it gets `systemone`.

- `systemone`: `model`, `state`, and `questions` as a JSON object keyed by the question key, each
  question's `criteria` having the per-letter meanings. The `answers` object returns keyed the same way.
- `decisions`: the state as `input`, and `questions` as an array of named questions. A `noul` question
  is sent as a `predicate`, which has no choices, so its true and false guidance goes into
  `instructions` (`predicateInstructions`); a `choice` question's criteria are `choices`, sorted by
  value. The `answers` array names the question each entry answers.

Both decoders map into `model.DecisionAnswer`, so a caller cannot tell the shapes apart. Under either,
`validateDecisionAnswers` refuses an answer naming a question kbase did not send. A `refusal`, a missing
answer and an answer with no value decode to an answer with no confidence, so `asks.Decide` finds no
letter and the question takes the multichoice-ask interface's retry-once-then-default path
([next](#the-multichoice-ask-interface)).

Both shapes are OpenAI-shaped, so the registry gives the `decision` role an `OpenAIClient` whatever its
provider's `api`. A provider whose `api` is `anthropic` keeps its Messages client for chat and gains an
OpenAI client, on the same base URL and key, for decisions. A Messages-only server has no decision API
and the call fails with that server's status. `AnthropicClient.decision` refuses, and nothing in kbase
reaches it. A decision call has no system message.

### The multichoice-ask interface

**Pattern:** Dependency injection with an architecture fitness test.

Each inference pass reaches the model only through `internal/asks`' multichoice-ask interface,
injected by `build`; nothing in `claimgraph` knows a network exists. `cmd/importpolicy_test.go`
enforces it two ways: `claimgraph` may not reach `internal/model` or `internal/asks/call` through any
chain of imports, and only `build` imports `internal/asks/call` directly.

- **`internal/asks`** contains the embedded templates and fragments, the composer and `Batch`. Every
  claim-graph model call is one question answered by one letter from a fixed set the build offers.
  `asks.Decide` takes the offered letter with the highest confidence, the first offered winning a tie,
  or else a one-character reply that is an offered letter. A reply with no letter is retried once
  with the identical question, the attempt ordinal keeping the two apart in the answer cache, then the
  item takes its deterministic draft and is recorded `defaulted`. The group record's outcome for an
  item is one of `answered`, `re-asked` (the retry gave the letter), `defaulted` and `failed` (its
  call never completed); these are record values (`asks.Outcome`). `AskGroup` launches the first item
  of a group of more than one alone and waits for it, so the server prefills and caches the group's
  shared prefix once, then launches the rest.
- **Compose.** Each kind composes its state through `MultichoiceItem.Compose`, its kind's template
  rendered from the item's template inputs. `asks.RecordedItems` builds items from a group record
  through the same constructor.
- **`internal/asks/call`** contains the caller, imported by `build` alone: `Caller.Launch` puts each
  question to the decision role's model through the item's slot, and `PassageLaunch` makes the overview
  call through `Slot.Chat`. It writes the per-call captures and the answer cache (see
  [the answer cache](#the-answer-cache)). A chat capture's request record has the effective
  `temperature` and `thinking`, the values `model.SamplingOf` reports. A decision capture's question
  has its `true-letter` where it has one. The overview capture's request has its `excerpts` and, on a
  retry, its `rejected-lines`, the inputs `asks.OverviewPrompt` renders. Each file is written once,
  after the item that made the call has released its permit (§2, I10 and I11).
- **Batch records.** `Batch.Wait` returns a record for each group. A batch that stops still returns
  its records: an item whose call never completed is recorded `failed`, one cancelled or never launched
  is absent, and so is a group with no item that made a call. The stage writes the records before it stops. A
  finished batch records every group it was given. A batch made by `asks.NewReplayBatch` records a
  failed item and continues.
- **Templates.** Templates (`*.tmpl.md`) and fragments are embedded files (§2, I7). A
  prompt is a `@!slot!@` template, and a call's system prompt is the fragment named for it:
  `overview-system`, the one system fragment, is the overview call's. A multichoice question has no
  system message. `asks.Render` builds the one user turn. Tests key on the offered letters, the slot
  set, the system fragment and the closing line's letters, and none asserts a template's prose. A
  template change is an experiment with a measured result.
- **Overview.** The overview text is written from excerpts composed deterministically from the tree,
  checked for structure and retried once through the `overview-correction` fragment. A call ending
  `truncated-empty` fails the stage without that retry.

### Credentials

**Pattern:** Local secrets file with a redacting secret type.

**Store.** `internal/auth` keeps one file per provider, `<provider>-key.enc` (`auth.FileSuffix`), in
`auth.DirName` under `statedir.Base`. The directory sits beside the per-KB state directories, whose
16-hex-digit names it cannot collide with. The name is lowercased, and `auth.CheckName` runs before any
path is built from it. `auth` imports no `config`, so `auth.NamePattern` restates
`config.ProviderNamePattern`, and `cmd/auth_test.go` pins the two equal. A key is sealed with
AES-256-GCM under a per-user key that HKDF derives from a constant in the source and the OS username
(`Store.sealer`). The constant is public on purpose: a key chosen at build time would leave each
release unable to read the last one's store, and disguising a shipped constant hides nothing. The
sealing is obfuscation, not a boundary: any process of the same user recovers the key, and separation
between users is the file's mode. `auth.FileMode` applies on creation and on every replace
(`atomicfile.WriteMode`), and `auth.DirMode` on every `Add`. The provider's name is not bound into the
file, so a file renamed by hand opens under its new name. kbase uses no OS keychain, and no reason is
recorded for that.

**Secret type.** `secret.Key` is the type every key travels in. fmt's verbs, a text, JSON or YAML
encoding and a log value all render `<redacted>`, and `Reveal` alone returns the text.
`cmd/revealpolicy_test.go` allows `Reveal` in four production files: `internal/auth/auth.go`, the two
wire clients, which set the auth header and scrub an echoed key, and `internal/model/transport.go`,
which truncates a status error's body around a key.

**Read path.** `cmd`'s `loadConfigContext` reads every file of the store through `internal/auth`
(`readStoredKeys`) before the load and gives `config.Load` a `config.StoredKeys`: each key that
opened, by the lowercase name its file is stored under, and a `config.StoreFault` per file that did
not, built from the `*auth.FileError`'s path and error apart. A store directory that exists and does
not list is one nameless `StoreFault` against the directory, with no keys, and the load goes on.
`config` imports no `auth`.

**Precedence.** `keyKind` is the single source of truth for which key an entry uses: the key stored
under the provider's name, whatever layer supplied its table; else the table's `apiKeyFile`, which
`LoadProviders` then reads, and which a provider with a stored key never reads, so it raises neither a
read fault nor a permission warning; else none (SPEC §9 The files are the one source of inference).
`keyFileExposure` warns on a key file readable beyond its owner, except on Windows (SPEC §9.4 Key-file
permission warning).

**Faults.** `config.Load` turns each store fault whose name a configured provider has into a warning
`ProviderFault` naming that provider, with `Source` `SourceStoreFile`, `Path` the store file and
`DoesNotOpen` the store fault's. A file no provider names is read by nothing and not reported. The
nameless directory fault names no provider and becomes a warning with `Source` `SourceStoreDir`,
`Path` the directory, which `warnFaults` writes as the credentials store's. A `ProviderFault`'s `Reason`
never repeats its `Path`; `ProviderFault.LocatedReason` leads a store fault's reason with its path for a
line that is complete on its own (`warnFaults`' and a provider's `fault` in `show-config`). `config.Report` reports
a store fault as its path and reason, with `config.AuthAddRemedy` as the remedy where the file does not
open and for a provider dropped because its `apiKeyFile` could not be read (`ProviderFault.KeyFileUnread`),
the stored key outranking the file. It alone computes a `file` key's `upgrade` and the warning that a
stored key leaves an `apiKeyFile` unread, so `show-config` alone emits them.

**Input and transport.** `kbase auth add` reads the key from stdin only, at most `authAddInputLimit`
bytes (§12); it reads one byte past the limit to tell more from exactly. A configured key
sent to an `http://` base URL on a host other than loopback travels in cleartext. `build`, `score` and
`probe` write one stderr line saying so through `warnCleartextKey` (a build writes one per endpoint),
which names the host and never the key, and proceed. No switch turns the warning off.

---

## 8. Ledger, state store and monitoring

**Pattern:** Checkpoint commits, a content-addressed answer cache, and a background job with a status file.

`internal/gitledger` is the only package that execs git (§2, I1). It execs the host's
git, which SPEC §2 Given interfaces requires, so the user's configuration, hooks and attributes apply,
and no git library is a dependency. `internal/build` owns the stage pipeline, the state store's writers,
resume, `--no-inference`, `--through`, progress and cancel. `internal/buildstore` owns the stage
vocabulary, the store's reading, status and the reading of checkpoint commits. Which packages and `cmd`
files import `gitledger` is in [§11](#dependency-rules).

### Checkpoint commits and resume

**Pattern:** Checkpoint/restart pipeline; stage = idempotent unit.

Whether a build is fresh or a resume is read from the checkpoint commits alone; nothing in the state
store is authoritative for position (SPEC §6 Git holds the recoverable record). The checkpoint commits
are `gitledger.Repo.Trail`: every boundary commit reachable from `HEAD` above `Repo.Removal`, the newest
non-boundary commit that took `kb-root/` out of the tree, so a removal ends the commits below it.
`buildstore.CurrentTrail` takes the newest build's part, back to its newest `start` boundary, so a fresh
build over a removed `kb-root/` starts a set of its own. The four tracked build records
(`kb-build-*.yaml`: node pass, unmarked, classification and cull) are read and written by `claimgraph`,
and `build` commits them.

- **Inputs.** Once it has found the repository, `build.Run` makes a fresh build's top `.tex` files and
  `--bibliography` files absolute against the repository root and sorts them by path (`inRepository`)
  before anything reads them: the launch checks, the document graph and the regeneration of records.
  Every boundary records them, and `start` also records `started-at`, the `HEAD` before it; on a branch
  with no commit it first makes an empty `kb-start:` commit. The line keys and their readers are
  `internal/buildstore`'s (`RecordedInputs`, `StartedAt`). `--resume` takes the inputs from the newest
  boundary in the order recorded, never re-sorted (`takeInputs`; SPEC §5 Inputs).
- **Document graph tree.** `docgraph.Build` writes it, then every leaf is rewritten with each link
  pandoc wraps around a cross-reference unwrapped to that cross-reference (`build.unwrapTree`), in the
  build and in the regeneration of records alike.
- **Interrupted stage.** A stage writes its build records and its `kb-root/` work once its model calls
  have all returned, so a stop during the calls leaves those paths as the boundary left them, and a kill
  during the write leaves them partial; either way they are uncommitted. A resume restores those paths
  to the last stage commit (`Repo.Restore`) and re-runs the stage whole. The answer cache supplies the
  answers already obtained, so the re-run makes no call it already has an answer for. The re-run mints fresh ids, and nothing outside the restored paths referenced
  the old ones. A classify prompt names claim ids, but those were minted in stages already committed
  and the restore reaches back only to the last boundary, so a resumed run's prompts are identical and
  the cache answers them; a fresh build's are not and it makes the calls again. Resume reads no build
  record for position. The progress record accounts for the dirt an interrupted stage leaves
  (`uncommittedIn`); the dirty-path refusal (SPEC §5 Starting over) applies to dirt it cannot account for.
- **Resume command.** The resume command is recorded as the argv `kbase build --resume` with every flag
  the launch had but `--through` and the inputs; `--config-dir` is written absolute. `walk.result`
  emits it only where an identical repeat continues the work: a `bounded` run gets none, a refusal that
  recorded no boundary leaves nothing to continue, and none is emitted where `kb-root/` does not exist
  when the run ends. A document renders it in the form each interface takes (see
  [per-interface rendering](#per-interface-rendering)). It is also emitted only where the recorded argv
  parses as a `kbase build` invocation (`buildstore.RecordedCommand`, SPEC §6 `kbase status`): the state
  store is a directory the caller names, so a record naming any other program is repository-supplied
  text, and no document renders it for a harness to run.
- **Progress units.** Units are leaves for the node pass, source groups for `references-found`, the
  multichoice question groups of each of `claims-culled`'s two passes (each announcing its own total) and
  question groups for classification. A defaulted item is a `fallback` event. A stage's `units` event
  is written before its model calls and its unit and `fallback` events in one append after them, so
  `status` changes between concurrent call batches. Every build record is written once per stage, after
  its calls.
- **`--no-inference`.** The overview call is the dropped step and no `README.md` is written
  (SPEC §5 Determinism; §4).

### Preconditions

**Pattern:** Precondition checks.

`run` refuses a `--resume` with nothing to resume as soon as it has the checkpoint commits, `kb-root/`
absent among the causes (`takeInputs`, SPEC §5 Starting over). `prepare` refuses a fresh build over any
`kb-root/` that exists with `buildstore.KBRootExists`, the refusal `init` gives, and drops the paths
under the absent `kb-root/` from the fresh build's dirt check, since its `start` boundary commits the
removal. `buildstore.TrailRefusal` refuses commits that record a stage past one they leave unrecorded,
in `Stages` order (`check: trail`), right after reading which stages are recorded and before the
launch checks and any restore: walking the gap would run a stage over the products of stages meant to
follow it. A `.git/index.lock` left by a kill during a boundary commit is refused by name
(`Repo.IndexLock`).

### Regenerating missing records

**Pattern:** Reproducible-build verification.

The document graph's records live in the state store. A resume that finds them absent re-runs the
document-graph stage into scratch (`walk.ensureRecords`) over the recorded inputs and requires its tree
byte-identical to the committed `kb-root/` (`firstDifference`); a difference is `failed`, naming the
first differing path (SPEC §5 Determinism).

### The answer cache

**Pattern:** Content-addressed memoization cache.

`internal/asks/call` keeps one answer per call under
`scratch/trails/<start-commit>/answers/`, written through `atomicfile`. A chat answer is keyed by the
digest of the model, the system prompt, the prompt, the attempt ordinal and the client's
sampling, which contributes key parts only where it is not kbase's default (`samplingKey`). A decision
answer is keyed by the model, the question whole, the state and the attempt ordinal. The attempt
ordinal is in the key so a retry reaches the model instead of finding the first answer.
`AskTotals.WarmCacheReadTokens` and `CallStats.CachedTokens` read 0 on every decision call: neither
decision API's usage block has a cached-token field, so the zero is the API's.

### State store layout and format

**Pattern:** Versioned on-disk schema with sequential migrations.

The state store's contents and path are SPEC §6 The state store is versioned and SPEC §6 Git holds the
recoverable record. `--state-dir` defaults to `statedir.Dir`: `statedir.Key` under `statedir.Base`
(§12). The layout:

- `format`, and `lock`, the run lock's second file, with its holder record ([Locks](#locks));
- `progress.jsonl`, and the rotated `progress-<UTC timestamp>.jsonl` files;
- `records/`: the document graph's records and the pre-pass count report;
- `reports/`: `<stage-id>.yaml`, each stage's records and findings, named by the result's
  `stages[].report`, and the captures of a build or score run the MCP server starts
  (`build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`);
- `scratch/trails/<start-commit>/`, one directory per build, holding `answers/`; `asks/`, one file per
  stage, `<kind>.yaml`, one YAML document per group (the group's shared template inputs once, each
  item's own inputs, its `prompt-sha256` from `asks.AskDigest`, its outcome and its calls), written by
  a stage whose batch stopped as well as by one that finished; `captures/`, every call's request, reply
  and usage; and `replays/<replay-id>/`;
- `scratch/regenerate/`, where a resume rebuilds the records;
- `score/<run>/`, a scoring run's directory ([§6](#the-ledger-and-the-state-directory)).

`<state-dir>/format` is one integer, `buildstore.StoreFormat`, written when a build first takes the
run lock (`readyStore`); an absent file is format 1. `build`, `status`, `wait`, `cancel`, `build-trail`
and the MCP `build` tool read it at open (`MonitorOptions.Open`, `walk.open`), and a store newer than
`StoreFormat` is refused (`buildstore.StoreVersion`; `check: state-dir`, `key` the marked `state-dir`
flag, `path` the store, SPEC §7 Item). A build converts an older store in place, in `readyStore`,
through `storeUpgrades` in `internal/build`: one step per version, each writing `format` when it
completes, so an interrupted upgrade resumes at the step it stopped at, and a gap in the chain is an
error. The chain is independent of the KB's `formats.Chain` ([§12](#format-migration-spec-10)). At
format 1 `storeUpgrades` has no steps.

### State store retention

**Pattern:** Keep-last-N retention.

Only a build or a scoring run prunes; `status` and `cancel` remove nothing. Counts are in
§12.

- **Captured builds.** After the run event, a build keeps the newest `keptBuilds`+1 captured builds
  under `reports/` and removes the rest (`pruneBuildCaptures`, `buildstore.PruneCaptures`). A capture is
  the `.stdout.yaml` and `.stderr.log` pair sharing a stem, ordered by name. It reads no progress record
  to do so.
- **Build directories.** Once its launch checks pass and before its first stage, a build prunes
  `scratch/trails/` (`pruneTrails`). It keeps the current build's directory, named by the commit of the
  newest `start` boundary, and the newest `keptBuilds` others by modification time. On a resume it
  creates the current directory if absent and sets its modification time to now; a fresh build's directory is created by its first model call, so its time is when it
  last gained a subdirectory. The rest are removed whole, with the undivided `scratch/answers/`,
  `scratch/asks/` and `scratch/captures/` directories. `status` computes `cache-entries` and
  `cache-bytes` over every build directory's `answers/`.
- **Progress.** At the same point, a fresh build whose `progress.jsonl` has more than
  `progressRotateLines` lines relocates every line before its own run event to `progress-<UTC timestamp>.jsonl`,
  leaves its run event alone in the record, and keeps the newest `keptProgressFiles` rotated files
  (`rotateProgress`). Nothing reads them. Stage reports are never pruned.
- **Scoring runs.** A scoring run prunes `score/` once its ledger has opened: of its kind (`run-` or
  `dry-`) it keeps the run it names and the newest `KeptRuns`-1 others by `run.yaml`'s start, and removes
  the rest whole. It keeps the newest `KeptRuns` captured runs under `reports/score-*`, by name
  (`buildstore.PruneCaptures`; [§6](#run-retention)).

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
file and a standing file is not by itself evidence of a live holder. Each wait is `buildstore.LockWait`
(§12). The record is written after the lock is taken, so a state directory named by the
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
(§12); a holder that keeps it past the wait gives `filelock.ErrHeld`, which
`write.HeldLockItem` turns into the `retry` item, `check: lock` with `path` the repository root (SPEC §8
One write lock). `write.Run` takes no lock;
the caller holds `LockKB` across it, except the build, which holds the run lock instead. `verify` and
the queries take no lock. The callers are
`cmd`'s `holdKB` (write batches, `update-format` and `init`) and `holdWriting`, and
`score.landValues`. `holdKB` probes `build.RunningOperation` for a held run lock before it waits on the
write lock and again after taking it, so a run that started during the wait is refused at once. A scoring
run's own start rebuild (`scoreRefresh`) and `landValues` take the KB write lock and make no probe: they
write as the run that holds the run lock. A scoring run holds the KB write lock around writing its
results and around its start rebuild, never across the run ([§6](#writing-the-results)).

### Status, progress, cancel and wait

**Pattern:** Background job with a status file, signal cancel and a polling wait.

- **`status`.** The verb is `score.Status`. `buildstore.Status` reads `<state-dir>/lock` and its record,
  `progress.jsonl` and the checkpoint commits, and returns the snapshot in SPEC §6 `kbase status`. Where
  the record names `score`, `score.Status` reports the run in its own shape, from the run's ledger and
  result document, and counts the run's result files while it is in its item loop. Where no live run
  holds the run lock and the record names no score run, it compares the start `score.NewestRun` reads from
  the newest run directory's `run.yaml` with the newest build invocation's in `progress.jsonl`
  (`buildstore.ReadState`), and reports the build where neither exists or the two do not order. A
  build's `stages` lists the stages done and not done in the newest build's checkpoint commits and names
  `build-trail` for their commits. Where no build run holds the run lock and `kb-root/` is absent, or the
  commits above a removal are empty, `status` reads no commits (`buildstore.noBuild`).
- **`build-trail`.** `buildstore.BuildTrail` reads the same checkpoint commits. Each boundary's command is
  `git reset --keep` and then the build resumed with the flags its last run was launched with: the resume
  argv the store's newest run recorded, through `RecordedCommand`, which is the `build` call with
  `resume` alone where no run recorded one that parses as a `kbase build` invocation. `started-at`'s is
  the reset alone (SPEC §6 `kbase build-trail`).
- **`cancel`.** `cmd`'s `cancelScore` signals the holder the record names. For a live scoring run it
  calls `build.StopHolder` and takes the resume call from `score.ResumeOf`; otherwise `build.Cancel`
  does the same through `StopHolder` and reads the resume call from the progress record. `StopHolder`
  sends `os.Interrupt` to the pid and polls `<state-dir>/lock` until it is let go, or ends `retry` after
  `cancelWait`. The holder abandons the calls in flight with nothing written for them, logs the event,
  releases the locks and exits `cancelled` (SPEC §6 `kbase cancel`).
- **`wait`.** `wait` polls `buildstore.ReadState` through `buildstore.WaitState` every `waitPoll`, the
  store alone, until the run it names is superseded or has ended, then renders `status`'s document
  through `buildstore.Status`. It takes no lock and reads no checkpoint commits until that final
  document. A score run's pid is waited for through `score.RunByPID` and the store's record
  (`score.WaitRun`), so `wait` returns the run's own result document, re-emitted by
  `result.Reemit`. Values are in §12.

### The build's development replay

**Pattern:** Offline eval / record-replay regression harness.

`build.OpenReplay` and `Replay.Walk` (`internal/build/replay.go`, tagged `//go:build kbasedev`, as the scoring replay is,
[§6](#the-development-replay)) repeat one build's recorded questions under the
binary's embedded templates and question files, from the inputs the build directory records
(§2, I10), so a prompt, a question file or a model can be refined against one build
without re-running it. The build is the one a hex prefix of its start commit names, or, given none,
the directory with the newest modification time, listed by `listTrails`, the listing `pruneTrails`
orders by.

- **Selection.** `OpenReplay` reads each kind's `asks/<kind>.yaml` (`asks.ReadGroupRecords`) and the
  overview captures (`call.ReadOverviewCaptures`) and selects in this order: the multichoice kinds in
  `asks.Kinds` order, groups in file order and items in record order, then the overview calls by
  attempt. The selection can be narrowed to one kind or `overview`, to the items recorded `failed` or
  `defaulted` and the overview calls `failed` or `rejected`, and to the first N. `OpenReplay` refuses
  (`check: usage`, `key: --replay`):
  - a store with no build directory, and a prefix matching none or more than one build (`path` the
    store's `scratch/trails`);
  - a build recording no group record and no overview capture, and an empty selection (`path` the
    build directory);
  - a selected record or capture with no inputs (`path` that file).
- **Calls.** The replay takes no lock. `asks.RecordedItems` rebuilds the items and `asks.NewReplayBatch`
  puts them to the `decision` role; each overview call is its own item on the `text` role, through
  `call.New` pointed at the replay's directory, every call on the invocation's queue.
  `MultichoiceItem.Compose` and `asks.OverviewPrompt` remain the only composition sites. A call that
  never completes is a `failed` row and the replay continues. Progress is logged as the scoring replay
  logs it, through `buildstore.ReplayProgress`, with `total` the selected rows. `countedLaunch` wraps
  the launch handed to `asks.NewReplayBatch`: it wraps each item's `MultichoiceReader` and counts the
  item by its last call, `done` where it returned no error, `failed` where it returned an
  `asks.IncompleteError`, neither where its error stopped the batch. Each overview call is counted
  the same way, so the last line's `done` and `failed` are `ReplayResult`'s.
- **Output.** It writes only under `scratch/trails/<start-commit>/replays/<replay-id>/`, the id minted
  `replay-<id>`: `replay.yaml` (`replay`, `trail`, `decision_model`, `text_model`, `started`); its own
  `answers/` and `captures/`, in the build directory's shapes; `asks/<kind>.yaml`, its own group
  records; and `comparison.jsonl`, one row per replayed item or overview call, with the keys `kind`, `group`,
  `item`, `attempt`, `original`, `replay` and `prompt_changed`:
  - an overview row has `kind` `overview`, `group` `""` and `item` `overview`, and it alone has
    `attempt`;
  - `original` and `replay` each hold `outcome`, `choice` and `confidence`; a multichoice item's are
    as its record holds them. An overview call's `outcome` is `failed` where its capture's last
    attempt did not complete, else the overview stage's judgement of its reply, `answered` or
    `rejected`, and its `choice` and `confidence` are null;
  - `prompt_changed` is true where the re-rendered prompt's digest differs from the recorded
    `prompt-sha256`. It never writes the build directory's `answers/`, `asks/`
  or `captures/`, nor `kb-root/`.
- **Directory handling.** Setup creates `replays/`, the replay's directory and its subdirectories with
  `os.Mkdir`, never `MkdirAll`, so a build directory the prune removed fails setup rather than being
  recreated, and restores the build directory's modification time, which is the current-build marker and
  the prune's order. After writing `replay.yaml`, the replay keeps itself and the newest `KeptRuns`-1
  other replays of its build's `replays/` by `started` (`buildstore.PruneByStart`), which removes only
  entries of `replays/`. Once the item loop drains, a replay whose `replay.yaml` no longer exists,
  because a prune removed it, removes its directory, and its `replays/` and the build directory where
  they contain nothing else, and ends `failed`; otherwise it writes its group records and
  `comparison.jsonl`, on every exit after it made any call.
- **Outcome.** `ReplayResult` counts the selected rows (`total`), those replayed to an outcome other than
  `failed` (`done`), those `failed`, and as `changed` the rows whose outcome or choice differs from the
  original's. The outcome is `done` when every selected row was put to a model, `cancelled` when the
  context stopped the replay, and `failed` for a fault of the replay itself: a store write, a compose
  error or the replay directory removed during the replay.
- **Verb.** `kbase build --replay [<start-commit>]` is in `cmd/build_replay.go`, reached from
  `cmd/build.go` only through the `buildReplay` hook that file's `init` sets. The tag, the hook,
  `mcp.ExcludeFlag` and `test-dist-no-replay` are as for the scoring replay ([§6](#the-verb)). `--replay`,
  `--kind`, `--failed` and `--limit` are excluded from MCP. Beside `--replay` it takes `--state-dir`,
  `--ask-concurrency` and `--config-dir`; any other of build's own flags is refused, naming `--replay`, as
  are `--kind`, `--failed` or `--limit` without it, more than one positional or one not matching
  `^[0-9a-f]+$`, a `--kind` outside the five (`paragraph`, `unmarked`, `colinear`, `classify`,
  `overview`) and a negative `--limit`. Models and sampling come from the build's own `Provider` function
  (`buildModels`, `cmd/build.go`). The result document has verb `build` and `outcome`, `state-dir`,
  `trail` (the start commit), `replay`, `dir` (absolute), `counts` and `changed`, then the final record
  its outcome names; the `cancelled` record has the keys `kind`, `group` and `item` of the first row
  not asked.

---

## 9. Result emitter

**Pattern:** Single structured-output writer.

`internal/result` owns the YAML result document and its emitter (§2, I5). `result.Write`
builds the document as a `yaml.Node` mapping from the ordered `Field` list it is given: the key order
is the caller's, every string is double-quoted, and a float is written without an exponent. A refusal is
a list of `Item`s and the emitter writes every item it contains; collecting every offender is each
producer's work, and nothing in `internal/result` checks it. `Emit` puts `outcome` first. The result
tests parse each covered verb's stdout as one document and compare its keys with the documented order
(`documentedKeys`, `cmd/resultkeys_test.go`). Production code names `os.Stdout` once, in `main`, and no
test scans for another stdout writer. SPEC §7 Tool results is the contract.

`result.ExitCode` maps `outcome` to the exit code and `result.IsError` says whether the outcome is an
error: `cmd` exits with the first, and an MCP tool result's `isError` is the second. `Emit` ends the
`detail` of each item that ends a `retry` or `failed` document with the outcome's closing sentence. `RetryAsk` and
`FailedAsk` are the single source of both sentences, and a caller passes `Emit` its items without
them. The YAML library is the one named in CONVENTIONS "Dependency Policy".

### Per-interface rendering

**Pattern:** Per-surface presentation layer.

Text that names a tool argument is written once, in a marked form, and each interface formats it as it
takes the argument. Recorded documents and help text are read through either interface, so they contain
the neutral form and the reader's interface formats it.

- **The marker.** `internal/argmark` is the single marker by which text names a tool argument (`Flag`,
  `Positional`, `FlagValue` with its value) or contains a kbase call (`Call`, with an arguments mapping
  whose values are JSON values, lists of mappings among them; `Recorded`, a recorded argv). A marker is
  delimited by two private-use code points, with its kind in the byte after the opening one. It imports
  no kbase package, so every producer of text an MCP reader can receive reaches it; `kb`, `config`,
  `result`, `statedir` and `log` use it.
- **Formatting.** `Spelling.Spell` formats each marker as one interface takes it. At the command line
  that is `--name`, `<name>` or `--name value`; over MCP, `name` or `name: value`. A call is formatted by
  the `Spelling.Call` its caller supplies, which `mcp.Spelling(root, overMCP)` builds from the command
  tree: over MCP, the call of the tool the command is exposed as (`toolCall`); at the command line, a
  `kbase` command line, with the values document of a command that reads one piped in on stdin. A call
  with no `Spelling.Call` stays marked. A marker inside other text is backticked where its formatting
  has no delimiter of its own (every one over MCP, a call at the command line); a marker that is the
  whole text stays bare. Text outside markers is unchanged.
- **Writers.** `result.Spelled(w, s)` wraps a writer so that every string `result.Write` puts on it is
  formatted by `s`, and so is any other text written on it, such as what cobra prints from a command's
  summary and flag usages. The root's stdout is such a writer for every subcommand but `mcp`, set in
  `PersistentPreRunE`; the MCP server wraps each tool call's output the same way. A writer already
  `Spelled` keeps its formatting, settled where the call came in (`documentOut`, `cliOut`). A document
  written on any other writer keeps the marked form.
- **`result.Verbatim`.** The KB text a result passes through (a title, a rationale, a context, a leaf's
  line, a conversion list's id and text) is a `Verbatim` and is written unchanged on every
  interface. Only text kbase composes is formatted, because KB text may contain the same code points
  and is the author's. `Items.Error` formats for the command line, since an error's text is read on
  stderr and in logs, and `internal/log` formats every line it writes the same way.
- **Recorded documents.** The progress record's items and a score run's result document beside its
  ledger contain the marked form, because each was written on a writer that formats nothing. `status` and
  `cancel` emit them again through `result.Emit` on the writer of the call reading them, so a recorded
  item and the recorded `Resume` argv reach each interface in its format; the argv goes through
  `buildstore.RecordedCommand`'s `argmark.Recorded` call and is read with `build`'s own flags into an
  arguments mapping where it is formatted. `wait` re-emits a score run's document through
  `result.Reemit`. The store's layout is unchanged: a marker sits inside the string fields, and
  `Resume` stays an argv.
- **Detached runs.** A run a tool call launched is told its document is read over MCP by `mcp.SurfaceEnv`
  in its environment, set by `Launch.start`. `cmd/main.go` reads it before `Execute` and wraps the
  process's stdout, so the run's own document, which the launch hands back where the run exits first, is
  the tool's.
- **Checks.** `cmd/argspelling_test.go` enforces the layer: no MCP rendering contains command-line
  formatting, no command-line output contains a marker, every marker names an argument, and a refusal's
  argument and a remedy's call are in the form of the interface that receives them.

---

## 10. Compatibility harness

**Pattern:** Black-box integration tests with golden fixtures.

Compatibility is judged by kbase's own readers and checks, never by a byte diff of presentation
(§2, I4). The hermetic check is `just test-integration`, whose nine members each judge
what their process produced:

- `test-integration-results-fixture`, `test-integration-init`, `test-integration-opscript-results`,
  `test-integration-mcp`, `test-integration-config`, `test-integration-auth`,
  `test-integration-parity-adjagent-sheet` and `test-integration-refused-write-adjagent-sheet` run
  `./bin/kbase` over fixtures they stage. They judge the result document against the verb's keys, the
  tree it wrote, the transcript it served, or the streams and files a credentials command left. The
  parity recipe judges the frozen answers the 2.0.1 binary gave over the KB the conversion reads, each
  moved only by a rule naming a designed outcome of the conversion. Each runs the script of its own name
  under `tools/scripts/` and writes its log under `test-data/transient/<recipe>/`.
- `test-dist-no-replay` judges the default build's command tree for the compiled-out replay.

The recipes that the rule in CONVENTIONS.md "Testing" keeps out of that check run by name;
`test-integration-mcp-build`, the MCP notification probe that builds a paper, is one.

Unit tests use golden-tree conformance fixtures: `internal/formats/testdata/` contains old bytes in and
new bytes out for each converter, with decoded-values goldens for the record files and a README naming
what each pair pins, and `internal/sheet/testdata/` contains the claim-graph sheet fixture and its DOT
goldens. A conversion or decode that differs from its golden is written under
`test-data/transient/unit_tests/formats/got/` with the golden's name, and a golden is replaced only by
copying that file over it.

---

## 11. Package map

**Pattern:** Package responsibility listing.

The table lists every package by role. The dependency rules below it are the whole rule, and the
import graph they allow is acyclic. A row states what the package is for; the Mechanism column links
to the section that owns how it works.

| Package | Responsibility | Pattern | Mechanism |
|---|---|---|---|
| `cmd` | cobra root; one file per subcommand; `surface.go` is the single registration file | Generated API surface from a command registry | [§12](#mcp-server-spec-11) |
| `internal/argmark` | The marker by which text names a tool argument (`Flag`, `Positional`, `FlagValue`) or contains a kbase call (`Call`, `Recorded`), and `Spelling.Spell`, each interface's formatting of it. Imports no kbase package | Per-surface presentation layer | [§9](#per-interface-rendering) |
| `internal/result` | The result document and its emitter; the outcome-to-exit table; `Spelled`, `Verbatim` and `Reemit` | Single structured-output writer | §9, [§9](#per-interface-rendering) |
| `internal/latex/pandoc` | Reader, writer and defaults-run interface to pandoc (I1) | Wrapped external tool with a version gate | §5 |
| `internal/latex/prepass` | The two source scans (I3) | Preprocessor registry with instrumentation | §5 |
| `internal/docgraph` | JSON transforms, whole-volume render and cut, outline, records, partition checks, `validate_build` | Pandoc AST filter | §5 |
| `internal/records` | The record schema `docgraph` writes and `claimgraph` reads | Sidecar metadata stream | §5 |
| `internal/export` | The LaTeX export: `export.Run`, one sequence over unexported steps (the traversal, the inputs of `gen-tex/`, the embedded filter and template, the starter `preamble.tex`, the report) | Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked output directory | §5 |
| `internal/kb` | The kb-root model: names, exclusions, links, the format stamp, the record loader `LoadGraph`, the scoring weights, `Source` (the read path), and re-exports of the live format `format400` | Domain model and schema package | [§6](#the-kb-package) |
| `internal/kbload` | `Open(kbRoot)`: reads the stamp, builds the `kb.Source`. Imports `buildrecords` for the root paths an older KB keeps its build records at | Schema validation at load, fail-closed | [§11](#loading) |
| `internal/formats` | A tree of packages, one per format and one per converter; no file I/O. The root holds `Chain`, the converter list and the version arithmetic (`ParseVersion`, `CompareMinor`) | Chained schema migrations (upcasters) with golden tests | [§12](#format-migration-spec-10) |
| `internal/formats/common` | What two or more of the superseded formats and converters use and no format defines (`Files`, `Warning`) | Chained schema migrations (upcasters) with golden tests | [§12](#format-migration-spec-10) |
| `internal/index` | `Derive`, cycle breaking (`BreakCycles`), strength, summary pages, `RefreshReporting`, `UpdateFormat`, `PendingConversion`, the link checks, `SectionNodes`, `DependsGraph.Path` | Topological fold over a DAG (Kahn's order); materialized views rebuilt after each write | [§6](#the-derived-layer), [§6](#cycle-breaking) |
| `internal/write` | The write batch (`Run`) and the ops, values, store and render code behind it; provenance; the build-only ops | All-or-none validated batch with dry run and per-file atomic writes | [§6](#write-api), [§6](#build-only-ops) |
| `internal/atomicfile` | The atomic file writer; `WriteMode` sets a file's permission exactly, umask and a replaced file's mode aside | Write to a temporary file, then rename | [§6](#write-api) |
| `internal/query` | The queries over `Derive`'s rows: `find`, `show`, `stats`, `edges`, `trace` | Read model | [§6](#the-query-layer) |
| `internal/sheet` | DOT emitter and `dot` invocation: rows, leaves and headings in, SVGs out | Graphviz rendering with a bounded process pool; a materialized view | [§6](#sheet) |
| `internal/buildrecords` | Readers for the four build records and their paths under `kb-root/.build/` (`Dir`, `RecordFiles`) and at the repository root (`RootRecordFile`), where a KB before `4.0.0` keeps them. Imported by `claimgraph`, by `kbdocs` (whether a build made the KB), and by `kbload` and `cmd/update-format.go` (the root paths the conversion moves) | Sidecar metadata stream | §4 |
| `internal/kbdocs` | Readiness stamps, the maintained blocks' fences and rewrite (`Maintain`), README assembly, `SessionRule` and `IndexRule` | Scaffold command | [§6](#scratch-creation) |
| `internal/claimgraph` | The claim-graph stages: conform, inventory, identify, attribute, shortlist, unmarked, order, cull, split, elision (the cull's issuer), equation sites, classify, equations, endcap, hand-named mentions, assemble, write, the build-time check (`gate.go`), the four build records | Novel: the claim-graph stages `claims-declared`, `claims-discovered` and `analyses-split` (THESIS "What is new" item 1). Machinery: candidate generation then pairwise classification | §4 |
| `internal/asks` | Embedded templates and fragments, composer, the multichoice-ask interface; `JudgedHash`, which `score`'s stale check and `write`'s value sets both call | Prompt templates with a lint test | [§6](#the-prompt-shelf) |
| `internal/asks/call` | The caller: multichoice model calls through a `Slot`'s `Decision`, the overview call through its `Chat`; captures; answer cache | Content-addressed memoization cache | §8 |
| `internal/gitledger` | Git checkpoint commits, by exec (I1) | One adapter per external process | §8 |
| `internal/filelock` | The advisory file lock behind the KB write lock, the run lock and the state store's holder lock | Advisory file lock with timeout | [§8](#locks) |
| `internal/build` | The stage walk and each stage's behaviour, the state store's writers, resume, `--no-inference`, `--through`, progress and cancel, the development replay (`kbasedev`) | Checkpoint/restart pipeline | §4, [§8](#locks) |
| `internal/buildstore` | The build's state, read: store layout, holder and progress records, stage vocabulary, `status`, `WaitState`, the checkpoint-commit reading (`CurrentTrail`, `BuildTrail`), `KBRootExists`, `OpenRepo`. Reaches no `model` and no `asks/call` | Versioned on-disk schema with sequential migrations | §8 |
| `internal/score` | The score passes: item text and its input guard, a run's scope and stale check, the parse, the runner, the run's entry (`Invoke`), its ledger, captures, recomputing values and run retention, the development replay. Imports not `model`; imported by `cmd` and `internal/mcp` only | Bounded worker pool over a job list, results written in one batch at the end | [§6](#the-runner) |
| `internal/statedir` | The state base `Base` (`$XDG_STATE_HOME/kbase`, `$XDG_STATE_HOME` defaulting to `~/.local/state` where unset or not an absolute path) and one KB's state directory: the override, else `<key>` under the base, never inside `kb-root/` | XDG state directory keyed by a path hash | §8 |
| `internal/auth` | The credentials store | Local secrets file with a redacting secret type | [§7](#credentials) |
| `internal/secret` | `Key`, the type every key travels in; imports no kbase package | Local secrets file with a redacting secret type | [§7](#credentials) |
| `internal/config` | The configuration grammar, the three roles, `Resolve`, `Select`, the loaded files and templates, the configure commands' rules | Layered configuration | §7 |
| `internal/model` | `Queue` and its `Slot`, the only exported entry to inference; the two wire clients | Counting semaphore with a capability token | §7 |
| `internal/modelclient` | The client registry (`Registry`, one `model.Client` per endpoint, protocol and sampling) and the mapping of configuration's `api`, decision API and sampling onto `model`'s types (`chatProtocol`, `decisionsSurface`, `samplingOf`) | Factory registry with lazy, shared clients | §7 |
| `internal/mcp` | The MCP exposure of the command tree: tool table, help renderings, resources, instructions, the detached build and score launches. Imported by `cmd` alone | MCP server generated from the command tree | [§12](#mcp-server-spec-11) |
| `internal/mcp/protocol` | JSON-RPC framing, dispatch, the `Tool`, `Schema` and `Property` types, the frame checks. Declares no subscribe capability | MCP server generated from the command tree | [§12](#mcp-server-spec-11) |
| `internal/log` | The logging interface; formats each marked argument for the command line | Per-surface presentation layer | [§9](#per-interface-rendering) |
| `internal/version` | `Current`, the version string | Named constants registry | §12 |

### Loading

**Pattern:** Schema validation at load, fail-closed.

Every KB file is read through `kb.Source`, built by `kbload.Open` (I8). `Open` constructs a new
`Source` on each call and no package-level state passes bytes from one load to the next. An older KB
arrives already in the current form, and no verb sees the on-disk bytes of an unmigrated one.
`cmd/readpolicy_test.go` enforces the single read path ([§11](#dependency-rules)).

The loader keeps the conversion record. `Source.Migration()` contains the chain's `From`, `To`, the
one-sentence `Note` (`kb.MigrationNote`) and the conversion's `Warnings`. The record is set where the
source was migrated and kept across the save that stamps the current format. Every call on an older
KB until that save therefore returns the `migration` record and writes its stderr lines: the
sentence, then one per warning (`migrationNotice` in `cmd/update-format.go`;
SPEC §10 The migration announces itself). `build` is the one loader that returns it nowhere: a
mid-build KB is not a finished state, and its checkpoint commits own that history.

`update-format` alone saves a migrated source (`index.UpdateFormat`). Every other writer refuses it
through `index.PendingConversion` before it writes: `write.Run`, `score`'s verb before its start
rebuild of the derived views, and the build's resume (`loadFormat`). No write lands on an unconverted
tree. The conversion's result is formatted in [§12](#format-migration-spec-10).
`kb.MigratedSource` carries the paths the conversion made obsolete, and the save removes them.

**The strict load** is `kb.LoadGraph`'s, at the load of every command that builds a state.
`format400.DecodeNodes` and `DecodeEdges` read both record files and return every
`format400.GraphFault{Check, Path, Line, Key, Allowed, Remedy, Detail}`, before any state exists:

- a file that does not parse;
- an unknown or missing field, a scored record's `judged-hash` among them;
- a wrong type or a value outside its domain;
- an id outside its kind's grammar, or a node id twice;
- a pairing `BearingLegal` refuses.

No summary page is rendered and no op runs. An absent record file is a fault of its own
(`check: index`, remedy "restore it from git"). The faults arrive as one `kb.GraphLoadError`, which
`index`'s refusal path (`graphItems`, `internal/index/derive.go`) maps to `result` items: `check:
metadata`, the field as `key`, the file and line. Every reader of a state, `write` included, refuses
with them.

**The integrity faults refuse too** (SPEC §10 The load: every fault refused); there is no degraded
state. Where both files decode clean, `kb.LoadGraph`'s `edgeIntegrity` adds a fault per edge record
naming a node no record has and per second record of one edge identity, each `check: metadata` with
`kb.ConsistentRecordsRemedy`. A cycle among the scoring edges is `index`'s: `deriveGraph` runs
`BreakCycles` on every load ([§6](#cycle-breaking)), and `Derive` refuses each edge the cycle
breaking removed as a `check: metadata` item naming it. The cycle stays out of `kb.LoadGraph`
because the build's cycle-breaking stage reads, through it, the records its own edge batch left,
cycle included. `index.DeriveOverCycles` is the one read that takes the removed records instead of
refusing. `write` uses it for the two build-only ops that run inside that window, `add-build-edges`
and `demote-claim-edge` (`opSpec.overCycles`), and the records at every checkpoint commit hold no
cycle.

**The rigor ceiling is checked at the same load.** `kb.LoadGraph` refuses a node record whose
`rigor` is above `kb.RigorMaximum` as a `check: metadata` fault naming the record and the field. The
`format400` decoder takes such a value because its rigor domain has no top. A value at the maximum
is no special case: an authored `rigor: 1.0` is in domain, and an axiom's `1.00` is `kb.AxiomRigor`'s
by rule, the same number and not a distinct read value.

### The loader's file set and its other refusals

**Pattern:** Schema validation at load, fail-closed.

A KB document that is a symbolic link is refused, not followed: `check: symlink` at the link's path,
raised by the document walk and by the conversion's own read of the KB's files alike, so no link out
of the tree is read or written through. A conversion from `1.0.0` holding bytes it cannot carry
refuses before any state is built: `check: conversion`, `path` the file, `line` and `detail` the
content. A `2.0.x` KB the `2.0.x` load refuses, and a `3.0.0` KB whose record files the `3.0.0` load
(`formats.GraphLoadError`) refuses, are refused with that load's own items, `check: metadata`
(`kbload`'s `conversionRefusal`).

The stamp is read from `kb.FormatFile` and, where that file does not exist, from the entry point's
superseded frontmatter (`kbload`'s `formatStamp`). A KB stamped in neither place is a `check:
kb-format` refusal naming both. Where the file exists its content alone is read, the frontmatter
beside it unconsulted.

### The zero-value hazard

**Pattern:** Go zero-value pitfall; required-field decoding.

A record file decoded outside `kb.LoadGraph` (by a test, by a tool, or by a command that read bytes
of its own into a struct) reads a missing `bearing` as the zero `kb.Bearing`. That is the value
`0.0` and a judgement: a premise contributing zero to the strength formula. The strict decoder
refuses a missing field for that reason, and the loader monopoly therefore carries a type's meaning
and not only a version's: every decode of a record file goes through `kb.LoadGraph`, over `Source`
bytes.

### Dependency rules

**Pattern:** Architecture fitness functions.

`cmd/importpolicy_test.go` checks the rules below over the import graph of the module's production
files, with `cmd/readpolicy_test.go`, `cmd/revealpolicy_test.go` and the exec policy beside it. A
rule marked `direct` in the test checks each package's own imports only; any other rule checks the
transitive graph, so a package outside its list that reaches the named package through any chain
fails the test. A rule that names a `cmd` file is checked at file level (`TestCmdFileImporters`).

- `docgraph` knows nothing of claims. The record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files.
- `claimgraph` never imports `latex/*`, and reaches neither `model` nor `asks/call`, transitively.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` reads the build records through it.
  `sheet` reads no build record and imports no kbase package but `kb`.
- `argmark` and `secret` import no kbase package. `argmark` is therefore reached by every producer of
  text an MCP reader can receive.
- `log` imports `argmark` and nothing else of kbase: every line it writes formats a marked argument
  as the command line takes it, so no log line contains a marker. A package that imports only `log`
  reaches `argmark` through it.
- `kb` imports no other kbase package but `log`, `formats/format400` (the live format it re-exports)
  and `argmark`. `docgraph`, `index`, `write`, `query`, `sheet`, `kbdocs` and `claimgraph` read the
  kb-root vocabulary, the link relation (in its two readings, one resolver), the label-name grammar
  and id minting from it. `export` reads the link relation a third way, `kb.ChildList`: an index
  page's child-list lines only, so a body link is never a navigation-tree edge.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph` or `build`.
- The `formats` tree imports nothing of kbase outside itself (I8), so maintenance of a package
  outside it can never change what an old KB converts to. It takes and returns files as bytes, and
  owns the parsers of every superseded format and the live format's definition. Outside the tree,
  only `kbload` imports the root and only `kb` imports `format400`; `kbload` reaches `format400`
  only through `kb` and the root. The write side drains the conversion's delete set; the tree
  touches no file. Within the tree imports run one way: `common` imports nothing of the tree; a
  superseded format package imports `common` and earlier format packages; a converter imports
  `common` and format packages up to its target; the root imports the converters, `common` and the
  format packages. `format400` imports no kbase package, `common` included, and restates any
  declaration it shares with an earlier format. No package but the root imports a converter, and none
  imports a later format (`formatsTreeViolations`).
- A file is opened by path only in the files `fileOpenOwners` in `cmd/readpolicy_test.go` lists, each
  for the purpose it names. A KB's files (the documents, the index and the build records beside
  them) are read through `kb.Source` alone, which `kb/source.go` implements and `kbload` constructs.
  Only `kbload`, `docgraph/validate.go` and `claimgraph/tree.go` construct a `Source` with
  `kb.OnDisk`, and only `write/store.go` and `index/refresh.go` call `Source.DiskFile`.
- `kbload` is imported by `cmd`, `build` and `write` only; `index` does not import it, since a
  refusal is a `result.Refusal`, one shape. `kbload` imports `buildrecords`, `kb`, `result` and the
  `formats` root; `buildrecords` stays free of `kbload`.
- `mcp` is imported by `cmd` alone and reaches no `model`, no `asks/call` and no `build` (I9);
  `mcp/protocol` imports `log` only (and `argmark` through it).
- `internal/model` imports no kbase package but `secret`, the key's type, and no `os`: it touches no
  file and knows no role or consumer (`TestModelImportsNoOS`).
- Only `asks/call`, `build`, `modelclient` and three `cmd` files import `model`: `cmd/build.go`
  (the build's role closure), `cmd/score.go` (the score verb's model call) and `cmd/probe.go` (the probe's
  one call). `internal/model/modeltest` imports it as test support, which nothing shipped imports.
  `modelclient` is the only constructor of a `model.Client`, and a verb opens one only where it is
  about to make a model call. `modelclient` imports no kbase package but `model`, `config`, `result`
  and `log`, and `cmd` alone imports it. Only `build` imports `asks/call`.
- `internal/config` imports no kbase package but `atomicfile`, `result`, `argmark` and `secret`. It
  holds the grammar, the three roles, `Resolve`, `Select`, the loaded files and the templates, and
  refuses in `result.Refusal`. It holds the configure commands' rules too
  (SPEC §9 Provider commands): `ConfigureModels` (the references given, the match of each provider
  part against the pool of the written layer's stack, each reference against its provider's
  catalogue, and the targeted write); `ProviderEntryRefusals` (an entry's arguments); and the
  targeted editors in `write.go` (`UpdateConfig`, `SetProvider`, `RemoveProvider`), with one line
  editor and one decode-compare check for both files
  (SPEC §9.5 The configure commands' targeted update (byte-preservation contract)). It reads a
  catalogue through a function value its caller supplies (`cmd/configure-models.go` binds it over
  the registry's `Catalogue`, under the `--timeout` deadline), and returns the catalogue misses and
  what the run wrote as data, from which the verb writes its stderr lines. It never imports `model`
  or `log`, opens no client and writes no output, and returns its faults as data, so each caller
  decides whether one is a warning or a result field. A refusal's remedy may name the command that
  fixes it (`kbase configure-providers`, `kbase configure-models`).
- `internal/score` imports `model` nowhere: the endpoint is below the verb's model-call interface,
  so the runner, the input guard, the parse and the ledger learn nothing of what answered. It imports
  `write` for a writing-the-results batch's ops and its lock, and never `build`, `kbload`,
  `statedir` or `kbdocs`: the verb takes the run's locks, opens the KB, resolves the state directory
  and runs the start rebuild of the derived views. A batch reopens the KB, to compare each value's
  judged hash with its item's text as the text is now, only through the function the verb supplies
  (`score.Invocation.Open`, `kbload.Open` in `cmd/score.go`). Only `cmd` and `internal/mcp` import
  `score`.
- The judged hash is defined in `internal/asks`, which owns the templates its hash covers: `score`
  imports `write`, so the function cannot be defined in `score`, and `write` imports `asks` for it
  alone. Neither reaches `model` through it.
- Only `build`, `buildstore` and, within `cmd`, `cmd/update-format.go` (its clean check, and the
  `git mv` of each root build record into `kb-root/.build/`, `Repo.Move`) and `cmd/write_op.go` (a
  committing batch's commits, made where every write interface runs its batch, and
  `update-format`'s, through the same `kbCommits`) import `gitledger`; `gitledger` imports `log` and
  nothing else of kbase. Every opener of a KB's repository goes through `buildstore.OpenRepo`, which
  refuses `worktree` where git's worktree root is not the directory `kb.RepositoryRoot` finds;
  `cmd/init.go`'s create commit opens through it and imports no `gitledger`. `write` runs no git: a
  committing batch's snapshot commits are the caller's `write.Options.Preflight`.
- `buildstore` reaches no `model` and no `asks/call`: the build's state, its stage vocabulary and
  its checkpoint commits are read without the runner, and `build` imports them.
- Only `build`, `cmd` and `mcp` import `kbdocs`: `build` and `cmd` stamp readiness documents (within
  `cmd`, file-level, `init` at a KB's creation and `update-format` rewriting the maintained blocks
  with `kbdocs.Maintain`). `mcp` imports it for the `.index/` rule and the session rule alone
  (`kbdocs.IndexRule`, `kbdocs.SessionRule`), which the instructions carry; it stamps nothing.
- Only `build`, `buildstore` and `cmd` import `statedir`.
- `build` imports `write` (the write-lock wait).
- Only `internal/latex/pandoc` execs pandoc; only `internal/gitledger` execs git; only
  `internal/sheet` execs `dot`; and only `internal/mcp/launch.go` (file-level) execs kbase itself.
  Those four are the exec policy's whole list (`cmd/execpolicy_test.go`): a process started anywhere
  else in non-test code, or a listed owner starting a binary other than its own, fails the test;
  test files are out of its scope.
- `internal/export` imports `kb` and `atomicfile`, and nothing else of kbase; the verb opens the KB
  and supplies every pandoc call (the preflight and the one `pandoc.RunDefaults` call) as a function
  value `export.Run` calls.
- `internal/auth` imports nothing of kbase but `atomicfile`, to write its files at their mode, and
  `secret`; only `cmd` imports it, and `config`'s own rule keeps it from the store.
- A key's text leaves a `secret.Key` through `Reveal` in four files alone:
  `internal/model/openai_client.go` and `anthropic_client.go` (setting the auth header and scrubbing
  an echoed key), `internal/model/transport.go` (cutting a status error's body around it) and
  `internal/auth/auth.go` (sealing it). `cmd/revealpolicy_test.go`, on the module walk, fails a
  production file outside them that names `Reveal`.

---

## 12. Constants

**Pattern:** Named constants registry.

Every numeric value of the implementation is defined here and nowhere else; other sections name the
constant and link to this one. Exit codes and the placeholder's hash form are defined in SPEC §7 and
§3, the pandoc range in SPEC §2 and the checkpoint commit subject in SPEC §6.

| Constant | Value | Defined in |
|---|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6 Git holds the recoverable record) | `statedir.KeyHexDigits` |
| State-store format | 1, the integer in `<state-dir>/format` (SPEC §6 The state store is versioned; §8) | `StoreFormat`, `internal/buildstore` |
| Run lock path | `<git dir>/kbase-operation.lock` (SPEC §6 Git holds the recoverable record; [§8](#locks)) | `buildstore.OperationLockFile` |
| KB write-lock wait | 30 s: how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8 One write lock) | `write.LockWait` |
| Monitor lock wait | 2 s: the wait for the run lock and the holder lock, so a monitor probing one does not turn a starting run away | `buildstore.LockWait` |
| Lock poll interval | 5 ms between attempts while waiting for a holder | `poll`, `internal/filelock` |
| Cancel wait | 20 s: how long `cancel` waits, after signalling the holder, for it to let the lock go before it reports the holder still running | `cancelWait`, `internal/build/monitor.go` |
| Builds kept | 5: the build trails, beside the current one, whose directories (answers, group records, call captures, the development replays) survive pruning; the captured builds under `reports/`, beside the newest, are kept to the same count (§8) | `keptBuilds`, `internal/build` |
| Progress rotation | 500 lines a fresh build's `progress.jsonl` holds before it is rotated, about five builds; 15 rotated files kept (§8) | `progressRotateLines`, `keptProgressFiles`, `internal/build` |
| Runs kept | 16: the scoring runs of each kind (`run-`, `dry-`), the replays of a run's `replays/`, the development replays of a build trail's `replays/`, and the captured score runs under `reports/` that survive pruning ([§6](#run-retention); §8) | `KeptRuns`, `internal/buildstore` |
| Replay progress interval | 10 s: the most often a development replay's walk logs its `replay progress` line ([§6](#the-development-replay)) | `ReplayProgressInterval`, `internal/buildstore` (`kbasedev`) |
| Item-text wrap width | 100 columns: the width each section of the item text is wrapped to before its lines are numbered ([§6](#the-closed-text-and-its-guard)) | `wrapWidth`, `internal/score/text.go` |
| Sheet render concurrency | 4: the most `dot` processes at once; output order is kept ([§6](#sheet)) | `sheet.renderConcurrency` |
| Unmarked shortlist, proximity slots | 3: the nearest targets in its own paper that `references-found` makes model calls about per source (§4) | `shortlistProximityK`, `internal/claimgraph` |
| Unmarked shortlist, wildcard slots | 2: the best-ranked targets by cosine asked about per source besides the proximity picks (§4) | `shortlistWildcardK`, `internal/claimgraph` |
| `stats` leverage bar | 0.65, the strength bar of `highest_leverage`. It is its own constant, not derived from the band ladder, so a change to a band's lower bound does not move `stats` | `query.DefaultMaxStrength` |
| `stats` dependents floor | 1, the dependents floor of `highest_leverage` | `query.DefaultMinDependents` |
| `stats` rank limit | 10: the rows each of `highest_leverage` and `weakest` carries | `query.StatsRankLimit` |
| Ask concurrency | Default 4: the model calls one invocation has in flight, and the idle connections per host its clients keep; `probe` runs a queue of one. Set by `[asks] concurrency` in `config.toml` (SPEC §9.2 `config.toml` — the choices) or `--ask-concurrency` on `build` and `score` | `config.DefaultAskConcurrency` |
| Probe ask concurrency | 1: the `probe` queue's one permit, for its one call | `probeAskConcurrency`, `cmd/probe.go` |
| Transport attempts | 3 per model call, chat and decision alike (§7 classifies what is retried) | `model.ChatAttempts` |
| Transport pauses | 5 s, then 30 s, between attempts: the build's overview call and each of `kbase score`'s model calls. The build's multichoice calls repeat at once. A pause the call's deadline cannot cover is not waited out: the call ends at once, naming the attempts it made | `overviewBackoff`, `internal/asks/call`; `scoreBackoff`, `cmd/score.go` |
| Response-header timeout | 2 min: the handshake alone, from the request written to the response headers read; a stream's body is bounded by the stream idle timeout | `responseHeaderTimeout`, `internal/model/transport.go` |
| Stream idle timeout | 2 min: the longest gap between reads on an open stream before it surfaces as idle | `streamIdleTimeout`, `internal/model/sse.go` |
| Error-body limit | 8 KiB (8192 bytes) of a non-2xx response body echoed into the error | `errorBodyLimit`, `internal/model/transport.go` |
| Messages completion allowance | `max_tokens` is the thinking budget plus `max(256, ceil(characters / 4))` over the request's messages and system text; 256 is the floor | `maxTokensFloor`, `maxTokensFor`, `internal/model/anthropic_client.go` |
| Thinking budgets (Messages) | `low` 1,024, `medium` 4,096, `high` 16,384 tokens (SPEC §9.1 The chat protocol) | `thinkingBudgetLow`, `thinkingBudgetMedium`, `thinkingBudgetHigh` |
| Sampling | `[sampling.text]` and `[sampling.scoring]` in `config.toml`, each `temperature` and `thinking`; unset is temperature 0 and thinking off; `thinking` takes the values of `config.ThinkingLevels`; the temperature ranges are `openai` [0, 2] and `anthropic` [0, 1] | `config.ThinkingLevels` |
| Model roles | `[models] decision` (the multichoice asks, on whichever decision API its entry speaks), `[models] text` (the overview call) and `[models] scoring` (`kbase score`'s calls), each a `<provider>/<model>` reference (SPEC §9.2 `config.toml` — the choices) | `config.toml` |
| Cached-token totals | `AskTotals.WarmCacheReadTokens` (and `CachedTokens`) report 0 on every decision call: neither decision API reports cached tokens, their `usage` being the one shared `wireDecisionUsage`, so the zero is the API's | `AskTotals` |
| `auth add` input bound | 65536 bytes, the most stdin `kbase auth add` accepts; it reads one byte past the bound to tell more from exactly that (SPEC §9 Provider commands) | `authAddInputLimit`, `cmd/auth.go` |
| Credentials store directory | `auth`, under the state base (`statedir.Base`), beside the per-KB state directories, whose 16-hex-digit keys it cannot collide with; mode `0700`, set on every add | `auth.DirName`, `auth.DirMode` |
| Credentials store file | `<provider>-key.enc`, the name lowercased; mode `0600` on creation and on every replace ([§7](#credentials)) | `auth.FileSuffix` (`-key.enc`), `auth.FileMode` |
| Detached-run start wait | 5 s: how long the `build` and `score` tools wait for the child to start, or to exit, before returning the run still starting (SPEC §11 Tools) | `launchStarting`, `internal/mcp/launch.go` |
| Detached-run poll interval | 50 ms between reads of the state store while a launch waits | `launchPoll`, `internal/mcp/launch.go` |
| `wait` poll interval | 1 s: how often `kbase wait` reads the state store while the run it names progresses (SPEC §6 `kbase wait`) | `waitPoll`, `cmd/wait.go` |
| MCP protocol revision | `2025-11-25`, the one revision the server implements (SPEC §11 Tools) | `protocol.ProtocolVersion` |
| Metadata format version | `4.0.0`, the one version read and written (SPEC §10 One version read and written); the stamp key is `kb-format` | `kb.FormatVersion` (`format400.Version400`) |
| Format stamp file | `.index/format.yaml`, the one-key block mapping that holds the stamp. `kb.EncodeFormatStamp` writes it and no package but `kbload` reads it, which falls back to the superseded entry-point frontmatter where the file does not exist (SPEC §10 The stamp) | `kb.FormatFile` (`format400.FormatFile`) |
| Pending literal | `*pending*`, the format's one no-number literal, written for a value not yet judged and accepted wherever a value is absent | `kb.PendingLiteral` |
| Conversion record | `migration`: a result's conditional record after `kb-root`, holding `from`, `to`, `note` (`kb.MigrationNote`, one sentence for every migration) and `warnings`; the same call writes the same lines to stderr (SPEC §10 The migration announces itself) | `Source.Migration()` |

### Format migration (SPEC §10)

**Pattern:** Chained schema migrations (upcasters) with golden tests.

A conversion step is bytes in, bytes out, plus the paths it made obsolete: a converter is a pure
function over `formats.Files` (`internal/formats/from<from>to<to>`) that parses its own superseded
form, reads each side's definition from its format package (`internal/formats/format<version>`) and
emits the next form. The chain's order is the version order and a gap in it is a test failure. The
pending obsolete set lives in the `Source`.

**Pattern:** Read-time upcasting plus an explicit migrate command.

Every verb that reads a KB (`update-format`, `verify`, the queries, the write ops, `export-latex`,
and the build's `spine-seed`, `phase-3a`, readiness stamp, overview and resume) calls
`kbload.Open(kbRoot)`; `status` and `cancel` do not load the KB. `Open` reads the stamp alone, from
`.index/format.yaml` (`kb.DecodeFormatStamp`), else from the superseded entry-point frontmatter
(`kb.FormatStamp`), before anything else.

- A stamp at the current major and minor builds a `kb.Source` that reads disk.
- An older stamp reads every file the format covers (`kb-root/` and the build records beside it) into
  a map of repository-relative paths to bytes and hands it to `formats.Chain(from, to)`. `Chain`
  applies each converter in turn and returns the current form's files, the union of the obsolete
  paths and every step's warnings. The `Source` serves the returned bytes and reads an obsolete path
  as absent. One converter covers every patch of its minor.
- A newer or unreadable stamp is refused `check: kb-format`; no entry point is refused `check:
  kb-root`.

The `formats` tree is the only home of a superseded parser and of the live format's definition
(`format400`, which `kb` re-exports). It imports nothing of kbase outside itself, and `kbload` is the
only importer of its root from outside it (import-policy test, [§11](#dependency-rules)). The five
warning list names (`common.ListIDs`, `ListBareIDs`, `ListUnresolvedCitations`, `ListNoEdge`,
`ListStrengthenBy`) are defined in `common` alone; the `formats` root and `kbload` re-export them,
and `update-format`'s result lists (`formatResultLists` in `cmd/update-format.go`) match on
`kbload`'s: warnings listed `ids`, `bare-ids`, `no-edge` or `strengthen-by` appear under keys of
those names and the rest under `migration`. The save reports the removals as `removed`
(SPEC §8 `update-format`).

**Pattern:** Crash-resumable ordered save.

`index.UpdateFormat` is the save, and only `cmd/update-format.go` calls it. Every other writer
refuses a migrated `Source` first (`index.PendingConversion`; [§11](#loading)). After the load
(`Precheck`, then `Derive`) the phases run in this order, each file atomically:

1. The caller's moves of the build records at the repository root (below).
2. `saveConverted`: `.index/edges.yaml`, then `.index/nodes.yaml`, then every other migrated file but
   the entry point.
3. The summary pages and the sheets.
4. The stamp file, written at the version the tree is still at, so a stamp always exists and always
   names the tree's version.
5. The obsolete paths, less what this save covered.
6. The entry point, its frontmatter (which stamped the superseded form) stripped by the conversion.
7. The stamp file again, at the current version, so the stamp marks a completed migration.

The verb then rebuilds the maintained blocks (`kbdocs.Maintain`, [§6](#scratch-creation)). A save
that stops part-way leaves the KB stamped at the version it converts from, its next load resuming the
conversion. `update-format` refuses that tree as not clean, and the recovery is to restore `kb-root/`
and any moved build record from `HEAD` (SPEC §8 `update-format`). A converter that finds
`.index/nodes.yaml` in place takes both record files as they are (`from200to300`'s `resumed`), since
only an interrupted save writes it, and passes the stamp file through unchanged. `from300to400`
likewise takes as they are the record files that already decode as `4.0.0`, which only an
interrupted save writes over a `3.0.0` stamp.

Build records move with `git mv` (`gitledger.Repo.Move`), the only mover. The verb checks the moved
records clean and hands `index.UpdateFormat` the moves (`index.RecordMove`) and `Move`, which it
calls as the phase after the load and before the save's first write. A refused conversion therefore
moves nothing, and the save finds each record in place and its root path gone. The save itself
writes and removes nothing outside `kb-root/` (`index.WriteKBFile` refuses such a target). Each
moved record is reported in `removed` at its root path, repository-relative, and in `written` at its
new path, kb-root-relative.

#### The `from300to400` converter

- **Records.** It decodes the `3.0.0` records through `format300` and writes them through
  `format400`, every `judged-hash` the zero `format400.JudgedHash`, the unjudged value. It reports
  each dropped `no-edge` and `strengthen-by` field as a `formats.Warning` carrying its list
  (`common.ListNoEdge`, `common.ListStrengthenBy`), the node's id and the text, fields
  `kb.MigrationWarning` carries identically.
- **Ids.** It mints each work's and axiom's id from its `3.0.0` id alone (`mintIDs`: the SHA-256 of
  the old id, then of the old id, a NUL and a counter, recomputed in ascending `3.0.0`-id order while
  the result equals an earlier minted id or any `3.0.0` id), its key or label the record's `label`. It renames
  every `3.0.0` work or axiom id in an edge's end and a prose field, each a warning listed `ids`
  (`common.ListIDs`), the `3.0.0` id under `ID` and the minted one under `Text`. The mapping is read
  off a `4.0.0` node file where an interrupted save left one, and an edge file decoding as `4.0.0`
  still has its `3.0.0` ends renamed.
- **Markers.** `convertMarkers` reads each kb-root document's `3.0.0` markers through
  `format300.StatementMarkerRE` and writes `format400.StatementMarker`s, the `lifts` edges of
  whichever record file form exists deciding the one move. It returns each `3.0.0` summary page,
  `claim-rigor.md` at kb-root and in a top-level directory, as an obsolete path, and the save renders
  the new summary pages.
- **Build records.** It moves each build record at the repository root into `kb-root/.build/`, bytes
  unchanged, its root path obsolete.
- **Citations.** `convertCitations` reads every link of each kb-root document but a summary page
  through `format400`'s link grammar. It resolves each anchor under `3.0.0` slugging
  (`format200.Slugify` over the heading as written) and writes it under `4.0.0`'s (`format400.Slugger`
  over the heading as converted, its bare ids already citations, which render as their link text); a
  document's nth heading is the same heading in both forms. An anchor naming one heading under
  `3.0.0` slugging and another under `4.0.0`'s is left as written.
  - A node's place is read from the converted markers, a `3.0.0` register entry from the records (its
    node's title slugged on the page its host's volume rendered), and a work's anchor on the root
    summary page from the page's heading texts (`format400.SummaryTitle` and the entry headings),
    taken only where no other heading there can take it.
  - No link stays aimed at a removed summary page. A link whose anchor names one entry with a place
    points at that place; one whose entries are all stated in one section points at that section,
    untitled; any other points at the summary page replacing the page (`format400.SummaryPage` over
    the page's own path), without its anchor. Where no node renders on that page, so the save writes
    none, the link goes and its text remains, a bare id in that text converted as any is.
  - A bare id is read in the `3.0.0` id grammar and cited under its minted id. Each `### Axiom
    <label>:` heading of `axioms.md` naming an axiom record ends in its axiom's marker; one naming
    none is a warning.
  - Each conversion of a bare id is a warning listed `bare-ids` (`common.ListBareIDs`). Each link or
    id it cannot resolve to one node, and each link it removes, is one listed `unresolved-citations`
    (`common.ListUnresolvedCitations`), each carrying the id where it has one. Only this converter
    makes them.

#### Golden tests

Each converter has a golden pair as its test. The record files of each `3.0.0` and `4.0.0` golden
tree, `internal/formats/testdata/<tree>-<version>/`, have a decoded-values golden beside them,
`<tree>-<version>.values.jsonl`: one JSON object per record in the files' order, its keys in its
type's field order, a pending value as the literal. That version's decoder, `format300`'s or
`format400`'s, reads the record files to those values (`TestRecordFilesDecodeToTheirValues`).

### MCP server (SPEC §11)

**Pattern:** MCP server generated from the command tree; JSON-RPC over stdio; detached child plus
status polling.

`internal/mcp` builds the server from the command tree its caller hands it; `internal/mcp/protocol`
holds the wire. Nothing is listed by hand.

#### Tool table

**Pattern:** Generated API surface from a command registry.

`mcp.Tools` builds one tool per command declaring `kbase.mcp` = `bound` (`mcp.Bind`); `mcp.Exclude`
declares `excluded` with a reason. `mcp`, `wait`, `auth` and cobra's own `completion` and `help`
declare `excluded`. The table reads the root's commands only (`boundCommands`), so a parent's
`excluded` keeps its subcommands out. `cmd/surface.go` adds every command to the tree, one `AddCommand` per group, its
declaration on that line (`readOnlyTool`, `readWriteTool`, `cliOnly`; for a family tool's operation
`queryOperation` or `writeOperation`, which add `kbase.mcp.family`). It also declares the MCP-only
interfaces: the resource families and the detached launches. It names no project for them:
`mcp.Server.Serve` binds the resources to the server's default project.

- **Read-only.** A command's read-only standing is the cobra annotation `kbase.readOnly`
  (`mcp.ReadOnly`), read when the table is built. A family tool is read-only where every member
  declares it.
- **Families.** Commands declaring one family (`kbase.mcp.family`, set by `mcp.InFamily`) are one
  tool together, in the place of the first. A family is an `mcp.Family` its caller hands it (name,
  description sentence, shared help): `query` (`toolFamilies`, `cmd/surface.go`). A family a `bound`
  command is named for is that command's, its members having no tool of their own: the write ops
  declare the family `write`, and the `write` tool is `kbase write`'s (`mcp.batchTool`), its schema
  the batch (`entry`, a required array of objects; `dry-run` and `commit`, booleans), its arguments
  handed whole to `Calls.WriteOp` as `write.Batch`'s values document. `runWriteOp` runs the batch
  with `ArgumentEntry` set: each refusal keeps its `entry`, and one naming no `path` drops `line` and
  `column`, which would point into a document the caller never wrote, while one naming a document
  keeps its line there. A refusal's `key` is the values key, which no interface formats differently.
- **Command tool schema.** `mcp.CommandSchema` derives it from the `Use` line's positionals and the
  command's flags, skipping a flag annotated `kbase.mcp` = `excluded` (`mcp.ExcludeFlag`), which is no
  argument of the tool: `score`'s `--replay` and `--failed` are such flags, and `internal/mcp/protocol`
  refuses an argument naming one as outside the schema. `required` is the positionals, less an
  optional variadic one that the `Use` line writes `[<name>...]`, plus each flag declared with
  `mcp.RequireFlag`, which is `trace`'s `direction` alone. `wait`'s `--pid`, the one flag a verb
  requires without that declaration, is outside the derivation: `wait` declares `excluded`. No other
  bound command refuses for want of a flag, `score`'s `--pass` among them, a run walking both passes
  unless it is told otherwise.
- **Family tool schema.** `operation` is a string enum of the members' names in the tree's order, and
  `arguments` an object. Each member is built as its own command tool would be, and the family keeps
  it by name, unlisted. A call takes the member `operation` names and checks `arguments` against that
  member's schema with `protocol.Schema.Faults`, the same check `internal/mcp/protocol` runs on a
  tool's arguments, so no argument list is stated twice. A fault is a usage refusal document, one item
  per fault naming the argument, the member's arguments as `allowed`, and a remedy naming `allowed`
  for the argument names and the member's help resource for what it does. Arguments that fit run the
  member as its own tool would. A values op's argument list reads `write.OpVocabulary` (the entry's
  keys in vocabulary order, each with the type, required mark and description its definition in
  `internal/write/values.go` carries), naming no flag, so it is true on both interfaces.
- **Results.** Every tool but `write` runs the command's `RunE` with its flags reset and its output
  captured to a buffer. The buffer is the result text; decoded once more, the YAML document is the
  `structuredContent`; the document's outcome gives `isError` (`result.IsError`). A `RunE` starts from
  `invocationDir(cmd)`, never `os.Getwd()`: the project the call names over MCP, else `--project-dir`
  (which must be a git worktree root), else the working directory (`cmd/mcp.go`). Each tool's result is formatted for MCP and checked for an
  item keyed `--name` or `<name>` of its own arguments (`warnCLIKeys`, a warning only).
- **The tool set is asserted.** `TestMCPToolsAreTheCommandTree` derives its expectation from the
  annotations alone (a family's name for its members, a command's own otherwise) and asserts the tool
  set for equality, so a tool listed by hand fails. `build-trail`, the build's checkpoint-commit
  history, is `cmd/build-trail.go`, a command tool like `status` (`buildstore.BuildTrail`), so its
  schema, key order and refusal keys are derived as any command's are;
  `TestBuildTrailIsOneDocumentOnBothSurfaces` runs it at the command line and through the server and
  requires the two documents to agree. Every tool has a command behind it (I9).
- **Protocol revision.** `protocol.ProtocolVersion` is the one constant; an `initialize` naming
  another revision is answered with the server's, for the client to decide.

#### Help texts

**Pattern:** Self-describing API.

Each command stores four help texts: `Short`, `Long`, an MCP note (`kbase.help.mcp`) and a CLI note
(`kbase.help.cli`). `internal/mcp/help.go` composes three renderings from them, each formatting its
markers for its interface through `mcp.Spelling` ([§9](#per-interface-rendering)):

- the tool description (`Short`, the MCP note, where the full help is), read on `tools/list`;
- the help resource `kbase://help/<command>` (`Long`, the argument list, the MCP note);
- `CLIHelp`, the `--help` text the root's help function prints (the usage line, `Long`, the argument
  list, the CLI note `CLINote`, which is closed by the flags excluded from MCP and, below the root, a
  line naming the global flags).

`ArgumentList` builds the argument list from the definitions a tool's schema reads: a command's
positionals and flags (`commandArguments`), a values op's `write.OpVocabulary`, the batch's
`batchSchema`, and the root's persistent and own flags, each interface typing a list and a mapping
its own way. `TestHelpRenderingsAreTheirParts` (`cmd/helpresources_test.go`) asserts, for every
`bound` command and with and without notes, that each rendering is exactly its parts. The
`measure-mcp-surface` target (`tools/scripts/measure-mcp-surface.sh`, with `TestMeasureMCPSurface`)
measures the always-read size (the `tools/list` result, the input schemas and the instructions) over
the release build. It is a measurement and gates nothing.

The root command's `Long` states the command line's form once each: `queryHelp`, `listQueryHelp` and
`valuesHelp` in `cmd`, the last closing on `batchHelp`, the write batch's contract. The `query`
family's `Help` is the MCP form for `kbase://help/query`, from the same text where the two agree
(`queryReads`, `queryAnswerHelp` and `listQueryHelp`, one marked text that each rendering formats for
its interface); the resource lists the members from the annotations the enum is built from.
`batchHelp` is also `kbase write`'s `Long`, so `kbase://help/write` has the contract in full, and
every write op shares one MCP note and one CLI note (`writeOpCommand`). The members' shared help is
appended to no member's `Long`.

`mcp.ServedInstructions` is what `initialize` reports (`instructions.go`): its opening, which
differs by mode (the default project's state where there is one, the `project-dir` lines where
there is none) and the sentence naming the skill; then `kbdocs.IndexRule` and `kbdocs.SessionRule`,
which the stamped `AGENTS.md` contains in its `index-rule` and `session-rule` slots; then
`mcp.EdgeDirection`, which the root's `Long` contains too; then the paragraphs only the instructions
have.

#### A call's project

`mcp.Tools` wraps every tool in `inProject`. The wrapper adds the `project-dir` property to the
schema, and on each call `callProject` resolves the project: the `project-dir` the call names, which
`buildstore.ProjectRoot` requires to be a git worktree root (the same check `--project-dir` gets on
the command line), else the server's default project. It removes `project-dir` from the arguments
and hands the inner tool the project. No tool closes over a directory: a command tool sets it as the
call's invocation directory, a write op passes it to `Calls.WriteOp`, and a launch starts its child
there.

The default project is `mcp.Located`, whose zero value is the explicit none: `cmd/mcp.go` sets it
from `kb.RepositoryRoot` of the invocation directory only where that finds a repository, so no
reader ever passes `""` to a path function that would read it as the working directory. The server
runs in one of two modes, chosen in `cmd/mcp.go` before serving (SPEC §11 Two modes). With a
default project, `projectDirDescription` describes `project-dir`, that text followed by the
sentences on the server's own project. Where `cmd/mcp.go` finds none, or under
`--project-dir-required`, the project is `mcp.ProjectRequired()`, the zero `mcp.Located`: `inProject`
lists `project-dir` in each schema's `required` with `projectDirMeaning` alone as its description.
The `callProject` refusal, a usage refusal keyed `project-dir`, stays behind that schema for a call
whose `project-dir` is empty. The global `--project-dir` given to `mcp` is a usage refusal keyed
`--project-dir` before serving; startup never fails for want of a project.

#### Wire and resources

`internal/mcp/protocol` holds newline-delimited JSON-RPC over stdio, `initialize`, `ping`,
`tools/list` and `tools/call`, the `Tool`, `Schema` and `Property` types, and the frame checks
(`jsonrpc`, `id`, `method`, known tool, object arguments). Arguments failing the schema (missing,
wrong type, a string outside its `enum`, unknown key) are JSON-RPC `-32602` errors, not refusal
documents; `Schema.Faults` is that check as data, which a family tool runs on its `arguments`.
`Serve(in, out, version, instructions, tools, resources, lg)` takes the server version, the
instructions, the tool table and the resources from its caller, and imports `log` only.

`resources/list` and `resources/read` serve the resources `internal/mcp` supplies: the sheets, under
`kbase://sheet/<kb-root-relative path>`; the model guidance at `kbase://model-hints`, which is
`internal/config`'s rows rendered as markdown and needs no KB; and the help of each family
(`kbase://help/<family>`) and of each command exposed as a tool (`kbase://help/<command>`,
`mcp.HelpResources`), composed in `internal/mcp/help.go`, which needs no KB. An unknown URI is
`-32002` and a missing `uri` `-32602`. `initialize` advertises `resources` with no `subscribe`, and
the protocol answers no subscribe method (`TestNoSubscription`).

#### Detached build and score

**Pattern:** Detached child process plus status polling.

`internal/mcp/launch.go` is the single self-exec owner under I1, file-level, enforced by the exec
policy. The `build` and `score` tools re-execute the kbase binary as `kbase build …` and `kbase
score …` by the same detachment:

- The child runs from the root of the call's project, with the server's environment plus
  `KBASE_RESULT_SURFACE` (`mcp.SurfaceEnv`), which tells the run its document is read over MCP. It
  has its own process group (detach flags per platform in `internal/mcp/detach_unix.go` and
  `detach_windows.go`) and no stdin.
- The server's `--config-dir`, `--log-level` and `--log-file` are passed through, and `--project-dir`
  where the call named one, so the child's recorded `resume` carries it.
- Every path the child is given is absolute, because its working directory is not the server's. A
  relative `state-dir` argument is resolved against the repository root; the server's `--config-dir`
  and `--log-file` are resolved against the server's working directory.
- Its stdout and stderr are files in the state store's `reports/`, `build-<UTC timestamp>-<random>`
  or `score-<UTC timestamp>-<random>` with `.stdout.yaml` and `.stderr.log`. Pruning keeps the
  newest `keptBuilds`+1 build captures (`build`'s prune) and the newest `KeptRuns` score captures
  (`score.PruneRuns`) (§12).
- Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
  `kb-root/`, returns the refusal `status` gives and starts no process.
- The `edge` flag of `score` is declared a mapping (`mcp.MappingFlag`), so its schema property is an
  object of the required strings `source`, `target` and `relation`, closed to other keys, and
  `CommandLine` joins the mapping into the child's `--edge <source>,<target>,<relation>`, as
  `mcp.Spelling` does in a call it renders. A value holding a comma is refused at `edge`
  (`commaInMapping`) and starts nothing.

The tool polls the state store every `launchPoll` and returns when the run has started or the child
has exited, bounded by `launchStarting` (§12). The child takes the run lock and the
holder lock and emits its first progress event before it waits out a write ([§8](#locks)). What
"started" means differs by tool:

| Tool | Started when | `done` document |
|---|---|---|
| `build` | the progress record shows a stage entered (`buildstore.Launched`) | `kb-root`, `state-dir`, `pid`, `resume` and `wait`. `resume` appears only where the record's command parses as a `kbase build` invocation |
| `score` | the run's ledger and pid exist (`score.RunByPID`: the run's directory written, its `pid` file reading back) | `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait` |

`wait` is the run's background command: `buildstore.ShellJoin` of the executable with `wait
[--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one.
Otherwise the result is the child's own document where it exited first (`isError` from its outcome),
or `done` with `state: starting`, `state-dir`, `pid` and `wait` where neither happened within the
bound; `status` follows the run.

A score run's result document is written beside its ledger before the run lets go of the state
store's holder lock, so `wait` reads the same document the run reported rather than racing the
lock's release. `wait` locates the run by the pid the launch named (`score.RunByPID` reads the run's
`pid` file) and reports the run's outcome; a run whose process ended without writing a result
reports the failure its ledger supports. `status` and `cancel` read the holder record (the pid, the
run, the start stamp and the kind `score`), so a run in flight is visible under the pid the tool
returned.

Nothing announces a run's end: no resource is subscribable and the server writes no frame a request
does not owe, so a session follows a launched run by calling `status`, or by running the `wait`
command the launch returned. `test-integration-mcp-build` (`TestMCPBuildFollowedByStatus`) drives a
real server to a build's end by polling `status`.

---

## 13. Slice verdicts

**Pattern:** Spike/prototype decision record.

The reader's design was settled by a slice: `kbase build --through document-graph` over six corpus
papers and a count report of all fifty, judged by comparing kbase's tree against the reference tree
for the same paper, with kbase's own readers. The hypotheses and their verdicts:

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | The conformance and inventory checks pass on kbase's tree; per-leaf word streams match; every citation state the reference trees exhibit appears in kbase's | A load-bearing form or word no JSON transform can produce | **Confirmed** on the six Slice-1 and Slice-2 papers; 49 of 50 corpus papers build |
| Records suffice for the inventory | Every reader and placement fact of the inventory (§5.5) is carried by kbase's records, item by item in scan order; every record joins exactly one document | A reader or placement fact that exists only in rendered bytes | **Confirmed**, with the visibility rule of §5.5: a block record exists only where the page has a label line the reader reads |
| The preceding text run suffices | Withdrawn: the word before a reference is defined over the written page, so it is a page-defined reading (§5.5), not a record | — | Withdrawn |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | **Confirmed**; the writer route is pandoc's gfm writer with `--wrap=none` (§5.4) |

### Slice-1 checks

Identical tree path set and child order; equal per-leaf word streams under the markdown tokeniser;
the Document-Tree conformance passes with matching counts of up-link lines, three-attribute
anchors, math fences, `data-cites` citation spans and labelled blockquotes with identifiers; equal
inventory; records that carry every reader and placement fact of that inventory; byte-identical
`kb-root/` and records across two runs. All pass: the writer route is pandoc's. Any fails: execution
stops and the owner rules (fix, amend the check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative controls to
refuse as SPEC §5 Inference preflight specifies (naming the paper and pandoc's error; naming each unloaded file
and the line that named it). **Slice-3** records a count report, with no pass threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change to any of I1
to I7 is escalated to the owner.

---

## 14. Limits

**Pattern:** Known-limitations section.

Where the format and this implementation stop. A limit whose mechanism already states it is not
repeated here: the zero-value default is defined in [§11](#the-zero-value-hazard), and the in-memory
index's lack of statement text in [§6](#the-query-layer).

**A value records whose call wrote it and what it was judged against, and nothing more.**
`manually-scored` says whether a person's call or a machine's wrote a rigor or a bearing, and
`judged-hash` what it was last judged against (SPEC §10 Provenance and the judged hash): no author,
time or reason. A work's rigor and a `founds` bearing, which no run writes, have neither. A
`*pending*` the caller wrote and one written because it left the value out differ only in that flag.
A value converted from `2.0.x` is marked model-scored whoever judged it, and nothing records that it
was converted.

**A `founds` bearing on a work names its finding in prose.** The `context` on the record says which
theorem, table or result the bearing judges, and no id, range or hash joins the two, so no check can
prove the judgement was about that finding. An axiom's `founds` edge needs no such prose: its source
is the axiom's own id.

### A conversion records no history

The value changes a conversion makes are recomputed from the converted records, and nothing but its
warnings says which of them rose or fell.

### Merged metadata is refused, not merged

Two branches' record files merge by text: a duplicate edge, a dangling edge or a cycle the merge
leaves is a load error, `check: metadata`, which every command that loads the KB refuses. No op
resolves it: the recovery is in git, returning the record files to a consistent commit, such as the
one before the merge, and reapplying the other side's changes through write ops
(SPEC §10 The load: every fault refused).

### No format sits between two majors

`formats.Chain` walks converters version by version and refuses a step no converter leads, so a KB
stamped `1.1.0` is refused at load rather than read; no `1.1.0` converter exists and none is
reachable.

### No refresh moves a judgement

A rebuild of the derived views, `update-format`'s included, makes no model call, so a derived value
changes only when the records do: a write op, or a scoring run writing its results. `verify`'s
freshness check is between the records and a summary page, never between two judgements.


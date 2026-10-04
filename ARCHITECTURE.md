# ARCHITECTURE – kbase

How this implementation meets [SPEC.md](SPEC.md). Purpose and intent are in
[THESIS.md](THESIS.md); house rules are in [CONVENTIONS.md](CONVENTIONS.md). kb_tools
citations name adjagent `main` at `e29bc3d97453c039995962994983f3330f428d7a`, by section
name; "kb_tools" is the reference implementation, read from `.claude/adjagent/kb_tools/`
(the clone this repository's agent set is installed from), and a module
path in `[brackets]` below is the kb_tools module a kbase package ports.

Every mechanism stated here is decided and landed; §13 records the verdicts of the slice
that settled the reader's design.

---

## 1. Purpose

kbase makes a knowledge base auditable for an academic lab running local inference. The
claim graph is in scope: the KB carries nodes and edges drafted mechanically and refined
by inference, and every refinement is checkable against the artifact it refined. kbase
builds exactly what kb_tools builds, as Go, keeping its own provider management and
inference invocation.

## 2. Invariants

Violating any is a blocking defect.

| # | Invariant |
|---|---|
| I1 | Exactly one package execs pandoc (checks version and `pandoc-api-version`, with named errors); exactly one execs git. |
| I2 | The claim-graph inventory's reader and placement facts come from records, never recovered from leaf markup; its page-defined readings (§5.5) come from the leaf through the same readers kb_tools uses; otherwise only stage-A conformance (the link relation, through `kb`), the prose and the marker machinery read leaf lines, and maintenance ops read only the metadata layer. |
| I3 | Pre-passes are kb_tools' two, in one registry, each naming the reader-level hole it fills and emitting a census. |
| I4 | System compatibility per SPEC §3; byte equality only where a named reader needs it; presentation is untested. |
| I5 | One result emitter owns stdout. |
| I6 | Every mutating op is idempotent under re-issue. |
| I7 | Imported prompt templates and fragments are byte-identical to upstream at a recorded commit, and each call's system prompt is the fragment kb_tools names for it. |

Deliberately unspecified: the internal AST representation, record field encoding, and
layout algorithms.

## 3. Principles

These steer choices the invariants leave open.

- **Propose-and-verify at every model seam.** The model never mutates canonical bytes. It
  emits data with a mechanically checkable post-condition; deterministic code validates
  and executes.
- **The model supplies values; the machine owns syntax.** An ask presents its subject
  fully resolved, states which values are open, and takes the answer in a grammar the
  machine wrote. No ask asks a model to construct a document.
- **Monotone safety.** Every model-refined artifact has a mechanical fallback that was
  already acceptable. Model failure degrades quality, never correctness. No stage exits
  on a model's opinion; every stage exits on a comparison of artifacts or a return code.
- **Provenance and reproducibility are artifact properties.** The same inputs and pandoc
  version give a byte-identical document graph — tree and records — and a claim graph
  identical modulo the ids it mints (SPEC §5); derived state is regenerable and
  freshness-gated.
- **No LaTeX parser is written here, partial or whole.** The reader is pandoc.
- **kb_tools is the differential oracle.** Every difference from it is a named divergence
  (SPEC §4) or a defect.
- **The kbase and personant integration is the target.** A prior decision that prevents
  it is removed, not designed around.

---

## 4. Stage table

The build walks these stages in order. A stage boundary is a commit (§8).

| Stage | Does | Ports |
|---|---|---|
| `start` | pandoc and git preflight; populated-`kb-root/` refusal; run lock; charter | kb_tools ARCHITECTURE, "The Driver" (`start`) |
| `document-graph` | The tree from the volume roots | `kb_docgraph/` |
| `spine-seed` | Seeds `.index/` over that tree | `graph-init` |
| `claims-declared` | Claim-graph pass over author-declared claims | `kb_claimgraph/` |
| `claims-discovered` | Claim-graph pass over discovered claims | `kb_claimgraph/` |
| `equations-minted` | Equation nodes minted | `kb_claimgraph/` |
| `depends-attributed` | Dependency edges attributed | `kb_claimgraph/` |
| `phase-3a` | `refresh`, `verify`, and the readiness stamp | `kb_driver` `phase-3a` |
| `overview-drafted` | The overview document | `kb_driver` overview |

`document-graph`, `spine-seed` and the four claim-graph stages are kb_tools' **head**.
Each claim-graph stage is one `kb_claimgraph` invocation; kb_claimgraph's own stage
letters (A conformance through G gate) are always written "kb_claimgraph stage B" and so
on, never bare.

- `--through <stage>` bounds the walk; `--no-inference` drops the stages' inference rows
  and walks the rest (SPEC §5). Which rows drop follows kb_tools' classification, derived from
  the rows that spend inference and never from a stage id (kb_tools ARCHITECTURE, "The
  Driver", `--no-inference` and `spends_inference`).
- `phase-3a` stamps `AGENTS.md`, `CLAUDE.md` and `CONVENTIONS.md`, each only if absent;
  `kb-root/CLAUDE.md` is exactly `@AGENTS.md`.
- There is no opt-in document-audit stage (ROADMAP).
- **Not ported from kb_tools:** `ov.docent-check` (checks a Claude Code command install);
  `spine-seed`'s runner include line and its barrier (the compatibility harness installs
  kb_tools' targets through kb_tools' own installer); relay cards and the exit ladder
  (replaced by SPEC §7); the environment-variable endpoint configuration
  (`inference/liaison_tools.check_environment`; kbase's providers are SPEC §9); the
  preflight items `docent-commands` and `scratch-ignored` and the whole-worktree dirty
  refusal (`kb_util.preflight_report`; kbase refuses on dirty kbase-owned paths only, SPEC
  §5); `--config` and `--decide` (kbase's configuration is SPEC §9, and a barrier is a
  refusal with its answer on the next invocation's flags); `install_location` and the
  installer; `kb_survey/manifest.py` and `skeleton.py` (uncalled). kb_tools' retired
  `phase-5` id is never given to a stage.
- Stage boundaries are where expensive work becomes recoverable: an inference stage's
  per-leaf work is uncommitted until its boundary.

---

## 5. Reader and transforms

### 5.1 pandoc seam

`internal/latex/pandoc` is the only package that execs pandoc. It checks the version
range and `pandoc-api-version` and raises named errors, including a missing pandoc that
names the install page. It runs LaTeX to JSON with no filters, and with `--citeproc`
exactly when a bibliography is offered: without one, citeproc would render every
citation unanswered, where kb_tools renders it as its own keys. It is the reader and the
writer seam; the writer runs standalone (`-s`), so the title and abstract arrive as the
rendering's YAML metadata block. [`pandoc.py`]

### 5.2 Pre-pass registry

`internal/latex/prepass` holds kb_tools' two source scans, `strip_environment_declarations`
and `theorem_display_names` [`kb_docgraph/convert.py`]. Nothing else massages source.
They live in one registry. Each entry names the reader-level hole it fills and emits a
census of what it found. A new entry is an ARCHITECTURE change. (I3)

### 5.3 JSON transforms

`internal/docgraph` transforms the pandoc JSON in Go in place of kb_tools' Lua filter
[`kb_docgraph/authored_blocks.lua`]:

- Cite span wrapping
- author-declared Div to labelled blockquote
- title-page drop
- display maths lifted out of emphasis

Whole-volume render and cut, and outline derivation, also live here [`kb_docgraph/*`].

### 5.4 Writer route

The route is pandoc's gfm writer over the transformed JSON with `--wrap=none`, decided at
the first slice gate (§13): every load-bearing form the slice checked is produced under it.

### 5.5 Records

Alongside the Markdown leaves, Go emits records (JSONL) into the build state store
(§8), never into `kb-root/`. kb_tools' claim graph takes the tree as its sole input, so its
stage B recovers every fact by scanning the pages; the records are kbase's channel for the
same facts, so nothing is recovered from leaf markup (I2). They carry:

- **reader facts:** display name, identifier, owning document, order; references —
  macro type as pandoc spells it, labels; the enclosing
  author-declared block of each reference and citation (pandoc's Div nesting); `\label`
  names inside display maths; citations — keys and state (resolved, unanswered,
  key-only); the bibliography's works; references in a proof's opening run;
- **placement facts**, set when the tree is cut: each reference's resolved document and
  fragment, and each copy of a heading's reference the navigation lines carry (the up-link
  line and the index child list).

A block record exists exactly where the written leaf carries a label line kb_tools' reader
reads [`inventory.LABEL_LINE_RE`]: an author block the transform emits as a top-level
labelled blockquote whose name that reader's environment group accepts (letters and
single spaces) — and, because the reader keys on the page, a bold-only paragraph opening
an author's blockquote, which it reads as a label line. A block nested inside another, one
rendered inside raw HTML (a figure), or one whose name the reader rejects (`table*`)
yields no record, and the facts inside it host to the enclosing readable block. One walk
derives the outline and the records, so the rule is applied once. Labels are recorded in
the page's attribute spelling: pandoc escapes `&<>"'` in attributes, and differently for a
`Link` than for a `Span` or `Div`, so a record compared against the page carries the
escaped form.

**Page-defined readings** are kb_tools' by definition over the written leaf and are read
from it, through the same readers: the locator span and title off a block's display line
[`inventory._blocks`, `page_text`]; line extents; the word the page shows before a
reference [`Anchor.preceding_word`]; a work's rendered text; a claim's statement text — its
block extent, prose paragraph or equation fence — and the paragraph around a reference, as
ask input [`classify.statements`, `hand_named.bodies`, `attribute._reference_line`];
printed name-and-number mentions in node bodies [`hand_named.printed_claims`]; the printed
number and optional title off the display line, which no record carries. Claim-bearing
classification and
proof-to-subject binding remain claim-graph rules. Field encoding is deliberately
unspecified.

### 5.6 Tree derivation, partition checks and `validate_build`

`internal/docgraph` derives the tree and runs the build's own checks: the two partition
checks, the maths-survival count, the anchor-landing check and the image-asset report
[`kb_docgraph/outline.py`, `judge.py`, `partition.py`, `build.py`], then the parts of
`validate_build` that read the written tree rather than restate the cut
[`kb_survey/validate.py`] and the dead-link gate over `kb-root/` (§6, shared with
verify). Each check compares one mechanical product against another: pandoc's parsed
document against pandoc's rendering (A), that rendering against the tree kbase cut (B),
pandoc's maths elements against the writer's three forms, the label read back off the
link pandoc wrote, the author's image files on disk, the tree as it lies on disk. Findings
land in the stage's report in the state store (§8, `reports/`); any FAIL makes the
outcome `failed`, with no fix loop. Not ported: `validate_build`'s tree-diff (it compares kbase's path list with
kbase's own write), kb_tools' `bibliography-read` and `declaration-read` report lines,
and the claim-id link check, which needs `.index/` and is verify's (§6).

---

## 6. KB model and maintenance

| Mechanism | Package | Ports | Status |
|---|---|---|---|
| **Metadata layer**: kb-root model: names, exclusions, link primitives and the dead-link gate over `kb-root/`; frontmatter, register and leaf parse, discovery, schema; the derived-field line renderers and the frontmatter splice refresh writes through | `internal/kb` | `kb_index_lib` parse, `kb_schema`, `kb_links`, `verify_md_links.py`, the render half of `kb_write` refresh needs | Landed |
| **Write API**: render, values, store, ops | `internal/write` | `kb_write/` | Landed: rendered metadata byte-identical to kb_tools' render goldens; its regression fixtures replay; the op script leaves a KB both toolchains' verify accept. An insert adopts by (register, title) whatever its other values, reporting the differing fields (SPEC §4, §8). The write lock is `internal/filelock`'s advisory lock on Unix; Windows has only the read-then-compare check against a concurrent writer |
| **Refresh and verify; the index**: solidity, aggregates, footers, `.index` emit and freshness, the citation gate and the claim-id link check | `internal/index` | `kb_index_lib` compute, `refresh_kb_metadata.py`, `verify_kb_metadata.py`, `verify_citations.py` | Landed: `.index/*.jsonl` and every derived field byte-identical to kb_tools' refresh; verify agrees with kb_tools on `mini-kb` and its mutations |
| **Queries** | `internal/query` | `kb_cmd/` | Landed: every query equals `kb_cmd --json` as data over the six kb_tools-built fixtures (1,550 cases) |
| **Sheet**: rendering module, index in and SVG out | `internal/sheet` | The drawing in kb_tools ARCHITECTURE, "The Claim-Graph Sheet" (not ported) | Landed as the placeholder (SPEC §3) |
| **Readiness stamping and README assembly** | `internal/kbdocs` | `installed/*.tmpl.md`, `kb_readme.py` | Landed: the stamp (only if absent), the overview excerpts, the passage structure check, the README assembly; the templates under `internal/kbdocs/templates/` |

**Write API.** `render` composes every metadata byte, so no op accepts a preformatted
heading, marker or bullet (kb_tools SPEC, "The Write API's Contract"). Input, idempotence
(I6) and the trailing refresh are SPEC §8; byte equality is SPEC §3.

**Sheet.** `internal/sheet` takes the index and returns an SVG. Today the body is the
placeholder (SPEC §3). `refresh`, `verify` and `render-claim-graph` reach it through one
seam, `index.WriteSheet`, which holds SPEC §3's rule for when a sheet is written; landing
the real graphing algorithm replaces the module's body and nothing else.

---

## 7. Inference seams

- Kept from kbase: provider management (`internal/config`), model detection
  (`internal/detect`) and the client (`internal/model`). The call path is `model.Chat`. kbase's provider tiers
  apply: letter asks go to the light tier, the overview call to the heavy tier, where
  kb_tools names one model (SPEC §4, inference endpoint).
- Every model call is one tool-less chat request answered from its prompt alone
  [`inference/liaison_tools.call_chat`]: temperature 0, thinking disabled
  (`chat_template_kwargs.enable_thinking: false`), the usage chunk requested; a call that
  does not complete is re-issued identically up to kb_tools' count
  [`kb_claimgraph/ask.TRANSPORT_ATTEMPTS`]. `model` makes it, with a system message. A
  build with a calling row left to walk checks a provider is configured before its first
  stage (SPEC §5) [`kb_driver/run.py`, `_require_server`].
- Each inference pass is built as plumbing on `model.Chat` where kb_tools places it,
  reaching the model only through `internal/asks`' letter seam, injected — nothing in
  `claimgraph` knows a network exists. `internal/asks` holds the embedded templates and
  fragments with their manifest (`provenance.yaml`), the composer, the caller (system
  fragment, per-call captures and the answer cache under the state store's `scratch/`),
  and the letter-ask seam: every claim-graph inference is one decision answered by one letter from
  a closed set the build offers, an unreadable reply asked once more and then defaulted
  to the item's mechanical draft, recorded as defaulted [`prompt_templates.py`,
  `kb_claimgraph/letters.py`, `ask.py`, `classify.py`]. Templates (`*.tmpl.md`) and
  fragments are imported byte-identical from upstream at a recorded commit, and kbase
  changes no prompt prose (I7).
- Imported prompts are `@!slot!@` templates (kb_tools ARCHITECTURE, "The Driver",
  `prompt_templates.py`); a call's system prompt is the fragment kb_tools names for it
  (`reader-system`, `overview-system`), never an agent definition. `asks.Render` builds
  the one user turn; slot values are formatted as kb_tools formats them, pinned by
  replaying kb_tools' composed-prompt goldens byte for byte.
- `claimgraph` reaches a model only through the letter seam injected by `build`; the
  caller that holds `model` lives in `internal/asks/call`, imported by `build` alone, and
  the import-policy test checks the transitive graph so `claimgraph` never reaches
  `model`.
- No prompt originates in an in-code string.
- The overview passage is written from excerpts composed mechanically from the tree
  [`kb_readme.compose_excerpts`], checked for structure and re-asked once through the
  `overview-correction` fragment.

---

## 8. Ledger, state store and monitoring

`internal/ledger` is the only package that execs git (I1); it execs the host's git, which
SPEC §2 already requires, so the user's configuration, hooks and attributes apply, and
no git library is a dependency. `internal/build` owns the stage table, the state store,
resume, `--no-inference`, `--through`, and status, progress and cancel [`kb_driver`
head, `phase-3a`, overview]. Only `build` imports `ledger`.

- **Fresh versus resume** is read from the commit trail alone (SPEC §6); nothing in the
  state store is authoritative for position.
- **Resume over an interrupted stage.** A cancelled or killed build leaves the stage's
  per-leaf work uncommitted in kbase-owned paths. A resume restores those paths to the
  last stage commit and re-runs the stage whole; the answer cache (`scratch/answers/`)
  is what carries the paid-for inference across, so the re-run asks nothing it already
  holds; it mints fresh ids, and nothing outside the restored paths referenced the old
  ones. A classify prompt names claim ids, but those were minted in stages already
  committed and the restore reaches back only to the last boundary, so a resumed run's
  prompts are identical and the cache answers them; a fresh build's are not, and it
  pays again. The node-pass
  and classification records are still written per leaf and per group as kb_tools
  writes them — the tracked record's content is kb_tools' — but kbase's resume does not
  read them for position. The progress record accounts for the dirt an interrupted stage
  leaves; the dirty-path refusal (SPEC §5) applies to dirt it cannot account for. The
  resume command carries every flag the launch did, `--charter` included.
- **Records missing on resume.** The docgraph records live in the state store. A resume
  that finds them absent regenerates them by re-running the document-graph stage into
  scratch and requires its tree byte-identical to the committed `kb-root/` (SPEC §5,
  determinism); a difference is `failed`, naming the first differing path.
- **One lock.** The run lock is `internal/filelock`'s advisory lock in the state store
  carrying the holder's pid and start time, the same mechanism `write` uses for its
  lock; `status` reads liveness from it and `cancel` signals its holder. On Windows,
  where no advisory lock exists, `status` cannot see a running build.
- **State store layout:** the lock, `progress.jsonl`, `records/` (the docgraph records
  and pre-pass census), `reports/<stage-id>.yaml` (each stage's row records and
  findings, named by the result's `stages[].report`), `scratch/` — `captures/` (every
  call's request, reply and usage), `answers/` (the answer cache, content-addressed by
  model, system prompt and prompt, written through `atomicfile`; an ask's attempt
  ordinal is part of the key so a re-ask reaches the model), `asks/` (composed prompts),
  `regenerate/` (the records regeneration on resume).
- **Progress units** are leaves for the node pass and ask groups for classification, so
  `status` moves during an inference stage; a defaulted item is a `fallback` event.
- **A stale `.git/index.lock`** left by a kill during a boundary commit is a refusal
  naming the lock.
- **`--state-dir`** defaults to the §12 key under `$XDG_STATE_HOME/kbase/`.
- **Under `--no-inference`** the overview call is the dropped row and no `README.md` is
  written, as in kb_tools' build.

- **Ledger.** Commit subject, tracked records and pathspec scoping are SPEC §6. The
  ledger is ported from kb_tools' [`kb_pipeline`, ledger half]; resume position is read
  from the trail alone. The two tracked build records — the node-pass record and the
  classification record — are `claimgraph`'s to read and write, as kb_tools' are
  [`kb_pipeline`, record half: `NODE_PASS_RELPATH`, `CLASSIFICATION_RELPATH`], in YAML
  (SPEC §4); `build` commits them.
- **State store.** Contents and path are SPEC §6; the key form is §12; the layout is
  below.
- **Status, progress, cancel.** `status` reads the lock, `progress.jsonl` and the commit
  trail and returns the YAML snapshot in SPEC §6. `cancel` signals the lock holder; the
  holder abandons the in-flight call with nothing written for it, logs the event,
  releases the lock and exits `cancelled`.

## 9. Result emitter

`internal/result` owns the YAML result document and its emitter, and nothing else
writes stdout (I5). Fixed key order, strings quoted on emit, a refusal enumerating every
offending item. `cmd/` maps `outcome` to exit code per SPEC §7. YAML uses the package named
in CONVENTIONS "Dependency Policy".

## 10. Compatibility harness

Compatibility is judged by kb_tools' own readers and checks, never by a byte diff of
presentation. The harness has two directions: a kbase-built KB judged by kb_tools'
checks, and a kb_tools-built KB judged by kbase's, with one shared op script applied
through each toolchain. Its recipes — `test-integration-{docgraph,verify,write,query,
claimgraph,build,build-live,monitor}-arxiv`, over kb_tools-built fixtures the
`prep-test-integration-kbtools-{ref,full}` recipes stage from the pinned clone — are
excluded from the hermetic test omnibus: they need pandoc and the adjagent reference, and
the live one a provider. Each writes its evidence and `comparison.yaml` under
`test_data/transient/<recipe>/`. The acceptance bar and the sheet exception are SPEC §3.

---

## 11. Package map

The listing groups packages by role; the dependency rules below it are the whole rule, and
the import graph they allow is acyclic.

```
cmd/                      cobra root; one file per subcommand; outcome-to-exit table
internal/result           YAML result document + emitter (I5)
internal/latex/pandoc     reader + writer seam (I1)       [pandoc.py]
internal/latex/prepass    kb_tools' two scans (I3)        [kb_docgraph/convert.py]
internal/docgraph         JSON transforms, whole-volume render+cut, outline,
                          records, partition checks, validate_build
                          [kb_docgraph/*, kb_survey/validate.py]
internal/records          the record schema (§5.5), written by docgraph, read by claimgraph
internal/kb               kb-root model: names, exclusions, link primitives, dead-link
                          gate, frontmatter and register parse, schema
                          [kb_index_lib parse, kb_schema, kb_links, verify_md_links]
internal/index            solidity, aggregates, footers, .index emit + freshness,
                          citation gate, claim-id link check
                          [kb_index_lib compute, refresh_/verify_kb_metadata, verify_citations]
internal/write            render / values / store / ops   [kb_write]
internal/atomicfile       the one atomic file writer
internal/query                                            [kb_cmd]
internal/sheet            rendering module: index in, SVG out; today the NYI placeholder
internal/kbdocs           readiness stamp, README assembly [installed/*.tmpl, kb_readme]
internal/claimgraph       conform, inventory (records + page-defined readings),
                          identify, attribute, classify, equation(s), endcap,
                          hand_named, assemble, write, gate   [kb_claimgraph]
internal/asks             embedded templates + fragments + manifest, composer, caller
                          (captures, answer cache), letter-ask seam
                                                          [prompt_templates, letters, ask, classify]
internal/ledger           git checkpoint seam (I1), by exec  [kb_pipeline ledger half, kb_driver/ledger]
internal/filelock         the one advisory file lock (write's lock, the run lock)
internal/build            stage table, state store, resume, --no-inference,
                          --through, status/progress/cancel [kb_driver head, phase-3a, overview]
internal/asks/call        the caller: model.Chat behind the letter seam, captures, answer
                          cache; imported by build alone
kept: config, detect, model (model.Chat is the call path), log, version
```

**Dependency rules.**

- `docgraph` knows nothing of claims; the record schema it writes is the one
  `claimgraph` reads, defined once in `internal/records`, which imports nothing of kbase.
- `claimgraph` never imports `latex/*`.
- `kb` imports no other kbase package but `log`; `docgraph`, `index`, `write`, `query`,
  `sheet`, `kbdocs` and `claimgraph` read the kb-root vocabulary, the link relation (in
  kb_tools' two readings, one resolver), the label-name grammar and the id minting from it.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph`
  or `build`.
- Only `asks/call`, `build` and `cmd`'s `models` and `configure` verbs import `model`;
  `claimgraph` reaches neither `model` nor `asks/call`, transitively.
- Only `build` imports `ledger`.
- Only `internal/latex/pandoc` execs pandoc; only `internal/ledger` execs git.

The dependency rules are enforced by an import-policy test over the transitive import
graph, beside the I1 exec-policy test.

---

## 12. Constants

| Constant | Value |
|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6) |
| Query default `--limit` | 50 results; chosen so a default-limit result from each list query fits within personant's 8 KB tool-result cap over the six kb_tools-built fixtures, asserted by a test |
| Transport attempts | 3 per call, as kb_tools' `TRANSPORT_ATTEMPTS`; letter asks re-issue at once, the overview call pauses 5 s then 30 s between attempts, as kb_tools' driver does |
| Reader concurrency | `[asks] readerConcurrency` in `config.toml` (SPEC §9), default 4 |

Exit codes and the sheet digest form are in SPEC §7 and §3; the pandoc range is SPEC §2;
the stage-commit subject is SPEC §6.

---

## 13. Slice verdicts

The reader's design was settled by a slice — `kbase build --through document-graph` over
six corpus papers and a census of all fifty — judged by comparing kbase's tree against
kb_tools' for the same paper with kb_tools' own readers. The hypotheses and their
verdicts:

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | kb_tools' conformance and inventory pass on kbase's tree; per-leaf word streams match; every citation state kb_tools' trees exhibit appears in kbase's | A load-bearing form or word no JSON transform can produce | **Confirmed** on the six Slice-1 and Slice-2 papers; 49 of 50 corpus papers build (`test_data/transient/test-integration-slice-arxiv/stage-{1,2,3}/`) |
| Records suffice for the inventory | Every reader and placement fact of kb_tools' stage B inventory (§5.5) is carried by kbase's records, item by item in scan order; every record joins exactly one document | A reader or placement fact that exists only in rendered bytes | **Confirmed**, with §5.5's visibility rule: a block record exists only where the page carries a label line kb_tools reads |
| The preceding text run suffices | Withdrawn: kb_tools defines the word before a reference over the written page, so it is a page-defined reading (§5.5), not a record | — | Withdrawn |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | **Confirmed**; the writer route is pandoc's gfm writer with `--wrap=none` (§5.4) |

**Slice-1 checks:** identical tree path set and child order; equal per-leaf word streams
under `kb_docgraph/text.markdown_tokens`; kb_tools' Document-Tree conformance passes
with matching counts of up-link lines, three-attribute anchors, math fences, `data-cites`
citation spans and labelled blockquotes with identifiers; equal inventory; records carry
every reader and placement fact of that inventory; byte-identical `kb-root/` and records
across two runs. All pass: the
writer route is pandoc's. Any fails: execution stops and the owner rules (fix, amend the
check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative
controls to refuse as SPEC §5 specifies (naming the paper and pandoc's error; naming each
unloaded file and the line that named it). **Slice-3** records a census, with no pass
threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change
to any of I1 to I7 is escalated to the owner.

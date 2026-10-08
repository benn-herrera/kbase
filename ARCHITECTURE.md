# ARCHITECTURE – kbase

How this implementation meets [SPEC.md](SPEC.md). Purpose and intent are in [THESIS.md](THESIS.md);
house rules are in [CONVENTIONS.md](CONVENTIONS.md). kb_tools citations name adjagent `main` at
`5a290afab53a9742b89db8e623880e5faa5d5bda`, by section name; "kb_tools" is the reference
implementation, read from `.claude/adjagent/kb_tools/` (the clone this repository's agent set is
installed from), and a module path in `[brackets]` below is the kb_tools module a kbase package
ports.

Every mechanism stated here is decided and landed; §13 records the verdicts of the slice that
settled the reader's design.

---

## 1. Purpose

kbase makes a knowledge base auditable for an academic lab running local inference. The claim graph
is in scope: the KB carries nodes and edges drafted mechanically and refined by inference, and every
refinement is checkable against the artifact it refined. kbase builds exactly what kb_tools builds,
as Go, keeping its own provider management and inference invocation.

## 2. Invariants

Violating any is a blocking defect.

| # | Invariant |
|---|---|
| I1 | Exactly one package execs pandoc (checks version and `pandoc-api-version`, with named errors); exactly one execs git; exactly one execs Graphviz `dot`; exactly one execs kbase itself (the MCP server's detached build, `cmd/mcp.go`). The exec-policy test names each owner. |
| I2 | The claim-graph inventory's reader and placement facts come from records, never recovered from leaf markup; its page-defined readings (§5.5) come from the leaf through the same readers kb_tools uses; otherwise only stage-A conformance (the link relation, through `kb`), the prose and the marker machinery, and the sheet (a claim's kind, at its marker) read leaf lines, and maintenance ops read only the metadata layer. |
| I3 | Pre-passes are kb_tools' two, in one registry, each naming the reader-level hole it fills and emitting a census. |
| I4 | System compatibility per SPEC §3; byte equality only where a named reader needs it; presentation is untested. |
| I5 | One result emitter owns stdout. |
| I6 | Every mutating op is idempotent under re-issue. |
| I7 | Every prompt template and fragment is an embedded file with one manifest entry stating its provenance: imported — byte-identical to kb_tools' file at a recorded commit — or kbase-authored, derived from a named kb_tools file at a recorded commit and reviewed by the prompt-engineer before first use. No prompt prose originates in code. Each call's system prompt is the fragment named for it. |
| I8 | One metadata format version is read and written (SPEC §10). Every KB file is read through `kb.Source`, built by `internal/kbload` alone; `kbload` is the one importer of `internal/migrate`, which converts an older KB at load — a map of repository-relative paths to bytes in, the same out, plus the obsolete paths; no file I/O, no import of a kbase package — and the result is written only by the next natural write, which drains the obsolete paths after a successful full save, minus what that save wrote. |
| I9 | The MCP server binds subcommands and adds no operation: a command declares its binding with the cobra annotation `kbase.mcp` = `bound` or `excluded`, and the tool table is built from the `bound` commands of the command tree, never listed by hand; the table's test derives its expectation from the same annotations; its stdout carries JSON-RPC frames and nothing else; no tool but `build` reaches a model, and `build` through it is the detached subcommand. |

Deliberately unspecified: the internal AST representation, record field encoding, and layout
algorithms.

## 3. Principles

These steer choices the invariants leave open.

- **Propose-and-verify at every model seam.** The model never mutates canonical bytes. It emits data
  with a mechanically checkable post-condition; deterministic code validates and executes.
- **The model supplies values; the machine owns syntax.** An ask presents its subject fully
  resolved, states which values are open, and takes the answer in a grammar the machine wrote. No
  ask asks a model to construct a document.
- **Monotone safety.** Every model-refined artifact has a mechanical fallback that was already
  acceptable. Model failure degrades quality, never correctness. No stage exits on a model's
  opinion; every stage exits on a comparison of artifacts or a return code.
- **Provenance and reproducibility are artifact properties.** The same inputs and pandoc version
  give a byte-identical document graph — tree and records — and a claim graph identical modulo the
  ids it mints (SPEC §5); derived state is regenerable and freshness-gated.
- **No LaTeX parser is written here, partial or whole.** The reader is pandoc.
- **kb_tools is the differential oracle.** Every difference from it is a named divergence (SPEC §4)
  or a defect.
- **The kbase and personant integration is the target.** A prior decision that prevents it is
  removed, not designed around.

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
| `references-found` | Unmarked references found: a mechanical shortlist per source claim within the two unmarked-shortlist budgets (§12), one letter ask per pair, a yes an edge candidate; writes nothing under `kb-root/` | `kb_claimgraph/` (`unmarked.py`, `shortlist.py`, `equation_sites.py`) |
| `depends-attributed` | Dependency edges attributed: one letter ask per candidate, except a candidate born of an unmarked yes, which is not asked and lands as a `depends` edge (SPEC §4) | `kb_claimgraph/` |
| `phase-3a` | `refresh`, `verify`, and the readiness stamp | `kb_driver` `phase-3a` |
| `overview-drafted` | The overview document | `kb_driver` overview |

`document-graph`, `spine-seed` and the five claim-graph stages are kb_tools' **head**. Each
claim-graph stage is one `kb_claimgraph` invocation; kb_claimgraph's own stage letters (A
conformance through G gate) are always written "kb_claimgraph stage B" and so on, never bare.

- `--through <stage>` bounds the walk; `--no-inference` drops the stages' inference rows and walks
  the rest (SPEC §5). Which rows drop follows kb_tools' classification, derived from the rows that
  spend inference and never from a stage id (kb_tools ARCHITECTURE, "The Driver", `--no-inference`
  and `spends_inference`).
- `phase-3a` stamps `AGENTS.md`, `CLAUDE.md` and `CONVENTIONS.md`, each only if absent;
  `kb-root/CLAUDE.md` is exactly `@AGENTS.md`.
- There is no opt-in document-audit stage (ROADMAP).
- **Not ported from kb_tools:** `ov.docent-check` (checks a Claude Code command install);
  `spine-seed`'s runner include line and its barrier (the compatibility harness installs kb_tools'
  targets through kb_tools' own installer); relay cards and the exit ladder (replaced by SPEC §7);
  the environment-variable endpoint configuration (`inference/liaison_tools.check_environment`;
  kbase's providers are SPEC §9); the preflight items `docent-commands` and `scratch-ignored` and
  the whole-worktree dirty refusal (`kb_util.preflight_report`; kbase refuses on dirty kbase-owned
  paths only, SPEC §5); `--config` and `--decide` (kbase's configuration is SPEC §9, and a barrier
  is a refusal with its answer on the next invocation's flags); `install_location` and the
  installer; `kb_survey/manifest.py` and `skeleton.py` (uncalled). kb_tools' retired `phase-5` id is
  never given to a stage.
- Stage boundaries are where expensive work becomes recoverable: an inference stage's per-leaf work
  is uncommitted until its boundary.

---

## 5. Reader and transforms

### 5.1 pandoc seam

`internal/latex/pandoc` is the only package that execs pandoc. It checks the version range and
`pandoc-api-version` and raises named errors, including a missing pandoc that names the install
page. It runs LaTeX to JSON with no filters, and with `--citeproc` exactly when a bibliography is
offered: without one, citeproc would render every citation unanswered, where kb_tools renders it as
its own keys. It is the reader and the writer seam; the writer runs standalone (`-s`), so the title
and abstract arrive as the rendering's YAML metadata block. [`pandoc.py`]

### 5.2 Pre-pass registry

`internal/latex/prepass` holds kb_tools' two source scans, `strip_environment_declarations` and
`theorem_display_names` [`kb_docgraph/convert.py`]. Nothing else massages source. They live in one
registry. Each entry names the reader-level hole it fills and emits a census of what it found. A new
entry is an ARCHITECTURE change. (I3)

### 5.3 JSON transforms

`internal/docgraph` transforms the pandoc JSON in Go in place of kb_tools' Lua filter
[`kb_docgraph/authored_blocks.lua`]:

- Cite span wrapping
- author-declared Div to labelled blockquote
- title-page drop
- display maths lifted out of emphasis

Whole-volume render and cut, and outline derivation, also live here [`kb_docgraph/*`].

### 5.4 Writer route

The route is pandoc's gfm writer over the transformed JSON with `--wrap=none`, decided at the first
slice gate (§13): every load-bearing form the slice checked is produced under it.

### 5.5 Records

Alongside the Markdown leaves, Go emits records (JSONL) into the build state store (§8), never into
`kb-root/`. kb_tools' claim graph takes the tree as its sole input, so its stage B recovers every
fact by scanning the pages; the records are kbase's channel for the same facts, so nothing is
recovered from leaf markup (I2). They carry:

- **reader facts:** display name, identifier, owning document, order; references — macro type as
  pandoc spells it, labels; the enclosing author-declared block of each reference and citation
  (pandoc's Div nesting); `\label` names inside display maths; citations — keys and state (resolved,
  unanswered, key-only); the bibliography's works; references in a proof's opening run;
- **placement facts**, set when the tree is cut: each reference's resolved document and fragment,
  and each copy of a heading's reference the navigation lines carry (the up-link line and the index
  child list).

A block record exists exactly where the written leaf carries a label line kb_tools' reader reads
[`inventory.LABEL_LINE_RE`]: an author block the transform emits as a top-level labelled blockquote
whose name that reader's environment group accepts (letters and single spaces) — and, because the
reader keys on the page, a bold-only paragraph opening an author's blockquote, which it reads as a
label line. A block nested inside another, one rendered inside raw HTML (a figure), or one whose
name the reader rejects (`table*`) yields no record, and the facts inside it host to the enclosing
readable block. One walk derives the outline and the records, so the rule is applied once. Labels
are recorded in the page's attribute spelling: pandoc escapes `&<>"'` in attributes, and differently
for a `Link` than for a `Span` or `Div`, so a record compared against the page carries the escaped
form.

**Page-defined readings** are kb_tools' by definition over the written leaf and are read from it,
through the same readers: the locator span and title off a block's display line
[`inventory._blocks`, `page_text`]; line extents; the word the page shows before a reference
[`Anchor.preceding_word`]; a work's rendered text; a claim's statement text — its block extent,
prose paragraph or equation fence — and the paragraph around a reference, as ask input
[`classify.statements`, `hand_named.bodies`, `attribute._reference_line`]; printed name-and-number
mentions in node bodies [`hand_named.printed_claims`]; the printed number and optional title off the
display line, which no record carries; the source leaf's render shown as the unmarked ask's body
[`unmarked.py`, `label.render`]. Claim-bearing classification and proof-to-subject binding remain
claim-graph rules. Field encoding is deliberately unspecified.

### 5.6 Tree derivation, partition checks and `validate_build`

`internal/docgraph` derives the tree and runs the build's own checks: the two partition checks, the
maths-survival count, the anchor-landing check and the image-asset report [`kb_docgraph/outline.py`,
`judge.py`, `partition.py`, `build.py`], then the parts of `validate_build` that read the written
tree rather than restate the cut [`kb_survey/validate.py`] and the dead-link gate over `kb-root/`
(§6, shared with verify). Each check compares one mechanical product against another: pandoc's
parsed document against pandoc's rendering (A), that rendering against the tree kbase cut (B),
pandoc's maths elements against the writer's three forms, the label read back off the link pandoc
wrote, the author's image files on disk, the tree as it lies on disk. Findings land in the stage's
report in the state store (§8, `reports/`); any FAIL makes the outcome `failed`, with no fix loop.
Not ported: `validate_build`'s tree-diff (it compares kbase's path list with kbase's own write),
kb_tools' `bibliography-read` and `declaration-read` report lines, and the claim-id link check,
which needs `.index/` and is verify's (§6).

---

## 6. KB model and maintenance

| Mechanism | Package | Ports | Status |
|---|---|---|---|
| **Metadata layer**: kb-root model: names, exclusions, link primitives and the dead-link gate over `kb-root/`; frontmatter, register and leaf parse, discovery, schema; the derived-field line renderers and the frontmatter splice refresh writes through | `internal/kb` | `kb_index_lib` parse, `kb_schema`, `kb_links`, `verify_md_links.py`, the render half of `kb_write` refresh needs | Landed |
| **Write API**: render, values, store, ops | `internal/write` | `kb_write/` | Landed: rendered metadata byte-identical to kb_tools' render goldens; its regression fixtures replay; the op script leaves a KB both toolchains' verify accept. An insert adopts by (register, title) whatever its other values, reporting the differing fields (SPEC §4, §8). There is one KB write lock, `write.LockKB(kbRoot)` (`lock.go`): `internal/filelock`'s advisory lock on the directory `kb-root/` sits in. The caller takes it and holds it across the whole run (SPEC §8): `cmd` from `kbload.Open` through the write, the trailing refresh and the sheets to the result, and around `refresh` and `render-claim-graph`; the ops and refresh themselves take none, and the per-directory splice locks are gone. A writer that cannot take it within the wait (§12) gets `retry`, `check: lock`, `path` the repository root. `render-citation`, `verify` and the queries take none. The build takes and releases it once to wait out a write in progress (§8). On Windows, where no advisory lock exists, only the read-then-compare check guards against a concurrent writer |
| **Refresh and verify; the index**: solidity, aggregates, footers, `.index` emit and freshness, the citation gate and the claim-id link check | `internal/index` | `kb_index_lib` compute, `refresh_kb_metadata.py`, `verify_kb_metadata.py`, `verify_citations.py` | Landed: `.index/*.yaml` and every derived field byte-identical to kb_tools' refresh; verify agrees with kb_tools on `mini-kb` and its mutations |
| **Queries** | `internal/query` | `kb_cmd/` | Landed: every query equals `kb_cmd --json` as data over the six kb_tools-built fixtures, `show`'s `strengthen_by` and `deps`' `context` included: 1,550 of 1,550 cases agree at the pin |
| **Sheet**: DOT emitter and `dot` invocation; index, build records and repository name in, SVGs out | `internal/sheet` | `claim_sheet.py`, the same drawing; the layout is Graphviz's | Landed as the drawing of SPEC §3; the placeholder only where `dot` is absent |
| **Readiness stamping and README assembly** | `internal/kbdocs` | `installed/*.tmpl.md`, `kb_readme.py` | Landed: the stamp (only if absent), the overview excerpts, the passage structure check, the README assembly; the templates under `internal/kbdocs/templates/` |

**Write API.** `render` composes every metadata byte, so no op accepts a preformatted heading,
marker or bullet (kb_tools SPEC, "The Write API's Contract"). Input, idempotence (I6) and the
trailing refresh are SPEC §8; byte equality is SPEC §3. One op, `add-build-edges`
(`write.AddBuildEdges`), is build-only: it writes `add-depends-on`'s lists and the `demoted` list,
is absent from `write.Ops()` and has no subcommand, so the public write surface cannot create a
`demoted` edge; the build's cuts reach the register through it. `resolve-demoted` is a public op.
`index.DependsGraph.Path` is the path finder over the acyclicity check's graph; the check and
`resolve-demoted`'s cycle refusal both use it.

**Frontmatter reading.** `internal/kb` takes a leading `---` block as frontmatter only when its
first non-empty line is key-shaped (`name:`, kebab-case); a block that then fails to parse as a YAML
mapping is the malformed-frontmatter refusal, and a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10).

**Sheet.** `internal/sheet` emits Graphviz DOT and renders it by running `dot -Tsvg` on `PATH`; the
SVG's bytes are Graphviz's, so the goldens pin the DOT, and a test that renders through `dot` skips
without it and asserts only that the DOT parses and the root fits the viewer. The goldens are
adjagent's fixture and its three DOT files, copied to `internal/sheet/testdata/adjagent-sheet/` and
checked byte-identical to the pinned clone's where the clone is present; the emitted DOT matches
them byte for byte, as `claim_sheet.py`'s does. Its inputs are the index (claims,
edges, solidity bands); the build records beside `kb-root/`, read through `internal/buildrecords`
that `claimgraph` and `sheet` both import — only the unmarked record, for each edge's provenance
(marked, unmarked ask answered A); the index's `demoted` rows, for the cuts and their origins; the
leaf, for a node's kind, with the title deciding where no marker is readable; the entry point's
first heading for the title and the `kb-root/`-level cluster; and each volume's `index.md` first
heading, falling back to the directory name, for the volume's cluster title. It produces the
full sheet always, and the volume digest and one sheet per volume where two or more volumes hold
nodes. Drawn content and styling are SPEC §3. It renders through at most four `dot` processes at
once (`sheet.renderConcurrency`, §12), keeping output order. Without `dot`, it renders nothing,
leaves existing sheets as they are, and yields the placeholder for a root that has no sheet.

`refresh` — and so every caller of `index.Refresh`: a write op's trailing refresh, the build's gate
— and `render-claim-graph` reach it through one seam, `index.WriteSheet`, which renders every sheet
on each call and writes one only where its bytes change, with no ownership test on an existing
sheet. `verify` never reaches it.

---

## 7. Inference seams

- Kept from kbase: provider management (`internal/config`), model detection (`internal/detect`) and
  the client (`internal/model`). The call path is `model.Chat`. kbase's provider tiers apply: letter
  asks go to the light tier, the overview call to the heavy tier, where kb_tools names one model
  (SPEC §4, inference endpoint).
- Every model call is one tool-less chat request answered from its prompt alone
  [`inference/liaison_tools.call_chat`]: temperature 0, thinking disabled
  (`chat_template_kwargs.enable_thinking: false`), the usage chunk requested; a call that does not
  complete is re-issued identically up to kb_tools' count [`kb_claimgraph/ask.TRANSPORT_ATTEMPTS`].
  `model` makes it, with a system message. A build with a calling row left to walk checks a provider
  is configured before its first stage (SPEC §5) [`kb_driver/run.py`, `_require_server`].
- Each inference pass is built as plumbing on `model.Chat` where kb_tools places it, reaching the
  model only through `internal/asks`' letter seam, injected — nothing in `claimgraph` knows a
  network exists. `internal/asks` holds the embedded templates and fragments with their manifest
  (`provenance.yaml`), the composer, the caller (system fragment, per-call captures and the answer
  cache under the state store's `scratch/`), and the letter-ask seam: every claim-graph inference is
  one decision answered by one letter from a closed set the build offers, an unreadable reply asked
  once more and then defaulted to the item's mechanical draft, recorded as defaulted
  [`prompt_templates.py`, `kb_claimgraph/letters.py`, `ask.py`, `classify.py`]. Templates
  (`*.tmpl.md`) and fragments are embedded files under I7's manifest: those still imported are
  byte-identical to kb_tools' at the recorded commit; those kbase authors (the unmarked ask first)
  are derived from kb_tools' and reviewed before first use, and kb_tools adopts them (SPEC §2,
  reference roles). A template change is an experiment with a measured result (ACTIVE_PLAN).
- Imported prompts are `@!slot!@` templates (kb_tools ARCHITECTURE, "The Driver",
  `prompt_templates.py`); a call's system prompt is the fragment kb_tools names for it
  (`reader-system`, `overview-system`), never an agent definition. `asks.Render` builds the one user
  turn; slot values are formatted as kb_tools formats them, pinned by replaying kb_tools'
  composed-prompt goldens byte for byte for the imported templates. No test asserts a template's
  prose; tests key on the offered letters, the slot set, the system fragment and the closing line's
  letters.
- `claimgraph` reaches a model only through the letter seam injected by `build`; the caller that
  holds `model` lives in `internal/asks/call`, imported by `build` alone, and the import-policy test
  checks the transitive graph so `claimgraph` never reaches `model`.
- No prompt originates in an in-code string.
- The overview passage is written from excerpts composed mechanically from the tree
  [`kb_readme.compose_excerpts`], checked for structure and re-asked once through the
  `overview-correction` fragment.

---

## 8. Ledger, state store and monitoring

`internal/ledger` is the only package that execs git (I1); it execs the host's git, which SPEC §2
already requires, so the user's configuration, hooks and attributes apply, and no git library is a
dependency. `internal/build` owns the stage table, the state store, resume, `--no-inference`,
`--through`, and status, progress and cancel [`kb_driver` head, `phase-3a`, overview]. Only `build`
imports `ledger`.

- **Fresh versus resume** is read from the commit trail alone (SPEC §6); nothing in the state store
  is authoritative for position.
- **Inputs on resume.** `walk.body` ends every boundary's body with the run's inputs (SPEC §6), from
  `Options.VolumeRoots` and `Options.Bibliographies`, each path made repository-relative with the
  path and the root both symlink-resolved. `run` compares this run's inputs with the newest trail
  entry's as soon as it reads the trail — before the launch checks, the `unchanged` and `bounded`
  exits and any restore — and refuses a difference (SPEC §5); an entry recording no inputs is not
  compared.
- **Resume over an interrupted stage.** A cancelled or killed build leaves the stage's per-leaf work
  uncommitted in kbase-owned paths. A resume restores those paths to the last stage commit and
  re-runs the stage whole; the answer cache (`scratch/answers/`) is what carries the paid-for
  inference across, so the re-run asks nothing it already holds; it mints fresh ids, and nothing
  outside the restored paths referenced the old ones. A classify prompt names claim ids, but those
  were minted in stages already committed and the restore reaches back only to the last boundary, so
  a resumed run's prompts are identical and the cache answers them; a fresh build's are not, and it
  pays again. The node-pass and classification records are still written per leaf and per group as
  kb_tools writes them — the tracked record's content is kb_tools' — but kbase's resume does not
  read them for position. The progress record accounts for the dirt an interrupted stage leaves; the
  dirty-path refusal (SPEC §5) applies to dirt it cannot account for. The resume command carries
  every flag the launch did, `--charter` included.
- **Records missing on resume.** The docgraph records live in the state store. A resume that finds
  them absent regenerates them by re-running the document-graph stage into scratch and requires its
  tree byte-identical to the committed `kb-root/` (SPEC §5, determinism); a difference is `failed`,
  naming the first differing path.
- **Two locks, both `internal/filelock` advisory locks.** The *run lock* is `<git
  dir>/kbase-build.lock`, recording the state-dir; the build takes it after it opens and holds it
  for the whole run, and removes it on release. Because it is per repository, a second build through
  any `--state-dir` is `refused` (`check: lock`, `path` the state-dir, detail "a build is running;
  its state-dir is …", no remedy). A write op, `refresh` or `render-claim-graph` that finds it held
  is refused with the same item; a write verb probes for it before waiting on the write lock and
  again after taking it. An empty lock file is the window between acquiring the lock and writing the
  state-dir: it reads as "starting", so a second build gets `retry` and a writer proceeds. The
  build's order is: the run lock; the state store's holder lock; its first progress event; and only
  then the wait for a write in progress, by taking and releasing the KB write lock (§6; the same
  wait, `retry` if it cannot). `status` therefore sees a waiting build, and a build that gives up
  waiting ends `retry`. The full order of a build is: refuse a newer store at open; the run lock;
  the holder lock; ready the store (upgrade, stamp); the run event; prune captures; wait out
  writers. Its own writes and its seed and gate refreshes take no write lock, so it cannot refuse
  itself. The *holder lock* is in the state store, carrying the holder's pid and start time;
  `status` reads liveness from it and the progress record, and `cancel` signals its holder. A second
  builder on the same state-dir is `refused`; where only the holder lock is held the result is
  `retry`. User-facing text calls it "holds the state store"; "run lock" names only `<git
  dir>/kbase-build.lock`. On Windows, where no advisory lock exists, `status` cannot see a running
  build.
- **State store version.** `<state-dir>/format` is one integer, `storeFormat` (§12), written when a
  build first holds the store; an absent file reads as 1. `build`, `status`, `cancel` and the MCP
  build tool read it at open, and a store newer than `storeFormat` is refused (`check: state-dir`,
  `key: --state-dir`, `path` the store, remedy "update kbase, or use another --state-dir"). An older
  store is converted in place by the build, in "ready the store", through the chain `storeUpgrades`
  in `internal/build`: one step per version, each stamping `format` when it completes, so an
  interrupted upgrade resumes at the step it stopped at; a gap in the chain is an error. The chain
  is independent of the KB's `migrate.Chain` (SPEC §10).
- **Retention.** At its start, after the run event, a build removes `reports/build-*` and
  `scratch/captures/` entries last modified before the start of the `keptBuilds`-th (§12) latest
  build that ended, counted from `progress.jsonl`; a build that ended `refused` or `retry` is not
  counted. `scratch/answers/` and the stage reports are never pruned, and `status` and `cancel`
  remove nothing. `status` computes `cache-entries` and `cache-bytes` from `scratch/answers/`.
- **State store layout:** `format`, the holder lock, `progress.jsonl`, `records/` (the docgraph
  records and pre-pass census), `reports/<stage-id>.yaml` (each stage's row records and findings,
  named by the result's `stages[].report`; also the captures of a build the MCP server starts,
  `build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`), `scratch/` — `captures/` (every
  call's request, reply and usage), `answers/` (the answer cache, content-addressed by model, system
  prompt and prompt, written through `atomicfile`; an ask's attempt ordinal is part of the key so a
  re-ask reaches the model), `asks/` (composed prompts), `regenerate/` (the records regeneration on
  resume).
- **Progress units** are leaves for the node pass, source groups for `references-found` and ask
  groups for classification, so `status` moves during an inference stage; a defaulted item is a
  `fallback` event. The node-pass, unmarked and classification records are written per leaf or per
  group as kb_tools writes them.
- **A stale `.git/index.lock`** left by a kill during a boundary commit is a refusal naming the
  lock.
- **`--state-dir`** defaults to the §12 key under `$XDG_STATE_HOME/kbase/`.
- **Under `--no-inference`** the overview call is the dropped row and no `README.md` is written, as
  in kb_tools' build.

- **Ledger.** Commit subject, tracked records and pathspec scoping are SPEC §6. The ledger is ported
  from kb_tools' [`kb_pipeline`, ledger half]; resume position is read from the trail alone. The
  three tracked build records — node-pass, unmarked and classification — are `claimgraph`'s to read
  and write, as kb_tools' are [`kb_pipeline`, record half: `NODE_PASS_RELPATH`, `UNMARKED_RELPATH`,
  `CLASSIFICATION_RELPATH`], as YAML `kb-build-*.yaml` (SPEC §6, §10). `build` commits them.
- **State store.** Contents and path are SPEC §6; the key form is §12; the layout is below.
- **Status, progress, cancel.** `status` reads the holder lock, `progress.jsonl` and the commit
  trail and returns the YAML snapshot in SPEC §6. `cancel` signals the lock holder; the holder
  abandons the in-flight call with nothing written for it, logs the event, releases the lock and
  exits `cancelled`.

## 9. Result emitter

`internal/result` owns the YAML result document and its emitter, and nothing else writes stdout
(I5). Fixed key order, strings quoted on emit, a refusal enumerating every offending item. `cmd/`
maps `outcome` to exit code per SPEC §7. YAML uses the package named in CONVENTIONS "Dependency
Policy".

## 10. Compatibility harness

Compatibility is judged by kb_tools' own readers and checks, never by a byte diff of presentation.
The harness has two directions: a kbase-built KB judged by kb_tools' checks, and a kb_tools-built KB
judged by kbase's, with one shared op script applied through each toolchain. Its recipes —
`test-integration-{docgraph,verify,write,query, claimgraph,build,build-live,monitor}-arxiv`, over
kb_tools-built fixtures the `prep-test-integration-kbtools-{ref,full}` recipes stage from the pinned
clone — are excluded from the hermetic test omnibus: they need pandoc and the adjagent reference,
and the live one a provider. Each writes its evidence and `comparison.yaml` under
`test_data/transient/<recipe>/`. The acceptance bar is SPEC §3.

---

## 11. Package map

The listing groups packages by role; the dependency rules below it are the whole rule, and the
import graph they allow is acyclic.

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
                          gate, frontmatter and register parse, schema (the node kinds
                          and the relation names, `Relation*` and `Relations`, spelled
                          nowhere else), `Source` (the one read path), the index row
                          types (one per index file), the index file inventory and the
                          one reader `ReadIndex[T]` (`indexstream.go`), `LabelLine`
                          (the label-line recogniser)
                          [kb_index_lib parse, kb_schema, kb_links, verify_md_links]
internal/kbload           `Open(kbRoot)`: reads the stamp, builds `kb.Source` (disk at
                          the current version, `migrate.Chain` in memory for an older
                          one); the only importer of `migrate`; imports `buildrecords`
                          for the record paths (`RecordFiles()`); imported by `cmd`,
                          `build` and `write` only
internal/index            solidity, aggregates, footers, .index emit + freshness,
                          citation gate, claim-id link check, the depends path finder
                          (`DependsGraph.Path`)
                          [kb_index_lib compute, refresh_/verify_kb_metadata, verify_citations]
internal/write            render / values / store / ops, the build-only `add-build-edges`
                          [kb_write]
internal/atomicfile       the one atomic file writer
internal/query                                            [kb_cmd]
internal/sheet            DOT emitter, `dot` invocation: index, build records, leaf,
                          entry-point heading in; SVGs out
`internal/buildrecords`   the three build records' readers and their paths
                          (`RecordFiles`), imported by claimgraph and sheet, and by
                          kbload for the record paths (the ledger's owned paths derive
                          from them; `migrate.SupersededRecordPaths()` names the old
                          spellings)
internal/kbdocs           readiness stamp, README assembly [installed/*.tmpl, kb_readme]
internal/claimgraph       conform, inventory (records + page-defined readings),
                          identify, attribute, shortlist, unmarked, equation sites,
                          classify, equation(s), endcap, hand_named, assemble, write,
                          gate, the three build records      [kb_claimgraph]
internal/asks             embedded templates + fragments + manifest, composer, caller
                          (captures, answer cache), letter-ask seam
                                                          [prompt_templates, letters, ask, classify]
internal/ledger           git checkpoint seam (I1), by exec  [kb_pipeline ledger half, kb_driver/ledger]
internal/filelock         the one advisory file lock (the KB write lock, the build's run
                          lock in the git directory, the state store's holder lock)
internal/build            stage table, state store, resume, --no-inference,
                          --through, status/progress/cancel [kb_driver head, phase-3a, overview]
internal/asks/call        the caller: model.Chat behind the letter seam, captures, answer
                          cache; imported by build alone
internal/migrate          (SPEC §10) the N→N+1 converters over the loose
                          representation and the chain; each converter returns the
                          next form plus the paths it made obsolete; no file I/O
internal/mcp              (SPEC §11) JSON-RPC framing, dispatch, the Tool, Schema and
                          Property types, the frame checks; imports `log` only. The
                          tool table, the resources and the server version are injected
                          by cmd/mcp.go through Serve(in, out, version, tools, resources, lg);
                          cmd/mcp.go is the one self-exec owner (I1)
kept: config, detect, model (model.Chat is the call path), log, version
```

**Dependency rules.**

- `docgraph` knows nothing of claims; the record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files.
- `claimgraph` never imports `latex/*`.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` and `sheet` read the build records
  through it, so `sheet` never imports `claimgraph`; `sheet` reads the letter tables from `asks`
  (never `asks/call`), so the records' yes letters have one home.
- `kb` imports no other kbase package but `log`; `docgraph`, `index`, `write`, `query`, `sheet`,
  `kbdocs` and `claimgraph` read the kb-root vocabulary, the link relation (in kb_tools' two
  readings, one resolver), the label-name grammar and the id minting from it.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph` or `build`.
- `migrate` imports nothing of kbase (I8): it takes and returns files as bytes, and owns the parsers
  of every superseded form; `kbload` is its only importer and the write side drains its delete set —
  `migrate` touches no file.
- No package but `kbload`, `atomicfile` and the build's stage machinery opens a KB file; every other
  read goes through the `kb.Source` from `kbload.Open`. Enforced by `cmd/readpolicy_test.go`, which
  fails on any direct KB file read outside an allowlist.
- `kbload` is imported by `cmd`, `build` and `write` only; `index` does not import it (a refusal is
  a `result.Refusal`, one shape). `kbload` imports `buildrecords` and `migrate`; `buildrecords`
  stays free of `kbload`.
- `mcp` is imported by `cmd` alone and reaches no `model` or `asks/call` (I9); it imports `log`
  only.
- Only `asks/call`, `build` and `cmd`'s `build`, `models` and `configure` verbs import `model`:
  within `cmd`, file-level, `cmd/build.go` and the provider dial the other two share,
  `cmd/provider.go`. `claimgraph` reaches neither `model` nor `asks/call`, transitively.
- Only `build` imports `ledger`; `ledger` imports `log` and nothing else of kbase.
- `build` imports `write` (the write-lock wait); `cmd` imports `build` and `filelock` (the refusal
  where the run lock is held; the held-around-the-run write lock is `write.LockKB`).
- Only `internal/latex/pandoc` execs pandoc; only `internal/ledger` execs git; only `internal/sheet`
  execs `dot`; only `cmd/mcp.go` (file-level) execs kbase itself.

The dependency rules are enforced by an import-policy test over the transitive import graph
(`cmd/importpolicy_test.go`), beside the I1 exec-policy test (`cmd/execpolicy_test.go`).

---

## 12. Constants

| Constant | Value |
|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6) |
| Query default `--limit` | 50 results; chosen so a default-limit result from each list query fits within personant's 8 KB tool-result cap over the six kb_tools-built fixtures, asserted by a test |
| Transport attempts | 3 per call, as kb_tools' `TRANSPORT_ATTEMPTS`; letter asks re-issue at once, the overview call pauses 5 s then 30 s between attempts, as kb_tools' driver does |
| Reader concurrency | `[asks] readerConcurrency` in `config.toml` (SPEC §9), default 4 |
| Metadata format version | The one version read and written (SPEC §10), `kb.FormatVersion`; the stamp key is `kb-format`. `1.0.0` |
| MCP protocol version | `mcp.ProtocolVersion`: the MCP revision the server implements (SPEC §11): `2025-11-25`, confirmed, not chosen, by the owner's live check |
| KB write-lock wait | `write.LockWait`: 30 s, how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8) |
| State-store format | `storeFormat` in `internal/build`: 1, the integer in `<state-dir>/format` (SPEC §6, §8) |
| Builds kept | `keptBuilds` in `internal/build`: 5, the ended builds whose captured output and call captures survive pruning (§8) |
| Run lock path | `<git dir>/kbase-build.lock` (SPEC §6, ARCHITECTURE §8) |
| Detached-build start wait | `build.StartWait` = `write.LockWait` + 10 s (40 s): how long the `build` tool waits for the child's progress record to show a stage entered, or for the child to exit, before returning; derived so a build waiting out a write is not reported as stuck (SPEC §11) |
| Sheet render concurrency | `sheet.renderConcurrency`: 4, the most `dot` processes at once; output order is kept (§6) |
| Unmarked shortlist, rest budget | `shortlistRestK` in `internal/claimgraph`: 4, the best-ranked prose and equation candidates `references-found` asks per source (SPEC §4) |
| Unmarked shortlist, block budget | `shortlistBlockK` in `internal/claimgraph`: 4, the best-ranked block candidates it asks per block source, besides the rest (SPEC §4) |

Exit codes and the placeholder's digest form are in SPEC §7 and §3; the pandoc range is SPEC §2; the
stage-commit subject is SPEC §6.

**Format migration (SPEC §10).** Every verb that reads a KB — `refresh`, `verify`,
`render-claim-graph`, the queries, the write ops, and the build's spine-seed, phase-3a, readiness
stamp, overview and resume — calls `kbload.Open(kbRoot)`; `status` and `cancel` do not load the KB.
`Open` reads `kb-format` alone from the entry point (`kb.ReadFormatVersion`, either frontmatter
form) before anything else. A stamp at the current major and minor builds a `kb.Source` that reads
disk. An older stamp reads every file the format covers (`kb-root/` and the build records beside it)
into a map of repository-relative paths to bytes and hands it to `migrate.Chain(from, to)`, which
applies each converter in turn and returns the current form's files plus the union of the obsolete
paths; the `Source` serves the returned bytes and reads an obsolete path as absent. A newer or
unreadable stamp is refused `check: kb-format`; no entry point is refused `check: kb-root`. Each
converter parses its own superseded form and emits the next; `migrate` is the only home of a
superseded parser, and `kbload` its only importer (import-policy test). The pending obsolete set
lives in the `Source` and reaches the write side only through `index.RefreshReporting`'s save, in
this order: every migrated file, atomically; then the obsolete paths minus what this save covered;
then the stamped entry point, once and last, so the stamp marks a completed migration. The removals
are reported as `removed` by `refresh`, every write op and `render-claim-graph`. A converter is a
pure function with a golden pair as its test; the chain's order is the version order and a gap in it
is a test failure. `internal/kbload/parity_test.go` holds a frozen copy of the `0.9.0` readers, in
test code only, and asserts the migrated KB reads to the same values as the original, over committed
`0.9.0` KBs, one of them built by kb_tools before it adopted `1.0.0`.

**MCP server (SPEC §11).** `cmd/mcp.go` builds the tool table from `rootCmd`: one Tool per command,
less `completion`, `help`, `models`, `configure` and `mcp`. A tool's schema is derived from the
command's `Use` line positionals and its flags. A command's read-only standing is the cobra
annotation `kbase.readOnly`, read when the table is built.

A write op runs through `runWriteOp` with one entry composed from the arguments (`create` lifted to
`--create`); its schema comes from the one exported vocabulary function, `write.OpVocabulary(op)`. A
refusal keeps `entry: 1` and drops `line` and `column`; its `key` keeps the CLI spelling. Every
other tool runs the command's RunE with its flags reset and its output captured to a buffer. The
buffer is the result text; decoded once more, the YAML document is the `structuredContent`; the
document's outcome gives the exit code and so `isError`. A RunE starts from `invocationDir(cmd)`,
never `os.Getwd()`, or `--kb-root` is ignored.

`internal/mcp` holds newline-delimited JSON-RPC over stdio, `initialize`, `ping`, `tools/list` and
`tools/call`, the `Tool`, `Schema` and `Property` types, and the frame checks (`jsonrpc`, `id`,
`method`, known tool, object arguments). Arguments failing the schema (missing, wrong type, unknown
key) are JSON-RPC `-32602` errors, not refusal documents. `Serve` takes the server version, the tool
table and the resources from its caller and imports `log` only. `resources/list` and
`resources/read` serve the sheets cmd/mcp.go supplies, under `kbase://sheet/<kb-root-relative
path>`; an unknown URI is `-32002`, a missing `uri` `-32602`; `initialize` advertises `resources`.

`--kb-root` is resolved before serving: a kb-root directory, or a repository root (holding `.git`)
with or without `kb-root/`; any other path is a usage refusal.

The `build` tool (`cmd/mcp.go`, the one self-exec owner under I1, file-level, enforced by the
exec-policy test) re-executes the kbase binary as `kbase build …`. The child runs from the
repository root, with the server's environment, in its own process group (detach flags per platform
in `cmd/mcp_detach_unix.go` and `cmd/mcp_detach_windows.go`), stdin unset, and the server's
`--config-dir`, `--log-level` and `--log-file` passed through. Every path the child is given is
absolute, because its working directory is not the server's: a relative `state-dir` argument is
resolved against the repository root, while the server's `--config-dir` and `--log-file` stay
relative to the server's working directory. Its stdout and stderr are files in the state store's
`reports/`, `build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`; they are not cleaned up.
The tool waits on the child's progress record and returns when the record shows a stage entered or
the child exits, bounded by `build.StartWait` (40 s, §12; the child takes the run and holder locks
and emits its first progress event before it waits out a write, §8):

| Child, within `build.StartWait` | Result |
|---|---|
| Entered a stage | `done` with `kb-root`, `state-dir`, `pid`, `resume` |
| Exited first | The child's own document; `isError` from its outcome |
| Neither | `failed`, an item naming the state directory and that the child is still starting |

Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
`kb-root/`, returns the refusal `status` gives and starts no process.

---

## 13. Slice verdicts

The reader's design was settled by a slice — `kbase build --through document-graph` over six corpus
papers and a census of all fifty — judged by comparing kbase's tree against kb_tools' for the same
paper with kb_tools' own readers. The hypotheses and their verdicts:

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | kb_tools' conformance and inventory pass on kbase's tree; per-leaf word streams match; every citation state kb_tools' trees exhibit appears in kbase's | A load-bearing form or word no JSON transform can produce | **Confirmed** on the six Slice-1 and Slice-2 papers; 49 of 50 corpus papers build (`test_data/transient/test-integration-slice-arxiv/stage-{1,2,3}/`) |
| Records suffice for the inventory | Every reader and placement fact of kb_tools' stage B inventory (§5.5) is carried by kbase's records, item by item in scan order; every record joins exactly one document | A reader or placement fact that exists only in rendered bytes | **Confirmed**, with §5.5's visibility rule: a block record exists only where the page carries a label line kb_tools reads |
| The preceding text run suffices | Withdrawn: kb_tools defines the word before a reference over the written page, so it is a page-defined reading (§5.5), not a record | — | Withdrawn |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | **Confirmed**; the writer route is pandoc's gfm writer with `--wrap=none` (§5.4) |

**Slice-1 checks:** identical tree path set and child order; equal per-leaf word streams under
`kb_docgraph/text.markdown_tokens`; kb_tools' Document-Tree conformance passes with matching counts
of up-link lines, three-attribute anchors, math fences, `data-cites` citation spans and labelled
blockquotes with identifiers; equal inventory; records carry every reader and placement fact of that
inventory; byte-identical `kb-root/` and records across two runs. All pass: the writer route is
pandoc's. Any fails: execution stops and the owner rules (fix, amend the check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative controls to
refuse as SPEC §5 specifies (naming the paper and pandoc's error; naming each unloaded file and the
line that named it). **Slice-3** records a census, with no pass threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change to any of I1
to I7 is escalated to the owner.

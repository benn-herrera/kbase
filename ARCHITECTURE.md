# ARCHITECTURE – kbase

How this implementation meets [SPEC.md](SPEC.md). Purpose and intent are in
[THESIS.md](THESIS.md); house rules are in [CONVENTIONS.md](CONVENTIONS.md). kb_tools
citations name adjagent commit `88ad25fdcccafdf608e09b8c360155c4b2c44eee`, by section
name; "kb_tools" is the reference implementation at `adjagent/kb_tools/`, and a module
path in `[brackets]` below is the kb_tools module a kbase package ports.

**Status vocabulary.** A mechanism stated without a marker is decided. **Pending slice**
marks a mechanism whose internals wait on the first implementation cut ("Pending slice",
below). A seam is named for each; no detail beyond it is stated. **Needs ruling** is
defined in SPEC's status vocabulary; SPEC §10 lists every item.

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
| I2 | The claim-graph inventory comes from records; only the prose and marker machinery reads leaf lines; maintenance ops read only the metadata layer. |
| I3 | Pre-passes are kb_tools' two, in one registry, each naming the reader-level hole it fills and emitting a census. |
| I4 | System compatibility per SPEC §3; byte equality only where a named reader needs it; presentation is untested. |
| I5 | One result emitter owns stdout. |
| I6 | Every mutating op is idempotent under re-issue. |
| I7 | Imported prompt text, fragments and seat definitions are byte-identical to upstream at a recorded commit, with the same system-prompt placement. |

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
  version give a byte-identical `kb-root/` and byte-identical records; derived state is
  regenerable and freshness-gated.
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
  (replaced by SPEC §7); `--permission-mode` and `inference/claude.py`;
  `install_location` and the installer; `kb_survey/manifest.py` and `skeleton.py`
  (uncalled); the opt-in document audit (`phase-5`).
- Stage boundaries are where expensive work becomes recoverable: an inference stage's
  per-leaf work is uncommitted until its boundary.

---

## 5. Reader and transforms

### 5.1 pandoc seam

`internal/latex/pandoc` is the only package that execs pandoc. It checks the version
range and `pandoc-api-version` and raises named errors, including a missing pandoc that
names the install page. It runs LaTeX to JSON with `--citeproc` and no filters. It is the
reader and the writer seam. [`pandoc.py`]

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

**Needs ruling** at the first slice gate. The route is pandoc's gfm writer over the
transformed JSON with `--wrap=none`, unless the slice finds a load-bearing form it
cannot produce; then the owner rules on a Go writer.

### 5.5 Records

Alongside the Markdown leaves, Go emits records (JSONL) into the build state store
(§8), never into `kb-root/`. They carry reader facts:

- display name, identifier, printed number, optional title, owning document, order;
- references: macro type as pandoc spells it, labels, and the bounded text run before
  each;
- citations: keys and state (resolved, unanswered, key-only);
- references in a proof's opening run.

Claim-bearing classification and proof-to-subject binding remain claim-graph rules. The
claim-graph inventory comes from records, not from re-parsing Markdown (I2). Field
encoding is deliberately unspecified.

### 5.6 Tree derivation, partition checks and `validate_build`

`internal/docgraph` derives the tree and runs the partition checks and `validate_build`
[`kb_docgraph/outline.py`, `judge.py`, `partition.py`; `kb_survey/validate.py`].
**Pending slice.**

---

## 6. KB model and maintenance

| Mechanism | Package | Ports | Status |
|---|---|---|---|
| **Metadata layer**: kb-root model: paths, exclusions, frontmatter and register parse, schema | `internal/kb` | `kb_index_lib` parse, `kb_schema`, `kb_links` | Pending slice |
| **Write API**: render, values, store, ops | `internal/write` | `kb_write/` | Pending slice, except as below |
| **Refresh and verify; the index**: solidity, aggregates, footers, `.index` emit and freshness, link and citation gates | `internal/index` | `kb_index_lib` compute, `refresh_kb_metadata.py`, `verify_kb_metadata.py` | Pending slice |
| **Queries** | `internal/query` | `kb_cmd/` | Pending slice |
| **Sheet**: rendering module, index in and SVG out | `internal/sheet` | The drawing in kb_tools ARCHITECTURE, "The Claim-Graph Sheet" (not ported) | Placeholder |
| **Readiness stamping and README assembly** | `internal/kbdocs` | `installed/*.tmpl`, `kb_readme.py` | Pending slice |

**Write API.** `render` composes every metadata byte, so no op accepts a preformatted
heading, marker or bullet (kb_tools SPEC, "The Write API's Contract"). Input, idempotence
(I6) and the trailing refresh are SPEC §8; byte equality is SPEC §3.

**Sheet.** `internal/sheet` takes the index and returns an SVG. Today the body is the
the placeholder (SPEC §3). `refresh` and
`verify` call it through the seam the real graphing algorithm will occupy; landing the
algorithm replaces the module's body and nothing else.

---

## 7. Inference seams

- Kept from kbase: provider management (`internal/config`), model detection
  (`internal/detect`), the client (`internal/model`), the prompt builder
  (`internal/prompt`), and the call runner (`internal/pipeline`: verification, informed
  retry, fallback, truncation handling).
- Each inference pass is built as plumbing on the runner where kb_tools places it.
  `internal/asks` holds the embedded templates, the composer and the envelope parsers
  [`prompt_templates.py`, `envelope.py`, the `kb_claimgraph` ask]. Prompts, fragments and
  seat definitions are imported byte-identical from upstream at a recorded commit, with
  the same system-prompt placement, and kbase changes no prompt prose (I7). Upstream's
  answer envelope comes with its asks.
- Imported prompts are `@!slot!@` templates (kb_tools ARCHITECTURE, "The Driver",
  `prompt_templates.py`) sent with the seat definition as the system prompt. `prompt`
  builds one user turn. The port adds a template composer and system-prompt support, and
  `model` gains a system message.
- No prompt originates in an in-code string; the overview stage's stub prompt is an
  embedded template file.
- The overview stage's input is **Needs ruling** (SPEC §10); until ruled it is assembled
  mechanically from the entry point, index child lists and volume overview leaves.

---

## 8. Ledger, state store and monitoring

`internal/ledger` is the only package that execs git (I1). `internal/build` owns the
stage table, the state store, resume, `--no-inference`, `--through`, and status,
progress and cancel [`kb_driver` head, `phase-3a`, overview]. Only `build` imports
`ledger`.

- **Ledger.** Commit subject, tracked records and pathspec scoping are SPEC §6. The
  ledger is ported from kb_tools' [`kb_pipeline`, ledger half]; resume position is read
  from the trail alone.
- **State store.** Contents and path are SPEC §6; the key form is §12. The answer cache
  and stamps are recycled from `internal/pipeline`'s store and stamps.
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
through each toolchain. Its recipe is excluded from the hermetic test omnibus: it needs
pandoc and the adjagent reference. The acceptance bar and the sheet exception are SPEC
§3. The harness's structure beyond this is **pending slice**.

---

## 11. Package map

Dependencies point downward only.

```
cmd/                      cobra root; one file per subcommand; outcome-to-exit table
internal/result           YAML result document + emitter (I5)
internal/latex/pandoc     reader + writer seam (I1)       [pandoc.py]
internal/latex/prepass    kb_tools' two scans (I3)        [kb_docgraph/convert.py]
internal/docgraph         JSON transforms, whole-volume render+cut, outline,
                          records, partition checks, validate_build
                          [kb_docgraph/*, kb_survey/validate.py]
internal/kb               kb-root model: paths, exclusions, frontmatter and
                          register parse, schema [kb_index_lib parse, kb_schema, kb_links]
internal/index            solidity, aggregates, footers, .index emit + freshness,
                          link + citation gates [kb_index_lib compute, refresh_/verify_*]
internal/write            render / values / store / ops   [kb_write]
internal/query                                            [kb_cmd]
internal/sheet            rendering module: index in, SVG out; today the NYI placeholder
internal/kbdocs           readiness stamp, README assembly [installed/*.tmpl, kb_readme]
internal/claimgraph       conform, inventory(records), identify, attribute,
                          equation(s), endcap, hand_named, assemble, write, gate
                                                          [kb_claimgraph]
internal/asks             embedded templates + composer + envelope parsers
                                                          [prompt_templates, envelope, claimgraph ask]
internal/ledger           git checkpoint seam (I1)        [kb_pipeline ledger half]
internal/build            stage table, state store, resume, --no-inference,
                          --through, status/progress/cancel [kb_driver head, phase-3a, overview]
kept: config, detect, model, prompt, pipeline (runner, store), log, tokens, text, version
```

**Dependency rules.**

- `docgraph` knows nothing of claims.
- `claimgraph` never imports `latex/*`.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph`
  or `build`.
- Only `asks` and `build` import `pipeline` and `model`.
- Only `build` imports `ledger`.
- Only `internal/latex/pandoc` execs pandoc; only `internal/ledger` execs git.

Each package's internals are **pending slice**. The slice's three packages
(`internal/latex/pandoc`, `internal/latex/prepass`, `internal/docgraph`) are written in
their final locations; whether the slice's code is kept or rewritten is decided at slice
exit.

---

## 12. Constants

| Constant | Value |
|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6) |

Exit codes and the sheet digest form are in SPEC §7 and §3; the pandoc range is SPEC §2;
the stage-commit subject is SPEC §6.

---

## 13. Pending slice

The slice is the first implementation cut: one thin path through real input, run in
three stages (Slice-1: two papers; Slice-2: four more plus two negative controls;
Slice-3: a census of the 50-paper corpus). It builds `kbase build <volume-root>
--through document-graph --state-dir DIR [--bibliography FILE]...`, writing
`<git root>/kb-root/` in `internal/latex/pandoc`, `internal/latex/prepass` and
`internal/docgraph`, with pandoc's gfm writer and `--wrap=none`. It makes no git commits
and takes no lock; it writes records to `--state-dir`. The ledger, lock and resume are
not in it.

Each hypothesis below is tested by comparing kbase's tree against kb_tools'
`document-graph` tree for the same paper, read with kb_tools' own readers. A verdict is
transcribed here when the slice reports.

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | kb_tools' conformance and inventory pass on kbase's tree; per-leaf word streams match; citations appear in all three states | A load-bearing form or word no JSON transform can produce | Pending |
| Records suffice for the inventory | kb_claimgraph stage B's inventory from records equals kb_tools' item by item; every record joins exactly one document | A needed fact exists only in rendered bytes | Pending |
| The preceding text run suffices | The derived word equals kb_tools' `Anchor.preceding_word` for every slice anchor | Space or NBSP handling in the AST loses it | Pending |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | Pending |

**Slice-1 checks:** identical tree path set and child order; equal per-leaf word streams
under `kb_docgraph/text.markdown_tokens`; kb_tools' Document-Tree conformance passes
with matching counts of up-link lines, three-attribute anchors, math fences, `data-cites`
citation spans and labelled blockquotes with identifiers; equal inventory; records equal
that inventory; byte-identical `kb-root/` and records across two runs. All pass: the
writer route is pandoc's. Any fails: execution stops and the owner rules (fix, amend the
check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative
controls to refuse as SPEC §5 specifies (naming the paper and pandoc's error; naming each
unloaded file and the line that named it). **Slice-3** records a census, with no pass
threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change
to any of I1 to I7 is escalated to the owner.

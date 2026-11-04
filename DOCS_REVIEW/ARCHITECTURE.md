# ARCHITECTURE – kbase

How this project's implementation meets requirements designated by SPEC

Must-see:
- No user-facing behaviors or outcomes are designed here, that belongs to SPEC
- No project-specific process is dictated, that belongs to CONVENTIONS
- Every mechanism stated here is decided and landed
  - §13 records the verdicts of the slice that settled the reader's design

## 1. Purpose

kbase makes a knowledge base auditable for an academic lab running local inference. The claim graph
is in scope: the KB carries nodes and edges drafted mechanically and refined by inference, and every
refinement is checkable against the artifact it refined. kbase builds that KB as Go, keeping its own
provider management and inference invocation.

## 2. Invariants

Violating any is a blocking defect.

| # | Invariant |
|---|---|
| I1 | Exactly one package execs pandoc (checks version and `pandoc-api-version`, with named errors); exactly one execs git; exactly one execs Graphviz `dot`; exactly one execs kbase itself (the MCP server's detached build and detached score runs, `internal/mcp/launch.go`). The exec-policy test names each owner and scans the shipped code: the monopoly binds non-test files, and a test may start the processes its fixtures need. |
| I2 | The claim-graph inventory's reader and placement facts come from records, never recovered from leaf markup; its page-defined readings (§5.5) come from the leaf through the page readers; otherwise only stage-A conformance (the link relation, through `kb`), the prose and the statement-marker machinery (the marker reader `kb.MarkerLines` and the hosts the load derives from it, the write ops' placing and moving of a marker, a scoring text's statement), and the sheet (a claim's kind, at its marker) read leaf lines, and maintenance ops read only the metadata layer: the node and edge records, the statement markers and the format stamp. |
| I3 | There are two pre-passes, in one registry, each naming the reader-level hole it fills and emitting a census. |
| I4 | System compatibility per SPEC §3; byte equality only where a named reader needs it; presentation is untested. |
| I5 | One result emitter owns stdout. |
| I6 | No re-issued write changes `kb-root/` ([page §2.10](NEW_MCP_CLI_DESIGN/2-conventions/2.10-existence.md)): an add of what stands, or a removal of what does not, is `refused`, and an edit to a value that stands is `unchanged`. The write ops the page's ops replace are idempotent under re-issue, a re-issue `unchanged`; `init`'s creation is birth-once, and `update-format` refuses a re-issue until its output is committed (SPEC §8). |
| I7 | Every prompt template and fragment is an embedded file. No prompt prose originates in code. Each call's system prompt is the fragment named for it. |
| I8 | One metadata format version is read and written (SPEC §10). Every KB file is read through `kb.Source`, built for a command's read of a KB by `internal/kbload` alone — the build's stage machinery reads the tree its own stages wrote through `kb.OnDisk` (`docgraph`'s `writtenTree`, `claimgraph`'s `builtTree`); `kbload` is the one importer of `internal/formats`' root from outside the tree, which converts an older KB at load — a map of repository-relative paths to bytes in, the same out, plus the obsolete paths; no file I/O; a tree of packages that import each other and no kbase package outside it, so maintenance of a package outside it can never change what an old KB converts to — and the result is written only by `update-format` (`index.UpdateFormat`), whose save removes the obsolete paths, minus what it wrote, after the converted files and before the entry point and the stamp — the save writes and removes nothing outside `kb-root/`: a build record the conversion moves out of the repository root is moved by the verb's `git mv` (`gitledger.Repo.Move`), which `index.UpdateFormat` calls once the load has passed and before the save's first write; every other write refuses a migrated source (`index.PendingConversion`, SPEC §10). The tree also holds the live format's one definition, `internal/formats/format400`, which imports no kbase package and which `kb` alone imports and re-exports. |
| I9 | The MCP server binds subcommands and adds no operation: a command declares its binding with the cobra annotation `kbase.mcp` = `bound` or `excluded`, and a family member its family with `kbase.mcp.family`; the tool table is built from the `bound` commands of the command tree — one tool per family, whose operations are its members, one per other bound command, and none for a member of a family a bound command is named for: the write ops are the family `write`, entries of `kbase write`'s batch, which its tool takes whole (SPEC §11) — and no tool listed by name: every tool has a subcommand behind it, `build-trail` among them; the table's test derives its expectation from the same annotations alone and asserts the set for equality, so a tool listed by name fails as a missing command does; its stdout carries JSON-RPC frames and nothing else; no tool performs inference in the call but `probe`, whose one paid model call its MCP note puts at the user's direction (SPEC §8 `probe`), `models`, `configure-providers` and `configure-models` reaching only the provider's catalogue — `models` reading it, `configure-providers` dialling it to report the catalogue of the entry it writes, `configure-models` checking references against it; `build` and `score` through it are the detached subcommands. A command that reads or writes KB content or metadata is bound, and so is `probe`, a read-only tool; only process control (`mcp`, `wait`), the credentials store's verbs (`auth` and its subcommands — a key arrives on stdin, which no tool call has, and no tool accepts a key; the tool table reads the root's commands alone, so the parent's declaration keeps its subcommands out) and cobra's own `help` and `completion` are CLI-only. |
| I10 | Every ask whose answer a build or a scoring run recorded leaves, for as long as its records are retained, what re-asking it under the binary's embedded templates and re-reading its reply as the run read it needs. A build leaves it through its commit trail, its records and the answer cache's keying on the composed ask, so re-running a stage from its boundary re-renders every ask; and its trail's group records hold each multichoice ask's template inputs — those its group shares once, the rest per item — and its `prompt-sha256`, and each overview call's capture holds its `excerpts` and, on a retry, its `rejected-lines`, so a recorded ask re-renders without the source tree; a scoring run leaves it through its `captures.jsonl` (§6, Scoring). `probe` is a diagnostic and outside it. |
| I11 | While an ask batch's asks are in flight, the only file written is one per result, unique to it, in the state store — a build's answer and capture per call, a score run's result per item — written by the item that made the call, after it has released its token; every file more than one result lands in is written after the ask batch has drained. An ask batch is a build stage's asks (each cull pass and the overview its own), a score walk or a replay walk. |
| I12 | Every model call an invocation makes — a multichoice ask, the overview call, a scoring ask, a probe — is made through a `model.Slot`, which only the invocation's one `model.Queue` issues, from `Go`, and only while one of its N tokens is free; nothing else bounds model calls in flight. An item holds its token across all of its calls and releases it before it writes a file. Within one consumer, items take their tokens in the order the consumer launches them. The methods that put a request on the wire are unexported, so the compiler holds every other package to a slot. |

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
- **The kbase and personant integration is the target.** A prior decision that prevents it is
  removed, not designed around.
- **The served surface is the operating contract.** What an agent needs to drive kbase reaches it
  from kbase itself: a command's own facts in its help; rules spanning commands in the MCP
  instructions, kept short because every session carries them; longer guidance as MCP resources,
  read on demand. The `kbase` skill carries the workflow, as the instructions say (SPEC §11),
  chiefly when to engage kbase at all; a rule a command or the server can state is served, not
  left to the skill.

---

## 4. Stage table

The build walks these stages in order. A stage boundary is a commit (§8).

| Stage | Does |
|---|---|
| `start` | pandoc and git preflight; a fresh build's refusal of any existing `kb-root/`, `init`'s own (`buildstore.KBRootExists`); operation lock; records `started-at` and the inputs, absolute and in path order, that every later stage reads (SPEC §5, §6) |
| `document-graph` | The tree from the volume roots, in path order |
| `spine-seed` | Seeds the format stamp where the tree carries none, then writes the two record files, empty, over that tree (`build.seedSpine`) |
| `claims-declared` | Claim-graph pass over author-declared claims |
| `claims-discovered` | Claim-graph pass over discovered claims |
| `equations-minted` | Equation nodes minted |
| `references-found` | Unmarked references found: adjacency links made by rule between consecutive non-equation claims of a leaf, unasked; per source claim, the proximity slots — nearest in its own paper by locality (same document, same directory, same paper), then by places apart in the sequence of every claim by reading position and id — then the wildcard slots, best by TF-IDF cosine less the proximity picks, counts in §12, no pool holding a marked pair or an adjacency link; each source's shortlist is its own, so a pair both ends pick is asked both ways, and a proximity slot its paper cannot fill stays empty; one multichoice ask per planned pair, a yes in either direction one undirected link for the cull; writes nothing under `kb-root/` |
| `claims-culled` | The co-linearity cull (`Culled`, `internal/claimgraph/cull.go`): one `Colinear` ask per link with a prose end — marked pairs, unmarked yeses and adjacency links, unordered, computed over the tree `references-found` read — grouped by the earlier claim in reading order (`p1-<id>`); one confirming ask per culled claim whose root is not its target, grouped by root (`p2-<root>`); the survivor rule and re-pointing in the pure `cullTargets`, `cullRoots`, `finalCulls` and `PlanCull`; culls issued through `IssueCulls` with no bridge and no claim-to-claim edge; the cull record written once, after the issue. Its row spends its own inference and is dropped under `--no-inference` (SPEC §5) |
| `depends-attributed` | Dependency edges attributed: the marked candidates and the cull record's links, each link oriented later claim to earlier (an equation node always the target) and skipped where a marked pair joins its claims; one multichoice ask per candidate; every answer's edge written in one `add-build-edges` batch, cycles included; then the ring row (`claimgraph.ringRow`): `index.BreakCycles` walks the records in the tree's hosted order and each edge it cuts is issued through `demote-claim-edge`, its warning in the stage report (SPEC §5, §10), before the gate |
| `analyses-split` | Analyses split (`Split`, `internal/claimgraph/split.go`): each claim whose bounds (`claimBodies`, `bindProofClaims`) hold text beyond its statement proper — a block claim's display line, or a prose claim's paragraph through the sentence `opener` picks for its title — gets one analysis. The analysis is hosted on the first blank quoted line of the claim's own block, or at the end of its paragraph's last line, with the claim's marker moved to just after that sentence on the line it ends on (`split-claim`'s `statement-line` and `statement-end`), or, where the claim's own text holds nothing more, on the first blank quoted line of its first bound proof in reading order (`readingOrder`); a claim whose host block has no blank line is left unsplit and counted (`unsplit-no-blank-line`). It is titled with the claim's title rebased to the analysis's register page. Every split is issued in one `split-claim` batch. The stage asks nothing, so no row drops under `--no-inference`, and it writes no build record (SPEC §6) |
| `phase-3a` | `refresh`, `verify`, and the readiness stamp |
| `overview-drafted` | The overview document |

Each claim-graph stage is one `internal/claimgraph` invocation. Every build record a stage writes
lands in `kb-root/.build/` (`buildrecords.Dir`), so the build owns `kb-root/` alone
(`buildstore.OwnedPaths`) and each boundary commit stays within it (SPEC §6).

- `--through <stage>` bounds the walk; `--no-inference` drops the stages' inference rows and walks
  the rest (SPEC §5). Which rows drop follows the classification on the row (`spendsInference` on
  the step table, `internal/buildstore`) and never a stage id. A row marked `spendsInferenceInPart`
  (`depends-attributed`) is not dropped: its asks spend inference, but the row itself still runs
  under `--no-inference`, with a nil `Launch` (`claimgraph.Options.Launch`), which is what not asking is. The provider preflight asks `CallsAModel`,
  which counts such a row.
- `phase-3a` stamps `AGENTS.md`, `CLAUDE.md` and `CONVENTIONS.md`, each only if absent;
  `kb-root/CLAUDE.md` is exactly `@AGENTS.md`. A build resuming over an older KB is refused while
  its conversion is pending (`loadFormat`, §12 "Format migration").
- There is no opt-in document-audit stage (ROADMAP).
- Stage boundaries are where expensive work becomes recoverable: an inference stage's per-leaf work
  is uncommitted until its boundary.

---

## 5. Reader and transforms

### 5.1 pandoc seam

`internal/latex/pandoc` is the only package that execs pandoc. It checks the version range and
`pandoc-api-version` and raises named errors, including a missing pandoc that names the install
page. It runs LaTeX to JSON with no filters, and with `--citeproc` exactly when a bibliography is
offered: without a bibliography, citeproc would render every citation unanswered. It refuses an
`\input` or `\include` resolving outside the volume's directory before
pandoc runs, reading every included file in turn so an escape below the first level is caught
(SPEC §5). It is the reader and the writer seam; the writer runs standalone (`-s`), so the title
and abstract arrive as the rendering's YAML metadata block. The export's one call is
`RunDefaults`, `pandoc -d <defaults file>`, under the same version preflight; the `export-latex`
verb supplies it, and `internal/export` never reaches pandoc. The export's mechanism is §5.7.

### 5.2 Pre-pass registry

`internal/latex/prepass` holds the two source scans, `stripEnvironmentDeclarations` and
`theoremDisplayNames`. Nothing else massages source. They live in one
registry. Each entry names the reader-level hole it fills and emits a census of what it found. A new
entry is an ARCHITECTURE change. (I3)

### 5.3 JSON transforms

`internal/docgraph` transforms the pandoc JSON in Go:

- Cite span wrapping
- author-declared Div to labelled blockquote
- title-page drop
- display maths lifted out of emphasis

Whole-volume render and cut, and outline derivation, also live here.

### 5.4 Writer route

The route is pandoc's gfm writer over the transformed JSON with `--wrap=none`, decided at the first
slice gate (§13): every load-bearing form the slice checked is produced under it.

### 5.5 Records

Alongside the Markdown leaves, Go emits records (JSONL) into the build state store (§8), never into
`kb-root/`. The claim graph takes the tree as its sole input; the records are kbase's channel for
the reader and placement facts, so nothing is recovered from leaf markup (I2). They carry:

- **reader facts:** display name, identifier, owning document, order; references — macro type as
  pandoc spells it, labels; the enclosing author-declared block of each reference and citation
  (pandoc's Div nesting); `\label` names inside display maths; citations — keys and state (resolved,
  unanswered, key-only); the bibliography's works; references in a proof's opening run;
- **placement facts**, set when the tree is cut: each reference's resolved document and fragment,
  and each copy of a heading's reference the navigation lines carry (the up-link line and the index
  child list).

A block record exists exactly where the written leaf carries a label line `kb.LabelLine` reads: an
author block the transform emits as a top-level labelled blockquote whose name that reader's
environment group accepts (letters and single spaces) — and, because the
reader keys on the page, a bold-only paragraph opening an author's blockquote, which it reads as a
label line. A block nested inside another, one rendered inside raw HTML (a figure), or one whose
name the reader rejects (`table*`) yields no record, and the facts inside it host to the enclosing
readable block. One walk derives the outline and the records, so the rule is applied once. Labels
are recorded in the page's attribute spelling: pandoc escapes `&<>"'` in attributes, and differently
for a `Link` than for a `Span` or `Div`, so a record compared against the page carries the escaped
form.

**Page-defined readings** are readings over the written leaf, taken from it through the page
readers: the locator span and title off a block's display line; line extents; the word the page shows
before a reference; a work's rendered text; a claim's statement text — its block extent, prose
paragraph, equation fence, or the section under a heading its marker ends — and the paragraph around
a reference, as ask input; printed
name-and-number mentions in node bodies; the printed number and optional title off the display line,
which no record carries; the source leaf's render shown as the unmarked ask's body. Claim-bearing
classification and proof-to-subject binding remain claim-graph rules. Field encoding is deliberately
unspecified.

### 5.6 Tree derivation, partition checks and `validate_build`

`internal/docgraph` derives the tree and runs the build's own checks: the two partition checks, the
maths-survival count, the anchor-landing check and the image-asset report, then the parts of
`validate_build` that read the written tree rather than restate the cut, and
the dead-link gate over `kb-root/` (§6, shared with verify). Each check compares one mechanical
product against another: pandoc's
parsed document against pandoc's rendering (A), that rendering against the tree kbase cut (B),
pandoc's maths elements against the writer's three forms, the label read back off the link pandoc
wrote, the author's image files on disk, the tree as it lies on disk. Findings land in the stage's
report in the state store (§8, `reports/`); any FAIL makes the outcome `failed`, with no fix loop.

### 5.7 LaTeX export

How `export-latex` meets SPEC §8, whose outcomes — the files of `gen-tex/`, the content rules,
simplification, idempotence — this section cites and does not restate. §5.3 chose Go over a
Lua filter because the forward transform emits records; the export emits none, so pandoc's own
filter mechanism suffices, and the author can re-run the conversion with plain pandoc.

#### One run, three owners

The verb (`cmd/export-latex.go`) opens the KB through `kbload.Open`, refuses a load error and
each `--volume` that is not a volume of `kb.Volumes` (`notVolumes`), and
hands the KB to `export.Run` with the pandoc calls as two function values it supplies: the version
preflight (`pandoc.Preflight`) and the one `pandoc.RunDefaults` call. `export.Run` is the
export's one sequence, its steps unexported so nothing drives them but it: it walks the KB
(`walk`), narrows to `--volume` (`selectVolumes`), assembles `gen-tex/`'s inputs in memory
(`assemble`), calls the preflight, writes the inputs (`writeOut`), calls the defaults run, and
writes the report (`writeReport`); the verb maps what it returns onto the result document. The
call is `pandoc -d defaults.yaml` run in `gen-tex/`, so every path in the defaults file is relative
and the author's re-run from there resolves the same way. The preflight runs before `writeOut`, so
a pandoc refusal precedes any write.

| Owner | Owns |
|---|---|
| Go (`internal/export`) | What pandoc cannot know: the spine's order and ranks, the label namespaces, the KB's contract forms as data. The stream files, the tables, the defaults file, assets, the manifest |
| The filter (`kbase-export.lua`) | Every rewrite of the parsed document: references, citations, links, labels, headings, theorem blocks, figures, maths fixes, the bibliography. Constant code over the tables |
| The template (`kbase-export.latex`) | The LaTeX preamble, package order, theorem machinery, running heads, front and main matter |

Both the filter and the template are embedded (`filter.go`) and written verbatim, so their bytes
do not vary with the KB.

#### Reader

`gfm+attributes-gfm_auto_identifiers`. The KB's maths is gfm's: pandoc 3.12.1's
`markdown` reader has `native_spans` but not `tex_math_gfm`, `gfm` the reverse. `markdown`
would not read the KB's maths, and `gfm` leaves the KB's contract forms (label spans,
reference anchors, citation spans) as raw-html open and close pairs, which the filter pairs
itself. `attributes` supplies the `{#id}` the document boundaries use; with `gfm_auto_identifiers`
off, a heading without one carries no identifier.

#### Streams and file scope

The stream is `kbase-export-front.md` — the entry point's prose, its
first heading taken as the document `title` and its up-link and volume-list lines removed, every
remaining heading marked `{.unnumbered}` — then one `kbase-export-vol{N}.md` per volume in order,
each holding the volume's index and every document of its spine with navigation stripped
(`export.Strip`). `defaults.yaml` lists them as `input-files` with `file-scope: true`, so pandoc
parses each file apart and prefixes its identifiers and `#`-link targets with `<file>__`. The
scope is the volume, for two reasons: one concatenated parse would collide the footnote labels of
different volumes, and a footnote's definition in a built KB can sit in a different document of its
volume than the reference, which a per-document scope would sever. The filter's first act removes
the prefix, for the file names in the tables' `streams` and no other `__`.

#### Document boundaries

The first heading of each document, outside code fences
(`kb.FirstHeading` over `kb.StripCodeFences`), carries the document's label as a `{#d{N}-…}` id,
written by Go; a volume's index carries `vol{N}`. A document with no heading opens with one
naming its path (the file name less its extension; the directory name for an index). Headings
rather than fenced divs, because a `:::` line inside a code fence closes a fenced div early. Go
marks the boundary and the filter reads it: the document a node belongs to is the last heading
before it whose id is a `documents` key, and before the first one the front matter.

#### Tables

`kbase-xref.json` is generated from the walk, each document's first heading and the
bibliography sources, and named to the filter by the metadata key `kbase-xref`. The field-level
schema is the head comment of `kbase-export.lua`; the keys:

- `documents`: heading id → path, volume `N`, rank;
- `paths`: kb-root-relative path → heading id (href resolution);
- `volumes`: in order, each with its `vol{N}` label and title;
- `theorems`: every display name `kb.LabelLine` reads off a label line of the front matter or an
  exported document, so the label-name grammar stays in Go;
- `bibliography`: citation key → entry body as gfm text. Each exported volume's `references.md`
  entries (`kb.BibliographyEntries`) and, where any exported volume has none, every work record
  (a work's title), the first entry of a key standing;
- `report`: the path the filter writes its findings to;
- `assets`: kb-root-relative path of a copied file → its path under the out directory; the
  claim-graph sheets (SPEC §3) are not copied, being drawings of the metadata;
- `entry_point`: the entry point's path, the front matter's source;
- `streams`: the stream file names in reading order.

#### The filter

Preparation, then two passes over the one document pandoc reads, then the end.

Preparation. The file-scope prefix is removed. Each block list is made native: a raw-html `figure`
or `table` the gfm reader split at its blank lines is reassembled and read as html (its MathML as
maths carrying the annotation's TeX; spans left as raw pairs), a raw `<img>` or `<embed>` becomes
an image, and the copy a built KB writes beside such a tag is dropped. A table with no caption or
id followed by a paragraph ending in an empty id span (the form pandoc's gfm writer gives a
captioned table, `{#tab:…}`) takes that paragraph as its caption and the id as its own.

Pass 1 collects author labels under the volume of the document they sit in, keyed (volume,
label): every `\label{L}` in a maths body, every `<span id="L">` pair, every figure and table id,
and every id on a native span, link, div or code block — the elements whose id the writer prints
as a label. A figure that holds a table is not one. A label defined twice in one volume is a
finding, as are two labels of one volume spelled alike in LaTeX.

Pass 2 rewrites in document order; anything a rule returns in place of its element is walked by
every rule, once.

- `<span id="L">…</span>` pair → span with id `v{N}-L`, which the writer prints as
  `\protect\phantomsection\label{…}`. A native span, link, div, code block, table or figure id
  is namespaced the same way.
- `<a data-reference-type=… data-reference=… href=…>…</a>` pair (also a link carrying the
  `reference` attribute, which the html reader makes of one inside a figure or table):
  `data-reference` names the target. Where the href names an exported document, or is absent,
  and the target volume (that document's, else the current one) defines every label named →
  `\eqref` for `eqref`, one `\cref{…}` for `ref+label`, one `\Cref{…}` for `ref+Label`, `\ref`
  otherwise, with labels `v{M}-L`. Where the href names an exported document but the volume does
  not define every label → `\ref{d{M}-…}`. Otherwise the anchor's text and a finding.
- `<span class="citation" data-cites="k1 k2">…</span>` pair → `\cite{k1,k2}` where every key has a
  `bibliography` entry, keys recorded in first-emission order; otherwise the span's text and a
  finding, so no `\cite` is undefined and no text is dropped.
- A `\label`, `\ref`, `\eqref`, `\cref` or `\Cref` with a braceless argument inside a maths body is
  namespaced to its document's volume; a reference whose label the volume does not define is
  left as written and a finding. A blank line in a maths body becomes a newline, since maths mode
  cannot hold a paragraph break.
- A plain link to an exported document (`[text](x.md#frag)`) → a link to that document's heading,
  which the writer prints as `\hyperref`; to a copied asset → the copy; an external link passes;
  any other internal link → its text and a finding. A link whose content holds a reference
  anchor → its content alone, the anchor naming the target. An image is pointed at its copy under
  `assets/`; one naming no copied file becomes its alternative text and a finding.
- A labelled blockquote — its first paragraph a label line (`**Name**`, optionally inside a label
  span pair) with `Name` in `theorems` — is unwrapped; see **Theorem blocks** below. Any other
  blockquote stays a quote.
- Headings are shifted by their document's rank: the document's own heading to level rank + 1
  (a volume index, rank 0, is the chapter), in-document headings below it by their offset. Past
  level 6 → a bold paragraph and a finding, a document's id kept as a label. Every heading id
  that is not a document's is removed, so only `vol{N}` and `d{N}-…` ids remain.
- A figure of more than nine blocks → its blocks one under another: pandoc 3.12.1's writer sets
  n blocks side by side at 0.9/n of the line and prints a width under 0.1 as `9.0e-2`, which
  LaTeX cannot read. A figure holding a table → the table's body and caption unfloated, and a
  finding, since the writer drops a figure's float, caption and label around a table.
- A table whose column widths are all the writer's default, and whose columns' longest cells
  (a formula counted as its TeX less markup) with the gaps between them run past the template's
  line, taken as 96 characters → relative widths, so its cells wrap: a column whose longest
  cell fits an equal share of the width left keeps that width, the others share the rest by
  their longest cells, none narrower than its longest word and two fifths (a bold word, alone in
  its cell, cannot break). A table that fits keeps the writer's natural columns; one that
  carries widths keeps them.
- Display maths that is one `eqnarray*`, `xalignat`, `xalignat*` or `xxalignat` environment →
  raw LaTeX: pandoc 3.12.1's writer wraps those, alone of LaTeX's and amsmath's display
  environments, in `\[…\]`. A maths body whose last line holds an unescaped `%` ends with a
  newline, since the writer closes the maths on that line and its own guard reads `\\%` as
  escaped.
- Raw html no rule claimed is a finding where it carries `data-reference`, `data-cites` or an
  `id`, or is a `figure`, `table`, `math`, `img` or `embed` that did not read as a native
  element: the writer drops raw html.

The writer does the rest unaided: display environments bare (but those above), inner ones
(`pmatrix`, `aligned`, `cases`) in `\[…\]`, inline maths in `\(…\)`; markers (raw html comments)
dropped. Prose is not matched: `(eq:L)`, `(Author, Year)` and a hand-maintained KB's bold-line
theorem blocks export as written, and `\tag{L}` stays, so the printed tag and the prose that names
it agree; such a KB's internal links resolve through the link rule and its work records form
the bibliography.

The end: `thebibliography` over the cited keys in first-emission order, each body rendered with
`pandoc.write(pandoc.read(body, "gfm"), "latex")`; the `\kbasetheorem` declarations (below) in
`header-includes`; and the findings, a JSON list of `{form, path, reason}` (`path` the document's
kb-root-relative path), written to the tables' `report` path.

#### Labels

Three families that cannot collide: volume `vol{N}`; document `d{N}-{path-slug}`
(`export.DocumentLabel`: the path below the volume directory less its extension, each segment
lowercased with runs outside `[a-z0-9]` one hyphen, joined by hyphens; an index slugs as its
directory name and `index`); author `v{N}-{label}`. Go mints the first two; `selectVolumes`
numbers them after `--volume` narrows the list. A document label another path already owns is a
walk finding, and that document gets no id. The author spelling is the filter's `namespaced`,
the one function every rule that writes a label or a reference calls, so a raw `\label` in maths
and an id the writer prints agree: it keeps ASCII letters, digits and `+-.:;=_` and spells any
other character `ux{hex}`, whatever the label's own shape.

#### Theorem blocks

A labelled blockquote whose first body paragraph opens with its printed number
— `**Display N**.` or `**Display N** (Title).` — becomes `\begin{kb-<name>}[{Title}]\label{v{N}-L}
… \end{kb-<name>}`, `<name>` the label line's name lowercased with spaces as hyphens, the opening
dropped, its title kept. A bare-number opening takes the label line's name for `Display`. For each
environment used, `\kbasetheorem{kb-<name>}{Display}{Plural}` is added to `header-includes`,
`Plural` by an English rule in the filter; the `kb-` prefix is no LaTeX environment's, so the
declaration is unconditional. Without that opening (a proof, which prints its own head and end
mark; a LaTeX environment's name such as `minipage`) the body blocks stand as they are, unquoted,
and the label span's id is kept as a label. The template's `\kbasetheorem` owns the rest: the
`definition` style, since the statements carry their own emphasis; one counter shared by every
kind, numbered within the chapter; and the cleveref names.

#### Template

`book`, `oneside` and `openany`, so every page is laid out alike and no blank page
precedes a chapter, with options for `hyperref`, `url`, `cleveref` and `geometry` (`margin=1in`,
on LaTeX's default letter paper) passed before the class so that an author's own load of them
does not clash and options the preamble passes them take effect; engine-branched fonts
(`iftex`); the maths packages, `amsthm` and `\kbasetheorem`; the Unicode mappings for Greek,
arrows, relations and sub- and superscript digits; the writer-conditional blocks (tables,
highlighting, strikeout and the rest, switched by pandoc's template variables); then
`\InputIfFileExists{../preamble.tex}`; then `geometry`, `microtype`, `hyperref`, `bookmark`,
`cleveref` last, then the `header-includes`. The preamble's path is relative to the working
directory, so the author compiles `main.tex` from inside `gen-tex/`; where `../preamble.tex` is
absent from the working directory, the template types a notice and loads nothing. The rest:
mixed-case running heads naming the chapter on every page; the first numbered chapter opens the
main matter, everything before it being front matter, the title page its unnumbered page i and
the contents following from ii; `\backmatter` before the bibliography; and
one generation comment naming kbase's version (`kbase-version`, set by the verb in
`defaults.yaml`'s `variables`).

#### `gen-tex/` and the preamble

`writeOut` owns `gen-tex/`. A `gen-tex/` holding files and no
`kbase-export.yaml` is an `ExportDirError` (the `export-dir` refusal) and nothing is written. The
manifest lists every file the export owns — the assembled files plus `main.tex`,
`kbase-export-findings.json` and the report — and is written after the files it names. A file
whose bytes are already there is not rewritten; a file the previous manifest listed and this
export does not own is removed, with the directories the removal empties; each manifest path is
checked to lie inside `gen-tex/` before any removal. The starter `preamble.tex` is written beside
`gen-tex/` only where nothing stands at that name and is never opened afterwards; the report
checks its existence only. These are the only writes outside `gen-tex/`.

#### Idempotence

Assembly is deterministic in the KB and the kbase version, so a re-issue
reproduces every assembled file byte for byte and `writeOut` writes none. pandoc rewrites
`main.tex` and the findings each run, so the verb compares their bytes before and after; the
outcome is `unchanged` when nothing was written, removed or rewritten with different bytes,
the report included.

#### The report

`kbase-export-report.yaml` is one YAML list: the walk's findings (`export.Report`:
a target not walked, a path named twice, a label collision), then the filter's, then, where `preamble.tex`
exists, a line naming it as the author's. `writeReport` reads the filter's findings back from
`kbase-export-findings.json`, so the report exists only after pandoc has run.

---

## 6. KB model and maintenance

| Mechanism | Package | Status |
|---|---|---|
| **Metadata layer**: kb-root model: names, exclusions, link primitives and the dead-link gate over `kb-root/`; the link grammar — an inline link with its title, a citation's id, a bare id (`LinkRE`, `Links`, `BareIDs`), defined by `format400` and re-exported here, the one pattern the dead-link gate, the tree's link readers and the converter read links through (SPEC §10); the links in a document's text (`TextLinks`: every inline link outside code, with its line and its offsets on the raw line; `TextLinkDefinitions`: every reference definition outside code, with its destination's offsets), the citations among them (`TextCitations`: each whose title is a node id) and the citing documents (`CitingDocuments`: every Markdown document but those standing at a summary page position), the one reading `citations-of`, `remove-node`'s refusal, the edits' renaming and every op's re-pointing share (SPEC §8); the uses of a citation key as a key (`CitationKeyRefs`, `bibliography.go`: a citation span's `data-cites` token and key-only rendering, a reference-list entry's id, a link's `#ref-` fragment, outside code, a blockquote's prefixes read past), which a work's label edit and a work's removal read (SPEC §8); the record loader (`LoadGraph`, the two record files decoded and encoded by `format400` and re-exported here); the statement-marker reader and the hosts it derives (`MarkerLines`, `Hosts`); a hosted node's statement as the layout around its marker gives it (`Statement`, SPEC §8 `add-citation`); the one "no node" refusal's text and the query its remedy points at (`NoNode`, `NodeLister`, beside `CheckUnknownID`); every heading's anchor in a document (`HeadingAnchors`, through `format400`'s slugger, `Slugify` and `Slugger`, re-exported here; SPEC §10); the document kind (`DocumentKind`, from the child list); the format stamp and its file; schema; the register pages' rule — their file name, title and entry headings (`RegisterFile`, `SummaryTitle`, `SummaryEntryHeadings`), the volumes among a KB's documents (`Volumes`), the page a host's entry stands on (`SummaryPage`) and every path a page can stand at (`SummaryPagePositions`: kb-root's and each volume's) — defined by `format400` and re-exported here, the one rule index renders by, the build rebases titles by and the converter predicts; the derived-line renderers the register pages are written through | `internal/kb` | Landed |
| **Write API**: render, values, store, ops — every write a batch, every op editing the records and the statement markers (`records.go`, `marks.go`), the batch's derivation computed in memory and written with them (`ops.go`) | `internal/write` | Landed. An add (`add-<kind>`, `adds.go`, SPEC §8) adopts nothing: it refuses, against the records as the batch's earlier entries left them, a title its kind holds (`titleHolder`) and a label a work or an axiom of its kind holds (`labelHolder`), then mints, for every kind, and places a hosted node's marker through `markAtLine` once `checkNamedLine` has refused a line past the end, a fence's opening or closing line and a line without the snippet. A hosted add and `add-work` run through `keepSummaryLinks`, which re-points every link into a summary entry whose repeat suffix the new entry shifts, from `anchored` before and after; `summaryMayShift` first rules out, without a derivation, an entry whose slug, its repeat suffixes stripped, no other title and no fixed rigor or strength heading shares, so a build's many adds derive nothing extra. `add-axiom` appends after every heading of `axioms.md` and shifts none; `add-axiom` marks the one unmarked heading of its text in `axioms.md` or appends its marked heading, refusing two unmarked headings of its text, `axioms.md` carried with the leaves as a `leafEdit`. An edit (`edit-<kind>`, `edits.go`, SPEC §8) replaces the fields its entry gives through `nodesOf`, each kind's records as their editable fields; an add and an edit judge a title's uniqueness within its kind by one lookup, `titleHolder`, and an edit a label's by `labelHolder`. A title or a label change (`rename`) reads each work's and axiom's link target and every node's summary entry before and after it from `index.Derive` over a source holding the batch's records and documents as its earlier entries left them (`anchored`, one derivation each side), then rewrites in every document but the summary pages, each document it changes a `leafEdit` like a leaf; `edit-axiom` rewrites its marked heading in `axioms.md`. The naming form takes the new title through `renameCitations`, which refuses at `title`, naming the citing `path:line`, a rewritten link `readsAsOneLink` does not see whole; a work's new label replaces every use of the old key as a key (`rekey`, over `kb.CitationKeyRefs`), after `refuseKeyInUse` has refused a label a span or an entry already uses. Those rewrites change lines in place, so `movedHeadings` finds each heading whose anchor they moved by comparing `kb.HeadingAnchors` line by line, before the entry and after it, and `summaries` adds each summary entry whose anchor moved; `repoint` then gives every link outside code, a citation or not, an inline link or a reference definition, one destination, computed from the one it had before the entry: the node's new target where a citation of it pointed at the old, otherwise the moved heading's or entry's new anchor, only the fragment changing (`movedAnchors`). One map, read once per link, so no link is re-pointed twice and a shifted repeat suffix (`on-x-1` → `on-x`) is never chained. The edge writes (`add-edge`, `edit-edge`, `remove-edge`, `restore-demoted`, `edges.go`, SPEC §8) look an edge's ends up in the batch's records (`edgeEnds`, refusing through `noNode`, the one "no node" refusal the edits share) and its identity (`holds`, `noEdge`); a scoring edge's cycle check is `closesCycle`, whose premise graph is built once per batch and cleared by every op that takes a scoring edge out (`dropEdges`, `demoteEdge`). An edge entry's `source`, `target` and `relation` are read from its values once it has applied; `restore-demoted`'s `restored`, known only while it applies, travels on the per-entry report `applyEntries` copies into `EntryResult`. `remove-node` and `move-node` (`nodes.go`, SPEC §8) name a node by `id`. `remove-node` reads the rows of the edges it removes from `index.Derive` over the batch's records and documents (`derived`, which `targetOf` shares), drops them through `dropEdges`, takes the record and the marker out through `dropNode` inside `keepSummaryLinks`, or an axiom's headings out of `axioms.md` through `dropAxiomHeadings`, which re-points every link into a later heading whose repeat suffix the removal lowered, and then refuses a mention that outlives the node (`refuseMentions`): a citation in any document but the summary pages, walked by `eachDocument` — `kb.CitingDocuments` and each document an earlier entry of the batch created — which `renameCitations`, `rekey` and `repoint` share, for a work a citation span carrying its label where no reference-list entry has that key (`kb.CitationKeyRefs`), or a surviving record's prose field (`graphProse`). Its `removed-edges` travels on the per-entry report as `restored` does. `move-node` checks the named line through `checkNamedLine`, places the marker with `appendMarker`, takes every other occurrence out (`unmarkEverywhere`, keeping the named line), and re-points every citation through `repoint` to `targetOf`, a hosted node's host and its marker's section anchor (`index.SectionSlug`, the derivation's own). A document the batch edits is a leaf as the derivation reads it, by its child list (`leafAt`), so a citation in a leaf leaves it a leaf. `add-citation` (`excerpt.go`, SPEC §8) opens any document of the KB as a `leafEdit` (`citingDocument`), finds the one span of the named line its snippet names outside code, links and markers (`citedSpan`; a missing one refused through `snippetAbsent`, which `checkNamedLine` shares), links it to `targetOf`, refuses a link the reader would not see whole through `readsAsOneLink`, and checks a quote against `kb.Statement`, the cited node's statement read from its host leaf by the layout around its marker, the lifts edges the batch holds deciding the split layouts. A link spliced into a heading moves its anchor, so `add-citation` runs `movedHeadings` and `repoint` after the splice, every link into a moved heading re-pointed, the new link among them; its document, line and link travel on the per-entry report as `restored` does. There is one KB write lock, `write.LockKB(kbRoot)` (`lock.go`): `internal/filelock`'s advisory lock on the directory `kb-root/` sits in. The caller takes it and holds it across the whole run (SPEC §8): `cmd` from `kbload.Open` through the batch's apply, derivation and write to the result, and around `update-format`; a scoring run takes it around its one landing, from `internal/score`, and around its start refresh, from `cmd/score.go`, never across a run (§6's Scoring). The ops and refresh themselves take none, and the per-directory splice locks are gone. A writer that cannot take it within the wait (§12) gets `retry`, `check: lock`, `path` the repository root — the one item `write.HeldLockItem` gives, beside `LockKB`. `verify` and the queries take none. The build takes and releases it once to wait out a write in progress (§8). On Windows, where no advisory lock exists, only the read-then-compare check guards against a concurrent writer |
| **Refresh and verify; the index**: the derived rows in memory (`Derive`), the cycle load error and the cycle walk (`BreakCycles`), strength, the register pages and their freshness, `update-format`'s save (`UpdateFormat`) and the pending-conversion refusal (`PendingConversion`), the bare-id check (`BareIDFindings`, SPEC §8 `verify`) and the claim-id link check, a citation's title id among the ids it checks | `internal/index` | Landed. The build's check is `Verify`; a citation is checked as every link is. `LinkFindings` hands `kb.DeadLinksIn` the register pages' derived slugs, which turns its fragment check on: a fragment into a register page is judged against those slugs, one into any other Markdown document under `kb-root/` against its heading anchors and element ids (`documentAnchors`), read once per document; `kb.DeadLinks`, which the build's document-graph and conformance gates call, checks files alone. Each `Finding` carries its own remedy: `update-format` for `index-freshness`, the empty write batch for `register-freshness` (`EmptyBatchRemedy`), but `update-format` over a KB whose conversion is pending, where the batch is refused, and a write batch of the `add-citation` entry for `bare-id`. A register page's entry anchors are computed at derive (`pageAnchors`): the page's title and every entry's headings (`kb.SummaryEntryHeadings`, defined by `format400`) through one `kb.Slugger` in page order, so a claim's and a work's canonical anchor, a page's links and the dead-link gate's page slugs read one map; a test holds the rendered page's headings to it |
| **Queries** | `internal/query` | Landed |
| **Sheet**: DOT emitter and `dot` invocation; `Derive`'s rows, the leaves and the entry point's and volumes' headings in, SVGs out | `internal/sheet` | Landed as the drawing of SPEC §3; the placeholder only where `dot` is absent |
| **Readiness stamping and README assembly** | `internal/kbdocs` | Landed: the stamp (only where absent), the maintained blocks (`Maintain`: each document's one ordered pair of fence lines, recognised by their ends, its span rewritten to the template's block; an absent `AGENTS.md` or `CONVENTIONS.md` written whole, `README.md` only where it stands, a document without the pair listed and untouched; `AGENTS.md`'s scope pin `AuthoredKBPin` where no build record stands, `init` writing none), the overview excerpts, the passage structure check, the README assembly; the templates under `internal/kbdocs/templates/` |
| **Scratch creation (`init`)**: the zero-case KB — entry point, the format stamp file, the two empty record files, readiness stamps, refresh over the skeleton, the plain-subject birth commit | `cmd` (the verb), reusing `kb`'s stamp codec and record encoder, `atomicfile`, `kbdocs.Stamp`, `index.RefreshReporting`, `write.LockKB` and `gitledger` | Landed: refuses a multi-line `--title`, outside a worktree and at any existing `kb-root/` before any write; the spine bytes come from the canonical stamp and record encoders; the stamp file stands before the loader opens the KB (§10); `verify` over the born KB is `test-integration-init`'s, not the verb's; the birth commit's subject is outside `ledger.Prefix`, so §5's fresh-vs-resume reads the trail as empty |
| **Scoring (`kbase score`)**: the closed text one call carries and its guard, the runner, the run's ledger, the run's entry and its landing | `internal/score` | Landed: the set is `query.ScoringSet`'s, the ledger stands under the build's state directory (`internal/statedir`, extracted from `internal/build`), and the verb is `cmd/score.go`, bound as a detached tool in MCP |

### Write API

Every write is a batch (SPEC §8), run by `write.Run` — `write.Batch` for `kbase write`'s mixed
batch, an op's name for a batch of that op alone — in the phases [page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md) orders:

1. *Shape.* `parseValues` checks every entry against its op's closed vocabulary, a mixed entry's op
   read from its `operation` key, then each op's own checks over its entries (`batchChecks`: an
   edit's `refuseFieldless`), and returns every fault at once.
2. *Apply.* `applyEntries` reads both record files through `kb.LoadGraph` and applies each entry
   in order to the batch's `kb.Graph` and to the leaves it marks; the first refusal stops the
   batch and names its entry. Each entry's report is
   whether the record files or leaves moved under it (`done` or `unchanged`) and, on an add, the
   id it minted (nothing is adopted: an add of what stands is refused, SPEC §8), or, on `restore-demoted`, the relations it put back, or, on
   `remove-node`, the edge records it removed, or, on `add-citation`, the node it cites and the
   document, line and link it wrote. The two files are rendered whole through `kb.EncodeEdges` and
   `kb.EncodeNodes`, and a file whose bytes did not move is no change.
3. *Derive.* The changed record files and leaves are put into the `kb.Source` overlay, unwritten,
   and `index.PlanDerivation` computes the register pages and sheets from it, writing nothing.
4. *Write.* Once `readBackMarkers` has found each marker where it was placed and the disk still
   holds what was read, `writeChanges` writes `edges.yaml`, `nodes.yaml`, the leaves, then the
   derived files, each temp-then-rename, and removes what the derivation drops — unless the batch
   is a dry run, whose result is the same with nothing written. A committing batch (the document's
   `commit`, or `write.Options.Commit`) first calls the caller's `write.Options.Preflight`, and its
   error stops the batch with nothing written; a committing dry run calls it too, which opens the
   repository and commits nothing. Once a real run's pre-flight returns, the disk is checked again against what was read, so a hook that edited a file during the pre-flight
   turns the batch into `retry`. `cmd/write_op.go`
   supplies it: `kbCommits.preflight` commits `kb-root/`'s uncommitted changes outside `.index/`
   (owned `kb-root` less `gitledger.Exclude("kb-root/.index")`), then those inside it, each only
   where `Repo.Dirty` finds any. Once `write.Run` returns `done`, `kbCommits.final` commits
   `kb-root/` under the label `batchLabel` composes from the entries' ops, their ids
   (`EntryResult.ID`) and the ids their values name (`EntryResult.Names`); each commit's `paths`
   are what `Repo.Paths` reads back from it ([page §2.2](NEW_MCP_CLI_DESIGN/2-conventions/2.2-write-batches.md)).

A failure in any phase before the write writes nothing; one inside the write is `failed`, naming
what already landed. Every batch derives, the build's and the scoring run's own writes included
(SPEC §5, [page §2.3](NEW_MCP_CLI_DESIGN/2-conventions/2.3-derivation.md)); no option skips phase 3. The one batch written without its
derivation is a build-only op's (`add-build-edges`, `demote-claim-edge`) that leaves the scoring
edges holding a cycle (`holdsOnlyCycles` in `ops.go`): the derivation cannot read such records,
and the build's ring cut demotes the cycle in the batch after, whose derivation brings the derived
files current before the stage's boundary. The build's node writes are `add-claim` and
`add-work`, its culls one mixed batch of `remove-node` entries ordered so a culled claim whose
rationale mentions another is removed first (`write.Mentions`, the test `remove-node` refuses
by), an `edit-claim` clearing a rationale where culled claims mention each other round a loop
(`internal/claimgraph/elision.go`); a title its kind holds already takes a counter, ` (2)` on
(`uniqueTitle` in `internal/claimgraph/write.go`). A block claim so countered no longer meets
its block by title in `readGraph`; it joins the block of its leaf whose display line its marker
stands on — the line `add-claim` was given, found by the same `write.ExcerptLines` search within
the block's extent — where no claim of that leaf carries the block's own title: an exact,
positional match, never the counter read back. That the decoder reads back
exactly what the encoder wrote is `format400`'s round-trip unit test, not a runtime check. A batch
that derives first refuses what `index.Precheck` refuses (SPEC §8). No op
accepts a preformatted record or marker: every value is a values key, and the marker is
`kb.StatementMarker`'s. A scoring edge's judgement is written by `add-edge` and `edit-edge` through their `bearing`, and a
node's local value by `add-<kind>` and `edit-<kind>` through their `rigor`; each refuses a value
outside the domain or the pending literal, and the edge ops a pairing the relations table does not join and a
`bearing` on `references`, and `edit-edge` a `demoted` record (SPEC §8). A written `0` re-reads
as `0` and a written `*pending*` as pending: `kb.Bearing` carries `Value` and `Pending` apart.

### Provenance

`write.Options.Provenance` says who supplied the values a call writes, and decides
the `manually-scored` flag each rigor and bearing is written with (SPEC §8): `ProvenanceCaller`, the
zero value, marks every value its caller supplied manual, so an internal caller that names none
never lets a run overwrite a person's value; `ProvenanceMachine` writes `false`. `claimgraph`'s
write runner, `internal/score`'s landing (`landValues`, which writes every value as `ProvenanceMachine`, a landing and a re-price alike) and the build-only
`add-build-edges` and `split-claim` (each a `recordEdit` marked `machine`) write as the machine. It
is an options field and never a values key, a flag or a schema property:
`TestNoSurfaceNamesTheProvenanceOrCreate` (`cmd/mcp_test.go`) asserts no command flag and no tool
property names it. No build path writes a judgement: `add-build-edges` writes `*pending*` on every
edge it adds and no number anywhere, and `split-claim` writes `*pending*` on the analysis's rigor
and its lift, so a claim a build attaches an edge to reads unchanged until that edge is scored.

Input, idempotence (I6) and the derivation after a batch are SPEC §8; byte equality is
SPEC §3. Three ops are build-only:
`add-build-edges` (`write.AddBuildEdges`), which writes the build's edges as `add-edge` writes one, without its cycle
refusal; `demote-claim-edge` (`write.DemoteClaimEdge`), which replaces one standing scoring edge,
named by its `source`, `target` and `relation`, with the `kb.DemotedEdge` that keeps it whole; and
`split-claim` (`write.SplitClaim`). Per claim, `split-claim` mints an analysis and appends its
marker to a given line of a given leaf, and where given a `statement-end` moves the claim's marker
to that byte offset of line `statement-line`, or of its own line where none is given. It adopts
instead the analysis already lifting the claim. It moves every `supports` and `founds` record into
the claim, and every `demoted` record that replaced a `supports` edge into it, onto that analysis,
and adds the analysis's `lifts` of the claim at `*pending*`. All three are absent from
`write.Ops()` and have no subcommand, so the public write surface cannot create or move a `demoted`
record. The first two run over records holding a cycle among the scoring edges, which the build's
edge batch closes and its ring row demotes within one stage (`index.DeriveOverCycles`). The build's cuts reach the records through
`demote-claim-edge`, from the ring row at the tail of the dependency-attribution stage. The
analysis split reaches them through `split-claim`, from the `analyses-split` stage.
`restore-demoted` is a public op, and `remove-edge` removes a `demoted` record.
`index.DependsGraph.Path` is the path finder over the scoring edges (`index.GraphDependsGraph`);
every public op that adds a scoring edge, through the batch's `closesCycle`, and `restore-demoted`
refuse an edge it finds closing a cycle, naming the path in edge direction: `closesCycle` reverses
the premise graph's path. `addEdge` deletes the `demoted` record (`kb.DemotedMatch`)
of a scoring edge it writes, so an add over a cut writes the edge with the add's values.

### The node vocabulary

`kb.NodeKinds` is five words — `claim`, `analysis`, `experiment`, `axiom`,
`work` — and nothing else is a `node_type` or a `target_kind`. `kb.BearingLegal(relation, sourceKind,
targetKind)` is the one predicate answering whether a pairing may carry a bearing: every scoring
pairing may, and `references` and `demoted` may not; it answers that and never whether an edge may
exist. Every node id is minted ([page §1.1](NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)): its kind's three-letter prefix over a body
`kb.MintID` draws, one grammar for every kind (`kb.IDBody`, over `kb.IDKinds`). Each prefix is a
`format400` constant re-exported through `kb` (`kb.ClaimPrefix`, `kb.AnalysisPrefix`,
`kb.ExperimentPrefix`, `kb.WorkPrefix`, `kb.AxiomPrefix`); `kb.NodeIDKind` reads an id's kind off
them, and `kb.NodeIDs` finds an id of any kind in text. A work's citation key and an axiom's label
live in the record's `label` field (`WorkNode.Label`, `AxiomNode.Label`), never in the id; the
strict load holds a work's label to `kb.WorkLabelPattern`. An axiom's prose is `axioms.md`, its
heading paired with its record by the axiom's marker: `kb.AxiomHeadings` reads each heading carrying
an axiom's marker, and each unmarked `### Axiom ` heading, for `add-axiom`, `edit-axiom`,
`remove-node` and `verify`'s `axiom-heading` check, and `kb.AxiomAnchor` gives the derivation an
axiom's anchor, that of its marked heading. A statement marker hosts only a claim, an analysis or
an experiment (`kb.HostedIDKinds`); an axiom's or a work's id in a marker hosts nothing.
`update-format` mints each `3.0.0` work's and axiom's id from its `3.0.0` id alone
(`from300to400`'s `mintIDs`). The legacy declaration forms are
the superseded readers' alone (`internal/formats`). SPEC §10 carries the prose legend the authors'
words map through, and it is the one place the synonyms live.

### The derived layer

`internal/index/strength.go` is the arithmetic's authority: the four
relations, their three stations — the minimum for `supports` and `founds`, the maximum for `lifts`,
the cap for `refutes` — the pending rule and the value's two spellings live there and nowhere else.
The walk computes at full precision and nothing else rounds a strength: `FormatStrengthRecord`
writes the value in the fewest decimals that read back as it — a register page's strength line and
premise annotation — and a query result carries the value `FormatStrengthDisplay` spells, where a
trace spells it.

| Relation | Source kinds | Target kinds | Station | Contribution |
|---|---|---|---|---|
| `supports` | claim, experiment | claim, analysis | minimum | `bearing × source strength` |
| `founds` | work, axiom | claim, analysis | minimum | `bearing × the source's rigor`; `kb.AxiomRigor` is an axiom's `1.0` by rule |
| `lifts` | analysis, experiment | claim, analysis | maximum | `bearing × the lifter's strength` |
| `refutes` | claim, analysis, experiment | claim, analysis | cap | `1 − bearing × refuter strength`; the smallest cap wins |

A claim of rigor `0.85` with a claim premise at `0.4 × 0.90`, a work at `0.8 × 0.90`, a lift at
`0.7 × 0.90` and a refutation at `0.5 × 0.90` derives `min(0.85, 0.36, 0.72) = 0.36`, the lift takes
the final to `0.63`, and the smallest cap, `0.55`, takes it to `0.55`. `ComputeStrength` walks each
claim into `Strength.Results` and each analysis into `Strength.AnalysisResults`;
`EdgeContribution` is one edge's value in the table's Contribution column, nil where its bearing or
its source's strength is pending. `MinDependencyStrength` (the `[= min(…)]` trace's operand), the
largest lift and the smallest cap each fold it over their station's edges, omitting an excluded
one, and the `trace` query (SPEC §8) reports it per edge, `internal/query` reading each source's
strength off `Derive`'s node rows and leaving a planned experiment's edge out as the walk does;
`RenderStrengthTrace` composes the station the walk took.
[STRENGTH_SCORING_SCHEME.md](PROJECT_DOCS/SPEC/scoring/strength/entry-point.md) states that arithmetic — the relations, the
formula and the worked example — and is cited, not reproduced.

### The query layer

`query.EdgesPayload` is the one incidence adapter: it selects the index's edge rows by relation and,
where `EdgesSelection` names a node, by `Direction` (`in` the rows whose target it is, `out` those whose
source it is, else both), and presents each as `query.EdgeRowRecord` — `source`, `target`, `relation`,
`bearing` (the index's number-or-literal text, null only where the pairing carries none), `context`,
`manually_scored` and `replaced_relation` — in the fixed relation order, then by source and target
(SPEC §8). Its answer is the whole selection: no list query slices an answer, and `find` and `trace`
cut only where a `--limit` stands beside a `--sort`. No query verb returns scoring text: the index
carries no statement body, so no passage ever becomes a query answer.
### Frontmatter reading

`internal/kb` takes a leading `---` block as frontmatter only when its
first non-empty line is key-shaped (`name:`, kebab-case); a block that then fails to parse as a YAML
mapping is the malformed-frontmatter refusal, and a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10). The one frontmatter value kbase reads is the
superseded stamp in the entry point (`kb.FormatStamp`), read only where `kb.FormatFile` does not
stand (§10); `verify`'s `frontmatter` check uses the same recogniser
(`kb.FindFrontmatter`) to report a block on any document, the entry point included, and the build's
conformance check (`claimgraph`) refuses any document that carries frontmatter at all.

### Sheet

`internal/sheet` emits Graphviz DOT and renders it by running `dot -Tsvg` on `PATH`; the
SVG's bytes are Graphviz's, so the goldens pin the DOT, and a test that renders through `dot` skips
without it and asserts only that the DOT parses and the root fits the viewer. The goldens are a
fixture and its three DOT files under `internal/sheet/testdata/adjagent-sheet/`: a `3.0.0` KB and
the DOT the sheet composes for it, with a build record beside it that the sheet does not read. The
emitted DOT matches them byte for byte. Its inputs are the derived rows (nodes, edges, strength
bands, cites)
`index`'s `planSheet` hands it from `index.Derive`, never a read of `.index/` — the `demoted` rows for
the cuts, one style per relation, and each node's `CanonicalPath` and `CanonicalAnchor` for its
link; the leaf, for a node's kind, with the title deciding where no marker is readable; the
entry point's first heading for the title and the cluster of nodes no volume hosts; and each
volume's `index.md` first heading, falling back to the directory name, for the volume's cluster
title. It reads no build record and imports no kbase package but `kb`. It produces the full sheet
always, and the volume digest and one sheet per volume where two or more
volumes hold nodes. Drawn content and styling are SPEC §3. It renders through at most four `dot`
processes at once (`sheet.renderConcurrency`, §12), keeping output order. Where `dot`'s spline
router fails on a dense sheet, the emitter composes the same DOT with straight-line splines and it
is rendered once more; where both fail, the first error is reported. Without `dot`, it renders
nothing, leaves existing sheets as they are, and yields the placeholder for a root that has no
sheet.

Every re-derivation — `index.UpdateFormat`, every caller of `index.Refresh` (`init`, `score`'s
refreshes, the build's gate) and a write batch's `index.PlanDerivation` — reaches it through one
seam, `index`'s `planSheet`, which renders every sheet on each call and yields one only where its
bytes change, with no ownership test on an existing sheet; a refresh writes what it yields, a batch
writes it with its records. `verify` never reaches it.

### Scratch creation

`cmd/init.go` is the only producer of a KB that is not the build. It composes
existing mechanisms and owns no new one: the format stamp through `kb.EncodeFormatStamp` and the
plain entry point,
the two empty record files through
`kb.EncodeNodes` and `kb.EncodeEdges`, every file through
`atomicfile.Write`, the loader's `Open` (I8), the readiness stamp through `kbdocs.Stamp` with
`kbdocs.AuthoredKBPin` in its scope-pin slot — its `written` list is the paths `Stamp` returns —
the derived layer through `index.RefreshReporting`, the lock through `write.LockKB`, and the birth
commit through `gitledger.Commit` — the plain-subject half of `gitledger` extracted beside `Record`, its
pathspec scoping the same so nothing the user staged rides along. Its order is load-bearing: the
entry point and the stamp file stand before `Open`, the stamps stand before the refresh, and the refresh 
stands before the commit. It runs no `verify` of its own: every input to the skeleton is kbase's,
so a red verify over it is a kbase defect, which `test-integration-init` catches by running 
`verify` right after birth.

### Scoring

`kbase score` is SPEC §8's; this states the mechanism. Six packages and the verb carry it:
`internal/score` (the run's entry, its scope and the dirty check, the runner, the closed text and
its passages, the parse, the run's ledger and captures, run retention, the landing, the
development replay), `internal/kb` (the fault vocabulary, the three weight tables and the price,
and the statement reader `kb.Statement`), `internal/asks` (the three templates, the check that they
name their class's ids, and the judged hash, `asks.JudgedHash`), `internal/query` (the set),
`internal/write` (the edit ops a landing calls, the KB write lock it takes, and the hash every
value set records), `internal/statedir` (the state directory) and `cmd/score.go`.

#### The runner

`score.Runner` owns the loop and no I/O of its own. It takes the enumerated items, a
`Launch` seam, which hands each item the `Ask` its one call goes through, a `Write` seam, the ledger, the model name the lines record and a
logger — and, because the text is composed in that package, the `kb.Source` and `query.Index` the
renderers read, and the `graph` they read built once from them (`graphOf`), which `Invoke` hands
over where it built it for the scope. The pass is read once before the walk into its `class`es; per item one derivation of
the item's class yields the instruction its one call carries and the table its charges are priced
under, and the composer, the parse's closed set and the price all read that one fact. Per item it
composes the closed text, asks, parses the reply as the charges it records against that text, prices
them into the item's result cell and writes its result file; when the walk ends the run lands the result cells' values and appends their captures and lines; a text the guard refuses, a reply the parse refuses, or a call or
write that fails appends `failed` with nothing written, and the loop continues. `prepare` returns,
beside the entry, the `call` the item will make — class, closed text and prompt, nil where the class cannot be derived or the guard refuses the text — and `attempt` returns the entry and whether the walk's stop interrupted the call;
the walk's end writes every capture before every line (below). Every item's prompt
is composed in the walk's launch loop, which launches the items in the set's order through the run's
`Launch` seam, at most N at once; each answer is held in its item's result cell and written in the
set's order when the walk ends. `Counts` is computed from the ledger's last status
per item, not this invocation's attempts, so a resumed run reports the whole run. A cancelled context
stops the walk and records nothing for an item without an answer.
#### The ask is one seam

Each item's `Ask` takes the rendered prompt whole and returns a
`score.Reply` — the reply's text and whether the endpoint cut it off at the completion cap — so
the runner has no view of a provider: `internal/score` imports `asks`, `kb`, `query`, `index`,
`write`, `result`, `atomicfile`, `buildstore` and `log`, and never `model` or `build`. A truncated
reply is the runner's to judge, not the seam's: `put` fails its item as it fails a call that
errored, whatever the text it reached would parse to. `scoringLaunch` in
`cmd/score.go` fills the `Launch` seam: it opens the scoring role's client through the registry — so a key-file refusal comes after the start refresh and before `score.Invoke`'s index load — builds the invocation's one `model.Queue`, and hands each item an `Ask` that makes one `Slot.Chat` call on the role's endpoint with an empty system prompt, the item's `--timeout` applied from the call's start, reporting `model.FinishLength` as the truncation; the runner learns nothing of what answered. The development replay takes its launch from the same function, so a replayed call carries the run's endpoint, timeout and the truncation the runner judges.
Each score template splices the `score-system` fragment into its own text, so the instruction is
carried once, in the user turn.

#### The prompt shelf

Three templates, one per class — `score-rigor-statement.tmpl.md`,
`score-rigor-method.tmpl.md` and `score-bearing.tmpl.md` — each with the one `dyn.` slot the runner
fills with the assembled text, and three shared fragments in `fragmentSlots`:
`fragments/score-system.tmpl.md`, the system prompt, and `score-answer-occurrences` and
`score-answer-close`, the answer section's text all three state alike, so no vocabulary can drift from
the parse or from its siblings. `asks.ScoreRigorStatement`, `asks.ScoreRigorMethod` and
`asks.ScoreBearing` render them through `asks.Render`; `asks.System` serves the system fragments alone
(`systemSlots`), never an answer fragment as a prompt. Provenance is the shelf's rule: embedded in the
binary, written by kbase, no upstream origin and no version pin to drift. The templates carry the
criteria and the line form; they carry no id the class does not accept, and no figure or word a model
could read as a judgement — no decimal in `[0,1]`, no percentage, no weight, no maximum, no band's
word — while a quantity of the subject's own, an example's rate or threshold, is the claim's and
stays: the price is `kb`'s and one fact lives once. A test asserts each template declares one `dyn.`
slot, fills it with the text the runner composed, splices the shared fragments, names exactly its
class's ids (`flt-[a-z0-9-]+` tokens against `kb.RigorStatement.IDs()`, `kb.RigorMethod.IDs()` or
`kb.BearingFaults.IDs()`), carries none of those judgement spellings, and holds the instruction fixed
around a subject of any size; the shelf's `asks.Lint` covers every file as it does every template.

#### The closed text and its guard

`internal/score/text.go`'s two unexported renderers assemble every
character a call may see. `rigorText` renders a node's title and its statement: the statement is
`kb.Statement`'s reading of the node in the leaf hosting it — the one reader `add-citation`'s quote
check reads too — which the run's `graph` holds for every hosted node, read once per leaf by
`graphOf` with the `lifts` edges of the loaded index, markers and `>` prefixes removed, with the
1-based line of the leaf it opens on (`statementLine`: the line nearest the node's marker whose
text, markers and `>` prefixes removed, begins with the statement's first line). A node no leaf
hosts — a work, an axiom, a node with no marker — renders its title alone; a work's record's
`rationale` is never sent. `bearingText` renders the source's and the target's statements by that
same rule, then the edge's authored `context` from the row `Index.Edge(ScoringEdge)` answers. Each
end's label opens a line of its own, and so does the context's, so no numbered line begins with
frame text. `assemble` wraps every passage at word boundaries to `wrapWidth` — 100 columns —
keeping the passage's own line breaks and rewriting nothing, and numbers the lines lying in a
passage with an `end`, from 1 over the whole text; `closedText.numbered` writes `N| ` before a
numbered line, which is the frame's gutter and no part of the carried text, and carries a label
line, a blank separator line and the edge's context bare — a line with no number is no site.
`scoreTokenRE` and `frameRE` scan each assembled passage — no guard pattern spans the blank line
between two — and a hit is `scoreTokenError`, naming the document and line, with the item `failed`
and no call made: the guard refuses, it never strips. The four scoring tokens are half of what it
refuses; `frameRE` adds the frame's own structure — a run of twelve backticks closes the fence the
templates draw around the corpus, and what follows then sits outside the instruction to judge it,
and a line opening `## Source:`, `## Target:` or `## Context` forges the frame's map of the text; a
label a blockquote or an indent displaces from the line's start opens nothing. Neither half is
adaptive to the text: a rendered prompt stays a function of its template, so a text carrying the
frame's structure is refused rather than escaped. An out-of-scope subject is refused by name and
type — a `wrk-` or `axm-` id on `rigorText`, a `references` or `demoted` relation on `bearingText`.
#### The parse resolves the numbered lines

`assemble` returns the closed text and a line per line of
it — the number the call writes, the passage its bytes lie in, and its byte range — because a charge's
`passage`, `lines` and `offset` are facts about that rendering: a `rigor` call's two passages are a
node's `title` and `statement`, a `bearing` call's are the `source`'s and the `target`'s.
`score.ParseAnswer` (`internal/score/answer.go`) reads a reply against them and the item's class's
`kb.FaultTable`: a charge line's field is one number or a range `N-M`, which `closedText.look`
resolves — every number one the call shows, all of them in one passage, and nothing between them — so
a range crossing from one passage to another, or over a blank line, a label or a heading, is
`malformed`. The recorded `citation` is the cited lines as the call carried them, without the gutter,
joined by newlines, and `offset` is that quote's byte offset in the closed text. The edge's
`## Context` — assembled but carrying no number — is a site for no charge; a numbered line lies in one
passage, so `both` is no longer produced, and a line written earlier may carry it and keeps the
meaning it was written with. The read yields `Answer{Faults, Moot, Value}`: the charges, the charge
lines a reply recorded beside a retired `flt-pending` (parsed, priced nowhere, and no longer produced),
and the price `kb` gives the ids. The parse also refuses a charge
naming no line, an id no table holds — `flt-pending` among them — and a number the call does not show.
Duplicate and
overlap collapsing is the parse's, over cited line sets: an identical set written twice collapses to
one charge, and two sets of one id that overlap without coinciding are refused. The ids the ledger
records are the whole input of the price, which is what makes a landing checkable and a run
re-scorable without a model. The context stays in the assembled text because the ask judges an edge
on it; the parse merely refuses to anchor a charge there.

#### The scoring set

`query.ScoringSet(ix, pass)` reads the loaded index in-package and returns
`[]query.Item`, each carrying a `ScoringNode` or a `ScoringEdge` and never both, in the walk's order;
`query.PassRigor`, `query.PassBearing` and `query.PassAll` are the three spellings the set and the
`--pass` flag share (`query.Passes`), `PassAll` the node set and then the edge set, each in its own
order. An item carries identity only, so an edge's row comes from `Index.Edge` and no caller builds
a `kb.EdgeRow` by hand. The set has no verb: it is consumed in-process, and `--dry-run` is the
inspection surface. Which nodes and edges are in it, and which are excluded, is SPEC §8's.

`internal/score/scope.go` narrows it to a run's scope. `scopeOf` is the one named item —
`namedNode` and `namedEdge` refusing an id or an edge that does not stand, one no pass asks, and,
through `namedItem`, a manually scored item without `--rescore-manual` — else each item of the
pass that `dirty` says is dirty or, under `--rescore-manual`, is manually scored. `dirty` compares
the hash each record carries (`recordedHashes`, read once through `kb.LoadGraph`) with
`judgedHash`, the hash of what the item would be judged against now: `asks.NodeJudgedHash` over a
node's statement, `asks.EdgeJudgedHash` over its two ends', each statement the one the run's
`graph` read through `kb.Statement`. A resume takes no scope: its set is the run's `items.jsonl`
(`readItems`). `ResumeTarget` picks the run a `--resume` continues — the newest `run-` directory by
its header's start (`newestRun`) — and refuses where none stands, where `ReadRun` counts that run
finished, and where a build invocation in the store's progress record (`buildstore.ReadState`)
started after it.

The skip comes from the KB the run loaded too: `Runner.Skip`, `score.ManualSkip(ix, rescore)` over
the run's `--rescore-manual` and each item's `manually-scored` flag (`query.Index.ManuallyScored`),
says which items the walk records `skipped` — status `score.StatusSkipped`, reason
`manually-scored`, no call and no capture — and `score.Resume` takes the same predicate, so
`Ledger.reprice` leaves a skipped item's landed line as it stands. The walk also records `skipped`,
once, an item that is no longer in the index it loaded (`inKB`), and `attempt.fail` records a
landing's `errItemGone` value the same way, each with that error's text as the reason.

Two predicates in `internal/kb` answer whether an edge carries a bearing, and they are not the same
question. `kb.Scoring` — the four relations in `kb.ScoringRelations` — is whether an edge carries a
bearing at all, and every reader of a stored bearing uses it: the values' relation check, the strength
walk, the records, `edges`, a node's payload, the index's load and the bearing text. `kb.BearingWalk` —
the three in `kb.BearingWalkRelations` — is whether a score run asks for that bearing, and the bearing
pass is its only reader: a `founds` edge carries a bearing and is never an item, its source lying
outside the corpus, so `edit-edge` alone writes it.
#### The ledger and the state directory

`score.Ledger` opens `<state-dir>/score/<run>/`, writes
`run.yaml` through `atomicfile.Write`, appends the walk's lines in one write, with `O_APPEND` and
`Sync`, when the walk ends, and reads back the last entry per item — its status and the evidence behind it — so `--resume`
skips what landed and a failed run names each failed item's reason. A line is appended after the item's
write and validation, never before: a crash between the two re-asks a write that is idempotent, where
the reverse would skip a value that never landed. `Create` never reuses a run directory: the run id
is minted past any that stands, and a directory that appears under it fails the call. `Resume`
refuses a `run.yaml` naming another KB root, format or pass, and drops a tail torn after its last
newline so the item it names is asked again, then folds the result files a stopped invocation left —
landed through the run's landing, recorded, removed; one whose attempt the ledger already records is
removed alone — before it re-prices. `items.jsonl` holds the set the run enumerated — one
pending line per item, written when the run opens its ledger, so status and a waiter count the whole
run rather than the attempts made so far; it is all a dry run writes beside `run.yaml`.

A line carries `faults` — the parsed charges, `id`, `passage`, `lines`, `citation`, `offset`, in the
order written — `moot`, the charge lines a reply recorded beside a retired `flt-pending`, parsed the
same way and priced nowhere, and `before`, the value its item carried in the index the run read
before the attempt wrote: a before/after pair on the line itself, absent where the record held no
value. `[]` is written for one reply alone, the word `none`, so an empty list always carries the
model's positive statement that it found no fault. A failed attempt carries neither charge list, and no text:
its `reply` is the reason, and its `before` stands all the same, and its `remedy` is the one the
call's failure named — a provider refusing the call's key (§7) — absent otherwise, and its optional `note` is the refused-key note
that goes with it, which a failure item appends after the bounded reason so the bound never cuts it;
the fields are optional, so a ledger written before them reads under the same format, and the run's
failure items and `status`'s carry them from the line. `backend` sits on every
line, because a resumed or mixed run's lines must say what answered each item. `pass` sits on every
line beside it, naming its own item's class: the header states the set the run walks, and an `all`
run's lines say which class each judgement came from. The header also records `rescore_manual`
where the run re-judges manually scored values, which a resume continues with. `Create` also stamps
`ask_sha256`, from the header's `Pass`, as it stamps the format — one digest over every instruction
the run can send, each rendered for an empty subject and concatenated in a fixed order, so a rigor
run's two variants are covered by the one field and an update to either moves it, and an `all` run's
carries the statement, the method and the bearing renders in that order. It is stamped at
one site, so no caller can open a run without the ask its judgements answer, and a render that fails
is a defect of the build rather than a run written with no digest. `Resume` reads the header's digest
and compares nothing against it: the digest is a record of which instruction answered which judgement,
and an update applies its behaviour. A header written before the rigor pair carries one render's
digest, and is read as the one ask that run began under. `reprice` (below) is `Resume`'s other half.

#### The captures

The walk's end appends every capture to `captures.jsonl` in one write, beside the
ledger, through `appendLine`, which takes the `O_APPEND` and `Sync` the ledger's own lines take. A capture is written
for each attempt that returns a `call`: `attempt` returns none where the class cannot be derived or
the guard refuses the text, and a call the context cancelled returns nothing, so none of the three
leaves a capture. the captures are written before the lines, and a capture that cannot
be written ends the run `failed` as a ledger line that cannot be written does. A line's keys, in
order: `run`, `pass`, the item's identity as the ledger line spells it (`identity`: `kind` and `id`,
or `source`, `target` and `relation`, the other's omitted), `class` (`statement`, `method` or
`bearing`, `class.name`), `text` and `lines` (the closed text whole, each line `{no, end, from, to}` as
`closedText` holds it), `prompt` (the bytes sent), `prompt_sha256`, `reply` (what the `Ask` seam
returned, `""` where the call errored or the reply was truncated), `error` (the call's, the parse's or the write's error text,
absent on a landing), `model`, `backend`, `started` and `ended`. The ledger is unchanged: its failed
line carries the reason and no reply text, and the capture is where the reply stands. `Resume` cuts
a torn last line from `captures.jsonl`, as from `ledger.jsonl`, through the one `trimTorn`, before
anything appends, so a crash mid-capture never joins two captures into one malformed line. A capture
is self-sufficient for a replay: `readLastCaptures` (in the replay's tagged `replay.go`; the raw read
is `ledger.go`'s untagged `readComplete`) returns, per item, its last capture with the closed text
rebuilt, reads the file to its last complete line so a run still appending is readable, and refuses
a line that names no class or whose line ranges fall outside its text. **The fidelity rule:**
`class.render` is the one site that renders a closed text into a prompt, shared by `compose` and the
replay, so rendering a capture's closed text through an unchanged template reproduces its
`prompt_sha256` byte for byte; `TestCapturesReRenderToTheirDigest` asserts it over captures the runner
wrote.

The state directory is `internal/statedir.Dir`'s — the override, else `$XDG_STATE_HOME/kbase/<key>`,
and never inside `kb-root/` — extracted from `internal/build/store.go` so that a run and a build
agree on what the state directory for a KB is; the build store's own format stamp is
`internal/buildstore`'s (§8). The run's locks are `build.Hold`'s (§8): the operation lock in the
repository's git directory, then the state store's holder lock at `<state-dir>`, its record naming
`kind: score` and the run id.

#### Run retention

`KeptRuns` (§12) runs of each kind stand under `score/`. A real run's id is minted
`run-<id>` and a dry run's `dry-<id>` (`kb.MintID`'s prefix, `RunKindReal` and `RunKindDry`), and a
directory's kind is its name's prefix (`runKind`), so the two are counted apart; a dry run an earlier kbase minted as `run-…` is
named, and counted, as a real run. `score.PruneRuns(dir, run)` keeps the run the invocation names —
created or resumed — and the newest `KeptRuns-1` other directories of its kind, ordered by the
`started` each `run.yaml` records, equal starts by name with the later name newer, and removes the
rest of that kind whole with `os.RemoveAll`, captures and replays with them, through
`buildstore.PruneByStart`. `buildstore.DirStarts` is the one per-directory read of that start and
`buildstore.CompareStarts` the one ordering, both shared with `NewestRun`,
so `status`'s newest run and the prune's agree; a directory whose `run.yaml` is absent or unreadable
yields none and stands. `score.Invoke` calls the prune once the run's ledger has opened — after `Create`
on the dry path, after `Create` or `Resume` on the real one — so the run it keeps stands, and an
invocation refused before then removes nothing; it runs under the holder lock, and a prune error ends
the invocation `failed`. The one writer under `score/` the holder lock does not exclude is a replay,
which takes no lock: a prune may remove the run a replay is walking, and the replay then fails
rather than writing into a removed directory (below). `runIDRE` accepts a `run-` id alone, so `--resume` of a
`dry-` id is refused by the id-shape check, whose detail names a real run's id; `status`, `wait` and
`RunByPID` find a run by its directory, whatever the prefix.

#### The one-op landing

`internal/score`'s `landValues` lands a run's answered values in one landing,
under one hold of the KB write lock taken through `write.LockKB`. Under that hold, before any
batch, it opens the KB afresh through the seam the verb supplies (`Invocation.Open`, `kbload.Open`
in `cmd/score.go`) and reads it through `query.Load` and `graphOf` (`graphAt`). A KB that does not
open or load fails every value with its error. A value whose node or edge is no longer in the KB
fails with `errItemGone` ("the node or edge is no longer in the KB; its value is discarded and a
resume does not ask it again"), which `attempt.fail` records as status `skipped`, not `failed`, so
`ReadRun` counts it skipped and `ResumeTarget` does not resume for it. A resume's walk records an
item of its run that is no longer in the index it loaded the same way, once (`Runner.Run`, `inKB`),
and asks nothing of it. A value whose `Landing.Judged` — the judged hash `prepare` recorded as its
call was composed, carried on the result file and the ledger line as `judged` — differs from
`judgedHash` over the text the KB now holds fails with `errTextChanged` ("its text was edited after
it was asked; the value is discarded and a resume asks it again"): a statement edited by hand while
no lock kept the edit out. Only the values left go into the batch, so the hash the edit op records
is always the hash of the text the value judged. It issues one mixed write batch
(`write.Batch`) through `write.Run`, an entry per value in the order given — `opOf` names the edit
op: `edit-claim`, `edit-analysis` or `edit-experiment` by the node's kind with `id` and `rigor`,
`edit-edge` with `source`, `target`, `relation` and `bearing` — under
`Provenance: write.ProvenanceMachine`, so a landed value is written `manually-scored: false`, and
with the derivation, so the records and the derived files are written together. A `done` or
`unchanged` batch lands. A refused batch writes nothing (SPEC §8): where the refusal names its
entries, those items fail and the batch is re-issued without them, until it lands or is empty;
where it names none, the batch's values are re-issued one at a time under the same hold, each
refusal then failing its own item. That case is a refusal the reopen did not foresee — a hand edit
landing between the reopen and the batch, or a check the write makes that a load does not. A
failed value's error is its details joined and spelled for the command line (`valueError`), so no
ledger line carries a marker. A batch that fails, as against being refused, fails every item
it carried. A lock that cannot be taken fails every item too: one held past the wait fails each
with the detail of `write.HeldLockItem`, the item a verb answers `retry` with, and any other lock
failure fails each with the lock's own error. The run goes on. `landValues` returns one error per
value, nil where it landed. The landing probes for no operation lock: it writes as the run
holding that lock, so no run can be holding the KB out.

Each value the batch sets records its judged hash in `internal/write` (`judged.go`): an entry
that sets a rigor or a bearing names the record through `valueSet` — a caller's value, `*pending*`
included, and a run's number; a run's `*pending*`, which the build writes for `score` to judge, is
no judgement and names none — and once the entry has applied, `hashValuesSet` gives each named
record `asks.NodeJudgedHash` or `asks.EdgeJudgedHash` over the statements the batch leaves, read
through `kb.Statement` from the host leaf, so a node an add marks in the same entry is hashed over
the statement its marker gives it.
#### The re-price

`Resume` takes the same `Write` seam a landing is opened with, and `reprice`
re-derives every landed line's value from the ids its line records, under the table the line's own
class holds — a run landed under one rigor table re-prices under the class table, which is intended.
Only a line that records `judged`, the hash its call was composed under, is re-priced: one written
before the key says nothing of what text its value judged, and stands as it is.
A value the running table gives differently is landed first through the landing's write batches,
under the KB lock as a landing holds it, and the line records it after, so no line claims a value
the KB did not take. A value the landing discards — its item's text was edited after it was asked
(`errTextChanged`), or its node or edge is no longer in the KB (`errItemGone`) — rewrites its line with
that reason through `attempt.fail`, as a walk's discarded landing does: `failed` for a changed item,
which the walk that follows asks again, and `skipped` for a removed one, which nothing asks again. Any other refused
write fails the resume with the ledger as it stood, and the next resume re-derives the same move,
the write being idempotent. Each moved line is
listed in `repriced.yaml` in the run's directory, written only where a value landed — the run, the time, and per line the item, the
value it held and the value it took — which is where a re-price is recorded, `run.yaml` being written
once. `Ledger.Repriced()` says how many values moved, so the verb's closing refresh can follow a
re-price as it follows a landing. No model is asked: the ids are the whole input of the price, and
the model never saw a weight.

#### The run's entry

`score.Invoke` runs one invocation over a KB its caller has opened, refreshed
and locked: `query.Load`, the run's `graph`, and the set — the scope (the scoring set), or a
resume's own `items.jsonl` under the pass and `rescore_manual` its `run.yaml` records; then either
the dry run's records and the listing of what it would ask and skip (`Report.Ask`, `Report.Skip`),
or the walk over the launch the verb built (`Invocation.Launch`), the ledger created or resumed,
the prune, the pid, `items.jsonl` and the walk, the values landed by the run's one landing when the
walk ends. It returns a `score.Report` — the outcome; whether the invocation reached the route its calls take — not a dry run, so `Launch` was handed to the walk (`Asked`); the
run and its ledger directory once the ledger stands with the pid and `items.jsonl` beside it, and on
a stop only where the walk stopped it; the counts and the items scored; the failures (one `check: error` item per item the
ledger records failed, detail its name and its line's reason) or the cancelled record (the walk's
first item with no attempt, `{}` where every one has one); and whether an item failed or was not
reached. `score.RunID` creates the runs directory and mints the id (`run-<id>`, or `dry-<id>` under
`--dry-run`) or takes the one `score.ResumeTarget` found, before the locks, because the holder
record names the run.
#### The verb

`runScore` is the body, and `cmd/score.go` keeps the role, the ask, the locks, the start refresh
and the resume call: `scoreUsage`'s checks of the scope arguments together and of `--edge`'s form;
the KB root; the state directory; for `--resume`, `score.ResumeTarget` and the pass and
`rescore_manual` its header records; the scoring role resolved before the KB is opened —
`scoringRole`, through `config.Resolve`, with `--scoring-model` supplying or overriding the model,
and skipped under `--dry-run` — so a setup refusal writes nothing and mints no run; `openKB` and the
migration record; `score.RunID`; the run's locks through `build.Hold`, before any record of the run
is written; one refresh through `holdWriting`, skipped under `--dry-run`, which releases the write
lock before the walk; then the launch, `scoringLaunch`, whose client the registry opens at the
first item launched, so a run refused at its scope or holding no item dials nothing, and where a
key-file refusal ends the run; then `score.Invoke`, given that launch; `roles` where the report's
`Asked` says the walk was handed it, then `run`, `ledger`, `counts`, a dry run's `ask` and `skip`,
and `scored`; `resume` (`scoreResume`) where an item failed or was not reached, ahead of the record
the outcome closes on; and last, the report's `failures` on `failed`, or its `cancelled` on
`cancelled`. The landings derive inside their batch, so no refresh follows the walk. The closing
records and `resume`'s reach are SPEC §8's. Registration is `readWriteTool(scoreCmd)` in
`cmd/surface.go` (SPEC §11): the command is bound and appears in `tools/list`, but the MCP server
starts it as a detached child (`mcp.ScoreLaunch`) — as it starts `build` — returning the run's
identity and a `wait` command instead of holding the turn for the walk's duration, a run still
starting after 5 s as `state: starting`. `--edge` is declared a mapping flag (`mcp.MappingFlag`):
over MCP an object of `source`, `target` and `relation`, joined into the `--edge` the child
takes.
#### The development replay

`kbase score --replay RUN` re-asks a run's captured subjects under the
binary's embedded templates and reports what the runtime would have done, so a change to a scoring
template is `edit → just build → replay` over the items that failed. SPEC states nothing of it.
Its code and tests are `cmd/score_replay.go` and `internal/score/replay.go` with their test files,
each carrying `//go:build kbasedev`. `cmd/score.go` reaches it only through the package-level
`scoreReplay` hook, which the tagged file's `init` sets and which is nil in the default build; that
`init` also declares `--replay`, `--failed` and `--limit` — a run takes no limit, so `--limit` is
the replay's alone — each annotated `kbase.mcp` = `excluded` through
`mcp.ExcludeFlag` (§12), so dev and release builds serve one tool table. The default build is the
release shape and excludes the replay's code; dev builds pass `-tags kbasedev`, as `just build`,
`just test` and `just test-race` do. What it reads is not dev-only: a run's ledger and captures are
written by `internal/score/ledger.go`, untagged, so a replay compiled into a dev build reads the
records a released kbase wrote. `edit-gate` vets both polarities, and `test-dist-no-replay` runs
`TestDistBuildHasNoReplay` (`cmd/dist_test.go`) on the untagged build, which asserts the three flags
absent and the hook nil, and the same of the build's development replay (§8).

`scoreReplayRun` takes `--pass`, `--failed`, `--limit`, `--scoring-model`, `--state-dir`, `--timeout`,
`--ask-concurrency` and `--config-dir` beside `--replay`. Any other score flag with it is a usage refusal naming `--replay`,
`--failed` or `--limit` without `--replay` one naming it, and a `--replay` value outside
`^(run|dry)-<id>$` (`replayRunRE`: the alphabet and length `kb.MintID` mints) one naming `--replay`. It opens no KB and
takes no KB, operation or holder lock; a run in progress may be replayed. With `--state-dir` it needs
no KB root and works from any directory; without it, it finds the store through the kb-root at or
above the working directory and is refused, `check: usage`, `key: --state-dir`, where there is none.
Both development replays, this one and the build's (§8), resolve their store through
`replayStateDir` in `cmd/replay.go`, beside `replayTakes`, which names the flags a replay takes in
its refusal. A relative `--state-dir` resolves against the project root where the working directory
has one, and against the working directory where it has none. `score.OpenReplay` selects
every item with a capture, in the order of the run's `items.jsonl`, each by its last capture;
`--pass rigor` or `--pass bearing` narrows the selection to that pass's calls — refused where the run
did not run it — and `--pass all` or no flag takes the run's whole list; `--failed` selects only
those the run's ledger last records failed, and `--limit N` takes
the first N. A run with no `captures.jsonl` — a dry run, one pruned, one that asked nothing — is
`refused`, `check: usage`, `key: --replay`. The scoring role is resolved by `scoringRole` and the
launch built by `scoringLaunch`, the functions `runScore` uses.

`Replay.Walk` renders each captured closed text through `class.render` — the binary's embedded
template for the capture's class — sends it through the `Ask` its launch hands the item, and reads the reply with `call.put`, the
runner's own parse against the captured closed text under the class's table, then prices it. It lands
nothing, and a per-item failure is counted, not the replay's outcome. While it walks it logs its
progress through the verb's logger, at info level, as the message `replay progress` with the keys
`done`, `failed`, `remaining` and `total`: at most once per `ReplayProgressInterval` (§12) and once more
when the walk ends, where `done` and `failed` are the result's counts. The counter and the one
goroutine that logs it are `buildstore.ReplayProgress`, which both development replays use; an item
only bumps the counter as its call returns — here an item whose status is `done` counts done, any
other answered item failed, and an interrupted one neither — and no item, and no progress line, writes a file
(I11). It writes only under
`score/<run>/replays/<replay-id>/`, each of its three files appended once, when the walk ends, the id minted `replay-<id>` as a run's is:

- `replay.yaml`: `replay`, `run`, `backend`, `provider`, `model`, `ask_sha256` (computed as `Create`
  computes it for the run's pass) and `started`.
- `ledger.jsonl`: one line per item asked, in the run ledger's shape, its `run` the replay id.
- `captures.jsonl`: the replay's own captures, in the captures' shape, `run` the replay id.
- `comparison.jsonl`: per item, `item` (`ItemName`), `before` (the value the run's line recorded
  before its ask), `original` and `replay` (each `status`, `value`
  and `faults`, the fault ids in order, `[]` for `none`) and `prompt_changed` (the replayed prompt's
  digest differs from the captured `prompt_sha256`).

Setup computes the digest and `replay.yaml` before creating anything, and creates `replays/` and the
replay's directory with `os.Mkdir`, never `MkdirAll`: a run the prune removed fails the replay rather
than being recreated without a `run.yaml`, which no prune would ever remove. After writing
`replay.yaml`, `Walk` keeps itself and the newest `KeptRuns-1` other replays of the run's `replays/`
by `replay.yaml`'s `started`, through `buildstore.PruneByStart`, which the run prune also uses, and after the walk a
replay whose directory no longer stands ends `failed` naming it. An item whose run records no ledger
line compares against an `original` with status `""` and is not counted in `changed`.

The result document is one SPEC §7 YAML document with the keys `outcome`, `state-dir`,
`run`, `replay`, `dir` (the replay directory, absolute), `counts` (`total`, `done`, `failed`) and
`changed` (an integer: the items whose status or value differs from the original's), then the closing
record the outcome names. The outcome is `done` when every selected item was asked, `cancelled` when
the context stopped the walk — its `cancelled` record naming the first item it did not ask — and
`failed` for a fault of the replay itself, an unreadable capture file or a store write.

---

## 7. Inference seams

- Kept from kbase: provider management (`internal/config`) and the client (`internal/model`). The
  call path is a `model.Slot`'s `Chat` for the overview call and for each scoring call, and its
  `Decision` for the multichoice asks. A `Slot` is issued only by the invocation's one `model.Queue`, from
  `Go`, and only while one of its N tokens is free. `build` and `score` build the queue from
  `--ask-concurrency`, else `[asks] concurrency`, else 4.
  `kbase probe` takes one call on one of the two by its selection, which the next item states; the decision role's model is an id on whichever surface its provider speaks
  (`decisionsFlavor`, SPEC §9.1), and kbase's three roles apply — multichoice asks go to the `decision`
  role, the overview call to `text`, and `kbase score`'s asks to `scoring` (SPEC §9). Each role
  names its own endpoint and its own model, in `[models]` as a `<provider>/<model>` reference; no
  role falls back to another.
- **`kbase probe` is a diagnostic on the same path**, the CLI command and the read-only `probe` tool
  alike (I9). `cmd/probe.go` resolves one selection through
  `config.ResolveFlag` — `--model` supplying a reference and naming itself in a refusal, a role flag
  (`--decision`, `--text`, `--scoring`) naming the role to resolve and supplying no model, so that
  role's model comes from the configuration and a refusal names its key alone — then opens
  the client through `internal/modelclient`'s registry exactly as `score` does, so the chat path speaks
  whichever chat protocol the provider's `api` names, and the registry is the only site that builds a
  wire client. One call, no backoff, bounded by `--timeout`, and the selection's own surface is
  what answers it: a `--decision` selection makes one `Slot.Decision` call on the surface its
  provider's `decisionsFlavor` names, every other selection the one `Slot.Chat` call; `--prompt`
  replaces either path's default check — `probeDecisionPrompt` on the decision one, `probePrompt`
  on chat — and a chat request to a decision selection is the mismatch a probe exists to find.
  The chat attempt's `FirstToken` — request start to the first non-empty content
  or reasoning delta, measured in `chatOnce`, zero where none arrived — is what the result reports
  beside the call's wall time; the decision surface streams nothing, so the document reports
  `first-token-ms` zero and `finish-reason` null. It opens no `kb.Source`, resolves no state
  directory and takes no lock: the call is one item on a queue of one (`model.NewQueue(1)`, no
  flag), and the decision path is `probeEndpoint` → `Registry.Get` → the item's `Slot.Decision` →
  the result document, the chat path the same through `Slot.Chat` (SPEC §8). A chat selection's
  client is the registry's for its role, so the call carries that role's sampling — the `scoring`
  role's for `--model` — and the document reports the effective settings, `model.SamplingOf(client)`; where the call ends
  `truncated-empty`, the chat path reports `ChatResult.Final` as the answer rather than failing. The
  call's deadline and its `wall-ms` clock start inside the item, so neither counts a wait for a token.
- Every chat call is one tool-less chat request answered from its prompt alone. The transport keeps
  at least N idle connections per host, N being the invocation's concurrency, so its connections are
  reused rather than redialled (`model.Endpoint.IdleConns`, set by `modelclient`'s registry). `internal/model`
  holds two implementations of that request, and the registry builds the one the endpoint provider's
  `api` selects — for the `decision` role always the OpenAI one, which alone carries the decision
  surfaces: `model.OpenAIClient` (`openai_client.go`) speaks the OpenAI-compatible
  `chat/completions` surface, and `model.AnthropicClient` (`anthropic_client.go`) speaks the
  Anthropic Messages surface — `POST <baseUrl>/messages`, with its own event sequence, its
  `x-api-key`/`anthropic-version` headers and its catalogue shape — and is what a provider silent on
  `api` gets.
  `model.NewClient` is the dispatch, and the protocol reaches it as a value the caller resolved onto
  `model.Endpoint` from `config.Endpoint.Protocol`, which the configuration fills from the provider.
  The body each client sends is its protocol's, never one shaped by
  host or provider name. `model.Sampling` is the one sampling type: each client resolves
  `model.Endpoint.Sampling` once, at construction, with `Sampling.Resolved()` — temperature 0 and
  thinking off where the role states none — and `chat()` copies it onto every request, shaping no
  field of either protocol. Each protocol's wire rules live only in its body encoder
  (`encodeRequest`, `encodeAnthropicRequest`). `model.SamplingOf` returns a copy of the resolved
  sampling — the effective settings — so no caller can alter what a client sends.
  On the OpenAI surface: the role's temperature; thinking off as the provider's own
  `reasoning: {enabled: false}` control with `chat_template_kwargs.enable_thinking: false` beside
  it, and a level as `reasoning_effort: <level>`, `reasoning: {effort: <level>}` and
  `chat_template_kwargs.enable_thinking: true` at once, since a server reads whichever control it
  speaks; the usage chunk requested; and no completion cap, the server's own applying. On the
  Messages surface: no `reasoning`, `reasoning_effort`, `chat_template_kwargs` or
  `prompt_cache_key` field exists, so a run's cache key has no home; thinking off is sent as the
  protocol's own `thinking: {type: disabled}`, the field's absence leaving it at the server's
  default, and a level as `thinking: {type: enabled, budget_tokens: <budget>}` (§12); the role's
  temperature is sent whatever the thinking level, a server that rejects the pair failing the call
  like any request it refuses; the system turn is lifted to the top-level `system` string; and the
  protocol's required `max_tokens` is the thinking budget, where thinking is on, plus the
  answer's allowance, `max(256, ceil(characters / 4))` over the ask's messages and system text.
  A call that does not complete is re-issued identically up to the attempt count, pausing where its
  caller's backoff says (§12), and a pause the call's context cannot cover is not waited out — the
  call gives up at once, its reason naming the attempts it made; a provider's non-2xx status ends it
  on the attempt that met it unless
  the status is a 429 or a 5xx (§12); an attempt cut off at the completion cap before any answer
  text ends the call on that attempt; both wire clients report a non-2xx as the same
  `model.StatusError`, so that classification is shared rather than re-decided per protocol. A
  scoring run's clients are built with the run's id, which the OpenAI client stamps on every
  request it sends as
  `prompt_cache_key` — one key per run, so its asks reach one upstream and share one cache entry; a
  verb that names no run builds its clients with none, and
  the field is then omitted from the body rather than sent empty. Each
  attempt's outcome names what ended it — `ok`, `served-nothing`, `truncated-empty` where the answer reached the completion cap before any
  answer text, `provider-refused` where the
  provider answered with a status that is not success, `transport-failure` where no answer of the
  provider's was had — and records whether the call reasoned, from a reasoning delta on the stream
  or from a usage block reporting reasoning tokens spent, so a call that thought without streaming
  the thought is recorded as having reasoned; the error a call returns carries the last attempt's
  outcome and its text, which is what a failed item's reason records (SPEC §8), and wraps that
  attempt's `model.StatusError` where it met one.
  `model` makes it, with a system message. This holds for the overview call; the multichoice asks take
  the decision surface instead (next item). A build with a calling row left to walk checks a
  provider is configured before its first stage (SPEC §5).
- **A refused key is classified where `model` is imported, and travels outward as data.**
  `StatusError.RefusedKey` (401 or 403) is the one predicate. `asks/call`, `cmd/score.go`'s ask,
  `cmd/probe.go` and `modelclient` (`KeyRefused`, which `Catalogue` applies) match the call's
  `StatusError` with `errors.As` and attach `config.AuthAddRemedy`, the name the dialled provider
  carries filled in, to what travels out: `asks.IncompleteError.Remedy`, which `claimgraph`'s
  inference stop and the build's overview failure carry onto their items; `score.RemedyError`,
  whose remedy and `config.RefusedKeyNote` the attempt's ledger line records; and `modelclient.KeyRefusedError`, which
  `cmd`'s `failed` reads onto the failure item of `probe`, `models` and `configure-models`. `asks/call`
  is told the provider it dials by `call.New`, from `build.Provider`'s per-role provider names. No site
  recovers a status from a reason's text, and nothing checks a key before the call (SPEC §9).
- Configuration is read from layers and merged once, before a verb's own work: `config.Layers`
  resolves the directories and `config.Load` reads and merges the two files of each, returning one
  `Loaded` — the pool and its faults, and the choices. They are
  `<repository>/.kbase/` over `~/.config/kbase`, or the one directory `--config-dir` /
  `$KBASE_CONFIG_DIR` names — `explicit` and `environment` in the source vocabulary — so an
  explicit choice isolates a run from a repository's own `.kbase/`, which every test and pinned
  deployment needs. The caller resolves the repository root with `kb.RepositoryRoot(workDir)`, which
  answers `""` outside a worktree, so `internal/config` takes no git dependency and reads the global
  layer alone there. `parseProviders` folds each table name to lowercase as it reads it, after
  refusing a file two of whose names differ only by case (SPEC §9.1), so the pool, its faults and
  every output name a provider in lowercase, and `config.ParseReference` and `config.Select` fold
  the names they are given the same way. A provider's table is replaced whole by a higher
  layer's of the same name; a `config.toml` field is overlaid one at a time. `config.Load` drops a
  fault naming a provider a higher layer already supplied — the pool's provider is the higher one, so
  the fault describes a value nothing reads — and keeps a higher layer's own fault, whose provider
  never entered the pool. `config.Layer` carries a directory, its source (`explicit`, `environment`,
  `repository`, `global`) and — once `config.Report` has probed it — whether the directory and each
  of the two files stand, `ConfigExists` beside `ProvidersExists` (`show-config`'s
  `config-exists` and `providers-exists`).
  `Provider.Layer` and `Config.ModelLayer` record the layer the merge took each provider and each
  role's value from; `ProviderFault.Path` names the `providers.toml` a fault came from, and a
  `Refusal`'s path names the file a lacking setting belongs in — the write target's
  `providers.toml` or `config.toml` — so
  a layered configuration says which file to edit. The write target is `config.WriteTarget`'s —
  the directory `--config-dir` or `$KBASE_CONFIG_DIR` names, else the global one — which is also
  where the configure commands write with no location flag (`cmd`'s `configureLocation`), so a
  refusal of a name the pool lacks, whose remedy is the `kbase configure-providers` call that adds
  it, names the file that call writes. `configure-models` checks against the stack it writes into
  instead — the layer written and the layers beneath it, `config.WrittenStack`, loaded by `cmd`'s
  `loadWrittenStack`, which `configure-providers --remove` also loads to name what still refers to
  the provider — so its refusal names the written layer's `providers.toml`, and its remedy carries
  the location flag the call carried. Both configure commands' `RunE` resolve the layers
  (`resolveVerbLayers`) without loading them, so no layer outside the stack written is read.
- **Keys: the store is read in `cmd`, the precedence lives in `config`.** `cmd`'s
  `loadConfigContext` reads every file of the credentials store through `internal/auth`
  (`readStoredKeys`) before the load, and hands `config.Load` a `config.StoredKeys` as data — each
  key that opened, by the lowercase name its file is stored under, and a `config.StoreFault` per
  file that did not, built from the `*auth.FileError`'s path and its error apart; a store directory
  that stands and does not list is one nameless `StoreFault` against the directory, with no keys,
  and the load goes on. `config` imports
  no `auth`. `keyKind` is the precedence's one home: the key stored under the provider's name, whatever
  layer supplied its table; else the table's `apiKeyFile`, which `LoadProviders` then reads, and
  which a provider with a stored key never reads, so it raises neither a read fault nor a permission
  warning; else none. `config.Load` turns each store fault whose name a configured provider carries
  into a warning `ProviderFault` naming that provider, `Source` `SourceStoreFile`, its
  `Path` the store file and its `DoesNotOpen` the store fault's, so every configuration-loading
  verb's `warnFaults` writes it; a file no provider names is read by nothing and not reported. The
  nameless directory fault names no provider and becomes a warning `ProviderFault` with `Source`
  `SourceStoreDir`, its `Path` the directory, which `warnFaults` writes as the credentials store's.
  A `ProviderFault`'s `Reason` never repeats its `Path`; `LocatedReason` leads a store fault's
  reason with its path, for a line that stands on its own — `warnFaults`' and a provider's `fault`
  in `show-config`. `config.Report` reads the faults alone, and reports a store fault's
  diagnostic as its path and reason with
  `config.AuthAddRemedy` — the piped form, also `kbase auth add`'s own input-refusal remedy and a
  refused key's failure remedy — as the
  remedy where the file does not open, and for a provider dropped because its `apiKeyFile` could not
  be read (`ProviderFault.KeyFileUnread`), the stored key outranking the file; it alone computes a
  `file` key's `upgrade` and the warning that a stored key leaves an `apiKeyFile` unread, so
  `show-config` alone emits them.
- **Resolution is pure, and connection is the registry's.** `config.Resolve`
  (`internal/config/resolve.go`) answers one role's `Endpoint` from the provider its reference
  names; for `text` and `scoring` it carries `Temperature` and `Thinking` from `[sampling.<role>]`
  with the layer of each, and refuses a temperature outside the protocol's range; it reads only the
  loaded configuration — no file, no connection, no stderr — though a refusal's remedy may name the
  command that fixes it (`kbase configure-models`). `config.ResolveFlag` is the same resolution with the flag that supplies a
  role's model spelled by the caller, so `probe`'s `--model` is named in a refusal where
  `--scoring-model` would be; the caller names no flag where none supplies a model, as `probe`'s role
  flags supply none, and a role the files leave unnamed then names only its key. `config.Select` is
  `models`' choice of
  a provider to dial — a
  flag, else the only provider — and no role is chosen that way. A configuration
  refusal is a `result.Refusal` whose items carry their check — `provider` (`config.CheckProvider`)
  unless it names its own, as `config.CheckUsage` — with several items for a decision that has more
  than one side to state; `internal/build`'s launch refusal reads the key and path of a one-item
  refusal. An item's remedy is its clearing text alone; `cmd`'s `refused` composes
  `config.ShowConfigRemedy` over every `provider` item a verb refuses with, so
  `show-config`, which refuses with none, reports its role diagnostics with the clearing text
  alone; the build's preflight refusal, which leaves through `buildstore`, composes it where it is
  built. No other site applies a selection or a
  resolution rule, and `internal/modelclient`'s `Registry` is the only constructor of a
  `model.Client`: one client per endpoint, wire protocol and resolved sampling — roles whose
  resolved sampling is equal, an unset setting equal to its stated default, share one client and its
  connection pool — opened on first use, keyed by the provider's name, by the protocol and by the
  resolved sampling, dialled with the key the provider resolved to at load, and closed together
  before the verb returns on every outcome. `samplingOf` maps the configuration's `thinking` onto
  `model.Sampling`.
  The report's other facts — the pool's providers and faults, each provider's key kind and upgrade, the
  diagnostics, the files' presence, `config.toml`'s values — come from `internal/config.Report`,
  which reads no file of its own: the caller passes the existence probe.
- A multichoice ask is one `POST` to the provider's decision surface (a `Slot`'s `Decision`; `decisionsFlavor`,
  SPEC §9.1), one `noul` or `choice` question asked and the answer read by the key it echoes — no
  reply text parsed. Two shapes stand behind that choice. `systemone`, what a provider silent on the
  flavour gets: `model`, `state`, and `questions` a JSON object keyed by the question key, each
  question's `criteria` carrying the per-letter meanings the templates' bullet lists used to carry;
  the `answers` object comes back keyed the same way. `decisions`: the state as `input`, `questions`
  an array of named questions — a `noul` question asked as a `predicate`, carrying no choices, its
  true/false guidance in `instructions`; a `choice` question carrying its criteria as `choices` —
  answered by an `answers` array, each entry naming the question it answers; a `refusal` carries no
  value and so takes the seam's re-ask-then-default path. Under either surface an answer naming a
  question kbase did not ask is refused. The request is re-issued identically up to the chat attempt
  count on a transport error or retry-class 5xx, with no streaming and no incomplete-stream path.
  Both surfaces are OpenAI-shaped, so the registry gives the `decision` role an `OpenAIClient`
  whatever its provider's `api`: a provider whose `api` is `anthropic` keeps its Messages client for chat
  and gains an OpenAI one, on the same base URL and key, for decisions. A Messages-only server has no
  decision surface, and the call fails with that server's status. The system-prompt sentence above
  is scoped to the overview call; a decision call has no system message.
- Each inference pass is built as plumbing reaching the model only through `internal/asks`' multichoice-ask
  seam, injected — nothing in `claimgraph` knows a network exists. `internal/asks` holds the embedded
  templates and fragments, the composer, the caller (system fragment, per-call captures — a chat
  capture's request record carrying the effective settings, `temperature` and `thinking`, the
  values `model.SamplingOf` reports; a decision capture's question carrying its `true-letter` where it has one, and the overview
  capture's request its `excerpts` and, on a retry, its `rejected-lines` — the inputs
  `asks.OverviewPrompt` renders — and the answer cache under the state store's `scratch/`, each written once, after the item that made the
  call has released its token, the answers into the current build trail's directory), and the multichoice-ask seam: every claim-graph
  inference is one decision answered by one letter from a closed set the build offers, an unreadable
  reply asked once more and then defaulted to the item's mechanical draft, recorded as defaulted.
  Each kind composes its state through `MultichoiceItem.Compose`, its kind's template rendered from
  the item's template inputs, and `asks.RecordedItems` builds items from a group record through the
  same constructor. A batch that stops still returns its records — an item whose call never
  completed recorded `failed`, one cancelled or never launched absent — as, after a stop, is a
  group with no asked item; a finished batch records every group it was given — and the stage writes them
  before it stops; a batch made by `asks.NewReplayBatch` records such an item and carries on.
  Templates (`*.tmpl.md`) and fragments are embedded files (I7). A template change is an experiment
  with a measured result.
- Prompts are `@!slot!@` templates; a call's system prompt is the fragment named for it
  (`overview-system` — the one surviving system fragment, the overview call's), never an agent
  definition; a multichoice ask carries no system message. `asks.Render` builds the one user turn. No test
  asserts a template's prose; tests key on the offered letters, the slot set, the system fragment and
  the closing line's letters.
- `claimgraph` reaches a model only through the multichoice-ask seam injected by `build`; the caller that
  holds `model` lives in `internal/asks/call`, imported by `build` alone, and the import-policy test
  checks the transitive graph so `claimgraph` never reaches `model`.
- No prompt originates in an in-code string.
- The overview passage is written from excerpts composed mechanically from the tree, checked for
  structure and re-asked once through the
  `overview-correction` fragment; a call ending `truncated-empty` fails the stage without that re-ask.

---

## 8. Ledger, state store and monitoring

`internal/gitledger` is the only package that execs git (I1); it execs the host's git, which SPEC §2
already requires, so the user's configuration, hooks and attributes apply, and no git library is a
dependency. `internal/build` owns the walk, the state store's writers, resume, `--no-inference`,
`--through`, progress and cancel; `internal/buildstore` owns the stage vocabulary, the store's
reading, status and the trail's reading. Which packages and `cmd` files import `gitledger` is
[§11's dependency rules](#dependency-rules).

- **Fresh versus resume** is read from the commit trail alone (SPEC §6); nothing in the state store
  is authoritative for position.
- **Inputs.** `build.Run`, once it has found the repository, makes a fresh build's volume roots and
  `--bibliography` files absolute against the repository root and puts them in path order before
  anything reads them: the launch checks, `walk.body`, the document graph and the records'
  regeneration. Every boundary records them, and `start` also records `started-at`, the HEAD before
  it — on a branch with no commit, an empty `kb-start:` commit made for it; the line keys and their
  readers are `internal/buildstore`'s. `--resume` takes the inputs from the newest trail entry in
  the order recorded, never re-sorted. `gitledger.Repo.Trail` stops at `Repo.Removal`, the newest
  commit off the trail that took `kb-root/` out of the tree, so a trail a committed removal ends is
  read by nothing. A resume reads the newest build's trail (`buildstore.CurrentTrail`: back to the
  newest `start` boundary), so a fresh build over a removed `kb-root/` starts a trail of its own; a
  fresh build reads none. `run` refuses, as soon as it reads the trail, a `--resume` with nothing to
  resume, `kb-root/` absent among it (SPEC §5), and `prepare` a fresh build over any `kb-root/`
  that exists, with `buildstore.KBRootExists`, the refusal `init` gives. A fresh build's dirt check
  drops the paths under the absent `kb-root/`, which its `start` boundary commits. `walk.result`
  hands out no `resume` where `kb-root/` does not exist when the run ends.
- **The document graph's tree** is written by `docgraph.Build`, then every leaf rewritten with each
  link pandoc wraps around a cross-reference unwrapped to that cross-reference
  (`build.unwrapTree`), in the build and in its records' regeneration alike.
- **Resume over an interrupted stage.** A stage writes its build records and its `kb-root/` work once
  its asks have all returned, so a stop during the asks leaves those paths as the boundary left them,
  and a kill during that write leaves them partial; either way it is uncommitted. A resume restores those paths to the last stage commit and
  re-runs the stage whole; the answer cache (`scratch/trails/<start-commit>/answers/`) is what carries the paid-for
  inference across, so the re-run asks nothing it already holds; it mints fresh ids, and nothing
  outside the restored paths referenced the old ones. A classify prompt names claim ids, but those
  were minted in stages already committed and the restore reaches back only to the last boundary, so
  a resumed run's prompts are identical and the cache answers them; a fresh build's are not, and it
  pays again. Each build record an inference stage writes is written once, after its asks; resume reads
  none of them for position. The progress record accounts for the dirt an interrupted
  stage leaves; the
  dirty-path refusal (SPEC §5) applies to dirt it cannot account for. The resume command is recorded as the argv
  `kbase build --resume` with every flag the launch carried but the bound, `--through`, and the inputs,
  and reaches a document as the build call each surface spells (below);
  `--config-dir` is written absolute: resuming past the bound is what resuming is for. It is emitted only where an identical re-issue continues the work: a `bounded` run is
  handed none, being finished at the bound — re-issuing its command is not what continues it;
  `build-trail` is what a reader uses to reset and resume. A refusal that
  recorded no boundary leaves nothing to continue. It is also emitted only where the
  recorded invocation reads as a `kbase build` invocation (SPEC §6): the state store is a directory
  the caller names, so a record naming any other program is repository-supplied text and no document
  renders it for a harness to run.
- **Records missing on resume.** The docgraph records live in the state store. A resume that finds
  them absent regenerates them by re-running the document-graph stage into scratch and requires its
  tree byte-identical to the committed `kb-root/` (SPEC §5, determinism); a difference is `failed`,
  naming the first differing path.
- **Two locks, both `internal/filelock` advisory locks, and both a run's — a build's or a scoring
  run's.** The *operation lock* is `<git dir>/kbase-operation.lock`, recording the state-dir; a run
  takes it after it opens and holds it for the whole run, and removes it on release. Because it is
  per repository, a second run through any `--state-dir` is `refused` (`check: lock`, `path` the
  state-dir, no remedy) with a detail naming the holder — "a build (pid 1) is running; its state-dir
  is …" or "a score run (pid 1) is running; …", both read from the store the lock names. A write op
  or `update-format` that finds it held is refused with the same item; a write verb
  probes for it before waiting on the write lock and again after taking it, and a scoring run's own
  landings and refreshes probe for it not at all: they write as the run that holds it. An empty
  lock file is the window between acquiring the lock and writing the state-dir: it reads as
  "starting", so a second run gets `retry` and a writer proceeds. A state-dir written before its
  store's holder record is the other window, and reads as "a run is starting". The build's order
  is: the operation lock; the state store's holder lock; its first progress event; and only
  then the wait for a write in progress, by taking and releasing the KB write lock (§6; the same
  wait, `retry` if it cannot). `status` therefore sees a waiting build, and a build that gives up
  waiting ends `retry`. The full order of a build is: refuse a newer store at open; the operation lock;
  the holder lock; ready the store (upgrade, stamp); the run event; prune the captured builds; wait out
  writers. Its own writes and its seed and gate refreshes take no write lock, so it cannot refuse
  itself. A scoring run takes the same two locks, in the same order, through `build.Hold`: the
  operation lock first, then the holder lock, before it writes the first record of its run or its
  start refresh. The *holder lock* is in the state store, its record carrying the holder's pid, its
  start, the verb that owns the run (`Kind`) and, for a scoring run, the run id; `status` reads
  liveness from it and the run it names, `wait` reads whether the run it waits for still holds it,
  and `cancel` signals its holder. Given no `--state-dir`, the monitors — `status`, `cancel`, `wait`,
  `build-trail` — resolve their store through `buildstore.MonitorOptions.Open`, which reads the
  store the operation lock names while its file stands and names one, else the default; the MCP
  launch keeps the default. A clean exit removes the file, cancel included, so a standing lock
  file is not by itself evidence of a live holder. A record naming no known `kind` is a store another
  version wrote: `buildstore.ReadHolder` refuses it (`check: state-dir`, `path` the record), and
  nothing reads it as a build's. A second builder on the same state-dir is `refused`; where only
  the holder lock is held the result is `retry`. User-facing text calls it "holds the state store";
  "operation lock" names only `<git dir>/kbase-operation.lock`. On Windows, where no advisory lock
  exists, `status` cannot see a running run.
- **State store version.** `<state-dir>/format` is one integer, `buildstore.StoreFormat` (§12),
  written when a build first holds the store; an absent file reads as 1. `build`, `status`, `wait`,
  `cancel` and the MCP build tool read it at open, and a store newer than `buildstore.StoreFormat`
  is refused (`check: state-dir`, `key` the marked `state-dir` flag, `path` the store, remedy "update
  kbase, or use another" and the same flag; SPEC §7 `key`). An older store is converted in place by the build, in "ready the store",
  through the chain `storeUpgrades` in `internal/build`: one step per version, each stamping
  `format` when it completes, so an interrupted upgrade resumes at the step it stopped at; a gap in
  the chain is an error. The chain is independent of the KB's `formats.Chain` (SPEC §10).
- **Retention.** At its start, after the run event, a build keeps the newest `keptBuilds`+1 (§12)
  captured builds under `reports/` — a capture being the `build-<UTC timestamp>-<random>` pair of
  `.stdout.yaml` and `.stderr.log`, ordered by name — and removes the rest; it reads no progress
  record to do so. Once its launch checks pass and before its first stage, a build prunes `scratch/trails/`,
  one directory per build trail named by the commit of the newest `start` entry of the trail the walk
  stands on, holding `answers/`, `asks/`, `captures/` and `replays/`: it keeps the current trail's
  directory — on a resume created if absent and stamped with the current time; a fresh build's
  directory is created by its first ask, so its time is when it last gained a subdirectory — and
  the newest `keptBuilds` (§12) others by modification time, and removes the rest
  whole, with the undivided `scratch/answers/`, `scratch/asks/` and `scratch/captures/` an earlier kbase wrote. At the
  same point, a fresh build whose `progress.jsonl` holds more than `progressRotateLines` (§12) lines
  moves every line before its own run event to `progress-<UTC timestamp>.jsonl`, leaves its run
  event alone in the record, and keeps the newest `keptProgressFiles` (§12) such files; nothing
  reads them. The stage reports are never pruned, and `status` and `cancel` remove nothing.
  `status` computes `cache-entries` and `cache-bytes` over every trail's `answers/`. A
  scoring run prunes `score/` once its run's ledger has opened: of its kind (`run-` or `dry-`) it
  keeps the run it names and the newest `KeptRuns-1` (§12) others by `run.yaml`'s start, and removes
  the rest whole (§6, Scoring). It keeps the newest `KeptRuns` (§12) captured runs under
  `reports/score-*`, by name, and removes the rest.
- **State store layout:** `format`, the holder lock, `progress.jsonl` and up to `keptProgressFiles` rotated
  `progress-<UTC timestamp>.jsonl` files, `records/` (the docgraph
  records and pre-pass census), `reports/<stage-id>.yaml` (each stage's row records and findings,
  named by the result's `stages[].report`; also the captures of a build the MCP server starts,
  `build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`), `scratch/` —
  `trails/<start-commit>/`, one per build trail, holding `answers/`
  (the answer cache, content-addressed by model, system prompt and prompt and the call's sampling
  settings where they are not kbase's default, written through
  `atomicfile`; an ask's attempt ordinal is part of the key so a re-ask reaches the model),
  `asks/` (the multichoice-ask group records, one file per stage, `<kind>.yaml`, one YAML document per
  group: the group's shared template inputs once, each item's own inputs, its `prompt-sha256`
  (`asks.AskDigest` of the state and the question), its outcome (`answered`, `re-asked`,
  `defaulted`, `failed`) and its calls; written by a stage whose batch stopped as well as by one that
  finished), `captures/` (every call's request, reply and usage) and `replays/<replay-id>/` (a
  development replay of the trail, below), `regenerate/` (the records regeneration on
  resume), `score/<run>/` (a scoring run's `run.yaml`, `items.jsonl`, `ledger.jsonl` and
  `captures.jsonl`, `results/` — one file per answered item, `<walk index>.json`, until the walk's
  end or the next resume folds it — and a development replay's `replays/<replay-id>/`; §6, Scoring).
- **The build's development replay.** `build.OpenReplay` and `Replay.Walk`
  (`internal/build/replay.go`, tagged `//go:build kbasedev` as §6's scoring replay is) re-ask one
  trail's recorded asks under the binary's embedded templates and question files, from the inputs
  the trail records (I10), so a prompt, a question file or a model can be refined against one build
  without re-running it. The trail is the one a hex prefix of its start commit names, or, given
  none, the trail directory with the newest modification time, listed by `listTrails`, the listing
  `pruneTrails` orders by. `OpenReplay` reads each kind's `asks/<kind>.yaml`
  (`asks.ReadGroupRecords`) and the overview captures (`call.ReadOverviewCaptures`) and selects in
  walk order — the multichoice kinds in `asks.Kinds` order, groups in file order and items in
  record order, then the overview calls by ask attempt — narrowed to one kind or to `overview`, to
  the items recorded `failed` or `defaulted` and the overview calls `failed` or `rejected`, and to
  the first N. It refuses, `check: usage`, `key: --replay`, a store with no trail directory and a
  prefix matching none or more than one (`path` `scratch/trails`), a trail recording no group
  record and no overview capture (`path` the trail), an empty selection (`path` the trail), and a selected record or capture carrying no
  inputs (`path` that file). The replay's support code is likewise tagged `//go:build kbasedev`,
  in `internal/asks/replay.go` and `internal/asks/call/overview_captures.go`. The replay takes no
  lock. Its items are rebuilt by
  `asks.RecordedItems` and asked as one `asks.NewReplayBatch` on the decision role, then each
  overview call as its own item on the text role, through `call.New` pointed at the replay's
  directory, every call on the invocation's queue; `MultichoiceItem.Compose` and
  `asks.OverviewPrompt` stay the only composition sites, the build's own. A call that never
  completes is a `failed` row and the walk carries on. It logs its progress as §6's scoring
  replay does, through `buildstore.ReplayProgress` and the verb's logger, its `total` the selected
  rows: the launch handed to `asks.NewReplayBatch` wraps each item's `MultichoiceReader`, and an
  item counts by its last call — `done` where it returned no error, `failed` where it returned an
  `asks.IncompleteError`, neither where its error stopped the batch — and each overview call
  likewise, so the last line's `done` and `failed` are `ReplayResult`'s. It writes only under
  `trails/<start-commit>/replays/<replay-id>/`, the id minted `replay-<id>`: `replay.yaml`
  (`replay`, `trail`, `decision_model`, `text_model`, `started`); its own `answers/` and
  `captures/`, in the trail's shapes; `asks/<kind>.yaml`, its own group records; and
  `comparison.jsonl`, one row per replayed item or overview call — `kind`, `group`, `item` (an
  overview row's `group` `""` and `item` `overview`), `attempt` (an overview row's alone),
  `original` and `replay` (each `outcome`, `choice` and `confidence`: a multichoice item's as its
  record holds them; an overview call's outcome `failed` where its capture's last attempt did not
  complete, else the overview stage's judgement of its reply, `answered` or `rejected`, and its
  choice and confidence null) and `prompt_changed` (the re-rendered prompt's digest differs from
  the recorded `prompt-sha256`). It never writes the trail's `answers/`, `asks/` or `captures/`,
  nor `kb-root/`. Setup creates `replays/`, the replay's directory and its subdirectories with
  `os.Mkdir`, never `MkdirAll`, so a trail the prune removed fails setup rather than standing
  again, and restores the trail directory's modification time, which is the current-trail marker
  and the prune's order. After writing `replay.yaml`, it keeps itself and the newest `KeptRuns-1`
  (§12) other replays of its trail's `replays/` by `replay.yaml`'s `started`, through
  `buildstore.PruneByStart`; pruning removes only entries of `replays/`, so the trail directory's
  time does not move. Once the walk drains, a replay whose `replay.yaml` no longer stands — a
  prune removed it, and its in-flight calls' own writes may have recreated parts of it — removes
  its directory — and its trail's `replays/` and the trail directory where they hold nothing
  else — and ends `failed`; otherwise it writes its group records and `comparison.jsonl`, on
  every exit after it asked anything. `ReplayResult` counts the selected rows (`total`), those
  replayed to an outcome other than `failed` (`done`) and those `failed`, and as `changed` the rows
  whose outcome or choice differs from the original's. Its outcome is `done` when every selected
  row was asked, `cancelled` when the context stopped the walk — naming the first row it did not
  ask — and `failed` for a fault of the replay itself: a store write, a compose error, or the
  replay directory removed under the walk.

  The verb is `kbase build --replay [<start-commit>]`, in `cmd/build_replay.go`, reached from
  `cmd/build.go` only through the `buildReplay` hook that file's `init` sets; the tag, the hook
  pattern, `mcp.ExcludeFlag` and `test-dist-no-replay` are as for score's replay (§6). The `init`
  declares `--replay` (a boolean), `--kind`, `--failed` and `--limit`, each excluded from MCP.
  Beside `--replay` it takes `--kind` (`paragraph`, `unmarked`, `colinear`, `classify` or
  `overview`), `--failed`, `--limit N`, `--state-dir`, `--ask-concurrency` and `--config-dir`; the
  store resolves as §6 states for both replays. Each refusal is `check: usage`: `--kind`,
  `--failed` or `--limit` without `--replay`, naming that flag; any other of build's own flags
  beside `--replay`, read from `LocalNonPersistentFlags`, naming `--replay`; more than one
  positional, or one not matching `^[0-9a-f]+$`, naming `--replay`; a `--kind` outside the five,
  naming `--kind`; a negative `--limit`, naming `--limit`; and the store's and `OpenReplay`'s
  refusals above. Models and sampling come from configuration through the build's own Provider
  seam and queue (`buildModels`, `cmd/build.go`): the decision role answers the multichoice asks
  and the text role the overview calls. The result document, verb `build`, carries `outcome`,
  `state-dir`, `trail` (the start commit), `replay`, `dir` (the replay directory, absolute),
  `counts` (`total`, `done`, `failed`) and `changed`, then the closing record its outcome names, a
  `cancelled` record holding the `kind`, `group` and `item` of the first row not asked.
- **Progress units** are leaves for the node pass, source groups for `references-found`, the ask
  groups of each of its two passes for `claims-culled` (each pass announcing its own total), ask
  groups for classification; a defaulted item is a `fallback` event. A stage's `units` event is written
  before its asks and its unit and `fallback` events in one append after them, so `status` moves
  between ask batches; every build record is written once per stage, after its asks.
- **Trail order.** `run` refuses a trail recording a stage past one it leaves unrecorded in
  `Stages` order (SPEC §5, `check: trail`), right after reading which stages it records and before
  the launch checks and any restore: walking the gap would run a stage over products of stages
  meant to follow it.
- **A stale `.git/index.lock`** left by a kill during a boundary commit is a refusal naming the
  lock.
- **`--state-dir`** defaults to the §12 key under `$XDG_STATE_HOME/kbase/`.
- **Under `--no-inference`** the overview call is the dropped row and no `README.md` is written.

- **Ledger.** Commit subject, tracked records and pathspec scoping are SPEC §6; resume position is
  read from the trail alone. The
  four tracked build records — node-pass, unmarked, classification and cull — are
  `claimgraph`'s to read and write, as YAML `kb-build-*.yaml`
  (SPEC §6, §10). `build` commits them.
- **State store.** Contents and path are SPEC §6; the key form is §12; the layout is above.
- **Status, progress, cancel, wait.** `status` reads the holder lock and its record,
  `progress.jsonl` and the commit trail and returns the YAML snapshot in SPEC §6; a record naming
  `score` is reported in the score run's own shape, read from the run's ledger and result document;
  that reading is `internal/score`'s (`score.Status`), which the verb renders. A build's `stages`
  lists the stages done and not done in the newest build's trail and names `build-trail` for their
  commits; `buildstore.BuildTrail` reads the same trail, each boundary's command its
  `git reset --keep` then the build resumed with the flags its last run was launched with: the
  resume argv the run the store stands at recorded, through `RecordedCommand` as `status` reads it
  (the marked `build` call with `resume` alone where no run recorded one that reads as a kbase
  build invocation), and `started-at`'s the reset alone.
  Where no build run holds the store and `kb-root/` is absent, or the trail above a removal of it is
  empty, both read no trail and `status` no run (`buildstore.noBuild`). `score.Status` also counts the run's result files while it walks. Where no live run holds the store and its record names no score run,
  `status` reports the store's newest run, comparing the start the newest run directory's `run.yaml`
  records (`score.NewestRun`) with the newest build invocation's in `progress.jsonl`
  (`buildstore.ReadState`), and reports the build where neither stands or the two do not order.
  `cancel` signals the holder the record names, a build's or a scoring run's, and waits for the
  holder lock to go through the one `build.StopHolder` — `cmd` choosing only which run's resume the
  document carries; the holder abandons
  the calls in flight with nothing written for them, logs the event, releases the locks and exits
  `cancelled`. `wait` polls `buildstore.ReadState` through `buildstore.WaitState` — the store
  alone — until the run it names is superseded or has ended, then renders
  `status`'s document through the same
  `buildstore.Status`: it takes no lock and reads no commit trail until that final document. A score
  run's pid is waited for through `score.RunByPID` and the store's holder record instead
  (`score.WaitRun`), so `wait` returns the run's own result document.
- **A recorded document is spelled when it is emitted again** (SPEC §7 `key`, `remedy`). The progress
  record's items and a score run's result document beside its ledger hold the marked form
  (`internal/argmark`): each was written on a writer that spells nothing. `status` and `cancel`
  emit them again through `result.Emit` on the writer of the call reading
  them, so a recorded item and the recorded `Resume` argv — `buildstore.RecordedCommand`'s
  `argmark.Recorded` call, read with `build`'s own flags into an arguments mapping where it is spelled —
  reach each surface in its spelling, and `wait` re-emits a score run's document through
  `result.Reemit`. The store's layout is unchanged: a marker sits inside the string fields, and
  `Resume` stays an argv. A launched run is told its document is read over MCP by
  `mcp.SurfaceEnv` in its environment, set by `Launch.start`; `cmd/main.go` reads it before
  `Execute` and spells the process's stdout for MCP, so the run's own document, which the launch
  hands back where the run exits first, is the tool's.

## 9. Result emitter

`internal/result` owns the YAML result document and its emitter, and nothing else writes stdout
(I5). Fixed key order, strings quoted on emit, a refusal enumerating every offending item.
`internal/result` maps `outcome` to exit code per SPEC §7 (`ExitCode`) and to whether the outcome is
an error (`IsError`): `cmd` exits with the first, and an MCP tool result's `isError` is the second.
`Emit` closes a `retry` or `failed` document's items with the outcome's ask, ending each `detail`
(SPEC §7); `RetryAsk` and `FailedAsk` are the one home of both sentences, and a caller hands `Emit`
its items without them. YAML uses the package named in CONVENTIONS "Dependency Policy".

## 10. Compatibility harness

Compatibility is judged by kbase's own readers and checks, never by a byte diff of presentation. The
hermetic gate is `just test-integration`, whose nine members — `test-integration-results-fixture`,
`test-integration-init`, `test-integration-opscript-results`, `test-integration-mcp`,
`test-integration-config`, `test-integration-auth`, `test-integration-parity-adjagent-sheet`,
`test-integration-refused-write-adjagent-sheet` and `test-dist-no-replay` — each judge what their
process produced: the first eight run `./bin/kbase` over fixtures they stage, judging the result
document against the verb's keys, the tree it wrote, the transcript it served, the streams and files a credentials command left, or — for the parity
recipe — the frozen answers the 2.0.1 binary gave over the KB the conversion reads, each moved only
by a rule naming a designed outcome of the conversion; `test-dist-no-replay` judges the default
build's command tree for the compiled-out replay. Each of the first eight runs the script of its own name
under `tools/scripts/`, and each writes its log under `test-data/transient/<recipe>/`. The recipes
the rule in CONVENTIONS.md "Testing" keeps out of the gate run by name —
`test-integration-mcp-build`, the MCP notification probe that builds a paper, among them.

---

## 11. Package map

The listing groups packages by role; the dependency rules below it are the whole rule, and the
import graph they allow is acyclic.

```
cmd/                      cobra root; one file per subcommand; surface.go, the one
                          registration file
internal/argmark          the one marker by which text names a tool argument (`Flag`,
                          `Positional`, `FlagValue` with its value) or holds a kbase
                          call (`Call`, an arguments mapping, lists of mappings
                          among its values; `Recorded`, a recorded
                          argv), and `Spelling.Spell`, each surface's one spelling:
                          `--name`, `<name>`, `--name value` at the command line,
                          `name`, `name: value` over MCP, a call through the
                          `Spelling.Call` its caller supplies; a marker inside other
                          text is backticked where its spelling is undelimited (every
                          one over MCP, a call at the command line), one that is the
                          whole text bare; imports no kbase package, and `log`
                          imports it to spell every line it writes for the command
                          line
internal/result           YAML result document + emitter (I5); the outcome-to-exit table;
                          a document's strings spelled by the writer it goes to
                          (`Spelled`, which spells any other text written on it
                          too: the root's stdout is one for every subcommand but
                          `mcp`, so what cobra prints itself — completion
                          descriptions, summaries, flag usages — is spelled; any
                          other writer keeps the marked form, a record's,
                          re-emitted spelled by `Reemit`), but a `Verbatim` — the
                          KB text a result passes through (a title, a rationale, a
                          context, a leaf's line, a conversion list's id and text),
                          written as it stands on every surface — and
                          `Items.Error` in the command line's spelling
internal/latex/pandoc     reader, writer and defaults-run seam (I1)
internal/latex/prepass    the two source scans (I3)
internal/docgraph         JSON transforms, whole-volume render+cut, outline,
                          records, partition checks, validate_build
internal/records          the record schema (§5.5), written by docgraph, read by claimgraph
internal/kb               kb-root model: names, exclusions, link primitives, dead-link
                          gate, the format stamp, its file and the frontmatter
                          recogniser, schema (the band ladder, spelled nowhere else;
                          the node kinds, the relation names, `Relation*` and
                          `Relations`, defined in `format400` and re-exported here
                          under the same names, as is the rest of the live format),
                          the record loader `LoadGraph` (the record files read
                          through `Source`, decoded by `format400`, a rigor above
                          `RigorMaximum` refused), the scoring faults' vocabulary
                          and the three weight tables (`faults.go`), `Source` (the one
                          read path), the derived row types `index.Derive` yields
                          (`indexstream.go`), the statement-marker reader
                          `MarkerLines` and the hosts it derives (`Hosts`), the
                          document kind (`DocumentKind`, from the child list),
                          `LabelLine` (the label-line recogniser), the child-list
                          reading of the tree relation (`ChildList`,
                          `ChildListLines`), the heading reader `FirstHeading`, the
                          reference-list entry reader `BibliographyEntries` and
                          the citation-key reader `CitationKeyRefs`, the
                          statement marker's one spelling (`StatementMarker`,
                          `StatementMarkerOpener`, `StatementMarkerRE`, re-exported
                          from `format400`), the heading anchors (`HeadingAnchors`
                          over `format400`'s `Slugify` and `Slugger`, re-exported),
                          the link grammar every link reader reads through
                          (`LinkRE`, `Links`, `Citation`, `BareIDs`, re-exported
                          from `format400`), the register pages' rule — the file
                          name, title and entry headings (`RegisterFile`,
                          `SummaryTitle`, `SummaryEntryHeadings`), the volumes
                          (`Volumes`) and a host's page (`SummaryPage`) —
                          re-exported from `format400`, which index and
                          claimgraph read it through
internal/kbload           `Open(kbRoot)`: reads the stamp, builds `kb.Source` (disk at
                          the current version, `formats.Chain` in memory for an older
                          one); the only importer of the `formats` root; imports `buildrecords`
                          for the root paths an older KB keeps the records at
                          (`buildrecords.RootRecordFile`); imported by `cmd`,
                          `build` and `write` only
internal/index            `Derive` (the derived rows in memory), the cycle load error
                          and the cycle walk (`BreakCycles`), strength,
                          the register pages, each node's page and
                          entry anchor there (`Rows.RegisterPages`,
                          `Rows.RegisterAnchors`), and their freshness,
                          the re-derivation (`RefreshReporting`), update-format's
                          save (`UpdateFormat`) and the pending-conversion refusal
                          (`PendingConversion`), the bare-id check,
                          the claim-id link check, the nodes a section's citation
                          can cite (`SectionNodes`), the path finder
                          (`DependsGraph.Path`)
internal/write            the write batch (`Run`: shape, apply in memory, the
                          derivation through `index.PlanDerivation`, then every
                          changed file written, or none under a dry run; §6) and
                          render / values / store / ops over the records and the
                          statement markers, provenance, the judged hash each value
                          set records (`judged.go`, through `asks`), the build-only
                          `add-build-edges`, `demote-claim-edge` and `split-claim`
internal/atomicfile       the one atomic file writer; `WriteMode` sets the file's
                          permission exactly, umask and a replaced file's mode aside
internal/query            the queries over `Derive`'s rows (SPEC §8): `find`'s
                          selection, sort and limit (`Find`, `FindUsage`; the
                          sort's checks and order in `rankUsage` and
                          `compareRanks`), `show`'s record — its host and section anchor
                          from the host leaf's marker, read once per leaf at
                          `Load`, its summary entry from `RegisterPages` and
                          `RegisterAnchors` — and `stats`, whose ranked sections
                          are `find` questions (`HighestLeverage`, `Weakest`);
                          `edges`' rows (`EdgeRowRecord`) and fixed order
                          (`compareEdgeRows`), and `trace`'s walk (`Trace`),
                          its contribution `index.EdgeContribution`'s
internal/sheet            DOT emitter, `dot` invocation: `Derive`'s rows, the leaves,
                          the entry point's and volumes' headings in; SVGs out
internal/export           the LaTeX export (§5.7): `export.Run`, the export's one
                          sequence, over unexported steps — the walk, the inputs of
                          `gen-tex/`, the embedded filter and template, the starter
                          `preamble.tex`, the report
internal/buildrecords     the four build records' readers (node-pass, unmarked,
                          classification, cull) and their paths under `kb-root/.build/`
                          (`Dir`, `RecordFiles`) and at the repository root, where a
                          KB before 4.0.0 keeps them (`RootRecordFile`); imported by
                          claimgraph, by kbdocs for whether a build made the KB, and
                          by kbload and `cmd/update-format.go` for the root paths the
                          conversion moves
internal/kbdocs           readiness stamp, the maintained blocks' fences and their
                          rewrite (`Maintain`), README assembly, the session rule
                          (`SessionRule`) and the `.index/` rule (`IndexRule`)
internal/claimgraph       conform, inventory (records + page-defined readings),
                          identify, attribute, shortlist, unmarked, order, cull,
                          split, elision (the cull's issuer), equation sites, classify,
                          equation(s), endcap, hand_named, assemble, write, gate,
                          the four build records
internal/asks             embedded templates + fragments, composer, caller
                          (captures, answer cache), multichoice-ask seam; the scoring
                          templates' line form, and the test that asserts the ids
                          each names against `kb`'s pass set; the judged hash's one
                          computation (`JudgedHash`; `NodeJudgedHash` and
                          `EdgeJudgedHash` name the template an item is judged under),
                          which score's dirty check and write's value sets both call
internal/gitledger        git checkpoint seam (I1), by exec
internal/filelock         the one advisory file lock (the KB write lock, the operation
                          lock in the git directory, the state store's holder lock)
internal/build            the walk and its steps' behaviour, the state store's writers,
                          resume, --no-inference, --through, progress and cancel,
                          and the development replay (`kbasedev`)
internal/buildstore       the build's state, read: the store's layout, holder record and
                          progress record, the stage vocabulary and the step table's
                          shape, status and the reading (holder lock, progress record,
                          the run it stands at) `wait` polls (`WaitState`), the commit
                          trail's reading, the newest build's (`CurrentTrail`) and
                          `build-trail`'s document of it (`BuildTrail`), the kb-root
                          refusal `init` and a fresh build share (`KBRootExists`),
                          and the progress logging both development replays share
                          (`ReplayProgress`, `kbasedev`); reaches no `model` or
                          `asks/call`
internal/score            the score passes' closed text, its guard and the passages it is
                          made of, a run's scope and the dirty check over the judged hash
                          (`scope.go`), the parse, the runner, the run's entry (`Invoke`) and
                          its landing through `write`, the run's ledger, its captures,
                          its re-pricing and the retention of runs, and the development
                          replay (`kbasedev`); imports not `model`; imported by cmd and
                          internal/mcp alone
internal/statedir         the state base (`Base`: $XDG_STATE_HOME/kbase,
                          $XDG_STATE_HOME defaulting to ~/.local/state where unset
                          or not an absolute path), and the
                          state directory of one KB: the override, else <key>
                          under the base, never inside kb-root/; imported by build,
                          buildstore and cmd
internal/auth             the credentials store: one file per provider,
                          `<provider>-key.enc`, the name lowercased, in the directory
                          its caller names, the key sealed by AES-GCM under a per-user
                          key HKDF derives from a constant in the source and the
                          username — obfuscation, not a boundary, separation between
                          users being the file's mode; the provider-name rule, stated
                          again from config's (`auth.NamePattern`,
                          `config.ProviderNamePattern`) and pinned equal by a cmd test;
                          imported by cmd alone
internal/secret           `Key`, the type every key travels in: fmt's verbs, a text,
                          JSON or YAML encoding and a log value render it
                          `<redacted>`, and `Reveal` alone returns the key; imports
                          no kbase package
internal/asks/call        the caller: the multichoice asks through a Slot's Decision behind the multichoice-ask
                          seam, the overview call through a Slot's Chat; captures, answer
                          cache; imported by build alone
internal/formats          (SPEC §10) a tree of packages, one per format and one per
                          converter; no file I/O. The root: `Chain`, the `converters`
                          list and the version arithmetic (`ParseVersion`,
                          `CompareMinor`, and `Chain` taking every patch of a minor
                          to that minor's converter), imported from outside the
                          tree by `kbload` alone.
                          `common`: what two or more of its superseded formats and
                          converters use and no format defines (`Files`, and
                          `Warning`, whose list, node id and text a conversion
                          result groups its dropped fields' warnings by).
                          `format<major><minor><patch>`: one format's definition —
                          stamp, field names, token patterns, vocabulary, record
                          shapes, its reader where one exists — defined once there
                          (`format200` covers every 2.0 patch). A format package may
                          change until a kbase release ships its version, and is
                          frozen after in what it accepts and what a conversion
                          produces. `format400` is the live format's: the record
                          types, field names, the judged hash, id grammar, node
                          kinds, relations, the statement marker, the heading
                          slugging, the link and citation grammar and the summary
                          pages' names, title, entry headings and volume rule
                          (`citation.go`) and the record files' encoder and decoder
                          over bytes; it imports no kbase package, `common` included,
                          and `kb` alone imports it from outside the tree. `format300`
                          is frozen, read by `from300to400`.
                          `from<from>to<to>`: one N→N+1 converter over the loose
                          representation, returning the next form plus the paths it
                          made obsolete
internal/mcp/protocol     (SPEC §11) JSON-RPC framing, dispatch, the Tool, Schema and
                          Property types and the frame checks; it declares no subscribe
                          capability and answers no subscribe method; imports `log`
                          only. The tool table, the resources and the server version
                          are injected by internal/mcp through Serve(in, out, version,
                          instructions, tools, resources, lg)
internal/mcp              the MCP exposure of the command tree: the `kbase.mcp`,
                          `kbase.mcp.family` and `kbase.readOnly` annotations, the
                          help notes' `kbase.help.mcp` and `kbase.help.cli`
                          annotations and the three help renderings (SPEC §11)
                          composed from a command's `Short`, `Long` and notes —
                          a tool's description, its help resource, and
                          `CLIHelp`, the `--help` text the root's help function
                          prints, the usage line, `Long`, the generated argument
                          list and the CLI note — the list (`ArgumentList`) built
                          from the definitions a tool's schema reads: a command's
                          positionals and flags (`commandArguments`), a values
                          op's `write.OpVocabulary`, the batch's `batchSchema`,
                          the root's persistent and own flags, each surface
                          typing a list and a mapping its own way; the CLI note
                          (`CLINote`) closed by the flags excluded from MCP and,
                          below the root, a line naming the global flags — each
                          rendering, and each schema description, spelling the
                          markers for its surface through `mcp.Spelling`, which
                          renders a call from the tree's annotations (`spelling.go`:
                          a bound command's tool call over MCP, `write`'s against
                          the batch schema; `CommandLine`'s command line at the
                          command line, where `write` and each write op take every
                          argument but their own flags as the values document,
                          piped in as `printf '<yaml>' | kbase <op>`), and each tool's
                          result spelled over MCP and checked for an item keyed
                          `--name` or `<name>` of its own arguments (`warnCLIKeys`,
                          a warning only) — the
                          tool table derived from the bound commands and their
                          families, a call run as its command's RunE, a family
                          tool's dispatch to its operation and the check of its
                          arguments against that operation's own schema, the
                          `write` tool, `kbase write`'s, whose arguments are the
                          batch and whose family's members, the write ops, have
                          no tool of their own, the
                          project a call runs in, the resources (each family's
                          help, `kbase://help/<family>`, and each bound
                          command's help, `kbase://help/<command>`, among them),
                          and the detached build and score launches, each reporting a
                          run still starting after 5 s (`launch.go`, the one self-exec
                          owner, I1), and the
                          instructions `initialize` reports; imported by cmd alone
internal/modelclient      the client registry (`Registry`, one `model.Client` per
                          endpoint, wire protocol and sampling settings) and the
                          mapping of configuration's `api`, flavour and sampling
                          onto `model`'s protocol, decision surface and sampling
                          (`chatProtocol`, `decisionsSurface`, `samplingOf`);
                          imports `model`, `config`, `result` and
                          `log`; imported by cmd alone
kept: config, model (the `Queue` and its `Slot`, the only exported entry to inference), log,
      version
```

### Loading

Every KB file is read through `kb.Source`, built by `kbload.Open` (I8), so an older KB
arrives already in the current form and no verb sees the on-disk bytes of an unmigrated one. The
loader carries the conversion record: `Source.Migration()` holds the chain's `From`, `To`, the one
sentence `Note` (`kb.MigrationNote`) and the conversion's `Warnings`, set where the source was
migrated and kept across the save that stamps the current format, so every call on an older KB
until that save carries the `migration` record and writes its stderr lines — the sentence, then one
per warning (`cmd`'s `migrationNotice`; SPEC §7, §10). `update-format` alone saves a migrated
source (`index.UpdateFormat`); every other writer — `write.Run`, `score`'s verb before its start
refresh, the build's resume (`loadFormat`) — refuses it through `index.PendingConversion` before it
writes, so no write ever lands on an unconverted tree (SPEC §10). `update-format`'s result lists the
warnings whose `List` is `ids`, `bare-ids`, `no-edge` or `strengthen-by` under keys of those names and
the rest under `migration`. `build` is the one loader that carries it
nowhere: a mid-build KB is not a finished state, and its stage commits own that trail.
`kb.MigratedSource` carries the paths the conversion made obsolete, and `update-format`'s save
removes them.

**The strict load** is `kb.LoadGraph`'s, and it sits at the load of every command that builds a
state: `format400.DecodeNodes` and `DecodeEdges` read both record files and return every
`format400.GraphFault{Check, Path, Line, Key, Allowed, Remedy, Detail}` — a file that does not
parse, an unknown or missing field (a scored record's `judged-hash` among them), a wrong type or a
value outside its domain, an id outside its
kind's grammar, a node id twice, a pairing `BearingLegal` refuses — before any state exists, so no
page is rendered and no op runs; an absent record file is a fault of its own (`check: index`, remedy
"restore it from git"). The faults arrive as one `kb.GraphLoadError`, which `index`'s refusal path
(`graphItems`, `internal/index/derive.go`) maps to `result` items — `check: metadata`, the field as
`key`, the file and line — and every reader of a
state, `write` included, refuses with them.

**The integrity faults refuse too** (SPEC §10, citing
[page §2.4](NEW_MCP_CLI_DESIGN/2-conventions/2.4-load-errors.md)); there is no degraded
state. Where both files decode clean, `kb.LoadGraph`'s `edgeIntegrity` adds a fault per edge record
naming a node no record carries and per second record of one edge identity, each `check: metadata`
with `kb.ConsistentRecordsRemedy`. A cycle among the scoring edges is `index`'s: `deriveGraph` runs
`BreakCycles` on every load, and `Derive` refuses each edge the walk cut as a `check: metadata` item
naming it. The cycle stays out of `kb.LoadGraph` because the build's ring row reads, through it, the
records its own edge batch left, cycle included. `index.DeriveOverCycles` is the one read that takes
the cut records instead of refusing: `write` uses it for the two build-only ops that run inside that
window, `add-build-edges` and `demote-claim-edge` (`opSpec.overCycles`), and the records at every
stage commit hold no cycle.

**The rigor ceiling is checked at the same gate.** `kb.LoadGraph` refuses a node record whose
`rigor` stands above `kb.RigorMaximum`, a value the `format400` decoder takes because its rigor
domain has no top, as a `check: metadata` fault naming the record and the field (SPEC §10). A value
at the maximum is no special case: an authored `rigor: 1.0` is in domain, and the ceiling is the
whole of the scale; an axiom's `1.00` is `kb.AxiomRigor`'s by rule, the same number and not a
distinct read value.

### The loader's file set and its other refusals

A KB document that is a symbolic link is refused,
not followed: `check: symlink` at the link's path, raised by the document walk and by the conversion's
own read of the KB's files alike, so no link out of the tree is read or written through. A conversion
from `1.0.0` holding bytes it cannot carry refuses before any state is built: `check: conversion`,
`path` the file, `line` and `detail` the content; a `2.0.x` KB the `2.0.x` load refuses, and a
`3.0.0` KB whose record files the `3.0.0` load (`formats.GraphLoadError`) refuses, are refused with
that load's own items, `check: metadata` (`kbload`'s `conversionRefusal`; SPEC §10). The stamp
is read from `kb.FormatFile` and, where that does not stand, from the entry point's superseded
frontmatter (`kbload`'s `formatStamp`): stamped in neither place is a `check: kb-format` refusal
naming both, and where the file stands its content alone is read, the frontmatter beside it
unconsulted.

### The zero-value hazard

A record file decoded outside `kb.LoadGraph` — by a test, by a tool, or by
a command that read bytes of its own into a struct — reads a missing `bearing` as the zero
`kb.Bearing`, which is the value `0.0` and a judgement: a premise contributing zero to the
derivation. The strict decoder refuses a missing field for that reason, and the loader monopoly
therefore carries a type's meaning and not only a version's: every decode of a record file goes
through `kb.LoadGraph`, over `Source` bytes.

### Dependency rules

- `docgraph` knows nothing of claims; the record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files.
- `claimgraph` never imports `latex/*`.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` reads the build records through it.
  `sheet` reads no build record and imports no kbase package but `kb`.
- `argmark` imports no kbase package, so every producer of text an MCP reader can receive reaches
  it.
- `log` imports `argmark` and nothing else of kbase: every line it writes spells a marked argument
  as the command line takes it, so no log line carries a marker. A package that imports only `log`
  reaches `argmark` through it.
- `kb` imports no other kbase package but `log`, `formats/format400`, the live format it
  re-exports, and `argmark`; `docgraph`, `index`, `write`, `query`, `sheet`,
  `kbdocs` and `claimgraph` read the kb-root vocabulary, the link relation (in its two readings,
  one resolver), the label-name grammar and the id minting from it. `export` reads the
  link relation a third way, `kb.ChildList`: an index's child-list lines only, so a body link is
  never a spine edge.
- `write`, `index`, `query`, `sheet` and `kbdocs` never import `docgraph`, `claimgraph` or `build`.
- The `formats` tree imports nothing of kbase outside itself (I8), so maintenance of a package
  outside it can never change what an old KB converts to: it takes and returns files as bytes, and
  owns the parsers of every superseded form and the live format's definition. Outside the tree,
  `kbload` imports the root and `kb` imports `format400`; no other package imports into it, and
  `kbload` reaches `format400` only through `kb` and the root. The write side drains the
  conversion's delete set — the tree touches no file. Within the tree imports run one way:
  `common` imports nothing of the tree; a superseded format package imports `common` and earlier
  format packages; a converter imports `common` and format packages; the root imports the
  converters, `common` and the format packages. `format400` imports no kbase package, `common`
  included, and restates any declaration it shares with an earlier format. No package but the root
  imports a converter, and none imports a later format.
- No package but `kbload`, `atomicfile` and the build's stage machinery opens a KB file; every other
  read goes through the `kb.Source` from `kbload.Open`. Enforced by `cmd/readpolicy_test.go`, which
  fails on any direct KB file read outside an allowlist.
- `kbload` is imported by `cmd`, `build` and `write` only; `index` does not import it (a refusal is
  a `result.Refusal`, one shape). `kbload` imports `buildrecords` and the `formats` root; `buildrecords`
  stays free of `kbload`.
- `mcp` is imported by `cmd` alone and reaches no `model` or `asks/call` (I9); `mcp/protocol`
  imports `log` only.
- `internal/model` imports no kbase package but `secret`, the key's type, and no `os`: it touches
  no file and knows no role or consumer (`cmd/importpolicy_test.go`, `TestModelImportsNoOS` for
  `os`).
- Only `asks/call`, `build`, `modelclient` and three `cmd` files import `model`: within `cmd`,
  file-level, `cmd/build.go`, the build's role closure, `cmd/score.go`, the score verb's ask, and
  `cmd/probe.go`, the probe's one call. `internal/model/modeltest` imports it as test support, which
  nothing shipped imports. `claimgraph` reaches neither `model` nor `asks/call`, transitively.
  `modelclient` is the only constructor of a `model.Client`, and a verb opens one only where it is
  about to ask something. `modelclient` imports `model`, `config`, `result` and `log` and nothing
  else of kbase, and `cmd` alone imports it.
- `internal/config` imports no kbase package but `atomicfile`, `result`, `argmark` and `secret`: it holds the
  grammar, the three roles, `Resolve`, `Select`, the loaded files and the templates, and refuses in `result.Refusal`. It holds the configure commands' rules too (SPEC §9, "Provider
  commands"): `ConfigureModels` — the references given, the match of each provider part against
  the pool of the written layer's stack, each reference against its provider's catalogue, and the targeted write;
  `ProviderEntryRefusals`, an entry's arguments; and the targeted editors in `write.go`
  (`UpdateConfig`, `SetProvider`, `RemoveProvider`), one line editor and one decode-compare guard
  for both files (SPEC §9.5). It reads a catalogue through a function value its caller supplies
  (`cmd/configure-models.go` binds it over the registry's `Catalogue`, under the `--timeout`
  deadline), and returns the catalogue misses and what the run wrote as data, from which the verb
  writes its stderr lines. It never imports `model` or `log`, opens no client and writes no output,
  and returns its faults as data, so each caller decides whether one is a warning or a result
  field. A refusal's remedy may name the command that fixes it (`kbase configure-providers`,
  `kbase configure-models`).
- `internal/score` imports `model` nowhere: the endpoint is below its `Launch` seam, so the runner, the
  guard, the parse and the ledger learn nothing of what answered. It imports `write` for a
  landing's ops and its lock, and never `build`, `kbload`, `statedir` or `kbdocs`: the verb takes
  the run's locks, opens the KB, resolves the state directory and runs the start refresh. A
  landing reopens the KB, to compare each value's judged hash with its item's text as it stands,
  only through the seam the verb supplies (`score.Invocation.Open`, `kbload.Open` in
  `cmd/score.go`).
- The judged hash lives in `internal/asks`, which owns the templates its digest is of: `score`
  imports `write`, so the function cannot live in `score`, and `write` imports `asks` for it
  alone. Neither reaches `model` through it.
- Only `build`, `buildstore` and, within `cmd`, file-level, `cmd/update-format.go` (its clean
  check, and the `git mv` of each root build record into `kb-root/.build/`, `Repo.Move`) and
  `cmd/write_op.go` (a committing batch's commits, made where every write surface runs its batch,
  and `update-format`'s, through the same `kbCommits`) import `gitledger`; `gitledger` imports
  `log` and nothing else of kbase. Every opener of a KB's repository goes through
  `buildstore.OpenRepo`, which refuses `worktree` where git's worktree root is not the directory
  `kb.RepositoryRoot` finds; `cmd/init.go`'s birth commit opens through it and imports no
  `gitledger`. `write` runs no git: a committing batch's pre-flight commits are the caller's
  `write.Options.Preflight`.
- `buildstore` reaches no `model` and no `asks/call`: the build's state, its stage vocabulary and
  its trail are read without the runner, and `build` imports them.
- Only `build` and `cmd` stamp readiness documents — `init` at a KB's birth, and `update-format`
  rewriting the maintained blocks (`kbdocs.Maintain`): within `cmd`, file-level, `cmd/init.go` and
  `cmd/update-format.go` alone import `kbdocs`. `mcp` imports `kbdocs` for the `.index/` rule and the
  session rule alone (`kbdocs.IndexRule`, `kbdocs.SessionRule`), which the instructions carry; it
  stamps nothing.
- `build` imports `write` (the write-lock wait); `cmd` imports `build` and `filelock` (the write
  lock's contention and the holder lock's release; the operation lock's refusal is `build`'s, and the
  held-around-the-run write lock is `write.LockKB`).
- Only `internal/latex/pandoc` execs pandoc; only `internal/gitledger` execs git; only `internal/sheet`
  execs `dot`; and only `internal/mcp/launch.go` (file-level) execs kbase itself. Those four are the
  exec policy's whole list (`cmd/execpolicy_test.go`): a process started anywhere else in non-test
  code, or a listed owner starting a binary other than its own, fails the test; test files are out
  of its scope.
- `internal/export` imports `kb` and `atomicfile`, and nothing else of kbase; the verb opens the
  KB and supplies every pandoc call — the preflight and the one `pandoc.RunDefaults` call — as a
  function value `export.Run` calls.
- `internal/auth` imports nothing of kbase but `atomicfile`, to write its files at their mode, and
  `secret`, and only `cmd` imports it; `config`'s own rule keeps it from the store.
- `internal/secret` imports no kbase package. A key's text leaves a `secret.Key` through `Reveal`
  in four files alone — `internal/model/openai_client.go` and `anthropic_client.go`, setting the
  auth header and scrubbing an echoed key, `internal/model/transport.go`, cutting a status error's
  body around it, and `internal/auth/auth.go`, sealing it — and `cmd/revealpolicy_test.go`, on
  the module walk, fails a production file outside them that names `Reveal`.

The dependency rules are enforced by an import-policy test over the transitive import graph
(`cmd/importpolicy_test.go`), beside the I1 exec-policy test (`cmd/execpolicy_test.go`).

---

## 12. Constants

| Constant | Value |
|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6) |
| Credentials store | `auth.DirName`: `auth`, the store's directory under the state base (`statedir.Base`), beside the per-KB state directories, whose 16-hex-digit keys it cannot collide with; `auth.FileSuffix`: `-key.enc`, ending each provider's file, `<provider>-key.enc` with the name lowercased; `auth.FileMode`: `0600`, each file's permission on creation and on every replace; `auth.DirMode`: `0700`, the store directory's permission, set on every add |
| `auth add` input bound | `authAddInputLimit` in `cmd/auth.go`: 65536 bytes, the most stdin `kbase auth add` accepts; it reads one byte past it to tell more from exactly (SPEC §9) |
| `stats` leverage bar and floor | `query.DefaultMaxStrength`: 0.65, the strength bar, and `query.DefaultMinDependents`: 1, the dependents floor, of `stats`' `highest_leverage`; the bar is its own constant, not derived from the band ladder, so a change to a band's lower bound does not move `stats` |
| Transport attempts | 3 per call; a provider's non-2xx status ends the call on its first attempt unless it is a 429 or a 5xx, an answer cut off at the completion cap before any answer text ends the call on its first attempt, and every transport failure and incomplete stream is re-issued; the build's multichoice asks re-issue at once, its overview call and `kbase score`'s each ask pause 5 s then 30 s between attempts, and a pause the call's deadline cannot cover is not waited out — the call gives up at once, naming the attempts it made |
| Cached-token totals | `AskTotals.WarmCacheReadTokens` (and `CachedTokens`) report 0 on every decision call: the SystemOne surface's `usage` has no cached-token field, so the zero is the API's, not a regression |
| Ask concurrency | `[asks] concurrency` in `config.toml` (SPEC §9.2), `--ask-concurrency` on `build` and `score`, default `config.DefaultAskConcurrency`, 4; the model calls one invocation has in flight, and the idle connections per host its clients keep; `probe` runs a queue of one |
| Sampling | `[sampling.text]` and `[sampling.scoring]` in `config.toml` (SPEC §9.2), each `temperature` and `thinking`; the `thinking` vocabulary `config.ThinkingLevels`; unset is temperature 0 and thinking off; the temperature ranges `openai` [0, 2] and `anthropic` [0, 1] |
| Thinking budgets (Messages) | `low` 1,024, `medium` 4,096, `high` 16,384 tokens, as named constants in `anthropic_client.go` (`thinkingBudgetLow`, `thinkingBudgetMedium`, `thinkingBudgetHigh`); `max_tokens` is the budget plus the answer's allowance (SPEC §9.1) |
| Model roles | `[models] decision` (the multichoice asks, on whichever surface its entry speaks), `[models] text` (the overview call) and `[models] scoring` (`kbase score`'s asks) in `config.toml`, each a `<provider>/<model>` reference (SPEC §9) |
| Metadata format version | The one version read and written (SPEC §10), `kb.FormatVersion` (`format400.Version400`); the stamp key is `kb-format`. `4.0.0` |
| Format stamp file | `kb.FormatFile` (`format400.FormatFile`): `.index/format.yaml`, the one-key block mapping that holds the stamp, written by `kb.EncodeFormatStamp` and read by no package but `kbload`, which falls back to the superseded entry-point frontmatter where the file does not stand (SPEC §10) |
| Pending literal | `kb.PendingLiteral`: `*pending*`, the format's one no-number literal, written for a value not yet judged and accepted wherever a value is absent (SPEC §10) |
| Conversion record | `migration`: a result's conditional record after `kb-root`, holding `from`, `to`, `note` (`kb.MigrationNote`, one sentence for every migration) and `warnings`, and the stderr lines the same call writes — the sentence, then one per warning (SPEC §7, §10) |
| MCP protocol version | `protocol.ProtocolVersion`: the MCP revision the server implements (SPEC §11): `2025-11-25`, confirmed, not chosen, by the owner's live check |
| KB write-lock wait | `write.LockWait`: 30 s, how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8) |
| State-store format | `StoreFormat` in `internal/buildstore`: 1, the integer in `<state-dir>/format` (SPEC §6, §8) |
| Builds kept | `keptBuilds` in `internal/build`: 5, the build trails, beside the current one, whose directories — answers, group records, call captures and the trail's development replays — survive pruning, and are pruned with them, and the captured builds, beside the newest, that survive it (§8) |
| Progress rotation | `progressRotateLines` in `internal/build`: 500, the lines a fresh build's `progress.jsonl` may hold before it is rotated, about five builds; `keptProgressFiles`: 15, the rotated files kept (§8) |
| Runs kept | `KeptRuns` in `internal/buildstore`: 16, the scoring runs of each kind (`run-`, `dry-`) that survive pruning, the replays of a run's `replays/`, and the development replays of a build trail's `replays/` (§6, Scoring; §8) |
| Replay progress interval | `ReplayProgressInterval` in `internal/buildstore` (`kbasedev`): 10 s, the most often a development replay's walk logs its `replay progress` line (§6, Scoring; §8) |
| Scoring wrap width | `wrapWidth` in `internal/score/text.go`: 100 columns, the width each passage of the closed text is wrapped to before its lines are numbered (§6, Scoring) |
| Operation lock path | `<git dir>/kbase-operation.lock` (SPEC §6, ARCHITECTURE §8) |
| Detached-run start wait | `launchStarting` in `internal/mcp/launch.go`: 5 s, how long the `build` and `score` tools wait for the child to start — a build's progress record to show a stage entered, a score run's ledger and pid to stand — or to exit, before returning the run still starting (SPEC §11) |
| `wait` poll interval | `waitPoll` in `cmd/wait.go`: 1 s, how often `kbase wait` reads the state store while the run it names progresses (SPEC §6) |
| Sheet render concurrency | `sheet.renderConcurrency`: 4, the most `dot` processes at once; output order is kept (§6) |
| Unmarked shortlist, proximity slots | `shortlistProximityK` in `internal/claimgraph`: 3, the nearest targets in its own paper `references-found` asks per source (§4) |
| Unmarked shortlist, wildcard slots | `shortlistWildcardK` in `internal/claimgraph`: 2, the best-ranked targets by cosine it asks per source besides the proximity picks (§4) |

Exit codes and the placeholder's digest form are in SPEC §7 and §3; the pandoc range is SPEC §2; the
stage-commit subject is SPEC §6.

### Format migration (SPEC §10)

Every verb that reads a KB — `update-format`, `verify`, the queries,
the write ops, `export-latex`, and the build's spine-seed, phase-3a, readiness
stamp, overview and resume — calls `kbload.Open(kbRoot)`; `status` and `cancel` do not load the KB.
`Open` reads the stamp alone — from `.index/format.yaml` (`kb.DecodeFormatStamp`), else from the
superseded entry-point frontmatter (`kb.FormatStamp`) — before anything else. A
stamp at the current major and minor builds a `kb.Source` that reads disk. An older stamp reads every file the format covers (`kb-root/` and the build records beside it)
into a map of repository-relative paths to bytes and hands it to `formats.Chain(from, to)`, which
applies each converter in turn and returns the current form's files, the union of the obsolete
paths and every step's warnings; the `Source` serves the returned bytes and reads an obsolete path
as absent. A newer or
unreadable stamp is refused `check: kb-format`; no entry point is refused `check: kb-root`. Each
converter (`internal/formats/from<from>to<to>`) parses its own superseded form and emits the
next, reading each side's definition from its format package (`internal/formats/format<version>`);
the `formats` tree is the only home of a superseded parser and of the live format's definition
(`format400`, which `kb` re-exports), it imports nothing of kbase outside itself so maintenance of
a package outside it can never change what an old KB converts to, and `kbload` is the only
importer of its root from outside it (import-policy test). The pending obsolete set
lives in the `Source` and reaches the write side only through `index.UpdateFormat`'s save, which
`cmd/update-format.go` alone calls; every other writer refuses a migrated `Source` first
(`index.PendingConversion`: `write.Run`, `score`'s verb, the build's `loadFormat`). The save runs in
this order (SPEC §10): `saveConverted` writes `.index/edges.yaml`, then `.index/nodes.yaml`, then
every other migrated file but the entry point, each atomically; then the register pages and the sheets; then the stamp file,
written at the version the tree is still at, so a stamp always stands and always names the tree's
version; then the obsolete paths minus what this save covered; then the entry point — its
frontmatter, which stamped the superseded form, stripped by the conversion; and last the stamp file
rewritten at the current version, so the stamp marks a completed migration. The verb then rewrites
the maintained blocks (`kbdocs.Maintain`, §6). A save that stops
part-way leaves the KB stamped at the version it converts from, its next load resuming the
conversion; `update-format` refuses that tree as not clean, and the recovery is to
restore `kb-root/` and any moved build record from `HEAD` (SPEC §10). A converter that finds `.index/nodes.yaml` standing
takes both record files as they stand (`from200to300`'s `resumed`), since only an interrupted save
writes it, and carries the stamp file through unchanged, holding the version the tree is still at;
`from300to400` likewise takes as it stands each record file that already decodes as `4.0.0`,
which only an interrupted save writes over a `3.0.0` stamp. `from300to400` decodes the `3.0.0`
records through `format300`, writes them through `format400` — every `judged-hash` the zero
`format400.JudgedHash`, the unjudged value — and reports each dropped `no-edge` and `strengthen-by`
field as a `formats.Warning` carrying its list (`common.ListNoEdge`, `common.ListStrengthenBy`),
the node's id and the text, fields `kb.MigrationWarning` carries identically (SPEC §10). It mints
each work's and axiom's id from its `3.0.0` id alone (`mintIDs`: the SHA-256 of the old id, then of
the old id, a NUL and a counter, re-derived in ascending `3.0.0`-id order while an earlier minted id
or any `3.0.0` id holds it), its key or label the record's `label`, and renames every `3.0.0` work
or axiom id in an edge's end and a prose field, each a warning listed `ids` (`common.ListIDs`), its
`3.0.0` id under `ID` and the minted one under `Text`; the mapping is read off a `4.0.0` node file
where an interrupted save left one, and an edge file decoding as `4.0.0` still has its `3.0.0`
ends renamed. Its
markers' steps (`convertMarkers`) read each kb-root document's `3.0.0` markers through
`format300.StatementMarkerRE` and write `format400.StatementMarker`s, the `lifts` edges of whichever
record file form stands deciding the one move; it returns each `3.0.0` register page,
`claim-rigor.md` at kb-root and in a top-level directory, as an obsolete path, and the save renders
the summary pages. It moves each build record standing at the repository root into `kb-root/.build/`,
bytes unchanged, its root path obsolete ([page §3.6](NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). `update-format` makes that move on disk
with `git mv` (`gitledger.Repo.Move`), the one mover: the verb checks the moved records clean and
hands `index.UpdateFormat` the moves (`index.RecordMove`) and `Move`, which it calls as the phase
after the load (`Precheck`, then `Derive`) and before the save's first write, so a refused
conversion moves nothing and the save finds each record in place and its root path gone. The save
itself writes and removes nothing outside `kb-root/` (`index.WriteKBFile` refuses such a target).
Each moved record is reported in `removed` at its root path, repository-relative, and in `written`
at its new path, kb-root-relative. Its citations' step (`convertCitations`) then reads every link of each kb-root
document but a summary page through `format400`'s link grammar, resolving each anchor under `3.0.0`
slugging (`format200.Slugify` over the heading as written, the `3.0.0` slugging verbatim) and writing
it under `4.0.0`'s (`format400.Slugger` over the heading as converted, its bare ids already
citations, which render as their link text), a document's nth heading being the same heading in both
forms; an anchor naming one heading under `3.0.0` slugging and another under `4.0.0`'s is left as
written. A node's place
is read from the converted markers, a `3.0.0` register entry from the records (its node's title
slugged on the page its host's volume rendered), and a work's anchor on the root summary page from
the page's heading texts (`format400.SummaryTitle` and the entry headings), taken only where no
other heading there can take it. No link stays aimed at a removed register page: one whose anchor
names one entry with a place points at that place; one whose entries are all stated in one section
points at that section, untitled; any other points at the summary page replacing the page
(`format400.SummaryPage` over the page's own path), without its anchor; and where no node renders
on that page, so the save writes none, the link goes and its text stands, a bare id in that text
converted as any is. A bare id is read in the `3.0.0` id grammar and cited under its minted id.
Each `### Axiom <label>:` heading of `axioms.md` naming an axiom record ends in its axiom's marker;
one naming none is a warning. Each conversion it makes of a bare id is a warning listed
`bare-ids` (`common.ListBareIDs`) and each link or id it cannot resolve to one node, and each
link it removes, one listed `unresolved-citations` (`common.ListUnresolvedCitations`), each carrying the id where it has one; only this converter makes them.
The five list names live in `common` alone; the `formats` root and `kbload` re-export them, and
`update-format`'s result lists (`cmd`'s `formatResultLists`) match on `kbload`'s.
The removals are reported as `removed` by `update-format`. A converter is a pure function with a golden pair as
its test; the chain's order is the version order and a gap in it is a test failure. The record files
of each `3.0.0` and `4.0.0` golden tree, `internal/formats/testdata/<tree>-<version>/`, have a
decoded-values golden beside it, `<tree>-<version>.values.jsonl`: one JSON object per record in the
files' order, its keys in its type's field order, a pending value as the literal, which that
version's decoder, `format300`'s or `format400`'s, reads them to (`TestRecordFilesDecodeToTheirValues`).

### MCP server (SPEC §11)

`internal/mcp` builds the tool table from the command tree its caller
hands it: one Tool per command declaring `kbase.mcp` = `bound` — `mcp`, `wait`, `auth` and
cobra's own `completion` and `help` declare `excluded`, each with its reason — save that the commands
declaring one family (`kbase.mcp.family`, set by `mcp.InFamily`) are one Tool together, in the place
of the first; no tool is listed by hand. A family is an `mcp.Family` its caller hands it (name,
description sentence, shared help), `query` (`toolFamilies`, `cmd/surface.go`); a family a bound
command is named for is that command's, its members no Tool of their own: the write ops declare the
family `write`, and the `write` Tool is `kbase write`'s (`mcp.batchTool`), its schema the batch —
`entry`, a required array of objects, and `dry-run` and `commit`, booleans — its arguments handed whole to
`Calls.WriteOp` as `write.Batch`'s values document. A
command tool's schema is derived from the command's `Use` line positionals and its flags,
`mcp.CommandSchema` skipping a flag annotated `kbase.mcp` = `excluded` (set by `mcp.ExcludeFlag`),
which is no argument of the tool; `score`'s `--replay` and `--failed` are such flags, and an
argument naming one is refused by `internal/mcp/protocol` as outside the schema. A command's
read-only standing is the cobra annotation `kbase.readOnly`, read when the table is built. A command
tool's `required` is its positionals alone, less an optional variadic one, which its `Use` line
spells `[<name>...]`: no bound command refuses for want of a flag, `score`'s `--pass` among them, a
run walking both passes unless it is told otherwise; `wait`'s `--pid`, the one flag a verb requires,
is outside the derivation, `wait` declaring `excluded` (§11).

A family tool's schema is `operation`, a string enum of its members' names in the tree's order, and
`arguments`, an object; its read-only hint is set where every member declares `kbase.readOnly`.
Each member is built as its own command tool would be, and
the family keeps it by name, unlisted: a call takes the member `operation` names and checks
`arguments` against that member's schema with `protocol.Schema.Faults`, the same check
`internal/mcp/protocol` runs on a tool's arguments, so no argument list is stated twice (a query's
from `mcp.CommandSchema`). A fault is a usage refusal
document, one item per fault naming the argument, the member's arguments as `allowed`, and a
remedy naming `allowed` for the argument names and the member's help resource for what it does;
arguments that fit run the member as its own tool would. A values op's help
lists its entry's keys in vocabulary order, each with the type, required mark and description its
definition in `internal/write/values.go` carries, through `write.OpVocabulary` (`mcp.ArgumentList`),
naming no flag, so it is true on both surfaces. The members' shared help is appended to no member's `Long`. The root command's `Long`
states the CLI's form once each — `queryHelp`, `listQueryHelp` and
`valuesHelp` in `cmd`, the last closing on `batchHelp`, the write batch's contract — and the `query`
family's `Help` the MCP form for `kbase://help/query`, from the same text where the two agree
(`queryReads`, `queryAnswerHelp`, and `listQueryHelp`, one marked text each rendering spells for its
surface, SPEC §7 `key`); the resource lists the members from the annotations
the enum is built from. `batchHelp` is also `kbase write`'s `Long`, so `kbase://help/write` carries
the contract in full, and every write op shares one MCP note and one CLI note (`writeOpCommand`).

`cmd/surface.go` adds every command to the tree, one `AddCommand` per group, its binding and
read-only hint on that line (`readOnlyTool`, `readWriteTool`, `cliOnly`, and for a family tool's
operation `queryOperation` or `writeOperation`, which add `kbase.mcp.family`), and declares beside them
the MCP-only surfaces: the resource families and the detached launches. It names no project for
them: `mcp.Server.Serve` binds the resources to the server's located project.

A call's project is applied in one place. `mcp.Tools` wraps every tool in
`inProject`: the wrapper adds the `project-dir` property to the schema, and on each call
`callProject` resolves the project — the `project-dir` the call names, which `buildstore.ProjectRoot`
requires to be a git worktree root (the same check `--project-dir` gets on the command line),
else the located project — removes `project-dir` from the arguments, and hands the inner tool the
project. No tool closes over a directory: a command tool sets it as the call's invocation
directory, a write op passes it to `Calls.WriteOp`, and a launch starts its child there. The located project is `mcp.Located`, whose zero value is the
explicit none: `cmd/mcp.go` sets it from `kb.RepositoryRoot` of the invocation directory only
where that finds a repository, so no reader ever passes `""` to a path function that would read it as
the working directory. With none, a call naming no project answers the usage refusal keyed
`project-dir`.

Every tool has a command behind it (I9): `build-trail`, the build's commit trail, is
`cmd/build-trail.go`, a command tool like `status` (`buildstore.BuildTrail`), so its schema, key
order and refusal keys are derived as any command's are.
`TestMCPToolsAreTheCommandTree` derives its expectation from the annotations alone — a family's name
for its members, a command's own otherwise — and asserts the tool set for equality, so a tool listed
by hand fails; `TestBuildTrailIsOneDocumentOnBothSurfaces` runs `build-trail` at the command line
and through the server and requires the two documents to agree, each surface spelling the build
call in each boundary's command as it takes it.

The `write` tool's arguments are the batch (`batchTool`), run through `runWriteOp` as
`write.Batch`'s values document with `ArgumentEntry` set: each refusal keeps its `entry`, and one
naming no `path` drops `line` and `column`, which would point into a document the caller never
wrote, while one naming a document keeps its line there; a refusal's `key` is the values key, which no
surface spells differently.
Every other tool runs the command's RunE with its flags reset and its output captured to a buffer. The
buffer is the result text; decoded once more, the YAML document is the `structuredContent`; the
document's outcome gives `isError` (`result.IsError`). A RunE starts from `invocationDir(cmd)` —
the call's project over MCP, else `--project-dir`, else the working directory — never
`os.Getwd()`.

`internal/mcp/protocol` holds newline-delimited JSON-RPC over stdio, `initialize`, `ping`,
`tools/list` and `tools/call`, the `Tool`, `Schema` and `Property` types, and the frame checks
(`jsonrpc`, `id`, `method`, known tool, object arguments). Arguments failing the schema (missing,
wrong type, a string outside its `enum`, unknown key) are JSON-RPC `-32602` errors, not refusal
documents; `Schema.Faults` is that check as data, which a family tool runs on its `arguments`. `Serve` takes the
server version, the instructions `initialize` reports, the tool table and the resources from its
caller, and imports `log` only. The instructions are `mcp.ServedInstructions`, spelled over MCP:
its opening, which varies by mode — the located project's state where it located one, the
`project-dir` lines where it located none — and the sentence naming the skill, then the `.index/`
rule, `kbdocs.IndexRule`, and the session rule, `kbdocs.SessionRule`, which the stamped
`AGENTS.md` carries through its `index-rule` and `session-rule` slots, then `mcp.EdgeDirection`,
which the root's `Long` carries too, then the paragraphs only the instructions carry
(`instructions.go`): the text a session reads. `resources/list` and `resources/read` serve the resources `internal/mcp` supplies: the
sheets, under `kbase://sheet/<kb-root-relative path>`; the model guidance, at
`kbase://model-hints`, which is `internal/config`'s rows rendered as markdown and needs no KB;
and each family's help, at `kbase://help/<family>`, then each bound command's, at
`kbase://help/<command>` (`mcp.HelpResources`), the latter the command's `Long` and its MCP note
(`kbase.help.mcp`), composed in `internal/mcp/help.go`; neither needs a KB.
An unknown URI is `-32002`, a missing `uri` `-32602`; `initialize` advertises `resources` with no
`subscribe`, and the protocol answers no subscribe method (`TestNoSubscription`).

The server runs in one of two modes, chosen in `cmd/mcp.go` before serving (SPEC §11). Located,
`kbase mcp` inside a repository, the located project is `kb.RepositoryRoot` of `invocationDir`.
Where that finds none, or under `--project-dir-required`, it is `mcp.ProjectRequired()`, the zero
`mcp.Located`: none, so that `inProject` lists `project-dir` in each schema's `required` with
`projectDirMeaning` alone as its description, where a located server gives
`projectDirDescription`, that text followed by the sentences on the server's own project. The `callProject`
refusal stays behind that schema for a call whose `project-dir` is empty. The global
`--project-dir` given to `mcp` is a usage refusal keyed `--project-dir` before serving; startup
never fails for want of a project.

The `build` tool (`internal/mcp/launch.go`, the one self-exec owner under I1, file-level, enforced
by the exec-policy test) re-executes the kbase binary as `kbase build …`. The child runs from the
root of the call's project, with the server's environment, in its own process group (detach flags
per platform in `internal/mcp/detach_unix.go` and `internal/mcp/detach_windows.go`), stdin unset,
the server's `--config-dir`, `--log-level` and `--log-file` passed through, and `--project-dir`
where the call named one, so the child's recorded `resume` carries it. Every path the child is
given is absolute, because its working directory is not the server's: a relative `state-dir`
argument is resolved against the repository root, while the server's `--config-dir` and
`--log-file` stay relative to the server's working directory. Its stdout and stderr are files in
the state store's `reports/`, `build-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`; they
are not cleaned up. The tool waits on the child's progress record and returns when the record shows
a stage entered or the child exits, bounded by `launchStarting` (5 s, §12; the child takes
the operation and holder locks and emits its first progress event before it waits out a write,
§8):

| Child, within 5 s | Result |
|---|---|
| Entered a stage | `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait` (`resume` only where the record's command reads as a `kbase build` invocation, SPEC §6; `wait` the run's background command, `buildstore.ShellJoin` of the executable with `wait [--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one) |
| Exited first | The child's own document; `isError` from its outcome |
| Neither | `done` with `state: starting`, `state-dir`, `pid` and `wait`; `status` follows the run |

Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
`kb-root/`, returns the refusal `status` gives and starts no process.

The `score` tool re-executes the kbase binary as `kbase score …` by the same detachment: the child
runs from the root of the call's project, in its own process group, with the server's environment,
`--config-dir`, `--log-level` and `--log-file`, `--project-dir` where the call named one, and its stdout and stderr are files in the state
store's `reports/`, `score-<UTC timestamp>-<random>.stdout.yaml` and `.stderr.log`, which `score.PruneRuns` keeps the newest `KeptRuns` of (§8). It waits on the
run's records and returns when the run's ledger and pid stand (the run's directory written, its pid
file reading back) or the child exits, bounded by the same `launchStarting`. The
`edge` flag is declared a mapping (`mcp.MappingFlag`), so its schema property is an object of the
required strings `source`, `target` and `relation`, closed to other keys, and `CommandLine` joins
the mapping into the child's `--edge <source>,<target>,<relation>`, as `mcp.Spelling` does in a
call it renders; a value holding a comma is refused at `edge` (`commaInMapping`) and starts
nothing:

| Child, within 5 s | Result |
|---|---|
| Ledger and pid stand | `done` with `kb-root`, `state-dir`, `pid`, `run`, `ledger` and `wait` (`wait` the run's background command, `buildstore.ShellJoin` of the executable with `wait [--project-dir <root>] --state-dir <store> --pid <pid>`, `--project-dir` where the call named one) |
| Exited first | The child's own document; `isError` from its outcome |
| Neither | `done` with `state: starting`, `state-dir`, `pid` and `wait`; `status` follows the run |

A score run's result document is written beside its ledger before the run lets go of the store's
holder lock, so `wait` reads the same document the run reported rather than racing the lock's release.
`wait` locates the run by the pid the launch named — `score.RunByPID` reads the run's `pid` file —
and reports the run's outcome; a run whose process ended without writing a result reports the
failure its ledger supports. `status` and `cancel` read the state store's holder record (the
pid, the run, the start stamp and the kind `score`), so a run in flight is visible under the pid the
tool returned.

Nothing announces a run's end: no resource is subscribable and the server writes no frame a
request does not owe, so a session follows a launched run by calling `status`, or by running the
`wait` command the launch returned (SPEC §11). `test-integration-mcp-build`
(`TestMCPBuildFollowedByStatus`) drives a real server to a build's end by polling `status`.

---

## 13. Slice verdicts

The reader's design was settled by a slice — `kbase build --through document-graph` over six corpus
papers and a census of all fifty — judged by comparing kbase's tree against the reference tree for
the same paper, with kbase's own readers. The hypotheses and their verdicts:

| Hypothesis | Confirmed by | Refuted by | Verdict |
|---|---|---|---|
| No filter layer is needed | The conformance and inventory checks pass on kbase's tree; per-leaf word streams match; every citation state the reference trees exhibit appears in kbase's | A load-bearing form or word no JSON transform can produce | **Confirmed** on the six Slice-1 and Slice-2 papers; 49 of 50 corpus papers build |
| Records suffice for the inventory | Every reader and placement fact of the inventory (§5.5) is carried by kbase's records, item by item in scan order; every record joins exactly one document | A reader or placement fact that exists only in rendered bytes | **Confirmed**, with §5.5's visibility rule: a block record exists only where the page carries a label line the reader reads |
| The preceding text run suffices | Withdrawn: the word before a reference is defined over the written page, so it is a page-defined reading (§5.5), not a record | — | Withdrawn |
| pandoc's writer suffices | Every Slice-1 check passes under it | A load-bearing form it cannot emit | **Confirmed**; the writer route is pandoc's gfm writer with `--wrap=none` (§5.4) |

### Slice-1 checks

identical tree path set and child order; equal per-leaf word streams under the
markdown tokeniser; the Document-Tree conformance passes with matching counts
of up-link lines, three-attribute anchors, math fences, `data-cites` citation spans and labelled
blockquotes with identifiers; equal inventory; records carry every reader and placement fact of that
inventory; byte-identical `kb-root/` and records across two runs. All pass: the writer route is
pandoc's. Any fails: execution stops and the owner rules (fix, amend the check, or a Go writer).

**Slice-2** applies the same checks to four more papers, and requires two negative controls to
refuse as SPEC §5 specifies (naming the paper and pandoc's error; naming each unloaded file and the
line that named it). **Slice-3** records a census, with no pass threshold.

A check that fails for a reason whose fix needs a third pre-pass, a filter, or a change to any of I1
to I7 is escalated to the owner.

---

## 14. Limits

Where the format and this implementation stop. A limit whose mechanism already states it is not
repeated here: the zero-value default is §11's, and the index's lack of statement text is §6's.

**A value records whose call wrote it and what it was judged against, and nothing more.**
`manually-scored` says whether a person's call or a machine's wrote a rigor or a bearing, and
`judged-hash` what it was last judged against (SPEC §10) — no author, time or reason — and a work's
rigor and a `founds` bearing, which no run writes, carry neither. A `*pending*` the caller wrote and
one written because it left the value out differ only in that flag. A value converted from `2.0.x`
is marked model-scored whoever judged it, and nothing records that it was converted.

**A `founds` bearing on a work names its finding in prose.** The `context` on the record says which
theorem, table or result the bearing judges, and no id, range or hash joins the two, so no check can
prove the judgement was about that finding. An axiom's `founds` edge needs no such prose: its source
is the axiom's own id.

### A conversion records no history

The value changes a conversion makes are re-derived from the
converted records, and nothing but its warnings says which of them rose or fell.

### Merged metadata is refused, not merged

Two branches' record files merge by text: a duplicate edge, a dangling edge or a cycle the
merge leaves is a load error, `check: metadata`, which every command that loads the KB
refuses. No op resolves it: the recovery is in git, returning the record files to a consistent
commit, such as the one before the merge, and reapplying the other side's changes through
write ops (SPEC §10).
### No format sits between two majors

`formats.Chain` walks converters version by version and
refuses a step no converter leads, so a KB stamped `1.1.0` is refused at load rather than read; no
`1.1.0` converter exists and none is reachable.

### No refresh moves a judgement

A refresh — `update-format`'s re-derivation included — asks no model, so a derived value changes only when the
records do — a write op or a scoring run's landing. `verify`'s freshness is
between the records and a register page, never between two judgements.

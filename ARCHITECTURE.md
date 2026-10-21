# ARCHITECTURE – kbase

How this implementation meets [SPEC.md](SPEC.md). Purpose and intent are in [THESIS.md](THESIS.md);
house rules are in [CONVENTIONS.md](CONVENTIONS.md).

Every mechanism stated here is decided and landed; §13 records the verdicts of the slice that
settled the reader's design.

---

## 1. Purpose

kbase makes a knowledge base auditable for an academic lab running local inference. The claim graph
is in scope: the KB carries nodes and edges drafted mechanically and refined by inference, and every
refinement is checkable against the artifact it refined. kbase builds that KB as Go, keeping its own
provider management and inference invocation.

## 2. Invariants

Violating any is a blocking defect.

| # | Invariant |
|---|---|
| I1 | Exactly one package execs pandoc (checks version and `pandoc-api-version`, with named errors); exactly one execs git; exactly one execs Graphviz `dot`; exactly one execs kbase itself (the MCP server's detached build, `cmd/mcp.go`). The exec-policy test names each owner. |
| I2 | The claim-graph inventory's reader and placement facts come from records, never recovered from leaf markup; its page-defined readings (§5.5) come from the leaf through the page readers; otherwise only stage-A conformance (the link relation, through `kb`), the prose and the marker machinery, and the sheet (a claim's kind, at its marker) read leaf lines, and maintenance ops read only the metadata layer. |
| I3 | There are two pre-passes, in one registry, each naming the reader-level hole it fills and emitting a census. |
| I4 | System compatibility per SPEC §3; byte equality only where a named reader needs it; presentation is untested. |
| I5 | One result emitter owns stdout. |
| I6 | Every mutating op is idempotent under re-issue; `init`'s creation is birth-once (SPEC §8). |
| I7 | Every prompt template and fragment is an embedded file. No prompt prose originates in code. Each call's system prompt is the fragment named for it. |
| I8 | One metadata format version is read and written (SPEC §10). Every KB file is read through `kb.Source`, built by `internal/kbload` alone; `kbload` is the one importer of `internal/migrate`, which converts an older KB at load — a map of repository-relative paths to bytes in, the same out, plus the obsolete paths; no file I/O, no import of a kbase package — and the result is written only by the next natural write, which drains the obsolete paths after a successful full save, minus what that save wrote. |
| I9 | The MCP server binds subcommands and adds no operation: a command declares its binding with the cobra annotation `kbase.mcp` = `bound` or `excluded`, and the tool table is built from the `bound` commands of the command tree plus one tool listed by name — `rewind-build`, which reports the build's commit trail and changes nothing, the whole exception; the table's test derives its expectation from the same annotations plus that name and asserts the set for equality, so a second listed tool fails as a missing command does; its stdout carries JSON-RPC frames and nothing else; no tool but `build` performs inference, `models` and `configure` reaching only the provider's catalogue — `models` reading it, `configure` dialling it to check a model map; `build` through it is the detached subcommand. |

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

---

## 4. Stage table

The build walks these stages in order. A stage boundary is a commit (§8).

| Stage | Does |
|---|---|
| `start` | pandoc and git preflight; populated-`kb-root/` refusal; run lock |
| `document-graph` | The tree from the volume roots |
| `spine-seed` | Seeds `.index/` over that tree |
| `claims-declared` | Claim-graph pass over author-declared claims |
| `claims-discovered` | Claim-graph pass over discovered claims |
| `equations-minted` | Equation nodes minted |
| `references-found` | Unmarked references found: adjacency links made by rule between consecutive non-equation claims of a leaf, unasked; per source claim, the proximity slots — nearest in its own paper by locality (same document, same directory, same paper), then by places apart in the sequence of every claim by reading position and id — then the wildcard slots, best by TF-IDF cosine less the proximity picks, counts in §12, no pool holding a marked pair or an adjacency link; each source's shortlist is its own, so a pair both ends pick is asked both ways, and a proximity slot its paper cannot fill stays empty; one letter ask per planned pair, a yes in either direction one undirected link for the cull; writes nothing under `kb-root/` |
| `claims-culled` | The co-linearity cull (`Culled`, `internal/claimgraph/cull.go`): one `Colinear` ask per link with a prose end — marked pairs, unmarked yeses and adjacency links, unordered, computed over the tree `references-found` read — grouped by the earlier claim in reading order (`p1-<id>`); one confirming ask per culled claim whose root is not its target, grouped by root (`p2-<root>`); the survivor rule and re-pointing in the pure `cullTargets`, `cullRoots`, `finalCulls` and `PlanCull`; culls issued through `IssueCulls` with no bridge and no claim-to-claim edge; the cull record written once, after the issue. Its row spends its own inference and is dropped under `--no-inference` (SPEC §5) |
| `depends-attributed` | Dependency edges attributed: the marked candidates and the cull record's links, each link oriented later claim to earlier (an equation node always the target) and skipped where a marked pair joins its claims; one letter ask per candidate; every answer's edge written in one `add-build-edges` batch, cycles included; then the ring row, every `supports` edge on a cycle demoted with its origin (SPEC §5, §10), before the gate |
| `phase-3a` | `refresh`, `verify`, and the readiness stamp |
| `overview-drafted` | The overview document |

Each claim-graph stage is one `internal/claimgraph` invocation.

- `--through <stage>` bounds the walk; `--no-inference` drops the stages' inference rows and walks
  the rest (SPEC §5). Which rows drop follows the classification on the row
  (`row.spendsInference`, `internal/build/build.go`) and never a stage id. A row marked
  `spendsInferenceInPart` (`depends-attributed`) is not dropped: its asks spend
  inference, but the row itself still runs under `--no-inference`, with a nil letter reader. The
  provider preflight asks `callsAModel`, which counts such a row.
- `phase-3a` stamps `AGENTS.md`, `CLAUDE.md` and `CONVENTIONS.md`, each only if absent;
  `kb-root/CLAUDE.md` is exactly `@AGENTS.md`.
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
verb makes it, and `internal/export` never reaches pandoc. The export's mechanism is §5.7.

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
paragraph or equation fence — and the paragraph around a reference, as ask input; printed
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
degradation, idempotence — this section cites and does not restate. §5.3 chose Go over a
Lua filter because the forward transform emits records; the export emits none, so pandoc's own
filter mechanism suffices, and the author can re-run the conversion with plain pandoc.

**One run, three owners.** The verb (`cmd/export-latex.go`) opens the KB through `kbload.Open`,
walks it (`export.Walk`), narrows to `--volume` (`export.SelectVolumes`), assembles `gen-tex/`'s
inputs in memory (`export.Assemble`), writes them (`export.WriteOut`), makes the one
`pandoc.RunDefaults` call, and writes the report (`export.WriteReport`). The call is `pandoc -d
defaults.yaml` run in `gen-tex/`, so every path in the defaults file is relative and the author's
re-run from there resolves the same way. The version preflight runs before `WriteOut`, so a pandoc
refusal precedes any write.

| Owner | Owns |
|---|---|
| Go (`internal/export`) | What pandoc cannot know: the spine's order and ranks, the label namespaces, the KB's contract forms as data. The stream files, the tables, the defaults file, assets, the manifest |
| The filter (`kbase-export.lua`) | Every rewrite of the parsed document: references, citations, links, labels, headings, theorem blocks, figures, maths fixes, the bibliography. Constant code over the tables |
| The template (`kbase-export.latex`) | The LaTeX preamble, package order, theorem machinery, running heads, front and main matter |

Both the filter and the template are embedded (`filter.go`) and written verbatim, so their bytes
do not vary with the KB.

**Reader.** `gfm+attributes-gfm_auto_identifiers`. The KB's maths is gfm's: pandoc 3.12.1's
`markdown` reader has `native_spans` but not `tex_math_gfm`, `gfm` the reverse. `markdown`
would not read the KB's maths, and `gfm` leaves the KB's contract forms (label spans,
reference anchors, citation spans) as raw-html open and close pairs, which the filter pairs
itself. `attributes` supplies the `{#id}` the document boundaries use; with `gfm_auto_identifiers`
off, a heading without one carries no identifier.

**Streams and file scope.** The stream is `kbase-export-front.md` — the entry point's prose, its
first heading taken as the document `title` and its up-link and volume-list lines removed, every
remaining heading marked `{.unnumbered}` — then one `kbase-export-vol{N}.md` per volume in order,
each holding the volume's index and every document of its spine with navigation stripped
(`export.Strip`). `defaults.yaml` lists them as `input-files` with `file-scope: true`, so pandoc
parses each file apart and prefixes its identifiers and `#`-link targets with `<file>__`. The
scope is the volume, for two reasons: one concatenated parse would collide the footnote labels of
different volumes, and a footnote's definition in a built KB can sit in a different document of its
volume than the reference, which a per-document scope would sever. The filter's first act removes
the prefix, for the file names in the tables' `streams` and no other `__`.

**Document boundaries.** The first heading of each document, outside code fences
(`kb.FirstHeading` over `kb.StripCodeFences`), carries the document's label as a `{#d{N}-…}` id,
written by Go; a volume's index carries `vol{N}`. A document with no heading opens with one
naming its path (the file name less its extension; the directory name for an index). Headings
rather than fenced divs, because a `:::` line inside a code fence closes a fenced div early. Go
marks the boundary and the filter reads it: the document a node belongs to is the last heading
before it whose id is a `documents` key, and before the first one the front matter.

**Tables.** `kbase-xref.json` is generated from the walk, each document's first heading and the
bibliography sources, and named to the filter by the metadata key `kbase-xref`. The field-level
schema is the head comment of `kbase-export.lua`; the keys:

- `documents`: heading id → path, volume `N`, rank;
- `paths`: kb-root-relative path → heading id (href resolution);
- `volumes`: in order, each with its `vol{N}` label and title;
- `theorems`: every display name `kb.LabelLine` reads off a label line of the front matter or an
  exported document, so the label-name grammar stays in Go;
- `bibliography`: citation key → entry body as gfm text. Each exported volume's `references.md`
  entries (`kb.BibliographyEntries`) and, where any exported volume has none, every register's
  works (a work's title), the first entry of a key standing;
- `report`: the path the filter writes its findings to;
- `assets`: kb-root-relative path of a copied file → its path under the out directory; the
  claim-graph sheets (SPEC §3) are not copied, being drawings of the metadata;
- `entry_point`: the entry point's path, the front matter's source;
- `streams`: the stream file names in reading order.

**The filter.** Preparation, then two passes over the one document pandoc reads, then the end.

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
it agree; such a KB's internal links resolve through the link rule and its register's works form
the bibliography.

The end: `thebibliography` over the cited keys in first-emission order, each body rendered with
`pandoc.write(pandoc.read(body, "gfm"), "latex")`; the `\kbasetheorem` declarations (below) in
`header-includes`; and the findings, a JSON list of `{form, path, reason}` (`path` the document's
kb-root-relative path), written to the tables' `report` path.

**Labels.** Three families that cannot collide: volume `vol{N}`; document `d{N}-{path-slug}`
(`export.DocumentLabel`: the path below the volume directory less its extension, each segment
lowercased with runs outside `[a-z0-9]` one hyphen, joined by hyphens; an index slugs as its
directory name and `index`); author `v{N}-{label}`. Go mints the first two; `SelectVolumes`
numbers them after `--volume` narrows the list. A document label another path already owns is a
walk finding, and that document gets no id. The author spelling is the filter's `namespaced`,
the one function every rule that writes a label or a reference calls, so a raw `\label` in maths
and an id the writer prints agree: it keeps ASCII letters, digits and `+-.:;=_` and spells any
other character `ux{hex}`, whatever the label's own shape.

**Theorem blocks.** A labelled blockquote whose first body paragraph opens with its printed number
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

**Template.** `book`, `oneside` and `openany`, so every page is laid out alike and no blank page
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

**`gen-tex/` and the preamble.** `WriteOut` owns `gen-tex/`. A `gen-tex/` holding files and no
`kbase-export.yaml` is an `ExportDirError` (the `export-dir` refusal) and nothing is written. The
manifest lists every file the export owns — the assembled files plus `main.tex`,
`kbase-export-findings.json` and the report — and is written after the files it names. A file
whose bytes are already there is not rewritten; a file the previous manifest listed and this
export does not own is removed, with the directories the removal empties; each manifest path is
checked to lie inside `gen-tex/` before any removal. The starter `preamble.tex` is written beside
`gen-tex/` only where nothing stands at that name and is never opened afterwards; the report
checks its existence only. These are the only writes outside `gen-tex/`.

**Idempotence.** Assembly is deterministic in the KB and the kbase version, so a re-issue
reproduces every assembled file byte for byte and `WriteOut` writes none. pandoc rewrites
`main.tex` and the findings each run, so the verb compares their bytes before and after; the
outcome is `unchanged` when nothing was written, removed or rewritten with different bytes,
the report included.

**The report.** `kbase-export-report.yaml` is one YAML list: the walk's findings (`export.Report`:
a target not walked, a path named twice, a label collision), then the filter's, then, where `preamble.tex`
exists, a line naming it as the author's. `WriteReport` reads the filter's findings back from
`kbase-export-findings.json`, so the report exists only after pandoc has run.

---

## 6. KB model and maintenance

| Mechanism | Package | Status |
|---|---|---|
| **Metadata layer**: kb-root model: names, exclusions, link primitives and the dead-link gate over `kb-root/`; frontmatter, register and leaf parse, discovery, schema; the derived-field line renderers and the frontmatter splice refresh writes through | `internal/kb` | Landed |
| **Write API**: render, values, store, ops | `internal/write` | Landed. An insert adopts by (register, title) whatever its other values, reporting the differing fields (SPEC §8). There is one KB write lock, `write.LockKB(kbRoot)` (`lock.go`): `internal/filelock`'s advisory lock on the directory `kb-root/` sits in. The caller takes it and holds it across the whole run (SPEC §8): `cmd` from `kbload.Open` through the write, the trailing refresh and the sheets to the result, and around `refresh` and `render-claim-graph`; `cmd/score.go` takes it around each landing and around each of the run's two refreshes, never across a run (§6's Scoring). The ops and refresh themselves take none, and the per-directory splice locks are gone. A writer that cannot take it within the wait (§12) gets `retry`, `check: lock`, `path` the repository root. `render-citation`, `verify` and the queries take none. The build takes and releases it once to wait out a write in progress (§8). On Windows, where no advisory lock exists, only the read-then-compare check guards against a concurrent writer |
| **Refresh and verify; the index**: strength, aggregates, footers, `.index` emit and freshness, the citation gate and the claim-id link check | `internal/index` | Landed |
| **Queries** | `internal/query` | Landed |
| **Sheet**: DOT emitter and `dot` invocation; index, build records and repository name in, SVGs out | `internal/sheet` | Landed as the drawing of SPEC §3; the placeholder only where `dot` is absent |
| **Readiness stamping and README assembly** | `internal/kbdocs` | Landed: the stamp (only if absent), the overview excerpts, the passage structure check, the README assembly; the templates under `internal/kbdocs/templates/` |
| **Scratch creation (`init`)**: the zero-case KB — entry point, `.index/`, readiness stamps, refresh over the skeleton, the plain-subject birth commit | `cmd` (the verb), reusing `kb`'s renderers, `atomicfile`, `kbdocs.Stamp`, `index.RefreshReporting`, `write.LockKB` and `ledger` | Landed: refuses a multi-line `--title`, outside a worktree and at any existing `kb-root/` before any write; the spine bytes come from the canonical frontmatter renderers; the entry point is stamped before the loader opens it and refresh re-stamps at its save (§10); `verify` over the born KB is `test-integration-init`'s, not the verb's; the birth commit's subject is outside `ledger.Prefix`, so §5's fresh-vs-resume reads the trail as empty |
| **Scoring (`kbase score`)**: the closed text one call carries and its guard, the runner, the run's ledger | `internal/score` | Landed: the set is `query.ScoringSet`'s, the ledger stands under the build's state directory (`internal/statedir`, extracted from `internal/build`), and the verb is `cmd/score.go`, excluded from MCP |

**Write API.** `render` composes every metadata byte, so no op accepts a preformatted heading,
marker or bullet, and no op renders a scoring bullet without its `(bearing …)`: the token is
`kb.RenderBearingAnnotation`'s, and a bullet `render` composes always carries one. One op writes a
scoring edge's judgement — `set-bearing`, values `id` (the source), `target`, `relation` and
`bearing` — and one writes a node's local value, `set-rigor`, values `id` and `rigor`; a claim's, an
analysis's and a work's local value are one register field, `- rigor:`, and an experiment's is its
host leaf's `rigor:`. `set-bearing` replaces the token at the edge's authored home — the target
entry's `rests-on` bullet for `supports` and `founds`, the source's pair line for `lifts`, the
source claim's `refutes:` pair for a claim-sourced `refutes` — and never inserts one: a missing token
is a load fault (SPEC §10) before the op runs, and a splice that finds none is `spliceFailed`. It
refuses a pairing `kb.BearingLegal` rejects, a relation outside the four, and a value outside the
domain or the pending literal. The read-back carries the edge whole: `expectedEdge` holds `Source`,
`Target` and `kb.Bearing`, `edgeOf` copies the bearing, and `edgesEqual` compares `Value` and
`Pending`, so a written `0` re-reads as `0` and a written `*pending*` as pending. No build path
writes a judgement: `add-build-edges` writes `(bearing *pending*)` on every premise bullet it adds
and no number anywhere, so a claim a build attaches an edge to reads unchanged until that edge is
scored.

Input, idempotence (I6) and the trailing refresh are SPEC §8; byte equality is
SPEC §3. Two ops are build-only:
`add-build-edges` (`write.AddBuildEdges`), which writes `add-rests-on`'s lists and the `demoted`
list, and `demote-claim-edge` (`write.DemoteClaimEdge`), which turns an existing `supports` edge
into a `demoted` row with the origin the build computed. Both are absent from `write.Ops()` and have
no subcommand, so the public write surface cannot create a `demoted` edge. The build's cuts reach the
register through `demote-claim-edge`, from the ring row at the tail of the dependency-attribution
stage; `add-build-edges` keeps its `demoted` list in the op vocabulary, and no build caller passes
it. `resolve-demoted` is a public op.
`index.DependsGraph.Path` is the path finder over the acyclicity check's graph; the check and
`resolve-demoted`'s cycle refusal both use it.

**The node vocabulary.** `kb.NodeKinds` is five words — `claim`, `analysis`, `experiment`, `axiom`,
`work` — and nothing else is a `node_type` or a `target_kind`. `kb.BearingLegal(relation, sourceKind,
targetKind)` is the one predicate answering whether a pairing may carry a bearing: every scoring
pairing may, and `references` and `demoted` may not; it answers that and never whether an edge may
exist. A node id carries its kind's three-letter prefix — `clm-`, `ana-`, `exp-` and `wrk-` over a
minted or authored body, `axm-<label>` over the label as authored, which no minted id's body grammar
governs. The axioms are one document: `axioms.md`, read by the one `### Axiom <label>: Title`
heading pattern, a declaration's id `axm-<label>` and its anchor the heading's slug. `kb.AxiomSource`
answers `axioms.md`, else a legacy `invariants.md`, else `AGENTS.md`, and the two legacy documents
are read by the patterns that heading cannot match, so a KB the converter has not rewritten still
loads. SPEC §10 carries the prose legend those documents' words map through, and it is the one place
the synonyms live.

**The derived layer.** `internal/index/strength.go` is the arithmetic's authority: the four
relations, their three stations — the minimum for `supports` and `founds`, the maximum for `lifts`,
the cap for `refutes` — the pending rule and the value's two spellings live there and nowhere else.
The walk computes at full precision and nothing else rounds a strength: `FormatStrengthRecord`
writes a record's value in the fewest decimals that read back as it — the record's own spelling, and
`verify`'s messages about it — and a query result carries the value `FormatStrengthDisplay` spells,
where a trace spells it.

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
`MinDependencyStrength` is the `[= min(…)]` trace's operand, multiplying every participating edge by
its bearing and omitting an excluded one; `RenderStrengthTrace` composes the station the walk took.
[STRENGTH_SCORING_SCHEME.md](STRENGTH_SCORING_SCHEME.md) states that arithmetic — the relations, the
formula and the worked example — and is cited, not reproduced.

**The query layer.** `query.DepsPayload` is the one incidence adapter: it presents an incident edge
from the reader's side — `relation`, `direction` (`in` where the other end supports `id`, `out` where
`id` supports it), `target`, `bearing` (the index's number-or-literal text, null only where the
pairing carries none), `context` — in the fixed relation order and then by the other end's id.
`DependentsOf` reads the push direction: the targets of an id's outgoing scoring edges. No query verb
returns scoring text: the index carries no statement body, and `query.Page` accepts list answers
only, so no passage ever becomes a query answer.

**Frontmatter reading.** `internal/kb` takes a leading `---` block as frontmatter only when its
first non-empty line is key-shaped (`name:`, kebab-case); a block that then fails to parse as a YAML
mapping is the malformed-frontmatter refusal, and a block whose first line is not key-shaped is
prose, the document having no frontmatter (SPEC §10).

**Sheet.** `internal/sheet` emits Graphviz DOT and renders it by running `dot -Tsvg` on `PATH`; the
SVG's bytes are Graphviz's, so the goldens pin the DOT, and a test that renders through `dot` skips
without it and asserts only that the DOT parses and the root fits the viewer. The goldens are a
fixture and its three DOT files under `internal/sheet/testdata/adjagent-sheet/`, copied as they
stand: a `2.0.0` KB, its unmarked build record, and the DOT the sheet composes for it. The emitted
DOT matches them byte for byte. Its inputs are the index (claims, edges, strength bands); the build
records beside `kb-root/`, read through `internal/buildrecords`
that `claimgraph` and `sheet` both import — only the unmarked record and the cull record's `links`,
for each edge's provenance (marked, or inferred by SPEC §3's rule), through
`buildrecords.InferredLinks`, the rule's one implementation, which `claimgraph`'s cut origin calls
too; any record's absence finds nothing; the index's `demoted` rows, for the cuts and
their origins; the leaf, for a node's kind, with the title deciding where no marker is readable; the
entry point's first heading for the title and the `kb-root/`-level cluster; and each volume's
`index.md` first heading, falling back to the directory name, for the volume's cluster title. It
produces the full sheet always, and the volume digest and one sheet per volume where two or more
volumes hold nodes. Drawn content and styling are SPEC §3. It renders through at most four `dot`
processes at once (`sheet.renderConcurrency`, §12), keeping output order. Where `dot`'s spline
router fails on a dense sheet, the emitter composes the same DOT with straight-line splines and it
is rendered once more; where both fail, the first error is reported. Without `dot`, it renders
nothing, leaves existing sheets as they are, and yields the placeholder for a root that has no
sheet.

`refresh` — and so every caller of `index.Refresh`: a write op's trailing refresh, the build's gate
— and `render-claim-graph` reach it through one seam, `index.WriteSheet`, which renders every sheet
on each call and writes one only where its bytes change, with no ownership test on an existing
sheet. `verify` never reaches it.

**Scratch creation.** `cmd/init.go` is the only producer of a KB that is not the build. It composes
existing mechanisms and owns no new one: the entry point's frontmatter through `kb`'s canonical
renderers (`RenderFrontmatterField`, `WrapFrontmatter`, `StampFormat`), every file through
`atomicfile.Write`, the loader's `Open` (I8), the readiness stamp through `kbdocs.Stamp` with
`kbdocs.AuthoredKBPin` in its scope-pin slot — its `written` list is the paths `Stamp` returns —
the derived layer through `index.RefreshReporting`, the lock through `write.LockKB`, and the birth
commit through `ledger.Commit` — the plain-subject half of `ledger` extracted beside `Record`, its
pathspec scoping the same so nothing the user staged rides along. Its order is load-bearing: the
stamped entry point stands before `Open`, the stamps stand before the refresh (whose precheck
refuses without the redirect), and the refresh stands before the commit. It runs no `verify` of its
own: every input to the skeleton is kbase's, so a red verify over it is a kbase defect, which
`test-integration-init` catches by running `verify` right after birth.

### Scoring

`kbase score` is SPEC §8's; this states the mechanism. Five packages and the verb carry it:
`internal/score` (the runner, the closed text and its passages, the parse, the run's ledger),
`internal/kb` (the fault vocabulary, the two weight tables and the price), `internal/asks` (the two
templates and the check that they name their pass's ids), `internal/query` (the set),
`internal/statedir` (the state directory) and `cmd/score.go`.

**The runner.** `score.Runner` owns the loop and no I/O of its own. It takes the enumerated items, an
`Ask` seam, a `Write` seam, the ledger, the model name the lines record, `Limit`, `DryRun` and a
logger — and, because the text is composed in that package, the `kb.Source` and `query.Index` the
renderers read. Per item it composes the closed text, asks, parses the reply as the charges it
records against that text, prices them, writes and appends `done`; a text the guard refuses, a reply
the parse refuses, or a call or write that fails appends `failed` with nothing written, and the loop
continues. Items are walked one at a time, in the set's order, with no concurrency. `Counts` is
computed from the ledger's last status per item, not this invocation's attempts, so a resumed run
reports the whole run. A cancelled context stops before the next ask and records nothing for the item
in flight.

**The two back ends are one seam.** The runner's `Ask` takes the rendered prompt whole and returns
the reply, so the runner has no view of a provider and none of a harness: `internal/score` imports
`asks`, `kb`, `query`, `result`, `atomicfile` and `log`, and reaches neither `model` nor `harness`.
`cmd/score.go` fills the seam twice — `model.Chat` on the scoring role's endpoint with an empty
system prompt, or `(*harness.Command).Ask` on the profile's argv — and the runner learns nothing of
which. Each score template splices the `score-system` fragment into its own text, so the instruction
is carried once, in the user turn.

**The prompt shelf.** Two templates, `score-rigor.tmpl.md` and `score-bearing.tmpl.md`, each with the
one `dyn.` slot the runner fills with the assembled text, and one shared fragment,
`fragments/score-system.tmpl.md`, registered in `fragmentSlots`; `asks.ScoreRigor` and
`asks.ScoreBearing` render them through `asks.Render`. Provenance is the shelf's rule: embedded in
the binary, written by kbase, no upstream origin and no version pin to drift. The templates carry the
criteria and the line form; they carry no id the pass does not accept, no weight, no maximum, no band
and no number — the price is `kb`'s and one fact lives once. A test asserts each template declares
one `dyn.` slot, fills it with the text the runner composed, splices the shared fragment, and names
exactly its pass's ids (`flt-[a-z0-9-]+` tokens against `kb.RigorFaults.IDs()` /
`kb.BearingFaults.IDs()`); the shelf's `asks.Lint` covers both files as it does every template.

**The closed text and its guard.** `internal/score/text.go`'s two unexported renderers assemble every
character a call may see. `rigorText` renders a node's title and its statement: a claim's statement
is the block its tier-2 marker sits in, in the leaf the index records as declaring it; an analysis's
or an experiment's is the section at its canonical anchor in the leaf its `CanonicalPath` names. A
claim no leaf marks renders its title alone, and a work renders its title alone — its register
entry's body is its `### Rigor` subsection, which the guard would refuse, and its `- rationale:` is
never sent. A claim's `CanonicalPath` names the register, which is never opened. `bearingText`
renders the source's and the target's statements by that same rule, then the edge's authored
`[context]` from the row `Index.Edge(ScoringEdge)` answers. `scoreTokenRE` and `frameRE` scan each
assembled passage — no guard pattern spans the blank line between two — and a hit is
`scoreTokenError`, naming the document and line, with the item `failed` and no call made: the guard
refuses, it never strips. The four scoring tokens are half of what it refuses; `frameRE` adds the
frame's own structure — a run of twelve backticks closes the fence the templates draw around the
corpus, and what follows then sits outside the instruction to judge it, and a line opening
`## Source:`, `## Target:` or `## Context` forges the frame's map of the text; a label a blockquote
or an indent displaces from the line's start opens nothing. Neither half is adaptive to the text:
a rendered prompt stays a function of its template, so a text carrying the frame's structure is
refused rather than escaped. An
out-of-scope subject is refused by name and type — a `wrk-` or `axm-` id on `rigorText`, a
`references` or `demoted` relation on `bearingText`.

**The parse reads the closed text's passages.** `assemble` returns the closed text *and the extents
of the passages it is made of*, because a charge's `passage` and its `offset` are facts about that
text: a `rigor` call's two passages are a node's `title` and `statement`, a `bearing` call's are the
`source`'s and the `target`'s. `score.ParseAnswer` (`internal/score/answer.go`) reads a reply against
them and the pass's `kb.FaultTable`: a charge line is looked up by collapsed whitespace and folded
case, **one passage at a time**, so a span crossing from one to another is unfound and `malformed`,
and the edge's `## Context` — assembled but not a passage — is citable by no charge. A span the two
passages carry alike records `both` at its earlier match. The read yields `Answer{Faults, Moot,
Value}`: the charges, the charge lines a `flt-pending` made moot, and the price `kb` gives the ids.
Duplicate and overlap collapsing is the parse's; the ids the ledger records are the whole input of
the price, which is what makes a landing checkable and a run re-scorable without a model. The
context stays in the assembled text because the ask judges an edge on it; the parse merely refuses to
anchor a charge there.

**The scoring set.** `query.ScoringSet(ix, pass)` reads the loaded index in-package and returns
`[]query.Item`, each carrying a `ScoringNode` or a `ScoringEdge` and never both, in the walk's order;
`query.PassRigor` and `query.PassBearing` are the two spellings the set and the `--pass` flag share.
An item carries identity only, so an edge's row comes from `Index.Edge` and no caller builds a
`kb.EdgeRow` by hand. The set has no verb: it is consumed in-process, and `--dry-run` is the
inspection surface. Which nodes and edges are in it, and which are excluded, is SPEC §8's.

**The ledger and the state directory.** `score.Ledger` opens `<state-dir>/score/<run>/`, writes
`run.yaml` through `atomicfile.Write`, appends each attempt as one JSON line with `O_APPEND` and
`Sync`, and reads back the last entry per item — its status and the evidence behind it — so `--resume`
skips what landed and a failed run names each failed item's reason. A line is appended after the item's
write and validation, never before: a crash between the two re-asks a write that is idempotent, where
the reverse would skip a value that never landed. `Create` never reuses a run directory: the run id
is minted past any that stands, and a directory that appears under it fails the call. `Resume`
refuses a `run.yaml` naming another KB root, format or pass, and drops a tail torn after its last
newline so the item it names is asked again. `items.jsonl` holds the run's set, which is all a dry
run writes beside `run.yaml`.

A line carries `faults` — the parsed charges, `id`, `passage`, `citation`, `offset`, in the order
written — and `moot`, the charge lines a `flt-pending` made moot, parsed the same way and priced
nowhere; `[]` is written for one reply alone, the word `none`, so an empty list always carries the
model's positive statement that it found no fault. A failed attempt carries neither, and no text:
its `reply` is the reason. `backend`, and under a harness `harness` and `harness_layer`, sit on every
line, because a resumed or mixed run's lines must say what answered each item. `Create` also stamps
`ask_sha256`, from the header's `Pass`, as it stamps the format — one site, so no caller can open a
run without the ask its judgements answer, and a render that fails is a defect of the build rather
than a run written with no digest. `Resume` reads the header's digest and compares nothing against
it: the digest is a record of which instruction answered which judgement, and an update applies its
behaviour. `reprice` (below) is `Resume`'s other half.

The state directory is `internal/statedir.Dir`'s — the override,
else `$XDG_STATE_HOME/kbase/<key>`, and never inside `kb-root/` — extracted from
`internal/build/store.go` so that a run and a build agree on what the state directory for a KB is;
the build store's own format stamp stays in `internal/build` (§8). The run lock is
`filelock.Acquire` on `<state-dir>/score` with no wait — the runs' directory stands before any run
does — so a second run is `retry`, `check: lock`, `path` that directory.

**The one-op landing.** `landScore` calls `write.Run` with `set-rigor` for a node and `set-bearing`
for an edge — values `id` and `rigor`, or `id` (the source), `target`, `relation` and `bearing` — and
`NoRefresh: true`, under the KB write lock, which `holdKB` takes for the landing and releases before
the next item. A `done` or `unchanged` write lands; a refusal or a failure is returned as the item's
error and recorded `failed`, and so is a lock `holdKB` could not take — the item's line carries the
detail of the item `holdKB` answered with (`retry` past the wait, `refused` at once while a build
runs) and the run goes on. The verb refreshes once after the loop where anything landed, so a run's
values are published by that one refresh and no refresh rewrites the KB per item.

**The re-price.** `Resume` takes the same `Write` seam a landing is opened with, and `reprice`
re-derives every landed line's value from the ids its line records, under the running pass's
`kb.FaultTable`. A value the running table gives differently is landed through that seam first —
`set-rigor`/`set-bearing`, under the KB lock as a landing holds it — and the line records it after,
so no line claims a value the KB did not take: a refused write fails the resume with the ledger as it
stood, and the next resume re-derives the same move, the write being idempotent. Each moved line is
listed in `repriced.yaml` in the run's directory — the run, the time, and per line the item, the
value it held and the value it took — which is where a re-price is recorded, `run.yaml` being written
once. `Ledger.Repriced()` says how many values moved, so the verb's closing refresh can follow a
re-price as it follows a landing. No model is asked: the ids are the whole input of the price, and
the model never saw a weight.

**The verb.** `runScore` is the body: the pass, limit and dry-run/resume checks; the KB root; the
state directory; the back end and the scoring role resolved before the KB is opened — the mode
decided, or the profile selected and its command probed — `config.HarnessArgv` refusing there a model
id beginning with `-`, because the profile's command would read it as an option — so a setup refusal
writes nothing and mints no run; `openKB` and the migration record; the run id; the run lock; one
refresh through
`holdKB`, skipped under `--dry-run`, which releases the write lock before the walk; `query.Load` and
the set; then either the dry run's records and an inert `Runner`, or the registry's client (none
under a harness), the one `Ask` the back end needs and the walk; `counts` and `scored`; `resume`
where any item is not `done`, ahead of the end refresh and of the record the outcome closes on; then,
where anything landed or a resume re-priced, the KB re-read and the end refresh
— either refresh ending the run where it cannot take the KB write lock, with `holdKB`'s own outcome
and its `lock` item — and last, `failures` on `failed` (one `check: error` item per item the ledger
records failed, detail its name and its line's reason, or one item naming the error a refresh stopped
on) or `cancelled` on `cancelled` (the walk's first item with no attempt, `{}` where every one has
one). The refresh cadence, the closing records and `resume`'s reach are SPEC §8's. Registration is
`mcpBinding(scoreCmd, mcpExcluded)` (SPEC §11): the command is bound with the excluded annotation, so
it appears in no `tools/list` and carries no read-only annotation.

---

## 7. Inference seams

- Kept from kbase: provider management (`internal/config`) and the client (`internal/model`). The
  call path is `model.Chat` for the overview call and for each scoring call, and `model.Decision`
  for the letter asks; the decision role's model is an id on whichever surface its entry speaks
  (`decisionsFlavor`, SPEC §9.1), and kbase's three roles apply — letter asks go to the `decision`
  role, the overview call to `text`, and `kbase score`'s asks to `scoring` (SPEC §9). Each role
  names its own endpoint and its own model, in `[models]` as a `<provider>/<model>` reference or in
  the environment by `KBASE_<ROLE>`; no role falls back to another.
- Every model call is one tool-less chat request answered from its prompt alone: temperature 0,
  thinking disabled (`chat_template_kwargs.enable_thinking: false`), the usage chunk requested; a
  call that does not complete is re-issued identically up to the attempt count (§12).
  `model` makes it, with a system message. This holds for the overview call; the letter asks take
  the decision surface instead (next item). A build with a calling row left to walk checks a
  provider is configured before its first stage (SPEC §5).
- Configuration is read from layers and merged once, before a verb's own work: `config.Layers`
  resolves the directories and `config.Load` reads and merges the three files of each, returning one
  `Loaded` — the pool and its faults, the harness profiles, and the choices. They are
  `<repository>/.kbase/` over `~/.config/kbase`, or the one directory `--config-dir` /
  `$KBASE_CONFIG_DIR` names — `explicit` and `environment` in the source vocabulary — so an
  explicit choice isolates a run from a repository's own `.kbase/`, which every test and pinned
  deployment needs. The caller resolves the repository root with `kb.RepositoryRoot(workDir)`, which
  answers `""` outside a worktree, so `internal/config` takes no git dependency and reads the global
  layer alone there. A pool entry and a harness profile are each replaced whole by a higher layer's
  of the same name; a `config.toml` field is overlaid one at a time. `config.Load` drops a fault
  naming an entry a higher layer already supplied — the pool's entry is the higher one, so the fault
  describes a value nothing reads — and keeps a higher layer's own fault, whose entry never entered
  the pool. `config.Layer` carries a directory, its source (`explicit`, `environment`,
  `repository`, `global`) and — once `config.Report` has probed it — whether the directory and each
  of the three files stand, `HarnessesExists` beside `ProvidersExists` and `ConfigExists`.
  `Provider.Layer` and `Config.ModelLayer` record the layer the merge took each pool entry and each
  role's value from; `Profile.Layer` records the layer whose file supplied each profile;
  `ProviderFault.Path` names the `providers.toml` a fault came from, and a `Refusal`'s path names
  the file a lacking setting belongs in — the write target's `providers.toml` or `config.toml` — so
  a layered configuration says which file to edit.
- **Resolution is three pure functions, and connection is the registry's.** `config.ModeOf`
  (`internal/config/resolve.go`) decides once per invocation which source supplies inference; it
  reads the loaded configuration and the environment's strings and nothing else, and its refusals
  are the mixed sources, the partial environment and the environment beside a standing `[models]`
  value. `config.Resolve` answers one role's `Endpoint` in that mode, or under a harness with no
  endpoint at all; it reads no file, opens no connection, writes no stderr and names no verb — the
  key file `KBASE_API_KEY_FILE` names is opened by the registry, and by `show-config`'s preflight,
  never by resolution. `config.Select` is the catalogue verbs' choice of a pool entry to dial — a
  flag, else the pool's only entry — and no role is chosen that way. `config.Refusal` (with
  `Refusals` for a decision that has more than one side to state) is the one typed refusal, and its
  `Named` is what `internal/build`'s launch refusal reads. No other site applies a selection or a
  resolution rule, and `cmd/provider.go`'s `clients` type is the only constructor of a
  `model.Client`: one client per endpoint, opened on first use, keyed by the entry's name or — in
  environment mode — by its base URL, and closed together before the verb returns on every outcome.
  The report's other facts — the pool's entries and faults, the files' presence, `config.toml`'s
  values, the harness profiles and the inference variable names the process sets — come from
  `internal/config.Report`, which reads no file and no environment of its own: the caller passes the
  existence probe and `getenv`.
- A letter ask is one `POST` to the entry's decision surface (`model.Decision`; `decisionsFlavor`,
  SPEC §9.1), one `noul` or `choice` question asked and the answer read by the key it echoes — no
  reply text parsed. Two shapes stand behind that choice. `systemone`, what an entry silent on the
  flavour gets: `model`, `state`, and `questions` a JSON object keyed by the question key, each
  question's `criteria` carrying the per-letter meanings the templates' bullet lists used to carry;
  the `answers` object comes back keyed the same way. `decisions`: the state as `input`, `questions`
  an array of named questions — a `noul` question asked as a `predicate`, carrying no choices, its
  true/false guidance in `instructions`; a `choice` question carrying its criteria as `choices` —
  answered by an `answers` array, each entry naming the question it answers; a `refusal` carries no
  value and so takes the seam's re-ask-then-default path. Under either surface an answer naming a
  question kbase did not ask is refused. The request is re-issued identically up to the chat attempt
  count on a transport error or retry-class 5xx, with no streaming and no incomplete-stream path.
  The system-prompt sentence above is scoped to the overview call; a decision call has no system
  message.
- Each inference pass is built as plumbing reaching the model only through `internal/asks`' letter
  seam, injected — nothing in `claimgraph` knows a network exists. `internal/asks` holds the embedded
  templates and fragments, the composer, the caller (system fragment, per-call captures and the
  answer cache under the state store's `scratch/`), and the letter-ask seam: every claim-graph
  inference is one decision answered by one letter from a closed set the build offers, an unreadable
  reply asked once more and then defaulted to the item's mechanical draft, recorded as defaulted.
  Templates (`*.tmpl.md`) and fragments are embedded files (I7). A template change is an experiment
  with a measured result (ACTIVE_PLAN).
- Prompts are `@!slot!@` templates; a call's system prompt is the fragment named for it
  (`overview-system` — the one surviving system fragment, the overview call's), never an agent
  definition; a letter ask carries no system message. `asks.Render` builds the one user turn. No test
  asserts a template's prose; tests key on the offered letters, the slot set, the system fragment and
  the closing line's letters.
- `claimgraph` reaches a model only through the letter seam injected by `build`; the caller that
  holds `model` lives in `internal/asks/call`, imported by `build` alone, and the import-policy test
  checks the transitive graph so `claimgraph` never reaches `model`.
- No prompt originates in an in-code string.
- The overview passage is written from excerpts composed mechanically from the tree, checked for
  structure and re-asked once through the
  `overview-correction` fragment.

---

## 8. Ledger, state store and monitoring

`internal/ledger` is the only package that execs git (I1); it execs the host's git, which SPEC §2
already requires, so the user's configuration, hooks and attributes apply, and no git library is a
dependency. `internal/build` owns the stage table, the state store, resume, `--no-inference`,
`--through`, and status, progress and cancel. Only `build`
and `cmd`'s `init` verb import `ledger`.

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
  pays again. The node-pass and classification records are still written per leaf and per group, but
  resume does not read them for position. The progress record accounts for the dirt an interrupted
  stage leaves; the
  dirty-path refusal (SPEC §5) applies to dirt it cannot account for. The resume command carries
  every flag the launch carried but the bound, `--through`: resuming past it is what resuming is
  for. It is emitted only where an identical re-issue continues the work: a `bounded` run is
  handed none, being finished at the bound — re-issuing its command is not what continues it; the
  trail document is what a reader uses to reset and re-issue. A refusal that
  recorded no boundary leaves nothing to continue. It is also emitted only where the
  recorded invocation reads as a `kbase build` invocation (SPEC §6): the state store is a directory
  the caller names, so a record naming any other program is repository-supplied text and no document
  renders it for a harness to run.
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
  `status` reads liveness from it and the progress record, and `cancel` signals its holder. A clean
  exit removes the file, cancel included, so a standing lock file is not by itself evidence of a
  live holder. A second builder on the same state-dir is `refused`; where only the holder lock is
  held the result is `retry`. User-facing text calls it "holds the state store"; "run lock" names
  only `<git dir>/kbase-build.lock`. On Windows, where no advisory lock exists, `status` cannot see
  a running build.
- **State store version.** `<state-dir>/format` is one integer, `storeFormat` (§12), written when a
  build first holds the store; an absent file reads as 1. `build`, `status`, `wait`, `cancel` and the
  MCP build tool read it at open, and a store newer than `storeFormat` is refused (`check: state-dir`,
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
- **Progress units** are leaves for the node pass, source groups for `references-found`, the ask
  groups of each of its two passes for `claims-culled` (each pass announcing its own total), ask
  groups for classification, so `status` moves during an inference stage; a defaulted item is a
  `fallback` event. The node-pass, unmarked and classification records are written per leaf or per
  group; the cull record once, after the culls are issued.
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
- **Status, progress, cancel, wait.** `status` reads the holder lock, `progress.jsonl` and the commit
  trail and returns the YAML snapshot in SPEC §6. `cancel` signals the lock holder; the holder
  abandons the in-flight call with nothing written for it, logs the event, releases the lock and
  exits `cancelled`. `wait` polls `build.ReadState` — the store alone, as the watcher does — until
  the run it names is superseded or has ended, then renders `status`'s document through the same
  `build.Status`: it takes no lock and reads no commit trail until that final document.

## 9. Result emitter

`internal/result` owns the YAML result document and its emitter, and nothing else writes stdout
(I5). Fixed key order, strings quoted on emit, a refusal enumerating every offending item. `cmd/`
maps `outcome` to exit code per SPEC §7. YAML uses the package named in CONVENTIONS "Dependency
Policy".

## 10. Compatibility harness

Compatibility is judged by kbase's own readers and checks, never by a byte diff of presentation. The
hermetic gate is `just test-integration`, whose five members —
`test-integration-results-fixture`, `test-integration-init`, `test-integration-mcp`,
`test-integration-config` and `test-dist` — each judge what their process produced: the first four
run `./bin/kbase` over fixtures they stage, judging the result document against the verb's keys, the
tree it wrote, the transcript it served; `test-dist` runs `tools/scripts/build-dist.sh` against stub
`go`, `gpg`, `curl` and `uname`, judging the release layout it stages and the installer it generates
byte for byte against its golden. Each writes its log under `test-data/transient/<recipe>/`. Recipes
needing pandoc, python3 or a provider stand outside the gate and run by name —
`test-integration-mcp-build`, the MCP notification probe that builds a paper, among them.

---

## 11. Package map

The listing groups packages by role; the dependency rules below it are the whole rule, and the
import graph they allow is acyclic.

```
cmd/                      cobra root; one file per subcommand; outcome-to-exit table
internal/result           YAML result document + emitter (I5)
internal/latex/pandoc     reader, writer and defaults-run seam (I1)
internal/latex/prepass    the two source scans (I3)
internal/docgraph         JSON transforms, whole-volume render+cut, outline,
                          records, partition checks, validate_build
internal/records          the record schema (§5.5), written by docgraph, read by claimgraph
internal/kb               kb-root model: names, exclusions, link primitives, dead-link
                          gate, frontmatter and register parse, schema (the node kinds,
                          the relation names, `Relation*` and `Relations`, and the band
                          ladder, spelled nowhere else), the scoring faults' vocabulary
                          and the two weight tables (`faults.go`), `Source` (the one
                          read path), the index row
                          types (one per index file), the index file inventory and the
                          one reader `ReadIndex[T]` (`indexstream.go`), `LabelLine`
                          (the label-line recogniser), the child-list reading of the
                          tree relation (`ChildList`, `ChildListLines`), the heading
                          reader `FirstHeading`, the in-body marker predicate `IsMarker`,
                          the reference-list entry reader `BibliographyEntries`
internal/kbload           `Open(kbRoot)`: reads the stamp, builds `kb.Source` (disk at
                          the current version, `migrate.Chain` in memory for an older
                          one); the only importer of `migrate`; imports `buildrecords`
                          for the record paths (`RecordFiles()`); imported by `cmd`,
                          `build` and `write` only
internal/index            strength, aggregates, footers, .index emit + freshness,
                          citation gate, claim-id link check, the path finder
                          (`DependsGraph.Path`)
internal/write            render / values / store / ops, the build-only `add-build-edges`
                          and `demote-claim-edge`
internal/atomicfile       the one atomic file writer
internal/query
internal/sheet            DOT emitter, `dot` invocation: index, build records, leaf,
                          entry-point heading in; SVGs out
internal/export           the LaTeX export (§5.7): the walk, the inputs of `gen-tex/`,
                          the embedded filter and template, the starter
                          `preamble.tex`, the report
internal/buildrecords     the four build records' readers (node-pass, unmarked,
                          classification, cull) and their paths
                          (`RecordFiles`), imported by claimgraph and sheet, and by
                          kbload for the record paths (the ledger's owned paths derive
                          from them; `migrate.SupersededRecordPaths()` names the old
                          spellings)
internal/kbdocs           readiness stamp, README assembly
internal/claimgraph       conform, inventory (records + page-defined readings),
                          identify, attribute, shortlist, unmarked, order, cull,
                          elision (the cull's issuer), equation sites, classify,
                          equation(s), endcap, hand_named, assemble, write, gate,
                          the four build records
internal/asks             embedded templates + fragments, composer, caller
                          (captures, answer cache), letter-ask seam; the scoring
                          templates' line form, and the test that asserts the ids
                          each names against `kb`'s pass set
internal/ledger           git checkpoint seam (I1), by exec
internal/filelock         the one advisory file lock (the KB write lock, the build's run
                          lock in the git directory, the state store's holder lock)
internal/build            stage table, state store, resume, --no-inference,
                          --through, status/progress/cancel, and the store reading
                          (holder lock, progress record, the run it stands at) that
                          status's state and the MCP watcher both derive from
internal/score            the score passes' closed text, its guard and the passages it is
                          made of, the parse, the runner, the run's ledger and its
                          re-pricing; imports neither model nor harness; imported by cmd
                          alone
internal/harness          one configured command, one prompt, one reply, one process
                          group: the setup probe, per-item argv substitution, the two
                          bounds and the kill on deadline; imports stdlib and `log`
                          alone of kbase; imported by cmd alone
internal/statedir         the state directory of one KB: the override, else
                          $XDG_STATE_HOME/kbase/<key>, never inside kb-root/;
                          imported by build and cmd
internal/asks/call        the caller: the letter asks through model.Decision behind the letter
                          seam, the overview call through model.Chat; captures, answer
                          cache; imported by build alone
internal/migrate          (SPEC §10) the N→N+1 converters over the loose
                          representation and the chain; each converter returns the
                          next form plus the paths it made obsolete; no file I/O
internal/mcp              (SPEC §11) JSON-RPC framing, dispatch, the Tool, Schema and
                          Property types, the frame checks, and the one subscription the
                          caller allows, whose frames it writes under the same mutex as
                          the responses; imports `log` only. The tool table, the resources,
                          the subscription and the server version are injected by
                          cmd/mcp.go through Serve(in, out, version, instructions, tools,
                          resources, subscription, lg); cmd/mcp.go is the one self-exec
                          owner (I1)
kept: config, model (model.Chat the overview call, model.Decision the letter asks), log, version
```

**Loading.** Every KB file is read through `kb.Source`, built by `kbload.Open` (I8), so an older KB
arrives already in the current form and no verb sees the on-disk bytes of an unmigrated one. The
loader carries the conversion record: `Source.Migration()` holds the step's `From`, `To` and the one
sentence `note`, set where the source was migrated and kept across the save that stamps the current
format, so every call on an older KB until that save carries the `migration` record and writes the
one stderr sentence (SPEC §7, §10). `build` is the one loader that carries it nowhere: a mid-build KB
is not a finished state, and its stage commits own that trail. `kb.MigratedSource` carries the paths
the conversion made obsolete, and the saving refresh removes them.

**The requiredness gate** is `kb.Discover`'s, and it sits at the load of every command that builds a
state: a `rests-on` bullet or a pair line that breaks a rule yields one `kb.AnnotationError` of
`AnnotationFault{Field, Path, Line, Allowed, Remedy, Detail}` before any state exists, so no index is
written and no op runs. A premise bullet naming an `ana-` id is one of those faults: `supports` from
an analysis is no legal pairing, so the bullet yields no edge (SPEC §10).
`internal/index/refresh.go`'s refusal path maps it to `result` items with `check: metadata`, the
field as `key`, and the bullet's file and line;
`internal/write` maps it the same way, so the op that would have run is refused instead. The
informational counterpart is `verify`'s: a legacy document that still declares an axiom beside an
`axioms.md` is one `check: axiom-source` finding, never a refusal (SPEC §7, §10).

**The loader's file set and its other refusals.** A KB document that is a symbolic link is refused,
not followed: `check: symlink` at the link's path, raised by the document walk and by the conversion's
own read of the KB's files alike, so no link out of the tree is read or written through. A conversion
holding bytes it cannot carry refuses before any state is built: `check: conversion`, `path` the
file, `line` and `detail` the content (SPEC §10).

**The zero-value hazard.** A register parsed outside `kb.Source` — by a test, by a tool, or by a
command that read bytes of its own — reads a missing `(bearing …)` as the zero `kb.Bearing`, which is
the value `0.0` and a judgement: a premise contributing zero to the derivation. The loader monopoly
therefore carries a type's meaning and not only a version's, and every parse in a command takes
`Source` bytes.

**Dependency rules.**

- `docgraph` knows nothing of claims; the record schema it writes is the one `claimgraph` reads,
  defined once in `internal/records`, which imports nothing of kbase but `atomicfile`, to write its
  files.
- `claimgraph` never imports `latex/*`.
- `buildrecords` imports no kbase package but `kb`; `claimgraph` and `sheet` read the build records
  through it, so `sheet` never imports `claimgraph`; `sheet` reads the letter tables from `asks`
  (never `asks/call`), so the records' yes letters have one home.
- `kb` imports no other kbase package but `log`; `docgraph`, `index`, `write`, `query`, `sheet`,
  `kbdocs` and `claimgraph` read the kb-root vocabulary, the link relation (in its two readings,
  one resolver), the label-name grammar and the id minting from it. `export` reads the
  link relation a third way, `kb.ChildList`: an index's child-list lines only, so a body link is
  never a spine edge.
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
- Only `asks/call`, `build` and `cmd`'s `build`, `models`, `configure` and `score` verbs import
  `model`: within `cmd`, file-level, `cmd/build.go`, the client registry the other three share,
  `cmd/provider.go`, and the score verb's ask, `cmd/score.go`. `claimgraph` reaches neither `model`
  nor `asks/call`, transitively. `cmd/provider.go` is the only constructor of a `model.Client`, and
  a verb opens one only where it is about to ask something.
- `internal/config` imports no kbase package but `atomicfile`: it holds the grammar, the three
  roles, the two modes, `ModeOf`, `Resolve`, `Select`, `HarnessArgv`, the refusals, the loaded files
  and the templates, and it never imports `model`, `result` or `log`, opens no client and names no
  verb.
- `internal/harness` imports stdlib and `log` alone of kbase. It knows no provider, no endpoint, no
  KB path, no configuration type and no prompt: it receives a prompt and hands it over whole, and
  composes none.
- `internal/score` imports neither `model` nor `harness`: the back end is below its `Ask` seam, so
  the runner, the guard, the parse and the ledger learn nothing of what answered.
- Only `build` and `cmd`'s `init` verb import `ledger`; `ledger` imports `log` and nothing else of
  kbase.
- Only `build` and `cmd`'s `init` verb stamp readiness documents: within `cmd`, file-level,
  `cmd/init.go` alone imports `kbdocs`.
- `build` imports `write` (the write-lock wait); `cmd` imports `build` and `filelock` (the refusal
  where the run lock is held; the held-around-the-run write lock is `write.LockKB`).
- Only `internal/latex/pandoc` execs pandoc; only `internal/ledger` execs git; only `internal/sheet`
  execs `dot`; only `cmd/mcp.go` (file-level) execs kbase itself; and only `internal/harness` execs
  the program a harness profile names. That fifth owner's binary is the one the exec policy cannot
  name statically: it is configuration's, chosen by the user's `--harness`, so `cmd/execpolicy_test.go`
  carries it with a sentinel and exempts its site from the per-binary check. What stays enforced is
  that no other package execs for this purpose, and that the only way it is reached is `score
  --harness`. A repository may supply the profile, which is the trust a repository's `providers.toml`
  already asks for: the source is announced, never silent (SPEC §9.3), and no verb runs a profile it
  was not told to run.
- `internal/export` imports `kb` and `atomicfile`, and nothing else of kbase; the verb opens the
  KB, has `export` write `gen-tex/`'s inputs, makes the one `pandoc.RunDefaults` call, and has
  `export` merge the filter's findings that run wrote into the report.

The dependency rules are enforced by an import-policy test over the transitive import graph
(`cmd/importpolicy_test.go`), beside the I1 exec-policy test (`cmd/execpolicy_test.go`).

---

## 12. Constants

| Constant | Value |
|---|---|
| State-store key | First 16 hex digits of the SHA-256 of the absolute, symlink-resolved `kb-root/` path (SPEC §6) |
| Query default `--limit` | 50 results; chosen so a default-limit result from each list query fits within personant's 8 KB tool-result cap over the six corpus fixtures, asserted by a test |
| Transport attempts | 3 per call; letter asks re-issue at once, the overview call pauses 5 s then 30 s between attempts |
| Cached-token totals | `AskTotals.WarmCacheReadTokens` (and `CachedTokens`) report 0 on every decision call: the SystemOne surface's `usage` has no cached-token field, so the zero is the API's, not a regression |
| Reader concurrency | `[asks] readerConcurrency` in `config.toml` (SPEC §9), default 4 |
| Model roles | `[models] decision` (the letter asks, on whichever surface its entry speaks), `[models] text` (the overview call) and `[models] scoring` (`kbase score`'s asks) in `config.toml`, each a `<provider>/<model>` reference; `KBASE_DECISION`/`KBASE_TEXT`/`KBASE_SCORING` name a bare id under the environment's endpoint instead (SPEC §9) |
| Harness bounds | `harness.ReplyLimit`, `StderrLimit` and `PromptLimit`: 64 KiB each — one item's reply, the stderr kept for a failure's reason, and the prompt one argv element can carry. A reply or stderr past its bound fails that item; a prompt past its bound fails it before a process starts. `harness.waitDelay`: 5 s, how long `Wait` waits on the child's output pipes once the child or its process group is gone — a command that daemonizes leaves a writer behind (§6's Scoring, SPEC §8) |
| Metadata format version | The one version read and written (SPEC §10), `kb.FormatVersion`; the stamp key is `kb-format`. `2.0.1` |
| Pending literal | `kb.PendingLiteral`: `*pending*`, the format's one no-number literal, written for a value not yet judged and accepted wherever a value is absent (SPEC §10) |
| Conversion record | `migration`: a result's conditional record after `kb-root`, holding `from`, `to` and `note`, and the one stderr sentence the same call writes (SPEC §7, §10) |
| MCP protocol version | `mcp.ProtocolVersion`: the MCP revision the server implements (SPEC §11): `2025-11-25`, confirmed, not chosen, by the owner's live check |
| KB write-lock wait | `write.LockWait`: 30 s, how long a writer, and the build's wait-out, try to take the KB write lock before `retry` (SPEC §8) |
| State-store format | `storeFormat` in `internal/build`: 1, the integer in `<state-dir>/format` (SPEC §6, §8) |
| Builds kept | `keptBuilds` in `internal/build`: 5, the ended builds whose captured output and call captures survive pruning (§8) |
| Run lock path | `<git dir>/kbase-build.lock` (SPEC §6, ARCHITECTURE §8) |
| Detached-build start wait | `build.StartWait` = `write.LockWait` + 10 s (40 s): how long the `build` tool waits for the child's progress record to show a stage entered, or for the child to exit, before returning; derived so a build waiting out a write is not reported as stuck (SPEC §11) |
| MCP build watcher interval | `buildWatchInterval` in `cmd/mcp.go`: 1 s, how often the watcher reads the state store while `kbase://build` is subscribed, after the reading the subscription itself makes (SPEC §11) |
| `wait` poll interval | `waitPoll` in `cmd/wait.go`: 1 s, how often `kbase wait` reads the state store while the run it names progresses (SPEC §6) |
| Sheet render concurrency | `sheet.renderConcurrency`: 4, the most `dot` processes at once; output order is kept (§6) |
| Unmarked shortlist, proximity slots | `shortlistProximityK` in `internal/claimgraph`: 3, the nearest targets in its own paper `references-found` asks per source (§4) |
| Unmarked shortlist, wildcard slots | `shortlistWildcardK` in `internal/claimgraph`: 2, the best-ranked targets by cosine it asks per source besides the proximity picks (§4) |

Exit codes and the placeholder's digest form are in SPEC §7 and §3; the pandoc range is SPEC §2; the
stage-commit subject is SPEC §6.

**Format migration (SPEC §10).** Every verb that reads a KB — `refresh`, `verify`,
`render-claim-graph`, the queries, the write ops, `export-latex`, and the build's spine-seed, phase-3a, readiness
stamp, overview and resume — calls `kbload.Open(kbRoot)`; `status` and `cancel` do not load the KB.
`Open` reads `kb-format` alone from the entry point (`kb.FormatStamp`, either frontmatter
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
`0.9.0` KBs.

**MCP server (SPEC §11).** `cmd/mcp.go` builds the tool table from `rootCmd`: one Tool per command
declaring `kbase.mcp` = `bound` — `mcp` and `wait` declare `excluded`, the latter because a held call
would spend the turn the build takes, and cobra's own `completion` and `help` declare nothing and
are skipped with them — plus the one listed tool below. A tool's
schema is derived from the command's `Use` line positionals and its flags. A command's read-only
standing is the cobra annotation `kbase.readOnly`, read when the table is built.

`rewind-build` is the one tool no command stands behind (I9), so it states for itself what the
derivation would have supplied: a hand-written schema (`target` and `state-dir`, both optional), the
document key order SPEC §11 gives it in place of a subcommand table's, and the schema property name
as its refusal key in place of a flag spelling. It reports the build's commit trail and mutates
nothing, which is what makes the exception affordable — the derivation buys a schema from flags, one
implementation and one test path, all of which an operation listed by hand would forgo.
`TestMCPToolsAreTheCommandTree` derives its expectation from the annotations plus that one name and
asserts the tool set for equality, so a second listed tool fails; `TestMCPRewindBuildIsTheBuildTrail`
calls the tool and reads the resource through the server's handlers, and requires the text to agree.

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
key) are JSON-RPC `-32602` errors, not refusal documents. `Serve` takes the server version, the
instructions `initialize` reports, the tool table and the resources from its caller, and imports
`log` only. `resources/list` and `resources/read` serve the resources cmd/mcp.go supplies: the
sheets, under `kbase://sheet/<kb-root-relative path>`, and the model guidance, at
`kbase://model-hints`, which is `internal/config`'s rows rendered as markdown and needs no KB.
Beside them `kbase://build-trail` is `internal/build`'s `Rewind` document (`rewind.go`) for the
repository the KB sits in, no target named: the same text `cmd/mcp.go`'s `rewindDocument` gives
the tool, because one function serves both — the resource is discovery, the tool is invocation —
and `TestMCPRewindBuildIsTheBuildTrail` asserts `resources/read` returns the tool's document byte
for byte. It is KB-scoped: `List` offers it whether or not a repository or a boundary is found,
and a repository the read cannot open is refused in the document, not raised as a list fault
(`TestMCPModelHintsResource` lists it with no KB in sight; `TestRewindOutsideAWorktree` is the
document the read then returns). An unknown URI is `-32002`, a missing `uri` `-32602`;
`initialize` advertises `resources`, and its `subscribe` once a client may hold one. A fourth
resource, `kbase://build`, is stated below with the build tool whose launch it follows.

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
| Entered a stage | `done` with `kb-root`, `state-dir`, `pid`, `resume` and `wait` (`resume` only where the record's command reads as a `kbase build` invocation, SPEC §6; `wait` the run's background command, `build.ShellJoin` of the executable with `wait --state-dir <store> --pid <pid>`) |
| Exited first | The child's own document; `isError` from its outcome |
| Neither | `failed`, an item naming the state directory and that the child is still starting |

Before starting, the tool runs `status`'s preconditions: no worktree, or a `--state-dir` inside
`kb-root/`, returns the refusal `status` gives and starts no process.

`kbase://build` (SPEC §11) is the build's state as the `status` tool reports it, for the store the
tool last launched: `cmd/mcp.go` records the store `build.Locate` resolved and the pid the launch
started, so a later launch retargets the resource and the watcher, and a process that launched none
follows the served repository's default store, which `build.Locate` resolves once because resolving
it asks git. The resource's document is the verb's: `statusDocument` renders `build.Status`'s
outcome and fields as the verb does, so a read costs the `git log` a `status` call costs. The
watcher's reading does not: it is `build.ReadState`, the store's holder lock and progress record and
the state they derive, behind the same derivation `status` reads. `internal/mcp` starts the watcher
when a client subscribes to the resource and stops it on the unsubscribe or on `Serve`'s return; it
reads at the subscription and then every `buildWatchInterval` (§12), and announces each run it reads
as over — once, keyed on the run's pid and start stamp: a resumed build is a new run, announced
once. A run already over at the subscription's first reading is announced there, after the
subscribe response, but only for a run the launch recorded or where this process launched none: a
launch is not atomic, and until the child takes the holder lock the run before it reads
`interrupted`. A run that ends between two readings and is superseded before the next is missed —
inherent to polling, which sees identities and not transitions. The request loop and the watcher
both write frames, so every write takes one mutex.

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

**Slice-1 checks:** identical tree path set and child order; equal per-leaf word streams under the
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

**A judgement and its absence read alike.** An entry with no `- rigor:` line and one written
`*pending*` are the same value on read, and a refresh restores neither, so a register records the
pending state and never that nobody looked at it. A migrated `1.0` and a judged `1.0` are one record
in the same way: the `migration` record is the only mark of an unscored corpus and the first save
ends it. Nothing records who judged a value either: `set-rigor` and `set-bearing` write a number or
`*pending*` and no author, so a value carries no attribution at all.

**A `founds` bearing on a work names its finding in prose.** The `[context]` on the bullet says which
theorem, table or result the bearing judges, and no id, range or hash joins the two, so no check can
prove the judgement was about that finding. An axiom's `founds` edge needs no such prose: its source
is the axiom's own id.

**The reader sees list bullets, and one bearing per bullet.** A `(bearing …)` is found anywhere in a
line the reader has already taken as a `rests-on` bullet or a pair line, so a field-shaped phrase in
a bullet's title is read as the field. A line that is no bullet of such a list is not read at all: a
bearing on it is neither refused nor kept. Inside a bullet, a misplaced token is R4 and a second one
R5 (SPEC §10).

**The conversion moves no edge's direction into the bytes.** The register stays claim-centric and the
push direction is the index's, which a refresh derives; a converter that wrote the direction into the
documents would carry a second model of the same edge. It records no history either: the value
changes a conversion makes are re-derived from the same claims, and nothing says which of them rose
or fell.

**No format sits between two majors.** `migrate.Chain` walks converters version by version and
refuses a step no converter leads, so a KB stamped `1.1.0` is refused at load rather than read; no
`1.1.0` converter exists and none is reachable.

**The reference comparison is flag-gated and partial.** `internal/index/compat_test.go`'s
comparisons run only when their `reference.*` flags are set, and they cannot run at all until the
reference toolchain reads `2.0.0`. The `0.9.0` parity cases in `internal/kbload/parity_test.go` are
three committed KBs plus whatever local reference clone lies under `test-data/transient`; none of the
committed ones carries a register, so a `0.9.0` register is compared by the converter's golden pairs
alone.

**No refresh moves a value.** `refresh` asks no model, so a derived value changes only when an
authored one does — `set-rigor`, `set-bearing`, or a hand edit. `verify`'s drift is between a record
and its derived surface, never between two judgements.

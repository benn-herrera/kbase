## 5. Reader and transforms

**Pattern:** Pandoc as the parser; Go transforms over its JSON AST.

pandoc parses and kbase never does. `docgraph.Build` converts each top `.tex` file (one volume) in
this order, then writes the trees:

1. `prepass.Apply` scans the source text ([§5.2](#52-pre-pass-registry)).
2. `pandoc.ReadLaTeX` parses it to the JSON AST ([§5.1](#51-pandoc-seam)).
3. `readVolume` walks the AST once and derives the outline and the records ([§5.5](#55-records)).
4. `transform` rewrites the AST ([§5.3](#53-json-transforms)).
5. `pandoc.WriteGFM` renders it ([§5.4](#54-writer-route)).
6. `buildTree` splits the rendering into documents and `place` places the records in them.
7. Five checks compare the products ([§5.6](#56-tree-derivation-partition-checks-and-validate_build)).

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
  reaches pandoc ([§5.7](#57-latex-export)).

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
emits records ([§5.5](#55-records)), which a filter cannot. The transforms are:

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
check settled it ([§13](#13-slice-verdicts)): every load-bearing form the slice checked is produced
under it.

### 5.5 Records

**Pattern:** Sidecar metadata stream.

Alongside the Markdown leaves, Go emits records (JSONL) into the build state store
([§8](#8-ledger-state-store-and-monitoring)), never into `kb-root/`: `records/docgraph.jsonl`, and
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
([§8](#8-ledger-state-store-and-monitoring), `reports/`). A `FAIL` finding makes the outcome
`failed`; a `FACT` finding describes state and never fails the stage. kbase runs no fix loop.

### 5.7 LaTeX export

**Pattern:** Pandoc multi-file conversion with a Lua filter, a custom template and a manifest-tracked
output directory.

The mechanism of `export-latex` (SPEC §8 `export-latex`), which defines the files of `gen-tex/`, the
content rules, simplification and idempotence. The export emits no records, so pandoc's own filter
mechanism suffices where [§5.3](#53-json-transforms) uses Go, and the author can re-run the
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
id, written by Go; a volume's index has `vol{N}`. A document with no heading opens with one naming
its path (the file name less its extension; the directory name for an index). Headings are used
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

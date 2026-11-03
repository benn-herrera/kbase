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


### The `3.0.0 → 4.0.0` migration

One converter reads a `3.0.0` KB's record files through the `3.0.0` load and writes them in the
`4.0.0` form. A `3.0.0` KB whose record files the `3.0.0` load refuses is refused with that load's
own items, `check: metadata`, one per fault. Every other file is kept as it is. The conversion's
steps:

- **Records.**
  - An experiment's `status: pending` is written `planned`.
  - Each `no-edge` and `strengthen-by` field is dropped, one warning per field naming the node's id
    and quoting its text (a claim's `strengthen-by` items joined with `; `), listed `no-edge` or
    `strengthen-by`.
  - `judged-hash` is written all zeros on every scored record, `manually-scored` kept.
  - Each work's and axiom's id is minted from its `3.0.0` id alone, the same id on every load and in
    `update-format`; its key or label is written as its `label`; every `3.0.0` work or axiom id in
    an edge's end or a prose field is renamed to the minted id; one warning per id is listed `ids`
    with the `3.0.0` id and the minted one. A bare id is read in the `3.0.0` grammar and cited under
    its minted id.
  - Each `### Axiom <label>:` heading of `axioms.md` naming an axiom record ends in that axiom's
    marker; one naming none is reported and left as written.
- **Markers.** Every statement marker in a `kb-root/` document is rewritten in the `kbase-id` form,
  attached with no space. A marker naming two ids is written as two abutting markers. An analysis
  marker alone on the line directly after a line ending in the marker of a claim it lifts is moved
  to directly after that claim's marker, its emptied line removed.
- **Register pages.** The `3.0.0` register pages, `claim-rigor.md` at `kb-root/` and directly inside
  each of its directories, are obsolete and reported under `removed`; the save renders the summary
  pages.
- **Citations**, resolving every anchor under `3.0.0` slugging:
  - Every link into a heading, a citation or an ordinary link, is rewritten to the heading's
    `4.0.0` anchor.
  - The node id is added as the title of each link whose anchor resolves to one node's statement — a
    heading's section holding that node's statement marker, an axiom's heading, or a `3.0.0`
    register entry, whose link is pointed at the node's place — and a work's citation is pointed at
    its entry on the root summary page.
  - Each bare node id in prose, and each link whose text is one, is made the node's citation in the
    naming form, one warning per conversion listed `bare-ids` with its path, line and id.
  - Each link or bare id it cannot resolve to one node without guessing — a section holding several
    nodes' statements, a register anchor several entries share, a heading the anchor names under
    neither slugging or names one heading under `3.0.0`'s slugging and another under `4.0.0`'s, a
    node with no place to point at — takes no id, one warning listed `unresolved-citations`.
  - A heading's `4.0.0` anchor is read after its own bare ids become citations.
  - A link into a removed register page never stays aimed at it: where every entry its anchor names
    is stated in one section it points at that section; otherwise at the summary page replacing the
    register page, without the anchor; and where no summary page replaces it, the link is removed
    and its text remains, a bare id in that text converted as any bare id is.
- **Build records.** Each build record at the repository root (`kb-build-node-pass.yaml`,
  `kb-build-classification.yaml`, `kb-build-unmarked.yaml`, `kb-build-cull.yaml`) is moved into
  `kb-root/.build/` with `git mv` by `update-format`, which refuses while a record it would move has
  uncommitted changes (`check: dirty-paths`, naming it). Its root path is an obsolete path.


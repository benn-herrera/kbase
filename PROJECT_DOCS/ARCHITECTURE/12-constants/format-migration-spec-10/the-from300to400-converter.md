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


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
the maintained blocks (`kbdocs.Maintain`, [§6](../6-kb-model-and-maintenance/index.md)). A save that stops
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
bytes unchanged, its root path obsolete ([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/update-format.md)). `update-format` makes that move on disk
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


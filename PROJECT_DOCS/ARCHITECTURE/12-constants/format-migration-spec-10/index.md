### Format migration (SPEC §10)

**Pattern:** Chained schema migrations (upcasters) with golden tests.

A conversion step is bytes in, bytes out, plus the paths it made obsolete: a converter is a pure
function over `formats.Files` (`internal/formats/from<from>to<to>`) that parses its own superseded
form, reads each side's definition from its format package (`internal/formats/format<version>`) and
emits the next form. The chain's order is the version order and a gap in it is a test failure. The
pending obsolete set lives in the `Source`.

**Pattern:** Read-time upcasting plus an explicit migrate command.

Every verb that reads a KB (`update-format`, `verify`, the queries, the write ops, `export-latex`,
and the build's `spine-seed`, `phase-3a`, readiness stamp, overview and resume) calls
`kbload.Open(kbRoot)`; `status` and `cancel` do not load the KB. `Open` reads the stamp alone, from
`.index/format.yaml` (`kb.DecodeFormatStamp`), else from the superseded entry-point frontmatter
(`kb.FormatStamp`), before anything else.

- A stamp at the current major and minor builds a `kb.Source` that reads disk.
- An older stamp reads every file the format covers (`kb-root/` and the build records beside it) into
  a map of repository-relative paths to bytes and hands it to `formats.Chain(from, to)`. `Chain`
  applies each converter in turn and returns the current form's files, the union of the obsolete
  paths and every step's warnings. The `Source` serves the returned bytes and reads an obsolete path
  as absent. One converter covers every patch of its minor.
- A newer or unreadable stamp is refused `check: kb-format`; no entry point is refused `check:
  kb-root`.

The `formats` tree is the only home of a superseded parser and of the live format's definition
(`format40`, which `kb` re-exports). It imports nothing of kbase outside itself, and `kbload` is the
only importer of its root from outside it (import-policy test, [§11](../../11-package-map/dependency-rules.md)). The five
warning list names (`common.ListIDs`, `ListBareIDs`, `ListUnresolvedCitations`, `ListNoEdge`,
`ListStrengthenBy`) are defined in `common` alone; the `formats` root and `kbload` re-export them,
and `update-format`'s result lists (`formatResultLists` in `cmd/update-format.go`) match on
`kbload`'s: warnings listed `ids`, `bare-ids`, `no-edge` or `strengthen-by` appear under keys of
those names and the rest under `migration`. The save reports the removals as `removed`
(SPEC §8 `update-format`).

**Pattern:** Crash-resumable ordered save.

`index.UpdateFormat` is the save, and only `cmd/update-format.go` calls it. Every other writer
refuses a migrated `Source` first (`index.PendingConversion`; [§11](../../11-package-map/loading.md)). After the load
(`Precheck`, then `Derive`) the phases run in this order, each file atomically:

1. The caller's moves of the build records at the repository root (below).
2. `saveConverted`: `.index/edges.yaml`, then `.index/nodes.yaml`, then every other migrated file but
   the entry point.
3. The summary pages and the sheets.
4. The stamp file, written at the version the tree is still at, so a stamp always exists and always
   names the tree's version.
5. The obsolete paths, less what this save covered.
6. The entry point, its frontmatter (which stamped the superseded form) stripped by the conversion.
7. The stamp file again, at the current version, so the stamp marks a completed migration.

The verb then rebuilds the maintained blocks (`kbdocs.Maintain`, [§6](../../6-kb-model-and-maintenance/scratch-creation.md)) and, last, cites every
bare id: `citeBareIDs` in `cmd/update-format.go` runs the `update-format`-only op `cite-bare-ids`
([§6](../../6-kb-model-and-maintenance/build-only-ops.md)) through `write.Run` over the saved KB,
under the write lock the verb already holds. Its refusal is checked before the save writes:
`checkCiting` runs `write.CheckCiteBareIDs` over the converted `Source` once `Precheck` and
`Derive` pass (a KB they refuse is the save's own refusal), so a refused run writes nothing. Each id
it cited joins the conversion's `bare-ids` warnings in `listConversion`, which lists `bare-ids` on
every run and the other three lists only where the format changed.

A save that stops part-way leaves the KB stamped at the version it converts from, its next load
resuming the conversion. `update-format` refuses that tree as not clean, and the recovery is to restore `kb-root/`
and any moved build record from `HEAD` (SPEC §8 `update-format`). A converter that finds
`.index/nodes.yaml` in place takes both record files as they are (`from20to30`'s `resumed`), since
only an interrupted save writes it, and passes the stamp file through unchanged. `from30to40`
likewise takes as they are the record files that already decode as `4.0.0`, which only an
interrupted save writes over a `3.0.0` stamp.

Build records move with `git mv` (`gitledger.Repo.Move`), the only mover. The verb checks the moved
records clean and hands `index.UpdateFormat` the moves (`index.RecordMove`) and `Move`, which it
calls as the phase after the load and before the save's first write. A refused conversion therefore
moves nothing, and the save finds each record in place and its root path gone. The save itself
writes and removes nothing outside `kb-root/` (`index.WriteKBFile` refuses such a target). Each
moved record is reported in `removed` at its root path, repository-relative, and in `written` at its
new path, kb-root-relative.

Contents:

- [§12 The `from30to40` converter](the-from30to40-converter.md)
- [§12 Golden tests](golden-tests.md)

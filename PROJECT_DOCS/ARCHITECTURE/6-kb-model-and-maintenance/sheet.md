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
processes at once (`sheet.renderConcurrency`, [§12](../12-constants/index.md)), keeping output order. Where `dot`'s spline
router fails on a dense sheet, the emitter composes the same DOT with straight-line splines and it
is rendered once more; where both fail, the first error is reported. Without `dot`, it renders
nothing, leaves existing sheets as they are, and yields the placeholder for a root that has no
sheet.

Every re-derivation — `index.UpdateFormat`, every caller of `index.Refresh` (`init`, `score`'s
refreshes, the build's gate) and a write batch's `index.PlanDerivation` — reaches it through one
seam, `index`'s `planSheet`, which renders every sheet on each call and yields one only where its
bytes change, with no ownership test on an existing sheet; a refresh writes what it yields, a batch
writes it with its records. `verify` never reaches it.


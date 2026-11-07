### Sheet

**Pattern:** Graphviz rendering with a bounded process pool; a materialized view.

`internal/sheet` emits Graphviz DOT and renders it by running `dot -Tsvg` on `PATH`. The SVG's bytes are
Graphviz's, so the goldens pin the DOT. `TestRenderDraws` renders through `dot`, skips without it, and
asserts only that each root is fitted to the viewer; `TestRenderTwiceIsByteIdentical` renders the
fixture twice through `dot`, skipping without it, and requires identical SVG bytes; `TestDOTQuoting` asserts, where `dot` is on
`PATH`, that the DOT it composes parses through `dot`. The goldens are a fixture and its three DOT files under
`internal/sheet/testdata/adjagent-sheet/`: a `3.0.0` KB and the DOT the sheet composes for it, with a
build record beside it that the sheet does not read. `TestAdjagentSheetGoldens` compares the emitted
DOT with them byte for byte.

The inputs are the rows `index.Derive` yields (nodes, edges, strength bands, cites), which `index`'s `planSheet` hands
it from `index.Derive`, never a read of `.index/`:

- the `demoted` rows, for the removed edges, and one style per relation;
- each node's `CanonicalPath` and `CanonicalAnchor`, for its link;
- the leaf, for a node's kind, with the title deciding where no marker is readable;
- the entry point's first heading, for the title and the cluster of nodes no volume hosts;
- each volume's `index.md` first heading, falling back to the directory name, for the volume's
  cluster title.

It reads no build record. Its non-test code imports `kb` alone today; the dependency rules only keep it
below the build ([§11](../11-package-map/dependency-rules.md)).

It produces the full sheet always, and the overview diagram and one sheet per volume where two or more
volumes contain nodes (SPEC §3 `claim-graph.svg`). It renders through a pool of `dot` processes bounded by
`sheet.renderConcurrency` ([§12](../12-constants/index.md)), keeping output order. `dot`'s spline router can fail
on a dense sheet, and straight lines bypass it, so the emitter then composes the same DOT with
straight-line splines and renders it once more; where both fail, the first error is reported. Without
`dot` it renders nothing, leaves existing sheets as they are, and yields the placeholder only for a root
that has no sheet.

Every rebuild of the derived views (`index.UpdateFormat`, every caller of `index.Refresh`: `init`,
`score`'s start rebuild, the build's `phase-3a`; and a write batch's `index.PlanDerivation`) reaches the
sheet through one interface, `index`'s `planSheet`, which renders every sheet on each call and yields
one only where its bytes change, with no ownership test on an existing sheet. A rebuild writes what it
yields; a batch writes it with its records. `verify` does not reach it.


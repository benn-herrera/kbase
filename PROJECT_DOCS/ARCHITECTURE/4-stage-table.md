## 4. Stage table

The build walks these stages in order. A stage boundary is a commit ([§8](8-ledger-state-store-and-monitoring.md)).

| Stage | Does |
|---|---|
| `start` | pandoc and git preflight; a fresh build's refusal of any existing `kb-root/`, `init`'s own (`buildstore.KBRootExists`); operation lock; records `started-at` and the inputs, absolute and in path order, that every later stage reads (SPEC §5, [§6](6-kb-model-and-maintenance/index.md)) |
| `document-graph` | The tree from the volume roots, in path order |
| `spine-seed` | Seeds the format stamp where the tree carries none, then writes the two record files, empty, over that tree (`build.seedSpine`) |
| `claims-declared` | Claim-graph pass over author-declared claims |
| `claims-discovered` | Claim-graph pass over discovered claims |
| `equations-minted` | Equation nodes minted |
| `references-found` | Unmarked references found: adjacency links made by rule between consecutive non-equation claims of a leaf, unasked; per source claim, the proximity slots — nearest in its own paper by locality (same document, same directory, same paper), then by places apart in the sequence of every claim by reading position and id — then the wildcard slots, best by TF-IDF cosine less the proximity picks, counts in [§12](12-constants/index.md), no pool holding a marked pair or an adjacency link; each source's shortlist is its own, so a pair both ends pick is asked both ways, and a proximity slot its paper cannot fill stays empty; one multichoice ask per planned pair, a yes in either direction one undirected link for the cull; writes nothing under `kb-root/` |
| `claims-culled` | The co-linearity cull (`Culled`, `internal/claimgraph/cull.go`): one `Colinear` ask per link with a prose end — marked pairs, unmarked yeses and adjacency links, unordered, computed over the tree `references-found` read — grouped by the earlier claim in reading order (`p1-<id>`); one confirming ask per culled claim whose root is not its target, grouped by root (`p2-<root>`); the survivor rule and re-pointing in the pure `cullTargets`, `cullRoots`, `finalCulls` and `PlanCull`; culls issued through `IssueCulls` with no bridge and no claim-to-claim edge; the cull record written once, after the issue. Its row spends its own inference and is dropped under `--no-inference` (SPEC §5) |
| `depends-attributed` | Dependency edges attributed: the marked candidates and the cull record's links, each link oriented later claim to earlier (an equation node always the target) and skipped where a marked pair joins its claims; one multichoice ask per candidate; every answer's edge written in one `add-build-edges` batch, cycles included; then the ring row (`claimgraph.ringRow`): `index.BreakCycles` walks the records in the tree's hosted order and each edge it cuts is issued through `demote-claim-edge`, its warning in the stage report (SPEC §5, [§10](10-compatibility-harness.md)), before the gate |
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
  its conversion is pending (`loadFormat`, [§12](12-constants/index.md) "Format migration").
- There is no opt-in document-audit stage (ROADMAP).
- Stage boundaries are where expensive work becomes recoverable: an inference stage's per-leaf work
  is uncommitted until its boundary.

---


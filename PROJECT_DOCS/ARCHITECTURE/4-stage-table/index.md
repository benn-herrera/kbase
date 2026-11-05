## 4. Stage table

**Pattern:** Checkpoint/restart pipeline.

The build walks these stages in order. A stage boundary is a checkpoint commit
([§8](../8-ledger-state-store-and-monitoring/index.md)). The resume position is read from the checkpoint commits
alone, never from the state store (SPEC §6 Git holds the recoverable record). "Detail below" marks a
stage whose mechanism follows the table.

| Stage | Does | Pattern |
|---|---|---|
| `start` | pandoc and git preflight; on a fresh build, refuses any existing `kb-root/` (`buildstore.KBRootExists`, as `init` does); takes the run lock ([§8](../8-ledger-state-store-and-monitoring/locks.md)); records `started-at` and the inputs, absolute and in path order, that every later stage reads (SPEC §5 Inputs; SPEC §6 Git holds the recoverable record) | Preflight checks plus a job manifest |
| `document-graph` | The tree from the top `.tex` files, in path order | Deterministic transform stage |
| `spine-seed` | Writes the format version where the tree carries none, then the two record files, empty, over that tree (`build.seedSpine`) | Deterministic transform stage |
| `claims-declared` | Claim-graph pass over author-declared claims. Detail: [§4](finding-claims/index.md). | Novel: THESIS "What is new" item 1, defined in SPEC §10 Five node types. Machinery: AST extraction of labelled environments |
| `claims-discovered` | Claim-graph pass over discovered claims. Detail: [§4](finding-claims/index.md). | Novel: THESIS "What is new" item 1, defined in SPEC §5 A build marks every claim and SPEC §10 Five node types. Machinery: LLM classification of prose spans against a fixed answer grammar |
| `equations-minted` | Equation nodes minted | Mechanical extraction |
| `references-found` | Unmarked references found. Detail below. | Candidate generation then pairwise classification |
| `claims-culled` | Co-linearity cull (SPEC §5 Co-linearity cull (`claims-culled`)). Detail below. | Near-duplicate clustering with a canonical representative |
| `depends-attributed` | Dependency edges attributed (SPEC §5 Dependency attribution (`depends-attributed`)), then cycles broken. Detail below. | Pairwise relation classification, then cycle removal |
| `analyses-split` | Analyses split (SPEC §5 Analysis split (`analyses-split`)). Detail below. | Novel: THESIS "What is new" item 1, defined in SPEC §5 Analysis split (`analyses-split`). Machinery: deterministic text segmentation |
| `phase-3a` | Rebuilds the derived views, runs `verify`, and writes the readiness documents (SPEC §3 Stamped documents) | Materialized views rebuilt after each write, with a staleness check |
| `overview-drafted` | The overview document | Generate → validate → retry once |

**`references-found`.** Adjacency links are made by rule between consecutive non-equation claims of a
leaf, unasked. Each source claim gets its own shortlist of proximity slots followed by wildcard slots
(counts in [§12](../12-constants/index.md)):
- proximity: the nearest claims in its own paper by locality (same document, same directory, same
  paper), then by places apart in the sequence of every claim by reading position and id;
- wildcard: the best by TF-IDF cosine (`rankShortlist`, `internal/claimgraph/shortlist.go`) less the
  proximity picks.

No pool holds a marked pair or an adjacency link. A pair both ends pick is asked both ways, and a
proximity slot its paper cannot fill stays empty. One multichoice call per planned pair; a yes in
either direction is one undirected link for the cull. The stage writes nothing under `kb-root/`.
Proximity slots exist because a claim's nearby paragraphs, in reading order, are far more likely to be
in its chain than distant ones, though never a hard filter; wildcard slots exist because a target in
another paper is outside every proximity tier. The fixed slot count keeps the model calls linear in
claims. The particular counts, 3 and 2, have no recorded reason.

**`claims-culled`.** `Culled` (`internal/claimgraph/cull.go`) asks one `Colinear` call per link with a
prose end (marked pairs, unmarked yeses and adjacency links, unordered, computed over the tree
`references-found` read), grouped by the earlier claim in reading order (`p1-<id>`). One confirming
call per culled claim whose root is not its target, grouped by root (`p2-<root>`). The survivor rule
and the update of references are the pure `cullTargets`, `cullRoots`, `finalCulls` and `PlanCull`;
culls are applied through `IssueCulls` with no bridge and no claim-to-claim edge; the cull record is
written once, after the culls are applied. Each answer names the claim that adds nothing, and that
claim is culled into its partner (SPEC §5 Co-linearity cull (`claims-culled`)). Reading order only
breaks ties: a claim several answers cull targets the earliest of their partners, ties by id, and a
cycle among targets is broken at its earliest claim. The tie-break is fixed so that the same answers
in any order give byte-identical cull records; why the earliest rather than another fixed choice is
not recorded.

**`depends-attributed`.** One multichoice call per candidate. Every answer's edge is written in one
`add-build-edges` batch, cycles included; then `index.BreakCycles` walks the records in the tree's
hosted order and each edge it cuts is applied through `demote-claim-edge` (`claimgraph.ringRow`), its
warning in the stage report (SPEC §10 Cycles: cut as the walk finds them). Cycle breaking is
[§6](../6-kb-model-and-maintenance/cycle-breaking.md). Cycles are written first and cut afterwards because the cut reads the records
the edge batch left, cycle included.

**`analyses-split`.** `Split` (`internal/claimgraph/split.go`) takes a claim's bounds from
`claimBodies` and `bindProofClaims`. The statement proper ends at the sentence `opener` picks for a
prose claim's title. The analysis's marker goes on the first blank quoted line of the claim's block,
or at the end of its paragraph's last line; the claim's marker moves to just after its sentence, on
the line the sentence ends on (`split-claim`'s `statement-line` and `statement-end` move the claim's
marker, not the analysis's). Where the claim's own text holds nothing more, the analysis's marker
goes on the first blank quoted line of the first bound proof in reading order (`readingOrder`). A claim whose host block has no blank line is left unsplit and
counted (`unsplit-no-blank-line`). The analysis's title is the claim's title rebased to the
analysis's summary page. Every split is applied in one `split-claim` batch. The stage asks no model,
so no step of it drops under `--no-inference`, and it writes no build record.

Each claim-graph stage is one `internal/claimgraph` invocation. Every build record a stage writes
lands in `kb-root/.build/` (`buildrecords.Dir`), so the build owns `kb-root/` alone
(`buildstore.OwnedPaths`) and each boundary commit stays within it (SPEC §5 Output and placement).
The build's writes go through the write ops ([§6](../6-kb-model-and-maintenance/build-only-ops.md)).

- `--through <stage>` stops the build after the named stage; `--no-inference` drops the steps that
  make model calls and runs the rest (SPEC §5 Determinism). Which steps drop follows each step's
  classification in the step table (`buildstore.Steps`: `spendsOwnInference`, or a system prompt),
  never a stage id.
- A step marked `spendsInferenceInPart` (`depends.attribute`) is not dropped: part of it makes model
  calls, but the step still runs under `--no-inference` with a nil `claimgraph.Options.Launch`,
  which is what not asking is. The provider preflight asks `CallsAModel`, which counts such a step.
- `phase-3a` writes the readiness documents through `kbdocs.Stamp`. A build resuming over an older KB
  is refused while its conversion is pending (`loadFormat`, [§12](../12-constants/format-migration-spec-10/index.md)).
- Stage boundaries are where expensive work becomes recoverable: a stage's per-leaf model work is
  uncommitted until its boundary.

Contents:

- [§4 Finding claims](finding-claims/index.md)

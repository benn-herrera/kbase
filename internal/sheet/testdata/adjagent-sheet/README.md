# claim-graph-sheet — the claim-graph sheets' input and their DOT goldens

The sheet composer reads a KB root's `.index/`, the H1s of `entry-point.md` and each
volume's `index.md`, the leaves hosting its claims, and the unmarked-reference build record beside
`kb-root/`. This directory is such a repository: `kb-root/` is a full KB whose every derived field
is what `kbase refresh` writes, and the build record sits beside it. kbase's sheet tests load it,
compose the sheets and byte-compare them with the three goldens here. Every line of `.index/` is a
record, which is what keeps the fixture well-formed. Every file is in KB metadata format `2.0.1`.

These are inputs to tests only.

| File | What it is |
|---|---|
| `claim-graph.dot` | `compose_sheet(load(kb-root))`, trailing newline included |
| `claim-graph-digest.dot` | `compose_digest(load(kb-root))`, trailing newline included |
| `alpha-claim-graph.dot` | `compose_volume_sheet(load(kb-root), "alpha")`, trailing newline included |
| `kb-build-unmarked.yaml` | the build record each `supports` edge's provenance is read from |
| `kb-root/entry-point.md`, `kb-root/alpha/index.md`, `kb-root/beta/index.md` | the KB title (the entry point stamped `kb-format: "2.0.1"`) and the two volume titles, Beta's carrying `&`, `"` and `<` |
| `kb-root/gamma/index.md` | a volume directory no node sits in, which gets no sheet and no digest box |
| `kb-root/alpha/results.md`, `kb-root/beta/governance.md` | the leaves whose Tier-2 markers place claim kinds |
| `kb-root/alpha/claim-rigor.md`, `kb-root/beta/claim-rigor.md`, `kb-root/claim-rigor.md` | the registers: a volume's claims and its analysis's local value, Beta's claims, and the root's work |
| `kb-root/axioms.md` | the two axioms `founds` edges name |
| `kb-root/alpha/analysis.md`, `kb-root/alpha/experiments/flow.md` | the leaves hosting the analysis and the experiment |
| `kb-root/.index/*.yaml` | the records `kbase refresh` derives; `strengthen-by.yaml` is empty |

## What the records cover

- **Nodes.** Claims in both volumes, an analysis and an experiment in `alpha`, and two axioms and a
  work at the root. Bands on all five ladder rungs and `unknown`. Two unattached claims in
  `alpha`, none in `beta`. One title carrying `` $`\nabla f`$ ``, `"`, `<` and `&`; one carrying
  `` $`\Gamma`$ ``, whose `\G` a tooltip must not read as Graphviz's graph-name escape; one long
  enough to be cut, and one carrying a Markdown link and emphasis.
- **Strength.** Every claim is either carried by a participating edge or visibly bare — never a
  third state. `clm-aaa002` stands on an axiom and reads its own rigor, `0.95`, as the cap under a
  premise worth `1.0`. `clm-aaa001` reads `min(0.9, 0.95, 1.0) = 0.9`, capped by its statement.
  `clm-aaa003` composes all three positions: `min(max(min(0.5, 0.9), lift 0.64), cap 0.60) = 0.6`.
  `clm-aaa004` has no premise and takes the lift alone — `max(0, 0.4) = 0.4`. `clm-aaa005` and
  `clm-aaa006` are the bare pair, rigor `0.3` and `0.1` and strength `0` on the `unsubstantiated`
  rung. `clm-bbb003` has a participating premise and no rigor, so its derivation is pending and so
  is its strength. `ana-aaa007`'s strength is its own rigor under an axiom premise, which is what
  its lift and its cap multiply.
- **Kinds.** A marker in a labelled blockquote (`clm-aaa001`), the same on the content line of a
  block whose label carries a `<span id>`, under a title that reads as prose (`clm-bbb002`), a
  marker inside a math fence under a prose title (`clm-aaa003`), and a marker in a paragraph
  (`clm-aaa004`). An equation with no marker is read by its title, as every equation is; four more
  claims fall back to the title and are counted: claims no leaf hosts at all.
- **Edges.** Every relation the schema defines: `supports`, `founds`, `lifts`, `refutes`,
  `references` and `demoted`. A cross-volume `supports`. An inferred `supports` (the unmarked
  record's "points" letter) and a cited one the unmarked record answered "does not point". A
  `references` pair in both directions, neither drawn. Two `demoted` rows, one per origin:
  `inferred` within `alpha`, and `cited`, carrying a context, from `beta` into `alpha`, which the
  digest bundles as a cut. A triangle — `clm-aaa003` rests on `clm-aaa002` directly and through
  `clm-aaa001` — whose direct edge is not drawn. Two `supports` records differing only in
  `context`, which draw as one stroke.

The index refuses an edge whose source names no node, so a refreshed KB carries no ghost box;
`TestGhostDraws` covers that box in the sheet package.

## Regenerating

`kb-root/.index/` and each register's strength lines, premise annotations and leaf-reference footer
are `kbase refresh`'s output, not authored text. The fixture directory is not a git worktree, and
refresh finds `kb-root/` beside the repository's `.git`, so refreshing means copying this
directory into a scratch worktree, running `kbase refresh` there, and copying the written files
back. Refresh draws `kb-root/*.svg` from whatever Graphviz it finds; this directory commits none of
them, so discard them.

The DOT text is ours and deterministic, so it needs no Graphviz and no version pin. No SVG is
committed and no test runs Graphviz: `dot`'s layout and bytes change between releases, so the DOT
text is the only pin on what is drawn. No recipe writes these three goldens either; the composer's
own bytes for each sheet are the golden, and a deliberate change is applied from a failing compare
or from a dump of `sheet.dot(false)`.

**A golden update is a decision, not a repair.** Its whole value is that a change nobody meant to
make shows up as a failing byte-compare; editing one to turn a red test green throws that away.
Read the diff, decide the change was intended, then apply it.

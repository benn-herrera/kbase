# claim-graph-sheet — the claim-graph sheets' input and their DOT goldens

The sheet composer reads the rows `index.Derive` yields from a KB root's records and statement
markers, the H1s of `entry-point.md` and each volume's `index.md`, and the leaves hosting its
claims. This directory is such a repository: `kb-root/` is a KB in metadata format `3.0.0` — the
`2.0.x → 3.0.0` converter's golden output (`internal/formats/testdata/adjagent-sheet-3.0.0/kb-root/`),
stamped `3.0.0`, with the register pages `kbase refresh` renders in place of the `2.0.1` registers —
and an unmarked-reference build record beside it that the sheets do not read. `kbase verify` over it
reports the four unhosted claims' `statement marker` fault and the two `demoted` records, and
nothing of the pages. kbase's sheet tests load it, compose the sheets and byte-compare them with the
three goldens here.

These are inputs to tests only.

| File | What it is |
|---|---|
| `claim-graph.dot` | `compose_sheet(load(kb-root))`, trailing newline included |
| `claim-graph-digest.dot` | `compose_digest(load(kb-root))`, trailing newline included |
| `alpha-claim-graph.dot` | `compose_volume_sheet(load(kb-root), "alpha")`, trailing newline included |
| `kb-build-unmarked.yaml` | a build record the sheets do not read: its "points" answer for `clm-aaa003` → `clm-aaa001` draws nothing of its own |
| `kb-root/entry-point.md`, `kb-root/alpha/index.md`, `kb-root/beta/index.md` | the KB title (a plain entry point; the format stamp `kb-format: "3.0.0"` stands in `.index/format.yaml`) and the two volume titles, Beta's carrying `&`, `"` and `<` |
| `kb-root/gamma/index.md` | a volume directory no node sits in, which gets no sheet and no digest box |
| `kb-root/alpha/results.md`, `kb-root/beta/governance.md` | the leaves whose statement markers place claim kinds |
| `kb-root/alpha/claim-rigor.md`, `kb-root/beta/claim-rigor.md`, `kb-root/claim-rigor.md` | the register pages `kbase refresh` renders from the records, which the sheets do not read |
| `kb-root/axioms.md` | the two axioms `founds` edges name, as prose |
| `kb-root/alpha/analysis.md`, `kb-root/alpha/experiments/flow.md` | the leaves hosting the analysis and the experiment, each marker directly after the first heading |
| `kb-root/.index/nodes.yaml`, `kb-root/.index/edges.yaml` | the node and edge records |

## What the records cover

- **Nodes.** Claims in both volumes, an analysis and an experiment in `alpha`, and two axioms and a
  work at the root. Bands on all five ladder rungs and `unknown`. Four claims carry no statement
  marker and so have no host (`clm-aaa005`, `clm-aaa006`, `clm-bbb001`, `clm-bbb003`): they draw in
  the `kb-root` cluster with an empty link, `clm-aaa005` and `clm-aaa006` unattached. One title carrying `` $`\nabla f`$ ``, `"`, `<` and `&`; one carrying
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
  (`clm-aaa004`). An equation-titled claim (`clm-aaa002`) whose marker stands on its leaf's first
  heading line, the leaf carrying no fence labelled for it, draws as prose: its kind is read at its
  marker. The four unhosted claims fall back to the title.
- **Edges.** Every relation the schema defines: `supports`, `founds`, `lifts`, `refutes`,
  `references` and `demoted`, each in one stroke. A `supports` from `alpha` into the `kb-root`
  cluster (`clm-aaa001` → `clm-bbb001`). Two `supports` pairs the unmarked record answered
  "points" and "does not point", drawn alike. A `references` pair in both directions, neither
  drawn. Two `demoted` records, each replacing a `supports` edge out of `clm-aaa004` and drawn in
  that edge's direction: one within `alpha`, and one carrying a context, from `alpha` into the
  `kb-root` cluster, which the digest bundles as a cut. A triangle — `clm-aaa003` rests on
  `clm-aaa002` directly and through `clm-aaa001` — whose direct edge is not drawn. One `supports`
  record whose context joins the two uses the `2.0.1` form authored as two bullets.

No edge names a node no record carries, so the fixture draws no ghost box; `TestGhostDraws` covers
that box in the sheet package.

## Regenerating

`kb-root/` is authored — the records, the markers and the documents — but for the register pages,
which `kbase refresh` renders. After a change to the records, the markers or what refresh renders,
run `./bin/kbase refresh` over a scratch copy of this directory with an empty `.git` beside its
`kb-root/`, and copy the three `claim-rigor.md` pages it writes back over these. Refresh also draws
`kb-root/*.svg` from whatever Graphviz it finds; this directory commits none of them, so discard
them.

The DOT text is ours and deterministic, so it needs no Graphviz and no version pin. No SVG is
committed and no test runs Graphviz: `dot`'s layout and bytes change between releases, so the DOT
text is the only pin on what is drawn. No recipe writes these three goldens either; the composer's
own bytes for each sheet are the golden, and a deliberate change is applied from a failing compare
or from a dump of `sheet.dot(false)`.

**A golden update is a decision, not a repair.** Its whole value is that a change nobody meant to
make shows up as a failing byte-compare; editing one to turn a red test green throws that away.
Read the diff, decide the change was intended, then apply it.

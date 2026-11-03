## Worked examples: one input set

The example above is the diagram's own fragment, so its figures are the diagram's and move with it. This
set holds the inputs still instead: the same values are authored on every node in every case, and the only
thing that varies is which edges exist. Nothing requires a KB to author the same values everywhere — the repetition
is the exercise's, not the format's — and it is what lets each relation be read against the same operands.

| authored input | value | the table it comes from |
|---|---|---|
| a work's `rigor` | `0.70` | method vocabulary, two findings |
| a claim's `rigor` | `1.00` | statement vocabulary, no charge — the table's maximum |
| an analysis's `rigor` | `1.00` | method vocabulary, no charge — the table's maximum |
| an experiment's `rigor` | `1.00` | method vocabulary, no charge — the table's maximum |
| `founds` `bearing` | `1.00` | bearing vocabulary, no charge |
| `supports` `bearing` | `0.20` | eight bearing charges |
| `lifts` `bearing` | `1.00` | bearing vocabulary, no charge |
| `refutes` `bearing` | `1.00` | bearing vocabulary, no charge |

Every one of these is a value its table reaches, not merely a multiple of `0.05`: the method vocabulary
reaches eight — every fifteen hundredths — and `0.70` is one of them, and the bearing vocabulary
reaches eleven — every tenth
([RIGOR_AND_BEARING_SCORING_SCHEME.md](../../rigor-and-bearing/the-band-outcomes/the-method-table.md), "The method table", "The
bearing values"). Three of the four bearings are `1.00`, a link with no bearing charge, and those
contributions are their sources' own values: the sources are what this set holds still. The exception is
the supporting edge's `0.20` — eight bearing charges, a claim
speaking to part of what it is cited for — and that is the set's one edge where a bearing visibly moves a
number.

The four products, one per relation, are therefore:

```
founds      1.00 x a work's rigor 0.70         = 0.70
supports    0.20 x a claim's strength 0.70     = 0.14
lifts       1.00 x an experiment's rigor 1.00  = 1.00
refutes     cap = 1 - (1.00 x a claim's strength 0.70) = 0.30
```

The refuter there is a claim like any other: its own strength is `0.70` once the work founds it, and the
cap multiplies that `0.70` — not the work's rigor.

Bands are `kb.StrengthBandLadder`'s, each threshold an inclusive lower bound:

| band | at or above |
|---|---|
| `ok-to-build` | `0.85` |
| `ok-with-caveats` | `0.65` |
| `input-only` | `0.45` |
| `do-not-build` | `0.20` |
| `unsubstantiated` | `0.00` |

Every figure below is exact at two decimals. The record trims a trailing zero — `0.20` is written
`- strength: 0.2`, `0.30` is `0.3` — while a query and the trace print the two decimals written here.

Contents:

- [§1 A chain: a work founds a claim, the claim supports another](1-a-founds-and-supports-chain.md)
- [§2 A lift over a weak base](2-a-lift-over-a-weak-base.md)
- [§3 A refutation caps — and sometimes does not](3-refutation-lowers-the-ceiling.md)
- [§4 A claim nothing supports](4-a-claim-nothing-supports.md)
- [§5 Only a node's own rigor makes it pending](5-only-own-rigor-makes-a-node-pending.md)
- [§6 A claim and an analysis on the same number](6-colocated-claim-and-analysis.md)
- [Summary table](summary-table.md)

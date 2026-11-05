## One premise, and none

`lifts` and `refutes` compose around `D` as the formula states. A claim whose only incoming scoring
edge is a lift takes `max(0, L) = L`; an analysis takes `max(r, L)`, its own method standing under
the lift. Either way the product is a contribution through an edge, and an empty premise set does
not suppress it. A claim whose only incoming edge is a refutation takes `min(0, C) = 0`: a cap is a
ceiling and never a floor, so an attack with nothing under it establishes nothing. An analysis takes
`min(r, C)` there — its method's number where the cap sits above it. A claim with no incoming
scoring edge at all has an empty `L` and an empty `C`, and reads `0`; an analysis with none reads
its `r`.

One premise proves the weaker of the two things in play — the claim's own statement and the premise's
contribution — and no premise proves nothing. Let `c` be a supporting edge's contribution,
`b(e)·σ(e)`, and `r` the node's rigor:

| a claim | rigor | strength |
|---|---|---|
| one supporting edge with contribution `c` | `r` | `min(r, c)` |
| no supporting edge | `r` | `0.00` |

| an analysis | rigor | strength |
|---|---|---|
| one supporting edge with contribution `c` | `r` | `min(r, c)` |
| no supporting edge | `r` | `r` |

The first row is the claim standing on one thing: the premise carries it and the statement caps it,
so a premise worth more than the statement cannot make the claim worth more than its statement, and a
thin premise gives it less. The second row is the bare claim, where `r` does not enter at all. A bare
claim whose statement fires nothing is the case `r = 1.00`, the statement vocabulary's maximum
([RIGOR_AND_BEARING_SCORING_SCHEME.md](../rigor-and-bearing/the-band-outcomes/the-statement-table.md), "The statement table"):
`1.00` rigor and `0.00` strength — a statement fully specified, consistent and single-sensed, with
nothing carrying it. What it proves is what it says, and nothing about whether anything supports it.

The analysis's second row is the other asymmetry. A claim is a statement, so nothing outside it can
stand under its assertion; an analysis is a method, and its inputs are the rules of inference rather
than a KB node — so one citing nothing reads its own `rigor`, and the claim it lifts off that
analysis reads it too. A method that does cite premises is unchanged: `min(r, c)` as for a claim, so
an analysis resting on an unsubstantiated claim still reads `0`, and a zero still propagates through
real dependencies.


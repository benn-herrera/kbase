## The band outcomes

The ladder is `kb.StrengthBandLadder` (`internal/kb/schema.go`): thresholds at `0.85`, `0.65`, `0.45`, `0.20`
and `0.00`, each an inclusive lower bound. The ladder reads `strength`, not `rigor` — a node's band
can sit below its rigor because of a premise, and `rigor` is not an upper bound on `strength` because
a lift station is placed above the derivation ([STRENGTH_SCORING_SCHEME.md](../../strength/the-walk.md),
"The walk"). The columns below are what a rigor value would read when rigor is what binds — that is,
when `P(n)` is non-empty and its weakest contribution is at least the rigor. Where `P(n)` is empty no
row in either rigor table reaches a band: strength is `0`, and the bottom rung is the answer.

Contents:

- [The statement table](the-statement-table.md)
- [The method table](the-method-table.md)
- [The bearing values](the-bearing-values.md)
- [The bottom rung](the-bottom-rung.md)

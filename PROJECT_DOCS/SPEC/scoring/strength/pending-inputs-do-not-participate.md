## Pending inputs do not participate

| state | effect |
|---|---|
| pending `rigor` | where `P(n)` is non-empty the derivation has no number; where it is empty a claim reads `0` and an analysis reads no number, having no method's value to stand on. A lift over either still publishes the lift's product |
| pending `bearing` | the edge does not participate: no link in the min, no lift, no cap. As a claim's only edge it leaves `P(n)` empty and the claim reads `0`; as an analysis's only edge the analysis reads its `r` |
| no source strength | the edge does not participate either: a premise whose own strength is pending, and any edge from a non-run experiment, add no operand. As a claim's only edge it likewise reads `0`, and as an analysis's only edge the analysis reads its `r` |
| `bearing 0.0` | participates with zero: the claim's min goes to zero |
| pending refute | no cap; an unfinished attack establishes nothing, and a cap alone publishes no number where the derivation is `⊥` |

**An edge that does not contribute is no supporter.** A pending bearing, a pending source strength,
an edge from a non-`run` experiment and a premise that no longer participates all add no operand, so
where such an edge is a claim's only one `P(n)` is empty and the claim reads `0`: an unjudged bearing
is not support yet, and a claim behind one reads `unsubstantiated` until that bearing is written, never
`*pending*`. An analysis behind one is a method with no premise it can cite, and reads its own
rigor. `bearing 0.0` reaches the same `0` for a claim by the other road — it participates, and the
min takes the zero — and the two stay different states in the record whatever the strength reads.

`bearing *pending*` is the absence of a judgement, so the edge is excluded. `bearing 0.0` is the
judgement that the supporting thing does not apply, so the edge participates and drags the min to
zero. Never judged and judged-irrelevant are different states, and their scores coincide only where
the min reaches `0` either way — most plainly where the edge is the node's only one: `0.0` empties
the min by taking it to zero, an excluded edge empties it by leaving no operand, and both read `0`.
The difference is the record's and the re-judge's, not the strength's. A source with no strength is
the first case, not the second: there is nothing to multiply, so there is no product.


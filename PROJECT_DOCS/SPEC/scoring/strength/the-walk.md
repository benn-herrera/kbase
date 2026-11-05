## The walk

```
Let n be a claim or an analysis. ⊥ means "no number yet"; an empty lift set contributes −∞ and an
empty cap set +∞, so neither changes anything. D is 0 where a claim's premise set is empty, and n's
own rigor where an analysis's is: a claim proves nothing bare, an analysis's method is its evidence.

r(n)  = n's authored rigor, or ⊥
σ(m)  = the published strength of a claim or an analysis m;
        the authored rigor of a work or an experiment; 1.0 for an axiom,
        its rigor by rule
b(e)  = the bearing authored on edge e, or ⊥
·     = an operand that is absent wherever either factor is ⊥

P(n)  = { b(e)·σ(e) : e ∈ supports(n) ∪ founds(n), e contributes }
D(n)  = 0                      where P(n) = ∅ and n is a claim
D(n)  = r(n)                   where P(n) = ∅ and n is an analysis
D(n)  = min({ r(n) } ∪ P(n))   otherwise
L(n)  = max{ b(e)·σ(e) : e ∈ lifts(n), e contributes }
C(n)  = min{ 1 − b(e)·σ(e) : e ∈ refutes(n), e contributes }

max(⊥, −∞) = ⊥     max(⊥, x) = x     max(x, −∞) = x
min(⊥, c)  = ⊥      min(x, +∞) = x

σ(n)  = min( max(D(n), L(n)), C(n) )
```

A **station** is the term of the formula that an edge feeds: `supports` and `founds` edges feed the min in `D(n)`, `lifts` edges feed the max `L(n)` above it, and `refutes` edges feed the cap `C(n)`.

**`r(n)` caps what the premises establish, and is not a premise.** Where `P(n)` is empty no premise
carries the node, and `D(n)` is what the node's kind is worth bare: `0` for a claim, whatever `r(n)`
says — judged, pending or absent — and `r(n)` itself for an analysis, whose method is the evidence
its premises would otherwise supply. Support is
structural: strength propagation reads that nothing supports the claim without knowing how good the node's own
text is, so it does not wait for a judgement. Where `P(n)` is non-empty and `r(n)` is pending, `D(n)`
stays `⊥`, because `min(⊥, c) = ⊥`. The asymmetry is the rule's content: an absent cap is unknown, an
absent premise is nothing.

`⊥` is "no number", not a small one: a lift over an unjudged node publishes the lift, and a cap over
one publishes nothing. So an empty `P(n)` with a pending `r(n)` publishes `0` for a claim — or the
lift over it — and not `⊥`, while an analysis publishes `⊥`, having no method's number to
stand on; and a non-empty `P(n)` with a pending `r(n)` stays `⊥` for either kind, and a lift over
it publishes the lift's product. Strength propagation uses full precision, so a dependent multiplies its
source's exact strength. The record writes a value to four decimals — trailing zeros trimmed, and
more only where four would lose it, so it reads back as the value; a query result contains the value
at two decimals and the trace prints it, and a `verify` message quotes the record. The trace on a
`- strength:` line prints the innermost station first, so the formula reads back: the min, the lift
above it, then the cap.


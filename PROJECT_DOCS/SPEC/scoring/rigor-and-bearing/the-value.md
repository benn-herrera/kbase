## The value

The maximum is per vocabulary: `M = 1.00` for the statement faults, `M = 1.00` for the method faults,
and `M = 1.00` for `bearing`. For a charge list `C` under the table the item's class selects:

```
value(C)             = max(0, M − Σ w(id(c)) for c ∈ C)
value(none)          = M
value([flt-pending]) = *pending*   (a recorded line only; the id is no answer)
```

The weight is charged **per occurrence**, each occurrence separately, and the table caps no count: a
text that commits a fault three times loses three weights. The parse enforces no count at all, and
the only mechanical pressure on one is that two occurrences of one id need two line sets that do not
coincide. A shared id has the same weight under either rigor vocabulary, so the price of a line that
contains only shared ids is the same whichever table read it; a line containing `flt-underspecified`,
`flt-unaddressed`, `flt-equivocation` or any method-only id is priced by the table that contains it, and
the item's class fixes which that is.

A count is bounded only where the row's own occurrence unit is an enumeration of a fixed set, and
there the bound is what *one occurrence* means rather than a ceiling the table adds.
`flt-unaddressed`'s respects are the fixed list of three — the steps, the result, the link to the
conclusion — so it has three. `flt-overreach`'s
respects are the fixed list of three, so it has three. `flt-inapplicable`'s parts are the source
argument's own — its stretches of reasoning — which the text fixes in outline and no field counts. A
charge beyond such a bound is not a further
occurrence: it is one of them written a second time. The parse cannot see that, because no respect and
no part is a ledger field; the reader can, comparing the reply's line sets against the text.

The floor at zero is the domain's, not a rule of its own: a list whose weights reach `M` is a text
with nothing left to carry the assertion, and `0` is what the register says for that. Every weight is
a multiple of `0.05`, so every sum and every value is exact at two decimals and nothing rounds.

**The ceiling is `1.00`.** `kb.RigorMaximum` is `1.00`, the same number as an axiom's rule value
(`kb.AxiomRigor`); an axiom premise contributes `bearing × 1.0`. An axiom is asked by neither pass —
its `rigor` is kbase's rule, not a judgement. Under either rigor vocabulary a text charged with
nothing, `none`, prices at `1.00`, so an authored `1.0` is in domain and reads as itself; only a
value above `1.00` is read at the ceiling, and `kb.RigorMaximum` is the ceiling's one statement.
`bearing` reaches `1.00` the same way, through `none`. The three tables below are priced at this
ceiling.


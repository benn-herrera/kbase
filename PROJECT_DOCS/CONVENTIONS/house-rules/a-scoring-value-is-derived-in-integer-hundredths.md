### A scoring value is derived in integer hundredths

Fixed-point arithmetic: weights, maxima and sums are integers; the
value is `maximum − Σ weights`, floored at zero and exact at two decimals. Pitfall: subtracting in
`float64`, where three weights land `0.3499999999999999` and the record's four-decimal rule breaks.
A value is a projection of a ledger line's ids alone, which is what makes the written results checkable and
a run re-scorable without asking a model.
- **A scoring page's value tables are derived, not authored.** Single source of truth: a page's maxima, its reachable-value
  tables and its worked figures are functions of the ceiling and the weights, so they change in the
  commit that changes either. Pitfall: leaving a page alone because no test pins its numbers — a value
  table duplicates the arithmetic, and a stale one states a value the code cannot produce.
  The reasoning a page gives for a superseded choice stays, marked superseded in one note; what changes
  is the numbers.

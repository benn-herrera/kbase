### A scoring value is derived in integer hundredths

Fixed-point arithmetic: weights, maxima and sums are integers; the
value is `maximum − Σ weights`, floored at zero and exact at two decimals. Pitfall: subtracting in
`float64`, where three weights land `0.3499999999999999` and the record's four-decimal rule breaks.
- **A scoring page's value tables are derived, not authored.** Single source of truth: a page's maxima, its reachable-value
  tables and its worked figures are functions of the ceiling and the weights, so they change in the
  commit that changes either. `TestScoringValueTablesAreDerived` (`internal/kb/valuetables_test.go`)
  is the rule's check, and covers six pages under `SPEC/scoring/rigor-and-bearing/`. On the three
  `the-fault-vocabulary/` pages (statement, method, bearing faults) it checks the fault ids listed,
  each fault's weight, the stated fault count and the stated maximum against `kb`'s fault table. On
  the three `the-band-outcomes/` pages (statement table, method table, bearing values) it checks each
  row's value for its count of charges, and its sum where the row gives one. The worked figures on
  any page, the strength pages' included, are unchecked. Pitfall:
  leaving a page alone because no test pins its worked figures — a value
  table duplicates the arithmetic, and a stale one states a value the code cannot produce.
  The reasoning a page gives for a superseded choice stays, marked superseded in one note; what changes
  is the numbers.

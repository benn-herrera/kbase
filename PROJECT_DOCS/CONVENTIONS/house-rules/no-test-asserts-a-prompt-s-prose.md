### No test asserts a prompt's prose

Single source of truth: a test keyed on a template's wording duplicates the
template; tests key on the offered letters, the slot set, the system fragment and the closing
line's letters.
- **Compatibility is judged by kbase's own readers and checks, never by a byte diff of
  presentation.** Byte equality is asserted only where SPEC "Compatibility contract" names a reader
  that needs it.
- **A derived strength is one value; only its formatting differs.** Strength propagation computes and publishes at
  full precision, so a dependent multiplies its source's exact strength. A summary page
  formats it — its `- strength:` line and a premise's `(strength …)` — in the fewest decimals that
  parse back as it: four in the ordinary case, more where four would lose it, trailing zeros
  trimmed; and strength propagation never reads that formatting. An interface that shows a value to a reader puts it
  at two decimals — a query result as the number, the trace on a `- strength:` line as the
  formatted value.

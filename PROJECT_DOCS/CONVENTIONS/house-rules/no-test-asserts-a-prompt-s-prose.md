### No test asserts a prompt's prose

A test keyed on a template's wording is a second copy of the
template; tests key on the offered letters, the slot set, the system fragment and the closing
line's letters.
- **Compatibility is judged by kbase's own readers and checks, never by a byte diff of
  presentation.** Byte equality is asserted only where SPEC "Compatibility contract" names a reader
  that needs it.
- **A derived strength is one value; only its spelling differs.** The walk computes and publishes at
  full precision, so a dependent multiplies its source's exact strength. A register page
  spells it — its `- strength:` line and a premise's `(strength …)` — in the fewest decimals that
  read back as it: four in the ordinary case, more where four would lose it, trailing zeros
  trimmed; and the walk never reads that spelling. A surface that shows a value to a reader puts it
  at two decimals — a query result as the number, the trace on a `- strength:` line as the
  spelling.

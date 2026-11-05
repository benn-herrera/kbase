## Frozen rules

These parse rules are settled here and not open:

- **The floor of one valid line number.** A citation names at least one line the call shows, lying in
  a section a charge may cite; a charge naming none is `malformed`.
- **`none` beside charges.** Refused as `malformed`; the two halves contradict each other and the
  parse has no preference for one.
- **No reply is a refusal.** A table's charges or `none` are the whole vocabulary; a claim that
  engages nothing, an empty text or a text that asserts nothing is charged, not declined. The retired
  `flt-pending` is an id outside the set.
- **Overlapping line sets for one id.** Two line sets of one id that overlap without coinciding are
  `malformed`, naming both lines; one id on one line set written twice collapses to one charge.

The selection of a line in the item text, the retirement of the `both` value and
the meaning of a line already written with it, and the malformed classes and the ledger's charge records
are settled in the same way.


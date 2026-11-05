### Malformed replies

| case | outcome |
|---|---|
| a line that is not a charge line | `malformed` |
| an id outside the call's set — the other rigor vocabulary's, the other pass's, a retired letter, a misspelling | `malformed`, naming the id and the call's ids as `allowed` |
| an id with no separator, or nothing after the colon | `malformed`, naming the id |
| `flt-pending`, alone or with any field | `malformed`, naming the id: no table contains it ([`flt-pending`](../the-fault-vocabulary/flt-pending-retired.md)) |
| a field that names no line — words, a quotation, a line of the context or of a label | `malformed`, naming the id and the field |
| a line number the call does not show, a reply cut short before it wrote one included | `malformed`, naming the id and the number |
| a range written backwards | `malformed`, naming the id and the range |
| a range crossing from one section to another, or over a gap — a blank line, a label or a heading between its lines | `malformed`, naming the id and the range |
| a number — a decimal, a fraction, `0`/`1`, or a `*pending*` literal | `malformed`: a number standing alone names no id, and a number in a charge's field is a line of the text, so a reply offering the first answers a question no longer asked |
| `none` beside charges | `malformed` |
| an empty reply, or whitespace only | `malformed`, nothing written |

A `malformed` reply is the pass's outcome: no charges and no judgement, the item re-asked on
`--resume` (SPEC §8). The attempt's record contains `status: failed` and the reason — the case from the
table and, where there is one, the id and the lines it named — and **not the reply's text**: a
malformed reply is blank, a truncated fragment or think-lock, and the data point that matters is that
the attempt failed, not the bytes it failed with.


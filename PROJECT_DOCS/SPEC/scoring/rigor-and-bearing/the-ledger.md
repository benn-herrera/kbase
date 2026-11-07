## The ledger

Each attempt is one record in the run's `ledger.jsonl`, containing the item, its status, `faults` as
parsed, the `reply` verbatim where the reply parsed, the `value` written, `before` — the value the item
had before the attempt, so the line is its own before/after pair — the model, the prompt's
digest and the attempt's bounds (SPEC §8), or the reason it failed and no text where it did not.
`faults` is the effective answer — one record per charge, in the order written — and `moot` contains any
charge lines a reply once recorded beside a retired `flt-pending`, parsed the same way and priced
nowhere:

| field | contains |
|---|---|
| `id` | the fault id, in lower case |
| `passage` | where the cited lines lie: `title` or `statement` for `rigor`; `source` or `target` for `bearing` |
| `lines` | the answer's own numbers: `12`, or `12-14` |
| `citation` | the cited lines as the call contained them, without the gutter, joined by newlines |
| `offset` | the quote's byte offset in the item text |

A rigor line's item is a `clm-`, `ana-` or `exp-` id; its prefix selects the statement or method table
a value or a recomputed value is computed under, so the rigor vocabulary is not a new field. A bearing line's
table is the pass's own. `[]` is written for one reply only —
the word `none` — so a record's empty list always contains the model's positive statement that it found
no fault; an empty or whitespace-only reply is `malformed`, so the attempt records `failed` and no
charges, and silence is never read as a clean pass. A line recorded before the pending id was retired
may contain `id: flt-pending` and no lines and no citation; no reply produces one now. The reply is kept
beside the parsed charges, so the parse's reading
of a line can be held against the line. A `--dry-run` writes no charge record: nothing is asked, so
nothing is returned.

**The value is a projection of the charge list, and in fact of its ids alone** — `value = table(counts
by id)`, the table chosen by the item's class. The citations are not the input, and that is why they
are the audit rather than the measure: they can be read, checked against the text and argued about,
and they cannot perturb the arithmetic.

- **The written values can be checked without a model.** A value that does not equal the table's output for
  its recorded ids is a defect of the write or of the table, not a judgement to argue with. Counts
  are integers, so the check is exact.
- **A run can be re-scored without a model.** Change a weight and recompute every value in the ledger
  by counting ids under the line's pass and, for rigor, its item's class; no call is made and no
  judgement re-asked. The durable record is the count per id.
- **Two runs are comparable on their fault sets, not their numbers.** Two runs agree on the judgement
  exactly when their counts per id match; their values are comparable only within one table. A value
  agreement is the weaker test: `0.35` is reached by two `flt-bare-measurement` and equally by one
  `flt-claimed-step` and one `flt-unstated-premise`, so a sum hides both the split and the identity.
  Reproducibility is measured on counts. A shared id crossing the two rigor vocabularies is one id in
  the count and one weight in the price, which is what keeps the comparison honest.

**The run records the prompt set, not the table.** What a charge set means is fixed by the instructions
the model was given — the fault set and the text describing it — and never by the weights, which the
model is never shown. So the run's header contains one `ask_sha256` over the pass's rendered
instructions, each rendered for an empty subject: rendering substitutes and never inspects the text,
so the empty input covers every fixed byte of an instruction and nothing that varies, and no
size-dependent element may enter a rendered prompt. A resume reads the digest it began under and its
lines contain their own digests as before.

- **An update applies its behaviour.** A run resumed under a changed table recomputes every written
  value from the recorded ids under its item's class, writes the values that moved and continues; one
  resumed under a changed prompt continues under the new prompt. Nothing is refused, and nothing asks the
  user to choose between versions: they updated, which is the choice.
- **The digests are a record, not a check.** `ask_sha256` says which instructions answered which
  class of item, so a corpus spanning an update can be read honestly. It decides nothing at run time.

**A truncated reply.** A reply cut off at the attempt's bound loses the lines it did not reach, and
where the cut falls between lines the remainder would parse cleanly — a silent under-charge the
parser cannot see. The attempt record contains the fact, not the text: a reply that reached the
bound is recorded as failed for that reason, so an under-charge from truncation is never written as a
judgement.

**A failed attempt keeps no text.** A reply that failed is blank, cut short or think-lock, and the
record's data point is that it failed; a reply that parsed keeps its text, where the shape of a good
answer — order, formatting, what a model chose to charge — is behaviour worth comparing.

**The record format is a contract.** One record per attempt, in the run's own file, with the fields
this section names and no fields that summarize them: no prose description of the judgement, no
count rolled up at write time, no value stored in place of the charges it came from. A parsed reply
is kept beside its charges; the value is kept beside both, derived and checkable; the run's header
contains the prompt set once, for every attempt in the run. This is what makes a run re-scorable, the
written values checkable and two runs comparable, so the format changes only with the scheme's version and
never as an implementation convenience: a field dropped at write time cannot be recovered by a later
reader, and a summary written by the writer is a judgement the corpus can no longer take apart.


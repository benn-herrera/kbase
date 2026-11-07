## The fault vocabulary

Fifteen id rows name the faults: four in the statement vocabulary, six in the method vocabulary and
five in `bearing`, with `flt-contradiction` and `flt-overreach` appearing in both rigor vocabularies.
Thirteen ids are distinct, and a ledger may also contain the retired `flt-pending` from a line written
before it went. An id is written `flt-[a-z0-9-]+` and writes out the criterion it names, so a
ledger line needs no table to read.

**An id names one fault wherever it appears.** The `bearing` set is disjoint from both rigor sets. The
two rigor vocabularies share two ids because `flt-contradiction` and `flt-overreach` name the same
fault at different subjects, and each shared id has the same weight in both tables, so a ledger
line's price never depends on which rigor vocabulary read it; only the definition's subject changes.
The ids have no severity and no order — the weight is the table's, below — and `none` is a word, not
an id: it names the absence of a charge and is not a member of any set.

Rows are in precedence order within a vocabulary. A criterion is met at each site of the text whose test it
satisfies, and where one site satisfies two rows of the same vocabulary the earlier row's id is the
charge and the later is not.

**That precedence binds the judgement, and the parse records what the reply wrote.** It is a rule on
how the model reads the text, and no check enforces it. The reason is the parse's whole remit: it
cannot rule on whether two criteria were met at one site, which is the same question as whether a
line earns an id, and it never asks that ([What the check does, and does not
do](#what-the-check-does-and-does-not-do)). A shared line is also not the same fact as a shared
site: two rows whose tests are met at different occurrence units — a minimal set, a respect, a
distinct proposition — may anchor at one line, so a parse that refused the second id on a line set
would refuse defensible answers. Two ids on one line set are therefore accepted and both charged, and
the shared quote and offset beside the verbatim reply are what let a later reader compare the line
against the precedence and see whether it was missed. The cost is stated rather than
hidden: a departure from the precedence is priced, not refused. The alternative — the parse refusing a
second id on a line set — is not the choice, because it turns a judgement it cannot check into
`malformed`, discarding both the answer and the evidence for it, and a retry is a fresh guess at the
same unverifiable question.

Contents:

- [The `statement` faults](the-statement-faults.md)
- [The `method` faults](the-method-faults.md)
- [The `bearing` faults](the-bearing-faults.md)
- [`flt-pending` (retired)](flt-pending-retired.md)

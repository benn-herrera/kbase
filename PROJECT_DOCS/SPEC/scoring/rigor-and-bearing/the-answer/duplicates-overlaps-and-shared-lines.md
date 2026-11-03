### Duplicates, overlaps and shared lines

| case | outcome |
|---|---|
| one id, one line set, written twice | one occurrence written twice: the pair collapses to one charge |
| one id, two line sets that overlap without coinciding | `malformed`, naming both lines — the reply asserts two occurrences and supplies one site |
| two ids, one line set | accepted, both charged, the quote and its offset shared in the ledger |

The duplicate collapses because the answer's own evidence cannot tell the two lines apart. The overlap
is refused because the count cannot be read from it, and a retry settles it, where reading it as one
would write a confidently high value. Two distinct faults may be cited at one line: the one-defect
rule is the walk's, not the parser's, and the shared offset is the signal that the precedence was
missed — not proof that it was ([the fault vocabulary](../the-fault-vocabulary/index.md)).


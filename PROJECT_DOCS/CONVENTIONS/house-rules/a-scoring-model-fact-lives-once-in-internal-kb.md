### A scoring-model fact lives once, in `internal/kb`

Single source of truth: the thirteen fault ids, the
word `none`, each id's weight per occurrence and each table's maximum are defined in `faults.go` and
nowhere else — the statement table, the method table and the bearing one, which a node's class
selects among. No table contains the retired pending id; `Price` still reads one from an old ledger
line. A template names only the ids its class accepts, and contains no figure or word a
model could interpret as a judgement on the answer — no decimal in `[0,1]`, no percentage, no weight, no
maximum, no band's word — while a quantity of the subject's own, an example's rate or threshold, is
the claim's and stays. `internal/asks` asserts both: the `flt-…` tokens it names are exactly its
class's set, checked against `kb`, and its text contains none of those strings. Pitfall: a weight in
a template, or a duplicate of the id list in the parser, which breaks the single source of truth and
drifts where the prompt does not.
- **A `flt-*` id is frozen for its meaning once named.** Stable identifiers: the set may grow or lose members, but a
  fault that has been named keeps its id for life: a ledger line records ids, and a rename would
  silently change what every line that carried the old one refers to. A new fault takes a new id; a retired fault's
  id never gets a new meaning.

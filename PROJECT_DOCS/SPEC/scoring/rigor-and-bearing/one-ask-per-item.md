## One ask per item

**The class gates the instruction, and nothing else changes.** A claim's rigor call renders the
**statement** variant; an analysis's or an experiment's renders the **method** variant; an edge's
bearing call renders the bearing ask as it stands. One item is one call and one instruction: no item
is asked twice, no call carries both vocabularies, and a run's call count and model cost are what they
were. The two rigor variants replace the single rigor template; the bearing instruction is untouched,
and its header entry is the one-element mapping.

**The run's identity is one digest over the set.** A rigor run now sends two different instructions, so
the header's `ask_sha256` is the digest of every instruction the pass can send, rendered for an empty
subject, concatenated in a fixed order — statement, then method for a rigor run; bearing alone for a
bearing run — and hashed once. One field, and it moves if any instruction moves: separate digests
would agree or differ together, so a set of them carries no more information than the one. The digest
covers the set the pass can send, fixed at the run's creation — not the subset the run's scope
reached.
Each attempt's `prompt_sha256` in the ledger is the instruction that item actually received, and it is
the line that answers "which variant judged this item". The header is written once and never
rewritten; a resumed run keeps the digest it began under.


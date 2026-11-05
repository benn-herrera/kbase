### Provenance and the judged hash

**The `manually-scored` flag** records whether a person's call wrote a record's rigor or bearing. It
is on claim, analysis and experiment records, and on `supports`, `lifts`, `refutes` and `demoted`
records. A work's rigor, an axiom and a `founds` edge have no flag, since no scoring pass reaches
them.

- A public op writes the flag `true` on a rigor or a bearing the caller supplied, `*pending*`
  included, and `false` on one it writes `*pending*` because the caller left it out. The ops are
  `add-<kind>`, `edit-<kind>`, `add-edge` and `edit-edge` ([§8](../8-maintenance-subcommands/add-kind.md), [§8](../8-maintenance-subcommands/edit-kind.md),
  [§8](../8-maintenance-subcommands/edge-ops.md)).
- An op leaves the flag as it is on a value it does not write. Re-asserting the current value of a
  record marked `false` marks it `true`, outcome `done`.
- A scoring run's results write `false` ([§8](../8-maintenance-subcommands/score/writing-the-results.md)), and so does the build
  ([§5](../5-build-behaviour/a-build-records-no-judgement.md)). No argument sets the flag directly.

**The *judged hash*** is the input hash a value was scored against; a changed hash marks the value
stale.

- It is the field `judged-hash`, the last field of claim, analysis and experiment records and of
  `supports`, `lifts` and `refutes` edge records, required on every one of them: a record without
  it is a load error. It is a SHA-256 digest as 64 lowercase hex digits.
- It covers the item's text plus the digest of the prompt template that judges it. For a node the text
  is its statement (Statements and anchors); for an edge, the statements of its two ends. The model
  is not part of the hash.
- A rigor or a bearing that an add or an edit is given — `*pending*` included — records the judged
  hash of the text as the batch leaves it, and so does every value a scoring run writes, over the
  text its call was composed under. A value set otherwise keeps the hash it has. An add given no
  value writes the never-judged hash.
- The never-judged hash is all zeros. No computed hash matches it, so the item is stale. A demoted
  record has no hash, so an edge restored from one has the never-judged hash.
- An item is *stale* when its current hash differs from the recorded one; a value never judged is
  stale. `score` scores the stale items ([§8](../8-maintenance-subcommands/score/index.md)).


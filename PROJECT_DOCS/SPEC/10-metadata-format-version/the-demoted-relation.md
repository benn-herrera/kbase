### The `demoted` relation

A `demoted` record is a tombstone for a scoring edge removed to break a cycle; `restore-demoted`
undoes it. The record keeps the edge whole: `relation: demoted`, `source`, `target`, `context`,
`replaced-relation` (`supports`, `lifts` or `refutes`), `bearing` and `manually-scored`. A `founds`
source, a work or an axiom, has no incoming scoring edge, so it closes no cycle. The record runs in
the replaced edge's direction. Its identity is (`demoted`, `replaced-relation`, `source`, `target`),
so more than one `demoted` record from one `source` to one `target` exists only where separate
cycles were removed at different relations between them.

- A `demoted` record is outside strength propagation and the cycle check.
- Only the build writes `demoted` records, in Cycles. No public write op creates one.
- Ops name a `demoted` record by `source`, `target` and `relation: demoted`. That name covers every
  `demoted` record from that source to that target.
- There are three ways to resolve one: `restore-demoted` puts the replaced edge back and deletes the
  record ([§8](../8-maintenance-subcommands/restore-demoted.md)); `remove-edge` with `relation: demoted` deletes the record and
  leaves the edge removed; `add-edge` of the replaced edge writes the edge with the add's own values
  and deletes the record ([§8](../8-maintenance-subcommands/edge-ops.md)).
- `verify` lists every `demoted` record under `findings` ([§7](../7-tool-result-contract-and-exit-codes/index.md)) with outcome `done`; a finding is not a
  fault. `edges` reports it as a row of its own relation, out of its source, with
  `replaced_relation`; `stats` counts them as `demoted_edges`, after `edges`, which counts every row;
  the sheet draws each as a cut ([§3](../3-compatibility-contract/index.md)); and its target's summary entry lists it.


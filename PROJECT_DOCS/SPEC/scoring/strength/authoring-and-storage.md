## Authoring and storage

Each edge is one record, written by `add-edge` with its `source`, `target`, `relation`, and
optionally its `bearing` and `context` ([SPEC §8 Edge ops](../../8-maintenance-subcommands/edge-ops.md)).
The author names the relation, over the legal pairings of the relations table above. The record
is stored as it runs, and a scoring edge pushes from the node that gives strength into the node
that takes it ([SPEC §10 Edge direction and authoring](../../10-metadata-format-version/edge-direction-and-authoring.md)):

| authored | stored edge (push) |
|---|---|
| a lemma C2 supports claim C1 | `clm-c20000 --supports--> clm-c10000` |
| a cited work founds C1 | `wrk-w00000 --founds--> clm-c10000` |
| an axiom founds C1 | `axm-a00000 --founds--> clm-c10000` |

Every node is named by its minted id, a work and an axiom included; a label is a field, never an id
([SPEC §10 Five node types](../../10-metadata-format-version/five-node-types.md)). An
analysis lifts or refutes and never supplies a premise, so a `supports` or `founds` edge from one is
refused.

`edges --id <id>` reads the stored edges from the reader's side ([§8](../../8-maintenance-subcommands/edges.md)): with `--direction in`
those whose target is the node (the other end supports it), with `out` those whose source is the
node, each row carrying its `bearing` — null on a row that cannot have one.

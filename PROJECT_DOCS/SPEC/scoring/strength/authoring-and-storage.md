## Authoring and storage

The register is claim-centric: a claim lists what it rests on.

| register (claim C1) | stored edge (push) |
|---|---|
| `- rests-on:` | |
| `  - clm-c20000 — the lemma (strength 0.65) (bearing 0.40)` | `clm-c20000 --supports--> clm-c10000` |
| `  - wrk-w00000 — A Cited Work (bearing 0.70)` | `wrk-w00000 --founds--> clm-c10000` |
| `  - axm-a00000 — a labelling rule (bearing 0.90)` | `axm-a00000 --founds--> clm-c10000` |

A bullet's target decides the relation: a `clm-` or `exp-` token is a `supports` edge into the entry,
and a `wrk-` or `axm-` token a `founds` edge into it. Every node is named by its minted id, a work
and an axiom included; a label is a field, never an id ([page §1.1](../../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)). An `ana-` token is refused — an analysis lifts or refutes and never supplies a
premise — and so is a bullet naming more than one scoring target.

`edges --id <id>` reads the stored edges from the reader's side ([§8](../../8-maintenance-subcommands/edges.md)): with `--direction in`
those whose target is the node (the other end supports it), with `out` those whose source is the
node, each row carrying its `bearing` — null on a row that cannot carry one.

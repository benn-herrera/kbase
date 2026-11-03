### Axioms

An axiom is declared by its node record, which `add-axiom` writes ([§8](../8-maintenance-subcommands/index.md));
`axioms.md` is prose stating it, one `### Axiom <label>: <title><!-- kbase-id: <id> -->` heading
each, the marker pairing heading and record ([page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), and is not read for declarations. `verify`'s
`axiom-heading` check holds the two together by the three checks page §1.1 lists, a fault the
author's prose edit clears ([page §3.6](../../../NEW_MCP_CLI_DESIGN/3-operations/3.6-checks-and-output/verify.md)).


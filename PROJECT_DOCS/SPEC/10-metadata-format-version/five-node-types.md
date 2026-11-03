### Five node types

`claim`, `analysis`, `experiment`, `work` and `axiom`. `node_type`,
`target_kind`, the index, the sheet and the result documents use these five words and no other.
Every id is minted, its kind's three-letter prefix over a minted body, and a work's citation key and
an axiom's label are its record's `label` field ([page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)), so a token names its kind without
translation. An axiom is declared by its record and stated in `axioms.md` (above), under the
heading its marker pairs with the record. Authoring prose may
say *invariant*, *framework premise*, *framework*, *proposition*, *evidence*, *trial*, *cited work*
or *reference*: *invariant* and *framework premise* name an axiom, *proposition* a claim, *evidence*
and *trial* an experiment, *cited work* and *reference* a work; *framework* names the document that
declares axioms and is never a node type. The graph has one word per role; the other words are the
author's.


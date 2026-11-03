### The node vocabulary

`kb.NodeKinds` is five words — `claim`, `analysis`, `experiment`, `axiom`,
`work` — and nothing else is a `node_type` or a `target_kind`. `kb.BearingLegal(relation, sourceKind,
targetKind)` is the one predicate answering whether a pairing may carry a bearing: every scoring
pairing may, and `references` and `demoted` may not; it answers that and never whether an edge may
exist. Every node id is minted ([page §1.1](../../../NEW_MCP_CLI_DESIGN/1-model/1.1-nodes.md)): its kind's three-letter prefix over a body
`kb.MintID` draws, one grammar for every kind (`kb.IDBody`, over `kb.IDKinds`). Each prefix is a
`format400` constant re-exported through `kb` (`kb.ClaimPrefix`, `kb.AnalysisPrefix`,
`kb.ExperimentPrefix`, `kb.WorkPrefix`, `kb.AxiomPrefix`); `kb.NodeIDKind` reads an id's kind off
them, and `kb.NodeIDs` finds an id of any kind in text. A work's citation key and an axiom's label
live in the record's `label` field (`WorkNode.Label`, `AxiomNode.Label`), never in the id; the
strict load holds a work's label to `kb.WorkLabelPattern`. An axiom's prose is `axioms.md`, its
heading paired with its record by the axiom's marker: `kb.AxiomHeadings` reads each heading carrying
an axiom's marker, and each unmarked `### Axiom ` heading, for `add-axiom`, `edit-axiom`,
`remove-node` and `verify`'s `axiom-heading` check, and `kb.AxiomAnchor` gives the derivation an
axiom's anchor, that of its marked heading. A statement marker hosts only a claim, an analysis or
an experiment (`kb.HostedIDKinds`); an axiom's or a work's id in a marker hosts nothing.
`update-format` mints each `3.0.0` work's and axiom's id from its `3.0.0` id alone
(`from300to400`'s `mintIDs`). The legacy declaration forms are
the superseded readers' alone (`internal/formats`). SPEC §10 carries the prose legend the authors'
words map through, and it is the one place the synonyms live.


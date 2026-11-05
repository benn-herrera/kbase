### The node vocabulary

**Novel.** What a claim graph is made of, THESIS "What is new" item 1: five node kinds and four
scoring relations, defined in SPEC §10 Five node types.

**Pattern:** Prefixed random ids.

`kb.NodeKinds` is five words, `claim`, `analysis`, `experiment`, `axiom` and `work`, and nothing else is
a `node_type` or a `target_kind`. `kb.BearingLegal(relation, sourceKind, targetKind)` is the single
source of whether a pairing may have a bearing: every scoring pairing may, `references` and `demoted`
may not. It answers that and never whether an edge may exist.

Every node id is minted: its kind's three-letter prefix over a body `kb.MintID` draws, `kb.HashLen`
characters uniform over `kb.HashAlphabet` and redrawn while taken. One grammar serves every kind
(`kb.IDBody`, over `kb.IDKinds`). Each prefix is a `format400` constant re-exported through `kb`
(`kb.ClaimPrefix`, `kb.AnalysisPrefix`, `kb.ExperimentPrefix`, `kb.WorkPrefix`, `kb.AxiomPrefix`);
`kb.NodeIDKind` reads an id's kind off them, and `kb.NodeIDs` finds an id of any kind in text. A KB
converted from `3.0.0` has its work and axiom ids minted by the migration ([§12](../12-constants/format-migration-spec-10/index.md)).

A work's citation key and an axiom's label live in the record's `label` field (`WorkNode.Label`,
`AxiomNode.Label`), never in the id; the strict load requires a work's label to match `kb.WorkLabelPattern`.
An axiom's prose is `axioms.md`, its heading paired with its record by the axiom's marker.
`kb.AxiomHeadings` reads each heading containing an axiom's marker, and each unmarked `### Axiom `
heading, for `add-axiom`, `edit-axiom`, `remove-node` and `verify`'s `axiom-heading` check.
`kb.AxiomAnchor` gives `index.Derive` an axiom's anchor, that of its marked heading. A statement marker
hosts only a claim, an analysis or an experiment (`kb.HostedIDKinds`); an axiom's or a work's id in a
marker hosts nothing.

The legacy declaration forms are the superseded readers' alone (`internal/formats`). SPEC §10 is the
single source of the prose legend the authors' words map through.

